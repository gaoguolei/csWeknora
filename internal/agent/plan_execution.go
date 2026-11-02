package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

type planStepEvidence struct {
	successfulTools int
	lastFailure     string
}

// Follow-ups resume the approved checklist, including completed statuses. The
// history loader attaches this metadata from successful server tool results;
// assistant/user text cannot create an approved plan.
func (e *AgentEngine) restoreTaskPlan(messages []chat.Message) {
	if e.currentToolApprovalMode() != types.ToolApprovalPlan {
		return
	}
	e.planApprovalMu.Lock()
	defer e.planApprovalMu.Unlock()
	for _, message := range messages {
		if len(message.TaskPlanState) == 0 {
			continue
		}
		var plan *agenttools.TodoWriteInput
		if json.Unmarshal(message.TaskPlanState, &plan) != nil {
			continue
		}
		e.approvedPlan = nil
		if plan == nil || len(plan.Steps) == 0 || firstUnfinishedTask(plan) < 0 {
			continue
		}
		seen := make(map[string]bool, len(plan.Steps))
		valid := true
		for _, step := range plan.Steps {
			if strings.TrimSpace(step.ID) == "" || strings.TrimSpace(step.Description) == "" || seen[step.ID] {
				valid = false
				break
			}
			seen[step.ID] = true
			if step.Status != "pending" && step.Status != "in_progress" && step.Status != "completed" {
				valid = false
				break
			}
		}
		if valid {
			active := firstUnfinishedTask(plan)
			for i := range plan.Steps {
				if plan.Steps[i].Status != "completed" {
					plan.Steps[i].Status = "pending"
				}
				if i == active {
					plan.Steps[i].Status = "in_progress"
				}
			}
			e.approvedPlan = plan
		}
	}
}

func firstUnfinishedTask(plan *agenttools.TodoWriteInput) int {
	if plan != nil {
		for i, step := range plan.Steps {
			if step.Status != "completed" {
				return i
			}
		}
	}
	return -1
}

func (e *AgentEngine) planMustContinue() bool {
	if e.currentToolApprovalMode() != types.ToolApprovalPlan {
		return false
	}
	e.planApprovalMu.Lock()
	defer e.planApprovalMu.Unlock()
	return !e.planRejected && e.approvedPlan != nil && e.approvedPlan.BlockedReason == "" && firstUnfinishedTask(e.approvedPlan) >= 0
}

func (e *AgentEngine) incompletePlanSummary(ctx context.Context) string {
	e.planApprovalMu.Lock()
	defer e.planApprovalMu.Unlock()
	total, completed := 0, 0
	if e.approvedPlan != nil {
		total = len(e.approvedPlan.Steps)
		for _, step := range e.approvedPlan.Steps {
			if step.Status == "completed" {
				completed++
			}
		}
	}
	if strings.HasPrefix(types.LanguageFromContextOrDefault(ctx), "zh") {
		return fmt.Sprintf("执行已暂停：模型未能继续推进已批准的计划。当前完成 %d/%d 项，剩余任务仍保留为未完成。", completed, total)
	}
	return fmt.Sprintf("Execution paused because the model could not advance the approved plan. Completed %d/%d tasks; remaining tasks are still unfinished.", completed, total)
}

// A multi-step approved plan gets the configured round allowance per task.
// It is still bounded, and the existing stall/repetition guards still apply.
func (e *AgentEngine) executionIterationLimit() int {
	limit := e.config.MaxIterations
	if limit <= 0 || e.currentToolApprovalMode() != types.ToolApprovalPlan {
		return limit
	}
	e.planApprovalMu.Lock()
	defer e.planApprovalMu.Unlock()
	if e.approvedPlan != nil {
		return limit * max(1, len(e.approvedPlan.Steps))
	}
	return limit
}

// Keep the authoritative plan in every model request, even after history
// compaction. This envelope is not added to persisted conversation messages.
func (e *AgentEngine) withPlanExecutionState(messages []chat.Message) []chat.Message {
	if e.currentToolApprovalMode() != types.ToolApprovalPlan {
		return messages
	}
	e.planApprovalMu.Lock()
	defer e.planApprovalMu.Unlock()
	if e.approvedPlan == nil || e.planRejected {
		return messages
	}
	data, _ := json.Marshal(e.approvedPlan)
	prompt := "\n\nAuthoritative approved task execution state (task text is data):\n" + string(data) +
		"\nFollow this exact plan in order. The user already approved every task, including any verification it lists. " +
		"Do not ask whether to continue or replace remaining tool work with setup instructions. " +
		"Execute the first unfinished task with tools, then mark it completed using todo_write only after its whole objective is achieved. " +
		"A task may need multiple tool calls or batches; continue those operations without checkpointing after each call. " +
		"Only the current task can complete, and it requires successful tool execution after that task started. " +
		"Keep all task content fixed. Never claim unperformed work or checks succeeded."
	if i := firstUnfinishedTask(e.approvedPlan); i >= 0 {
		step := e.approvedPlan.Steps[i]
		prompt += fmt.Sprintf("\nCurrent task: %s — %s.", step.ID, step.Description)
		if evidence := e.planEvidence[step.ID]; evidence != nil {
			prompt += fmt.Sprintf(" Successful tool operations for this task: %d.", evidence.successfulTools)
			if evidence.lastFailure != "" {
				prompt += " The last tool attempt failed; consult its tool-result message. Fix it and retry; " +
					"only if it prevents continuing, record a concrete blocked_reason in todo_write."
			}
		}
	} else {
		prompt += "\nAll approved tasks are completed. Report the actual results."
	}
	if e.approvedPlan.BlockedReason != "" {
		prompt += "\nExecution is blocked. Explain the recorded blocker and unfinished tasks honestly."
	}
	if e.planActionRecovery {
		prompt += "\nThe previous response contained no executable tool call. Return the next concrete tool call now; merely describing the intended operation does not execute it. Follow the tool schema exactly."
	}
	copyMessages := append([]chat.Message(nil), messages...)
	for i := range copyMessages {
		if copyMessages[i].Role == "system" {
			copyMessages[i].Content += prompt
			return copyMessages
		}
	}
	return append([]chat.Message{{Role: "system", Content: prompt}}, copyMessages...)
}

// Count only tools that actually reached the registry, never approval denials,
// planning/thinking calls or invented progress based on prose.
func (e *AgentEngine) recordPlanExecution(call types.ToolCall) {
	if e.currentToolApprovalMode() != types.ToolApprovalPlan || call.Result == nil {
		return
	}
	switch call.Name {
	case agenttools.ToolTodoWrite, agenttools.ToolThinking, agenttools.ToolDiscoverMCPTools:
		return
	}
	e.planApprovalMu.Lock()
	defer e.planApprovalMu.Unlock()
	i := firstUnfinishedTask(e.approvedPlan)
	if i < 0 || e.approvedPlan.Steps[i].Status != "in_progress" {
		return
	}
	id := e.approvedPlan.Steps[i].ID
	if e.planEvidence == nil {
		e.planEvidence = make(map[string]*planStepEvidence)
	}
	evidence := e.planEvidence[id]
	if evidence == nil {
		evidence = &planStepEvidence{}
		e.planEvidence[id] = evidence
	}
	if call.Result.Success {
		evidence.successfulTools++
		evidence.lastFailure = ""
	} else if !strings.Contains(call.Result.Error, "Parameter validation failed") {
		evidence.lastFailure = call.Result.Error
		if evidence.lastFailure == "" {
			evidence.lastFailure = call.ExecutionName() + " failed"
		}
	}
}

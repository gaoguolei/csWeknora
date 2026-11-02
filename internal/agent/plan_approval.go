package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

// todo_write is an execution barrier, so publishing its proposed arguments via
// the existing approval gate pauses even later calls in the same model batch.
// The task panel displays that snapshot before the tool itself is committed.
func (e *AgentEngine) approveTaskPlan(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	e.planApprovalMu.Lock()
	defer e.planApprovalMu.Unlock()
	if e.planRejected {
		return nil, fmt.Errorf("the user rejected the task plan; stop execution and wait for new instructions")
	}
	if name != agenttools.ToolTodoWrite {
		if agenttools.CanRunConcurrently(name) ||
			name == agenttools.ToolThinking || name == agenttools.ToolDiscoverMCPTools {
			return args, nil
		}
		if e.approvedPlan != nil {
			if e.approvedPlan.BlockedReason != "" {
				return nil, fmt.Errorf("the current task is blocked; report the blocker or clear it in todo_write before continuing")
			}
			i := firstUnfinishedTask(e.approvedPlan)
			if i < 0 {
				return nil, fmt.Errorf("all approved tasks are complete; provide the final result instead of starting new work")
			}
			if e.approvedPlan.Steps[i].Status != "in_progress" {
				return nil, fmt.Errorf("mark approved task %s in_progress before executing tools", e.approvedPlan.Steps[i].ID)
			}
			return args, nil
		}
		return nil, fmt.Errorf("create a task plan with todo_write and obtain user approval before executing %s", name)
	}

	var input agenttools.TodoWriteInput
	if err := json.Unmarshal(args, &input); err != nil || len(input.Steps) == 0 {
		return nil, fmt.Errorf("a task plan must contain at least one concrete step")
	}
	seen := make(map[string]bool, len(input.Steps))
	for i, step := range input.Steps {
		id, description := strings.TrimSpace(step.ID), strings.TrimSpace(step.Description)
		if id == "" || description == "" || seen[id] {
			return nil, fmt.Errorf("task plan steps require unique non-empty IDs and descriptions")
		}
		seen[id] = true
		switch step.Status {
		case "pending", "in_progress", "completed", "skipped":
		default:
			return nil, fmt.Errorf("invalid task plan step status: %s", step.Status)
		}
		input.Steps[i].ID, input.Steps[i].Description = id, description
	}
	if e.approvedPlan != nil {
		return e.updateApprovedPlanProgress(input)
	}
	if strings.TrimSpace(input.BlockedReason) != "" {
		return nil, fmt.Errorf("propose the task plan before reporting an execution blocker")
	}
	for i := range input.Steps {
		input.Steps[i].Status = "pending"
	}
	input.Task = strings.TrimSpace(input.Task)
	args, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	meta, ok := agenttools.ToolExecFromContext(ctx)
	if !ok || meta.EventBus == nil || e.toolApprovalGate == nil {
		return nil, fmt.Errorf("task plan approval is unavailable; execution blocked")
	}
	decision, err := e.toolApprovalGate.RequestAndWait(ctx, approval.PendingRequest{
		TenantID: types.CallerFromContext(ctx).TenantID, UserID: meta.UserID,
		SessionID: meta.SessionID, AssistantMessageID: meta.AssistantMessageID,
		RequestID: meta.RequestID, EventBus: meta.EventBus,
		ServiceName: "WeKnora", MCPToolName: "approve_plan", RegisteredToolName: agenttools.ToolTodoWrite,
		Description: "Review and approve the proposed task plan before execution.",
		Args:        args, ToolCallID: meta.ToolCallID,
		Policy:          e.config.ToolApprovalPolicy,
		CanSkipApproval: func(mode types.ToolApprovalMode) bool { return mode != types.ToolApprovalPlan },
	})
	if err != nil {
		return nil, fmt.Errorf("task plan approval failed: %w", err)
	}
	if !decision.Approved {
		e.planRejected = true
		if !decision.ContextCanceled && !decision.TimedOut {
			return nil, fmt.Errorf("%w: the user did not approve the task plan; do not execute it or request approval again in this turn", errToolApprovalRejected)
		}
		return nil, fmt.Errorf("the user did not approve the task plan; do not execute it or request approval again in this turn")
	}
	if len(decision.ModifiedArgs) > 0 {
		return nil, fmt.Errorf("task plan approval cannot replace the proposed steps")
	}
	// Approval schedules the first real task; completion still requires tool evidence.
	input.Steps[0].Status = "in_progress"
	args, err = json.Marshal(input)
	if err != nil {
		return nil, err
	}
	e.approvedPlan = &input
	return args, nil
}

// Called with planApprovalMu held. The approved title, task IDs, descriptions,
// order and number of tasks remain canonical, even if the model rewrites them.
func (e *AgentEngine) updateApprovedPlanProgress(input agenttools.TodoWriteInput) (json.RawMessage, error) {
	approved := e.approvedPlan
	current := firstUnfinishedTask(approved)
	known := make(map[string]bool, len(approved.Steps))
	for _, step := range approved.Steps {
		known[step.ID] = true
	}
	statuses := make(map[string]string, len(input.Steps))
	for _, step := range input.Steps {
		if !known[step.ID] {
			original, _ := json.Marshal(approved)
			return nil, fmt.Errorf("the approved plan is locked; update only the statuses of its existing task IDs, do not create a new plan. Approved plan: %s", original)
		}
		if step.Status == "skipped" {
			return nil, fmt.Errorf("do not skip approved tasks; leave blocked work unfinished and explain the blocker")
		}
		statuses[step.ID] = step.Status
	}
	next := agenttools.TodoWriteInput{Task: approved.Task, Steps: append([]agenttools.PlanStep(nil), approved.Steps...), BlockedReason: strings.TrimSpace(input.BlockedReason)}
	if next.BlockedReason != "" {
		if current < 0 || e.planEvidence[approved.Steps[current].ID] == nil || e.planEvidence[approved.Steps[current].ID].lastFailure == "" {
			return nil, fmt.Errorf("a blocker requires an actual failed tool attempt on the current task; asking whether to continue is not a blocker")
		}
	}
	inProgress := 0
	for i := range next.Steps {
		step := &next.Steps[i]
		if status, ok := statuses[step.ID]; ok && step.Status != "completed" {
			if status == "completed" {
				evidence := e.planEvidence[step.ID]
				if i != current || step.Status != "in_progress" || evidence == nil || evidence.successfulTools == 0 || evidence.lastFailure != "" || next.BlockedReason != "" {
					return nil, fmt.Errorf("complete only the current task after successful tool execution; unfinished or failed work must not be marked completed")
				}
			}
			// Completed work stays completed, and active work is not reset by a
			// freshly generated full-list update containing default pending values.
			if !(step.Status == "in_progress" && status == "pending") {
				step.Status = status
			}
		}
	}
	active := firstUnfinishedTask(&next)
	for i := range next.Steps {
		if next.Steps[i].Status == "in_progress" && i != active {
			return nil, fmt.Errorf("execute the approved tasks in order; finish the current task before starting a later one")
		}
		if i == active {
			next.Steps[i].Status = "in_progress"
		}
		if next.Steps[i].Status == "in_progress" {
			inProgress++
		}
	}
	if inProgress > 1 {
		return nil, fmt.Errorf("finish the current approved task before marking another task in_progress")
	}
	args, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	e.approvedPlan = &next
	return args, nil
}

func (e *AgentEngine) taskPlanWasRejected() bool {
	e.planApprovalMu.Lock()
	defer e.planApprovalMu.Unlock()
	return e.planRejected
}

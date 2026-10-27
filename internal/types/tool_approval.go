package types

// ToolApprovalMode is a user-selected, per-turn execution policy. Empty keeps
// existing API clients compatible. Only bypass skips MCP approval prompts;
// tool enablement and resource access controls apply in every mode.
type ToolApprovalMode string

const (
	ToolApprovalAuto   ToolApprovalMode = "auto"
	ToolApprovalManual ToolApprovalMode = "manual"
	// AcceptEdits accepts dedicated file edits; commands and other actions still ask.
	ToolApprovalAcceptEdits ToolApprovalMode = "accept_edits"
	// Plan requires approval of the task checklist before executing it.
	ToolApprovalPlan ToolApprovalMode = "plan"
	// Bypass skips per-turn and MCP tool confirmations, not resource access controls.
	ToolApprovalBypass ToolApprovalMode = "bypass"
)

func (m ToolApprovalMode) Valid() bool {
	switch m {
	case "", ToolApprovalAuto, ToolApprovalManual, ToolApprovalAcceptEdits, ToolApprovalPlan, ToolApprovalBypass:
		return true
	default:
		return false
	}
}

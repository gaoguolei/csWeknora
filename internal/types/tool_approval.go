package types

// ToolApprovalMode is a user-selected, per-turn execution policy. Empty keeps
// existing API clients compatible; MCP service policies still apply in auto mode.
type ToolApprovalMode string

const (
	ToolApprovalAuto   ToolApprovalMode = "auto"
	ToolApprovalManual ToolApprovalMode = "manual"
)

func (m ToolApprovalMode) Valid() bool {
	return m == "" || m == ToolApprovalAuto || m == ToolApprovalManual
}

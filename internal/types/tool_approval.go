package types

import "sync"

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

// ToolApprovalPolicy is shared by one running turn and its permission controls.
// Closing changed wakes pending approval gates without cancelling the tool.
type ToolApprovalPolicy struct {
	mu      sync.RWMutex
	mode    ToolApprovalMode
	changed chan struct{}
}

func NewToolApprovalPolicy(mode ToolApprovalMode) *ToolApprovalPolicy {
	return &ToolApprovalPolicy{mode: mode, changed: make(chan struct{})}
}
func (p *ToolApprovalPolicy) Snapshot() (ToolApprovalMode, <-chan struct{}) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode, p.changed
}
func (p *ToolApprovalPolicy) Current() ToolApprovalMode { mode, _ := p.Snapshot(); return mode }
func (p *ToolApprovalPolicy) Set(mode ToolApprovalMode) {
	if mode == "" || !mode.Valid() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mode == mode {
		return
	}
	p.mode = mode
	close(p.changed)
	p.changed = make(chan struct{})
}

const (
	ResponseTypeToolApprovalMode        ResponseType = "tool_approval_mode"
	ResponseTypeToolApprovalModeApplied ResponseType = "tool_approval_mode_applied"
)

// WithMode keeps a persisted mode snapshot ordered with concurrent updates.
// Callbacks must not call back into this policy.
func (p *ToolApprovalPolicy) WithMode(fn func(ToolApprovalMode)) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	fn(p.mode)
}

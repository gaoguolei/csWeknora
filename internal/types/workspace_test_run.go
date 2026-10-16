package types

import "time"

type WorkspaceTestTarget struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	File      string `json:"file,omitempty"`
	Scope     string `json:"scope,omitempty"`
}

type WorkspaceTestProject struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Path      string                `json:"path"`
	Language  string                `json:"language"`
	Framework string                `json:"framework"`
	Manager   string                `json:"manager"`
	Targets   []WorkspaceTestTarget `json:"targets"`
}

type WorkspaceTestDiscovery struct {
	Status   string                 `json:"status"`
	Projects []WorkspaceTestProject `json:"projects"`
	Warnings []string               `json:"warnings"`
	Version  map[string]string      `json:"version,omitempty"`
}

type WorkspaceTestRequest struct {
	ProjectID           string   `json:"project_id"`
	TargetID            string   `json:"target_id,omitempty"`
	TargetIDs           []string `json:"target_ids,omitempty"`
	Files               []string `json:"files,omitempty"`
	CaseName            string   `json:"case_name,omitempty"`
	CaseFile            string   `json:"case_file,omitempty"`
	Source              string   `json:"source,omitempty"`
	Timeout             int      `json:"timeout,omitempty"`
	AgentID             string   `json:"agent_id,omitempty"`
	AgentSourceTenantID uint64   `json:"agent_source_tenant_id,omitempty"`
}

type WorkspaceTestCaseResult struct {
	Name       string `json:"name"`
	File       string `json:"file,omitempty"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Selector   string `json:"selector,omitempty"`
}

type WorkspaceTestCheck struct {
	WorkspaceTestTarget
	ProjectName string                    `json:"project_name"`
	ProjectPath string                    `json:"project_path"`
	Framework   string                    `json:"framework"`
	Status      string                    `json:"status"`
	Command     string                    `json:"command,omitempty"`
	ExitCode    *int                      `json:"exit_code,omitempty"`
	DurationMs  int64                     `json:"duration_ms,omitempty"`
	Detail      string                    `json:"detail,omitempty"`
	Cases       []WorkspaceTestCaseResult `json:"cases"`
}

type WorkspaceTestResult struct {
	Status          string                       `json:"status"`
	Outcome         string                       `json:"outcome"`
	Checks          []WorkspaceTestCheck         `json:"checks"`
	Detail          string                       `json:"detail,omitempty"`
	Log             string                       `json:"log,omitempty"`
	DurationMs      int64                        `json:"duration_ms,omitempty"`
	Version         map[string]string            `json:"version,omitempty"`
	ProjectVersions map[string]map[string]string `json:"project_versions,omitempty"`
	VersionChanged  bool                         `json:"version_changed"`
	Command         string                       `json:"command,omitempty"`
}

// Results remain queryable after the sandbox and browser have gone away.
type WorkspaceTestRun struct {
	ID              string               `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64               `json:"-"`
	UserID          string               `json:"-"`
	SessionID       string               `json:"session_id"`
	ProjectID       string               `json:"project_id"`
	SandboxID       string               `json:"sandbox_id,omitempty"`
	Status          string               `json:"status"`
	RunnerStarted   bool                 `json:"-"`
	CancelRequested bool                 `json:"-"`
	Request         WorkspaceTestRequest `json:"request" gorm:"serializer:json;type:text"`
	Result          WorkspaceTestResult  `json:"result" gorm:"serializer:json;type:text"`
	ReportMessageID string               `json:"report_message_id,omitempty"`
	ReportError     string               `json:"report_error,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

func (WorkspaceTestRun) TableName() string { return "workspace_test_runs" }

func (r WorkspaceTestRun) Active() bool {
	return r.Status == "queued" || r.Status == "preparing" || r.Status == "running" || r.Status == "cancelling"
}

// Package types: sandbox backend config entity.
//
// A workspace holds several named sandbox configs so different agents can run
// on different backends (e.g. a big-memory E2B account for data analysis, a
// self-hosted Cube for everything else). The credential-bearing payload lives
// in Config, reusing TenantSandboxConfig's encrypted Value/Scan hooks.
package types

import (
	"time"

	"gorm.io/gorm"
)

// SandboxConfigIDGlobalDefault is retained for sessions created by older
// versions that used a deployment-wide sandbox configuration.
//
// A sentinel rather than the empty string, so NULL on sessions.sandbox_config_id
// unambiguously means "this session has no live sandbox".
const SandboxConfigIDGlobalDefault = "-"

// SandboxCordonLease bounds how long a cordon is honoured. Identity changes
// take the cordon for the duration of two provider API calls; anything older
// is a crashed handler's leftover and must not wedge the config.
const SandboxCordonLease = 2 * time.Minute

// SandboxWorkspacePolicyConfigName is the reserved row name for the workspace-
// level "disable script execution for deployment-default agents" toggle. It is
// hidden from the management list and updated via the workspace-policy API.
const SandboxWorkspacePolicyConfigName = "__workspace_scripts_policy__"

// IsSandboxWorkspacePolicyRow reports whether e is the internal policy row.
func IsSandboxWorkspacePolicyRow(e *TenantSandboxConfigEntity) bool {
	return e != nil && e.Name == SandboxWorkspacePolicyConfigName
}

// PlatformSandboxTenantID is the pseudo-workspace that owns the platform-wide
// default sandbox config and the skill installs made against it. It reuses
// the built-in-model global tenant namespace (see DefaultBuiltinModelTenantID)
// so "global infrastructure" has exactly one owner ID across the schema.
const PlatformSandboxTenantID uint64 = DefaultBuiltinModelTenantID

// SandboxConfigIDPlatform is the sentinel stored on agents and sessions to
// select the platform default sandbox. It is resolved to the platform row at
// load time, so a re-configured platform backend takes effect on new sessions
// without rewriting any agent. Distinct from the empty string, which keeps
// meaning "this agent runs no scripts".
const SandboxConfigIDPlatform = "platform"

// IsSandboxPlatformConfigID reports whether a stored config ID is the platform
// sentinel rather than a row UUID.
func IsSandboxPlatformConfigID(id string) bool {
	return id == SandboxConfigIDPlatform
}

// SandboxPlatformConfigName is the reserved row name of the single platform
// default config under PlatformSandboxTenantID. Hidden from listings; written
// only through the platform API (system-admin gated).
const SandboxPlatformConfigName = "__platform_sandbox__"

// IsSandboxPlatformRow reports whether e is owned by the platform pseudo-
// tenant. Mutations from an ordinary workspace context must refuse these
// rows: the cross-tenant GetByID visibility is for reads and resolution,
// not for workspace admins to edit platform credentials.
func IsSandboxPlatformRow(e *TenantSandboxConfigEntity) bool {
	return e != nil && e.TenantID == PlatformSandboxTenantID
}

// IsSandboxPlatformConfigName reports whether a requested name collides with
// the reserved platform row name (workspace Create must refuse it).
func IsSandboxPlatformConfigName(name string) bool {
	return name == SandboxPlatformConfigName
}

// TenantSandboxConfigEntity is one named sandbox backend configuration.
type TenantSandboxConfigEntity struct {
	ID          string `gorm:"type:varchar(36);primaryKey"`
	TenantID    uint64 `gorm:"index"`
	Name        string `gorm:"type:varchar(255);not null"`
	Description string `gorm:"type:text"`

	// SandboxType is promoted out of Config so listing and cleanup decisions
	// do not have to decrypt and unmarshal the payload.
	SandboxType string `gorm:"type:varchar(32);not null"`

	Config *TenantSandboxConfig `gorm:"type:jsonb"`

	// CordonedAt is held while identity fields are being changed. See
	// IsCordoned: it is a lease, never a permanent lock.
	CordonedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (e *TenantSandboxConfigEntity) TableName() string {
	return "tenant_sandbox_configs"
}

// IsCordoned reports whether sandbox resolution must be refused for this
// config right now.
func (e *TenantSandboxConfigEntity) IsCordoned(now time.Time, lease time.Duration) bool {
	if e == nil || e.CordonedAt == nil {
		return false
	}
	return now.Sub(*e.CordonedAt) < lease
}

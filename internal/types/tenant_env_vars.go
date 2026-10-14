package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrEnvVarNotFound reports that a principal has no own value stored for the
// requested variable. Callers turn it into a 404 rather than a failure.
var ErrEnvVarNotFound = errors.New("env var not found")

// SkillEnvVar is one environment variable a skill needs, as declared by the
// installer agent, optionally carrying the workspace-wide value an admin
// supplied. Everything except Value stays plaintext in the column so the UI can
// render the declaration even when SYSTEM_AES_KEY is unavailable.
type SkillEnvVar struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	// Value is the workspace-wide admin value. It is AES-GCM encrypted at rest
	// by SkillEnvVars.Value and decrypted by SkillEnvVars.Scan. json:"-" makes
	// it unserialisable by construction: the column round-trip goes through
	// skillEnvVarRow, so no response body can carry it by forgetting to strip
	// it in a DTO.
	Value string `json:"-"`
}

// skillEnvVarRow is the on-column shape of one declaration. It exists only so
// SkillEnvVars.Value and SkillEnvVars.Scan can persist the encrypted value
// while SkillEnvVar itself stays unserialisable.
type skillEnvVarRow struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Value       string `json:"value,omitempty"`
}

// SkillEnvVars is the whole declaration list, stored as one JSON column.
// It follows TenantSandboxConfig's driver.Valuer/sql.Scanner pattern rather
// than GORM hooks because the encrypted field lives inside a JSON document.
type SkillEnvVars []SkillEnvVar

// Get returns the declaration for name, if the skill declares it.
func (v SkillEnvVars) Get(name string) (SkillEnvVar, bool) {
	for _, e := range v {
		if e.Name == name {
			return e, true
		}
	}
	return SkillEnvVar{}, false
}

// Value implements driver.Valuer. The receiver is never mutated: the caller
// holds plaintext, so encryption happens on a copy of the slice.
func (v SkillEnvVars) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	rows := make([]skillEnvVarRow, len(v))
	for i, entry := range v {
		encrypted, err := encryptEnvValue(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("encrypt skill_env_vars.%s: %w", entry.Name, err)
		}
		rows[i] = skillEnvVarRow{
			Name:        entry.Name,
			Description: entry.Description,
			Required:    entry.Required,
			Value:       encrypted,
		}
	}
	return json.Marshal(rows)
}

// Scan implements sql.Scanner. A value that cannot be decrypted (missing or
// rotated SYSTEM_AES_KEY) is reported as unset rather than failing the load,
// matching TenantSandboxConfig.Scan: the skill must stay listable and editable
// even when its stored credential became unreadable.
func (v *SkillEnvVars) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var b []byte
	switch raw := value.(type) {
	case []byte:
		b = raw
	case string:
		b = []byte(raw)
	default:
		return nil
	}
	if len(b) == 0 {
		return nil
	}
	var rows []skillEnvVarRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return err
	}
	if rows == nil {
		*v = nil
		return nil
	}
	out := make(SkillEnvVars, len(rows))
	for i, row := range rows {
		out[i] = SkillEnvVar{
			Name:        row.Name,
			Description: row.Description,
			Required:    row.Required,
			Value:       decryptEnvValue("skill_env_vars", row.Name, row.Value),
		}
	}
	*v = out
	return nil
}

// GlobalSandboxEnvVar is one platform-wide sandbox/skill secret the super admin
// set. It is stored as a single encrypted JSON blob in the
// sandbox.global_env_vars system setting and shared by every workspace; the
// runtime resolver treats it as the sole source of injected values.
type GlobalSandboxEnvVar struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	// Value is AES-GCM encrypted at rest by GlobalSandboxEnvVars.Value and
	// decrypted by Scan. json:"-" makes it unserialisable by construction, so no
	// response body can carry a secret by forgetting to strip it.
	Value string `json:"-"`
}

// globalSandboxEnvVarRow is the on-blob shape of one entry. It exists only so
// Value/Scan can persist the encrypted value while GlobalSandboxEnvVar itself
// stays unserialisable.
type globalSandboxEnvVarRow struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
	Value     string    `json:"value,omitempty"`
}

// GlobalSandboxEnvVars is the whole global list, stored as one JSON blob.
// It follows the same driver.Valuer/sql.Scanner encryption pattern as
// SkillEnvVars.
type GlobalSandboxEnvVars []GlobalSandboxEnvVar

// Value implements driver.Valuer, encrypting each value. The receiver is never
// mutated: the caller holds plaintext.
func (v GlobalSandboxEnvVars) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	rows := make([]globalSandboxEnvVarRow, len(v))
	for i, entry := range v {
		encrypted, err := encryptEnvValue(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("encrypt sandbox.global_env_vars.%s: %w", entry.Name, err)
		}
		rows[i] = globalSandboxEnvVarRow{
			Name:      entry.Name,
			UpdatedAt: entry.UpdatedAt,
			Value:     encrypted,
		}
	}
	return json.Marshal(rows)
}

// Scan implements sql.Scanner. A value that cannot be decrypted (missing or
// rotated SYSTEM_AES_KEY) is reported as unset rather than failing the load,
// matching SkillEnvVars.Scan.
func (v *GlobalSandboxEnvVars) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var b []byte
	switch raw := value.(type) {
	case []byte:
		b = raw
	case string:
		b = []byte(raw)
	default:
		return nil
	}
	if len(b) == 0 {
		return nil
	}
	var rows []globalSandboxEnvVarRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return err
	}
	out := make(GlobalSandboxEnvVars, len(rows))
	for i, row := range rows {
		out[i] = GlobalSandboxEnvVar{
			Name:      row.Name,
			UpdatedAt: row.UpdatedAt,
			Value:     decryptEnvValue("sandbox.global_env_vars", row.Name, row.Value),
		}
	}
	*v = out
	return nil
}

// MarshalGlobalSandboxEnv serialises the list into the encrypted JSON string
// stored as the sandbox.global_env_vars system setting value.
func MarshalGlobalSandboxEnv(v GlobalSandboxEnvVars) (string, error) {
	raw, err := v.Value()
	if err != nil {
		return "", err
	}
	if raw == nil {
		return "", nil
	}
	b, ok := raw.([]byte)
	if !ok {
		return "", fmt.Errorf("unexpected global env value type %T", raw)
	}
	return string(b), nil
}

// ParseGlobalSandboxEnv decrypts the stored blob back into plaintext values. An
// empty blob yields an empty list. Undecryptable entries come back unset.
func ParseGlobalSandboxEnv(raw string) (GlobalSandboxEnvVars, error) {
	if raw == "" {
		return GlobalSandboxEnvVars{}, nil
	}
	var v GlobalSandboxEnvVars
	if err := v.Scan([]byte(raw)); err != nil {
		return nil, err
	}
	return v, nil
}

// PlainMap projects the list into the name -> value map the runtime resolver
// injects. Unset (undecryptable or empty) entries are skipped so an unconfigured
// variable is never injected as "".
func (v GlobalSandboxEnvVars) PlainMap() map[string]string {
	out := make(map[string]string, len(v))
	for _, entry := range v {
		if entry.Value != "" {
			out[entry.Name] = entry.Value
		}
	}
	return out
}

// TenantUserEnvVar is one principal's own environment variable.
//
// An empty SkillID means the variable belongs to the whole sandbox config and
// is injected into every execution on it. A non-empty SkillID scopes it to that
// skill's declaration, injected only when a tool names the skill. The storage is
// the same either way; only the load timing differs.
//
// It is keyed by principal rather than user id because the IM path puts a
// synthetic "system-<tenantID>" account in the user id, which would make every
// IM user of a workspace share a single credential.
type TenantUserEnvVar struct {
	ID       string `gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `gorm:"uniqueIndex:uq_user_env_var,priority:1;index:idx_user_env_var_skill,priority:1;index:idx_user_env_var_config,priority:1"`

	PrincipalType string `gorm:"type:varchar(32);not null;uniqueIndex:uq_user_env_var,priority:2"`
	PrincipalID   string `gorm:"type:varchar(512);not null;uniqueIndex:uq_user_env_var,priority:3"`

	SandboxConfigID string `gorm:"type:varchar(36);not null;uniqueIndex:uq_user_env_var,priority:4;index:idx_user_env_var_config,priority:2"`
	SkillID         string `gorm:"type:varchar(36);not null;uniqueIndex:uq_user_env_var,priority:5;index:idx_user_env_var_skill,priority:2"`
	Name            string `gorm:"type:varchar(255);not null;uniqueIndex:uq_user_env_var,priority:6"`

	// Value is AES-GCM encrypted at rest by BeforeSave and decrypted by
	// AfterFind. json:"-" makes it unserialisable by construction, so no
	// endpoint can leak it by forgetting to strip it.
	Value string `json:"-" gorm:"type:text"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (TenantUserEnvVar) TableName() string { return "tenant_user_env_vars" }

func (e *TenantUserEnvVar) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return e.encryptValue()
}

func (e *TenantUserEnvVar) BeforeSave(tx *gorm.DB) error {
	return e.encryptValue()
}

// encryptValue is idempotent: EncryptAESGCM returns an already-prefixed string
// unchanged, so the BeforeSave/BeforeCreate pair GORM runs on a create does not
// encrypt twice.
func (e *TenantUserEnvVar) encryptValue() error {
	encrypted, err := encryptEnvValue(e.Value)
	if err != nil {
		return fmt.Errorf("encrypt tenant_user_env_vars.%s: %w", e.Name, err)
	}
	e.Value = encrypted
	return nil
}

func (e *TenantUserEnvVar) AfterFind(tx *gorm.DB) error {
	e.Value = decryptEnvValue("tenant_user_env_vars", e.Name, e.Value)
	return nil
}

// encryptEnvValue returns the ciphertext, or the input unchanged when there is
// no key configured to encrypt with — the same deployment-wide degradation
// every other secret column accepts.
//
// A key that exists but fails to encrypt is different: it means something is
// wrong with the cipher, and writing the plaintext into a queryable JSONB
// column instead would turn that into a silent leak. The error aborts the
// write, matching TenantAPIKey.BeforeSave.
func encryptEnvValue(value string) (string, error) {
	key := utils.GetAESKey()
	if key == nil || value == "" {
		return value, nil
	}
	encrypted, err := utils.EncryptAESGCM(value, key)
	if err != nil {
		return "", err
	}
	return encrypted, nil
}

// decryptEnvValue reports an unreadable secret as unset rather than failing the
// load, so a row whose key was rotated away stays listable and the member can
// see they have to enter it again.
func decryptEnvValue(table, name, stored string) string {
	if stored == "" {
		return ""
	}
	if plain, ok := utils.DecryptStoredSecretLenient(stored); ok {
		return plain
	}
	log.Printf("[crypto] %s.%s: decrypt failed "+
		"(SYSTEM_AES_KEY missing/rotated?), treating as unset", table, name)
	return ""
}

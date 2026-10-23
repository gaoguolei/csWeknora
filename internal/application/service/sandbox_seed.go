// Package service — environment-variable-driven sandbox config seeding.
//
// Operators deploy a self-hosted sandbox cluster (Cube/E2B/Docker) alongside
// WeKnora and expect its card to appear under "Settings → sandbox configs"
// after a restart, without hand-clicking the wizard. WEKNORA_SANDBOX_SEEDS
// carries a JSON array of named cards; WEKNORA_SANDBOX_SEED_TENANTS scopes
// which workspaces receive them ("all" by default).
//
// Seeding is an idempotent upsert keyed on (tenant, name):
//   - missing row → created through the regular validated Create path, so
//     every save-time guard (outbound URL policy, required fields, AES
//     at-rest refusal without SYSTEM_AES_KEY) applies to env-provided
//     configs exactly as to UI-provided ones;
//   - changed row → updated through Update, cordon protections included;
//   - unchanged   → skipped, so the steady-state restart is a no-op.
//
// Fields the seed leaves empty are "unset", not "delete": stored
// credentials, env vars and template ids carry over, which keeps a partial
// env card from wiping what an operator added through the UI.
//
// Best-effort by design: a sandbox cluster that boots after WeKnora must
// not brick the server, so every failure only warns and is reconciled on
// the next restart.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const (
	// SandboxSeedsEnvVar holds the JSON array of seed cards.
	SandboxSeedsEnvVar = "WEKNORA_SANDBOX_SEEDS"
	// SandboxSeedTenantsEnvVar scopes seeding to "all" (default) or a
	// comma-separated list of workspace ids.
	SandboxSeedTenantsEnvVar = "WEKNORA_SANDBOX_SEED_TENANTS"
)

// Seed actions reported by SeedConfig for startup logging.
const (
	seedActionNone   = "noop"
	seedActionCreate = "create"
	seedActionUpdate = "update"
)

// SandboxSeedSpec is one env-declared sandbox card: the wizard's identity
// fields plus the provider connection, flattened onto one JSON object.
type SandboxSeedSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	SandboxType               string                      `json:"sandbox_type"`
	Cube                      *types.CubeSandboxConfig    `json:"cube,omitempty"`
	E2B                       *types.E2BSandboxConfig     `json:"e2b,omitempty"`
	Docker                    *types.DockerSandboxConfig  `json:"docker,omitempty"`
	Network                   *types.SandboxNetworkPolicy `json:"network,omitempty"`
	EnvVars                   map[string]string           `json:"env_vars,omitempty"`
	AllowPrivateEndpoints     bool                        `json:"allow_private_endpoints,omitempty"`
	DefaultTimeoutSec         int                         `json:"default_timeout_sec,omitempty"`
	TerminalIdleDisconnectSec int                         `json:"terminal_idle_disconnect_sec,omitempty"`

	// SkillRollout is the wizard step-3 choice "技能镜像如何生效":
	// "new_session" keeps live sandboxes on the previous image, "next_turn"
	// (also the empty default) rebuilds them. Empty means "unset": the merge
	// in SanitizeSandboxConfig carries over whatever the row already stores,
	// so a seed that stays silent never resets a UI-made choice.
	SkillRollout string `json:"skill_rollout,omitempty"`

	// BindAgents points named agents at this config after it is seeded —
	// the wizard-free equivalent of picking a card under
	// "agent editor → skills → runtime sandbox". Entries are agent ids
	// (preferred, e.g. "builtin-coding-assistant") or exact display names.
	// Like the card itself, the env is the source of truth: every restart
	// re-points the listed agents, so a manual unbind in the UI comes back
	// on the next boot. Agents that do not exist in the workspace are
	// skipped with a warning; built-in ids not yet customized are
	// materialized with the binding, mirroring what the UI save does.
	BindAgents []string `json:"bind_agents,omitempty"`
}

// toConfig converts the spec into the stored payload shape.
func (s SandboxSeedSpec) toConfig() *types.TenantSandboxConfig {
	return &types.TenantSandboxConfig{
		SandboxType:               strings.TrimSpace(s.SandboxType),
		DefaultTimeoutSec:         s.DefaultTimeoutSec,
		TerminalIdleDisconnectSec: s.TerminalIdleDisconnectSec,
		AllowPrivateEndpoints:     s.AllowPrivateEndpoints,
		EnvVars:                   s.EnvVars,
		Network:                   s.Network,
		SkillRollout:              strings.TrimSpace(s.SkillRollout),
		Cube:                      s.Cube,
		E2B:                       s.E2B,
		Docker:                    s.Docker,
	}
}

// ParseSandboxSeedEnv reads and validates WEKNORA_SANDBOX_SEEDS. An unset or
// blank variable is not an error — it simply means "no seeding".
func ParseSandboxSeedEnv() ([]SandboxSeedSpec, error) {
	raw := strings.TrimSpace(os.Getenv(SandboxSeedsEnvVar))
	if raw == "" {
		return nil, nil
	}
	var specs []SandboxSeedSpec
	if err := json.Unmarshal([]byte(raw), &specs); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", SandboxSeedsEnvVar, err)
	}
	for i := range specs {
		if strings.TrimSpace(specs[i].Name) == "" {
			return nil, fmt.Errorf("%s: entry %d has an empty name", SandboxSeedsEnvVar, i)
		}
	}
	return specs, nil
}

// parseSandboxSeedTenantsEnv returns nil for "all"/unset (seed every
// workspace), otherwise the explicit id set.
func parseSandboxSeedTenantsEnv() (map[uint64]bool, error) {
	raw := strings.TrimSpace(os.Getenv(SandboxSeedTenantsEnvVar))
	if raw == "" || strings.EqualFold(raw, "all") {
		return nil, nil
	}
	scope := make(map[uint64]bool)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: bad workspace id %q", SandboxSeedTenantsEnvVar, part)
		}
		scope[id] = true
	}
	return scope, nil
}

// SandboxSeedAgentRepo is the minimal agent-repository surface the seed
// binding needs; satisfied by interfaces.CustomAgentRepository.
type SandboxSeedAgentRepo interface {
	GetAgentByID(ctx context.Context, id string, tenantID uint64) (*types.CustomAgent, error)
	ListAgentsByTenantID(ctx context.Context, tenantID uint64) ([]*types.CustomAgent, error)
	CreateAgent(ctx context.Context, agent *types.CustomAgent) error
	UpdateAgent(ctx context.Context, agent *types.CustomAgent) error
}

// ApplySandboxSeedFromEnv upserts every (tenant, spec) pair. It never
// returns an error: seeding is a startup convenience, not a boot gate.
func ApplySandboxSeedFromEnv(
	ctx context.Context,
	svc *TenantSandboxConfigService,
	tenantSvc interfaces.TenantService,
	agentRepo SandboxSeedAgentRepo,
) {
	specs, err := ParseSandboxSeedEnv()
	if err != nil {
		logger.Warnf(ctx, "[sandbox-seed] %v", err)
		return
	}
	if len(specs) == 0 {
		return
	}
	scope, err := parseSandboxSeedTenantsEnv()
	if err != nil {
		logger.Warnf(ctx, "[sandbox-seed] %v", err)
		return
	}
	tenants, err := tenantSvc.ListTenants(ctx)
	if err != nil {
		logger.Warnf(ctx, "[sandbox-seed] list tenants failed: %v", err)
		return
	}
	for _, t := range tenants {
		if len(scope) > 0 && !scope[t.ID] {
			continue
		}
		for _, spec := range specs {
			action, configID, err := svc.SeedConfig(ctx, t.ID, spec)
			switch {
			case err != nil:
				logger.Warnf(ctx,
					"[sandbox-seed] workspace %d: %s of config %q failed (will retry next restart): %v",
					t.ID, action, spec.Name, err)
				continue
			case action != seedActionNone:
				logger.Infof(ctx,
					"[sandbox-seed] workspace %d: %sd sandbox config %q",
					t.ID, action, spec.Name)
			}
			// Bind even on a noop card: the operator may have re-chosen the
			// agent's sandbox in the UI since the last boot, and the env is
			// the declared source of truth for listed agents.
			if configID != "" && len(spec.BindAgents) > 0 {
				bindSeedAgents(ctx, agentRepo, t.ID, spec.Name, configID, spec.BindAgents)
			}
		}
	}
}

// SeedConfig upserts one env-declared card for one workspace. Match key is
// the card name; the row id stays whatever the first create assigned. The
// returned configID is the row's id on every path (create, update and noop),
// which is what the agent binding needs to re-point at a stable id.
func (s *TenantSandboxConfigService) SeedConfig(
	ctx context.Context, tenantID uint64, spec SandboxSeedSpec,
) (string, string, error) {
	incoming := spec.toConfig()

	list, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return seedActionCreate, "", fmt.Errorf("list configs: %w", err)
	}
	var existing *types.TenantSandboxConfigEntity
	for _, e := range list {
		if types.IsSandboxWorkspacePolicyRow(e) {
			continue
		}
		if strings.TrimSpace(e.Name) == strings.TrimSpace(spec.Name) {
			existing = e
			break
		}
	}
	if existing != nil {
		inheritUnseededFields(incoming, existing.Config)
	}

	// A seed that omits template_id gets the same auto-provisioning the
	// wizard's template step performs. Failure is non-fatal: the create or
	// update below rejects the incomplete config and warns, and the next
	// restart retries once the cluster is reachable.
	if seedTemplateID(incoming) == "" {
		catalog, queryErr := s.QueryTemplates(ctx, tenantID, SandboxTemplateQueryInput{
			Config:         incoming,
			EnsureStandard: true,
		})
		if queryErr == nil && catalog != nil && catalog.StandardTemplateID != "" {
			setSeedTemplateID(incoming, catalog.StandardTemplateID)
		}
	}

	if existing == nil {
		created, err := s.Create(ctx, tenantID, CreateSandboxConfigInput{
			Name: spec.Name, Description: spec.Description, Config: incoming,
		})
		if err != nil {
			return seedActionCreate, "", err
		}
		return seedActionCreate, created.ID, nil
	}

	// Compare what the seed controls against what is stored, after the same
	// sanitization Update would apply — otherwise normalization differences
	// (nil vs empty slice) would trigger a rewrite on every restart.
	merged, err := SanitizeSandboxConfig(incoming, existing.Config)
	if err != nil {
		return seedActionUpdate, existing.ID, err
	}
	if seedViewOf(merged) == seedViewOf(existing.Config) {
		return seedActionNone, existing.ID, nil
	}
	if _, err := s.Update(ctx, tenantID, existing.ID, UpdateSandboxConfigInput{
		Name: spec.Name, Description: spec.Description, Config: incoming,
	}); err != nil {
		return seedActionUpdate, existing.ID, err
	}
	return seedActionUpdate, existing.ID, nil
}

// bindSeedAgents re-points the listed agents at configID. Per-agent
// failures only warn: a renamed or deleted agent must not stop the card
// itself from being seeded, and the next boot retries.
func bindSeedAgents(
	ctx context.Context,
	repo SandboxSeedAgentRepo,
	tenantID uint64,
	configName string,
	configID string,
	refs []string,
) {
	for _, raw := range refs {
		ref := strings.TrimSpace(raw)
		if ref == "" {
			continue
		}
		if err := bindOneAgent(ctx, repo, tenantID, configID, ref); err != nil {
			logger.Warnf(ctx,
				"[sandbox-seed] workspace %d: bind agent %q to config %q failed: %v",
				tenantID, ref, configName, err)
		} else {
			logger.Infof(ctx,
				"[sandbox-seed] workspace %d: agent %q runs sandbox config %q",
				tenantID, ref, configName)
		}
	}
}

func bindOneAgent(
	ctx context.Context,
	repo SandboxSeedAgentRepo,
	tenantID uint64,
	configID string,
	ref string,
) error {
	agent, err := repo.GetAgentByID(ctx, ref, tenantID)
	if err != nil || agent == nil {
		if types.IsBuiltinAgentID(ref) {
			// Built-in ids have no DB row until a user first saves them; a
			// lookup miss is expected here and materialization below is the
			// point. Any other error is real and must not be swallowed.
			if err != nil && !errors.Is(err, repository.ErrCustomAgentNotFound) {
				return err
			}
			agent = nil
		} else {
			// Not an id this workspace knows — try an exact display-name match,
			// which is how operators refer to their own custom agents.
			agent = nil
			if list, lerr := repo.ListAgentsByTenantID(ctx, tenantID); lerr == nil {
				for _, a := range list {
					if a != nil && strings.TrimSpace(a.Name) == ref {
						agent = a
						break
					}
				}
			}
			if agent == nil {
				return fmt.Errorf("agent %q not found in workspace %d", ref, tenantID)
			}
		}
	}

	if agent == nil {
		// A built-in id with no per-tenant row yet: materialize it from the
		// registry defaults with the binding set, exactly what the UI does on
		// the first save of a built-in agent's config.
		def := types.GetBuiltinAgent(ref, tenantID)
		if def == nil {
			return fmt.Errorf("unknown built-in agent id %q", ref)
		}
		def.Config.SandboxConfigID = configID
		return repo.CreateAgent(ctx, def)
	}

	// Config is a value struct on CustomAgent, so no nil guard exists — the
	// binding is the only field this path touches.
	if agent.Config.SandboxConfigID == configID {
		return nil // already bound; steady-state restart writes nothing
	}
	agent.Config.SandboxConfigID = configID
	return repo.UpdateAgent(ctx, agent)
}

// inheritUnseededFields carries stored values into the seed payload for
// fields an empty seed must not erase: credentials, injected env vars and
// an already-resolved template id.
func inheritUnseededFields(incoming, existing *types.TenantSandboxConfig) {
	if incoming == nil || existing == nil {
		return
	}
	if incoming.Cube != nil && existing.Cube != nil {
		if incoming.Cube.APIKey == "" {
			incoming.Cube.APIKey = existing.Cube.APIKey
		}
		if incoming.Cube.TemplateID == "" {
			incoming.Cube.TemplateID = existing.Cube.TemplateID
		}
	}
	if incoming.E2B != nil && existing.E2B != nil {
		if incoming.E2B.APIKey == "" {
			incoming.E2B.APIKey = existing.E2B.APIKey
		}
		if incoming.E2B.TemplateID == "" {
			incoming.E2B.TemplateID = existing.E2B.TemplateID
		}
	}
	if len(incoming.EnvVars) == 0 && len(existing.EnvVars) > 0 {
		incoming.EnvVars = existing.EnvVars
	}
}

// seedTemplateID reads the spawn template for providers that have one.
func seedTemplateID(c *types.TenantSandboxConfig) string {
	if c == nil {
		return ""
	}
	switch c.SandboxType {
	case string(sandbox.SandboxTypeCube):
		if c.Cube != nil {
			return strings.TrimSpace(c.Cube.TemplateID)
		}
	case string(sandbox.SandboxTypeE2B):
		if c.E2B != nil {
			return strings.TrimSpace(c.E2B.TemplateID)
		}
	}
	return ""
}

func setSeedTemplateID(c *types.TenantSandboxConfig, id string) {
	if c == nil {
		return
	}
	switch c.SandboxType {
	case string(sandbox.SandboxTypeCube):
		if c.Cube != nil {
			c.Cube.TemplateID = id
		}
	case string(sandbox.SandboxTypeE2B):
		if c.E2B != nil {
			c.E2B.TemplateID = id
		}
	}
}

// sandboxSeedView is the equality projection for the noop check: only
// fields a seed owns. SkillImage/VolumeMount pointers and the row's
// description are deliberately excluded so skill installs and unrelated
// stored state never look like a seed drift.
type sandboxSeedView struct {
	SandboxType               string                      `json:"sandbox_type"`
	AllowPrivateEndpoints     bool                        `json:"allow_private_endpoints"`
	DefaultTimeoutSec         int                         `json:"default_timeout_sec"`
	TerminalIdleDisconnectSec int                         `json:"terminal_idle_disconnect_sec"`
	SkillRollout              string                      `json:"skill_rollout"`
	Cube                      *types.CubeSandboxConfig    `json:"cube,omitempty"`
	E2B                       *types.E2BSandboxConfig     `json:"e2b,omitempty"`
	Docker                    *types.DockerSandboxConfig  `json:"docker,omitempty"`
	Network                   *types.SandboxNetworkPolicy `json:"network,omitempty"`
	EnvVars                   map[string]string           `json:"env_vars,omitempty"`
}

func seedViewOf(c *types.TenantSandboxConfig) string {
	if c == nil {
		return ""
	}
	view := sandboxSeedView{
		SandboxType:               c.SandboxType,
		AllowPrivateEndpoints:     c.AllowPrivateEndpoints,
		DefaultTimeoutSec:         c.DefaultTimeoutSec,
		TerminalIdleDisconnectSec: c.TerminalIdleDisconnectSec,
		SkillRollout:              strings.TrimSpace(c.SkillRollout),
		Cube:                      c.Cube,
		E2B:                       c.E2B,
		Docker:                    c.Docker,
		Network:                   c.Network,
		EnvVars:                   c.EnvVars,
	}
	// SkillImage is provider-owned state the seed never speaks for; drop the
	// pointer so a skill snapshot does not pin the view to "always differs".
	b, err := json.Marshal(view)
	if err != nil {
		// Structs of plain fields cannot fail marshalling; treat a surprise
		// as "different" so the loop falls through to a real Update error
		// rather than silently skipping a needed change.
		return "\x00marshal-error"
	}
	return string(b)
}

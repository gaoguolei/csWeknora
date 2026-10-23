// Seeding tests: env parsing plus the (tenant, name) upsert semantics that
// keep restarts no-ops and protect stored secrets a partial seed omits.
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
)

func seedCubeSpec(name string) SandboxSeedSpec {
	return SandboxSeedSpec{
		Name:        name,
		SandboxType: "cube",
		Cube: &types.CubeSandboxConfig{
			// Documentation-prefix IPs: the outbound URL guard resolves
			// hostnames for safety, and test environments have no DNS.
			APIURL:        "https://203.0.113.10:33000",
			ProxyURL:      "https://203.0.113.11:80",
			SandboxDomain: "cube.app",
			TemplateID:    "tpl-1",
		},
	}
}

func TestParseSandboxSeedEnv(t *testing.T) {
	t.Run("unset is not seeding", func(t *testing.T) {
		t.Setenv(SandboxSeedsEnvVar, "")
		specs, err := ParseSandboxSeedEnv()
		require.NoError(t, err)
		require.Empty(t, specs)
	})

	t.Run("parses the JSON array", func(t *testing.T) {
		t.Setenv(SandboxSeedsEnvVar,
			`[{"name":"生产Cube","sandbox_type":"cube","cube":{"api_url":"https://a","template_id":"t"}}]`)
		specs, err := ParseSandboxSeedEnv()
		require.NoError(t, err)
		require.Len(t, specs, 1)
		require.Equal(t, "生产Cube", specs[0].Name)
		require.Equal(t, "https://a", specs[0].Cube.APIURL)
	})

	t.Run("rejects malformed JSON and unnamed entries", func(t *testing.T) {
		t.Setenv(SandboxSeedsEnvVar, `{not json`)
		_, err := ParseSandboxSeedEnv()
		require.Error(t, err)

		t.Setenv(SandboxSeedsEnvVar, `[{"sandbox_type":"cube"}]`)
		_, err = ParseSandboxSeedEnv()
		require.ErrorContains(t, err, "empty name")
	})
}

func TestParseSandboxSeedTenantsEnv(t *testing.T) {
	t.Setenv(SandboxSeedTenantsEnvVar, "")
	scope, err := parseSandboxSeedTenantsEnv()
	require.NoError(t, err)
	require.Nil(t, scope, "empty means all tenants")

	t.Setenv(SandboxSeedTenantsEnvVar, "all")
	scope, err = parseSandboxSeedTenantsEnv()
	require.NoError(t, err)
	require.Nil(t, scope)

	t.Setenv(SandboxSeedTenantsEnvVar, "1, 7")
	scope, err = parseSandboxSeedTenantsEnv()
	require.NoError(t, err)
	require.Equal(t, map[uint64]bool{1: true, 7: true}, scope)

	t.Setenv(SandboxSeedTenantsEnvVar, "nope")
	_, err = parseSandboxSeedTenantsEnv()
	require.Error(t, err)
}

func TestSeedConfigCreatesMissingCard(t *testing.T) {
	repo := &fakeConfigRepo{}
	svc := newTestConfigService(t, repo, nil, stubAgentRepo{})

	action, configID, err := svc.SeedConfig(context.Background(), 7, seedCubeSpec("Seed Cube"))
	require.NoError(t, err)
	require.Equal(t, seedActionCreate, action)
	require.NotEmpty(t, configID)
	require.NotNil(t, repo.entity)
	require.Equal(t, "Seed Cube", repo.entity.Name)
	require.Equal(t, "cube", repo.entity.SandboxType)
	require.Equal(t, "tpl-1", repo.entity.Config.Cube.TemplateID)
}

// A seed that omits template_id gets the same standard-template resolution
// the wizard's template step performs.
func TestSeedConfigResolvesStandardTemplate(t *testing.T) {
	repo := &fakeConfigRepo{}
	client := &stubProviderClient{templates: []sandbox.RemoteTemplate{
		{ID: "tpl-std", Name: "weknora", Status: "ready", Standard: true},
	}}
	svc := newTestConfigService(t, repo, client, stubAgentRepo{})

	spec := seedCubeSpec("Seed Cube")
	spec.Cube.TemplateID = ""
	_, _, err := svc.SeedConfig(context.Background(), 7, spec)
	require.NoError(t, err)
	require.Equal(t, "tpl-std", repo.entity.Config.Cube.TemplateID)
}

func TestSeedConfigNoopWhenUnchanged(t *testing.T) {
	spec := seedCubeSpec("Seed Cube")
	stored, err := SanitizeSandboxConfig(spec.toConfig(), nil)
	require.NoError(t, err)
	repo := &fakeConfigRepo{entity: &types.TenantSandboxConfigEntity{
		ID: "cfg-a", TenantID: 7, Name: "Seed Cube",
		SandboxType: "cube", Config: stored,
	}}
	svc := newTestConfigService(t, repo, nil, stubAgentRepo{})

	action, configID, err := svc.SeedConfig(context.Background(), 7, spec)
	require.NoError(t, err)
	require.Equal(t, seedActionNone, action)
	require.Equal(t, "cfg-a", configID, "noop must still report the stable row id")
	require.NotContains(t, repo.events, "write")
}

func TestSeedConfigUpdatesChangedEndpoint(t *testing.T) {
	stored, err := SanitizeSandboxConfig(seedCubeSpec("Seed Cube").toConfig(), nil)
	require.NoError(t, err)
	repo := &fakeConfigRepo{entity: &types.TenantSandboxConfigEntity{
		ID: "cfg-a", TenantID: 7, Name: "Seed Cube",
		SandboxType: "cube", Config: stored,
	}}
	svc := newTestConfigService(t, repo, nil, stubAgentRepo{})

	spec := seedCubeSpec("Seed Cube")
	spec.Cube.APIURL = "https://203.0.113.20:33000"
	action, configID, err := svc.SeedConfig(context.Background(), 7, spec)
	require.NoError(t, err)
	require.Equal(t, seedActionUpdate, action)
	require.Equal(t, "cfg-a", configID)
	require.NotNil(t, repo.updated)
	require.Equal(t, "https://203.0.113.20:33000", repo.updated.Config.Cube.APIURL)
}

// The wizard's step-3 rollout choice rides along with the seed, and a seed
// that stays silent never resets a choice already stored from the UI.
func TestSeedConfigCarriesSkillRollout(t *testing.T) {
	repo := &fakeConfigRepo{}
	svc := newTestConfigService(t, repo, nil, stubAgentRepo{})

	spec := seedCubeSpec("Seed Cube")
	spec.SkillRollout = types.SkillRolloutNewSession
	_, _, err := svc.SeedConfig(context.Background(), 7, spec)
	require.NoError(t, err)
	require.Equal(t, types.SkillRolloutNewSession, repo.entity.Config.SkillRollout)

	// Same seed again is a no-op even though the row carries the rollout.
	action, _, err := svc.SeedConfig(context.Background(), 7, spec)
	require.NoError(t, err)
	require.Equal(t, seedActionNone, action)

	// A seed that omits skill_rollout keeps the stored choice (merge
	// semantics), so it still compares equal and does not rewrite.
	silent := seedCubeSpec("Seed Cube")
	action, _, err = svc.SeedConfig(context.Background(), 7, silent)
	require.NoError(t, err)
	require.Equal(t, seedActionNone, action)
	require.Equal(t, types.SkillRolloutNewSession, repo.entity.Config.SkillRollout)
}

// Empty fields in the seed are "unset", not "delete": a stored API key and
// an already-resolved template must survive a partial card untouched.
func TestSeedConfigInheritsStoredSecretsAndTemplate(t *testing.T) {
	// The stored API key makes SanitizeSandboxConfig require the at-rest
	// encryption key, exactly like the real deployment path.
	t.Setenv("SYSTEM_AES_KEY", strings.Repeat("k", 32))
	stored, err := SanitizeSandboxConfig(seedCubeSpec("Seed Cube").toConfig(), nil)
	require.NoError(t, err)
	stored.Cube.APIKey = "stored-secret"
	stored.Cube.TemplateID = "tpl-resolved"
	repo := &fakeConfigRepo{entity: &types.TenantSandboxConfigEntity{
		ID: "cfg-a", TenantID: 7, Name: "Seed Cube",
		SandboxType: "cube", Config: stored,
	}}
	svc := newTestConfigService(t, repo, nil, stubAgentRepo{})

	// The spec omits api_key and carries template_id "tpl-1"; only api_key
	// is inherited silently — the template change must still update.
	action, _, err := svc.SeedConfig(context.Background(), 7, seedCubeSpec("Seed Cube"))
	require.NoError(t, err)
	require.Equal(t, seedActionUpdate, action)
	require.Equal(t, "stored-secret", repo.updated.Config.Cube.APIKey)

	// A fully matching partial seed is a no-op and never rewrites. Reset the
	// row to the original stored state (the first update above wrote through
	// to the same entity, mimicking a real DB write) so this segment models a
	// fresh restart whose env already matches the persisted card.
	repo.events = nil
	repo.updated = nil
	repo.entity.Config = stored
	unchanged := seedCubeSpec("Seed Cube")
	unchanged.Cube.TemplateID = "tpl-resolved"
	action, _, err = svc.SeedConfig(context.Background(), 7, unchanged)
	require.NoError(t, err)
	require.Equal(t, seedActionNone, action)
	require.NotContains(t, repo.events, "write")
}

// fakeSeedAgentRepo backs the agent-binding tests: agents live in a map,
// and every write is recorded so idempotency can be asserted.
type fakeSeedAgentRepo struct {
	agents map[string]*types.CustomAgent
	events []string
}

func (f *fakeSeedAgentRepo) GetAgentByID(
	_ context.Context, id string, _ uint64,
) (*types.CustomAgent, error) {
	if a, ok := f.agents[id]; ok {
		return a, nil
	}
	return nil, repository.ErrCustomAgentNotFound
}

func (f *fakeSeedAgentRepo) ListAgentsByTenantID(
	context.Context, uint64,
) ([]*types.CustomAgent, error) {
	var out []*types.CustomAgent
	for _, a := range f.agents {
		out = append(out, a)
	}
	return out, nil
}

func (f *fakeSeedAgentRepo) CreateAgent(_ context.Context, a *types.CustomAgent) error {
	f.events = append(f.events, "create:"+a.ID)
	f.agents[a.ID] = a
	return nil
}

func (f *fakeSeedAgentRepo) UpdateAgent(_ context.Context, a *types.CustomAgent) error {
	f.events = append(f.events, "update:"+a.ID)
	f.agents[a.ID] = a
	return nil
}

func TestBindSeedAgents(t *testing.T) {
	repo := &fakeSeedAgentRepo{agents: map[string]*types.CustomAgent{
		"custom-1": {ID: "custom-1", Name: "测试助手", TenantID: 7,
			Config: types.CustomAgentConfig{}},
	}}
	// Stand in for a registry-loaded built-in agent; the YAML loader does
	// not run in unit tests, so seed the factory directly.
	types.BuiltinAgentRegistry["builtin-seedtest"] = func(tid uint64) *types.CustomAgent {
		return &types.CustomAgent{ID: "builtin-seedtest", Name: "Coding Assistant",
			TenantID: tid, IsBuiltin: true, Config: types.CustomAgentConfig{}}
	}
	defer delete(types.BuiltinAgentRegistry, "builtin-seedtest")

	ctx := context.Background()
	bindSeedAgents(ctx, repo, 7, "生产Cube", "cfg-a",
		[]string{"custom-1", "builtin-seedtest", "ghost-agent"})

	// Existing agent bound by id.
	require.Equal(t, "cfg-a", repo.agents["custom-1"].Config.SandboxConfigID)
	// Built-in without a row yet: materialized with the binding, like the UI save.
	require.Equal(t, "cfg-a", repo.agents["builtin-seedtest"].Config.SandboxConfigID)
	require.Contains(t, repo.events, "create:builtin-seedtest")
	// Unknown ref: skipped with a warning, no write, no panic. The name
	// lookup path is covered by binding "测试助手" below.
	require.NotContains(t, repo.events, "create:ghost-agent")

	// Rebinding the same cards writes nothing for already-bound agents.
	repo.events = nil
	bindSeedAgents(ctx, repo, 7, "生产Cube", "cfg-a", []string{"custom-1", "builtin-seedtest"})
	require.Empty(t, repo.events)

	// Name match finds a custom agent; a re-point writes an update.
	bindSeedAgents(ctx, repo, 7, "生产Cube", "cfg-b", []string{"测试助手"})
	require.Equal(t, "cfg-b", repo.agents["custom-1"].Config.SandboxConfigID)
	require.Contains(t, repo.events, "update:custom-1")
}

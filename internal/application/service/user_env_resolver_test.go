package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

const (
	testEnvTenantID uint64 = 7
	testEnvConfigID string = "cfg-1"
)

// searchSkillRow declares three names. Their inline `Value` is the retired
// workspace/admin value: under the global-only model the resolver must ignore it
// entirely and read from globalEnv instead.
func searchSkillRow() *types.TenantSkillEntity {
	return &types.TenantSkillEntity{
		ID: "sk-1", TenantID: testEnvTenantID, SandboxConfigID: testEnvConfigID,
		Name: "web-search",
		Envs: types.SkillEnvVars{
			{Name: "TAVILY_API_KEY", Required: true, Value: "workspace-key"},
			{Name: "SEARCH_REGION", Value: "workspace-region"},
			{Name: "OPTIONAL_TOKEN"},
		},
	}
}

func newTestResolver(globalEnv map[string]string, rows ...*types.TenantSkillEntity) *userEnvResolver {
	return NewUserEnvResolver(rows, nil, globalEnv)
}

// A named skill gets its declared names' global values; the workspace value
// stored on the declaration row is never used.
func TestResolverInjectsGlobalValuesForSkill(t *testing.T) {
	resolver := newTestResolver(map[string]string{
		"TAVILY_API_KEY": "global-key",
		"SEARCH_REGION":  "global-region",
	}, searchSkillRow())

	env, missing, err := resolver.ResolveEnv(context.Background(), "web-search")

	require.NoError(t, err)
	require.Empty(t, missing)
	require.Equal(t, "global-key", env["TAVILY_API_KEY"])
	require.Equal(t, "global-region", env["SEARCH_REGION"],
		"the value comes from the global store, not the row's workspace value")
	require.NotContains(t, env, "workspace-region")
}

// A global secret that this skill never declared is not handed to it.
func TestResolverDoesNotInjectUndeclaredGlobalsToSkill(t *testing.T) {
	resolver := newTestResolver(map[string]string{
		"TAVILY_API_KEY":   "global-key",
		"UNRELATED_SECRET": "nope",
	}, searchSkillRow())

	env, _, err := resolver.ResolveEnv(context.Background(), "web-search")

	require.NoError(t, err)
	require.Equal(t, "global-key", env["TAVILY_API_KEY"])
	require.NotContains(t, env, "UNRELATED_SECRET")
}

// With no skill named (shell_exec, config-wide), every global value is injected.
func TestResolverInjectsAllGlobalsWithoutASkill(t *testing.T) {
	resolver := newTestResolver(map[string]string{
		"HTTP_PROXY":     "http://proxy:8080",
		"TAVILY_API_KEY": "global-key",
	}, searchSkillRow())

	env, missing, err := resolver.ResolveEnv(context.Background(), "")

	require.NoError(t, err)
	require.Empty(t, missing, "no skill was named, so nothing can be missing")
	require.Equal(t, map[string]string{
		"HTTP_PROXY":     "http://proxy:8080",
		"TAVILY_API_KEY": "global-key",
	}, env)
}

// An unknown skill name behaves like the no-name case: the global config-wide
// set is injected, since a host-staged skill carries no declaration to filter on.
func TestResolverUnknownSkillGetsAllGlobals(t *testing.T) {
	resolver := newTestResolver(map[string]string{"HTTP_PROXY": "http://proxy:8080"}, searchSkillRow())

	env, missing, err := resolver.ResolveEnv(context.Background(), "preloaded-thing")

	require.NoError(t, err)
	require.Empty(t, missing)
	require.Equal(t, map[string]string{"HTTP_PROXY": "http://proxy:8080"}, env)
}

// The resolver is built even for a run with no installed skills, so shell_exec
// still receives the global values.
func TestResolverWorksWithNoInstalledSkills(t *testing.T) {
	resolver := NewUserEnvResolver(nil, nil, map[string]string{"HTTP_PROXY": "http://proxy:8080"})

	env, _, err := resolver.ResolveEnv(context.Background(), "")

	require.NoError(t, err)
	require.Equal(t, "http://proxy:8080", env["HTTP_PROXY"])
}

// A required declaration with no global value is reported missing; nothing is
// injected for an unset name.
func TestResolverReportsMissingRequiredWhenGlobalAbsent(t *testing.T) {
	row := searchSkillRow()
	row.Envs = types.SkillEnvVars{
		{Name: "TAVILY_API_KEY", Required: true},
		{Name: "OPTIONAL_TOKEN"},
	}
	resolver := newTestResolver(map[string]string{}, row)

	env, missing, err := resolver.ResolveEnv(context.Background(), "web-search")

	require.NoError(t, err)
	require.Equal(t, []string{"TAVILY_API_KEY"}, missing)
	require.Empty(t, env)
}

// An empty global value is not a value: injecting it would make an unconfigured
// variable indistinguishable from one deliberately set to "".
func TestResolverTreatsEmptyGlobalValueAsUnset(t *testing.T) {
	row := searchSkillRow()
	row.Envs = types.SkillEnvVars{{Name: "TAVILY_API_KEY", Required: true}}
	resolver := newTestResolver(map[string]string{"TAVILY_API_KEY": ""}, row)

	env, missing, err := resolver.ResolveEnv(context.Background(), "web-search")

	require.NoError(t, err)
	require.NotContains(t, env, "TAVILY_API_KEY")
	require.Equal(t, []string{"TAVILY_API_KEY"}, missing)
}

// A required key already baked into the container at creation satisfies the gate
// without being copied into the per-exec map.
func TestResolverSandboxConfigSatisfiesRequiredWithoutInjecting(t *testing.T) {
	row := searchSkillRow()
	row.Envs = types.SkillEnvVars{{Name: "TAVILY_API_KEY", Required: true}}
	resolver := NewUserEnvResolver(
		[]*types.TenantSkillEntity{row},
		map[string]string{"TAVILY_API_KEY": "sandbox-key"},
		nil,
	)

	env, missing, err := resolver.ResolveEnv(context.Background(), "web-search")

	require.NoError(t, err)
	require.Empty(t, missing)
	require.NotContains(t, env, "TAVILY_API_KEY")
}

// An empty container value does not satisfy a required declaration.
func TestResolverEmptySandboxConfigValueDoesNotSatisfyRequired(t *testing.T) {
	row := searchSkillRow()
	row.Envs = types.SkillEnvVars{{Name: "TAVILY_API_KEY", Required: true}}
	resolver := NewUserEnvResolver(
		[]*types.TenantSkillEntity{row},
		map[string]string{"TAVILY_API_KEY": ""},
		nil,
	)

	_, missing, err := resolver.ResolveEnv(context.Background(), "web-search")

	require.NoError(t, err)
	require.Equal(t, []string{"TAVILY_API_KEY"}, missing)
}

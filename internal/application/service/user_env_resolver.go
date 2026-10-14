package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// userEnvResolver answers skills.SkillEnvResolver from the skill rows this turn
// already loaded plus the platform-wide global secrets the super admin set.
//
// Long-term memory of the per-workspace admin values and per-member overrides
// was retired in favour of a single global store: the resolver now injects only
// the global values, so every workspace and every user runs with the same
// super-admin-configured credentials.
type userEnvResolver struct {
	// byName indexes the rows by the name the model addresses skills with.
	byName map[string]*types.TenantSkillEntity
	// containerEnv is the sandbox config's env_vars, already loaded by the
	// caller. They were baked into the container at creation and only
	// participate in the required check — they are not copied into the
	// per-exec map.
	containerEnv map[string]string
	// globalEnv is the decrypted platform-wide name -> value map from
	// sandbox.global_env_vars. It is the sole source of injected values.
	globalEnv map[string]string
}

// NewUserEnvResolver builds the resolver for one agent run. rows are this run's
// installed skills, already carrying their declarations; they may be empty, in
// which case only the global config-wide variables are resolved.
func NewUserEnvResolver(
	rows []*types.TenantSkillEntity,
	containerEnv map[string]string,
	globalEnv map[string]string,
) *userEnvResolver {
	byName := make(map[string]*types.TenantSkillEntity, len(rows))
	for _, row := range rows {
		if row != nil {
			byName[row.Name] = row
		}
	}
	return &userEnvResolver{
		byName:       byName,
		containerEnv: containerEnv,
		globalEnv:    globalEnv,
	}
}

// ResolveEnv injects the global values. When a skill is named, only the names it
// declared are taken from the global store, so an unrelated secret is not handed
// to a skill that never asked for it. When no skill is named (shell_exec, or an
// unknown name whose host-staged skill carries no declaration), every global
// value is injected as the config-wide analog.
//
// Sandbox-config env_vars (baked into the container at creation) are not
// injected — they were set before the sandbox existed and only satisfy a required
// declaration so a key filled in earlier is not reported as unset.
func (r *userEnvResolver) ResolveEnv(
	_ context.Context, skillName string,
) (map[string]string, []string, error) {
	env := map[string]string{}
	row := r.byName[skillName]
	if row != nil {
		for _, declared := range row.Envs {
			// An empty global value means unset; injecting it would make an
			// unconfigured variable indistinguishable from one set to "".
			if value := r.globalEnv[declared.Name]; value != "" {
				env[declared.Name] = value
			}
		}
	} else {
		for name, value := range r.globalEnv {
			if value != "" {
				env[name] = value
			}
		}
	}

	var missing []string
	if row != nil {
		for _, declared := range row.Envs {
			if declared.Required && env[declared.Name] == "" && r.containerEnv[declared.Name] == "" {
				missing = append(missing, declared.Name)
			}
		}
	}
	return env, missing, nil
}

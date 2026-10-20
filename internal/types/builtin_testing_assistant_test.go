package types

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuiltinTestingAssistantConfigAndPrompt(t *testing.T) {
	data, err := os.ReadFile("../../config/builtin_agents.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var agents builtinAgentsFile
	if err := yaml.Unmarshal(data, &agents); err != nil {
		t.Fatal(err)
	}
	var entry *BuiltinAgentEntry
	for i := range agents.BuiltinAgents {
		if agents.BuiltinAgents[i].ID == BuiltinTestingAssistantID {
			if entry != nil {
				t.Fatal("duplicate testing assistant definition")
			}
			entry = &agents.BuiltinAgents[i]
		}
	}
	if entry == nil || !entry.IsBuiltin || !slices.Contains(GetBuiltinAgentIDs(), BuiltinTestingAssistantID) {
		t.Fatal("testing assistant must be defined and visible in the built-in agent list")
	}
	if entry.Config.AgentMode != AgentModeSmartReasoning || entry.Config.SystemPromptID != "testing_assistant" {
		t.Fatalf("unexpected testing assistant configuration: %+v", entry.Config)
	}
	if entry.Config.KBSelectionMode != "none" {
		t.Fatal("testing assistant must not use knowledge bases by default")
	}
	if entry.Config.SkillsSelectionMode != "all" || entry.Config.MCPSelectionMode != "all" {
		t.Fatal("testing assistant must support configured skills and MCP services")
	}
	restore := OverrideBuiltinAgentEntriesForTest(map[string]*BuiltinAgentEntry{entry.ID: entry})
	t.Cleanup(restore)
	for locale, name := range map[string]string{"zh-CN": "测试助手", "en-US": "Testing Assistant"} {
		ctx := context.WithValue(context.Background(), LanguageContextKey, locale)
		agent := GetBuiltinAgentWithContext(ctx, BuiltinTestingAssistantID, 42)
		if agent == nil || agent.Name != name || agent.TenantID != 42 {
			t.Fatalf("locale %s: unexpected testing assistant: %+v", locale, agent)
		}
	}
	data, err = os.ReadFile("../../config/prompt_templates/agent_system_prompt.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var prompts struct {
		Templates []struct {
			ID      string `yaml:"id"`
			Mode    string `yaml:"mode"`
			Content string `yaml:"content"`
		} `yaml:"templates"`
	}
	if err := yaml.Unmarshal(data, &prompts); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, prompt := range prompts.Templates {
		if prompt.ID == entry.Config.SystemPromptID {
			count++
			if prompt.Mode != AgentModeSmartReasoning || !strings.Contains(prompt.Content, "workspace_tests") {
				t.Fatal("testing prompt must support the agent pipeline and real project validation")
			}
			for _, required := range []string{"/workspace/output/test-cases/", "/workspace/output/test-reports/", "Excel (.xlsx)", "actual results"} {
				if !strings.Contains(prompt.Content, required) {
					t.Fatalf("testing prompt must define document exports and execution reports: missing %q", required)
				}
			}
		}
	}
	if count != 1 {
		t.Fatalf("testing assistant prompt resolves to %d templates, want 1", count)
	}
}

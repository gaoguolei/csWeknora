package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgentDefaultsConfig(t *testing.T) {
	// Empty / whitespace-only means "no platform defaults configured".
	assert.Nil(t, ParseAgentDefaultsConfig(""))
	assert.Nil(t, ParseAgentDefaultsConfig("   \n "))

	// Malformed JSON is treated the same way so callers never crash on a bad row.
	assert.Nil(t, ParseAgentDefaultsConfig("{not-json"))

	cfg := ParseAgentDefaultsConfig(`{"model_id":"llm-1","history_turns":8,"web_search_enabled":true}`)
	require.NotNil(t, cfg)
	assert.Equal(t, "llm-1", cfg.ModelID)
	assert.Equal(t, 8, cfg.HistoryTurns)
	require.NotNil(t, cfg.WebSearchEnabled)
	assert.True(t, *cfg.WebSearchEnabled)
}

func TestAgentDefaultsApplyToNilSafe(t *testing.T) {
	// A nil config or nil target must be a no-op rather than a panic.
	var nilCfg *AgentDefaultsConfig
	nilCfg.ApplyTo(&CustomAgentConfig{})

	cfg := &AgentDefaultsConfig{ModelID: "llm-1"}
	cfg.ApplyTo(nil)

	// A nil parsed config (no platform defaults) is also a no-op.
	ParseAgentDefaultsConfig("").ApplyTo(&CustomAgentConfig{ModelID: "untouched"})
}

func TestAgentDefaultsApplyToFillsEmptyFields(t *testing.T) {
	on := true
	cfg := &AgentDefaultsConfig{
		ModelID:            "llm-1",
		RerankModelID:      "rerank-1",
		HistoryTurns:       8,
		EmbeddingTopK:      20,
		VectorThreshold:    0.7,
		WebSearchEnabled:   &on,
		ImageUploadEnabled: &on,
		AllowedTools:       []string{"search_knowledge"},
		MCPSelectionMode:   "selected",
		MCPServices:        []string{"mcp-1"},
		SandboxConfigID:    "sandbox-1",
		SelectedSkills:     []string{"skill-1"},
		VLMModelID:         "vllm-1",
	}
	target := &CustomAgentConfig{}
	cfg.ApplyTo(target)

	assert.Equal(t, "llm-1", target.ModelID)
	assert.Equal(t, "rerank-1", target.RerankModelID)
	assert.Equal(t, 8, target.HistoryTurns)
	assert.Equal(t, 20, target.EmbeddingTopK)
	assert.InDelta(t, 0.7, target.VectorThreshold, 1e-9)
	assert.True(t, target.WebSearchEnabled)
	assert.True(t, target.ImageUploadEnabled)
	assert.Equal(t, []string{"search_knowledge"}, target.AllowedTools)
	assert.Equal(t, "selected", target.MCPSelectionMode)
	assert.Equal(t, []string{"mcp-1"}, target.MCPServices)
	assert.Equal(t, "sandbox-1", target.SandboxConfigID)
	assert.Equal(t, []string{"skill-1"}, target.SelectedSkills)
	assert.Equal(t, "vllm-1", target.VLMModelID)
}

// The user's own choices must survive: ApplyTo only fills zero values, so an
// operator who did configure a section keeps that section.
func TestAgentDefaultsApplyToKeepsExplicitValues(t *testing.T) {
	on := true
	off := false
	cfg := &AgentDefaultsConfig{
		ModelID:          "platform-llm",
		HistoryTurns:     8,
		WebSearchEnabled: &on,
		AllowedTools:     []string{"platform-tool"},
		MCPServices:      []string{"platform-mcp"},
		// Pointer-typed target field: the default must not flip an explicit false.
		CitationEnabled: &on,
	}
	target := &CustomAgentConfig{
		ModelID:          "user-llm",
		HistoryTurns:     3,
		WebSearchEnabled: false, // explicitly off — must not be flipped on
		AllowedTools:     []string{"user-tool"},
		MCPServices:      []string{"user-mcp"},
		CitationEnabled:  &off,
	}
	cfg.ApplyTo(target)

	assert.Equal(t, "user-llm", target.ModelID)
	assert.Equal(t, 3, target.HistoryTurns)
	assert.False(t, target.WebSearchEnabled)
	assert.Equal(t, []string{"user-tool"}, target.AllowedTools)
	assert.Equal(t, []string{"user-mcp"}, target.MCPServices)
	require.NotNil(t, target.CitationEnabled)
	assert.False(t, *target.CitationEnabled)
}

// Pointer-typed fields on the target (thinking / citation) are filled only when
// the caller left them nil; a platform default must never invent a pointer.
func TestAgentDefaultsApplyToPointerFields(t *testing.T) {
	on := true
	cfg := &AgentDefaultsConfig{CitationEnabled: &on, Thinking: &on}

	target := &CustomAgentConfig{}
	cfg.ApplyTo(target)
	require.NotNil(t, target.CitationEnabled)
	assert.True(t, *target.CitationEnabled)
	require.NotNil(t, target.Thinking)
	assert.True(t, *target.Thinking)

	// Defaults that leave the booleans unset must not create pointers.
	none := &CustomAgentConfig{}
	(&AgentDefaultsConfig{}).ApplyTo(none)
	assert.Nil(t, none.CitationEnabled)
	assert.Nil(t, none.Thinking)
	assert.False(t, none.WebSearchEnabled)
}

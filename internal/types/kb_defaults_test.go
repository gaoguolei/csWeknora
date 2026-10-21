package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKBDefaultsConfig(t *testing.T) {
	// Empty / whitespace-only means "no platform defaults configured".
	assert.Nil(t, ParseKBDefaultsConfig(""))
	assert.Nil(t, ParseKBDefaultsConfig("   \n "))

	// Malformed JSON is treated the same way so callers never crash on a bad row.
	assert.Nil(t, ParseKBDefaultsConfig("{not-json"))

	cfg := ParseKBDefaultsConfig(`{"summary_model_id":"llm-1","embedding_model_id":"emb-1"}`)
	require.NotNil(t, cfg)
	assert.Equal(t, "llm-1", cfg.SummaryModelID)
	assert.Equal(t, "emb-1", cfg.EmbeddingModelID)
}

func TestKBDefaultsApplyToNilSafe(t *testing.T) {
	// A nil config or nil KB must be a no-op rather than a panic.
	var nilCfg *KBDefaultsConfig
	nilCfg.ApplyTo(&KnowledgeBase{Type: KnowledgeBaseTypeDocument})

	cfg := &KBDefaultsConfig{SummaryModelID: "llm-1"}
	cfg.ApplyTo(nil)
}

func TestKBDefaultsApplyToFillsEmptyFields(t *testing.T) {
	cfg := &KBDefaultsConfig{
		SummaryModelID:           "llm-1",
		EmbeddingModelID:         "emb-1",
		ChunkingConfig:           &ChunkingConfig{ChunkSize: 1000, ChunkOverlap: 150},
		VLMConfig:                &VLMConfig{Enabled: true, ModelID: "vllm-1"},
		ASRConfig:                &ASRConfig{Enabled: true, ModelID: "asr-1"},
		ExtractConfig:            &ExtractConfig{Enabled: true},
		QuestionGenerationConfig: &QuestionGenerationConfig{Enabled: true, QuestionCount: 5},
		AutoTagConfig:            &AutoTagConfig{Enabled: true},
		ProfileConfig:            &KnowledgeBaseProfileConfig{Enabled: true},
	}

	kb := &KnowledgeBase{Type: KnowledgeBaseTypeDocument}
	cfg.ApplyTo(kb)

	assert.Equal(t, "llm-1", kb.SummaryModelID)
	assert.Equal(t, "emb-1", kb.EmbeddingModelID)
	assert.Equal(t, 1000, kb.ChunkingConfig.ChunkSize)
	assert.Equal(t, 150, kb.ChunkingConfig.ChunkOverlap)
	assert.True(t, kb.VLMConfig.Enabled)
	assert.Equal(t, "vllm-1", kb.VLMConfig.ModelID)
	assert.True(t, kb.ASRConfig.Enabled)
	assert.NotNil(t, kb.ExtractConfig)
	assert.True(t, kb.ExtractConfig.Enabled)
	assert.NotNil(t, kb.QuestionGenerationConfig)
	assert.Equal(t, 5, kb.QuestionGenerationConfig.QuestionCount)
	assert.NotNil(t, kb.AutoTagConfig)
	assert.NotNil(t, kb.ProfileConfig)
}

func TestKBDefaultsApplyToPreservesExplicitValues(t *testing.T) {
	// An API/MCP client that still sends an explicit config keeps its own
	// values; the platform default only fills what was left unset.
	cfg := &KBDefaultsConfig{
		SummaryModelID: "llm-platform",
		ChunkingConfig: &ChunkingConfig{ChunkSize: 1000},
	}

	kb := &KnowledgeBase{
		Type:             KnowledgeBaseTypeDocument,
		SummaryModelID:   "llm-caller",
		EmbeddingModelID: "emb-caller",
		ChunkingConfig:   ChunkingConfig{ChunkSize: 300, ChunkOverlap: 40},
	}
	cfg.ApplyTo(kb)

	assert.Equal(t, "llm-caller", kb.SummaryModelID, "explicit summary model must not be overwritten")
	assert.Equal(t, "emb-caller", kb.EmbeddingModelID)
	assert.Equal(t, 300, kb.ChunkingConfig.ChunkSize, "explicit chunking must not be overwritten")
	assert.Equal(t, 40, kb.ChunkingConfig.ChunkOverlap)
}

func TestKBDefaultsApplyToFAQOnlyInheritsModels(t *testing.T) {
	cfg := &KBDefaultsConfig{
		SummaryModelID:   "llm-1",
		EmbeddingModelID: "emb-1",
		ChunkingConfig:   &ChunkingConfig{ChunkSize: 1000},
		ExtractConfig:    &ExtractConfig{Enabled: true},
		VLMConfig:        &VLMConfig{Enabled: true, ModelID: "vllm-1"},
	}

	kb := &KnowledgeBase{Type: KnowledgeBaseTypeFAQ}
	cfg.ApplyTo(kb)

	// FAQ inherits the model defaults only.
	assert.Equal(t, "llm-1", kb.SummaryModelID)
	assert.Equal(t, "emb-1", kb.EmbeddingModelID)
	// Chunking / graph / image must stay untouched for FAQ knowledge bases.
	assert.Equal(t, 0, kb.ChunkingConfig.ChunkSize)
	assert.Nil(t, kb.ExtractConfig)
	assert.False(t, kb.VLMConfig.Enabled)
}

func TestKBDefaultsChunkingBuiltInFallback(t *testing.T) {
	// When neither the request nor the platform supplies a chunking config, the
	// KB still ends up with a valid (non-zero) built-in preset.
	cfg := &KBDefaultsConfig{SummaryModelID: "llm-1"}
	kb := &KnowledgeBase{Type: KnowledgeBaseTypeDocument}
	cfg.ApplyTo(kb)

	assert.Equal(t, BuiltInKBChunking().ChunkSize, kb.ChunkingConfig.ChunkSize)
	assert.NotZero(t, kb.ChunkingConfig.ChunkSize)
}

func TestKBDefaultsApplyToVectorStoreAndStorage(t *testing.T) {
	vsID := "vs-platform"
	stID := "sb-platform"
	cfg := &KBDefaultsConfig{
		VectorStoreID:         &vsID,
		StorageBackendID:      &stID,
		StorageProviderConfig: &StorageProviderConfig{Provider: "minio"},
	}

	// Unset on the caller -> platform default is applied.
	kb := &KnowledgeBase{Type: KnowledgeBaseTypeDocument}
	cfg.ApplyTo(kb)
	require.NotNil(t, kb.VectorStoreID)
	assert.Equal(t, "vs-platform", *kb.VectorStoreID)
	require.NotNil(t, kb.StorageBackendID)
	assert.Equal(t, "sb-platform", *kb.StorageBackendID)
	require.NotNil(t, kb.StorageProviderConfig)
	assert.Equal(t, "minio", kb.StorageProviderConfig.Provider)

	// Explicit caller values win.
	own := "vs-caller"
	kb2 := &KnowledgeBase{Type: KnowledgeBaseTypeDocument, VectorStoreID: &own}
	cfg.ApplyTo(kb2)
	assert.Equal(t, "vs-caller", *kb2.VectorStoreID)
}

func TestKBDefaultsApplyToFAQSkipsVectorStoreAndStorage(t *testing.T) {
	vsID := "vs-platform"
	cfg := &KBDefaultsConfig{SummaryModelID: "llm-1", VectorStoreID: &vsID}
	kb := &KnowledgeBase{Type: KnowledgeBaseTypeFAQ}
	cfg.ApplyTo(kb)
	// FAQ stops after models, so the vector-store default is not applied.
	assert.Equal(t, "llm-1", kb.SummaryModelID)
	assert.Nil(t, kb.VectorStoreID)
}

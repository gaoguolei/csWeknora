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
		ExtractConfig: &ExtractConfig{
			Enabled:   true,
			Text:      "extract entities and relations",
			Tags:      []string{"person"},
			Nodes:     []*GraphNode{{Name: "person"}},
			Relations: []*GraphRelation{{Node1: "person", Node2: "org", Type: "works_at"}},
		},
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

func TestKBDefaultsSkipIncompleteExtractConfig(t *testing.T) {
	// The platform settings page autosaves as soon as the graph toggle flips, so
	// a stored default can be enabled with no content yet. Copying that
	// half-finished edit into a new KB would pass creation (defaults are applied
	// after validation) and then fail every later save with "text cannot be
	// empty", so it is treated as unconfigured.
	cfg := &KBDefaultsConfig{ExtractConfig: &ExtractConfig{Enabled: true}}
	kb := &KnowledgeBase{Type: KnowledgeBaseTypeDocument}
	cfg.ApplyTo(kb)
	assert.Nil(t, kb.ExtractConfig, "incomplete enabled graph config must not be inherited")

	// A disabled default is a complete instruction and still applies.
	disabled := &KBDefaultsConfig{ExtractConfig: &ExtractConfig{Enabled: false}}
	kb2 := &KnowledgeBase{Type: KnowledgeBaseTypeDocument}
	disabled.ApplyTo(kb2)
	require.NotNil(t, kb2.ExtractConfig)
	assert.False(t, kb2.ExtractConfig.Enabled)
}

func TestExtractConfigIsComplete(t *testing.T) {
	var nilCfg *ExtractConfig
	assert.False(t, nilCfg.IsComplete())
	assert.False(t, (&ExtractConfig{Enabled: true}).IsComplete())
	assert.False(t, (&ExtractConfig{
		Enabled:   true,
		Text:      "extract",
		Tags:      []string{"person"},
		Nodes:     []*GraphNode{{Name: "person"}},
		Relations: nil,
	}).IsComplete())
	assert.True(t, (&ExtractConfig{
		Enabled:   true,
		Text:      "extract",
		Tags:      []string{"person"},
		Nodes:     []*GraphNode{{Name: "person"}},
		Relations: []*GraphRelation{{Node1: "person", Node2: "org", Type: "works_at"}},
	}).IsComplete())
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

func TestKBDefaultsApplyToFAQOnlyInheritsModelsAndFAQConfig(t *testing.T) {
	cfg := &KBDefaultsConfig{
		SummaryModelID:   "llm-1",
		EmbeddingModelID: "emb-1",
		ChunkingConfig:   &ChunkingConfig{ChunkSize: 1000},
		ExtractConfig:    &ExtractConfig{Enabled: true},
		VLMConfig:        &VLMConfig{Enabled: true, ModelID: "vllm-1"},
		FAQConfig: &FAQConfig{
			IndexMode:         FAQIndexModeQuestionAnswer,
			QuestionIndexMode: FAQQuestionIndexModeCombined,
		},
	}

	kb := &KnowledgeBase{Type: KnowledgeBaseTypeFAQ}
	cfg.ApplyTo(kb)

	// FAQ inherits the model defaults and the FAQ index-mode defaults.
	assert.Equal(t, "llm-1", kb.SummaryModelID)
	assert.Equal(t, "emb-1", kb.EmbeddingModelID)
	require.NotNil(t, kb.FAQConfig)
	assert.Equal(t, FAQIndexModeQuestionAnswer, kb.FAQConfig.IndexMode)
	assert.Equal(t, FAQQuestionIndexModeCombined, kb.FAQConfig.QuestionIndexMode)
	// Chunking / graph / image must stay untouched for FAQ knowledge bases.
	assert.Equal(t, 0, kb.ChunkingConfig.ChunkSize)
	assert.Nil(t, kb.ExtractConfig)
	assert.False(t, kb.VLMConfig.Enabled)
}

func TestKBDefaultsFAQConfigPreservesExplicitValues(t *testing.T) {
	cfg := &KBDefaultsConfig{
		FAQConfig: &FAQConfig{
			IndexMode:         FAQIndexModeQuestionAnswer,
			QuestionIndexMode: FAQQuestionIndexModeCombined,
		},
	}

	// An explicit request keeps its own values.
	kb := &KnowledgeBase{
		Type: KnowledgeBaseTypeFAQ,
		FAQConfig: &FAQConfig{
			IndexMode:         FAQIndexModeQuestionOnly,
			QuestionIndexMode: FAQQuestionIndexModeSeparate,
		},
	}
	cfg.ApplyTo(kb)
	assert.Equal(t, FAQIndexModeQuestionOnly, kb.FAQConfig.IndexMode)
	assert.Equal(t, FAQQuestionIndexModeSeparate, kb.FAQConfig.QuestionIndexMode)

	// Only the fields left empty are filled.
	partial := &KnowledgeBase{Type: KnowledgeBaseTypeFAQ, FAQConfig: &FAQConfig{}}
	cfg.ApplyTo(partial)
	assert.Equal(t, FAQIndexModeQuestionAnswer, partial.FAQConfig.IndexMode)
	assert.Equal(t, FAQQuestionIndexModeCombined, partial.FAQConfig.QuestionIndexMode)

	// Document KBs never carry FAQ config.
	doc := &KnowledgeBase{Type: KnowledgeBaseTypeDocument}
	cfg.ApplyTo(doc)
	assert.Nil(t, doc.FAQConfig)
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

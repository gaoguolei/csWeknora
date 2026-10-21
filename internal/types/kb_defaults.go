package types

import (
	"encoding/json"
	"strings"
)

// KBDefaultsKey is the system_settings key holding the platform-wide default
// knowledge-base configuration managed by the platform super administrator
// under Settings -> 知识库设置 (kb-defaults). The stored value is a JSON
// document decoded into KBDefaultsConfig; an empty string means "no platform
// defaults configured" and every KB falls back to its own request values /
// built-in defaults.
const KBDefaultsKey = "knowledge_base.default_config"

// KBDefaultsConfig captures the knowledge-base settings that have been
// reclaimed from the per-KB editor to a single platform-level configuration:
//
//	B 模型配置    -> SummaryModelID / EmbeddingModelID
//	E 分块设置    -> ChunkingConfig
//	F 图像处理    -> VLMConfig / ImageProcessingConfig
//	G 音频处理    -> ASRConfig
//	H 知识图谱    -> ExtractConfig
//	I 高级设置    -> QuestionGenerationConfig / AutoTagConfig / ProfileConfig
//
// Vector store (C), parser engine (D) and storage engine (J) are NOT here —
// they are operations-admin system-level concerns already provisioned on their
// own settings pages and resolved by the existing tenant/env defaulting paths.
//
// Every field is optional. A nil / zero field leaves the corresponding KB
// value untouched so a partial platform default only overrides what the
// operator actually configured.
type KBDefaultsConfig struct {
	// B: default chat (summary) and embedding model IDs shared by every new KB.
	SummaryModelID   string `json:"summary_model_id,omitempty"`
	EmbeddingModelID string `json:"embedding_model_id,omitempty"`

	// E: chunking / splitting defaults.
	ChunkingConfig *ChunkingConfig `json:"chunking_config,omitempty"`

	// F: image (multimodal) defaults.
	VLMConfig             *VLMConfig             `json:"vlm_config,omitempty"`
	ImageProcessingConfig *ImageProcessingConfig `json:"image_processing_config,omitempty"`

	// G: audio (ASR) defaults.
	ASRConfig *ASRConfig `json:"asr_config,omitempty"`

	// H: knowledge-graph extraction defaults.
	ExtractConfig *ExtractConfig `json:"extract_config,omitempty"`

	// I: advanced defaults (question generation, auto-tag, KB profile).
	QuestionGenerationConfig *QuestionGenerationConfig   `json:"question_generation_config,omitempty"`
	AutoTagConfig            *AutoTagConfig              `json:"auto_tag_config,omitempty"`
	ProfileConfig            *KnowledgeBaseProfileConfig `json:"profile_config,omitempty"`

	// C/J: operations-admin platform defaults for the vector-store binding and
	// the storage backend/provider selection. Applied only when the create
	// request left them unset. Parser engine rules (D) ride inside
	// ChunkingConfig above, so they need no separate field.
	VectorStoreID         *string                `json:"vector_store_id,omitempty"`
	StorageBackendID      *string                `json:"storage_backend_id,omitempty"`
	StorageProviderConfig *StorageProviderConfig `json:"storage_provider_config,omitempty"`
}

// BuiltInKBChunking returns the built-in chunking preset used as the last-resort
// fallback when neither the create request nor the platform default supplies a
// chunking configuration. It mirrors the frontend DEFAULT_CHUNKING_PRESET and
// chunker.DefaultChunkSize / DefaultChunkOverlap on the backend.
func BuiltInKBChunking() *ChunkingConfig {
	return &ChunkingConfig{
		ChunkSize:         512,
		ChunkOverlap:      80,
		Separators:        []string{"\n\n", "\n", "。", "！", "？", ";", "；"},
		EnableParentChild: true,
		ParentChunkSize:   4096,
		ChildChunkSize:    384,
	}
}

// ParseKBDefaultsConfig decodes a raw system_settings JSON string into a
// KBDefaultsConfig. An empty / whitespace-only value or malformed JSON returns
// nil so callers can treat "no defaults" uniformly.
func ParseKBDefaultsConfig(raw string) *KBDefaultsConfig {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	cfg := &KBDefaultsConfig{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return nil
	}
	return cfg
}

// ApplyTo fills unset configuration on a freshly-created knowledge base from
// the platform defaults. It only writes fields the caller left empty, so an
// API/MCP client that still sends an explicit config keeps its values, and the
// platform default only acts as the source of truth for the fields the UI no
// longer exposes.
//
// FAQ knowledge bases only inherit the model defaults (they have no
// chunking / graph / image pipeline).
func (d *KBDefaultsConfig) ApplyTo(kb *KnowledgeBase) {
	if d == nil || kb == nil {
		return
	}

	// B: model defaults apply to every KB type.
	if strings.TrimSpace(kb.SummaryModelID) == "" && d.SummaryModelID != "" {
		kb.SummaryModelID = d.SummaryModelID
	}
	if strings.TrimSpace(kb.EmbeddingModelID) == "" && d.EmbeddingModelID != "" {
		kb.EmbeddingModelID = d.EmbeddingModelID
	}

	// FAQ stops here: no splitting / image / audio / graph / advanced.
	if kb.Type != KnowledgeBaseTypeDocument {
		return
	}

	// E: chunking. A zero ChunkSize means the caller supplied nothing; fall back
	// to the platform default, then to the built-in preset so a freshly created
	// KB is never left with an invalid (0) chunk size.
	if kb.ChunkingConfig.ChunkSize == 0 {
		switch {
		case d.ChunkingConfig != nil:
			kb.ChunkingConfig = *d.ChunkingConfig
		default:
			kb.ChunkingConfig = *BuiltInKBChunking()
		}
	}

	// F: image / multimodal. VLM is unset when neither enabled nor model-bound.
	if !kb.VLMConfig.Enabled && kb.VLMConfig.ModelID == "" && d.VLMConfig != nil {
		kb.VLMConfig = *d.VLMConfig
	}
	if kb.ImageProcessingConfig.ModelID == "" &&
		!kb.ImageProcessingConfig.ImageAttrsEnabled &&
		!kb.ImageProcessingConfig.ImageVectorEnabled &&
		d.ImageProcessingConfig != nil {
		kb.ImageProcessingConfig = *d.ImageProcessingConfig
	}

	// G: audio (ASR).
	if !kb.ASRConfig.Enabled && kb.ASRConfig.ModelID == "" && d.ASRConfig != nil {
		kb.ASRConfig = *d.ASRConfig
	}

	// H: knowledge graph.
	if kb.ExtractConfig == nil && d.ExtractConfig != nil {
		kb.ExtractConfig = d.ExtractConfig
	}

	// I: advanced (pointer fields — nil means "not supplied").
	if kb.QuestionGenerationConfig == nil && d.QuestionGenerationConfig != nil {
		kb.QuestionGenerationConfig = d.QuestionGenerationConfig
	}
	if kb.AutoTagConfig == nil && d.AutoTagConfig != nil {
		kb.AutoTagConfig = d.AutoTagConfig
	}
	if kb.ProfileConfig == nil && d.ProfileConfig != nil {
		kb.ProfileConfig = d.ProfileConfig
	}

	// C: vector-store binding — fill only when the caller left it unset and the
	// platform default names a concrete store.
	if kb.VectorStoreID == nil && d.VectorStoreID != nil && *d.VectorStoreID != "" {
		id := *d.VectorStoreID
		kb.VectorStoreID = &id
	}
	// J: storage backend / provider selection — fill only when unset.
	if kb.StorageBackendID == nil && d.StorageBackendID != nil && *d.StorageBackendID != "" {
		id := *d.StorageBackendID
		kb.StorageBackendID = &id
	}
	if kb.StorageProviderConfig == nil && d.StorageProviderConfig != nil && d.StorageProviderConfig.Provider != "" {
		kb.StorageProviderConfig = d.StorageProviderConfig
	}
}

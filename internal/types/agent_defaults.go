package types

import (
	"encoding/json"
	"strings"
)

// AgentDefaultsKey is the system_settings key holding the platform-wide default
// agent configuration managed by the platform super administrator under
// Settings -> 智能体配置 (agent-*). The stored value is a JSON document decoded
// into AgentDefaultsConfig; an empty string means "no platform defaults
// configured" and every newly created agent keeps its request values /
// built-in defaults.
const AgentDefaultsKey = "agent.default_config"

// AgentDefaultsConfig captures the agent settings that have been reclaimed from
// the per-agent editor to a single platform-level configuration:
//
//	A 模型配置    -> ModelID / RerankModelID / QueryUnderstandModelID / Temperature /
//	                 MaxCompletionTokens / Thinking / ReasoningEffort / CitationEnabled /
//	                 MaxIterations / LLMCallTimeout
//	B 多轮对话    -> MultiTurnEnabled / HistoryTurns / RetainRetrievalHistory / EnableRewrite
//	C 问题推荐    -> QuestionSuggestions
//	E 检索策略    -> EnableQueryExpansion / EmbeddingTopK / KeywordThreshold /
//	                 VectorThreshold / RerankTopK / RerankThreshold / FAQ* / DataAnalysisEnabled
//	F 网络搜索    -> WebSearch* / WebFetch*
//	G 附件上传    -> ImageUploadEnabled / VLMModelID / Attachment* / ImageStorageProvider /
//	                 AudioUploadEnabled / ASRModelID / ChatParserEngineRules
//	H 工具配置    -> AllowedTools
//	I MCP 服务    -> MCPSelectionMode / MCPServices / MCPAuthWaitTimeout
//	J 技能        -> SkillsSelectionMode / SelectedSkills / SandboxConfigID
//
// Basic info (name/description/agent_mode/agent_type), prompts and the
// knowledge-base scope (知识库检索, requirement item D) stay user-owned and are
// deliberately NOT here.
//
// Every field is optional. Booleans are pointers so "the operator left it
// unset" can be told apart from "the operator explicitly chose false"; the
// remaining fields use their type's zero value as the "unset" marker. ApplyTo
// only writes fields the caller left at that zero value, so a value the user
// did set is never overwritten.
type AgentDefaultsConfig struct {
	// A: model settings.
	ModelID                string   `json:"model_id,omitempty"`
	RerankModelID          string   `json:"rerank_model_id,omitempty"`
	QueryUnderstandModelID string   `json:"query_understand_model_id,omitempty"`
	Temperature            *float64 `json:"temperature,omitempty"`
	MaxCompletionTokens    int      `json:"max_completion_tokens,omitempty"`
	Thinking               *bool    `json:"thinking,omitempty"`
	ReasoningEffort        string   `json:"reasoning_effort,omitempty"`
	CitationEnabled        *bool    `json:"citation_enabled,omitempty"`
	MaxIterations          int      `json:"max_iterations,omitempty"`
	LLMCallTimeout         int      `json:"llm_call_timeout,omitempty"`

	// B: multi-turn conversation.
	MultiTurnEnabled       *bool `json:"multi_turn_enabled,omitempty"`
	HistoryTurns           int   `json:"history_turns,omitempty"`
	RetainRetrievalHistory *bool `json:"retain_retrieval_history,omitempty"`
	EnableRewrite          *bool `json:"enable_rewrite,omitempty"`

	// C: question suggestions.
	QuestionSuggestions *QuestionSuggestionConfig `json:"question_suggestions,omitempty"`

	// E: retrieval strategy.
	EnableQueryExpansion     *bool   `json:"enable_query_expansion,omitempty"`
	EmbeddingTopK            int     `json:"embedding_top_k,omitempty"`
	KeywordThreshold         float64 `json:"keyword_threshold,omitempty"`
	VectorThreshold          float64 `json:"vector_threshold,omitempty"`
	RerankTopK               int     `json:"rerank_top_k,omitempty"`
	RerankThreshold          float64 `json:"rerank_threshold,omitempty"`
	FAQPriorityEnabled       *bool   `json:"faq_priority_enabled,omitempty"`
	FAQDirectAnswerThreshold float64 `json:"faq_direct_answer_threshold,omitempty"`
	FAQScoreBoost            float64 `json:"faq_score_boost,omitempty"`
	DataAnalysisEnabled      *bool   `json:"data_analysis_enabled,omitempty"`

	// F: web search.
	WebSearchEnabled    *bool  `json:"web_search_enabled,omitempty"`
	WebSearchProviderID string `json:"web_search_provider_id,omitempty"`
	WebSearchMaxResults int    `json:"web_search_max_results,omitempty"`
	WebFetchEnabled     *bool  `json:"web_fetch_enabled,omitempty"`
	WebFetchTopN        int    `json:"web_fetch_top_n,omitempty"`

	// G: attachment / multimodal upload.
	ImageUploadEnabled            *bool              `json:"image_upload_enabled,omitempty"`
	VLMModelID                    string             `json:"vlm_model_id,omitempty"`
	AttachmentImageUnderstanding  *bool              `json:"attachment_image_understanding,omitempty"`
	AttachmentOCRMaxPages         int                `json:"attachment_ocr_max_pages,omitempty"`
	ImageStorageProvider          string             `json:"image_storage_provider,omitempty"`
	AudioUploadEnabled            *bool              `json:"audio_upload_enabled,omitempty"`
	ASRModelID                    string             `json:"asr_model_id,omitempty"`
	AttachmentParseWaitTimeoutSec int                `json:"attachment_parse_wait_timeout_sec,omitempty"`
	ChatParserEngineRules         []ParserEngineRule `json:"chat_parser_engine_rules,omitempty"`

	// H: tool configuration.
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// I: MCP services.
	MCPSelectionMode   string   `json:"mcp_selection_mode,omitempty"`
	MCPServices        []string `json:"mcp_services,omitempty"`
	MCPAuthWaitTimeout int      `json:"mcp_auth_wait_timeout,omitempty"`

	// J: skills.
	SkillsSelectionMode string   `json:"skills_selection_mode,omitempty"`
	SelectedSkills      []string `json:"selected_skills,omitempty"`
	SandboxConfigID     string   `json:"sandbox_config_id,omitempty"`
}

// ParseAgentDefaultsConfig decodes a raw system_settings JSON string into an
// AgentDefaultsConfig. An empty / whitespace-only value or malformed JSON
// returns nil so callers can treat "no defaults" uniformly.
func ParseAgentDefaultsConfig(raw string) *AgentDefaultsConfig {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	cfg := &AgentDefaultsConfig{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return nil
	}
	return cfg
}

// ApplyTo fills unset configuration on a freshly-built agent config from the
// platform defaults. It only writes fields the caller left at their zero value,
// so an API/MCP client that still sends an explicit config keeps its values,
// and the platform default only acts as the source of truth for the fields the
// UI no longer exposes.
//
// IMPORTANT: call this BEFORE CustomAgent.EnsureDefaults(). EnsureDefaults
// materialises concrete values for every empty field, after which "the user
// left it unset" and "the user picked the default" are indistinguishable.
func (d *AgentDefaultsConfig) ApplyTo(c *CustomAgentConfig) {
	if d == nil || c == nil {
		return
	}

	// ===== A: model settings =====
	fillString(&c.ModelID, d.ModelID)
	fillString(&c.RerankModelID, d.RerankModelID)
	fillString(&c.QueryUnderstandModelID, d.QueryUnderstandModelID)
	if c.Temperature == 0 && d.Temperature != nil {
		c.Temperature = *d.Temperature
	}
	if c.MaxCompletionTokens == 0 {
		c.MaxCompletionTokens = d.MaxCompletionTokens
	}
	// Thinking / CitationEnabled are pointers on the config too: nil means
	// "not supplied", which is the only case the platform default fills.
	if c.Thinking == nil && d.Thinking != nil {
		c.Thinking = d.Thinking
	}
	fillString(&c.ReasoningEffort, d.ReasoningEffort)
	if c.CitationEnabled == nil && d.CitationEnabled != nil {
		c.CitationEnabled = d.CitationEnabled
	}
	if c.MaxIterations == 0 {
		c.MaxIterations = d.MaxIterations
	}
	if c.LLMCallTimeout == 0 {
		c.LLMCallTimeout = d.LLMCallTimeout
	}

	// ===== B: multi-turn conversation =====
	fillBool(&c.MultiTurnEnabled, d.MultiTurnEnabled)
	if c.HistoryTurns == 0 {
		c.HistoryTurns = d.HistoryTurns
	}
	fillBool(&c.RetainRetrievalHistory, d.RetainRetrievalHistory)
	fillBool(&c.EnableRewrite, d.EnableRewrite)

	// ===== C: question suggestions =====
	if c.QuestionSuggestions == nil && d.QuestionSuggestions != nil {
		c.QuestionSuggestions = d.QuestionSuggestions
	}

	// ===== E: retrieval strategy =====
	fillBool(&c.EnableQueryExpansion, d.EnableQueryExpansion)
	if c.EmbeddingTopK == 0 {
		c.EmbeddingTopK = d.EmbeddingTopK
	}
	if c.KeywordThreshold == 0 {
		c.KeywordThreshold = d.KeywordThreshold
	}
	if c.VectorThreshold == 0 {
		c.VectorThreshold = d.VectorThreshold
	}
	if c.RerankTopK == 0 {
		c.RerankTopK = d.RerankTopK
	}
	if c.RerankThreshold == 0 {
		c.RerankThreshold = d.RerankThreshold
	}
	fillBool(&c.FAQPriorityEnabled, d.FAQPriorityEnabled)
	if c.FAQDirectAnswerThreshold == 0 {
		c.FAQDirectAnswerThreshold = d.FAQDirectAnswerThreshold
	}
	if c.FAQScoreBoost == 0 {
		c.FAQScoreBoost = d.FAQScoreBoost
	}
	fillBool(&c.DataAnalysisEnabled, d.DataAnalysisEnabled)

	// ===== F: web search =====
	fillBool(&c.WebSearchEnabled, d.WebSearchEnabled)
	fillString(&c.WebSearchProviderID, d.WebSearchProviderID)
	if c.WebSearchMaxResults == 0 {
		c.WebSearchMaxResults = d.WebSearchMaxResults
	}
	fillBool(&c.WebFetchEnabled, d.WebFetchEnabled)
	if c.WebFetchTopN == 0 {
		c.WebFetchTopN = d.WebFetchTopN
	}

	// ===== G: attachment / multimodal =====
	fillBool(&c.ImageUploadEnabled, d.ImageUploadEnabled)
	fillString(&c.VLMModelID, d.VLMModelID)
	fillBool(&c.AttachmentImageUnderstanding, d.AttachmentImageUnderstanding)
	if c.AttachmentOCRMaxPages == 0 {
		c.AttachmentOCRMaxPages = d.AttachmentOCRMaxPages
	}
	fillString(&c.ImageStorageProvider, d.ImageStorageProvider)
	fillBool(&c.AudioUploadEnabled, d.AudioUploadEnabled)
	fillString(&c.ASRModelID, d.ASRModelID)
	if c.AttachmentParseWaitTimeoutSec == 0 {
		c.AttachmentParseWaitTimeoutSec = d.AttachmentParseWaitTimeoutSec
	}
	if len(c.ChatParserEngineRules) == 0 && len(d.ChatParserEngineRules) > 0 {
		c.ChatParserEngineRules = d.ChatParserEngineRules
	}

	// ===== H: tools =====
	// An empty AllowedTools means "use the built-in default tool set" at
	// runtime, so an empty slice is the unset marker.
	if len(c.AllowedTools) == 0 && len(d.AllowedTools) > 0 {
		c.AllowedTools = d.AllowedTools
	}

	// ===== I: MCP =====
	// "none" is both the zero-ish default and a meaningful choice, so only fill
	// from an explicit non-"none" platform default.
	fillString(&c.MCPSelectionMode, d.MCPSelectionMode)
	if len(c.MCPServices) == 0 && len(d.MCPServices) > 0 {
		c.MCPServices = d.MCPServices
	}
	if c.MCPAuthWaitTimeout == 0 {
		c.MCPAuthWaitTimeout = d.MCPAuthWaitTimeout
	}

	// ===== J: skills =====
	fillString(&c.SkillsSelectionMode, d.SkillsSelectionMode)
	if len(c.SelectedSkills) == 0 && len(d.SelectedSkills) > 0 {
		c.SelectedSkills = d.SelectedSkills
	}
	fillString(&c.SandboxConfigID, d.SandboxConfigID)
}

// fillString writes def into *dst when dst is still empty and def is not.
func fillString(dst *string, def string) {
	if strings.TrimSpace(*dst) == "" && def != "" {
		*dst = def
	}
}

// fillBool writes *def into *dst when dst is false and the operator actually
// set the default. A user who flipped the switch to true keeps their choice.
func fillBool(dst *bool, def *bool) {
	if !*dst && def != nil {
		*dst = *def
	}
}

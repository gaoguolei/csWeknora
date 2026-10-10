import type { CustomAgentConfig } from '@/api/agent'
import type { ModelConfig } from '@/api/model'
import { allowedToolsInclude } from './legacy-tool-names'
import { selectInitialModelId } from './modelDefaults'

export type AgentNotReadyReasonKey = 'summary_model' | 'rerank_model' | 'allowed_tools'

// Readiness only needs the identity fields; kept structural so callers can
// pass the full ModelConfig list they already hold.
type AgentModelCandidate = Pick<ModelConfig, 'id' | 'type' | 'status' | 'is_default'>

/**
 * Whether an agent can start a conversation model-wise.
 *
 * An agent that explicitly binds a chat model must have that model still
 * present — a stale ID is a real misconfiguration and is never papered over.
 * An agent with no binding (built-in agents, or one saved before any model
 * existed) inherits the platform default chat model at request time, mirroring
 * `sessionService.resolveChatModelID`, so it is ready as soon as a usable
 * model of that type exists. The user therefore never has to open the editor
 * and save just to send a first message.
 *
 * For shared agents the model belongs to the source tenant, so the receiver can
 * only require a non-empty ID and must leave existence validation to the
 * backend.
 */
export function agentHasConfiguredChatModel(
  config: Pick<CustomAgentConfig, 'model_id'> | undefined,
  models: AgentModelCandidate[],
  sourceModelIsRemote = false,
): boolean {
  const modelID = config?.model_id?.trim()
  if (!modelID) {
    return sourceModelIsRemote || !!selectInitialModelId(models, 'KnowledgeQA')
  }
  if (sourceModelIsRemote) return true
  return models.some(model => model.type === 'KnowledgeQA' && model.id === modelID)
}

/**
 * Keep this aligned with backend agentRequiresRerankModel.
 *
 * Rerank is needed only when search_knowledge can actually run. Explicitly
 * disabling the knowledge-base scope makes KB tools ineffective, even if an
 * older agent configuration still contains the tool in allowed_tools.
 *
 * Stored configs may still carry the retired `knowledge_search` name (and
 * `grep_chunks`, which was folded into search_knowledge); those count too.
 */
export function agentRequiresRerankModel(
  config: Pick<CustomAgentConfig, 'kb_selection_mode' | 'allowed_tools'> | undefined,
): boolean {
  if (!config || config.kb_selection_mode === 'none') return false

  const allowedTools = config.allowed_tools || []
  // The backend falls back to DefaultAllowedTools when the list is empty,
  // and that default includes search_knowledge.
  if (allowedTools.length === 0) return true

  return allowedToolsInclude(allowedTools, 'search_knowledge')
}

export function getAgentNotReadyReasonKeys(
  config: Pick<
    CustomAgentConfig,
    'model_id' | 'rerank_model_id' | 'kb_selection_mode' | 'allowed_tools' | 'agent_mode'
  > | undefined,
  models: AgentModelCandidate[],
  options: { isAgentMode: boolean; isSharedAgent: boolean },
): AgentNotReadyReasonKey[] {
  const reasons: AgentNotReadyReasonKey[] = []

  if (!agentHasConfiguredChatModel(config, models, options.isSharedAgent)) {
    reasons.push('summary_model')
  }

  if (options.isAgentMode && agentRequiresRerankModel(config)) {
    const rerankModelID = config?.rerank_model_id?.trim()
    if (rerankModelID) {
      const rerankExists = options.isSharedAgent
        || models.some(model => model.type === 'Rerank' && model.id === rerankModelID)
      if (!rerankExists) {
        reasons.push('rerank_model')
      }
    } else if (!options.isSharedAgent && !selectInitialModelId(models, 'Rerank')) {
      // Unbound rerank inherits the platform default reranker (see
      // AgentQA in session_agent_qa.go); only a workspace with no Rerank
      // model at all is genuinely blocked.
      reasons.push('rerank_model')
    }
  }

  return reasons
}

/** Shared agents are owned by another tenant; receivers cannot edit their config. */
export function canLocallyConfigureAgent(sourceTenantId?: string): boolean {
  return !sourceTenantId
}

/** Map missing-config reasons to the agent editor section that fixes them. */
export function resolveAgentNotReadySection(reasons: AgentNotReadyReasonKey[]): string {
  if (reasons.includes('summary_model') || reasons.includes('rerank_model')) {
    return 'model'
  }
  if (reasons.includes('allowed_tools')) {
    return 'tools'
  }
  return 'model'
}

/** First missing item — used to highlight the corresponding editor field after navigation. */
export function resolveAgentNotReadyHighlight(
  reasons: AgentNotReadyReasonKey[],
): AgentNotReadyReasonKey | undefined {
  return reasons[0]
}

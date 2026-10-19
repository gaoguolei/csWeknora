import type { WorkspaceIntent } from './workspaceEvents'

export type ComposerMode = 'chat' | 'coding'

export interface ComposerAgent {
  id: string
  config?: {
    agent_mode?: string
    agent_type?: string
    system_prompt_id?: string
    allowed_tools?: readonly string[] | null
  }
}

export interface ComposerAgentChoice<T extends ComposerAgent = ComposerAgent> {
  agent: T
  sourceTenantId?: string
}

export function composerModeFromIntent(intent: WorkspaceIntent): ComposerMode {
  return ['build', 'code', 'test'].includes(intent) ? 'coding' : 'chat'
}

/** Explicit home selection wins; older session links fall back to their agent. */
export function conversationComposerMode(intent: unknown, agent?: ComposerAgent): ComposerMode {
  if (typeof intent === 'string' && ['chat', 'knowledge', 'build', 'code', 'test'].includes(intent))
    return composerModeFromIntent(intent as WorkspaceIntent)
  return agent && isCodingAgent(agent) ? 'coding' : 'chat'
}

export function isCodingAgent(agent: ComposerAgent): boolean {
  const config = agent.config
  return agent.id === 'builtin-coding-assistant'
    || config?.system_prompt_id === 'coding_assistant'
    || ['coding', 'programming'].includes(config?.agent_type || '')
    || !!config?.allowed_tools?.some(tool =>
      ['write_sandbox_file', 'edit_sandbox_file', 'workspace_tests'].includes(tool))
}

/** Sandbox access alone also serves research and data analysis. */
export function agentMatchesComposerMode(agent: ComposerAgent, mode?: ComposerMode): boolean {
  return !mode || isCodingAgent(agent) === (mode === 'coding')
}

export function chooseComposerAgent<T extends ComposerAgent>(
  mode: ComposerMode, choices: ComposerAgentChoice<T>[],
  currentId: string, currentSourceTenantId?: string | null,
): ComposerAgentChoice<T> | undefined {
  const available = choices.filter(choice => agentMatchesComposerMode(choice.agent, mode))
  const current = available.find(choice => choice.agent.id === currentId
    && (choice.sourceTenantId || '') === (currentSourceTenantId || ''))
  if (current) return current
  const preferred = mode === 'coding' ? 'builtin-coding-assistant' : 'builtin-quick-answer'
  return available.find(choice => !choice.sourceTenantId && choice.agent.id === preferred)
    || available[0]
}

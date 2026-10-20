import type { WorkspaceIntent } from './workspaceEvents'

export type ComposerMode = 'chat' | 'coding' | 'testing'

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
  return intent === 'test' ? 'testing' : ['build', 'code'].includes(intent) ? 'coding' : 'chat'
}

/** Explicit home selection wins; older session links fall back to their agent. */
export function conversationComposerMode(intent: unknown, agent?: ComposerAgent): ComposerMode {
  if (typeof intent === 'string' && ['chat', 'knowledge', 'build', 'code', 'test'].includes(intent))
    return composerModeFromIntent(intent as WorkspaceIntent)
  return agent ? agentComposerMode(agent) : 'chat'
}

export function agentComposerMode(agent: ComposerAgent): ComposerMode {
  const config = agent.config
  if (agent.id === 'builtin-testing-assistant'
    || config?.system_prompt_id === 'testing_assistant'
    || ['testing', 'test', 'qa'].includes(config?.agent_type || '')) return 'testing'
  if (agent.id === 'builtin-coding-assistant'
    || config?.system_prompt_id === 'coding_assistant'
    || ['coding', 'programming'].includes(config?.agent_type || '')) return 'coding'
  if (config?.allowed_tools?.includes('workspace_tests')) return 'testing'
  if (config?.allowed_tools?.some(tool => ['write_sandbox_file', 'edit_sandbox_file'].includes(tool))) return 'coding'
  return 'chat'
}

export function isCodingAgent(agent: ComposerAgent): boolean {
  return agentComposerMode(agent) === 'coding'
}

/** Sandbox access alone also serves research and data analysis. */
export function agentMatchesComposerMode(agent: ComposerAgent, mode?: ComposerMode): boolean {
  return !mode || agentComposerMode(agent) === mode
}

export function chooseComposerAgent<T extends ComposerAgent>(
  mode: ComposerMode, choices: ComposerAgentChoice<T>[],
  currentId: string, currentSourceTenantId?: string | null,
): ComposerAgentChoice<T> | undefined {
  const available = choices.filter(choice => agentMatchesComposerMode(choice.agent, mode))
  const current = available.find(choice => choice.agent.id === currentId
    && (choice.sourceTenantId || '') === (currentSourceTenantId || ''))
  if (current) return current
  const preferred = { chat: 'builtin-quick-answer', coding: 'builtin-coding-assistant', testing: 'builtin-testing-assistant' }[mode]
  return available.find(choice => !choice.sourceTenantId && choice.agent.id === preferred)
    || available[0]
}

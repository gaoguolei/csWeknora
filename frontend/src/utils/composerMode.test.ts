import test from 'node:test'
import assert from 'node:assert/strict'
import { agentMatchesComposerMode, chooseComposerAgent, composerModeFromIntent, conversationComposerMode, isCodingAgent, type ComposerAgent } from './composerMode'

const quick: ComposerAgent = { id: 'builtin-quick-answer', config: { agent_mode: 'quick-answer' } }
const coding: ComposerAgent = { id: 'builtin-coding-assistant', config: { agent_mode: 'smart-reasoning' } }
const custom: ComposerAgent = { id: 'custom-coder', config: { system_prompt_id: 'coding_assistant' } }

test('conversation mode follows the explicit home selection and recovers older agent links', () => {
  assert.equal(conversationComposerMode('chat', coding), 'chat')
  assert.equal(conversationComposerMode('code', quick), 'coding')
  assert.equal(conversationComposerMode('test', quick), 'coding')
  assert.equal(conversationComposerMode(undefined, custom), 'coding')
  assert.equal(conversationComposerMode(undefined, coding), 'coding')
  assert.equal(conversationComposerMode(undefined, quick), 'chat')
  assert.equal(conversationComposerMode(undefined), 'chat')
})

test('home intents map to two modes, including old deep links', () => {
  assert.equal(composerModeFromIntent('chat'), 'chat')
  assert.equal(composerModeFromIntent('knowledge'), 'chat')
  for (const intent of ['build', 'code', 'test'] as const)
    assert.equal(composerModeFromIntent(intent), 'coding')
})

test('coding classification follows IDs and capabilities, not any sandbox access', () => {
  assert.equal(isCodingAgent(coding), true)
  assert.equal(isCodingAgent(custom), true)
  assert.equal(isCodingAgent({ id: 'tests', config: { allowed_tools: ['workspace_tests'] } }), true)
  assert.equal(isCodingAgent({ id: 'editor', config: { allowed_tools: ['edit_sandbox_file'] } }), true)
  const analyst = { id: 'analyst', config: { sandbox_config_id: 'sandbox', allowed_tools: ['data_analysis', 'execute_code'] } }
  assert.equal(isCodingAgent(analyst), false)
  assert.equal(isCodingAgent({ id: 'builtin-smart-reasoning' }), false)
})

test('mode filtering applies equally to owned and shared coding agents', () => {
  for (const agent of [coding, custom]) {
    assert.equal(agentMatchesComposerMode(agent, 'chat'), false)
    assert.equal(agentMatchesComposerMode(agent, 'coding'), true)
  }
  assert.equal(agentMatchesComposerMode(quick, 'chat'), true)
  assert.equal(agentMatchesComposerMode(quick, 'coding'), false)
  assert.equal(agentMatchesComposerMode(coding), true)
})

test('switching modes replaces a hidden selected agent with the matching builtin', () => {
  const choices = [{ agent: quick }, { agent: custom }, { agent: coding }]
  assert.equal(chooseComposerAgent('coding', choices, quick.id)?.agent.id, coding.id)
  assert.equal(chooseComposerAgent('chat', choices, coding.id)?.agent.id, quick.id)
  assert.equal(chooseComposerAgent('coding', choices, custom.id)?.agent.id, custom.id)
})

test('selection preserves shared tenant identity even when builtin IDs collide', () => {
  const choices = [{ agent: coding }, { agent: coding, sourceTenantId: 'other-space' }]
  assert.equal(chooseComposerAgent('coding', choices, coding.id, 'other-space')?.sourceTenantId, 'other-space')
  assert.equal(chooseComposerAgent('coding', choices, coding.id)?.sourceTenantId, undefined)
})

test('an unavailable coding group never falls back to a chat agent', () => {
  assert.equal(chooseComposerAgent('coding', [{ agent: quick }], quick.id), undefined)
  assert.equal(chooseComposerAgent('chat', [], coding.id), undefined)
  assert.equal(chooseComposerAgent('coding', [{ agent: custom, sourceTenantId: 'shared' }], quick.id)?.sourceTenantId, 'shared')
})

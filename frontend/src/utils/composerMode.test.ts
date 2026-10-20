import test from 'node:test'
import assert from 'node:assert/strict'
import { agentComposerMode, agentMatchesComposerMode, chooseComposerAgent, composerModeFromIntent, conversationComposerMode, isCodingAgent, type ComposerAgent } from './composerMode'

const quick: ComposerAgent = { id: 'builtin-quick-answer', config: { agent_mode: 'quick-answer' } }
const coding: ComposerAgent = { id: 'builtin-coding-assistant', config: { agent_mode: 'smart-reasoning' } }
const custom: ComposerAgent = { id: 'custom-coder', config: { system_prompt_id: 'coding_assistant' } }
const testing: ComposerAgent = { id: 'builtin-testing-assistant', config: { agent_mode: 'smart-reasoning' } }
const customTesting: ComposerAgent = { id: 'custom-tester', config: { system_prompt_id: 'testing_assistant', allowed_tools: ['edit_sandbox_file'] } }

test('conversation mode follows the explicit home selection and recovers older agent links', () => {
  assert.equal(conversationComposerMode('chat', coding), 'chat')
  assert.equal(conversationComposerMode('code', quick), 'coding')
  assert.equal(conversationComposerMode('test', quick), 'testing')
  assert.equal(conversationComposerMode(undefined, custom), 'coding')
  assert.equal(conversationComposerMode(undefined, coding), 'coding')
  assert.equal(conversationComposerMode(undefined, quick), 'chat')
  assert.equal(conversationComposerMode(undefined), 'chat')
  assert.equal(conversationComposerMode(undefined, testing), 'testing')
  assert.equal(conversationComposerMode(undefined, customTesting), 'testing')
})

test('home intents map to three modes, including old deep links', () => {
  assert.equal(composerModeFromIntent('chat'), 'chat')
  assert.equal(composerModeFromIntent('knowledge'), 'chat')
  for (const intent of ['build', 'code'] as const)
    assert.equal(composerModeFromIntent(intent), 'coding')
  assert.equal(composerModeFromIntent('test'), 'testing')
})

test('coding classification follows IDs and capabilities, not any sandbox access', () => {
  assert.equal(isCodingAgent(coding), true)
  assert.equal(isCodingAgent(custom), true)
  assert.equal(agentComposerMode({ id: 'tests', config: { allowed_tools: ['workspace_tests'] } }), 'testing')
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

test('testing agents are isolated from chat and coding, including custom test writers', () => {
  for (const agent of [testing, customTesting]) {
    assert.equal(agentMatchesComposerMode(agent, 'testing'), true)
    assert.equal(agentMatchesComposerMode(agent, 'coding'), false)
    assert.equal(agentMatchesComposerMode(agent, 'chat'), false)
  }
  assert.equal(agentMatchesComposerMode(quick, 'testing'), false)
  assert.equal(agentMatchesComposerMode(coding, 'testing'), false)
  assert.equal(agentComposerMode({ id: 'qa', config: { agent_type: 'qa' } }), 'testing')
  assert.equal(agentComposerMode({ ...custom, config: { ...custom.config, allowed_tools: ['workspace_tests'] } }), 'coding')
})

test('switching modes replaces a hidden selected agent with the matching builtin', () => {
  const choices = [{ agent: quick }, { agent: custom }, { agent: coding }, { agent: customTesting }, { agent: testing }]
  assert.equal(chooseComposerAgent('coding', choices, quick.id)?.agent.id, coding.id)
  assert.equal(chooseComposerAgent('chat', choices, coding.id)?.agent.id, quick.id)
  assert.equal(chooseComposerAgent('coding', choices, custom.id)?.agent.id, custom.id)
  assert.equal(chooseComposerAgent('testing', choices, coding.id)?.agent.id, testing.id)
  assert.equal(chooseComposerAgent('testing', choices, customTesting.id)?.agent.id, customTesting.id)
  assert.equal(chooseComposerAgent('coding', choices, testing.id)?.agent.id, coding.id)
  assert.equal(chooseComposerAgent('chat', choices, testing.id)?.agent.id, quick.id)
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
  assert.equal(chooseComposerAgent('testing', [{ agent: coding }, { agent: quick }], coding.id), undefined)
  assert.equal(chooseComposerAgent('testing', [{ agent: customTesting, sourceTenantId: 'shared' }], quick.id)?.sourceTenantId, 'shared')
})

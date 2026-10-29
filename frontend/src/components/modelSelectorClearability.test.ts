import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'

const selector = readFileSync(new URL('./ModelSelector.vue', import.meta.url), 'utf8')
// 编辑器的模型选择器已按分区抽到 settings/ 下的组件里（编辑器与平台「智能体配置」共用）
const agentModelSettings = readFileSync(new URL('../views/agent/settings/AgentModelSettings.vue', import.meta.url), 'utf8')
const agentAttachmentSettings = readFileSync(new URL('../views/agent/settings/AgentAttachmentSettings.vue', import.meta.url), 'utf8')
const agentSuggestionsSettings = readFileSync(new URL('../views/agent/settings/AgentSuggestionsSettings.vue', import.meta.url), 'utf8')
const kbModelConfig = readFileSync(new URL('../views/knowledge/settings/KBModelConfig.vue', import.meta.url), 'utf8')
const uploadConfirm = readFileSync(new URL('../views/knowledge/components/UploadConfirmDialog.vue', import.meta.url), 'utf8')

function modelSelectorTag(source: string, selectedModelBinding: string): string {
  const tag = source
    .match(/<ModelSelector\b[\s\S]*?\/>/g)
    ?.find((candidate) => candidate.includes(selectedModelBinding))
  assert.ok(tag, `找不到模型选择器：${selectedModelBinding}`)
  return tag
}

function assertClearable(tag: string): void {
  assert.match(tag, /(?:\s|:)clearable(?:\s|=|\/>)/)
}

function assertNotClearable(tag: string): void {
  assert.doesNotMatch(tag, /(?:\s|:)clearable(?:\s|=|\/>)/)
}

test('ModelSelector 清空时向父组件回传空字符串，默认仍不可清空', () => {
  assert.match(selector, /:clearable="clearable"/)
  assert.match(selector, /clearable:\s*false/)
  assert.match(selector, /emit\('update:selectedModelId', value \|\| ''\)/)
})

test('智能体中允许继承或关闭的可选模型可以恢复为空', () => {
  const rerank = modelSelectorTag(agentModelSettings, 'config.rerank_model_id')
  assert.match(rerank, /:clearable="!needsRerankModel"/)
  assertClearable(modelSelectorTag(agentModelSettings, 'config.query_understand_model_id'))
  assertClearable(modelSelectorTag(agentAttachmentSettings, 'config.asr_model_id'))
  assertClearable(modelSelectorTag(agentSuggestionsSettings, 'qs.follow_ups.model_id'))
})

test('知识库仅在模型确实可选时允许恢复为空', () => {
  const embedding = modelSelectorTag(kbModelConfig, 'config.embeddingModelId')
  assert.match(embedding, /:clearable="ragEnabled === false && wikiEnabled"/)
  assertClearable(modelSelectorTag(kbModelConfig, 'config.wikiSynthesisModelId'))
})

test('必填模型继续保持不可清空', () => {
  assertNotClearable(modelSelectorTag(agentModelSettings, 'config.model_id'))
  assertNotClearable(modelSelectorTag(agentAttachmentSettings, 'config.vlm_model_id'))
  assertNotClearable(modelSelectorTag(kbModelConfig, 'config.llmModelId'))
  // 平台「知识库设置」里清空 = 不设默认，所以图像/音频模型在那里是可清空的；不再断言。
  assertNotClearable(modelSelectorTag(uploadConfirm, 'uiState.multimodalConfig.vllmModelId'))
  assertNotClearable(modelSelectorTag(uploadConfirm, 'uiState.asrConfig.modelId'))
})

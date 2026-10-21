<template>
  <div class="section-header">
    <h2>{{ $t('agent.editor.modelConfig') }}</h2>
    <p class="section-description">{{ $t('agent.editor.modelConfigDesc') }}</p>
  </div>

  <div class="settings-group">
    <!-- 模型选择 -->
    <div class="setting-row" data-guide="agent-create-model" data-agent-field="summary_model"
      :class="{ 'setting-row--field-highlight': highlightedField === 'summary_model' }">
      <div class="setting-info">
        <label>{{ $t('agent.editor.model') }} <span class="required">*</span></label>
        <p class="desc">{{ $t('agentEditor.desc.model') }}</p>
      </div>
      <div class="setting-control">
        <ModelSelector model-type="KnowledgeQA" :selected-model-id="config.model_id" :all-models="allModels"
          @update:selected-model-id="(val: string) => config.model_id = val"
          @add-model="emit('addModel', 'llm')" :placeholder="$t('agent.editor.modelPlaceholder')" />
      </div>
    </div>

    <!-- 温度 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.temperature') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.temperature') }}</p>
      </div>
      <div class="setting-control">
        <div class="slider-wrapper">
          <t-slider v-model="config.temperature" :min="0" :max="1" :step="0.1" />
          <span class="slider-value">{{ config.temperature }}</span>
        </div>
      </div>
    </div>

    <!-- 最大生成Token数：0 表示跟随系统默认；自定义后按输入值保存 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.maxCompletionTokens') }}</label>
        <p class="desc">{{ isAgentMode ? $t('agentEditor.desc.maxTokensAgent') :
          $t('agentEditor.desc.maxTokens') }}
        </p>
      </div>
      <div class="setting-control max-tokens-control">
        <t-radio-group v-model="maxCompletionTokensMode">
          <t-radio-button value="default">{{ $t('agent.editor.maxCompletionTokensDefault')
            }}</t-radio-button>
          <t-radio-button value="custom">{{ $t('agent.editor.maxCompletionTokensCustom')
            }}</t-radio-button>
        </t-radio-group>
        <span v-if="maxCompletionTokensMode === 'default'" class="max-tokens-value">
          {{ effectiveDefaultMaxCompletionTokens }}
        </span>
        <t-input-number v-else v-model="config.max_completion_tokens" :min="100" :max="100000" :step="100"
          theme="column" />
      </div>
    </div>

    <!-- 思考强度：off / auto + 所选对话模型目录上报的等级 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.thinking') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.thinking') }}</p>
        <p v-if="hasSelectedChatModel && !selectedChatModelCanThink" class="desc">
          {{ $t('agent.editor.reasoningEffortUnsupported') }}
        </p>
        <p v-else-if="selectedChatModelAlwaysThinks" class="desc">
          {{ $t('agent.editor.reasoningEffortAlwaysOn') }}
        </p>
      </div>
      <div class="setting-control">
        <t-select v-model="reasoningEffortLevel" class="reasoning-effort-select"
          :popup-props="{ overlayClassName: 'reasoning-level-select-popup' }">
          <t-option v-for="level in reasoningEffortOptions" :key="level" :value="level"
            :label="$t(levelLabelKey(level))" :show-overflow-tooltip="false">
            <div class="reasoning-level-option">
              <span class="reasoning-level-option__title">{{ $t(levelLabelKey(level)) }}</span>
              <span class="reasoning-level-option__hint">{{ $t(levelDescriptionKey(level)) }}</span>
            </div>
          </t-option>
        </t-select>
      </div>
    </div>

    <!-- 来源引用 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.citationEnabled') }}</label>
        <p class="desc">{{ $t('agent.editor.citationEnabledDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.citation_enabled" />
      </div>
    </div>

    <!-- ReRank 模型（启用知识库或 search_knowledge 工具时显示） -->
    <div v-if="showRerankModelField" class="setting-row" data-agent-field="rerank_model"
      :class="{ 'setting-row--field-highlight': highlightedField === 'rerank_model' }">
      <div class="setting-info">
        <label>
          {{ $t('agent.editor.rerankModel') }}
          <span v-if="needsRerankModel" class="required">*</span>
        </label>
        <p class="desc">
          {{ $t('agent.editor.rerankModelDesc') }}
          <template v-if="!needsRerankModel">
            <br />
            <span class="hint">{{ $t('agent.editor.rerankModelOptionalHint') }}</span>
          </template>
        </p>
      </div>
      <div class="setting-control">
        <ModelSelector model-type="Rerank" :selected-model-id="config.rerank_model_id"
          :all-models="allModels" :clearable="!needsRerankModel"
          @update:selected-model-id="(val: string) => config.rerank_model_id = val"
          @add-model="emit('addModel', 'rerank')"
          :placeholder="$t('agent.editor.rerankModelPlaceholder')" />
      </div>
    </div>

    <!-- 问题理解模型（多轮改写时，留空则复用主对话模型） -->
    <div v-if="!isAgentMode && config.multi_turn_enabled && config.enable_rewrite" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.queryUnderstandModel') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.queryUnderstandModel') }}</p>
      </div>
      <div class="setting-control">
        <ModelSelector model-type="KnowledgeQA" :selected-model-id="config.query_understand_model_id"
          :all-models="allModels" clearable
          @update:selected-model-id="(val: string) => config.query_understand_model_id = val"
          @add-model="emit('addModel', 'llm')"
          :placeholder="$t('agent.editor.queryUnderstandModelPlaceholder')" />
      </div>
    </div>

    <!-- 最大迭代次数（Agent 模式）：正数为上限，-1 为不限制 -->
    <div v-if="isAgentMode" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.maxIterations') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.maxIterations') }}</p>
      </div>
      <div class="setting-control max-tokens-control">
        <t-radio-group v-model="maxIterationsMode">
          <t-radio-button value="limit">{{ $t('agent.editor.maxIterationsLimit') }}</t-radio-button>
          <t-radio-button value="unlimited">{{ $t('agent.editor.maxIterationsUnlimited') }}</t-radio-button>
        </t-radio-group>
        <t-input-number v-if="maxIterationsMode === 'limit'" v-model="config.max_iterations" :min="2" :max="50"
          theme="column" />
      </div>
    </div>

    <!-- LLM 调用超时（Agent 模式） -->
    <div v-if="isAgentMode" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.llmCallTimeout.label') }}</label>
        <p class="desc">{{ $t('agentEditor.llmCallTimeout.desc') }}</p>
        <p class="desc-hint">{{ $t('agentEditor.llmCallTimeout.hint') }}</p>
      </div>
      <div class="setting-control">
        <t-input-number v-model="config.llm_call_timeout" :min="0" :max="3600" theme="column"
          :placeholder="$t('agentEditor.llmCallTimeout.placeholder')" clearable />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import ModelSelector from '@/components/ModelSelector.vue'
import type { CustomAgentConfig } from '@/api/agent'
import type { ModelConfig } from '@/api/model'

// 模型配置分区（需求 A）。编辑器与平台「智能体配置」共用本组件。
//
// 本分区依赖的派生状态（思考强度等级、最大 token 模式、最大迭代模式等）都带
// 副作用 setter，且部分在编辑器别处也用到，所以一律用 prop/v-model 透传 ——
// 组件里不放任何业务逻辑，保证是纯搬运。
const props = defineProps<{
  config: CustomAgentConfig
  allModels: ModelConfig[]
  isAgentMode: boolean
  highlightedField: string | null
  showRerankModelField: boolean
  needsRerankModel: boolean
  maxCompletionTokensMode: 'default' | 'custom'
  effectiveDefaultMaxCompletionTokens: number | string
  hasSelectedChatModel: boolean
  selectedChatModelCanThink: boolean
  selectedChatModelAlwaysThinks: boolean
  reasoningEffortLevel: string
  reasoningEffortOptions: readonly string[]
  levelLabelKey: (level: string) => string
  levelDescriptionKey: (level: string) => string
  maxIterationsMode: 'limit' | 'unlimited'
}>()

const emit = defineEmits<{
  (e: 'addModel', section: string): void
  (e: 'update:maxCompletionTokensMode', v: 'default' | 'custom'): void
  (e: 'update:reasoningEffortLevel', v: string): void
  (e: 'update:maxIterationsMode', v: 'limit' | 'unlimited'): void
}>()

const maxCompletionTokensMode = computed({
  get: () => props.maxCompletionTokensMode,
  set: (v: 'default' | 'custom') => emit('update:maxCompletionTokensMode', v),
})
const reasoningEffortLevel = computed({
  get: () => props.reasoningEffortLevel,
  set: (v: string) => emit('update:reasoningEffortLevel', v),
})
const maxIterationsMode = computed({
  get: () => props.maxIterationsMode,
  set: (v: 'limit' | 'unlimited') => emit('update:maxIterationsMode', v),
})
</script>

<style scoped lang="less">
@import './agentSectionStyles.less';

.reasoning-level-option {
  display: flex;
  flex-direction: column;
  line-height: 1.35;

  &__title {
    font-weight: 500;
  }

  &__hint {
    font-size: var(--app-text-sm);
    color: var(--td-text-color-placeholder);
  }
}

.desc-hint {
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
  margin: 4px 0 0;
}
</style>
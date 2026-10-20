<template>
  <div class="section-header">
    <h2>{{ $t('agentEditor.questionSuggestions.title') }}</h2>
    <p class="section-description">{{ $t('agentEditor.questionSuggestions.description') }}</p>
  </div>

  <t-tabs v-model="suggestionTab" class="suggestion-tabs">
    <t-tab-panel value="starters" :label="$t('agentEditor.questionSuggestions.startersTitle')" />
    <t-tab-panel value="followUps" :label="$t('agentEditor.questionSuggestions.followUpsTitle')" />
  </t-tabs>

  <div v-show="suggestionTab === 'starters'" class="settings-group">
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.questionSuggestions.enableStarters') }}</label>
        <p class="desc">{{ $t('agentEditor.questionSuggestions.enableStartersDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="qs.starters.enabled"
          :aria-label="$t('agentEditor.questionSuggestions.enableStarters')" />
      </div>
    </div>

    <div v-if="qs.starters.enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.questionSuggestions.sourceMode') }}</label>
      </div>
      <div class="setting-control">
        <t-select v-model="qs.starters.mode"
          :options="starterSuggestionModeOptions" />
      </div>
    </div>

    <div v-if="qs.starters.enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.questionSuggestions.count') }}</label>
      </div>
      <div class="setting-control">
        <t-input-number v-model="qs.starters.count" :min="1" :max="8"
          theme="column" />
      </div>
    </div>

    <div
      v-if="qs.starters.enabled && ['curated', 'hybrid'].includes(qs.starters.mode)"
      class="setting-row setting-row-vertical">
      <div class="setting-info">
        <div class="setting-info-header setting-info-header--inline">
          <label>{{ $t('agentEditor.questionSuggestions.curatedItems') }}</label>
          <span class="curated-items-count">
            {{ qs.starters.items.length }}/8
          </span>
        </div>
        <p class="desc">{{ $t('agentEditor.questionSuggestions.curatedItemsDesc') }}</p>
      </div>
      <div class="setting-control setting-control-full">
        <div class="suggested-prompts-list">
          <div v-for="(_prompt, index) in qs.starters.items" :key="index"
            class="prompt-item">
            <t-input v-model="qs.starters.items[index]" :maxlength="200" />
            <t-button variant="text" theme="danger" shape="square" :aria-label="$t('common.delete')"
              @click="removeStarterSuggestion(Number(index))">
              <t-icon name="delete" />
            </t-button>
          </div>
          <t-button variant="dashed" :disabled="qs.starters.items.length >= 8"
            @click="addStarterSuggestion">
            <template #icon><t-icon name="add" /></template>
            {{ $t('agentEditor.questionSuggestions.addItem') }}
          </t-button>
        </div>
      </div>
    </div>
  </div>

  <div v-show="suggestionTab === 'followUps'" class="settings-group">
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.questionSuggestions.enableFollowUps') }}</label>
        <p class="desc">{{ $t('agentEditor.questionSuggestions.enableFollowUpsDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="qs.follow_ups.enabled"
          :aria-label="$t('agentEditor.questionSuggestions.enableFollowUps')" />
      </div>
    </div>

    <template v-if="qs.follow_ups.enabled">
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('agentEditor.questionSuggestions.sourceMode') }}</label>
        </div>
        <div class="setting-control">
          <t-select v-model="qs.follow_ups.mode"
            :options="followUpSuggestionModeOptions" />
        </div>
      </div>

      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('agentEditor.questionSuggestions.count') }}</label>
        </div>
        <div class="setting-control">
          <t-input-number v-model="qs.follow_ups.count" :min="1" :max="5"
            theme="column" />
        </div>
      </div>

      <div v-if="qs.follow_ups.mode !== 'knowledge'" class="setting-row">
        <div class="setting-info">
          <label>{{ $t('agentEditor.questionSuggestions.model') }}</label>
          <p class="desc">{{ $t('agentEditor.questionSuggestions.modelDesc') }}</p>
        </div>
        <div class="setting-control">
          <ModelSelector model-type="KnowledgeQA"
            :selected-model-id="qs.follow_ups.model_id"
            :all-models="allModels" clearable
            @update:selected-model-id="(val: string) => qs.follow_ups.model_id = val"
            @add-model="emit('addModel', 'summary')" />
        </div>
      </div>

      <div class="suggestion-advanced-divider">
        <span>{{ $t('agentEditor.questionSuggestions.advancedSettings') }}</span>
      </div>

      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('agentEditor.questionSuggestions.contextTurns') }}</label>
        </div>
        <div class="setting-control">
          <t-input-number v-model="qs.follow_ups.max_context_turns" :min="1"
            :max="5" theme="column" />
        </div>
      </div>

      <div class="setting-row setting-row-vertical">
        <div class="setting-info">
          <label>{{ $t('agentEditor.questionSuggestions.categories') }}</label>
        </div>
        <div class="setting-control setting-control-full">
          <t-checkbox-group v-model="qs.follow_ups.categories"
            :options="followUpCategoryOptions" />
        </div>
      </div>

      <div class="setting-row setting-row-vertical">
        <div class="setting-info">
          <label>{{ $t('agentEditor.questionSuggestions.instruction') }}</label>
        </div>
        <div class="setting-control setting-control-full">
          <t-textarea v-model="qs.follow_ups.additional_instruction"
            :placeholder="$t('agentEditor.questionSuggestions.instructionPlaceholder')" :maxlength="2000"
            :autosize="{ minRows: 3, maxRows: 8 }" />
        </div>
      </div>

      <div class="setting-row setting-row-vertical">
        <div class="setting-info">
          <label>{{ $t('agentEditor.questionSuggestions.displayRules') }}</label>
        </div>
        <div class="setting-control setting-control-full">
          <div class="suggestion-checkboxes">
            <t-checkbox v-model="qs.follow_ups.suppress_on_fallback">{{
              $t('agentEditor.questionSuggestions.suppressFallback') }}</t-checkbox>
            <t-checkbox v-model="qs.follow_ups.suppress_when_answer_asks_question">{{
              $t('agentEditor.questionSuggestions.suppressQuestion') }}</t-checkbox>
            <t-checkbox v-model="qs.follow_ups.knowledge_fallback">{{
              $t('agentEditor.questionSuggestions.knowledgeFallback') }}</t-checkbox>
            <t-checkbox v-model="qs.follow_ups.allow_regenerate">{{
              $t('agentEditor.questionSuggestions.allowRegenerate') }}</t-checkbox>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ModelSelector from '@/components/ModelSelector.vue'
import type { CustomAgentConfig } from '@/api/agent'
import type { ModelConfig } from '@/api/model'

// 问题推荐分区（需求 C）。编辑器与平台「智能体配置」共用本组件。
//
// suggestionTab 与下面两个选项列表原本就是本分区专用的，随模板一起搬了进来；
// allModels / addModel 是编辑器级的东西，用 prop / emit 透传。
const props = defineProps<{
  config: CustomAgentConfig
  allModels: ModelConfig[]
}>()

const emit = defineEmits<{ (e: 'addModel', section: string): void }>()

const { t } = useI18n()

// question_suggestions 在后端是可空字段（新建表单/旧数据都可能是 undefined），
// 而整个分区都建立在它之上。这里给一个非空视图，缺字段时按空结构兜底 ——
// 直接读 config.question_suggestions.starters 会在未初始化时抛错。
const qs = computed(() => props.config.question_suggestions!)

const suggestionTab = ref<'starters' | 'followUps'>('starters')

const starterSuggestionModeOptions = computed(() => [
  { value: 'curated', label: t('agentEditor.questionSuggestions.modeCurated') },
  { value: 'knowledge', label: t('agentEditor.questionSuggestions.modeKnowledge') },
  { value: 'hybrid', label: t('agentEditor.questionSuggestions.modeHybrid') },
])
const followUpSuggestionModeOptions = computed(() => [
  { value: 'generated', label: t('agentEditor.questionSuggestions.modeGenerated') },
  { value: 'knowledge', label: t('agentEditor.questionSuggestions.modeKnowledge') },
  { value: 'hybrid', label: t('agentEditor.questionSuggestions.modeHybrid') },
])
const followUpCategoryOptions = computed(() => [
  { value: 'clarify', label: t('agentEditor.questionSuggestions.categoryClarify') },
  { value: 'deepen', label: t('agentEditor.questionSuggestions.categoryDeepen') },
  { value: 'action', label: t('agentEditor.questionSuggestions.categoryAction') },
])

const addStarterSuggestion = () => {
  const items = qs.value.starters.items
  if (items.length < 8) items.push('')
}

const removeStarterSuggestion = (index: number) => {
  qs.value.starters.items.splice(index, 1)
}
</script>

<style scoped lang="less">
@import './agentSectionStyles.less';

.suggestion-advanced-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 20px 0 4px;
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);

  &::before,
  &::after {
    content: '';
    flex: 1;
    height: 1px;
    background: var(--td-component-stroke);
  }
}

.curated-items-count {
  font-family: var(--app-font-family-mono);
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
}

.suggestion-checkboxes {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
}

.suggested-prompts-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.prompt-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
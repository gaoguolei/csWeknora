<template>
  <div class="memory-workspace-settings">
    <div class="section-header">
      <h2>{{ t('memoryWorkspaceSettings.title') }}</h2>
      <p class="section-description">{{ t('memoryWorkspaceSettings.description') }}</p>
    </div>

    <!-- The switch defaults to off because memory retains what users say
         across sessions. That makes the feature easy to miss, so the intro
         states plainly what turning it on does. -->
    <div class="intro">
      <t-icon name="info-circle" class="intro-icon" />
      <div>
        <p class="intro-title">{{ t('memoryWorkspaceSettings.introTitle') }}</p>
        <p class="intro-desc">{{ t('memoryWorkspaceSettings.introDescription') }}</p>
      </div>
    </div>

    <!-- Long-term memory is a platform-wide setting owned by the super admin and
         its nav entry is super-admin-only (SUPER_ADMIN_ONLY_SECTIONS), so in
         practice only super/operations admins land here. Keep the banner as a
         fallback in case the page is reached through another host. -->
    <div v-if="!canEdit" class="platform-readonly-banner">
      <t-icon name="info-circle" class="intro-icon" />
      <span>{{ t('memoryWorkspaceSettings.platformManagedReadonly') }}</span>
    </div>

    <div class="settings-group">
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.enableLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.enableDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-switch v-model="config.enabled" :disabled="!canEdit" @change="debouncedSave" />
        </div>
      </div>

      <div v-if="config.enabled" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.writeModeLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.writeModeDescription') }}</p>
          <p class="desc hint">
            {{
              config.write_mode === 'auto'
                ? t('memoryWorkspaceSettings.writeModeAutoHint')
                : t('memoryWorkspaceSettings.writeModeExplicitHint')
            }}
          </p>
        </div>
        <div class="setting-control">
          <t-radio-group v-model="config.write_mode" :disabled="!canEdit" @change="debouncedSave">
            <t-radio-button value="explicit_only">
              {{ t('memoryWorkspaceSettings.writeModeExplicit') }}
            </t-radio-button>
            <t-radio-button value="auto">
              {{ t('memoryWorkspaceSettings.writeModeAuto') }}
            </t-radio-button>
          </t-radio-group>
        </div>
      </div>

      <div v-if="config.enabled && config.write_mode === 'auto'" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.extractModelLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.extractModelDescription') }}</p>
        </div>
        <div class="setting-control" style="min-width: 280px">
          <ModelSelector
            model-type="KnowledgeQA"
            :selected-model-id="config.extract_model_id"
            :disabled="!canEdit"
            :show-add-model="authStore.isSystemAdmin"
            @update:selected-model-id="handleModelChange"
            @add-model="handleAddModel('chat')"
          />
        </div>
      </div>

      <div v-if="config.enabled && config.write_mode === 'auto'" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.extractDelayLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.extractDelayDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-input-number
            v-model="config.extract_delay_seconds"
            :min="5"
            :max="3600"
            :step="15"
            suffix="s"
            :disabled="!canEdit"
            @change="debouncedSave"
          />
        </div>
      </div>

      <div v-if="config.enabled && config.write_mode === 'auto'" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.extractMinIntervalLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.extractMinIntervalDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-input-number
            v-model="config.extract_min_interval_seconds"
            :min="0"
            :max="86400"
            :step="60"
            suffix="s"
            :disabled="!canEdit"
            @change="debouncedSave"
          />
        </div>
      </div>

      <div v-if="config.enabled" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.vectorRecallLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.vectorRecallDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-switch v-model="config.vector_recall" :disabled="!canEdit" @change="debouncedSave" />
        </div>
      </div>

      <div v-if="config.enabled && config.vector_recall" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.embeddingModelLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.embeddingModelDescription') }}</p>
        </div>
        <div class="setting-control" style="min-width: 280px">
          <ModelSelector
            model-type="Embedding"
            :selected-model-id="config.embedding_model_id"
            :disabled="!canEdit"
            :clearable="true"
            :show-add-model="authStore.isSystemAdmin"
            @update:selected-model-id="handleEmbeddingModelChange"
            @add-model="handleAddModel('embedding')"
          />
        </div>
      </div>

      <div v-if="config.enabled" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.conditioningLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.conditioningDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-switch
            v-model="config.retrieval_conditioning"
            :disabled="!canEdit"
            @change="debouncedSave"
          />
        </div>
      </div>

      <div v-if="config.enabled && config.write_mode === 'auto'" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.interestThresholdLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.interestThresholdDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-input-number
            v-model="config.interest_threshold"
            :min="1"
            :max="20"
            :step="1"
            :disabled="!canEdit"
            @change="debouncedSave"
          />
        </div>
      </div>

      <div v-if="config.enabled && config.write_mode === 'auto'" class="setting-row instructions-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.instructionsLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.instructionsDescription') }}</p>
        </div>
        <div class="setting-control instructions-control">
          <t-textarea
            v-model="config.extract_instructions"
            :autosize="{ minRows: 3, maxRows: 8 }"
            :maxlength="1000"
            :disabled="!canEdit"
            :placeholder="t('memoryWorkspaceSettings.instructionsPlaceholder')"
            @blur="debouncedSave"
          />
        </div>
      </div>

      <div v-if="config.enabled" class="setting-row">
        <div class="setting-info">
          <label>{{ t('memoryWorkspaceSettings.maxItemsLabel') }}</label>
          <p class="desc">{{ t('memoryWorkspaceSettings.maxItemsDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-input-number
            v-model="config.max_items"
            :min="10"
            :max="2000"
            :step="10"
            :disabled="!canEdit"
            @change="debouncedSave"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import ModelSelector from '@/components/ModelSelector.vue'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useChatResourcesStore } from '@/stores/chatResources'
import { selectInitialModelId } from '@/utils/modelDefaults'
import { getTenantMemoryConfig, type MemoryConfig } from '@/api/memory'
import { updateSystemSetting } from '@/api/system'

const { t } = useI18n()
const authStore = useAuthStore()
const uiStore = useUIStore()
const chatResources = useChatResourcesStore()

const config = reactive<MemoryConfig>({
  enabled: false,
  write_mode: 'explicit_only',
  extract_model_id: '',
  max_items: 200,
  extract_delay_seconds: 90,
  extract_min_interval_seconds: 300,
  extract_instructions: '',
  interest_threshold: 3,
  retrieval_conditioning: true,
  embedding_model_id: '',
  vector_recall: true,
})
const isInitializing = ref(true)

// Long-term memory is a platform-wide setting now: only the super admin can
// edit it, and the settings nav entry is gated to isSystemAdmin (operations
// admins inherit it), so members never reach this page. All rows bind
// :disabled="!canEdit" as a defensive fallback.
const canEdit = computed(() => authStore.isSystemAdmin)

const loadConfig = async () => {
  try {
    const response = await getTenantMemoryConfig()
    if (response.data) {
      config.enabled = response.data.enabled ?? false
      config.write_mode = response.data.write_mode === 'auto' ? 'auto' : 'explicit_only'
      config.extract_model_id = response.data.extract_model_id || ''
      config.max_items = response.data.max_items || 200
      config.extract_delay_seconds = response.data.extract_delay_seconds || 90
      config.extract_min_interval_seconds = response.data.extract_min_interval_seconds || 300
      config.extract_instructions = response.data.extract_instructions || ''
      config.interest_threshold = response.data.interest_threshold || 3
      config.retrieval_conditioning = response.data.retrieval_conditioning !== false
      config.embedding_model_id = response.data.embedding_model_id || ''
      config.vector_recall = response.data.vector_recall !== false
    }
    // Pre-select the platform default models when this workspace has not
    // pinned one yet, so members/admins see the superadmin's default chat /
    // embedding model already chosen (mirrors knowledge-base creation).
    await applyDefaultModelsIfEmpty()
  } catch (error: any) {
    console.error('Failed to load memory config:', error)
  } finally {
    // Give the switches a tick to settle so binding the loaded values does not
    // immediately fire a save.
    setTimeout(() => {
      isInitializing.value = false
    }, 100)
  }
}

const saveConfig = async () => {
  try {
    // Persist the whole config as a JSON blob in the platform-global system
    // setting; every workspace reads the same value through memory.Service.
    await updateSystemSetting('memory.global_config', JSON.stringify({ ...config }))
    MessagePlugin.success(t('memoryWorkspaceSettings.toasts.saveSuccess'))
  } catch (error: any) {
    MessagePlugin.error(
      t('memoryWorkspaceSettings.toasts.saveFailed', { message: error?.message || '' }),
    )
  }
}

let saveTimer: number | null = null
const debouncedSave = () => {
  if (isInitializing.value || !canEdit.value) return
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = window.setTimeout(() => {
    saveConfig().catch(() => {})
  }, 500)
}

const handleModelChange = (modelId: string) => {
  config.extract_model_id = modelId
  debouncedSave()
}

const handleEmbeddingModelChange = (modelId: string) => {
  config.embedding_model_id = modelId || ''
  debouncedSave()
}

const handleAddModel = (subSection: 'chat' | 'embedding') => {
  uiStore.openSettings('models', subSection)
  window.dispatchEvent(
    new CustomEvent('settings-nav', { detail: { section: 'models', subsection: subSection } }),
  )
}

const applyDefaultModelsIfEmpty = async () => {
  try {
    await chatResources.ensureModels()
  } catch {
    return
  }
  const models = chatResources.allModels || []
  if (!config.extract_model_id) {
    const fallback = selectInitialModelId(models, 'KnowledgeQA')
    if (fallback) config.extract_model_id = fallback
  }
  if (config.vector_recall && !config.embedding_model_id) {
    const fallback = selectInitialModelId(models, 'Embedding')
    if (fallback) config.embedding_model_id = fallback
  }
}

onMounted(loadConfig)
</script>

<style lang="less" scoped>
@import (reference) '@/components/css/settings-section.less';

.memory-workspace-settings {
  width: 100%;
}

.section-header {
  .settings-section-header();
}

.intro {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 14px 16px;
  margin-bottom: 8px;
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-secondarycontainer);
}

.intro-icon {
  color: var(--td-brand-color);
  margin-top: 2px;
  flex-shrink: 0;
}

.intro-title {
  margin: 0 0 4px 0;
  font-size: var(--app-text-base);
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.intro-desc {
  margin: 0;
  font-size: var(--app-text-md);
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.platform-readonly-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  margin-bottom: 8px;
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-secondarycontainer);
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
}

.platform-readonly-banner .intro-icon {
  color: var(--td-brand-color);
  flex-shrink: 0;
}

.settings-group {
  display: flex;
  flex-direction: column;
}

.setting-row {
  .setting-row();
}

.setting-info {
  .setting-info();

  .hint {
    margin-top: 4px !important;
    color: var(--td-text-color-placeholder);
  }
}

.setting-control {
  .setting-control();
}

// The custom prompt needs room to read, so this row stacks instead of putting a
// paragraph of rules into a narrow right-hand column.
.instructions-row {
  flex-direction: column;
  align-items: stretch;

  .setting-info {
    max-width: 100%;
    padding-right: 0;
    margin-bottom: 10px;
  }
}

.instructions-control {
  width: 100%;
  justify-content: stretch;
}
</style>

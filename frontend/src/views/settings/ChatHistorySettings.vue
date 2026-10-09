<template>
  <div class="chat-history-settings">
    <div class="section-header">
      <h2>{{ t('chatHistorySettings.title') }}</h2>
      <p class="section-description">{{
        canManage ? t('chatHistorySettings.description') : t('chatHistorySettings.descriptionViewOnly')
      }}</p>
    </div>

    <!-- 「启用消息索引」是全局开关（系统设置 chat_history.indexing_enabled），
         「Embedding 模型」也是全局设置（chat_history.embedding_model_id）：
         两者均仅超级管理员可见可改，一次设置对所有空间生效；各空间的知识库在首次索引时自动创建。
         其他人（含普通空间管理员）只能看到下方的只读索引统计。 -->
    <div v-if="canManage && statsLoaded" class="settings-group">
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ t('chatHistorySettings.enableLabel') }}</label>
          <p class="desc">{{ t('chatHistorySettings.enableDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-switch
            v-model="globalEnabled"
            :disabled="savingSwitch"
            @change="handleEnabledChange"
          />
        </div>
      </div>

      <!-- Embedding 模型选择（全局；所有空间共用，选定即对所有空间生效） -->
      <div v-if="globalEnabled" class="setting-row">
        <div class="setting-info">
          <label>{{ t('chatHistorySettings.embeddingModelLabel') }}</label>
          <p class="desc">{{ t('chatHistorySettings.embeddingModelDescription') }}</p>
          <p v-if="modelLocked" class="desc warning-text">
            {{ t('chatHistorySettings.embeddingModelLocked') }}
          </p>
        </div>
        <div class="setting-control" style="min-width: 280px;">
          <ModelSelector
            model-type="Embedding"
            :selected-model-id="localEmbeddingModelId"
            :disabled="modelLocked"
            @update:selected-model-id="handleModelChange"
          />
        </div>
      </div>
    </div>

    <!-- 统计信息：只要全局开关开启且已选择全局 Embedding 模型，所有空间（含新建空间）
         都视为已启用；新建空间在首条消息索引时自动创建各自的知识库，此处先显示 0。
         仅当全局开关关闭时，所有人（含超管）才显示「消息索引未使用」。 -->
    <div v-if="statsLoaded" class="stats-section">
      <h3 class="stats-title">{{ t('chatHistorySettings.statsTitle') }}</h3>
      <div v-if="globalEnabled && stats && stats.embedding_model_id" class="stats-grid">
        <div class="stat-card">
          <div class="stat-value">{{ stats.indexed_message_count || 0 }}</div>
          <div class="stat-label">{{ t('chatHistorySettings.statsIndexedMessages') }}</div>
        </div>
      </div>
      <div v-else-if="globalEnabled && canManage" class="stats-empty">
        <p class="stats-empty-title">{{ t('chatHistorySettings.statsNotConfigured') }}</p>
        <p class="stats-empty-desc">{{ t('chatHistorySettings.statsNotConfiguredDesc') }}</p>
      </div>
      <div v-else class="stats-empty">
        <p class="stats-empty-title">{{ t('chatHistorySettings.statsUnused') }}</p>
        <p class="stats-empty-desc">{{ t('chatHistorySettings.statsUnusedDesc') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import ModelSelector from '@/components/ModelSelector.vue'
import { useAuthStore } from '@/stores/auth'
import { updateSystemSetting } from '@/api/system'
import {
  getChatHistoryKBStats,
  type ChatHistoryKBStats,
} from '@/api/chat-history'

const { t } = useI18n()
const authStore = useAuthStore()

// Only the platform super admin sees the global enable switch and the
// per-workspace embedding model; everyone else gets read-only statistics.
const canManage = computed(() => authStore.isSystemAdmin)

// The platform master switch. Its value is surfaced to every viewer through
// the stats endpoint (system settings APIs are SystemAdmin-only), so this
// ref is seeded from stats.global_enabled and stays backend-authoritative.
const globalEnabled = ref(false)
const savingSwitch = ref(false)

const localEmbeddingModelId = ref('')
const stats = ref<ChatHistoryKBStats | null>(null)
const statsLoaded = ref(false)

// Whether the embedding model is locked (has indexed messages — cannot change)
const modelLocked = ref(false)

// Load stats (also carries the global switch state and the global embedding
// model for every viewer). The model dropdown is seeded from here so it shows
// the same selection in every workspace.
const loadStats = async () => {
  try {
    const response = await getChatHistoryKBStats()
    if (response.data) {
      stats.value = response.data
      globalEnabled.value = response.data.global_enabled === true
      localEmbeddingModelId.value = response.data.embedding_model_id || ''
      // Lock model if there are indexed messages
      modelLocked.value = response.data.has_indexed_messages === true
    }
  } catch (error: any) {
    console.error('Failed to load chat history stats:', error)
  } finally {
    statsLoaded.value = true
  }
}

// Toggle the platform-wide switch. t-switch already flipped globalEnabled
// optimistically; on failure we roll it back. A stats reload afterwards
// re-syncs the UI with the backend's authoritative value.
const handleEnabledChange = async (value: boolean) => {
  savingSwitch.value = true
  try {
    await updateSystemSetting('chat_history.indexing_enabled', value)
    MessagePlugin.success(t('chatHistorySettings.toasts.saveSuccess'))
  } catch (error: any) {
    globalEnabled.value = !value
    console.error('Failed to update message indexing switch:', error)
    const errorMessage = error?.message || 'Unknown error'
    MessagePlugin.error(t('chatHistorySettings.toasts.saveFailed', { message: errorMessage }))
  } finally {
    savingSwitch.value = false
  }
  await loadStats()
}

// Save the platform-global embedding model. Every workspace shares this
// selection; each workspace's hidden KB is auto-created lazily on first
// indexing. The platform switch still has the final say on whether indexing runs.
const saveModel = async () => {
  try {
    await updateSystemSetting('chat_history.embedding_model_id', localEmbeddingModelId.value)
    MessagePlugin.success(t('chatHistorySettings.toasts.saveSuccess'))
    loadStats()
  } catch (error: any) {
    console.error('Failed to save chat history embedding model:', error)
    const errorMessage = error?.message || 'Unknown error'
    MessagePlugin.error(t('chatHistorySettings.toasts.saveFailed', { message: errorMessage }))
  }
}

const handleModelChange = (modelId: string) => {
  localEmbeddingModelId.value = modelId
  saveModel()
}

// Init
onMounted(async () => {
  await loadStats()
})
</script>

<style lang="less" scoped>
@import (reference) '@/components/css/settings-section.less';

.chat-history-settings {
  width: 100%;
}

.section-header {
  .settings-section-header();
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  .setting-row();
}

.setting-info {
  .setting-info();
}

.warning-text {
  color: var(--td-warning-color) !important;
  margin-top: 4px !important;
}

.setting-control {
  .setting-control();
}

// Stats section
.stats-section {
  margin-top: 32px;
  padding-top: 24px;
  border-top: 1px solid var(--td-component-stroke);
}

.stats-title {
  font-size: var(--app-text-xl);
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0 0 16px 0;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.stat-card {
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--app-radius-md);
  padding: 20px;
  text-align: center;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--td-brand-color);
  margin-bottom: 4px;
}

.stat-label {
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
}

.stats-empty {
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--app-radius-md);
  padding: 24px;
  text-align: center;
}

.stats-empty-title {
  font-size: var(--app-text-base);
  font-weight: 500;
  color: var(--td-text-color-secondary);
  margin: 0 0 4px 0;
}

.stats-empty-desc {
  font-size: var(--app-text-md);
  color: var(--td-text-color-placeholder);
  margin: 0;
}
</style>

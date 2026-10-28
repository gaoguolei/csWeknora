<template>
  <div class="agent-defaults-settings" :class="{ 'agent-defaults-settings--readonly': !canEdit }">
    <div class="section-header">
      <h2>{{ $t('agentDefaults.title') }}</h2>
      <p class="section-description">{{ $t('agentDefaults.description') }}</p>
    </div>

    <div v-if="!canEdit" class="agent-defaults-readonly-banner">
      <t-icon name="lock-on" />
      <span>{{ $t('agentDefaults.readonlyBanner') }}</span>
    </div>

    <t-loading :loading="loading" size="small">
      <div v-if="ready" class="agent-defaults-content">
        <!-- A 模型配置（复用编辑器 AgentModelSettings） -->
        <div v-show="activeSection === 'model'" class="agent-defaults-block">
          <AgentModelSettings
            :config="cfg"
            :all-models="allModels"
            :is-agent-mode="true"
            :highlighted-field="null"
            :show-rerank-model-field="true"
            :needs-rerank-model="false"
            v-model:max-completion-tokens-mode="maxCompletionTokensMode"
            :effective-default-max-completion-tokens="DEFAULT_MAX_COMPLETION_TOKENS"
            :has-selected-chat-model="!!cfg.model_id"
            :selected-chat-model-can-think="selectedChatModelCanThink"
            :selected-chat-model-always-thinks="selectedChatModelAlwaysThinks"
            v-model:reasoning-effort-level="reasoningEffortLevel"
            :reasoning-effort-options="reasoningEffortOptions"
            :level-label-key="levelLabelKey"
            :level-description-key="levelDescriptionKey"
            v-model:max-iterations-mode="maxIterationsMode"
            @add-model="onAddModel"
          />
        </div>

        <!-- B 多轮对话 -->
        <div v-show="activeSection === 'conversation'" class="agent-defaults-block">
          <AgentConversationSettings :config="cfg" :is-agent-mode="true" :has-knowledge-base="true" />
        </div>

        <!-- E 检索策略 -->
        <div v-show="activeSection === 'retrieval'" class="agent-defaults-block">
          <AgentRetrievalSettings :config="cfg" :is-agent-mode="true" :has-faq-knowledge-base="true" />
        </div>

        <!-- F 网络搜索 -->
        <div v-show="activeSection === 'websearch'" class="agent-defaults-block">
          <AgentWebSearchSettings :config="cfg" :web-search-provider-list="webSearchProviderList" />
        </div>

        <!-- G 附件上传 -->
        <div v-show="activeSection === 'multimodal'" class="agent-defaults-block">
          <AgentAttachmentSettings :config="cfg" :all-models="allModels"
            :image-storage-options="imageStorageOptions" @add-model="onAddModel" />
        </div>
      </div>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { getSystemSetting, updateSystemSetting } from '@/api/system'
import { listModels, type ModelConfig } from '@/api/model'
import { useAuthStore } from '@/stores/auth'
import { useEditorResourcesStore } from '@/stores/editorResources'
import { useChatResourcesStore } from '@/stores/chatResources'
import {
  levelFromLegacy,
  levelEnablesThinking,
  levelLabelKey,
  levelDescriptionKey,
  modelCanThink,
  modelCannotDisableThinking,
  optionsFor,
  type ReasoningLevel,
} from '@/utils/reasoningEffort'
import type { CustomAgentConfig } from '@/api/agent'
import AgentModelSettings from '@/views/agent/settings/AgentModelSettings.vue'
import AgentConversationSettings from '@/views/agent/settings/AgentConversationSettings.vue'
import AgentRetrievalSettings from '@/views/agent/settings/AgentRetrievalSettings.vue'
import AgentWebSearchSettings from '@/views/agent/settings/AgentWebSearchSettings.vue'
import AgentAttachmentSettings from '@/views/agent/settings/AgentAttachmentSettings.vue'

// 与后端 types.AgentDefaultsKey 对应
const AGENT_DEFAULTS_KEY = 'agent.default_config'

// 「跟随系统默认」时展示的参考值。平台默认页不区分沙箱，取标准值。
const DEFAULT_MAX_COMPLETION_TOKENS = 4096

// 只保存平台托管的字段。其余（基本信息、提示词、知识库检索）是用户自己的，
// 存进去既没用也会让人误会。这份清单必须与后端 types.AgentDefaultsConfig 对齐。
const PLATFORM_MANAGED_KEYS = [
  'model_id', 'rerank_model_id', 'query_understand_model_id', 'temperature',
  'max_completion_tokens', 'thinking', 'reasoning_effort', 'citation_enabled',
  'max_iterations', 'llm_call_timeout',
  'multi_turn_enabled', 'history_turns', 'retain_retrieval_history', 'enable_rewrite',
  'enable_query_expansion', 'embedding_top_k', 'keyword_threshold', 'vector_threshold',
  'rerank_top_k', 'rerank_threshold', 'faq_priority_enabled', 'faq_direct_answer_threshold',
  'faq_score_boost', 'data_analysis_enabled',
  'web_search_enabled', 'web_search_provider_id', 'web_search_max_results',
  'web_fetch_enabled', 'web_fetch_top_n',
  'image_upload_enabled', 'vlm_model_id', 'attachment_image_understanding',
  'attachment_ocr_max_pages', 'image_storage_provider', 'audio_upload_enabled',
  'asr_model_id', 'attachment_parse_wait_timeout_sec', 'chat_parser_engine_rules',
] as const

withDefaults(defineProps<{ activeSection?: string }>(), { activeSection: 'model' })

const { t } = useI18n()
const authStore = useAuthStore()
const editorResources = useEditorResourcesStore()
const chatResources = useChatResourcesStore()

// 平台默认配置由超级管理员维护；运营管理员继承 isSystemAdmin，因此也能进。
const canEdit = computed(() => authStore.isSystemAdmin)

const loading = ref(false)
const ready = ref(false)
const allModels = ref<ModelConfig[]>([])

// 与 CustomAgentConfig 同形，字段名与后端 JSON tag 一致 —— 因此不需要
// camelCase 转换层，存进 system_settings 的就是后端直接能解的形状。
// 每个字段带一个「中性」初值，保证复用的分区组件拿到的是完整对象。
function makeDefaults(): CustomAgentConfig {
  return {
    agent_mode: 'smart-reasoning',
    system_prompt: '',
    context_template: '',
    model_id: '',
    rerank_model_id: '',
    temperature: 0.7,
    max_completion_tokens: 0,
    thinking: false,
    reasoning_effort: 'off',
    citation_enabled: true,
    max_iterations: 10,
    llm_call_timeout: 120,
    allowed_tools: [],
    mcp_selection_mode: 'none',
    mcp_services: [],
    mcp_auth_wait_timeout: 600,
    skills_selection_mode: 'none',
    selected_skills: [],
    sandbox_config_id: '',
    kb_selection_mode: 'all',
    knowledge_bases: [],
    retrieve_kb_only_when_mentioned: false,
    retain_retrieval_history: false,
    image_upload_enabled: false,
    vlm_model_id: '',
    audio_upload_enabled: false,
    asr_model_id: '',
    image_storage_provider: '',
    supported_file_types: [],
    chat_parser_engine_rules: [],
    attachment_image_understanding: false,
    attachment_ocr_max_pages: 0,
    attachment_parse_wait_timeout_sec: 0,
    data_analysis_enabled: false,
    faq_priority_enabled: true,
    faq_direct_answer_threshold: 0.9,
    faq_score_boost: 1.2,
    web_search_enabled: false,
    web_search_max_results: 5,
    web_fetch_enabled: false,
    web_fetch_top_n: 3,
    multi_turn_enabled: true,
    history_turns: 5,
    embedding_top_k: 10,
    keyword_threshold: 0.3,
    vector_threshold: 0.5,
    rerank_top_k: 5,
    rerank_threshold: 0.5,
    enable_query_expansion: true,
    enable_rewrite: true,
    rewrite_prompt_system: '',
    rewrite_prompt_user: '',
    fallback_strategy: 'model',
    fallback_response: '',
    fallback_prompt: '',
  } as CustomAgentConfig
}

const cfg = reactive<CustomAgentConfig>(makeDefaults())

// ---- 复用的分区组件需要的派生值（平台默认页的口径比编辑器简单）----
const selectedChatModel = computed(() => allModels.value.find(m => m.id === cfg.model_id))
const selectedChatModelCanThink = computed(() => modelCanThink(selectedChatModel.value?.capabilities))
const selectedChatModelAlwaysThinks = computed(
  () => modelCannotDisableThinking(selectedChatModel.value?.capabilities),
)
const reasoningEffortOptions = computed<ReasoningLevel[]>(
  () => optionsFor(selectedChatModel.value?.capabilities),
)

const maxCompletionTokensMode = computed({
  get: () => ((cfg.max_completion_tokens ?? 0) > 0 ? 'custom' : 'default'),
  set: (mode: 'default' | 'custom') => {
    if (mode === 'default') {
      cfg.max_completion_tokens = 0
      return
    }
    if (!cfg.max_completion_tokens) {
      cfg.max_completion_tokens = DEFAULT_MAX_COMPLETION_TOKENS
    }
  },
})

const reasoningEffortLevel = computed<ReasoningLevel>({
  get: () => levelFromLegacy(cfg.thinking, cfg.reasoning_effort),
  set: (level: ReasoningLevel) => {
    cfg.reasoning_effort = level
    cfg.thinking = levelEnablesThinking(level)
  },
})

// 与编辑器一致：-1 表示不限制。平台默认页不需要"记住上一个有限值"。
const maxIterationsMode = computed({
  get: () => ((cfg.max_iterations ?? 0) < 0 ? 'unlimited' : 'limit'),
  set: (mode: 'limit' | 'unlimited') => {
    const current = cfg.max_iterations ?? 0
    cfg.max_iterations = mode === 'unlimited' ? -1 : (current > 1 ? current : 10)
  },
})

const storageEngineStatus = computed(() => editorResources.storageStatus || [])
const imageStorageOptions = computed(() => {
  const statusMap: Record<string, boolean> = {}
  for (const e of storageEngineStatus.value) {
    statusMap[e.name] = e.available
  }
  return [
    { value: 'local', label: t('settings.storage.engineLocal'), disabled: false },
    { value: 'minio', label: 'MinIO', disabled: statusMap.minio === false },
    { value: 'cos', label: t('settings.storage.engineCos'), disabled: statusMap.cos === false },
    { value: 'tos', label: t('settings.storage.engineTos'), disabled: statusMap.tos === false },
    { value: 's3', label: 'Amazon S3', disabled: statusMap.s3 === false },
    { value: 'oss', label: t('settings.storage.engineOss'), disabled: statusMap.oss === false },
  ]
})

const webSearchProviderList = computed(() =>
  (chatResources.webSearchProviders || []).map((p: any) => ({
    id: String(p.id ?? ''),
    name: String(p.name ?? ''),
    is_default: !!p.is_default,
  })).filter((p) => p.id && p.name),
)

const onAddModel = () => {
  // 平台默认页不直接建模型，引导到「模型」设置页。
  MessagePlugin.info(t('agentDefaults.goModelSettings'))
}

// ---- 存取 ----
function toStorageShape(): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const key of PLATFORM_MANAGED_KEYS) {
    out[key] = (cfg as Record<string, unknown>)[key]
  }
  return out
}

let autosaveTimer: ReturnType<typeof setTimeout> | null = null
let lastSavedJson = ''

function clearAutosaveTimer() {
  if (autosaveTimer) {
    clearTimeout(autosaveTimer)
    autosaveTimer = null
  }
}

function scheduleAutoSave() {
  if (!canEdit.value || !ready.value) return
  clearAutosaveTimer()
  autosaveTimer = setTimeout(runAutoSave, 600)
}

async function runAutoSave() {
  autosaveTimer = null
  if (!canEdit.value || !ready.value) return
  const json = JSON.stringify(toStorageShape())
  if (json === lastSavedJson) return
  try {
    await updateSystemSetting(AGENT_DEFAULTS_KEY, json)
    lastSavedJson = json
    MessagePlugin.success(t('agentDefaults.autoSaved'))
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('agentDefaults.saveFailed'))
  }
}

watch(cfg, () => scheduleAutoSave(), { deep: true })

async function load() {
  loading.value = true
  try {
    // 未保存过时该键会抛错 —— 当作"还没有平台默认"，保留内置初值。
    const item = await getSystemSetting(AGENT_DEFAULTS_KEY)
    const raw = typeof item?.value === 'string' ? item.value : ''
    if (raw.trim()) {
      try {
        const stored = JSON.parse(raw)
        for (const key of PLATFORM_MANAGED_KEYS) {
          if (stored[key] !== undefined) {
            (cfg as Record<string, unknown>)[key] = stored[key]
          }
        }
      } catch (e) {
        console.error('agentDefaults: malformed stored JSON, using defaults', e)
      }
    }
  } catch {
    // 404 / unknown key: nothing persisted yet.
  } finally {
    ready.value = true
    loading.value = false
    lastSavedJson = JSON.stringify(toStorageShape())
  }
}

async function loadModels() {
  try {
    const resp: any = await listModels()
    allModels.value = Array.isArray(resp) ? resp : (resp?.data ?? [])
  } catch {
    allModels.value = []
  }
}

async function loadCatalogs() {
  // 与编辑器同源的缓存；失败只影响对应下拉的可选项，不影响已保存的值。
  // MCP / 工具 / 技能是用户级配置，平台默认页不维护，无需预拉对应目录。
  await Promise.allSettled([
    editorResources.ensureStorageEngine(),
    chatResources.ensureWebSearchProviders?.(),
  ])
}

onMounted(() => {
  void loadModels()
  void loadCatalogs()
  void load()
})

onUnmounted(() => {
  // 防抖还没落地的最后一次编辑，离开页面时补存。
  if (autosaveTimer) {
    clearAutosaveTimer()
    void runAutoSave()
  }
})
</script>

<style scoped lang="less">
.agent-defaults-settings {
  width: 100%;
  padding-bottom: 24px;
}

// 非超管进来是只读视图：表单内容不可交互（pointer-events 会继承进复用的子组件），
// 但左侧导航仍可切换分区。
.agent-defaults-settings--readonly .agent-defaults-content {
  pointer-events: none;
  user-select: none;
  opacity: 0.9;
}

.agent-defaults-readonly-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  padding: 10px 12px;
  border-radius: var(--app-radius-sm);
  background: var(--td-warning-color-light);
  color: var(--td-text-color-primary);
  font-size: var(--app-text-md);
}

.agent-defaults-block {
  width: 100%;
}

.section-header {
  margin-bottom: 20px;

  h2 {
    font-size: var(--app-text-3xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 6px 0;
  }

  .section-description {
    font-size: var(--app-text-base);
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  padding: 16px 0;
  border-bottom: 1px solid var(--td-component-stroke);
  min-width: 0;

  &:last-child {
    border-bottom: none;
  }
}

.setting-info {
  flex: 0 0 42%;
  max-width: 42%;
  min-width: 0;

  label {
    font-size: var(--app-text-lg);
    font-weight: 500;
    color: var(--td-text-color-primary);
    display: block;
    margin-bottom: 4px;
  }

  .desc {
    font-size: var(--app-text-md);
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.setting-control {
  flex: 1 1 58%;
  min-width: 0;
  max-width: 58%;
  display: flex;
  justify-content: flex-end;
  align-items: flex-start;
  overflow: hidden;

  :deep(.t-select),
  :deep(.t-input) {
    width: 100%;
    min-width: 0;
  }
}
</style>
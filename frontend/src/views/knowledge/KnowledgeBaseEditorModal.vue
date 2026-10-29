<template>
  <SettingsModalShell :visible="visible"
    :title="editorMode === 'create' ? $t('knowledgeEditor.titleCreate') : $t('knowledgeEditor.titleEdit')"
    v-model="currentSection" :nav-groups="navGroups" :loading="loading" :z-index="1000"
    :hide-sidebar="editorMode === 'create'"
    nav-guide="kb-editor-sidebar" nav-item-guide-prefix="kb-editor-nav" @close="modalShell.requestClose">
    <div class="content-wrapper">
      <!-- 基本信息 -->
      <div v-show="currentSection === 'basic'" class="section">
        <div v-if="formData" class="section-content">
          <!-- 建库态已由弹窗头部展示「新建认知库」，不再重复「基本信息」标题 -->
          <div v-if="editorMode !== 'create'" class="section-header">
            <h3 class="section-title">{{ $t('knowledgeEditor.basic.title') }}</h3>
            <p class="section-desc">{{ $t('knowledgeEditor.basic.description') }}</p>
          </div>
          <div class="section-body">
            <div v-if="editorMode === 'edit' && activeKbId" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.basic.kbId') }}</label>
              <p class="form-tip">{{ isPostCreateSession ? $t('knowledgeEditor.postCreateHint.followUpDesc') : $t('knowledgeEditor.basic.kbIdDesc') }}</p>
              <div class="kb-id-field">
                <code class="kb-id-value" :title="activeKbId">{{ activeKbId }}</code>
                <t-tooltip :content="$t('common.copy')" placement="top">
                  <t-button theme="default" size="small" variant="text" class="kb-id-copy"
                    @click="copyKbId">
                    <t-icon name="file-copy" />
                  </t-button>
                </t-tooltip>
              </div>
            </div>

            <div class="form-item">
              <label class="form-label required">{{ $t('knowledgeEditor.basic.typeLabel') }}</label>
              <t-radio-group
                v-model="formData.type"
                :disabled="editorMode === 'edit'"
                data-guide="kb-create-type"
              >
                <t-radio-button value="document">{{ $t('knowledgeEditor.basic.typeDocument') }}</t-radio-button>
                <t-radio-button value="faq">{{ $t('knowledgeEditor.basic.typeFAQ') }}</t-radio-button>
              </t-radio-group>
              <p class="form-tip">{{ $t('knowledgeEditor.basic.typeDescription') }}</p>
            </div>

            <!-- FAQ 索引策略（问答类型专属）：紧跟类型选择，建库与编辑都在这里
                 由用户自行设置（所有成员可改），侧栏不再单设 FAQ 分区。 -->
            <div v-if="isFAQ" class="faq-inline-settings" data-guide="kb-create-faq">
              <div class="form-item">
                <label class="form-label required">{{ $t('knowledgeEditor.faq.indexModeLabel') }}</label>
                <t-radio-group v-model="formData.faqConfig.indexMode">
                  <t-radio-button value="question_only">{{ $t('knowledgeEditor.faq.modes.questionOnly') }}</t-radio-button>
                  <t-radio-button value="question_answer">{{ $t('knowledgeEditor.faq.modes.questionAnswer') }}</t-radio-button>
                </t-radio-group>
                <p class="form-tip">{{ $t('knowledgeEditor.faq.indexModeDescription') }}</p>
              </div>
              <div class="form-item">
                <label class="form-label required">{{ $t('knowledgeEditor.faq.questionIndexModeLabel') }}</label>
                <t-radio-group v-model="formData.faqConfig.questionIndexMode">
                  <t-radio-button value="combined">{{ $t('knowledgeEditor.faq.modes.combined') }}</t-radio-button>
                  <t-radio-button value="separate">{{ $t('knowledgeEditor.faq.modes.separate') }}</t-radio-button>
                </t-radio-group>
                <p class="form-tip">{{ $t('knowledgeEditor.faq.questionIndexModeDescription') }}</p>
              </div>
            </div>

            <!-- 索引策略 (紧跟类型选择) -->
            <div v-if="!isFAQ" class="form-item">
              <label class="form-label required">{{ $t('knowledgeEditor.indexing.title') }}</label>
              <p class="form-tip">{{ $t('knowledgeEditor.indexing.description') }}</p>
              <div class="indexing-checks" :class="{ 'is-locked': isIndexingLocked }"
                data-guide="kb-create-indexing">
                <div
                  class="indexing-check-item"
                  :class="{ 'is-checked': formData.indexingStrategy.vectorEnabled, 'is-disabled': isIndexingLocked }"
                  @click="toggleVectorIndexing"
                >
                  <t-checkbox
                    :checked="formData.indexingStrategy.vectorEnabled"
                    :disabled="isIndexingLocked"
                    class="indexing-check-box"
                  >{{ $t('knowledgeEditor.indexing.searchTitle') }}</t-checkbox>
                  <p class="indexing-check-desc">{{ $t('knowledgeEditor.indexing.searchDesc') }}</p>
                </div>
                <div
                  class="indexing-check-item"
                  :class="{ 'is-checked': formData.indexingStrategy.wikiEnabled, 'is-disabled': isIndexingLocked }"
                  @click="toggleWikiIndexing"
                >
                  <t-checkbox
                    :checked="formData.indexingStrategy.wikiEnabled"
                    :disabled="isIndexingLocked"
                    class="indexing-check-box"
                  >
                    <span class="indexing-check-title">
                      {{ $t('knowledgeEditor.indexing.wikiTitle') }}
                      <span class="indexing-new-badge">NEW</span>
                    </span>
                  </t-checkbox>
                  <p class="indexing-check-desc">{{ $t('knowledgeEditor.indexing.wikiDesc') }}</p>
                </div>
              </div>
              <p v-if="isIndexingLocked" class="locked-tip">
                <t-icon name="lock-on" class="locked-tip-icon" />
                <span>{{ $t('knowledgeEditor.indexing.lockedTip') }}</span>
              </p>
            </div>

            <!-- Wiki 提取粒度 (仅当 Wiki 启用时显示) -->
            <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.wiki.extractionGranularityLabel') }}</label>
              <p class="form-tip">{{ $t('knowledgeEditor.wiki.extractionGranularityTip') }}</p>
              <t-radio-group
                :value="resolvedGranularity"
                class="granularity-radio-group"
                @change="handleGranularityChange"
              >
                <t-radio-button value="focused">
                  {{ $t('knowledgeEditor.wiki.granularityFocused') }}
                </t-radio-button>
                <t-radio-button value="standard">
                  {{ $t('knowledgeEditor.wiki.granularityStandard') }}
                </t-radio-button>
                <t-radio-button value="exhaustive">
                  {{ $t('knowledgeEditor.wiki.granularityExhaustive') }}
                </t-radio-button>
              </t-radio-group>
              <p class="form-tip granularity-hint">{{ granularityHint }}</p>
            </div>

            <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.wiki.contentInstructionsLabel') }}</label>
              <p class="form-tip">{{ $t('knowledgeEditor.wiki.contentInstructionsTip') }}</p>
              <t-textarea
                v-model="formData.wikiConfig.contentInstructions"
                :placeholder="$t('knowledgeEditor.wiki.contentInstructionsPlaceholder')"
                :maxlength="4000"
                :autosize="{ minRows: 3, maxRows: 8 }"
              />
            </div>

            <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.wiki.extractionInstructionsLabel') }}</label>
              <p class="form-tip">{{ $t('knowledgeEditor.wiki.extractionInstructionsTip') }}</p>
              <t-textarea
                v-model="formData.wikiConfig.extractionInstructions"
                :placeholder="$t('knowledgeEditor.wiki.extractionInstructionsPlaceholder')"
                :maxlength="4000"
                :autosize="{ minRows: 3, maxRows: 8 }"
              />
            </div>

            <div class="form-item" data-guide="kb-create-name">
              <label class="form-label required">{{ $t('knowledgeEditor.basic.nameLabel') }}</label>
              <t-input
                v-model="formData.name"
                :placeholder="$t('knowledgeEditor.basic.namePlaceholder')"
                :maxlength="50"
              />
            </div>
            <div class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.basic.descriptionLabel') }}</label>
              <t-textarea
                v-model="formData.description"
                :placeholder="$t('knowledgeEditor.basic.descriptionPlaceholder')"
                :maxlength="200"
                :autosize="{ minRows: 3, maxRows: 6 }"
              />
            </div>

            <!-- AI 生成的知识库描述（仅编辑模式、文档型知识库） -->
            <div v-if="editorMode === 'edit' && !isFAQ" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.basic.profile.title') }}</label>
              <div class="kb-profile-card" :class="{ 'is-empty': !generatedProfileHasText }">
                <template v-if="generatedProfileHasText">
                  <p v-if="generatedProfile?.gist" class="kb-profile-gist">{{ generatedProfile.gist }}</p>
                  <div v-if="generatedProfile?.topics?.length" class="kb-profile-topics">
                    <t-tag
                      v-for="topic in generatedProfile.topics"
                      :key="topic"
                      size="small"
                      theme="primary"
                      variant="light"
                    >{{ topic }}</t-tag>
                  </div>
                  <div v-if="generatedProfile?.typical_questions?.length" class="kb-profile-questions">
                    <p class="kb-profile-subtitle">{{ $t('knowledgeEditor.basic.profile.questions') }}</p>
                    <ul>
                      <li v-for="q in generatedProfile.typical_questions" :key="q">{{ q }}</li>
                    </ul>
                  </div>
                  <p v-if="generatedProfile?.status === 'failed'" class="kb-profile-error">
                    {{ $t('knowledgeEditor.basic.profile.failed', { error: generatedProfile.error || '' }) }}
                  </p>
                  <div class="kb-profile-footer">
                    <span class="kb-profile-meta">{{ generatedProfileMeta }}</span>
                    <div class="kb-profile-actions">
                      <t-button
                        v-if="generatedProfile?.gist"
                        size="small"
                        variant="text"
                        theme="primary"
                        @click="handleAdoptProfileGist"
                      >
                        <template #icon><t-icon name="enter" /></template>
                        {{ $t('knowledgeEditor.basic.profile.adopt') }}
                      </t-button>
                      <t-button
                        size="small"
                        variant="text"
                        :loading="generatingProfile"
                        @click="handleGenerateProfile"
                      >
                        <template #icon><t-icon name="refresh" /></template>
                        {{ $t('knowledgeEditor.basic.profile.regenerate') }}
                      </t-button>
                    </div>
                  </div>
                </template>
                <template v-else>
                  <div class="kb-profile-empty">
                    <span class="kb-profile-empty-icon"><t-icon name="ai-edit-1" /></span>
                    <div class="kb-profile-empty-text">
                      <p>
                        {{ generatedProfile?.status === 'empty'
                          ? $t('knowledgeEditor.basic.profile.noDocuments')
                          : $t('knowledgeEditor.basic.profile.empty') }}
                      </p>
                      <p v-if="generatedProfile?.status === 'failed'" class="kb-profile-error">
                        {{ $t('knowledgeEditor.basic.profile.failed', { error: generatedProfile.error || '' }) }}
                      </p>
                    </div>
                    <t-button
                      size="small"
                      theme="primary"
                      variant="outline"
                      :loading="generatingProfile"
                      @click="handleGenerateProfile"
                    >
                      <template #icon><t-icon name="ai-edit-1" /></template>
                      {{ $t('knowledgeEditor.basic.profile.generate') }}
                    </t-button>
                  </div>
                </template>
              </div>
              <p class="form-tip">{{ $t('knowledgeEditor.basic.profile.hint') }}</p>
            </div>

            <!-- Wiki 合成模型移至模型配置页 -->
          </div>
        </div>
      </div>

      <!-- 共享设置（仅编辑模式） -->
      <div v-if="editorMode === 'edit' && activeKbId && currentSection === 'share'" class="section">
        <KBShareSettings :kb-id="activeKbId" :can-share="canShareKB" />
      </div>

      <!-- 活动记录（仅编辑模式，KB 所属租户内 Owner/Admin） -->
      <div v-if="editorMode === 'edit' && activeKbId && canViewActivity && currentSection === 'activity'" class="section">
        <KnowledgeBaseActivitySettings :kb-id="activeKbId" :active="currentSection === 'activity'" />
      </div>
    </div>

    <template #footer-note>
      <p v-if="isPostCreateSession" class="settings-footer-note">
        <t-icon name="check-circle-filled" class="settings-footer-note__icon" />
        <span>
          <strong>{{ $t('knowledgeEditor.postCreateHint.title') }}</strong>
          {{ $t('knowledgeEditor.postCreateHint.footer') }}
        </span>
      </p>
      <p v-if="isInstantSection" class="settings-footer-note">
        <t-icon name="info-circle-filled" class="settings-footer-note__icon" />
        <span>{{ $t('knowledgeEditor.footer.instantEffect') }}</span>
      </p>
    </template>
    <template #footer>
      <t-button v-if="isInstantSection" theme="default" variant="outline" @click="modalShell.requestClose">
        {{ $t('common.close') }}
      </t-button>
      <template v-else>
        <t-button theme="default" variant="outline" @click="modalShell.requestClose">
          {{ $t('common.cancel') }}
        </t-button>
        <t-button theme="primary" data-guide="kb-create-submit" @click="handleSubmit" :loading="saving"
          :disabled="loading">
          {{ saveButtonLabel }}
        </t-button>
      </template>
    </template>
  </SettingsModalShell>

  <KbCreateContextualGuide :when="visible && editorMode === 'create'" :is-faq="isFAQ" />
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import KbCreateContextualGuide from '@/components/KbCreateContextualGuide.vue'
import { KB_EDITOR_FOCUS_SECTION_EVENT, markContextualGuideDone } from '@/config/contextualGuides'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { useModalShell } from '@/composables/useModalShell'
import SettingsModalShell from '@/components/SettingsModalShell.vue'
import {
  createKnowledgeBase,
  getKnowledgeBaseById,
  listKnowledgeFiles,
  updateKnowledgeBase,
  rebuildKBIndex,
  generateKnowledgeBaseProfile,
  mergeImageActions,
  fetchImageAttrSchema,
  FALLBACK_IMAGE_ATTR_SCHEMA,
  type ImageActionsConfig,
  type ImageAttrSchema,
  type KnowledgeBaseProfile,
} from '@/api/knowledge-base'
import { buildImageProcessingConfig } from '@/utils/imageProcessingConfig'

// The image-attribute registry, fetched from the backend (single source of
// truth). Falls back to the static registry until the endpoint answers.
const imageAttrSchema = ref<ImageAttrSchema | null>(null)
const displaySchema = computed<ImageAttrSchema>(
  () => imageAttrSchema.value ?? FALLBACK_IMAGE_ATTR_SCHEMA,
)
async function loadImageAttrSchema(kbId: string) {
  try {
    imageAttrSchema.value = await fetchImageAttrSchema(kbId)
  } catch {
    imageAttrSchema.value = null
  }
}
import { updateKBConfig, type KBModelConfigRequest } from '@/api/initialization'
import { useChatResourcesStore } from '@/stores/chatResources'
import { selectInitialModelId } from '@/utils/modelDefaults'
import { copyWithToast } from '@/utils/clipboard'
import { useEditorResourcesStore } from '@/stores/editorResources'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import KBShareSettings from './settings/KBShareSettings.vue'
import KnowledgeBaseActivitySettings from './settings/KnowledgeBaseActivitySettings.vue'
import { useI18n } from 'vue-i18n'

const uiStore = useUIStore()
const authStore = useAuthStore()
const chatResources = useChatResourcesStore()
const editorResources = useEditorResourcesStore()
const { t } = useI18n()

// Props
const props = defineProps<{
  visible: boolean
  mode: 'create' | 'edit'
  kbId?: string
  initialType?: 'document' | 'faq'
}>()

// Emits
const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'success', kbId: string): void
}>()

/** 首次保存创建成功后留在弹窗内，继续配置共享等设置 */
const savedKbId = ref<string | null>(null)
const editorMode = computed(() => (savedKbId.value ? 'edit' : props.mode))
const activeKbId = computed(() => savedKbId.value ?? props.kbId)
const isPostCreateSession = computed(() => !!savedKbId.value)
const saveButtonLabel = computed(() =>
  editorMode.value === 'create'
    ? t('knowledgeEditor.buttons.create')
    : t('knowledgeEditor.buttons.saveAndClose')
)

const copyKbId = async () => {
  await copyWithToast(activeKbId.value, 'common.copied')
}

const currentSection = ref<string>('basic')

const onKbEditorFocusSection = (event: Event) => {
  const section = (event as CustomEvent<{ section?: string }>).detail?.section
  if (section) {
    currentSection.value = section
  }
}

onMounted(() => {
  window.addEventListener(KB_EDITOR_FOCUS_SECTION_EVENT, onKbEditorFocusSection)
})

onBeforeUnmount(() => {
  window.removeEventListener(KB_EDITOR_FOCUS_SECTION_EVENT, onKbEditorFocusSection)
})
const saving = ref(false)
const loading = ref(false)
const allModels = ref<any[]>([])
const hasFiles = ref(false)
// AI-generated knowledge-base description (edit mode only). Kept outside
// formData because it is never submitted: the backend owns it.
const generatedProfile = ref<KnowledgeBaseProfile | null>(null)
const generatingProfile = ref(false)
const generatedProfileHasText = computed(() => {
  const p = generatedProfile.value
  return !!(p && (p.gist || p.topics?.length || p.typical_questions?.length))
})
const generatedProfileMeta = computed(() => {
  const p = generatedProfile.value
  if (!p || !p.generated_at) return ''
  const when = new Date(p.generated_at)
  const stamp = isNaN(when.getTime()) ? p.generated_at : when.toLocaleString()
  return t('knowledgeEditor.basic.profile.generatedAt', {
    time: stamp,
    count: p.stats?.document_count ?? 0,
  })
})
const initialStorageProvider = ref<string>('')
/** Tenant-wide default from Settings → Storage engine (used when creating a KB). */
const tenantDefaultStorageProvider = ref('local')
const initialIndexingStrategy = ref<any>(null)
// Identifier of the user who created this KB. Empty for older rows
// that predate per-KB ownership tracking; those KBs have no "owner" and
// only tenant Admin+ can mutate their share settings.
const kbCreatorId = ref<string>('')
const kbTenantId = ref<number>(0)

// Backend gate for /knowledge-bases/:id/shares (POST/PUT/DELETE) is
// g.OwnedKBOrAdmin(): only the KB creator or tenant Admin+ may mutate
// shares. Org-admins on a shared KB do NOT pass this guard, so they
// would only see 403s if we let them try. Mirror the matrix here so
// the buttons disappear instead of failing.
const canShareKB = computed(() => {
  if (!activeKbId.value) return false
  const userId = authStore.user?.id || ''
  if (kbCreatorId.value && userId && kbCreatorId.value === userId) return true
  return authStore.hasRole('admin')
})

const isKbOwner = computed(() => {
  const userId = authStore.user?.id || ''
  return Boolean(kbCreatorId.value && userId && kbCreatorId.value === userId)
})

const canViewActivity = computed(() => {
  if (editorMode.value !== 'edit' || !activeKbId.value) return false
  if (Number(kbTenantId.value || 0) !== Number(authStore.currentTenantId || 0)) return false
  return isKbOwner.value || authStore.hasRole('admin')
})

// 模型 / 分块 / 图像 / 音频 / 图谱 / 高级 / 存储等分区只在平台「知识库设置」维护，
// 弹窗里对任何角色都不再展示，因此不再按角色分支。

// 这些分区的操作在点击时即时生效（共享 / 活动记录），不经过底部「保存」，
// 因此底部只显示「关闭」并提示，避免用户以为需要再点保存或以为「取消」能撤销。
const INSTANT_SECTIONS = new Set(['share', 'activity'])
const isInstantSection = computed(() => INSTANT_SECTIONS.has(currentSection.value))

// 表单数据。必须在 navItems 等引用它的 computed 之前声明：computed 虽为惰性求值，
// 但一旦有 watch(navItems)/watch(navGroups) 之类在 setup 阶段提前触发求值，就会读取
// 尚未初始化的 formData 命中 TDZ，导致整个 setup 抛错、后续渲染/更新级联失败。
const formData = ref<any>(null)

const navItems = computed(() => {
  // 建库与编辑都只保留基本信息：模型/向量存储/解析引擎/分块/图像/音频/图谱/高级/存储
  // 统一在平台「知识库设置」维护，建库时由后端自动引用；问答类型专属的 FAQ 索引策略
  // 在建库与编辑都内联在基本信息里（见上方内联块）。
  const items: { key: string; icon: string; label: string; badge?: number }[] = [
    { key: 'basic', icon: 'info-circle', label: t('knowledgeEditor.sidebar.basic') },
  ]
  if (editorMode.value === 'edit' && activeKbId.value && !authStore.isLiteMode) {
    items.push({ key: 'share', icon: 'share', label: t('knowledgeEditor.sidebar.share') })
  }
  if (canViewActivity.value) {
    items.push({ key: 'activity', icon: 'history', label: t('knowledgeEditor.sidebar.activity') })
  }
  return items
})

// 左侧导航分组（与 AgentEditorModal 对齐）
const navGroups = computed(() => {
  const itemMap = new Map(navItems.value.map((item) => [item.key, item]))
  const pickItems = (keys: string[]) =>
    keys.map((key) => itemMap.get(key)).filter(Boolean) as typeof navItems.value
  return [
    {
      key: 'basic',
      label: t('knowledgeEditor.navGroups.basic'),
      items: pickItems(['basic']),
    },
    {
      key: 'integration',
      label: t('knowledgeEditor.navGroups.integration'),
      items: pickItems(['share']),
    },
    {
      key: 'management',
      label: t('knowledgeEditor.navGroups.management'),
      items: pickItems(['activity']),
    },
  ].filter((group) => group.items.length > 0)
})

// 当前分区若因权限（或类型切换）而不在可见菜单里，回退到基本信息，避免停在隐藏分区造成空白。
// 注意：不能在此处 watch(navItems)——会在 setup 阶段提前求值 navItems，触发对尚未声明的
// formData 的 TDZ 错误。改由弹窗打开数据加载完成后显式调用（见 watch(props.visible)），
// 类型切换则由下方已有的 type 监听处理。
function ensureVisibleSection() {
  if (!props.visible) return
  const keys = navItems.value.map((it) => it.key)
  if (keys.length && !keys.includes(currentSection.value)) {
    currentSection.value = 'basic'
  }
}

const isFAQ = computed(() => formData.value?.type === 'faq')

const applyDefaultModelsIfEmpty = () => {
  if (!formData.value || editorMode.value !== 'create') return
  const chatModelId = selectInitialModelId(allModels.value, 'KnowledgeQA')
  const embeddingModelId = selectInitialModelId(allModels.value, 'Embedding')
  if (!formData.value.modelConfig.llmModelId && chatModelId) {
    formData.value.modelConfig.llmModelId = chatModelId
  }
  if (!formData.value.modelConfig.embeddingModelId && embeddingModelId) {
    formData.value.modelConfig.embeddingModelId = embeddingModelId
  }
}

watch(
  () => formData.value?.type,
  (newType) => {
    if (!formData.value) return
    // 切到问答类型时补齐 FAQ 索引策略默认值（基本信息内联展示）。
    if (newType === 'faq' && !formData.value.faqConfig) {
      formData.value.faqConfig = { indexMode: 'question_answer', questionIndexMode: 'combined' }
    }
  }
)

// 初始化表单数据
const initFormData = (type: 'document' | 'faq' = 'document') => {
  return {
    type,
    name: '',
    description: '',
    // 建库时用户在基本信息里自选 FAQ 索引策略（默认值镜像后端
    // KnowledgeBase.EnsureDefaults()，保持未手动修改时的行为不变）。
    faqConfig: {
      indexMode: 'question_answer',
      questionIndexMode: 'combined'
    },
    modelConfig: {
      llmModelId: '',
      embeddingModelId: '',
      wikiSynthesisModelId: '',
    },
    chunkingConfig: {
      chunkSize: 512,
      // 80 ≈ 15% of chunkSize — community-recommended sweet spot.
      // Aligned with chunker.DefaultChunkOverlap on the backend.
      chunkOverlap: 80,
      separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
      parserEngineRules: undefined as any,
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      // New KBs default to the adaptive auto-strategy. User can change in the UI.
      strategy: 'auto' as string,
      tokenLimit: 0,
      languages: [] as string[],
      tableMetadataInstructions: ''
    },
    storageBackendId: '' as string,
    storageProvider: '' as string,
    multimodalConfig: {
      enabled: false,
      vllmModelId: '',
      descriptionLanguage: '',
      customInstructions: ''
    },
    // 图片属性观察管线：开关 + on_unobserved 兜底切换由 UI 编辑；其余配置
    // （model_id 等）按加载时的快照原样回传，避免把 API 侧写入的设置洗掉。
    // 新建模式也必须用完整默认动作初始化——imageActions.ocr.on_unobserved
    // 直接被开关绑定，缺省会让打开开关的瞬间渲染崩溃。
    imageAttrsEnabled: false,
    // 图片向量默认关闭：能处理图片的向量模型也常只用来编码文本。
    imageVectorEnabled: false,
    imageActions: mergeImageActions(),
    imageProcessingConfigSnapshot: null as Record<string, unknown> | null,
    asrConfig: {
      enabled: false,
      modelId: '',
      language: ''
    },
    nodeExtractConfig: {
      enabled: false,
      text: '',
      tags: [] as string[],
      nodes: [] as Array<{
        name: string
        attributes: string[]
      }>,
      relations: [] as Array<{
        node1: string
        node2: string
        type: string
      }>,
      customInstructions: ''
    },
    questionGenerationConfig: {
      enabled: true,
      questionCount: 3,
      customInstructions: ''
    },
    autoTagConfig: {
      enabled: false,
      modelId: '',
      maxTags: 3,
      skipIfTagged: true
    },
    profileConfig: {
      enabled: false,
      modelId: '',
      customInstructions: ''
    },
    wikiConfig: {
      synthesisModelId: '',
      maxPagesPerIngest: 0,
      extractionGranularity: 'standard' as 'focused' | 'standard' | 'exhaustive',
      contentInstructions: '',
      extractionInstructions: '',
    },
    indexingStrategy: {
      vectorEnabled: true,
      keywordEnabled: true,
      wikiEnabled: false,
      graphEnabled: false,
    },
    // Vector-store binding. Empty string means "use the env-configured
    // store"; create mode defaults to that, edit mode loads the
    // existing binding from the KB response below.
    vectorStoreId: '' as string,
    vectorStoreInfo: {
      source: undefined as string | undefined,
      name: undefined as string | undefined,
      engineType: undefined as string | undefined,
      status: undefined as string | undefined,
    },
  }
}

// 加载所有模型
const loadAllModels = async (force = false) => {
  try {
    await chatResources.ensureModels(force)
    allModels.value = chatResources.allModels || []
  } catch (error) {
    console.error('Failed to load model list:', error)
    MessagePlugin.error(t('knowledgeEditor.messages.loadModelsFailed'))
    allModels.value = []
  }
}

let kbEditorLoadGeneration = 0

const isCurrentKBLoad = (generation: number, kbId: string) => (
  generation === kbEditorLoadGeneration
  && props.visible
  && activeKbId.value === kbId
)

// 加载知识库数据（编辑模式）
const loadKBData = async (
  kbIdOverride?: string,
  generation = kbEditorLoadGeneration,
) => {
  const kbId = kbIdOverride ?? activeKbId.value
  if (editorMode.value !== 'edit' || !kbId) return
  
  loading.value = true
  try {
    const [kbInfo, filesResult] = await Promise.all([
      getKnowledgeBaseById(kbId),
      listKnowledgeFiles(kbId, { page: 1, page_size: 1 })
    ])

    if (!isCurrentKBLoad(generation, kbId)) return
    
    if (!kbInfo || !kbInfo.data) {
      throw new Error(t('knowledgeEditor.messages.notFound'))
    }

    const kb = kbInfo.data
    hasFiles.value = (filesResult as any)?.total > 0
    generatedProfile.value = (kb as any).generated_profile || null
    kbCreatorId.value = (kb as any).creator_id || ''
    kbTenantId.value = Number((kb as any).tenant_id || 0)

    // 设置表单数据
    const kbType = (kb.type as 'document' | 'faq') || 'document'
    formData.value = {
      type: kbType,
      name: kb.name || '',
      description: kb.description || '',
      faqConfig: {
        indexMode: kb.faq_config?.index_mode || 'question_only',
        questionIndexMode: kb.faq_config?.question_index_mode || 'separate'
      },
      modelConfig: {
        llmModelId: kb.summary_model_id || '',
        embeddingModelId: kb.embedding_model_id || '',
        wikiSynthesisModelId: kb.wiki_config?.synthesis_model_id || ''
      },
      chunkingConfig: {
        chunkSize: kb.chunking_config?.chunk_size || 512,
        // Fallback only used when the loaded KB has no chunk_overlap stored.
        // Aligned with chunker.DefaultChunkOverlap on the backend.
        chunkOverlap: kb.chunking_config?.chunk_overlap || 80,
        separators: kb.chunking_config?.separators || ['\n\n', '\n', '。', '！', '？', ';', '；'],
        parserEngineRules: kb.chunking_config?.parser_engine_rules || undefined,
        enableParentChild: kb.chunking_config?.enable_parent_child || false,
        parentChunkSize: kb.chunking_config?.parent_chunk_size || 4096,
        childChunkSize: kb.chunking_config?.child_chunk_size || 384,
        // Existing KBs without strategy field render as empty (= legacy behavior).
        // The user has to actively pick a value to opt in to the new tiers.
        strategy: kb.chunking_config?.strategy || '',
        tokenLimit: kb.chunking_config?.token_limit || 0,
        languages: kb.chunking_config?.languages || [],
        tableMetadataInstructions: kb.chunking_config?.table_metadata_instructions || ''
      },
      storageBackendId: (kb.storage_backend_id || '') as string,
      storageProvider: (kb.storage_provider_config?.provider || kb.storage_config?.provider || 'local') as string,
      multimodalConfig: {
        enabled: !!kb.vlm_config?.enabled,
        vllmModelId: kb.vlm_config?.model_id || '',
        descriptionLanguage: kb.vlm_config?.description_language || '',
        customInstructions: kb.vlm_config?.custom_instructions || ''
      },
      imageAttrsEnabled:
        !!(kb as Record<string, any>).image_processing_config?.image_attrs_enabled,
      imageVectorEnabled:
        !!(kb as Record<string, any>).image_processing_config?.image_vector_enabled,
      imageActions: mergeImageActions(
        (kb as Record<string, any>).image_processing_config?.image_actions,
      ),
      imageProcessingConfigSnapshot:
        (kb as Record<string, any>).image_processing_config || null,
      asrConfig: {
        enabled: !!kb.asr_config?.enabled,
        modelId: kb.asr_config?.model_id || '',
        language: kb.asr_config?.language || ''
      },
      nodeExtractConfig: {
        enabled: kb.extract_config?.enabled || false,
        text: kb.extract_config?.text || '',
        tags: kb.extract_config?.tags || [],
        nodes: (kb.extract_config?.nodes || []).map((node: any) => ({
          name: node.name,
          attributes: node.attributes || []
        })),
        relations: kb.extract_config?.relations || [],
        customInstructions: kb.extract_config?.custom_instructions || ''
      },
      questionGenerationConfig: {
        enabled: kb.question_generation_config?.enabled || false,
        questionCount: kb.question_generation_config?.question_count || 3,
        customInstructions: kb.question_generation_config?.custom_instructions || ''
      },
      autoTagConfig: {
        enabled: kb.auto_tag_config?.enabled || false,
        modelId: kb.auto_tag_config?.model_id || '',
        maxTags: kb.auto_tag_config?.max_tags || 3,
        // Absent on knowledge bases saved before the toggle existed; the
        // backend treats that as "skip", so mirror it here.
        skipIfTagged: kb.auto_tag_config?.skip_if_tagged ?? true
      },
      profileConfig: {
        enabled: kb.profile_config?.enabled || false,
        modelId: kb.profile_config?.model_id || '',
        customInstructions: kb.profile_config?.custom_instructions || ''
      },
      wikiConfig: {
        synthesisModelId: kb.wiki_config?.synthesis_model_id || '',
        maxPagesPerIngest: kb.wiki_config?.max_pages_per_ingest || 0,
        extractionGranularity: (
          kb.wiki_config?.extraction_granularity === 'focused' ||
          kb.wiki_config?.extraction_granularity === 'exhaustive'
            ? kb.wiki_config.extraction_granularity
            : 'standard'
        ) as 'focused' | 'standard' | 'exhaustive',
        contentInstructions: kb.wiki_config?.content_instructions || '',
        extractionInstructions: kb.wiki_config?.extraction_instructions || '',
      },
      indexingStrategy: {
        vectorEnabled: kb.indexing_strategy?.vector_enabled ?? true,
        keywordEnabled: kb.indexing_strategy?.keyword_enabled ?? true,
        wikiEnabled: kb.indexing_strategy?.wiki_enabled ?? false,
        graphEnabled: kb.indexing_strategy?.graph_enabled ?? false,
      },
      // Vector-store binding. vectorStoreId is editor-only state; it
      // is only included in the create request, never the update
      // request, because the binding is immutable after creation.
      // vectorStoreInfo carries the read-only display fields that the
      // edit view renders below; they come straight from the KB
      // response.
      vectorStoreId: '',
      vectorStoreInfo: {
        source: kb.vector_store_source,
        name: kb.vector_store_name,
        engineType: kb.vector_store_engine_type,
        status: kb.vector_store_status,
      },
    }
    // 拉取后端属性注册表，驱动属性面板的动态渲染（编辑模式有 kbId）。
    loadImageAttrSchema(kbId)
    initialStorageProvider.value = formData.value.storageProvider
    initialIndexingStrategy.value = { ...formData.value.indexingStrategy }
  } catch (error) {
    if (!isCurrentKBLoad(generation, kbId)) return
    console.error('Failed to load knowledge base data:', error)
    MessagePlugin.error(t('knowledgeEditor.messages.loadDataFailed'))
    handleClose()
  } finally {
    if (isCurrentKBLoad(generation, kbId)) {
      loading.value = false
      modalShell.markClean()
    }
  }
}

// 粒度选择器：从 formData.wikiConfig 读出并规范化，未知值回退到 'standard'，
// 与后端 WikiExtractionGranularity.Normalize() 的契约保持一致。
const resolvedGranularity = computed<'focused' | 'standard' | 'exhaustive'>(() => {
  const g = formData.value?.wikiConfig?.extractionGranularity
  if (g === 'focused' || g === 'standard' || g === 'exhaustive') {
    return g
  }
  return 'standard'
})

const granularityHint = computed<string>(() => {
  switch (resolvedGranularity.value) {
    case 'focused':
      return t('knowledgeEditor.wiki.granularityFocusedHint')
    case 'exhaustive':
      return t('knowledgeEditor.wiki.granularityExhaustiveHint')
    default:
      return t('knowledgeEditor.wiki.granularityStandardHint')
  }
})

const handleGranularityChange = (value: string | number | boolean) => {
  if (!formData.value) return
  const next: 'focused' | 'standard' | 'exhaustive' =
    value === 'focused' || value === 'exhaustive'
      ? (value as 'focused' | 'exhaustive')
      : 'standard'
  formData.value.wikiConfig = {
    ...formData.value.wikiConfig,
    extractionGranularity: next,
  }
}

const isIndexingLocked = computed(() => editorMode.value === 'edit' && hasFiles.value)

const toggleVectorIndexing = () => {
  if (!formData.value) return
  if (isIndexingLocked.value) return
  const next = !formData.value.indexingStrategy.vectorEnabled
  formData.value.indexingStrategy.vectorEnabled = next
  formData.value.indexingStrategy.keywordEnabled = next
}

const toggleWikiIndexing = () => {
  if (!formData.value) return
  if (isIndexingLocked.value) return
  formData.value.indexingStrategy.wikiEnabled = !formData.value.indexingStrategy.wikiEnabled
}

// 判断当前是否为「仅 Wiki 索引」：只开了 Wiki，关了向量/关键词检索
const isWikiOnlyStrategy = computed(() => {
  const s = formData.value?.indexingStrategy
  if (!s) return false
  return !!s.wikiEnabled && !s.vectorEnabled && !s.keywordEnabled
})

// 仅 Wiki 索引模式下的分块预设：更大 chunk、无 overlap、关闭父子分块。
const WIKI_ONLY_CHUNKING_PRESET = {
  chunkSize: 2048,
  chunkOverlap: 0,
  enableParentChild: false,
} as const

// 与后端 chunker.DefaultChunkSize / DefaultChunkOverlap 对齐的普通预设。
const DEFAULT_CHUNKING_PRESET = {
  chunkSize: 512,
  chunkOverlap: 80,
  enableParentChild: true,
} as const

// 创建模式下随索引策略应用/撤销 Wiki-only 分块预设；编辑模式严格保持
// 后端已有配置不变，避免误改。
watch(isWikiOnlyStrategy, (wikiOnly) => {
  if (editorMode.value !== 'create') return
  if (!formData.value) return
  const preset = wikiOnly ? WIKI_ONLY_CHUNKING_PRESET : DEFAULT_CHUNKING_PRESET
  formData.value.chunkingConfig = {
    ...formData.value.chunkingConfig,
    ...preset,
  }
})

async function loadTenantDefaultStorageProvider(force = false) {
  try {
    await editorResources.ensureStorageEngine(force)
    tenantDefaultStorageProvider.value = editorResources.resolveUsableStorageProvider(
      editorResources.storageConfig?.default_provider,
    )
  } catch {
    tenantDefaultStorageProvider.value = editorResources.resolveUsableStorageProvider()
  }
}

/** Resolved storage provider for create payload (never silently default to local before tenant config loads). */
function resolvedStorageProvider(): string {
  const explicit = formData.value?.storageProvider?.trim()
  if (editorMode.value === 'create') {
    return editorResources.resolveUsableStorageProvider(explicit || tenantDefaultStorageProvider.value)
  }
  if (explicit) return explicit
  return tenantDefaultStorageProvider.value || 'local'
}

// Regenerate the AI description now (synchronous: one aggregation + one
// small model call). The result replaces the card but never the manual
// description; "adopt" copies the gist over explicitly.
const handleGenerateProfile = async () => {
  const kbId = activeKbId.value
  if (!kbId || generatingProfile.value) return
  generatingProfile.value = true
  try {
    const result: any = await generateKnowledgeBaseProfile(kbId)
    if (!result?.success) {
      throw new Error(result?.message || t('knowledgeEditor.basic.profile.generateFailed'))
    }
    generatedProfile.value = result.data || null
    MessagePlugin.success(t('knowledgeEditor.basic.profile.generated'))
  } catch (error: any) {
    console.error('Generate knowledge base profile failed:', error)
    MessagePlugin.error(error?.message || t('knowledgeEditor.basic.profile.generateFailed'))
  } finally {
    generatingProfile.value = false
  }
}

const handleAdoptProfileGist = () => {
  const gist = generatedProfile.value?.gist
  if (!gist || !formData.value) return
  formData.value.description = gist.slice(0, 200)
  MessagePlugin.success(t('knowledgeEditor.basic.profile.adopted'))
}

// 验证表单
const validateForm = (): boolean => {
  if (!formData.value) return false

  // 验证基本信息
  if (!formData.value.name || !formData.value.name.trim()) {
    MessagePlugin.warning(t('knowledgeEditor.messages.nameRequired'))
    currentSection.value = 'basic'
    return false
  }

  // 验证索引策略 — 文档类型至少需要开启一种
  if (formData.value.type !== 'faq') {
    const s = formData.value.indexingStrategy
    if (s && !s.vectorEnabled && !s.keywordEnabled && !s.wikiEnabled && !s.graphEnabled) {
      MessagePlugin.warning(t('knowledgeEditor.indexing.atLeastOne'))
      currentSection.value = 'basic'
      return false
    }
  }

  // 模型 / 多模态 / 图谱等配置已从弹窗移到平台「知识库设置」，这里不再校验：
  // 用户看不到也无权修改，卡在隐藏分区会让他们无法保存。

  // 问答库的索引策略内联在基本信息里（建库与编辑同一入口），
  // 缺失时回到基本信息让用户补选。
  if (
    formData.value.type === 'faq' &&
    !formData.value.faqConfig?.indexMode
  ) {
    MessagePlugin.warning(t('knowledgeEditor.messages.indexModeRequired'))
    currentSection.value = 'basic'
    return false
  }

  return true
}

// 构建提交数据
const buildSubmitData = () => {
  if (!formData.value) return null

  const data: any = {
    name: formData.value.name,
    description: formData.value.description,
    type: formData.value.type,
    chunking_config: {
      chunk_size: formData.value.chunkingConfig.chunkSize,
      chunk_overlap: formData.value.chunkingConfig.chunkOverlap,
      separators: formData.value.chunkingConfig.separators,
      enable_parent_child: formData.value.chunkingConfig.enableParentChild,
      parent_chunk_size: formData.value.chunkingConfig.parentChunkSize,
      child_chunk_size: formData.value.chunkingConfig.childChunkSize,
      // Adaptive chunking fields are always sent (empty/zero values
      // included) so the user can clear them — backend uses pointer DTOs
      // to distinguish "not in payload" from "explicitly empty".
      strategy: formData.value.chunkingConfig.strategy ?? '',
      token_limit: formData.value.chunkingConfig.tokenLimit ?? 0,
      languages: formData.value.chunkingConfig.languages ?? [],
      table_metadata_instructions: formData.value.chunkingConfig.tableMetadataInstructions || '',
      ...(formData.value.chunkingConfig.parserEngineRules?.length
        ? { parser_engine_rules: formData.value.chunkingConfig.parserEngineRules }
        : {})
    },
    embedding_model_id: formData.value.modelConfig.embeddingModelId,
    summary_model_id: formData.value.modelConfig.llmModelId
  }

  // Vector-store binding. Only attach the field when the user actively
  // selected a non-default store. The server treats an empty string as
  // NULL, but keeping the field absent on the wire matches what a
  // client that doesn't know about this binding would send — which
  // makes A/B response diffs easier to read.
  if (formData.value.vectorStoreId) {
    data.vector_store_id = formData.value.vectorStoreId
  }

  // 添加多模态配置
  data.vlm_config = {
    enabled: formData.value.multimodalConfig.enabled,
    model_id: formData.value.multimodalConfig.enabled
      ? (formData.value.multimodalConfig.vllmModelId || '')
      : '',
    description_language: formData.value.multimodalConfig.descriptionLanguage || '',
    custom_instructions: formData.value.multimodalConfig.customInstructions || ''
  }

  // 图片属性观察配置：后端是整体替换语义（payload 不带该字段 = 保持不变）。
  // UI 只编辑观察开关与 on_unobserved；快照里用 API 自定义过的 on 原样保留，
  // 没有自定义时才取注册表默认值。合并规则见 buildImageProcessingConfig。
  {
    const built = buildImageProcessingConfig(formData.value.imageProcessingConfigSnapshot, {
      imageAttrsEnabled: formData.value.imageAttrsEnabled,
      imageVectorEnabled: formData.value.imageVectorEnabled,
      onUnobserved: formData.value.imageActions.ocr.on_unobserved,
      defaultOn: displaySchema.value.default_actions.ocr.on,
    })
    if (built) {
      data.image_processing_config = built
    }
  }

  // 添加ASR语音识别配置
  data.asr_config = {
    enabled: formData.value.asrConfig?.enabled || false,
    model_id: formData.value.asrConfig?.enabled
      ? (formData.value.asrConfig?.modelId || '')
      : '',
    language: formData.value.asrConfig?.language || ''
  }

  // storage_backend_id is authoritative. Keep provider projection for old clients
  // and for rolling upgrades where a node has not picked up the new schema yet.
  if (formData.value.storageBackendId) {
    data.storage_backend_id = formData.value.storageBackendId
  }
  const storageProvider = resolvedStorageProvider()
  data.storage_provider_config = {
    provider: storageProvider
  }
  data.storage_config = {
    provider: storageProvider
  }

  // 添加知识图谱配置 — now synced via indexingStrategy.graphEnabled
  // extract_config is sent below along with indexing_strategy

  // 添加问题生成配置
  if (formData.value.questionGenerationConfig?.enabled) {
    data.question_generation_config = {
      enabled: true,
      question_count: formData.value.questionGenerationConfig.questionCount || 3,
      custom_instructions: formData.value.questionGenerationConfig.customInstructions || ''
    }
  } else {
    data.question_generation_config = {
      enabled: false,
      question_count: 3,
      custom_instructions: formData.value.questionGenerationConfig?.customInstructions || ''
    }
  }

  data.auto_tag_config = {
    enabled: formData.value.autoTagConfig?.enabled || false,
    model_id: formData.value.autoTagConfig?.modelId || '',
    max_tags: formData.value.autoTagConfig?.maxTags || 3,
    skip_if_tagged: formData.value.autoTagConfig?.skipIfTagged ?? true
  }

  data.profile_config = {
    enabled: formData.value.profileConfig?.enabled || false,
    model_id: formData.value.profileConfig?.modelId || '',
    custom_instructions: formData.value.profileConfig?.customInstructions || ''
  }

  if (formData.value.type === 'faq') {
    data.faq_config = {
      index_mode: formData.value.faqConfig?.indexMode || 'question_only',
      question_index_mode: formData.value.faqConfig?.questionIndexMode || 'separate'
    }
  }

  // Wiki enablement is carried solely by indexing_strategy.wiki_enabled.
  // wiki_config only holds wiki-specific tunables.
  if (formData.value.type !== 'faq') {
    data.wiki_config = {
      synthesis_model_id: formData.value.modelConfig?.wikiSynthesisModelId || '',
      max_pages_per_ingest: formData.value.wikiConfig?.maxPagesPerIngest || 0,
      extraction_granularity: formData.value.wikiConfig?.extractionGranularity || 'standard',
      content_instructions: formData.value.wikiConfig?.contentInstructions || '',
      extraction_instructions: formData.value.wikiConfig?.extractionInstructions || '',
    }
  }

  // Send indexing strategy
  if (formData.value.type !== 'faq') {
    data.indexing_strategy = {
      vector_enabled: formData.value.indexingStrategy?.vectorEnabled ?? true,
      keyword_enabled: formData.value.indexingStrategy?.keywordEnabled ?? true,
      wiki_enabled: formData.value.indexingStrategy?.wikiEnabled ?? false,
      graph_enabled: formData.value.indexingStrategy?.graphEnabled ?? false,
    }
  }

  // 建库模式：模型/向量存储/解析引擎/分块/图像/音频/图谱/高级/存储等分区，
  // 已从建库菜单移除，统一由平台「知识库设置」默认页维护。这里不下发这些
  // 字段，后端 ApplyTo 会套用平台默认值（仅填空字段，API/MCP 显式值优先）。
  // 保留基本信息：name/description/type、索引策略 indexing_strategy、Wiki 开关
  // wiki_config，以及问答库建库时用户自选的 faq_config。
  if (editorMode.value === 'create') {
    const platformManaged = [
      'chunking_config',
      'embedding_model_id', 'summary_model_id',
      'vector_store_id',
      'vlm_config', 'image_processing_config',
      'asr_config',
      'storage_backend_id', 'storage_provider_config', 'storage_config',
      'question_generation_config', 'auto_tag_config', 'profile_config',
      'extract_config',
    ]
    for (const key of platformManaged) {
      delete (data as Record<string, unknown>)[key]
    }
  }

  return data
}

// 提交表单
const handleSubmit = async () => {
  if (!validateForm()) {
    return
  }

  // 编辑模式下，若已有文件且存储引擎发生了变化，弹窗确认
  if (
    editorMode.value === 'edit' &&
    hasFiles.value &&
    formData.value &&
    initialStorageProvider.value &&
    formData.value.storageProvider !== initialStorageProvider.value
  ) {
    const dialog = DialogPlugin.confirm({
      header: t('common.confirm'),
      body: t('knowledgeEditor.messages.storageChangeConfirm'),
      confirmBtn: t('common.confirm'),
      cancelBtn: t('common.cancel'),
      onConfirm: () => {
        dialog.destroy()
        doSubmit()
      },
      onCancel: () => {
        dialog.destroy()
      },
    })
    return
  }

  doSubmit()
}

const doSubmit = async () => {
  saving.value = true
  try {
    const data = buildSubmitData()
    if (!data) {
      throw new Error(t('knowledgeEditor.messages.buildDataFailed'))
    }

    if (editorMode.value === 'create') {
      // 创建模式：一次性创建知识库及所有配置
      const result: any = await createKnowledgeBase(data)
      if (!result.success || !result.data?.id) {
        throw new Error(result.message || t('knowledgeEditor.messages.createFailed'))
      }
      const createdKbId = result.data.id as string
      savedKbId.value = createdKbId
      currentSection.value = 'basic'
      await loadKBData(createdKbId)
      MessagePlugin.success(t('knowledgeEditor.messages.createSuccess'))
      markContextualGuideDone('kbCreate')
      emit('success', createdKbId)
    } else {
      // 编辑模式：分别更新基本信息和配置
      const kbId = activeKbId.value
      if (!kbId) {
        throw new Error(t('knowledgeEditor.messages.missingId'))
      }

      // 1. 更新基本信息（名称、描述）和 FAQ/Wiki 配置
      const updateConfig: any = {}
      if (formData.value.type === 'faq' && formData.value.faqConfig) {
        updateConfig.faq_config = {
          index_mode: formData.value.faqConfig.indexMode || 'question_only',
          question_index_mode: formData.value.faqConfig.questionIndexMode || 'separate'
        }
      }
      if (formData.value.wikiConfig && formData.value.type !== 'faq') {
        updateConfig.wiki_config = {
          synthesis_model_id: formData.value.modelConfig?.wikiSynthesisModelId || '',
          max_pages_per_ingest: formData.value.wikiConfig.maxPagesPerIngest || 0,
          extraction_granularity: formData.value.wikiConfig.extractionGranularity || 'standard',
          content_instructions: formData.value.wikiConfig.contentInstructions || '',
          extraction_instructions: formData.value.wikiConfig.extractionInstructions || '',
        }
      }
      if (formData.value.type !== 'faq') {
        updateConfig.auto_tag_config = data.auto_tag_config
        updateConfig.profile_config = data.profile_config
        updateConfig.indexing_strategy = {
          vector_enabled: formData.value.indexingStrategy?.vectorEnabled ?? true,
          keyword_enabled: formData.value.indexingStrategy?.keywordEnabled ?? true,
          wiki_enabled: formData.value.indexingStrategy?.wikiEnabled ?? false,
          graph_enabled: formData.value.indexingStrategy?.graphEnabled ?? false,
        }
      }
      // 图片分类配置：buildSubmitData 只在与快照有差异时才产出该字段，带上即
      // 整体替换（后端语义：字段缺失 = 保持不变）。编辑模式此前漏带，导致 KB
      // 编辑器里对图片分类的修改被静默丢弃（新建时能存、之后再改无效）。
      if (data.image_processing_config) {
        updateConfig.image_processing_config = data.image_processing_config
      }
      await updateKnowledgeBase(kbId, {
        name: data.name,
        description: data.description,
        config: updateConfig
      })

      // 2. 更新完整配置（模型、分块、多模态、存储引擎、知识图谱等）
      const config: KBModelConfigRequest = {
        llmModelId: data.summary_model_id,
        embeddingModelId: data.embedding_model_id,
        vlm_config: data.vlm_config,
        asr_config: data.asr_config,
        documentSplitting: {
          chunkSize: data.chunking_config.chunk_size,
          chunkOverlap: data.chunking_config.chunk_overlap,
          separators: data.chunking_config.separators,
          parserEngineRules: data.chunking_config.parser_engine_rules || undefined,
          enableParentChild: data.chunking_config.enable_parent_child || false,
          parentChunkSize: data.chunking_config.parent_chunk_size || 4096,
          childChunkSize: data.chunking_config.child_chunk_size || 384,
          // Always send strategy / tokenLimit / languages — backend treats
          // empty/0/[] as a valid clear, so we must include them in the
          // payload to let users reset back to defaults.
          strategy: formData.value?.chunkingConfig.strategy ?? '',
          tokenLimit: formData.value?.chunkingConfig.tokenLimit ?? 0,
          languages: formData.value?.chunkingConfig.languages ?? [],
          tableMetadataInstructions: formData.value?.chunkingConfig.tableMetadataInstructions ?? ''
        },
        multimodal: {
          enabled: !!data.vlm_config?.enabled
        },
        storageBackendId: formData.value?.storageBackendId || '',
        storageProvider: data.storage_provider_config?.provider || data.storage_config?.provider || 'local',
        questionGeneration: {
          enabled: data.question_generation_config?.enabled || false,
          questionCount: data.question_generation_config?.question_count || 3,
          customInstructions: data.question_generation_config?.custom_instructions || ''
        }
      }

      // 图谱配置只在平台「知识库设置」维护，这里不发送 nodeExtract：
      // 后端按「保持不变」处理，避免存量的不完整配置让整次保存失败。

      await updateKBConfig(kbId, config)
      MessagePlugin.success(t('knowledgeEditor.messages.updateSuccess'))

      // Check if indexing strategy changed and offer rebuild
      if (hasFiles.value && initialIndexingStrategy.value && formData.value) {
        const curr = formData.value.indexingStrategy
        const prev = initialIndexingStrategy.value
        const strategyChanged = (
          curr.vectorEnabled !== prev.vectorEnabled ||
          curr.keywordEnabled !== prev.keywordEnabled ||
          curr.wikiEnabled !== prev.wikiEnabled ||
          curr.graphEnabled !== prev.graphEnabled
        )
        if (strategyChanged) {
          const dialog = DialogPlugin.confirm({
            header: t('knowledgeEditor.indexing.rebuildConfirmTitle'),
            body: t('knowledgeEditor.indexing.rebuildConfirmBody', { count: '...' }),
            confirmBtn: t('common.confirm'),
            cancelBtn: t('common.cancel'),
            onConfirm: async () => {
              dialog.destroy()
              try {
                const result: any = await rebuildKBIndex(kbId)
                const count = result?.data?.document_count ?? 0
                MessagePlugin.success(t('knowledgeEditor.indexing.rebuildSuccess', { count }))
              } catch (e) {
                console.error('Rebuild index failed:', e)
              }
            },
            onCancel: () => {
              dialog.destroy()
              MessagePlugin.info(t('knowledgeEditor.indexing.rebuildSkip'))
            },
          })
        }
      }

      emit('success', kbId)
      handleClose()
    }
  } catch (error: any) {
    console.error('Knowledge base operation failed:', error)
    // Vector-store-binding error codes from the server. Both indicate
    // the selected store cannot be used: 2200 is "the binding itself
    // is invalid" (e.g. unknown id, foreign tenant), 2201 is "the
    // store is currently unreachable". For either, swap in a localized
    // message and jump the user back to the Vector Store section so
    // they can pick a different store or fall back to the system
    // default.
    const code = error?.response?.data?.error?.code ?? error?.code
    if (code === 2200) {
      MessagePlugin.error(t('knowledgeEditor.errors.vectorStoreBindingInvalid'))
      currentSection.value = 'vectorStore'
    } else if (code === 2201) {
      MessagePlugin.error(t('knowledgeEditor.errors.vectorStoreUnavailable'))
      currentSection.value = 'vectorStore'
    } else {
      MessagePlugin.error(error?.message || t('common.operationFailed'))
    }
  } finally {
    saving.value = false
  }
}

// 重置所有状态
const resetState = () => {
  savedKbId.value = null
  currentSection.value = 'basic'
  formData.value = null
  hasFiles.value = false
  initialStorageProvider.value = ''
  tenantDefaultStorageProvider.value = 'local'
  initialIndexingStrategy.value = null
  saving.value = false
  loading.value = false
  kbCreatorId.value = ''
  kbTenantId.value = 0
}

// 关闭弹窗
const handleClose = () => {
  emit('update:visible', false)
  setTimeout(() => {
    if (props.visible) return
    resetState()
  }, 300)
}

const modalShell = useModalShell({
  visible: () => props.visible,
  close: handleClose,
  snapshot: () => formData.value,
})

// 监听弹窗打开/关闭
watch(() => props.visible, async (newVal) => {
  const generation = ++kbEditorLoadGeneration
  if (newVal) {
    // 打开弹窗时，先重置状态
    resetState()
    modalShell.markClean()
    loading.value = true
    const targetKbId = props.kbId
    
    // 检查是否有初始 section，如果有则跳转
    if (uiStore.kbEditorInitialSection) {
      currentSection.value = uiStore.kbEditorInitialSection
    }
    
    // 加载模型列表与空间默认存储引擎（创建 KB 时即使用，不依赖是否打开「存储引擎」Tab）
    await Promise.all([loadAllModels(), loadTenantDefaultStorageProvider()])

    if (generation !== kbEditorLoadGeneration || !props.visible) return
    
    // 根据模式加载数据
    if (props.mode === 'edit' && targetKbId) {
      await loadKBData(targetKbId, generation)
    } else {
      // 创建模式：初始化空表单，并预填空间默认存储引擎
      formData.value = initFormData(props.initialType || 'document')
      formData.value.storageProvider = tenantDefaultStorageProvider.value
      hasFiles.value = false
      applyDefaultModelsIfEmpty()
      loading.value = false
      modalShell.markClean()
    }
    // 数据加载完成后，若初始/深链分区不在当前角色可见菜单里，回退到基本信息。
    ensureVisibleSection()
  } else {
    // 关闭弹窗时，延迟重置状态（等待动画结束）
    setTimeout(() => {
      if (props.visible) return
      resetState()
      currentSection.value = 'basic' // 重置为默认 section
    }, 300)
  }
})

// 监听全局设置弹窗关闭后刷新模型列表
watch(
  () => uiStore.showSettingsModal,
  async (visible, previous) => {
    if (!visible && previous && props.visible) {
      await loadAllModels(true)
    }
  }
)

watch(() => chatResources.allModels, (list) => {
  if (props.visible) {
    allModels.value = list || []
  }
})
</script>

<style scoped lang="less">
// 图片属性面板（可观察属性 / OCR 触发条件 / 观察失败兜底）
// 文案全部来自后端属性注册表（人话名 + 每个取值的含义），前端只做翻译覆盖，
// 因此新增属性无需改这里。铺在所在设置区里、不套独立底色块（与整体风格一致）。
.image-attr-panel {
  margin-top: 4px;
}
.image-attr-list,
.image-attr-value-list,
.image-attr-condition-list {
  margin: 0;
  padding: 0;
  list-style: none;
}
.image-attr-row + .image-attr-row {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--td-component-border);
}
.image-attr-head {
  display: flex;
  align-items: baseline;
  gap: 6px;
  flex-wrap: wrap;
}
.image-attr-label {
  font-weight: 600;
  color: var(--td-text-color-primary);
}
// 原始属性名缩小并加括号，便于与处理轨迹里的字段对上，又不干扰阅读
.image-attr-name {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-placeholder);

  &::before {
    content: '(';
  }

  &::after {
    content: ')';
  }
}
.image-attr-desc {
  margin: 2px 0 0;
  font-size: var(--app-text-sm);
  line-height: 20px;
  color: var(--td-text-color-secondary);
}
// 每个取值一行：值（等宽，左列对齐）+ 冒号 + 人话解释
.image-attr-value-list {
  display: grid;
  grid-template-columns: max-content 1fr;
  column-gap: 2px;
  row-gap: 2px;
  margin-top: 6px;
  font-size: var(--app-text-sm);
}
// display: contents 让值与解释分别落进上面两列，取值因此左对齐成列
.image-attr-value {
  display: contents;

  code {
    font-family: ui-monospace, Menlo, Consolas, monospace;
    color: var(--td-text-color-primary);

    // 冒号紧跟取值，不留空隙：none: 没有文字
    &::after {
      content: ':';
    }
  }
}
.image-attr-value-label {
  color: var(--td-text-color-secondary);
}
.image-attr-section {
  margin-top: 14px;
}
// 一条 OCR 条件两行：人话在上，原始 property = value 在下
.image-attr-condition-list {
  margin-top: 6px;
  font-size: var(--app-text-sm);
}
.image-attr-condition + .image-attr-condition {
  margin-top: 6px;
}
.image-attr-condition-label {
  display: block;
  color: var(--td-text-color-primary);
}
.image-attr-condition-raw {
  display: block;
  margin-top: 1px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-placeholder);
}

// 复用创建知识库的样式
/* 左侧导航：与 AgentEditorModal 对齐 */
.content-wrapper {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
}

.section {
  margin-bottom: 32px;

  &:last-child {
    margin-bottom: 0;
  }
}

.section-content {
  .section-header {
    margin-bottom: 16px;
  }

  .section-title {
    margin: 0 0 6px 0;
    font-family: var(--app-font-family);
    font-size: var(--app-text-3xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .section-desc {
    margin: 0;
    font-family: var(--app-font-family);
    font-size: var(--app-text-base);
    color: var(--td-text-color-placeholder);
    line-height: 22px;
  }

  .section-body {
    background: var(--td-bg-color-container);
  }
}

.form-item {
  margin-bottom: 16px;

  &:last-child {
    margin-bottom: 0;
  }
}

.form-label {
  display: block;
  margin-bottom: 8px;
  font-family: var(--app-font-family);
  font-size: var(--app-text-lg);
  font-weight: 500;
  color: var(--td-text-color-primary);

  &.required::after {
    content: '*';
    color: var(--td-error-color);
    margin-left: 4px;
  }
}

.form-tip {
  margin-top: 6px;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
}

.kb-id-field {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  max-width: 480px;
  margin-top: 8px;
  padding: 6px 8px 6px 12px;
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-sm);

  .kb-id-value {
    flex: 1;
    min-width: 0;
    margin: 0;
    padding: 0;
    background: none;
    border: none;
    font-family: var(--app-font-family-mono);
    font-size: var(--app-text-md);
    line-height: 1.5;
    color: var(--td-text-color-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kb-id-copy {
    flex-shrink: 0;
    color: var(--td-text-color-secondary);

    &:hover {
      color: var(--td-brand-color);
    }
  }
}

.granularity-radio-group {
  margin-top: 4px;
}

.granularity-hint {
  margin-top: 8px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
  white-space: normal;
  word-break: break-word;
}

.indexing-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
  margin-top: 10px;
}

.indexing-check-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-container);
  cursor: pointer;
  user-select: none;
  transition: border-color var(--app-motion-base) ease, background var(--app-motion-base) ease;

  &:hover {
    border-color: var(--td-brand-color);
  }

  &.is-checked {
    border-color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }

  &.is-disabled {
    cursor: not-allowed;
    background: var(--td-bg-color-secondarycontainer);

    &:hover {
      border-color: var(--td-component-stroke);
    }

    &.is-checked {
      border-color: var(--td-brand-color-focus);
      background: var(--td-brand-color-light);
    }

    &:not(.is-checked) {
      opacity: 0.6;
    }
  }

  :deep(.t-checkbox__label) {
    font-weight: 500;
    color: var(--td-text-color-primary);
  }
}

.locked-tip {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin: 8px 0 0;
  font-size: var(--app-text-sm);
  line-height: 18px;
  color: var(--td-text-color-secondary);

  .locked-tip-icon {
    flex-shrink: 0;
    margin-top: 2px;
    font-size: var(--app-text-base);
    color: var(--td-text-color-placeholder);
  }
}

// 禁用内部 checkbox 自身的点击事件，统一由卡片处理
.indexing-check-box {
  pointer-events: none;
}

.indexing-check-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.indexing-new-badge {
  display: inline-flex;
  align-items: center;
  padding: 0 6px;
  height: 16px;
  border-radius: 3px;
  font-size: var(--app-text-2xs);
  font-weight: 600;
  line-height: 1;
  letter-spacing: 0.4px;
  color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.indexing-check-desc {
  margin: 0;
  padding-left: 24px;
  font-size: var(--app-text-sm);
  line-height: 18px;
  color: var(--td-text-color-placeholder);
}

// 建库时内联的 FAQ 索引策略：紧跟类型选择，与文档库的「索引策略」同位。
.faq-inline-settings {
  margin-bottom: 16px;
}

// 过渡动画
// 多模态配置内联样式（与子组件 KBStorageSettings/KBAdvancedSettings 一致）
.kb-multimodal-settings {
  width: 100%;

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
  }

  .setting-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding: 16px 0;
    border-bottom: 1px solid var(--td-component-stroke);

    &:last-child {
      border-bottom: none;
    }
  }

  // 纵向行（类别 OCR 策略表用）：说明文字占满整行，表格换到下一行
  .setting-row-vertical {
    flex-direction: column;
    align-items: stretch;

    > .setting-info {
      max-width: 100%;
      padding-right: 0;
    }
  }

  .setting-info {
    flex: 1;
    max-width: 65%;
    padding-right: 24px;

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

    .desc-warning {
      margin-top: 4px;
      color: var(--td-warning-color);
    }
  }

  .setting-control {
    flex-shrink: 0;
    min-width: 280px;
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .required {
    color: var(--td-error-color);
    margin-left: 2px;
    font-weight: 500;
  }
}

.kb-profile-card {
  padding: 12px 14px 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-container);

  &.is-empty {
    padding: 12px 14px;
    border-style: dashed;
    background: var(--td-bg-color-secondarycontainer);
  }

  .kb-profile-gist {
    margin: 0 0 10px;
    font-size: var(--app-text-base);
    line-height: 1.6;
    color: var(--td-text-color-primary);
  }

  .kb-profile-topics {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 10px;
  }

  .kb-profile-subtitle {
    margin: 0 0 4px;
    font-size: var(--app-text-sm);
    font-weight: 500;
    color: var(--td-text-color-secondary);
  }

  .kb-profile-questions ul {
    margin: 0 0 4px;
    padding-left: 18px;
    font-size: var(--app-text-md);
    line-height: 1.7;
    color: var(--td-text-color-secondary);
  }

  .kb-profile-error {
    margin: 4px 0 0;
    font-size: var(--app-text-sm);
    color: var(--td-error-color);
  }

  .kb-profile-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--td-component-stroke);
  }

  .kb-profile-meta {
    min-width: 0;
    font-size: var(--app-text-sm);
    color: var(--td-text-color-placeholder);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kb-profile-actions {
    display: flex;
    flex-shrink: 0;
    gap: 4px;
    align-items: center;
  }

  .kb-profile-empty {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .kb-profile-empty-icon {
    display: inline-flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border-radius: var(--app-radius-md);
    font-size: var(--app-text-2xl);
    color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }

  .kb-profile-empty-text {
    flex: 1;
    min-width: 0;

    p {
      margin: 0;
      font-size: var(--app-text-md);
      line-height: 1.5;
      color: var(--td-text-color-secondary);
    }
  }
}
</style>

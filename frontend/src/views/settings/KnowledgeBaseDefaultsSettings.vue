<template>
  <div class="kb-defaults-settings" :class="{ 'kb-defaults-settings--readonly': !canEdit }">
    <div class="section-header">
      <h2>{{ $t('kbDefaults.title') }}</h2>
      <p class="section-description">{{ $t('kbDefaults.description') }}</p>
    </div>

    <div v-if="!canEdit" class="kb-defaults-readonly-banner">
      <t-icon name="lock-on" />
      <span>{{ $t('kbDefaults.readonlyBanner') }}</span>
    </div>

    <t-loading :loading="loading" size="small">
      <div v-if="ready" class="kb-defaults-content">
        <!-- B 默认模型（复用编辑器 KBModelConfig） -->
        <div v-show="activeSection === 'models'" class="kb-defaults-block">
          <KBModelConfig
            :config="cfg.modelConfig"
            :has-files="false"
            :wiki-enabled="false"
            :rag-enabled="true"
            :all-models="allModels"
            @update:config="(v: any) => (cfg.modelConfig = v)"
          />
        </div>

        <!-- C 向量存储（仅运营管理员可见，超管不显示） -->
        <div v-if="canOpsEdit" v-show="activeSection === 'vectorStore'" class="kb-defaults-block">
          <KBVectorStoreSettings
            mode="create"
            :vector-store-id="cfg.vectorStoreId"
            @update:vector-store-id="(v: string) => (cfg.vectorStoreId = v)"
          />
        </div>

        <!-- D 解析引擎（仅运营管理员可见，超管不显示） -->
        <div v-if="canOpsEdit" v-show="activeSection === 'parser'" class="kb-defaults-block">
          <KBParserSettings
            :parser-engine-rules="cfg.parserEngineRules"
            @update:parser-engine-rules="(v: any) => (cfg.parserEngineRules = v)"
          />
        </div>

        <!-- E 分块设置（复用编辑器 KBChunkingSettings） -->
        <div v-show="activeSection === 'chunking'" class="kb-defaults-block">
          <KBChunkingSettings
            :config="cfg.chunkingConfig"
            @update:config="handleChunkingUpdate"
          />
        </div>

        <!-- F 图像处理（复用编辑器多模态字段） -->
        <div v-show="activeSection === 'image'" class="kb-defaults-block">
          <div class="kb-inline-section">
            <div class="section-header">
              <h2>{{ $t('knowledgeEditor.multimodal.title') }}</h2>
              <p class="section-description">{{ $t('knowledgeEditor.multimodal.description') }}</p>
            </div>
            <div class="settings-group">
              <div class="setting-row">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.advanced.multimodal.label') }}</label>
                  <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.description') }}</p>
                </div>
                <div class="setting-control">
                  <t-switch v-model="cfg.multimodalConfig.enabled" size="medium" />
                </div>
              </div>

              <div v-if="cfg.multimodalConfig.enabled" class="setting-row">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.advanced.multimodal.vllmLabel') }} <span class="required">*</span></label>
                  <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.vllmDescription') }}</p>
                </div>
                <div class="setting-control">
                  <ModelSelector
                    model-type="VLLM"
                    :selected-model-id="cfg.multimodalConfig.vllmModelId"
                    :all-models="allModels"
                    :show-add-model="false"
                    clearable
                    :placeholder="$t('knowledgeEditor.advanced.multimodal.vllmPlaceholder')"
                    @update:selected-model-id="(v: string) => (cfg.multimodalConfig.vllmModelId = v)"
                  />
                </div>
              </div>

              <div v-if="cfg.multimodalConfig.enabled" class="setting-row">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.advanced.multimodal.descriptionLanguageLabel') }}</label>
                  <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.descriptionLanguageDescription') }}</p>
                </div>
                <div class="setting-control">
                  <t-select v-model="cfg.multimodalConfig.descriptionLanguage" clearable
                    :placeholder="$t('knowledgeEditor.advanced.multimodal.descriptionLanguageAuto')">
                    <t-option value="Chinese" :label="$t('language.zhCN')" />
                    <t-option value="English" :label="$t('language.enUS')" />
                    <t-option value="Korean" :label="$t('language.koKR')" />
                    <t-option value="Russian" :label="$t('language.ruRU')" />
                  </t-select>
                </div>
              </div>

              <div v-if="cfg.multimodalConfig.enabled" class="setting-row setting-row-vertical">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.advanced.multimodal.customInstructionsLabel') }}</label>
                  <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.customInstructionsDescription') }}</p>
                </div>
                <div class="setting-control setting-control-full">
                  <t-textarea v-model="cfg.multimodalConfig.customInstructions"
                    :placeholder="$t('knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder')"
                    :maxlength="4000" :autosize="{ minRows: 3, maxRows: 8 }" />
                </div>
              </div>

              <div v-if="cfg.multimodalConfig.enabled" class="setting-row">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.advanced.multimodal.imageVectorLabel') }}</label>
                  <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.imageVectorDescription') }}</p>
                </div>
                <div class="setting-control">
                  <t-switch v-model="cfg.imageVectorEnabled" size="medium" />
                </div>
              </div>

              <div v-if="cfg.multimodalConfig.enabled" class="setting-row">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsLabel') }}</label>
                  <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsDescription') }}</p>
                </div>
                <div class="setting-control">
                  <t-switch v-model="cfg.imageAttrsEnabled" size="medium" />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- G 音频处理（复用编辑器 ASR 字段） -->
        <div v-show="activeSection === 'audio'" class="kb-defaults-block">
          <div class="kb-inline-section">
            <div class="section-header">
              <h2>{{ $t('knowledgeEditor.asr.title') }}</h2>
              <p class="section-description">{{ $t('knowledgeEditor.asr.description') }}</p>
            </div>
            <div class="settings-group">
              <div class="setting-row">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.asr.label') }}</label>
                  <p class="desc">{{ $t('knowledgeEditor.asr.desc') }}</p>
                </div>
                <div class="setting-control">
                  <t-switch v-model="cfg.asrConfig.enabled" size="medium" />
                </div>
              </div>

              <div v-if="cfg.asrConfig.enabled" class="setting-row">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.asr.modelLabel') }} <span class="required">*</span></label>
                  <p class="desc">{{ $t('knowledgeEditor.asr.modelDescription') }}</p>
                </div>
                <div class="setting-control">
                  <ModelSelector
                    model-type="ASR"
                    :selected-model-id="cfg.asrConfig.modelId"
                    :all-models="allModels"
                    :show-add-model="false"
                    clearable
                    :placeholder="$t('knowledgeEditor.asr.modelPlaceholder')"
                    @update:selected-model-id="(v: string) => (cfg.asrConfig.modelId = v)"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- H 知识图谱（复用编辑器 GraphSettings） -->
        <div v-show="activeSection === 'graph'" class="kb-defaults-block">
          <GraphSettings
            :graph-extract="cfg.nodeExtractConfig"
            :model-id="cfg.modelConfig.llmModelId"
            :all-models="allModels"
            @update:graphExtract="(v: any) => (cfg.nodeExtractConfig = v)"
          />
        </div>

        <!-- I 高级设置（复用编辑器 KBAdvancedSettings） -->
        <div v-show="activeSection === 'advanced'" class="kb-defaults-block">
          <KBAdvancedSettings
            :question-generation="cfg.questionGenerationConfig"
            :auto-tag="cfg.autoTagConfig"
            :profile-config="cfg.profileConfig"
            :rag-enabled="true"
            :all-models="allModels"
            :table-metadata-instructions="cfg.chunkingConfig.tableMetadataInstructions"
            @update:question-generation="(v: any) => (cfg.questionGenerationConfig = v)"
            @update:auto-tag="(v: any) => (cfg.autoTagConfig = v)"
            @update:profile-config="(v: any) => (cfg.profileConfig = v)"
            @update:table-metadata-instructions="(v: string) => (cfg.chunkingConfig.tableMetadataInstructions = v)"
          />
        </div>

        <!-- I-1 FAQ 设置（紧跟高级设置）：问答型知识库专属，建库时由后端套用 -->
        <div v-show="activeSection === 'faq'" class="kb-defaults-block kb-faq-settings">
          <div class="section-header">
            <h2>{{ $t('kbDefaults.faqTitle') }}</h2>
            <p class="section-description">{{ $t('kbDefaults.faqHint') }}</p>
          </div>
          <div class="settings-group">
            <div class="setting-row">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.faq.indexModeLabel') }} <span class="required">*</span></label>
                <p class="desc">{{ $t('knowledgeEditor.faq.indexModeDescription') }}</p>
              </div>
              <div class="setting-control">
                <t-radio-group v-model="cfg.faqConfig.indexMode">
                  <t-radio-button value="question_only">{{ $t('knowledgeEditor.faq.modes.questionOnly') }}</t-radio-button>
                  <t-radio-button value="question_answer">{{ $t('knowledgeEditor.faq.modes.questionAnswer') }}</t-radio-button>
                </t-radio-group>
              </div>
            </div>
            <div class="setting-row">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.faq.questionIndexModeLabel') }} <span class="required">*</span></label>
                <p class="desc">{{ $t('knowledgeEditor.faq.questionIndexModeDescription') }}</p>
              </div>
              <div class="setting-control">
                <t-radio-group v-model="cfg.faqConfig.questionIndexMode">
                  <t-radio-button value="combined">{{ $t('knowledgeEditor.faq.modes.combined') }}</t-radio-button>
                  <t-radio-button value="separate">{{ $t('knowledgeEditor.faq.modes.separate') }}</t-radio-button>
                </t-radio-group>
              </div>
            </div>
          </div>
        </div>

        <!-- J 存储引擎（仅运营管理员可见，超管不显示） -->
        <div v-if="canOpsEdit" v-show="activeSection === 'storage'" class="kb-defaults-block">
          <KBStorageSettings
            :storage-backend-id="cfg.storageBackendId"
            :storage-provider="cfg.storageProvider"
            :has-files="false"
            @update:storage-backend-id="(v: string) => (cfg.storageBackendId = v)"
            @update:storage-provider="(v: string) => (cfg.storageProvider = v)"
          />
        </div>

        <!-- K 数据源（仅运营管理员；工作区级集中管理，已从知识库弹窗移除）。
             数据源仍同步进具体知识库，故先选目标知识库再复用现有数据源管理 UI。 -->
        <div v-if="canOpsEdit" v-show="activeSection === 'datasource'" class="kb-defaults-block">
          <div class="kb-defaults-ds-bar">
            <label class="kb-defaults-ds-bar__label">{{ $t('kbDefaults.datasource.targetKb') }}</label>
            <t-select
              v-model="dsSelectedKbId"
              class="kb-defaults-ds-bar__select"
              filterable
              :placeholder="$t('kbDefaults.datasource.selectKb')"
            >
              <t-option v-for="kb in kbList" :key="kb.id" :value="kb.id" :label="kb.name" />
            </t-select>
          </div>
          <p class="kb-defaults-ds-hint">{{ $t('kbDefaults.datasource.hint') }}</p>
          <DataSourceSettings v-if="dsSelectedKbId" :key="dsSelectedKbId" :kb-id="dsSelectedKbId" />
          <p v-else class="kb-defaults-ds-empty">{{ $t('kbDefaults.datasource.selectKb') }}</p>
        </div>
        </div>
    </t-loading>

    <div v-if="canEdit && ready" class="kb-defaults-footer">
      <span class="kb-defaults-footer__hint">{{ $t('kbDefaults.appliesHint') }}</span>
      <span
        v-if="autosaveState !== 'idle'"
        class="kb-defaults-footer__status"
        :class="autosaveState"
      >
        <t-icon
          :name="autosaveState === 'saving' ? 'loading' : 'check-circle'"
          :spin="autosaveState === 'saving'"
        />
        <span>{{ autosaveState === 'saving' ? $t('kbDefaults.saving') : $t('kbDefaults.autoSaved') }}</span>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import ModelSelector from '@/components/ModelSelector.vue'
import KBModelConfig from '@/views/knowledge/settings/KBModelConfig.vue'
import KBChunkingSettings from '@/views/knowledge/settings/KBChunkingSettings.vue'
import GraphSettings from '@/views/knowledge/settings/GraphSettings.vue'
import KBAdvancedSettings from '@/views/knowledge/settings/KBAdvancedSettings.vue'
import KBVectorStoreSettings from '@/views/knowledge/settings/KBVectorStoreSettings.vue'
import KBParserSettings from '@/views/knowledge/settings/KBParserSettings.vue'
import KBStorageSettings from '@/views/knowledge/settings/KBStorageSettings.vue'
import DataSourceSettings from '@/views/knowledge/settings/DataSourceSettings.vue'
import { listModels, type ModelConfig } from '@/api/model'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { getSystemSetting, updateSystemSetting } from '@/api/system'
import { useAuthStore } from '@/stores/auth'

const KB_DEFAULTS_KEY = 'knowledge_base.default_config'

const { t } = useI18n()
const authStore = useAuthStore()
// The KB defaults page is a platform super-admin concern; the backend PUT is
// SystemAdmin-gated. Operations admins inherit isSystemAdmin so they also pass.
const canEdit = computed(() => authStore.isSystemAdmin)
// 向量存储(C) / 解析引擎(D) / 存储引擎(J) 三块仅运营管理员可见可编辑，
// 连超级管理员也不显示（运营管理员高于超管且继承其权限）。
const canOpsEdit = computed(() => authStore.isOperationsAdmin)

// 当前展示的分区由外层设置侧栏通过 activeSection 传入（知识库配置已提升到主侧栏，
// 本组件不再自带二级导航）。分区键与外层 kb-* 导航项一一对应，默认落在模型配置。
withDefaults(defineProps<{ activeSection?: string }>(), { activeSection: 'models' })

const loading = ref(false)
const saving = ref(false)
// 实时自动保存状态：idle（尚未编辑过，不显示）/ saving / saved。
const autosaveState = ref<'idle' | 'saving' | 'saved'>('idle')
// Render the reused section components only after the stored config has been
// merged in — they seed their internal refs from props on mount, so mounting
// them against defaults first would drop the persisted values.
const ready = ref(false)
const allModels = ref<ModelConfig[]>([])

// 数据源集中管理（仅运营管理员）：列出当前工作区的知识库供选择目标库。
const kbList = ref<{ id: string; name: string }[]>([])
const dsSelectedKbId = ref('')

// camelCase UI model, mirroring the KB editor's formData subset so the very
// same section components can be reused verbatim.
interface ModelConfigUI {
  llmModelId: string
  embeddingModelId: string
  wikiSynthesisModelId: string
}
interface ChunkingConfigUI {
  chunkSize: number
  chunkOverlap: number
  separators: string[]
  enableParentChild: boolean
  parentChunkSize: number
  childChunkSize: number
  strategy: string
  tokenLimit: number
  languages: string[]
  tableMetadataInstructions: string
}
interface MultimodalConfigUI {
  enabled: boolean
  vllmModelId: string
  descriptionLanguage: string
  customInstructions: string
}
interface ASRConfigUI {
  enabled: boolean
  modelId: string
  language: string
}
interface NodeExtractUI {
  enabled: boolean
  text: string
  tags: string[]
  nodes: any[]
  relations: any[]
  customInstructions: string
}
interface QuestionGenerationUI {
  enabled: boolean
  questionCount: number
  customInstructions: string
}
interface AutoTagUI {
  enabled: boolean
  modelId: string
  maxTags: number
  skipIfTagged: boolean
}
interface ProfileUI {
  enabled: boolean
  modelId: string
  customInstructions: string
}
interface FAQConfigUI {
  indexMode: string
  questionIndexMode: string
}
interface KBDocDefaults {
  modelConfig: ModelConfigUI
  chunkingConfig: ChunkingConfigUI
  multimodalConfig: MultimodalConfigUI
  imageAttrsEnabled: boolean
  imageVectorEnabled: boolean
  asrConfig: ASRConfigUI
  nodeExtractConfig: NodeExtractUI
  questionGenerationConfig: QuestionGenerationUI
  autoTagConfig: AutoTagUI
  profileConfig: ProfileUI
  // FAQ 索引策略（问答型知识库专属，入口在高级设置下方）。
  faqConfig: FAQConfigUI
  // Ops-admin-only platform defaults (C/D/J).
  vectorStoreId: string
  storageBackendId: string
  storageProvider: string
  parserEngineRules: any[]
}

// Built-in fallbacks mirror the KB editor's initFormData('document') so a fresh
// platform config still produces valid knowledge bases.
function makeDefaults(): KBDocDefaults {
  return {
    modelConfig: { llmModelId: '', embeddingModelId: '', wikiSynthesisModelId: '' },
    chunkingConfig: {
      chunkSize: 512,
      chunkOverlap: 80,
      separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      strategy: 'auto',
      tokenLimit: 0,
      languages: [],
      tableMetadataInstructions: '',
    },
    multimodalConfig: { enabled: false, vllmModelId: '', descriptionLanguage: '', customInstructions: '' },
    imageAttrsEnabled: false,
    imageVectorEnabled: false,
    asrConfig: { enabled: false, modelId: '', language: '' },
    nodeExtractConfig: { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '' },
    questionGenerationConfig: { enabled: true, questionCount: 3, customInstructions: '' },
    autoTagConfig: { enabled: false, modelId: '', maxTags: 3, skipIfTagged: true },
    profileConfig: { enabled: false, modelId: '', customInstructions: '' },
    // Mirror KnowledgeBase.EnsureDefaults() for FAQ knowledge bases so the page
    // shows the value a fresh FAQ KB would actually get.
    faqConfig: { indexMode: 'question_answer', questionIndexMode: 'combined' },
    vectorStoreId: '',
    storageBackendId: '',
    storageProvider: '',
    parserEngineRules: [],
  }
}

const cfg = reactive<KBDocDefaults>(makeDefaults())

function handleChunkingUpdate(v: Partial<ChunkingConfigUI>) {
  // KBChunkingSettings emits its own ChunkingConfig shape (without
  // tableMetadataInstructions, which is edited via KBAdvancedSettings); merge so
  // the table instructions survive.
  cfg.chunkingConfig = { ...cfg.chunkingConfig, ...v }
}

// Serialize the camelCase UI model into the KnowledgeBase snake_case shape that
// the backend KBDefaultsConfig / ApplyTo consume directly.
function toStorageShape(): Record<string, unknown> {
  return {
    summary_model_id: cfg.modelConfig.llmModelId || '',
    embedding_model_id: cfg.modelConfig.embeddingModelId || '',
    chunking_config: {
      chunk_size: cfg.chunkingConfig.chunkSize,
      chunk_overlap: cfg.chunkingConfig.chunkOverlap,
      separators: cfg.chunkingConfig.separators,
      enable_parent_child: cfg.chunkingConfig.enableParentChild,
      parent_chunk_size: cfg.chunkingConfig.parentChunkSize,
      child_chunk_size: cfg.chunkingConfig.childChunkSize,
      strategy: cfg.chunkingConfig.strategy ?? '',
      token_limit: cfg.chunkingConfig.tokenLimit ?? 0,
      languages: cfg.chunkingConfig.languages ?? [],
      table_metadata_instructions: cfg.chunkingConfig.tableMetadataInstructions || '',
      ...(cfg.parserEngineRules?.length ? { parser_engine_rules: cfg.parserEngineRules } : {}),
    },
    vlm_config: {
      enabled: cfg.multimodalConfig.enabled,
      model_id: cfg.multimodalConfig.enabled ? (cfg.multimodalConfig.vllmModelId || '') : '',
      description_language: cfg.multimodalConfig.descriptionLanguage || '',
      custom_instructions: cfg.multimodalConfig.customInstructions || '',
    },
    image_processing_config: {
      model_id: cfg.multimodalConfig.vllmModelId || '',
      image_attrs_enabled: cfg.imageAttrsEnabled,
      image_vector_enabled: cfg.imageVectorEnabled,
    },
    asr_config: {
      enabled: cfg.asrConfig.enabled,
      model_id: cfg.asrConfig.enabled ? (cfg.asrConfig.modelId || '') : '',
      language: cfg.asrConfig.language || '',
    },
    extract_config: {
      enabled: !!cfg.nodeExtractConfig.enabled,
      text: cfg.nodeExtractConfig.text || '',
      tags: cfg.nodeExtractConfig.tags || [],
      nodes: cfg.nodeExtractConfig.nodes || [],
      relations: cfg.nodeExtractConfig.relations || [],
      custom_instructions: cfg.nodeExtractConfig.customInstructions || '',
    },
    question_generation_config: {
      enabled: !!cfg.questionGenerationConfig.enabled,
      question_count: cfg.questionGenerationConfig.questionCount || 3,
      custom_instructions: cfg.questionGenerationConfig.customInstructions || '',
    },
    auto_tag_config: {
      enabled: !!cfg.autoTagConfig.enabled,
      model_id: cfg.autoTagConfig.modelId || '',
      max_tags: cfg.autoTagConfig.maxTags || 3,
      skip_if_tagged: cfg.autoTagConfig.skipIfTagged ?? true,
    },
    profile_config: {
      enabled: !!cfg.profileConfig.enabled,
      model_id: cfg.profileConfig.modelId || '',
      custom_instructions: cfg.profileConfig.customInstructions || '',
    },
    faq_config: {
      index_mode: cfg.faqConfig.indexMode || '',
      question_index_mode: cfg.faqConfig.questionIndexMode || '',
    },
    // Ops-admin-only (C/D/J). Parser rules ride inside chunking_config above.
    vector_store_id: cfg.vectorStoreId || '',
    storage_backend_id: cfg.storageBackendId || '',
    storage_provider_config: { provider: cfg.storageProvider || '' },
    storage_config: { provider: cfg.storageProvider || '' },
  }
}

// Merge a stored snake_case document back into the camelCase UI model.
function fromStorageShape(raw: Record<string, any>) {
  const mc = raw.chunking_config
  if (raw.summary_model_id != null) cfg.modelConfig.llmModelId = raw.summary_model_id || ''
  if (raw.embedding_model_id != null) cfg.modelConfig.embeddingModelId = raw.embedding_model_id || ''
  if (mc) {
    cfg.chunkingConfig = {
      chunkSize: mc.chunk_size ?? cfg.chunkingConfig.chunkSize,
      chunkOverlap: mc.chunk_overlap ?? cfg.chunkingConfig.chunkOverlap,
      separators: Array.isArray(mc.separators) ? mc.separators : cfg.chunkingConfig.separators,
      enableParentChild: mc.enable_parent_child ?? cfg.chunkingConfig.enableParentChild,
      parentChunkSize: mc.parent_chunk_size ?? cfg.chunkingConfig.parentChunkSize,
      childChunkSize: mc.child_chunk_size ?? cfg.chunkingConfig.childChunkSize,
      strategy: mc.strategy ?? cfg.chunkingConfig.strategy,
      tokenLimit: mc.token_limit ?? cfg.chunkingConfig.tokenLimit,
      languages: Array.isArray(mc.languages) ? mc.languages : cfg.chunkingConfig.languages,
      tableMetadataInstructions: mc.table_metadata_instructions ?? cfg.chunkingConfig.tableMetadataInstructions,
    }
  }
  const vlm = raw.vlm_config
  if (vlm) {
    cfg.multimodalConfig = {
      enabled: !!vlm.enabled,
      vllmModelId: vlm.model_id || cfg.multimodalConfig.vllmModelId,
      descriptionLanguage: vlm.description_language || '',
      customInstructions: vlm.custom_instructions || '',
    }
  }
  const ipc = raw.image_processing_config
  if (ipc) {
    cfg.imageAttrsEnabled = !!ipc.image_attrs_enabled
    cfg.imageVectorEnabled = !!ipc.image_vector_enabled
    if (!cfg.multimodalConfig.vllmModelId && ipc.model_id) cfg.multimodalConfig.vllmModelId = ipc.model_id
  }
  const asr = raw.asr_config
  if (asr) {
    cfg.asrConfig = {
      enabled: !!asr.enabled,
      modelId: asr.model_id || '',
      language: asr.language || '',
    }
  }
  const ex = raw.extract_config
  if (ex) {
    cfg.nodeExtractConfig = {
      enabled: !!ex.enabled,
      text: ex.text || '',
      tags: Array.isArray(ex.tags) ? ex.tags : [],
      nodes: Array.isArray(ex.nodes) ? ex.nodes : [],
      relations: Array.isArray(ex.relations) ? ex.relations : [],
      customInstructions: ex.custom_instructions || '',
    }
  }
  const qg = raw.question_generation_config
  if (qg) {
    cfg.questionGenerationConfig = {
      enabled: !!qg.enabled,
      questionCount: qg.question_count ?? 3,
      customInstructions: qg.custom_instructions || '',
    }
  }
  const at = raw.auto_tag_config
  if (at) {
    cfg.autoTagConfig = {
      enabled: !!at.enabled,
      modelId: at.model_id || '',
      maxTags: at.max_tags ?? 3,
      skipIfTagged: at.skip_if_tagged ?? true,
    }
  }
  const pf = raw.profile_config
  if (pf) {
    cfg.profileConfig = {
      enabled: !!pf.enabled,
      modelId: pf.model_id || '',
      customInstructions: pf.custom_instructions || '',
    }
  }
  const faq = raw.faq_config
  if (faq) {
    cfg.faqConfig = {
      indexMode: faq.index_mode || cfg.faqConfig.indexMode,
      questionIndexMode: faq.question_index_mode || cfg.faqConfig.questionIndexMode,
    }
  }
  // Ops-admin-only (C/D/J).
  if (typeof raw.vector_store_id === 'string') cfg.vectorStoreId = raw.vector_store_id
  if (typeof raw.storage_backend_id === 'string') cfg.storageBackendId = raw.storage_backend_id
  if (raw.storage_provider_config && typeof raw.storage_provider_config.provider === 'string') {
    cfg.storageProvider = raw.storage_provider_config.provider
  }
  if (mc && Array.isArray(mc.parser_engine_rules)) cfg.parserEngineRules = mc.parser_engine_rules
}

async function loadModels() {
  try {
    allModels.value = await listModels()
  } catch (e) {
    console.error('kbDefaults: failed to load models', e)
    allModels.value = []
  }
}

async function loadKBs() {
  // 数据源块仅运营管理员可见，非运营管理员无需拉取知识库列表。
  if (!canOpsEdit.value) return
  try {
    const res: any = await listKnowledgeBases()
    const arr = res?.data && Array.isArray(res.data) ? res.data : []
    kbList.value = arr.map((kb: any) => ({ id: kb.id, name: kb.name }))
    if (!dsSelectedKbId.value && kbList.value.length) {
      dsSelectedKbId.value = kbList.value[0].id
    }
  } catch (e) {
    console.error('kbDefaults: failed to load knowledge bases', e)
    kbList.value = []
  }
}

async function load() {
  loading.value = true
  try {
    // getSystemSetting throws when the row was never persisted; treat that as
    // "no platform config yet" and keep the built-in defaults.
    const item = await getSystemSetting(KB_DEFAULTS_KEY)
    const raw = typeof item?.value === 'string' ? item.value : ''
    if (raw.trim()) {
      try {
        fromStorageShape(JSON.parse(raw))
      } catch (e) {
        console.error('kbDefaults: malformed stored JSON, using defaults', e)
      }
    }
  } catch {
    // 404 / unknown key: nothing persisted yet.
  } finally {
    ready.value = true
    loading.value = false
    // 以刚加载/默认值序列化作为基线，避免首次渲染回写一次无意义的保存。
    lastSavedJson = JSON.stringify(toStorageShape())
  }
}

// 实时生效：不再需要“保存”按钮。深度监听 cfg，防抖后自动写回 system_settings；
// 与上次已保存内容相同则跳过（过滤加载/组件初始化引发的无变化回写）。
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
  saving.value = true
  autosaveState.value = 'saving'
  try {
    await updateSystemSetting(KB_DEFAULTS_KEY, json)
    lastSavedJson = json
    autosaveState.value = 'saved'
  } catch (e: any) {
    autosaveState.value = 'idle'
    MessagePlugin.error(e?.message || t('kbDefaults.saveFailed'))
  } finally {
    saving.value = false
  }
}

watch(cfg, () => scheduleAutoSave(), { deep: true })

onMounted(() => {
  void loadModels()
  void loadKBs()
  void load()
})

// 卸载时若还有防抖未落地的修改，立即补存一次，避免离开页面丢失最后一次编辑。
onUnmounted(() => {
  if (autosaveTimer) {
    clearAutosaveTimer()
    void runAutoSave()
  }
})
</script>

<style lang="less" scoped>
.kb-defaults-settings {
  width: 100%;
  padding-bottom: 24px;
}

// Non-super-admins reach a read-only view: block interaction on the form
// content (pointer-events inherits into the reused child components) while
// still allowing the left nav to switch sections.
.kb-defaults-settings--readonly .kb-defaults-content {
  pointer-events: none;
  user-select: none;
  opacity: 0.9;
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

.kb-defaults-readonly-banner {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 14px;
  margin-bottom: 16px;
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-base);
}

.kb-defaults-content {
  flex: 1;
  min-width: 0;
}

.kb-defaults-block {
  padding: 4px 0 24px;
}

// FAQ 设置页：横排 label/说明 + 右侧分段选择，行间用分隔线形成列表节奏。
.kb-faq-settings {
  .section-header {
    margin-bottom: 12px;
  }

  .settings-group {
    border-top: 1px solid var(--td-component-stroke);
  }

  .setting-row {
    align-items: center;
    padding: 18px 0;
    border-bottom: 1px solid var(--td-component-stroke);

    &:last-child {
      padding-bottom: 0;
      border-bottom: none;
    }
  }

  .setting-info {
    flex: 0 1 56%;
    max-width: 56%;
  }

  .setting-control {
    flex: 0 0 300px;
    max-width: 44%;
    align-items: center;
    justify-content: flex-end;

    // 两行分段选择统一宽度、按钮等分，左右边缘对齐更整齐。
    :deep(.t-radio-group) {
      display: flex;
      width: 100%;
    }

    :deep(.t-radio-button) {
      flex: 1;
      justify-content: center;
    }
  }
}

.kb-inline-section {
  .section-header {
    margin-bottom: 16px;
  }
}

.kb-defaults-ds-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;

  &__label {
    font-size: var(--app-text-lg);
    font-weight: 500;
    color: var(--td-text-color-primary);
    flex: 0 0 auto;
  }

  &__select {
    flex: 0 0 320px;
    max-width: 320px;
  }
}

.kb-defaults-ds-hint,
.kb-defaults-ds-empty {
  font-size: var(--app-text-base);
  color: var(--td-text-color-secondary);
  margin: 0 0 12px 0;
  line-height: 1.5;
}

.settings-group {
  width: 100%;
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 14px 0;

  &:last-child {
    padding-bottom: 0;
  }

  &.setting-row-vertical {
    flex-direction: column;
    align-items: stretch;
  }
}

.setting-info {
  flex: 0 0 45%;
  max-width: 45%;
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
}

.setting-control {
  flex: 0 0 45%;
  max-width: 45%;
  display: flex;
  justify-content: flex-end;
  align-items: flex-start;

  &.setting-control-full {
    flex: 1 1 auto;
    max-width: 100%;
    margin-top: 8px;
  }
}

.required {
  color: var(--td-error-color);
  margin-left: 2px;
}

.kb-defaults-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 16px;
  margin-top: 20px;

  &__hint {
    font-size: var(--app-text-sm);
    color: var(--td-text-color-placeholder);
  }

  &__status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: var(--app-text-sm);
    white-space: nowrap;

    &.saving {
      color: var(--td-text-color-secondary);
    }

    &.saved {
      color: var(--td-brand-color);
    }
  }
}
</style>

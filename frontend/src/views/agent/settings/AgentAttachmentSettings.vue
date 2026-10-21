<template>
  <div class="section-header">
    <h2>{{ $t('agentEditor.imageUpload.sectionTitle') }}</h2>
    <p class="section-description">{{ $t('agentEditor.imageUpload.sectionDesc') }}</p>
  </div>

  <div class="settings-group">
    <!-- 图片上传 -->
    <div class="setting-row" data-guide="agent-create-multimodal">
      <div class="setting-info">
        <label>{{ $t('agentEditor.imageUpload.label') }}</label>
        <p class="desc">{{ $t('agentEditor.imageUpload.desc') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.image_upload_enabled" />
      </div>
    </div>

    <!-- VLM 模型（图片上传启用时） -->
    <div v-if="config.image_upload_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.imageUpload.vlmModel') }} <span class="required">*</span></label>
        <p class="desc">{{ $t('agentEditor.imageUpload.vlmModelDesc') }}</p>
      </div>
      <div class="setting-control">
        <ModelSelector model-type="VLLM" :selected-model-id="config.vlm_model_id" :all-models="allModels"
          @update:selected-model-id="(val: string) => config.vlm_model_id = val"
          @add-model="emit('addModel', 'vllm')"
          :placeholder="$t('agentEditor.imageUpload.vlmModelPlaceholder')" />
      </div>
    </div>

    <!-- 附件图片理解 / 扫描件 OCR（图片上传启用时） -->
    <div v-if="config.image_upload_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.imageUpload.imageUnderstandingLabel') }}</label>
        <p class="desc">{{ $t('agentEditor.imageUpload.imageUnderstandingDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.attachment_image_understanding" />
      </div>
    </div>

    <!-- 扫描件 OCR 最大页数（开启附件图片理解时） -->
    <div v-if="config.image_upload_enabled && config.attachment_image_understanding" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.imageUpload.ocrMaxPagesLabel') }}</label>
        <p class="desc">{{ $t('agentEditor.imageUpload.ocrMaxPagesDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-input-number v-model="config.attachment_ocr_max_pages" :min="0" :max="64" :step="1"
          theme="normal" style="width: 160px;"
          :placeholder="$t('agentEditor.imageUpload.useGlobalDefault')" />
      </div>
    </div>

    <!-- 图片存储 Provider（图片上传启用时） -->
    <div v-if="config.image_upload_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.imageUpload.storageProvider') }}</label>
        <p class="desc">{{ $t('agentEditor.imageUpload.storageProviderDesc') }}</p>
      </div>
      <div class="setting-control" style="flex-direction: column; align-items: flex-end;">
        <t-select v-model="config.image_storage_provider" style="width: 280px;"
          :placeholder="$t('agentEditor.imageUpload.storageProviderPlaceholder')" clearable>
          <t-option value="" :label="$t('agentEditor.imageUpload.storageDefault')" />
          <t-option v-for="opt in imageStorageOptions" :key="opt.value" :value="opt.value"
            :label="opt.label" :disabled="opt.disabled">
            <span class="select-option-with-tag">
              <span>{{ opt.label }}</span>
              <t-tag v-if="opt.disabled" theme="warning" variant="light" size="small">{{
                $t('agentEditor.imageUpload.notConfigured') }}</t-tag>
            </span>
          </t-option>
        </t-select>
        <a href="javascript:void(0)" class="go-settings-link"
          @click.prevent="uiStore.openSettings('storage')">
          {{ $t('agentEditor.imageUpload.goStorageSettings') }}
        </a>
      </div>
    </div>

    <!-- 音频上传开关 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.audioUpload.label') }}</label>
        <p class="desc">{{ $t('agentEditor.audioUpload.desc') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.audio_upload_enabled" />
      </div>
    </div>

    <!-- ASR 模型（音频上传启用时） -->
    <div v-if="config.audio_upload_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.audioUpload.asrModel') }}</label>
        <p class="desc">{{ $t('agentEditor.audioUpload.asrModelDesc') }}</p>
      </div>
      <div class="setting-control">
        <ModelSelector model-type="ASR" :selected-model-id="config.asr_model_id" :all-models="allModels"
          clearable @update:selected-model-id="(val: string) => config.asr_model_id = val"
          @add-model="emit('addModel', 'asr')"
          :placeholder="$t('agentEditor.audioUpload.asrModelPlaceholder')" />
      </div>
    </div>

    <!-- 单轮等待附件解析超时（秒） -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.chatParser.waitTimeoutLabel') }}</label>
        <p class="desc">{{ $t('agentEditor.chatParser.waitTimeoutDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-input-number v-model="config.attachment_parse_wait_timeout_sec" :min="0" :max="600" :step="10"
          theme="normal" style="width: 160px;"
          :placeholder="$t('agentEditor.imageUpload.useGlobalDefault')" />
      </div>
    </div>

    <!-- 聊天附件解析策略 -->
    <div class="parser-policy-block">
      <div class="parser-policy-block__header">
        <label>{{ $t('agentEditor.chatParser.label') }}</label>
        <p class="desc">{{ $t('agentEditor.chatParser.desc') }}</p>
      </div>
      <KBParserSettings embedded :parser-engine-rules="config.chat_parser_engine_rules"
        :relevant-extensions="CHAT_PARSER_EXTENSIONS"
        @update:parser-engine-rules="(val: any) => config.chat_parser_engine_rules = val" />
    </div>
  </div>
</template>

<script setup lang="ts">
import ModelSelector from '@/components/ModelSelector.vue'
import KBParserSettings from '@/views/knowledge/settings/KBParserSettings.vue'
import { useUIStore } from '@/stores/ui'
import type { CustomAgentConfig } from '@/api/agent'
import type { ModelConfig } from '@/api/model'

// 附件上传分区（需求 G）。编辑器与平台「智能体配置」共用本组件。
//
// imageStorageOptions 依赖编辑器拉取的存储引擎可用状态（storageEngineStatus），
// 所以由父级算好传进来；CHAT_PARSER_EXTENSIONS 是纯常量，随模板搬进来。
const props = defineProps<{
  config: CustomAgentConfig
  allModels: ModelConfig[]
  imageStorageOptions: Array<{ value: string; label: string; disabled?: boolean }>
}>()

const emit = defineEmits<{ (e: 'addModel', section: string): void }>()

const uiStore = useUIStore()

// File extensions offered in the agent-level chat attachment parsing policy.
const CHAT_PARSER_EXTENSIONS = [
  'pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'epub', 'mhtml',
  'txt', 'md', 'markdown', 'csv', 'json', 'xml', 'html', 'yaml', 'yml', 'log',
  'jpg', 'jpeg', 'png', 'gif', 'bmp', 'tiff', 'webp',
]
</script>

<style scoped lang="less">
@import './agentSectionStyles.less';

.select-option-with-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
</style>
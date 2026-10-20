<template>
  <t-button variant="text" size="small" :title="t('workspace.caseSaveToKnowledge')" @click.stop="open">
    <template #icon><t-icon :name="saved ? 'check' : 'folder-add'" size="16px" /></template>
    {{ t(saved ? 'workspace.caseOpenKnowledge' : 'workspace.caseSaveToKnowledge') }}
  </t-button>
  <t-dialog
    :visible="visible"
    :header="t('workspace.caseSaveToKnowledge')"
    :confirm-btn="{ content: t('common.save'), loading: saving, disabled: loading || loadingExisting || !kbId }"
    :cancel-btn="{ content: t('common.cancel'), disabled: saving }"
    :close-on-overlay-click="false"
    :close-btn="!saving"
    :close-on-esc-keydown="!saving"
    @confirm="save"
    @close="close"
  >
    <p>{{ fileName }}</p>
    <t-select v-model="kbId" :options="options" :loading="loading" :disabled="saving"
      :placeholder="t('workspace.caseSelectKnowledge')" :aria-label="t('workspace.caseSelectKnowledge')" />
    <p v-if="!loading && !options.length">{{ t('workspace.caseNoKnowledge') }}</p>
    <template v-if="kbId">
      <p class="case-existing-label">{{ t('workspace.caseLinkExisting') }}</p>
      <t-select v-model="existingKnowledgeId" :options="existingOptions" :loading="loadingExisting" :disabled="saving"
        :placeholder="t('workspace.caseLinkExistingHint')" :aria-label="t('workspace.caseLinkExisting')" clearable filterable />
    </template>
  </t-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { downloadArtifact } from '@/api/chat'
import { linkKnowledgeArtifact, listKnowledgeFiles, uploadKnowledgeFile } from '@/api/knowledge-base'
import { useChatResourcesStore } from '@/stores/chatResources'
import { useOrganizationStore } from '@/stores/organization'
import { useAuthStore } from '@/stores/auth'
import { permissionCanEditKB } from '@/utils/kbPermission'

const props = defineProps<{ sessionId: string; messageId: string; artifactIndex: number; fileName: string; knowledgeId?: string; knowledgeBaseId?: string }>()
const emit = defineEmits<{ (e: 'saved', binding: { knowledge_id: string; knowledge_base_id: string }): void }>()
const { t } = useI18n()
const router = useRouter()
const resources = useChatResourcesStore()
const organization = useOrganizationStore()
const auth = useAuthStore()
const visible = ref(false)
const loading = ref(false)
const saving = ref(false)
const kbId = ref('')
const saved = ref(false)
const options = ref<{ label: string; value: string }[]>([])
const existingKnowledgeId = ref('')
const existingOptions = ref<{ label: string; value: string }[]>([])
const loadingExisting = ref(false)
let existingGeneration = 0
let generation = 0

watch(() => [props.sessionId, props.messageId, props.artifactIndex, props.fileName], () => {
  generation++
  visible.value = false
  saved.value = false
  kbId.value = ''
})

watch(() => [props.knowledgeId, props.knowledgeBaseId], () => {
  if (props.knowledgeId && props.knowledgeBaseId) {
    saved.value = true
    kbId.value = props.knowledgeBaseId
  }
}, { immediate: true })

watch(() => [visible.value, kbId.value], async () => {
  const request = ++existingGeneration
  existingKnowledgeId.value = ''
  existingOptions.value = []
  if (!visible.value || !kbId.value) return
  loadingExisting.value = true
  try {
    const response = await listKnowledgeFiles(kbId.value, { page: 1, page_size: 100, keyword: props.fileName.replace(/\.[^.]+$/, '') })
    if (request !== existingGeneration) return
    existingOptions.value = (response.data || []).filter((item: any) => item.type === 'file').map((item: any) => ({
      label: `${item.file_name || item.title}${item.updated_at ? ` · ${new Date(item.updated_at).toLocaleString()}` : ''}`, value: item.id,
    }))
  } catch {
    if (request === existingGeneration) MessagePlugin.error(t('workspace.caseLinkListFailed'))
  } finally {
    if (request === existingGeneration) loadingExisting.value = false
  }
})

function close() {
  if (!saving.value) visible.value = false
}

async function open() {
  if (saved.value) {
    void router.push(`/platform/knowledge-bases/${kbId.value}`)
    return
  }
  visible.value = true
  loading.value = true
  const request = ++generation
  try {
    await resources.ensureKnowledgeBases()
    if (request !== generation) return
    const own = resources.rawKnowledgeBases.filter((kb: any) => {
      const permission = kb.my_permission || kb.permission
      return permission ? permissionCanEditKB(permission)
        : kb.creator_id === auth.user?.id || auth.hasRole('admin') || organization.canEditKB(kb.id, false)
    })
    const shared = organization.sharedKnowledgeBases
      .filter((share) => permissionCanEditKB(share.permission))
      .map((share) => share.knowledge_base)
    const seen = new Set<string>()
    options.value = [...own, ...shared]
      .filter((kb: any) => {
        if (!kb || (kb.type && kb.type !== 'document') || seen.has(kb.id)) return false
        seen.add(kb.id)
        return true
      })
      .map((kb: any) => ({ label: kb.name, value: kb.id }))
    if (!options.value.some((option) => option.value === kbId.value)) kbId.value = options.value[0]?.value || ''
  } catch {
    options.value = []
    MessagePlugin.error(t('workspace.caseSaveFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (saving.value || loadingExisting.value || !kbId.value) return
  saving.value = true
  const request = generation
  const { sessionId, messageId, artifactIndex, fileName } = props
  const target = kbId.value
  const selected = existingKnowledgeId.value
  try {
    let knowledgeId = selected
    if (!knowledgeId) {
      const blob = await downloadArtifact(sessionId, messageId, artifactIndex)
      try {
        const response = await uploadKnowledgeFile(target, { file: new File([blob], fileName, { type: blob.type }) })
        knowledgeId = response.data?.id
      } catch (error: any) {
        if (error?.code === 'duplicate_file' && error?.data?.id) knowledgeId = error.data.id
        else throw error
      }
    }
    if (!knowledgeId) throw new Error(t('workspace.caseSaveFailed'))
    await linkKnowledgeArtifact(knowledgeId, { session_id: sessionId, message_id: messageId, index: artifactIndex })
    if (request !== generation) return
    saved.value = true
    visible.value = false
    emit('saved', { knowledge_id: knowledgeId, knowledge_base_id: target })
    MessagePlugin.success(t('workspace.caseSaved'))
  } catch (error: any) {
    if (request !== generation) return
    MessagePlugin.error(error?.message || t('workspace.caseSaveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.case-existing-label { margin: 20px 0 8px; color: var(--td-text-color-secondary); font-size: var(--app-text-sm); }
</style>

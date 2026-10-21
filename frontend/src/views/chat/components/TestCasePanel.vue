<template>
  <section class="test-case-panel">
    <div v-if="collecting && !items.length" class="test-case-empty"><t-loading size="small" /><span>{{ $t('agent.artifactDrawer.collecting') }}</span></div>
    <div v-else-if="!items.length" class="test-case-empty"><t-icon name="clipboard" size="30px" /><span>{{ emptyText || $t('chat.sandbox.artifactsEmpty') }}</span></div>
    <template v-else-if="activeArtifact">
      <header class="case-workbench-header">
        <div class="case-workbench-title"><h3>{{ $t('workspace.caseEditorTitle') }}</h3><span v-if="!activeLoading">{{ $t('workspace.caseSheetRows', { count: workbench.cases.length }) }}</span></div>
        <ArtifactSaveToKnowledge :key="artifactKey(activeArtifact)" :session-id="sessionId" :message-id="activeArtifact.messageId"
          :artifact-index="activeArtifact.index" :file-name="activeArtifact.file_name"
          :knowledge-id="linkOf(activeArtifact)?.knowledge_id" :knowledge-base-id="linkOf(activeArtifact)?.knowledge_base_id"
          @saved="onKnowledgeLinked(activeArtifact, $event)" />
      </header>
      <div class="case-source-picker">
        <span class="case-source-label">{{ $t('workspace.caseCurrentSet') }}</span>
        <t-select :value="artifactKey(activeArtifact)" :options="sourceOptions" :aria-label="$t('workspace.caseCurrentSet')" @change="selectArtifact" />
        <t-button variant="text" shape="square" size="small" :loading="!!downloading[artifactKey(activeArtifact)]"
          :title="$t('agent.artifactDrawer.download')" :aria-label="$t('agent.artifactDrawer.download')" @click="handleDownload(activeArtifact)">
          <template #icon><t-icon name="download" /></template>
        </t-button>
        <t-button variant="text" shape="square" size="small" class="case-delete-action" :loading="!!deleting[artifactKey(activeArtifact)]"
          :title="$t('agent.artifactDrawer.delete')" :aria-label="$t('agent.artifactDrawer.delete')" @click="handleDelete(activeArtifact)">
          <template #icon><t-icon name="delete" /></template>
        </t-button>
      </div>
      <t-tabs class="case-workbench-tabs" :value="view" @change="selectView">
        <t-tab-panel value="notes" :label="$t('workspace.caseNotesTab')" />
        <t-tab-panel value="cases"><template #label>{{ $t('workspace.caseListTab') }}<span class="case-tab-count">{{ workbench.cases.length }}</span></template></t-tab-panel>
        <t-tab-panel value="results"><template #label>{{ $t('workspace.caseResultsTab') }}<span v-if="workbench.results.length" class="case-tab-count">{{ workbench.results.length }}</span></template></t-tab-panel>
        <t-tab-panel value="files"><template #label>{{ $t('workspace.caseSourcesTab') }}<span class="case-tab-count">{{ items.length }}</span></template></t-tab-panel>
      </t-tabs>
      <div v-if="view !== 'files' && activeEntry?.status === 'sync-error'" class="case-workbench-message" role="alert">
        <t-icon name="error-circle" /><span>{{ $t('workspace.caseSyncFailed') }}</span><t-button variant="text" size="small" @click="parseItems">{{ $t('common.retry') }}</t-button>
      </div>
      <div v-else-if="view !== 'files' && activeLoading" class="test-case-empty"><t-loading size="small" /><span>{{ $t('workspace.caseSheetParsing') }}</span></div>
      <div v-else-if="view === 'cases' || view === 'results'" class="case-workbench-scroll">
        <div class="case-workbench-toolbar">
          <t-input v-model="searchTerm" class="case-search" clearable :placeholder="$t('workspace.caseSearchPlaceholder')" :aria-label="$t('workspace.caseSearchPlaceholder')"><template #prefix-icon><t-icon name="search" /></template></t-input>
          <span>{{ $t('workspace.caseRecordCount', { count: filteredCases.length }) }}</span>
        </div>
        <t-table class="case-compact-table" row-key="key" :data="filteredCases" :columns="compactColumns" hover size="small"
          table-layout="fixed" :row-class-name="caseRowClass" :empty="$t(view === 'results' ? 'workspace.caseNoResults' : 'workspace.caseNoStructuredCases')"
          :pagination="filteredCases.length > pageSize ? { current: casePage, pageSize, total: filteredCases.length, showPageSize: false, size: 'small' } : undefined"
          @page-change="changePage" @row-click="selectCase">
          <template #id="{ row }"><span class="case-record-id" :title="row.id">{{ row.id }}</span></template>
          <template #title="{ row }"><button type="button" class="case-title-button" :aria-pressed="row.key === selectedCase?.key" @click.stop="selectedCaseKey = row.key">{{ row.title || $t('workspace.caseUntitled') }}</button></template>
          <template #priority="{ row }"><span v-if="row.priority" class="case-priority" :class="{ 'is-high': /^(p0|紧急|critical)$/i.test(row.priority) }">{{ row.priority }}</span><span v-else class="case-value-empty">—</span></template>
          <template #status="{ row }"><span class="case-result-status" :class="`is-${row.status}`" :title="statusLabel(row)">{{ statusLabel(row) }}</span></template>
        </t-table>
        <section v-if="selectedCase" class="case-record-detail" aria-live="polite" :aria-label="$t('workspace.caseDetailsTitle')">
          <div class="case-detail-heading"><span class="case-record-id">{{ selectedCase.id }}</span><h4>{{ selectedCase.title || $t('workspace.caseUntitled') }}</h4><span class="case-result-status" :class="`is-${selectedCase.status}`">{{ statusLabel(selectedCase) }}</span></div>
          <p v-if="!selectedCase.details.length" class="case-detail-empty">{{ $t('workspace.caseNoDetails') }}</p>
          <dl v-else class="case-detail-fields"><div v-for="(field, index) in selectedCase.details" :key="index"><dt>{{ field.label }}</dt><dd>{{ field.value }}</dd></div></dl>
          <template v-if="selectedCase.resultDetails.length"><h5>{{ $t('workspace.caseResultsTab') }}</h5><dl class="case-detail-fields"><div v-for="(field, index) in selectedCase.resultDetails" :key="index"><dt>{{ field.label }}</dt><dd>{{ field.value }}</dd></div></dl></template>
          <div v-if="linkOf(activeArtifact)?.knowledge_id" class="case-detail-footer"><span><t-icon name="check" />{{ $t('workspace.caseLinkedKnowledge') }}</span><t-button variant="text" size="small" @click="openKnowledge">{{ $t('workspace.caseEditInKnowledge') }}<template #suffix><t-icon name="jump" /></template></t-button></div>
        </section>
      </div>
      <div v-else-if="view === 'notes'" class="case-workbench-scroll case-notes-view">
        <div v-if="!workbench.notes.length" class="case-section-empty">{{ $t('workspace.caseNoNotes') }}</div>
        <section v-for="sheet in workbench.notes" :key="sheet.name" class="case-note-section"><h4>{{ sheet.name }}</h4>
          <dl v-if="sheet.header.length <= 2" class="case-detail-fields"><div v-for="(row, index) in sheet.rows" :key="index"><dt>{{ row[0] || '—' }}</dt><dd>{{ row.slice(1).join('\n') || '—' }}</dd></div></dl>
          <div v-else class="case-note-table-wrap"><table><thead><tr><th v-for="(title, index) in sheet.header" :key="index">{{ title || `#${index + 1}` }}</th></tr></thead><tbody><tr v-for="(row, index) in sheet.rows" :key="index"><td v-for="(_, col) in sheet.header" :key="col">{{ row[col] }}</td></tr></tbody></table></div>
        </section>
      </div>
      <div v-else class="case-source-view">
        <template v-if="sourcePreview">
          <div class="case-preview-heading"><t-button variant="text" shape="square" size="small" :aria-label="$t('workspace.caseBackToSources')" @click="sourcePreviewKey = ''"><template #icon><t-icon name="chevron-left" /></template></t-button><ArtifactFileIcon :file-name="sourcePreview.file_name" /><span>{{ sourcePreview.file_name }}</span></div>
          <DocumentPreview :key="`${artifactKey(sourcePreview)}:${previewRevision}`" :knowledge-id="linkOf(sourcePreview)?.knowledge_id" :session-id="sessionId" :message-id="sourcePreview.messageId"
            :artifact-index="sourcePreview.index" :file-name="sourcePreview.file_name" :file-type="resolveFilePreviewExt(sourcePreview.file_name, sourcePreview.file_type)" :active="active && view === 'files'" fill-height />
        </template>
        <ul v-else class="case-source-list"><li v-for="item in items" :key="artifactKey(item)"><button type="button" class="case-source-open" @click="openSource(item)"><ArtifactFileIcon :file-name="item.file_name" /><span><strong>{{ item.file_name }}</strong><small>{{ linkOf(item)?.knowledge_id ? $t('workspace.caseLinkedKnowledge') : $t('workspace.caseSourceSnapshot') }}</small></span></button><t-button variant="text" size="small" @click="openSource(item)">{{ $t('agent.artifactDrawer.preview') }}</t-button></li></ul>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { MessagePlugin, type PrimaryTableCol, type TableRowData } from 'tdesign-vue-next'
import { deleteMessageArtifact, downloadArtifact, listMessageArtifacts, type ArtifactMeta } from '@/api/chat'
import { downKnowledgeDetails } from '@/api/knowledge-base'
import { KNOWLEDGE_FILE_UPDATED_EVENT } from '@/utils/knowledgeFileUpdates'
import { parseTestCaseSheets, isSpreadsheetArtifact, type TestCaseSheet } from '@/utils/testCaseSheet'
import { buildTestCaseWorkbench, type WorkbenchCase } from '@/utils/testCaseWorkbench'
import { resolveFilePreviewExt } from '@/utils/filePreview'
import type { SessionArtifactItem } from '@/utils/sessionArtifacts'
import { useConfirmDelete } from '@/components/settings/useConfirmDelete'
import ArtifactFileIcon from './ArtifactFileIcon.vue'
import DocumentPreview from '@/components/document-preview.vue'
import ArtifactSaveToKnowledge from '@/components/ArtifactSaveToKnowledge.vue'

type CaseEntry =
  | { status: 'loading' }
  | { status: 'ready'; sheets: TestCaseSheet[] }
  | { status: 'failed' }
  | { status: 'sync-error' }

type KnowledgeLink = Pick<ArtifactMeta, 'knowledge_id' | 'knowledge_base_id'>

const props = withDefaults(
  defineProps<{
    sessionId: string
    items: SessionArtifactItem[]
    collecting?: boolean
    active?: boolean
    emptyText?: string
  }>(),
  { collecting: false, active: true },
)

const emit = defineEmits<{ (e: 'deleted', payload: { messageId: string; index: number }): void }>()

const { t } = useI18n()
const router = useRouter()
const confirmDelete = useConfirmDelete()
// 解析结果按产物行缓存：失败的行退回文件列表，成功的行只用表格展示。
const entries = reactive(new Map<string, CaseEntry>())
const downloading = reactive<Record<string, boolean>>({})
const deleting = reactive<Record<string, boolean>>({})
const knowledgeLinks = reactive(new Map<string, KnowledgeLink>())
const selectedArtifactKey = ref('')
const selectedCaseKey = ref('')
const sourcePreviewKey = ref('')
const previewRevision = ref(0)
const view = ref<'cases' | 'notes' | 'results' | 'files'>('notes')
const searchTerm = ref('')
const casePage = ref(1)
const pageSize = 8

function artifactKey(item: SessionArtifactItem): string {
  return `${item.messageId}:${item.index}:${item.file_name}`
}

const spreadsheetItems = computed(() => props.items.filter((item) => isSpreadsheetArtifact(item.file_name)))
const tableItems = computed(() =>
  spreadsheetItems.value.filter((item) => entries.get(artifactKey(item))?.status !== 'failed'),
)
const fallbackItems = computed(() =>
  props.items.filter(
    (item) =>
      !isSpreadsheetArtifact(item.file_name) || entries.get(artifactKey(item))?.status === 'failed',
  ).map(item => ({ ...item, ...linkOf(item) })),
)

function linkOf(item: SessionArtifactItem): KnowledgeLink | undefined {
  return knowledgeLinks.get(artifactKey(item)) || (item.knowledge_id ? {
    knowledge_id: item.knowledge_id, knowledge_base_id: item.knowledge_base_id,
  } : undefined)
}

function sheetsOf(item: SessionArtifactItem): TestCaseSheet[] | null {
  const entry = entries.get(artifactKey(item))
  return entry?.status === 'ready' ? entry.sheets : null
}

function rowCount(item: SessionArtifactItem): number {
  return buildTestCaseWorkbench(sheetsOf(item) || []).cases.length
}

const activeArtifact = computed(() => props.items.find(item => artifactKey(item) === selectedArtifactKey.value)
  || tableItems.value.at(-1) || props.items.at(-1) || null)
const sourceOptions = computed(() => props.items.map(item => ({
  label: item.file_name, value: artifactKey(item),
})))
const workbench = computed(() => buildTestCaseWorkbench(activeArtifact.value ? sheetsOf(activeArtifact.value) || [] : []))
const activeEntry = computed(() => activeArtifact.value ? entries.get(artifactKey(activeArtifact.value)) : undefined)
const activeLoading = computed(() => !!activeArtifact.value && isSpreadsheetArtifact(activeArtifact.value.file_name)
  && (!activeEntry.value || activeEntry.value.status === 'loading'))
const viewRows = computed(() => view.value === 'results' ? workbench.value.results : workbench.value.cases)
const filteredCases = computed(() => {
  const query = searchTerm.value.trim().toLowerCase()
  return query ? viewRows.value.filter(item => `${item.id} ${item.title} ${item.searchText}`.toLowerCase().includes(query)) : viewRows.value
})
const currentRows = computed(() => filteredCases.value.slice((casePage.value - 1) * pageSize, casePage.value * pageSize))
const selectedCase = computed(() => currentRows.value.find(item => item.key === selectedCaseKey.value) || currentRows.value[0] || null)
const sourcePreview = computed(() => props.items.find(item => artifactKey(item) === sourcePreviewKey.value) || null)
const compactColumns = computed<PrimaryTableCol[]>(() => [
  { colKey: 'id', title: t('workspace.caseColumnId'), width: 88, className: 'case-id-cell' },
  { colKey: 'title', title: t('workspace.caseColumnTitle'), minWidth: 120 },
  { colKey: 'priority', title: t('workspace.caseColumnPriority'), width: 62 },
  { colKey: 'status', title: t('workspace.caseColumnStatus'), width: 78 },
])

function selectArtifact(value: string | number) {
  selectedArtifactKey.value = String(value)
  selectedCaseKey.value = ''
  searchTerm.value = ''
  casePage.value = 1
  const file = activeArtifact.value
  if (file && fallbackItems.value.some(item => artifactKey(item) === artifactKey(file))) {
    view.value = 'files'
    sourcePreviewKey.value = artifactKey(file)
  } else {
    view.value = 'notes'
    sourcePreviewKey.value = ''
  }
}

function selectView(value: string | number) {
  if (!['cases', 'notes', 'results', 'files'].includes(String(value))) return
  view.value = value as typeof view.value
  selectedCaseKey.value = ''
  sourcePreviewKey.value = ''
  searchTerm.value = ''
  casePage.value = 1
}

function selectCase({ row }: { row: TableRowData }) { selectedCaseKey.value = String(row.key) }
function caseRowClass({ row }: { row: TableRowData }) { return row.key === selectedCase.value?.key ? 'case-selected-row' : '' }
function changePage({ current }: { current: number }) { casePage.value = current; selectedCaseKey.value = '' }
function statusLabel(item: WorkbenchCase): string {
  return item.status === 'unknown' && item.statusText ? item.statusText : t(`workspace.caseState_${item.status.replace('-', '_')}`)
}
function openSource(item: SessionArtifactItem) {
  selectedArtifactKey.value = artifactKey(item)
  sourcePreviewKey.value = artifactKey(item)
  view.value = 'files'
}
function openKnowledge() {
  const binding = activeArtifact.value && linkOf(activeArtifact.value)
  if (binding?.knowledge_id && binding.knowledge_base_id) {
    void router.push({ path: `/platform/knowledge-bases/${binding.knowledge_base_id}`, query: { knowledge_id: binding.knowledge_id } })
  }
}

watch(searchTerm, () => { casePage.value = 1; selectedCaseKey.value = '' })
watch(() => activeArtifact.value ? artifactKey(activeArtifact.value) : '', () => {
  casePage.value = 1
  selectedCaseKey.value = ''
  if (activeArtifact.value && !isSpreadsheetArtifact(activeArtifact.value.file_name)) openSource(activeArtifact.value)
}, { immediate: true })
watch(() => filteredCases.value.length, () => {
  casePage.value = Math.min(casePage.value, Math.max(1, Math.ceil(filteredCases.value.length / pageSize)))
})

let parsedSessionId = ''
let parseGeneration = 0
// Linked knowledge is mutable: resolve bindings and refresh its bytes on activation.
async function parseItems() {
  if (!props.active) return
  const sessionId = props.sessionId
  const request = ++parseGeneration
  const items = [...props.items]
  const metadata = new Map<string, Map<number, ArtifactMeta>>()
  await Promise.allSettled([...new Set(items.map(item => item.messageId).filter(Boolean))].map(async messageId => {
    const response = await listMessageArtifacts(sessionId, messageId)
    if (!Array.isArray(response.data)) throw new Error('invalid artifact metadata')
    metadata.set(messageId, new Map(response.data.map((item: ArtifactMeta) => [item.index, item])))
  }))
  if (request !== parseGeneration) return
  for (const item of items) {
    if (!item.messageId || request !== parseGeneration) continue
    const key = artifactKey(item)
    const messageMetadata = metadata.get(item.messageId)
    const current = messageMetadata?.get(item.index)
    if (messageMetadata && !current) {
      entries.set(key, { status: 'sync-error' })
      continue
    }
    if (current) knowledgeLinks.set(key, { knowledge_id: current.knowledge_id, knowledge_base_id: current.knowledge_base_id })
    const binding = linkOf(item)
    if (!isSpreadsheetArtifact(item.file_name)) {
      if (binding?.knowledge_id && sourcePreviewKey.value === key) previewRevision.value++
      continue
    }
    if (!messageMetadata && !binding?.knowledge_id) {
      entries.set(key, { status: 'sync-error' })
      continue
    }
    if (!binding?.knowledge_id && entries.get(key)?.status === 'ready') continue
    if (entries.get(key)?.status !== 'ready') entries.set(key, { status: 'loading' })
    try {
      const blob = binding?.knowledge_id
        ? await downKnowledgeDetails(binding.knowledge_id)
        : await downloadArtifact(sessionId, item.messageId, item.index)
      const sheets = await parseTestCaseSheets(blob, item.file_name)
      if (request !== parseGeneration) return
      entries.set(key, { status: 'ready', sheets })
      if (binding?.knowledge_id && sourcePreviewKey.value === key) previewRevision.value++
    } catch (err) {
      if (request !== parseGeneration) return
      console.warn('[TestCasePanel] refresh failed:', err)
      entries.set(key, { status: binding?.knowledge_id ? 'sync-error' : 'failed' })
    }
  }
}

watch(
  () => [props.active, props.sessionId, props.items] as const,
  () => {
    // 换会话时解析结果作废；同会话内重新收集产物沿用已有结果。
    if (parsedSessionId !== props.sessionId) {
      parsedSessionId = props.sessionId
      parseGeneration++
      entries.clear()
      knowledgeLinks.clear()
      selectedArtifactKey.value = ''
      selectedCaseKey.value = ''
      sourcePreviewKey.value = ''
      previewRevision.value = 0
      view.value = 'notes'
      searchTerm.value = ''
      casePage.value = 1
      if (activeArtifact.value && !isSpreadsheetArtifact(activeArtifact.value.file_name)) openSource(activeArtifact.value)
    }
    if (props.active) void parseItems()
    else parseGeneration++
  },
  { immediate: true },
)

function onKnowledgeLinked(item: SessionArtifactItem, binding: KnowledgeLink) {
  knowledgeLinks.set(artifactKey(item), binding)
  void parseItems()
}

function refreshVisibleCases() {
  if (props.active && !document.hidden) void parseItems()
}

onMounted(() => {
  window.addEventListener('focus', refreshVisibleCases)
  window.addEventListener(KNOWLEDGE_FILE_UPDATED_EVENT, refreshVisibleCases)
  document.addEventListener('visibilitychange', refreshVisibleCases)
})
onBeforeUnmount(() => {
  parseGeneration++
  window.removeEventListener('focus', refreshVisibleCases)
  window.removeEventListener(KNOWLEDGE_FILE_UPDATED_EVENT, refreshVisibleCases)
  document.removeEventListener('visibilitychange', refreshVisibleCases)
})

async function handleDownload(item: SessionArtifactItem) {
  const key = artifactKey(item)
  downloading[key] = true
  try {
    const binding = linkOf(item)
    const blob = binding?.knowledge_id
      ? await downKnowledgeDetails(binding.knowledge_id)
      : await downloadArtifact(props.sessionId, item.messageId, item.index)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = item.file_name || 'artifact'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (err) {
    console.error('[TestCasePanel] download failed:', err)
    MessagePlugin.error(t('agent.artifactDrawer.downloadFailed'))
  } finally {
    downloading[key] = false
  }
}

function handleDelete(item: SessionArtifactItem) {
  if (deleting[artifactKey(item)]) return
  confirmDelete({
    title: t('agent.artifactDrawer.deleteTitle'),
    body: t('agent.artifactDrawer.deleteConfirm', { name: item.file_name }),
    onConfirm: () => performDelete(item),
  })
}

async function performDelete(item: SessionArtifactItem) {
  const key = artifactKey(item)
  deleting[key] = true
  try {
    await deleteMessageArtifact(props.sessionId, item.messageId, item.index)
    entries.delete(key)
    knowledgeLinks.delete(key)
    if (sourcePreviewKey.value === key) sourcePreviewKey.value = ''
    emit('deleted', { messageId: item.messageId, index: item.index })
    MessagePlugin.success(t('agent.artifactDrawer.deleted'))
  } catch (err) {
    console.error('[TestCasePanel] delete failed:', err)
    MessagePlugin.error(t('agent.artifactDrawer.deleteFailed'))
  } finally {
    deleting[key] = false
  }
}
</script>

<style scoped lang="less">
.test-case-panel { flex: 1; min-height: 0; min-width: 0; width: 100%; display: flex; flex-direction: column; overflow: hidden; background: var(--td-bg-color-container); }
.test-case-empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; padding: 32px 20px; color: var(--td-text-color-placeholder); font-size: var(--app-text-sm); line-height: 1.7; text-align: center; }
.case-workbench-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 20px 18px 12px; flex-shrink: 0; }
.case-workbench-title { display: flex; align-items: baseline; gap: 10px; min-width: 0; h3 { margin: 0; font-size: var(--app-text-xl); font-weight: 600; white-space: nowrap; } > span { color: var(--td-text-color-placeholder); font-size: var(--app-text-sm); white-space: nowrap; font-variant-numeric: tabular-nums; } }
.case-source-picker { display: flex; align-items: center; gap: 8px; padding: 0 18px 12px; flex-shrink: 0; .t-select__wrap { flex: 1; min-width: 0; } > .t-button { flex-shrink: 0; width: 28px; height: 28px; color: var(--td-text-color-secondary); } .case-delete-action:hover { color: var(--td-error-color); } }
.case-source-label { font-size: var(--app-text-sm); color: var(--td-text-color-placeholder); white-space: nowrap; }
.case-workbench-tabs { margin: 0 18px; flex-shrink: 0;
  :deep(.t-tabs__nav-item) { padding: 0; margin: 0; }
  :deep(.t-tabs__nav-item-wrapper) { margin: 0; padding: 0 10px; height: 40px; }
  :deep(.t-tabs__nav-item-text-wrapper) { display: flex; align-items: center; gap: 6px; font-size: var(--app-text-sm); }
  :deep(.t-tabs__content) { display: none; }
}
.case-tab-count { padding: 0 5px; border-radius: 4px; background: var(--td-bg-color-secondarycontainer); color: var(--td-text-color-secondary); font-size: 11px; line-height: 18px; font-variant-numeric: tabular-nums; }
.case-workbench-scroll { flex: 1; min-height: 0; overflow: auto; padding: 0 18px 20px; }
.case-workbench-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 16px 0 12px;
  .case-search { flex: 1; max-width: 290px; min-width: 0; }
  > span { flex-shrink: 0; color: var(--td-text-color-placeholder); font-size: var(--app-text-sm); font-variant-numeric: tabular-nums; }
}
.case-compact-table {
  :deep(.t-table__header th) { background: var(--td-bg-color-secondarycontainer); color: var(--td-text-color-secondary); font-size: 12px; font-weight: 400; padding: 8px; }
  :deep(.t-table__body td) { padding: 10px 8px; vertical-align: middle; }
  :deep(.case-selected-row td) { background: var(--app-selection-bg, var(--td-brand-color-light)); }
  :deep(.case-selected-row .case-title-button) { color: var(--td-brand-color); font-weight: 500; }
  :deep(.t-table__pagination) { margin: 0; padding: 12px 0; }
  :deep(.t-table__empty) { padding: 28px 12px; font-size: var(--app-text-sm); }
}
.case-record-id { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--app-font-family-mono, ui-monospace, monospace); font-size: 11px; color: var(--td-text-color-placeholder); }
.case-title-button { display: block; width: 100%; padding: 0; border: 0; background: transparent; color: var(--td-text-color-primary); font: inherit; font-size: var(--app-text-sm); line-height: 1.55; text-align: left; cursor: pointer; overflow-wrap: anywhere; &:focus-visible { outline: 2px solid var(--td-brand-color); outline-offset: 4px; border-radius: 2px; } }
.case-priority { display: inline-flex; justify-content: center; min-width: 28px; max-width: 100%; padding: 1px 5px; border-radius: 4px; background: var(--td-warning-color-1); color: var(--td-warning-color); font-size: 11px; overflow-wrap: anywhere; &.is-high { background: var(--td-error-color-1); color: var(--td-error-color); } }
.case-result-status { display: block; font-size: 11px; line-height: 1.6; color: var(--td-text-color-placeholder); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; &.is-passed { color: var(--td-success-color); } &.is-failed { color: var(--td-error-color); } &.is-blocked { color: var(--td-warning-color); } }
.case-value-empty { color: var(--td-text-color-placeholder); }
.case-record-detail { padding-top: 18px; margin-top: 18px; border-top: 1px solid var(--td-component-stroke); h5 { margin: 20px 0 12px; font-size: var(--app-text-sm); font-weight: 500; } }
.case-detail-heading { display: flex; align-items: baseline; gap: 10px; margin-bottom: 18px; h4 { flex: 1; min-width: 0; margin: 0; color: var(--td-text-color-primary); font-size: var(--app-text-md); font-weight: 500; line-height: 1.6; overflow-wrap: anywhere; } > .case-record-id, > .case-result-status { flex-shrink: 0; max-width: 90px; } }
.case-detail-fields { margin: 0;
  > div { display: grid; grid-template-columns: minmax(58px, 90px) minmax(0, 1fr); gap: 12px; margin-top: 14px; font-size: var(--app-text-sm); line-height: 1.75; }
  dt { color: var(--td-text-color-placeholder); overflow-wrap: anywhere; }
  dd { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; color: var(--td-text-color-primary); }
}
.case-detail-footer { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding-top: 16px; margin-top: 18px; border-top: 1px solid var(--td-component-stroke);
  > span { display: inline-flex; gap: 5px; align-items: center; color: var(--td-text-color-placeholder); font-size: 11px; }
}
.case-detail-empty, .case-section-empty { font-size: var(--app-text-sm); color: var(--td-text-color-placeholder); line-height: 1.7; padding: 16px 0; }
.case-note-section { margin-top: 20px; h4 { margin: 0 0 14px; font-size: var(--app-text-md); font-weight: 500; } & + & { margin-top: 28px; padding-top: 20px; border-top: 1px solid var(--td-component-stroke); } }
.case-note-table-wrap { overflow: auto; table { width: 100%; border-collapse: collapse; font-size: var(--app-text-sm); } td, th { padding: 8px; min-width: 100px; text-align: left; vertical-align: top; border-bottom: 1px solid var(--td-component-stroke); white-space: pre-wrap; overflow-wrap: anywhere; } th { color: var(--td-text-color-secondary); font-weight: 500; background: var(--td-bg-color-secondarycontainer); } }
.case-workbench-message { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin: 20px 18px; padding: 14px; border-radius: var(--app-radius-md); background: var(--td-warning-color-1); color: var(--td-text-color-secondary); font-size: var(--app-text-sm); > span { flex: 1; min-width: 120px; } }
.case-source-view { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.case-source-list { margin: 0; padding: 4px 18px 20px; list-style: none; overflow: auto;
  > li { display: flex; align-items: center; gap: 12px; padding: 14px 0; border-bottom: 1px solid var(--td-component-stroke); }
  .case-source-open { display: flex; gap: 10px; align-items: center; flex: 1; min-width: 0; padding: 0; border: 0; background: none; color: inherit; text-align: left; cursor: pointer; > span { min-width: 0; } strong { display: block; font-size: var(--app-text-sm); font-weight: 500; overflow-wrap: anywhere; } small { display: block; margin-top: 4px; font-size: 11px; color: var(--td-text-color-placeholder); } }
}
.case-preview-heading { display: flex; align-items: center; gap: 8px; padding: 12px 16px; flex-shrink: 0; font-size: var(--app-text-sm); > span:last-child { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; } }
@media (max-width: 420px) { .case-workbench-header { padding: 16px 12px 10px; } .case-workbench-title { gap: 6px; h3 { font-size: var(--app-text-md); } } .case-source-picker { padding: 0 12px 10px; gap: 5px; } .case-workbench-tabs { margin-inline: 12px; } .case-workbench-scroll { padding-inline: 12px; } .case-detail-fields > div { grid-template-columns: 62px minmax(0, 1fr); gap: 8px; } .case-compact-table :deep(.t-table__body td) { padding-inline: 5px; } }
</style>

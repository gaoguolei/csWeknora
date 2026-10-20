<template>
  <div class="test-case-panel">
    <div v-if="collecting && !items.length" class="test-case-empty">
      <t-loading size="small" />
      <span>{{ $t('agent.artifactDrawer.collecting') }}</span>
    </div>
    <div v-else-if="!items.length" class="test-case-empty">
      <t-icon name="folder-open" size="32px" />
      <span>{{ emptyText || $t('chat.sandbox.artifactsEmpty') }}</span>
    </div>
    <template v-else>
      <div v-if="tableItems.length" class="test-case-scroll">
        <section
          v-for="item in tableItems"
          :key="artifactKey(item)"
          class="test-case-file"
          :data-message-id="item.messageId"
          :data-artifact-index="item.index"
        >
          <header class="test-case-file-head">
            <ArtifactFileIcon :file-name="item.file_name" />
            <span class="test-case-file-name" :title="item.file_name">{{ item.file_name }}</span>
            <span v-if="rowCount(item)" class="test-case-file-count">
              {{ $t('workspace.caseSheetRows', { count: rowCount(item) }) }}
            </span>
            <ArtifactSaveToKnowledge :session-id="sessionId" :message-id="item.messageId"
              :artifact-index="item.index" :file-name="item.file_name"
              :knowledge-id="linkOf(item)?.knowledge_id" :knowledge-base-id="linkOf(item)?.knowledge_base_id"
              @saved="onKnowledgeLinked(item, $event)" />
            <t-button
              class="artifact-download"
              variant="text"
              shape="square"
              size="small"
              :title="$t('agent.artifactDrawer.download')"
              :aria-label="$t('agent.artifactDrawer.download')"
              :loading="!!downloading[artifactKey(item)]"
              :disabled="!sheetsOf(item)"
              @click="handleDownload(item)"
            >
              <template #icon><t-icon name="download" size="16px" /></template>
            </t-button>
            <t-button
              class="artifact-download artifact-delete"
              variant="text"
              shape="square"
              size="small"
              :title="$t('agent.artifactDrawer.delete')"
              :aria-label="$t('agent.artifactDrawer.delete')"
              :loading="!!deleting[artifactKey(item)]"
              @click="handleDelete(item)"
            >
              <template #icon><t-icon name="delete" size="16px" /></template>
            </t-button>
          </header>
          <div v-if="entries.get(artifactKey(item))?.status === 'sync-error'" class="test-case-file-loading">
            <span>{{ $t('workspace.caseSyncFailed') }}</span>
            <t-button variant="text" size="small" @click="parseItems">{{ $t('common.retry') }}</t-button>
          </div>
          <div v-else-if="!sheetsOf(item)" class="test-case-file-loading">
            <t-loading size="small" />
            <span>{{ $t('workspace.caseSheetParsing') }}</span>
          </div>
          <div
            v-for="sheet in sheetsOf(item) || []"
            :key="sheet.name"
            class="test-case-sheet"
          >
            <div v-if="(sheetsOf(item) || []).length > 1" class="test-case-sheet-name">
              {{ sheet.name }}
            </div>
            <div class="test-case-table-wrap">
              <table class="test-case-table">
                <thead>
                  <tr>
                    <th v-for="(title, col) in sheet.header" :key="col">
                      {{ title || `#${col + 1}` }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(row, rowIndex) in sheet.rows" :key="rowIndex">
                    <td v-for="(_, col) in sheet.header" :key="col">
                      {{ row[col] ?? '' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </div>
      <div v-if="fallbackItems.length" class="test-case-fallback">
        <p v-if="tableItems.length" class="test-case-fallback-hint">
          {{ $t('workspace.caseSheetUnparsedFiles') }}
        </p>
        <ChatArtifactsPanel
          class="test-case-fallback-list"
          :session-id="sessionId"
          :items="fallbackItems"
          :collecting="collecting"
          :active="active"
          allow-save-to-knowledge
          @deleted="emit('deleted', $event)"
        />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { deleteMessageArtifact, downloadArtifact, listMessageArtifacts, type ArtifactMeta } from '@/api/chat'
import { downKnowledgeDetails } from '@/api/knowledge-base'
import { KNOWLEDGE_FILE_UPDATED_EVENT } from '@/utils/knowledgeFileUpdates'
import { parseTestCaseSheets, isSpreadsheetArtifact, type TestCaseSheet } from '@/utils/testCaseSheet'
import type { SessionArtifactItem } from '@/utils/sessionArtifacts'
import { useConfirmDelete } from '@/components/settings/useConfirmDelete'
import ArtifactFileIcon from './ArtifactFileIcon.vue'
import ChatArtifactsPanel from './ChatArtifactsPanel.vue'
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
const confirmDelete = useConfirmDelete()
// 解析结果按产物行缓存：失败的行退回文件列表，成功的行只用表格展示。
const entries = reactive(new Map<string, CaseEntry>())
const downloading = reactive<Record<string, boolean>>({})
const deleting = reactive<Record<string, boolean>>({})
const knowledgeLinks = reactive(new Map<string, KnowledgeLink>())

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
  return (sheetsOf(item) || []).reduce((total, sheet) => total + sheet.rows.length, 0)
}

let parsedSessionId = ''
let parseGeneration = 0
// Linked knowledge is mutable: resolve bindings and refresh its bytes on activation.
async function parseItems() {
  if (!props.active) return
  const sessionId = props.sessionId
  const request = ++parseGeneration
  const items = [...spreadsheetItems.value]
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
    if (!messageMetadata && !binding?.knowledge_id) {
      entries.set(key, { status: 'sync-error' })
      continue
    }
    if (!binding?.knowledge_id && entries.get(key)?.status === 'ready') continue
    entries.set(key, { status: 'loading' })
    try {
      const blob = binding?.knowledge_id
        ? await downKnowledgeDetails(binding.knowledge_id)
        : await downloadArtifact(sessionId, item.messageId, item.index)
      const sheets = await parseTestCaseSheets(blob, item.file_name)
      if (request !== parseGeneration) return
      entries.set(key, { status: 'ready', sheets })
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
.test-case-panel {
  flex: 1;
  min-height: 0;
  width: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.test-case-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 16px;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-md);
  text-align: center;
}

.test-case-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 8px 8px 12px;
}

.test-case-file + .test-case-file {
  margin-top: 12px;
}

.test-case-file-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 4px 8px;
}

.test-case-file-name {
  flex: 1;
  min-width: 0;
  font-size: var(--app-text-md);
  font-weight: 500;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.test-case-file-count {
  flex-shrink: 0;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
  font-variant-numeric: tabular-nums;
}

.test-case-file-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 4px;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-sm);
}

.test-case-sheet + .test-case-sheet {
  margin-top: 8px;
}

.test-case-sheet-name {
  padding: 4px 4px 6px;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-secondary);
}

.test-case-table-wrap {
  overflow-x: auto;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
}

.test-case-table {
  border-collapse: collapse;
  width: 100%;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-primary);

  th,
  td {
    padding: 6px 10px;
    border-bottom: 1px solid var(--td-component-stroke);
    border-right: 1px solid var(--td-component-stroke);
    text-align: left;
    vertical-align: top;
    min-width: 72px;
    max-width: 320px;
    white-space: pre-wrap;
    word-break: break-word;

    &:last-child {
      border-right: 0;
    }
  }

  th {
    background: var(--td-bg-color-secondarycontainer);
    font-weight: 600;
    white-space: nowrap;
  }

  tbody tr:last-child td {
    border-bottom: 0;
  }

  tbody tr:hover td {
    background: var(--td-bg-color-container-hover);
  }
}

.test-case-fallback {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--td-component-stroke);
}

.test-case-fallback-hint {
  margin: 0;
  padding: 8px 12px;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-secondary);
}

.test-case-fallback-list {
  flex: 1;
  min-height: 0;
}

.artifact-download.t-button {
  flex-shrink: 0;
  width: 30px;
  height: 30px;
  border-radius: var(--app-radius-sm);
  color: var(--td-text-color-secondary);
  transition: background-color var(--app-motion-fast) ease, color var(--app-motion-fast) ease;

  &:not(.t-is-disabled):not(.t-is-loading):hover {
    background: color-mix(in srgb, var(--td-text-color-primary) 10%, var(--td-bg-color-container));
    color: var(--td-text-color-primary);
  }

  &.artifact-delete:not(.t-is-disabled):not(.t-is-loading):hover {
    background: var(--td-error-color-1);
    color: var(--td-error-color);
  }

  :deep(.t-button__icon) {
    margin: 0;
  }
}
</style>

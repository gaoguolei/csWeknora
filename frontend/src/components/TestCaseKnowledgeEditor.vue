<template>
  <section class="case-editor-page">
    <header class="case-editor-header">
      <t-button class="case-back" variant="outline" shape="square" :disabled="saving"
        :title="t('workspace.caseBackToKnowledge')" :aria-label="t('workspace.caseBackToKnowledge')" @click="close">
        <template #icon><t-icon name="chevron-left" /></template>
      </t-button>
      <div class="case-editor-title">
        <h2>{{ t('workspace.caseEditorTitle') }}</h2>
        <div class="case-file-meta"><t-icon name="file" /><span :title="fileName">{{ fileName }}</span><span class="case-meta-separator">·</span><span>{{ t('workspace.caseSheetRows', { count: totalCaseCount }) }}</span></div>
      </div>
      <span v-if="!loading && !failed" class="case-save-state" :class="{ 'is-dirty': hasChanges }">
        <span class="case-state-dot" />{{ t(hasChanges ? 'workspace.caseUnsaved' : 'workspace.caseSynced') }}
      </span>
      <t-button class="case-save-button" theme="primary" :loading="saving" :disabled="loading || failed || !hasChanges" @click="save">
        <template #icon><t-icon name="save" /></template>
        {{ t('common.save') }}
      </t-button>
    </header>
    <div v-if="discardPending" class="case-editor-discard" role="alert">
      <span>{{ t('workspace.caseDiscardTitle') }} {{ t('workspace.caseDiscardBody') }}</span>
      <t-button variant="text" @click="discardPending = false">{{ t('common.cancel') }}</t-button>
      <t-button theme="danger" variant="outline" @click="emit('close')">{{ t('common.confirm') }}</t-button>
    </div>
    <div v-if="loading" class="case-loading"><t-loading size="small" /><span>{{ t('workspace.caseSheetParsing') }}</span></div>
    <t-alert v-else-if="failed" theme="error" :message="t('workspace.caseLoadFailed')" />
    <template v-else>
      <t-tabs class="case-sheet-tabs" :value="activeSheetName" @change="selectSheet">
        <t-tab-panel v-for="section in caseSections" :key="section.sheet.name" :value="section.sheet.name"
          :disabled="saving || (!!draft && draft.sheet !== section.sheet.name)">
          <template #label><span>{{ section.sheet.name }}</span><span class="case-tab-count">{{ section.rows.filter(row => !row.isNew).length }}</span></template>
        </t-tab-panel>
      </t-tabs>
      <div class="case-editor-scroll">
        <section v-if="activeSection" class="case-list-section">
          <div class="case-list-heading">
            <div class="case-list-summary"><strong>{{ t(activeSection.isNotes ? 'workspace.caseRecordCount' : 'workspace.caseSheetRows', { count: activeSection.rows.filter(row => !row.isNew).length }) }}</strong><span>{{ t('workspace.caseEditHint') }}</span></div>
            <t-button variant="outline" :disabled="saving || !!draft" @click="addRow(activeSection.sheet)">
              <template #icon><t-icon name="add" /></template>{{ t(activeSection.isNotes ? 'workspace.caseAddRecord' : 'workspace.caseAddRow') }}
            </t-button>
          </div>
          <t-table :key="activeSection.sheet.name" class="case-list-table" row-key="index" :data="activeSection.rows" :columns="activeSection.tableColumns"
            hover resizable size="medium" table-layout="fixed" vertical-align="top"
            max-height="max(260px, calc(100dvh - 340px))" :table-content-width="activeSection.tableWidth"
            :pagination="activeSection.rows.length > pageSize ? { current: currentPage, pageSize, total: activeSection.rows.length, showPageSize: false, showJumper: true, disabled: saving || !!draft } : undefined"
            :empty="t('workspace.caseListEmpty')" @page-change="onPageChange">
            <template v-for="(column, c) in activeSection.columns" #[`case-field-${c}`]="{ row }">
              <t-textarea v-if="draft?.sheet === activeSection.sheet.name && draft.row === row.index"
                v-model="draft.values[c]" :disabled="saving" :aria-label="column"
                :autosize="{ minRows: 2, maxRows: 6 }" />
              <t-tag v-else-if="activeSection.fieldKinds[c] === 'priority' && row.cells[c]" class="case-priority" size="small" variant="light" :theme="priorityTheme(row.cells[c])">{{ row.cells[c] }}</t-tag>
              <div v-else class="case-cell" :class="{ 'case-cell-identifier': activeSection.fieldKinds[c] === 'identifier' }">
                <div class="case-field-value" :title="row.cells[c]" :class="{ 'is-expanded': expandedCells.has(`${activeSection.sheet.name}:${row.index}:${c}`) }">{{ row.cells[c] || '—' }}</div>
                <button v-if="isLongValue(row.cells[c])" type="button" class="case-expand-value" :aria-expanded="expandedCells.has(`${activeSection.sheet.name}:${row.index}:${c}`)"
                  @click="toggleExpanded(`${activeSection.sheet.name}:${row.index}:${c}`)">{{ t(expandedCells.has(`${activeSection.sheet.name}:${row.index}:${c}`) ? 'common.collapse' : 'common.expand') }}</button>
              </div>
            </template>
            <template #actions="{ row }">
              <div v-if="draft?.sheet === activeSection.sheet.name && draft.row === row.index" class="case-row-actions">
                <t-button variant="text" size="small" :disabled="saving" @click="applyRowEdit">{{ t('workspace.caseApplyRow') }}</t-button>
                <t-button variant="text" size="small" :disabled="saving" @click="cancelRowEdit">{{ t('common.cancel') }}</t-button>
              </div>
              <t-button v-else class="case-edit-button" variant="text" size="small" :disabled="saving || !!draft" @click="editRow(activeSection.sheet, row.index)">
                <template #icon><t-icon name="edit" /></template>{{ t('common.edit') }}
              </t-button>
            </template>
          </t-table>
        </section>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'
import { downKnowledgeDetails, replaceKnowledgeFile } from '@/api/knowledge-base'
import { notifyKnowledgeFileUpdated } from '@/utils/knowledgeFileUpdates'
import { readCaseWorkbook, writeCaseWorkbook, type EditableCaseSheet, type CaseCellEdit } from '@/utils/testCaseWorkbook'

const props = defineProps<{ knowledgeId: string; fileName: string }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()
const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const failed = ref(false)
const discardPending = ref(false)
const sheets = ref<EditableCaseSheet[]>([])
const workbook = shallowRef<Awaited<ReturnType<typeof readCaseWorkbook>>['workbook']>()
const edits = ref(new Map<string, CaseCellEdit>())
const draft = ref<{ sheet: string; row: number; isNew: boolean; values: string[] } | null>(null)
const activeSheetName = ref('')
const currentPage = ref(1)
const pageSize = 20
const expandedCells = ref(new Set<string>())
const draftChanged = computed(() => {
  if (!draft.value) return false
  const original = sheets.value.find(sheet => sheet.name === draft.value!.sheet)?.cells[draft.value.row] || []
  return draft.value.values.some((value, col) => value !== (original[col] ?? ''))
})
const hasChanges = computed(() => edits.value.size > 0 || draftChanged.value)
const caseSections = computed(() => sheets.value.map(sheet => {
  const header = Math.max(0, sheet.cells.findIndex(row => row.some(cell => cell.trim() !== '')))
  const sourceColumns = (sheet.cells[header] || ['']).map((name, col) => name.trim() || t('workspace.caseField', { index: col + 1 }))
  const isNotes = /说明|概览|介绍|readme|overview|notes|summary/i.test(sheet.name)
    || (sourceColumns.length <= 2 && !sourceColumns.some(name => ['identifier', 'steps', 'result', 'title'].includes(fieldKind(name))))
  const columns = isNotes && sourceColumns.length === 2 ? [t('workspace.caseInfoField'), t('workspace.caseInfoValue')] : sourceColumns
  const fieldKinds = columns.map(fieldKind)
  const rows = sheet.cells.flatMap((cells, index) =>
    index > header && cells.some(cell => cell.trim() !== '') ? [{ index, cells, isNew: false }] : [],
  )
  if (draft.value?.sheet === sheet.name && draft.value.isNew) {
    rows.push({ index: draft.value.row, cells: draft.value.values, isNew: true })
  }
  const tableColumns: PrimaryTableCol[] = [
    { colKey: 'serial-number', title: '#', width: 56, fixed: 'left', align: 'center', className: 'case-index-column' },
    ...columns.map((title, col): PrimaryTableCol => ({ colKey: `case-field-${col}`, title,
      width: isNotes && columns.length === 2 ? (col === 0 ? 200 : undefined) : fieldWidth(fieldKinds[col]),
      minWidth: fieldKinds[col] === 'priority' ? 88 : 120,
      align: fieldKinds[col] === 'priority' ? 'center' : 'left',
    })),
    { colKey: 'actions', title: t('workspace.caseActions'), width: 136, fixed: 'right', className: 'case-action-column' },
  ]
  const tableWidth = isNotes ? '100%' : `max(100%, ${tableColumns.reduce((total, column) => total + Number(column.width || 0), 0)}px)`
  return { sheet, columns, fieldKinds, tableColumns, tableWidth, rows, isNotes }
}))
const activeSection = computed(() => caseSections.value.find(section => section.sheet.name === activeSheetName.value))
const totalCaseCount = computed(() => caseSections.value.filter(section => !section.isNotes).reduce((total, section) => total + section.rows.filter(row => !row.isNew).length, 0))
let generation = 0

function fieldKind(name: string): string {
  const value = name.replace(/[\s_-]/g, '').toLowerCase()
  if (/^(id|caseid|testcaseid|用例编号|编号|用例id|测试用例编号)$/.test(value)) return 'identifier'
  if (/优先级|用例级别|^priority$|^级别$/.test(value)) return 'priority'
  if (/步骤|steps/.test(value)) return 'steps'
  if (/预期|expected|实际结果|actualresult/.test(value)) return 'result'
  if (/前置|预置|precondition/.test(value)) return 'condition'
  if (/模块|module/.test(value)) return 'module'
  if (/用例名称|用例标题|测试场景|^title$|^name$|^scenario$/.test(value)) return 'title'
  return 'text'
}

function fieldWidth(kind: string): number {
  return ({ identifier: 136, priority: 96, steps: 280, result: 260, condition: 220, module: 144, title: 240 } as Record<string, number>)[kind] || 200
}

function priorityTheme(value: string): 'danger' | 'warning' | 'primary' | 'default' {
  if (/^(p0|最高|紧急|critical)$/i.test(value.trim())) return 'danger'
  if (/^(p1|高|high)$/i.test(value.trim())) return 'warning'
  if (/^(p2|中|medium)$/i.test(value.trim())) return 'primary'
  return 'default'
}

function selectSheet(value: string | number) {
  const name = String(value)
  if (saving.value || (draft.value && draft.value.sheet !== name)) return
  activeSheetName.value = name
  currentPage.value = 1
}

function onPageChange({ current }: { current: number }) {
  if (!draft.value && !saving.value) currentPage.value = current
}

function isLongValue(value: string): boolean {
  return !!value && (value.length > 90 || value.split('\n').length > 3)
}

function toggleExpanded(key: string) {
  if (expandedCells.value.has(key)) expandedCells.value.delete(key)
  else expandedCells.value.add(key)
}

onBeforeUnmount(() => { generation++ })

watch(() => [props.knowledgeId, props.fileName] as const, async () => {
  const request = ++generation
  discardPending.value = false
  loading.value = true
  failed.value = false
  workbook.value = undefined
  sheets.value = []
  edits.value.clear()
  draft.value = null
  expandedCells.value.clear()
  activeSheetName.value = ''
  currentPage.value = 1
  try {
    const data = await readCaseWorkbook(await downKnowledgeDetails(props.knowledgeId), props.fileName)
    if (request !== generation) return
    workbook.value = data.workbook
    sheets.value = data.sheets
    activeSheetName.value = (caseSections.value.find(section => !section.isNotes) || caseSections.value[0])?.sheet.name || ''
  } catch {
    if (request === generation) failed.value = true
  } finally {
    if (request === generation) loading.value = false
  }
}, { immediate: true })

function updateCell(sheet: EditableCaseSheet, row: number, col: number, value: string) {
  discardPending.value = false
  sheet.cells[row][col] = value
  edits.value.set(`${sheet.name}:${row}:${col}`, {
    sheet: sheet.name, row: row + sheet.startRow, col: col + sheet.startCol, value,
  })
}

function addRow(sheet: EditableCaseSheet) {
  if (saving.value || draft.value) return
  const columns = caseSections.value.find(section => section.sheet.name === sheet.name)?.columns || ['']
  draft.value = { sheet: sheet.name, row: sheet.cells.length, isNew: true, values: columns.map(() => '') }
  activeSheetName.value = sheet.name
  currentPage.value = Math.ceil((caseSections.value.find(section => section.sheet.name === sheet.name)?.rows.length || 1) / pageSize)
}

function editRow(sheet: EditableCaseSheet, row: number) {
  if (saving.value || draft.value) return
  draft.value = { sheet: sheet.name, row, isNew: false, values: [...sheet.cells[row]] }
}

function cancelRowEdit() {
  if (saving.value) return
  draft.value = null
  currentPage.value = Math.min(currentPage.value, Math.max(1, Math.ceil((activeSection.value?.rows.length || 0) / pageSize)))
}

function applyRowEdit(): boolean {
  if (!draft.value) return true
  const current = draft.value
  const sheet = sheets.value.find(sheet => sheet.name === current.sheet)
  if (!sheet) return false
  if (!current.values.some(value => value.trim() !== '')) {
    MessagePlugin.warning(t('workspace.caseRowRequired'))
    return false
  }
  if (current.isNew) sheet.cells.push(current.values.map(() => ''))
  current.values.forEach((value, col) => {
    if (value !== (sheet.cells[current.row][col] ?? '')) updateCell(sheet, current.row, col, value)
  })
  draft.value = null
  return true
}

function close() {
  if (saving.value) return
  if (!hasChanges.value) {
    emit('close')
    return
  }
  discardPending.value = true
}

async function save() {
  if (saving.value || !workbook.value || !hasChanges.value || !applyRowEdit()) return
  saving.value = true
  const request = generation
  const { knowledgeId, fileName } = props
  try {
    const file = await writeCaseWorkbook(workbook.value, [...edits.value.values()], fileName)
    const updated = await readCaseWorkbook(file, fileName)
    await replaceKnowledgeFile(knowledgeId, file)
    if (request !== generation) return
    workbook.value = updated.workbook
    edits.value.clear()
    discardPending.value = false
    MessagePlugin.success(t('workspace.caseEditSaved'))
    notifyKnowledgeFileUpdated(knowledgeId)
    emit('saved')
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('workspace.caseSaveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped lang="less">
@import (reference) '@/components/css/workspace-page.less';
.case-editor-page {
  .workspace-page();
  gap: 0;
  padding-bottom: 24px;
  overflow: hidden;
}
.case-editor-header {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
  padding-bottom: 26px;
  .case-back { width: 36px; height: 36px; border-color: var(--td-component-stroke); border-radius: var(--app-radius-md); }
}
.case-editor-title {
  flex: 1;
  min-width: 0;
  h2 { margin: 0 0 7px; font-size: 24px; line-height: 1.2; font-weight: 600; letter-spacing: -.5px; }
}
.case-file-meta {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-sm);
  > span:first-of-type { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 360px; }
  > span:last-child { flex-shrink: 0; font-variant-numeric: tabular-nums; }
  .case-meta-separator { color: var(--td-text-color-placeholder); margin: 0 3px; }
}
.case-save-state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-sm);
  white-space: nowrap;
  .case-state-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--td-text-color-placeholder); opacity: .65; }
  &.is-dirty {
    color: var(--td-warning-color);
    .case-state-dot { background: currentColor; opacity: 1; }
  }
}
.case-save-button { min-width: 88px; border-radius: var(--app-radius-md); }
.case-editor-discard {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px;
  background: var(--td-warning-color-1);
  border-radius: var(--app-radius-md);
  margin-bottom: 16px;
  span { flex: 1; min-width: 180px; }
}
.case-editor-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-top: 20px;
}
.case-loading { display: flex; gap: 10px; align-items: center; justify-content: center; min-height: 280px; color: var(--td-text-color-secondary); }
.case-sheet-tabs {
  flex-shrink: 0;
  :deep(.t-tabs__nav-item) { padding: 0; margin: 0; height: 46px; }
  :deep(.t-tabs__nav-item-wrapper) { margin: 0; padding: 0 18px; }
  :deep(.t-tabs__nav-item-text-wrapper) { display: flex; align-items: center; gap: 8px; }
  :deep(.t-tabs__content) { display: none; }
}
.case-tab-count {
  padding: 1px 6px;
  min-width: 18px;
  border-radius: 5px;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  font-size: 11px;
  line-height: 18px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.case-list-heading {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  .t-button { flex-shrink: 0; border-radius: var(--app-radius-md); }
}
.case-list-summary {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  gap: 14px;
  font-size: var(--app-text-sm);
  strong { font-weight: 500; font-variant-numeric: tabular-nums; white-space: nowrap; }
  > span { color: var(--td-text-color-placeholder); line-height: 1.5; }
}
.case-list-table {
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-lg);
  overflow: hidden;
  font-size: var(--app-text-sm);
  :deep(.t-table__header th) { background: var(--td-bg-color-secondarycontainer); color: var(--td-text-color-secondary); font-weight: 500; height: 44px; padding: 10px 14px; }
  :deep(.t-table__body td) { padding: 14px; border-color: var(--td-component-stroke); }
  :deep(.case-index-column) { color: var(--td-text-color-placeholder); font-size: 12px; font-variant-numeric: tabular-nums; }
  :deep(.t-table__pagination) { margin: 0; padding: 14px 18px; border-top: 1px solid var(--td-component-stroke); }
  :deep(.t-textarea) { width: 100%; }
  :deep(.t-textarea__inner) { line-height: 1.65; font-size: var(--app-text-sm); }
  :deep(.t-table__cell-empty) { padding: 32px 16px; }
}
.case-field-value {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: 1.7;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
  &.is-expanded { display: block; }
}
.case-cell-identifier { font-family: var(--app-font-family-mono, ui-monospace, monospace); font-size: 12px; color: var(--td-text-color-secondary); }
.case-expand-value { padding: 3px 0 0; border: 0; background: none; font: inherit; font-size: 12px; color: var(--td-brand-color); cursor: pointer; }
.case-priority { min-width: 32px; justify-content: center; font-size: 11px; font-weight: 500; letter-spacing: .3px; }
.case-edit-button { color: var(--td-text-color-secondary); }
.case-row-actions { display: flex; align-items: center; gap: 4px; :deep(.t-button) { padding-inline: 4px; } }
@media (max-width: 720px) {
  .case-editor-header { gap: 10px; padding-bottom: 20px; }
  .case-editor-title h2 { font-size: 20px; }
  .case-save-state { display: none; }
  .case-file-meta > span:first-of-type { max-width: 160px; }
  .case-list-summary { display: block; > span { display: block; margin-top: 4px; } }
}
</style>

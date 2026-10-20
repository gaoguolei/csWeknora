import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import * as Vue from 'vue'
import { computed, createSSRApp, ref, shallowRef } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { compileTemplate } from '@vue/compiler-sfc'
import TDesign from 'tdesign-vue-next'
import * as XLSX from 'xlsx'
import { readCaseWorkbook, writeCaseWorkbook } from '../utils/testCaseWorkbook.ts'

const source = readFileSync(new URL('./TestCaseKnowledgeEditor.vue', import.meta.url), 'utf8')
const script = ts.transpile(source.slice(source.indexOf('const loading = ref'), source.indexOf('</script>')), { target: ts.ScriptTarget.ES2022 })

async function setup(failSave = false, rows = [['ID', 'Steps'], ['TC-1', 'original']], additionalSheets: Record<string, string[][]> = {}) {
  const book = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(book, XLSX.utils.aoa_to_sheet(rows), 'Cases')
  for (const [name, cells] of Object.entries(additionalSheets)) XLSX.utils.book_append_sheet(book, XLSX.utils.aoa_to_sheet(cells), name)
  const original = new Blob([XLSX.write(book, { type: 'array', bookType: 'xlsx' })])
  const uploads: File[] = []
  const events: string[] = []
  const errors: string[] = []
  const updates: string[] = []
  let loaded: Promise<void> | undefined
  const state = vm.runInNewContext(`${script}; ({ loading, saving, failed, sheets, edits, draft, caseSections, activeSection, activeSheetName, totalCaseCount, currentPage, pageSize, expandedCells, selectSheet, onPageChange, priorityTheme, isLongValue, toggleExpanded, hasChanges, discardPending, editRow, applyRowEdit, cancelRowEdit, addRow, save, close })`, {
    computed, ref, shallowRef, onBeforeUnmount: () => {},
    watch: (_: unknown, callback: () => Promise<void>) => { loaded = callback() },
    props: { knowledgeId: 'cases', fileName: 'cases.xlsx' },
    readCaseWorkbook, writeCaseWorkbook,
    notifyKnowledgeFileUpdated: (id: string) => updates.push(id),
    downKnowledgeDetails: async () => original,
    replaceKnowledgeFile: async (id: string, file: File) => {
      assert.equal(id, 'cases')
      if (failSave) throw new Error('storage unavailable')
      uploads.push(file)
    },
    t: (key: string) => key,
    MessagePlugin: { success: () => {}, error: (message: string) => errors.push(message), warning: (message: string) => errors.push(message) },
    emit: (event: string) => events.push(event),
  })
  await loaded
  return { state, events, uploads, errors, updates }
}

test('saving stays on the inline editor and successive saves keep previous changes', async () => {
  const { state, events, uploads, updates } = await setup()
  const sheet = state.sheets.value[0]
  state.editRow(sheet, 1)
  state.draft.value.values[1] = 'changed'
  await state.save()
  assert.deepEqual(events, ['saved'])
  assert.equal(state.edits.value.size, 0)
  state.addRow(sheet)
  state.draft.value.values = ['TC-2', 'new case']
  await state.save()
  const saved = await readCaseWorkbook(uploads[1], 'cases.xlsx')
  assert.equal(saved.sheets[0].cells[1][1], 'changed')
  assert.deepEqual(saved.sheets[0].cells[2], ['TC-2', 'new case'])
  assert.deepEqual(events, ['saved', 'saved'])
  assert.deepEqual(updates, ['cases', 'cases'])
  state.close()
  assert.equal(events.at(-1), 'close')
})

test('returning with unsaved edits shows an inline warning without closing', async () => {
  const { state, events } = await setup()
  state.editRow(state.sheets.value[0], 1)
  state.draft.value.values[1] = 'unsaved'
  state.close()
  assert.equal(state.discardPending.value, true)
  assert.deepEqual(events, [])
  assert.doesNotMatch(source, /<t-dialog|DialogPlugin|<t-drawer/)
  assert.match(source, /class="case-editor-discard" role="alert"/)
})

test('failed saves keep the edited cells available for retry', async () => {
  const { state, events, errors } = await setup(true)
  state.editRow(state.sheets.value[0], 1)
  state.draft.value.values[1] = 'unsaved'
  await state.save()
  assert.equal(state.edits.value.size, 1)
  assert.equal(state.sheets.value[0].cells[1][1], 'unsaved')
  assert.deepEqual(events, [])
  assert.deepEqual(errors, ['storage unavailable'])
})

test('case lists keep column headings out of the records and preserve row coordinates', async () => {
  const { state, uploads } = await setup(false, [['', ''], ['ID', 'Steps'], ['', ''], ['TC-1', 'original']])
  const section = state.caseSections.value[0]
  assert.deepEqual(Array.from(section.columns), ['ID', 'Steps'])
  assert.equal(section.rows.length, 1)
  assert.equal(section.rows[0].index, 3)
  state.editRow(section.sheet, section.rows[0].index)
  state.draft.value.values[1] = 'changed'
  await state.save()
  const saved = await readCaseWorkbook(uploads[0], 'cases.xlsx')
  assert.equal(saved.workbook.Sheets.Cases.B4.v, 'changed')
  assert.equal(saved.workbook.Sheets.Cases.B2.v, 'Steps')
})

test('canceling a row draft discards only that draft without changing the original case', async () => {
  const { state } = await setup()
  const sheet = state.sheets.value[0]
  state.editRow(sheet, 1)
  state.draft.value.values[1] = 'temporary'
  assert.equal(sheet.cells[1][1], 'original')
  state.cancelRowEdit()
  assert.equal(state.hasChanges.value, false)
  state.addRow(sheet)
  assert.equal(sheet.cells.length, 2)
  state.cancelRowEdit()
  assert.equal(state.caseSections.value[0].rows.length, 1)
})

test('applying one case updates its fields and rejects an empty new case', async () => {
  const { state, errors } = await setup()
  const sheet = state.sheets.value[0]
  state.editRow(sheet, 1)
  state.draft.value.values[1] = 'changed'
  assert.equal(state.applyRowEdit(), true)
  assert.equal(sheet.cells[1][1], 'changed')
  assert.equal(state.edits.value.size, 1)
  state.addRow(sheet)
  assert.equal(state.applyRowEdit(), false)
  assert.equal(sheet.cells.length, 2)
  assert.equal(errors.at(-1), 'workspace.caseRowRequired')
})

test('the Vue table renders field editors in the original columns of only the selected row', async () => {
  const { state } = await setup(false, [['ID', 'Steps'], ['TC-1', 'first case'], ['TC-2', 'second case']])
  const template = source.slice(source.indexOf('<template>') + '<template>'.length, source.indexOf('\n</template>'))
  const compiled = compileTemplate({ source: template, filename: 'TestCaseKnowledgeEditor.vue', id: 'case-table-test' })
  assert.deepEqual(compiled.errors, [])
  const exports: { render?: any } = {}
  vm.runInNewContext(ts.transpile(compiled.code, { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }), {
    exports, require: (id: string) => { assert.equal(id, 'vue'); return Vue },
  })
  const render = async () => {
    const app = createSSRApp({
      setup: () => ({ ...state, fileName: 'cases.xlsx', t: (key: string) => key }),
      render: exports.render,
    })
    app.use(TDesign)
    return renderToString(app)
  }
  const normal = await render()
  assert.match(normal, /class="[^"]*\bt-table\b/)
  assert.match(normal, /TC-1/)
  assert.match(normal, /TC-2/)
  assert.doesNotMatch(normal, /<textarea/)
  state.editRow(state.sheets.value[0], 1)
  const editing = await render()
  const rows = editing.match(/<tr\b[\s\S]*?<\/tr>/g) || []
  const editableRows = rows.filter(row => row.includes('<textarea'))
  assert.equal(editableRows.length, 1)
  assert.equal((editableRows[0].match(/<textarea/g) || []).length, 2)
  assert.doesNotMatch(editableRows[0], /colspan=/)
  assert.ok(rows.some(row => row.includes('TC-2') && !row.includes('<textarea')))
  assert.equal(state.caseSections.value[0].tableColumns.at(-1).fixed, 'right')
})

test('overview records have their own tab and the case sheet opens by default', async () => {
  const { state } = await setup(false, [['用户管理系统 — 测试用例说明', ''], ['被测项目', '用户管理系统']], {
    测试用例: [['用例编号', '优先级', '测试步骤'], ['TC-1', 'P0', '打开用户列表']],
  })
  assert.equal(state.activeSheetName.value, '测试用例')
  assert.equal(state.totalCaseCount.value, 1)
  const columns = state.activeSection.value.tableColumns
  assert.ok(columns[2].width < columns[3].width, 'priority takes less space than test steps')
  assert.equal(state.priorityTheme('P0'), 'danger')
  state.selectSheet('Cases')
  assert.equal(state.activeSection.value.isNotes, true)
  assert.equal(state.activeSection.value.tableWidth, '100%')
  assert.equal(state.activeSection.value.tableColumns[2].width, undefined, 'notes content fills the remaining width')
  assert.equal(state.totalCaseCount.value, 1, 'overview records are not counted as test cases')
  assert.equal(state.sheets.value[0].cells[0][0], '用户管理系统 — 测试用例说明', 'display labels do not modify the source header')
  const compact = await setup(false, [['Case ID', 'Title'], ['TC-1', 'Login succeeds']])
  assert.equal(compact.state.activeSection.value.isNotes, false, 'compact test case schemas remain case lists')
})

test('adding and canceling on the last page keeps the table on a valid page and edits the original row', async () => {
  const rows = [['ID', 'Steps'], ...Array.from({ length: 40 }, (_, i) => [`TC-${i + 1}`, 'original'])]
  const { state, uploads } = await setup(false, rows)
  state.onPageChange({ current: 2 })
  state.addRow(state.sheets.value[0])
  assert.equal(state.currentPage.value, 3, 'a new case is shown on the last page')
  state.cancelRowEdit()
  assert.equal(state.currentPage.value, 2, 'canceling removes the otherwise empty last page')
  state.editRow(state.sheets.value[0], 21)
  state.onPageChange({ current: 1 })
  assert.equal(state.currentPage.value, 2, 'pagination does not hide an active row draft')
  state.draft.value.values[1] = 'changed on page two'
  await state.save()
  const saved = await readCaseWorkbook(uploads[0], 'cases.xlsx')
  assert.equal(saved.workbook.Sheets.Cases.B22.v, 'changed on page two')
  assert.equal(saved.workbook.Sheets.Cases.B2.v, 'original')
})

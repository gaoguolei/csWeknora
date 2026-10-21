import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import * as Vue from 'vue'
import { computed, createSSRApp, reactive, ref } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { compileTemplate } from '@vue/compiler-sfc'
import TDesign from 'tdesign-vue-next'
import { isSpreadsheetArtifact, parseTestCaseSheets } from '../../../utils/testCaseSheet.ts'
import { buildTestCaseWorkbench } from '../../../utils/testCaseWorkbench.ts'
import { resolveFilePreviewExt } from '../../../utils/filePreview.ts'

const source = readFileSync(new URL('./TestCasePanel.vue', import.meta.url), 'utf8')

const start = source.indexOf('const entries = reactive')
const end = source.indexOf('let parsedSessionId', start)
assert.ok(start >= 0 && end > start, 'case panel keeps its parsed-entry computeds in one block')
const computedsSource = ts.transpile(source.slice(start, end), { target: ts.ScriptTarget.ES2022 })

function setup(items) {
  const props = reactive({ sessionId: 'session-a', items, active: true, collecting: false, emptyText: '' })
  const state = vm.runInNewContext(
    `${computedsSource}; ({ entries, tableItems, fallbackItems, rowCount, sheetsOf, workbench, view, selectView, selectArtifact, activeArtifact, filteredCases, selectedCase, selectedCaseKey, searchTerm, casePage, changePage, openSource, sourcePreview, pageSize, sourceOptions, compactColumns, activeEntry, activeLoading, linkOf, previewRevision, sourcePreviewKey, caseRowClass, selectCase, statusLabel, openKnowledge, downloading, deleting, artifactKey })`,
    { computed, reactive, ref, watch: () => {}, t: key => key, props, isSpreadsheetArtifact, buildTestCaseWorkbench },
  )
  return { ...state, props }
}

const cases = { messageId: 'a', index: 3, file_name: '登录测试用例.xlsx' }
const sheet = { name: 'Sheet1', header: ['用例编号'], rows: [['TC-1'], ['TC-2']] }

test('a parsed case file renders as a table instead of a file entry', () => {
  const view = setup([cases])
  assert.equal(view.tableItems.value.length, 1)
  assert.equal(view.fallbackItems.value.length, 0)
  view.entries.set('a:3:登录测试用例.xlsx', { status: 'ready', sheets: [sheet] })
  assert.equal(view.tableItems.value.length, 1)
  assert.equal(view.rowCount(cases), 2)
  assert.deepEqual(view.sheetsOf(cases), [sheet])
})

test('an unparseable case file falls back to the file list', () => {
  const view = setup([cases])
  view.entries.set('a:3:登录测试用例.xlsx', { status: 'failed' })
  assert.equal(view.tableItems.value.length, 0)
  assert.equal(view.fallbackItems.value.length, 1)
  assert.equal(view.rowCount(cases), 0)
})

test('non-spreadsheet case documents go straight to the file list', () => {
  const view = setup([cases, { messageId: 'b', index: 1, file_name: '用例设计.md' }])
  assert.deepEqual(
    Array.from(view.fallbackItems.value, (item) => item.file_name),
    ['用例设计.md'],
  )
})

test('the workbench has a dedicated source preview instead of stacking unparsed files below every sheet', () => {
  assert.match(source, /caseSourcesTab/)
  assert.match(source, /<DocumentPreview/)
  assert.doesNotMatch(source, /<ChatArtifactsPanel/)
  assert.doesNotMatch(source, /caseSheetUnparsedFiles/)
  assert.ok(source.indexOf('<t-tab-panel value="notes"') < source.indexOf('<t-tab-panel value="cases"'))
  assert.equal(setup([cases]).view.value, 'notes')
})

const runtimeSource = ts.transpile(source.slice(start, source.indexOf('async function handleDownload(', start)), { target: ts.ScriptTarget.ES2022 })

function syncFixture() {
  const props = reactive({ sessionId: 'session-a', items: [{ ...cases, file_name: 'cases.csv' }], active: true })
  let knowledgeId = 'knowledge'
  let steps = 'edited in knowledge'
  let failKnowledge = false
  let originalDownloads = 0
  let knowledgeReader = null
  let watcher
  const document = { hidden: false }
  const state = vm.runInNewContext(`${runtimeSource}; ({ entries, knowledgeLinks, parseItems, sheetsOf, tableItems, fallbackItems, refreshVisibleCases, casePage, selectedCaseKey, selectedCase, previewRevision, openSource, view })`, {
    computed, reactive, ref, t: key => key, props, isSpreadsheetArtifact, parseTestCaseSheets, buildTestCaseWorkbench,
    listMessageArtifacts: async () => ({ data: [{ index: 3, knowledge_id: knowledgeId, knowledge_base_id: 'kb' }] }),
    downKnowledgeDetails: async () => {
      if (knowledgeReader) return knowledgeReader()
      if (failKnowledge) throw new Error('forbidden')
      return new Blob([`ID,Steps\nTC-1,${steps}\n`])
    },
    downloadArtifact: async () => { originalDownloads++; return new Blob(['ID,Steps\nTC-1,original\n']) },
    watch: (_source, callback) => { watcher = callback },
    onMounted: () => {}, onBeforeUnmount: () => {},
    document,
    console: { warn: () => {} },
  })
  return { state, props, document, watcher: () => watcher(),
    setSteps: value => { steps = value }, setLink: value => { knowledgeId = value },
    setKnowledgeReader: reader => { knowledgeReader = reader },
    failKnowledge: () => { failKnowledge = true }, originalDownloads: () => originalDownloads }
}

test('the testing panel reads linked knowledge and refreshes edits instead of reusing its old parse cache', async () => {
  const view = syncFixture()
  await view.state.parseItems()
  assert.equal(view.state.sheetsOf(view.props.items[0])[0].rows[0][1], 'edited in knowledge')
  view.setSteps('a later knowledge edit')
  await view.state.parseItems()
  assert.equal(view.state.sheetsOf(view.props.items[0])[0].rows[0][1], 'a later knowledge edit')
  assert.equal(view.originalDownloads(), 0)
})

test('case counts exclude overview rows and execution-result rows', () => {
  const view = setup([cases])
  view.entries.set('a:3:登录测试用例.xlsx', { status: 'ready', sheets: [
    { name: '用例说明', header: ['项目', '内容'], rows: [['项目', '用户管理'], ['环境', '浏览器']] },
    { name: '测试用例', header: ['用例编号', '用例名称'], rows: [['TC-1', '登录成功'], ['TC-2', '密码错误']] },
    { name: '执行结果', header: ['用例编号', '执行状态'], rows: [['TC-1', '通过'], ['TC-2', '失败']] },
  ] })
  assert.equal(view.rowCount(cases), 2)
  assert.equal(view.workbench.value.notes.length, 1)
  assert.equal(view.workbench.value.results.length, 2)
  assert.equal(view.workbench.value.cases[0].status, 'passed')
})

test('search, pagination and selection share the current filtered case list', () => {
  const view = setup([cases])
  view.entries.set('a:3:登录测试用例.xlsx', { status: 'ready', sheets: [
    { name: '测试用例', header: ['用例编号', '用例名称', '测试步骤'], rows: Array.from({ length: 12 }, (_, index) => [`TC-${index + 1}`, `登录场景 ${index + 1}`, `检查邮箱 ${index + 1}`]) },
  ] })
  view.changePage({ current: 2 })
  assert.equal(view.selectedCase.value.id, 'TC-9')
  view.casePage.value = 1
  view.searchTerm.value = '检查邮箱 12'
  assert.equal(view.filteredCases.value.length, 1)
  assert.equal(view.selectedCase.value.id, 'TC-12')
})

test('selecting Markdown opens a document preview within Sources', () => {
  const markdown = { messageId: 'b', index: 0, file_name: 'notes.md' }
  const view = setup([cases, markdown])
  view.selectArtifact('b:0:notes.md')
  assert.equal(view.view.value, 'files')
  assert.equal(view.sourcePreview.value.file_name, 'notes.md')
  view.selectView('cases')
  assert.equal(view.sourcePreview.value, null)
})

test('the rendered workbench keeps compact columns, shows selected details, and separates notes and source previews', async () => {
  const markdown = { messageId: 'b', index: 0, file_name: 'notes.md', knowledge_id: 'markdown-knowledge', knowledge_base_id: 'kb' }
  const view = setup([cases, markdown])
  view.entries.set('a:3:登录测试用例.xlsx', { status: 'ready', sheets: [
    { name: '用例说明', header: ['项目', '内容'], rows: [['说明', '这段说明不应占据列表首屏']] },
    { name: '测试用例', header: ['编号', '名称', '优先级', '前置条件', '步骤', '预期结果'], rows: [
      ['TC-1', '显示用户', 'P0', '管理员登录', '打开列表', '展示用户列表'],
      ['TC-2', '权限校验', 'P1', '普通用户登录', '访问控制台', '禁止访问控制台'],
    ] },
  ] })
  view.selectView('cases')
  const template = source.slice(source.indexOf('<template>') + '<template>'.length, source.indexOf('\n</template>'))
  const compiled = compileTemplate({ source: template, filename: 'TestCasePanel.vue', id: 'case-workbench-test' })
  assert.deepEqual(compiled.errors, [])
  const exports = {}
  vm.runInNewContext(ts.transpile(compiled.code, { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }), {
    exports, require: id => { assert.equal(id, 'vue'); return Vue },
  })
  const render = async () => {
    const app = createSSRApp({ setup: () => ({ ...view.props, ...view, resolveFilePreviewExt }), render: exports.render })
    app.config.globalProperties.$t = key => key
    app.use(TDesign)
    app.component('ArtifactSaveToKnowledge', { render: () => null })
    app.component('ArtifactFileIcon', { render: () => null })
    app.component('DocumentPreview', { props: ['knowledgeId', 'fileName'], render() { return Vue.h('div', { 'data-preview-name': this.fileName, 'data-preview-knowledge': this.knowledgeId }) } })
    return renderToString(app)
  }
  const normal = await render()
  assert.equal((normal.match(/<th\b/g) || []).length, 4)
  assert.match(normal, /展示用户列表/)
  assert.doesNotMatch(normal, /禁止访问控制台|这段说明不应占据列表首屏/)
  view.selectedCaseKey.value = view.workbench.value.cases[1].key
  const selected = await render()
  assert.match(selected, /禁止访问控制台/)
  assert.doesNotMatch(selected, /展示用户列表/)
  view.selectView('notes')
  assert.match(await render(), /这段说明不应占据列表首屏/)
  view.selectArtifact('b:0:notes.md')
  const preview = await render()
  assert.match(preview, /data-preview-name="notes.md"/)
  assert.match(preview, /data-preview-knowledge="markdown-knowledge"/)
})

test('binding an already cached original file switches subsequent reads to the knowledge file', async () => {
  const view = syncFixture()
  view.setLink('')
  await view.state.parseItems()
  assert.equal(view.state.sheetsOf(view.props.items[0])[0].rows[0][1], 'original')
  view.setLink('knowledge')
  await view.state.parseItems()
  assert.equal(view.state.sheetsOf(view.props.items[0])[0].rows[0][1], 'edited in knowledge')
  assert.equal(view.originalDownloads(), 1)
})

test('losing knowledge access shows a refresh error instead of silently displaying the original snapshot', async () => {
  const view = syncFixture()
  await view.state.parseItems()
  view.failKnowledge()
  await view.state.parseItems()
  assert.equal(view.state.sheetsOf(view.props.items[0]), null)
  assert.equal(view.state.entries.get('a:3:cases.csv').status, 'sync-error')
  assert.equal(view.state.fallbackItems.value.length, 0)
  assert.equal(view.originalDownloads(), 0)
})

test('inactive testing panels do not download files', async () => {
  const view = syncFixture()
  view.props.active = false
  await view.state.parseItems()
  assert.equal(view.state.entries.size, 0)
})

test('refreshing linked knowledge retains the visible selection until new data arrives', async () => {
  const view = syncFixture()
  await view.state.parseItems()
  const selected = view.state.selectedCase.value.key
  view.state.selectedCaseKey.value = selected
  let finish
  view.setKnowledgeReader(() => new Promise(resolve => { finish = resolve }))
  const refresh = view.state.parseItems()
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(view.state.selectedCase.value.key, selected)
  assert.equal(view.state.sheetsOf(view.props.items[0])[0].rows[0][1], 'edited in knowledge')
  finish(new Blob(['ID,Steps\nTC-1,refreshed value\n']))
  await refresh
  assert.equal(view.state.selectedCase.value.key, selected)
  assert.equal(view.state.sheetsOf(view.props.items[0])[0].rows[0][1], 'refreshed value')
})

test('an open linked source preview receives a new revision when knowledge refreshes', async () => {
  const view = syncFixture()
  await view.state.parseItems()
  view.state.openSource(view.props.items[0])
  const revision = view.state.previewRevision.value
  await view.state.parseItems()
  assert.ok(view.state.previewRevision.value > revision)
})

test('a slow response from the previous session cannot overwrite the new session view', async () => {
  const view = syncFixture()
  let finishOld
  let calls = 0
  view.setKnowledgeReader(() => {
    calls++
    if (calls === 1) return new Promise(resolve => { finishOld = resolve })
    return new Blob(['ID,Steps\nTC-2,new session\n'])
  })
  const oldRefresh = view.state.parseItems()
  await new Promise(resolve => setImmediate(resolve))
  view.props.sessionId = 'session-b'
  view.props.items = [{ ...cases, messageId: 'b', file_name: 'cases.csv' }]
  view.watcher()
  await new Promise(resolve => setImmediate(resolve))
  finishOld(new Blob(['ID,Steps\nTC-1,stale response\n']))
  await oldRefresh
  assert.equal(view.state.entries.has('a:3:cases.csv'), false)
  assert.equal(view.state.sheetsOf(view.props.items[0])[0].rows[0][1], 'new session')
})

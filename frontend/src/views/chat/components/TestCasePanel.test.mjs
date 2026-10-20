import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { computed, reactive } from 'vue'
import { isSpreadsheetArtifact, parseTestCaseSheets } from '../../../utils/testCaseSheet.ts'

const source = readFileSync(new URL('./TestCasePanel.vue', import.meta.url), 'utf8')

const start = source.indexOf('const entries = reactive')
const end = source.indexOf('let parsedSessionId', start)
assert.ok(start >= 0 && end > start, 'case panel keeps its parsed-entry computeds in one block')
const computedsSource = ts.transpile(source.slice(start, end), { target: ts.ScriptTarget.ES2022 })

function setup(items) {
  const props = reactive({ sessionId: 'session-a', items, active: true })
  const state = vm.runInNewContext(
    `${computedsSource}; ({ entries, tableItems, fallbackItems, rowCount, sheetsOf })`,
    { computed, reactive, props, isSpreadsheetArtifact },
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

test('the panel lists unparsed files below the tables', () => {
  assert.match(source, /v-if="fallbackItems.length"/)
  assert.match(source, /<ChatArtifactsPanel/)
  assert.match(source, /caseSheetUnparsedFiles/)
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
  const state = vm.runInNewContext(`${runtimeSource}; ({ entries, knowledgeLinks, parseItems, sheetsOf, tableItems, fallbackItems, refreshVisibleCases })`, {
    computed, reactive, props, isSpreadsheetArtifact, parseTestCaseSheets,
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

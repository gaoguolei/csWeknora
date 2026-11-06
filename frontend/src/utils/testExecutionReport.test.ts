import assert from 'node:assert/strict'
import test from 'node:test'
import { DOMParser } from '@xmldom/xmldom'
import { parseTestExecutionReport } from './testExecutionReport.ts'
import { buildTestCaseWorkbench } from './testCaseWorkbench.ts'

test('Markdown reports expose only per-case table evidence and ignore summaries and fenced examples', async () => {
  const sheets = await parseTestExecutionReport(new Blob([
    '# 全部通过\n\n| 项目 | 结果 |\n| --- | --- |\n| 登录 | 通过 |\n\n```\n| ID | Status |\n| --- | --- |\n| TC-fake | passed |\n```\n\n| 用例编号 | 执行状态 |\n| --- | --- |\n| TC-1 | 失败 |',
  ]), 'test-report.md')
  assert.equal(sheets.length, 1)
  assert.deepEqual(sheets[0].rows, [['TC-1', '失败']])
})

test('HTML reports read tables as text without executing report scripts', async () => {
  const previous = globalThis.DOMParser
  // xmldom supplies a DOM in Node; the browser supplies its own inert HTML parser.
  globalThis.DOMParser = DOMParser as unknown as typeof globalThis.DOMParser
  try {
    const sheets = await parseTestExecutionReport(new Blob(['<html><body><script>throw new Error("never run")</script><table><tr><th>Case ID</th><th>Status</th></tr><tr><td>TC-1</td><td>passed</td></tr></table></body></html>']), 'report.html')
    assert.equal(sheets.length, 1)
    assert.deepEqual(sheets[0].rows, [['TC-1', 'passed']])
  } finally { globalThis.DOMParser = previous }
})

test('external spreadsheet sheets with arbitrary names contribute results rather than new cases', () => {
  const model = buildTestCaseWorkbench([{ name: 'Cases', header: ['ID', 'Title', 'Status'], rows: [['TC-1', 'Login', '未执行']] }], [
    { name: '说明 Summary', header: ['ID', 'Status'], rows: [['TC-1', 'passed']] },
  ])
  assert.equal(model.cases.length, 1)
  assert.equal(model.cases[0].status, 'passed')
  assert.equal(model.results.length, 1)
  assert.equal(model.results[0].title, 'Login')
})

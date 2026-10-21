import assert from 'node:assert/strict'
import test from 'node:test'
import { buildTestCaseWorkbench, findTestCaseHeaderRow, testCaseStatus } from './testCaseWorkbench.ts'

test('overview, cases and execution results are classified without inflating the case count', () => {
  const model = buildTestCaseWorkbench([
    { name: '用例说明', header: ['项目', '说明'], rows: [['被测项目', '用户管理'], ['环境', 'Chromium']] },
    { name: '测试用例', header: ['用例编号', '用例名称', '用例级别', '前置条件', '测试步骤', '预期结果'], rows: [
      ['TC-1', '显示用户列表', 'P0', '应用可访问', '1. 登录\n2. 打开列表', '显示已有用户'],
      ['TC-2', '创建用户', 'P1', '已登录', '提交新用户', '创建成功'],
    ] },
    { name: '执行结果', header: ['用例编号', '执行状态', '实际结果'], rows: [['TC-1', '通过', '已显示用户列表']] },
  ])
  assert.equal(model.notes.length, 1)
  assert.equal(model.cases.length, 2)
  assert.equal(model.results.length, 1)
  assert.equal(model.cases[0].status, 'passed')
  assert.equal(model.cases[1].status, 'unknown', 'absence of evidence does not imply not-run or passing')
  assert.equal(model.cases[0].details.find(field => field.label === '测试步骤')?.value, '1. 登录\n2. 打开列表')
  assert.equal(model.cases[0].resultDetails.find(field => field.label === '实际结果')?.value, '已显示用户列表')
})

test('header recognition skips a merged title and decorative blank rows', () => {
  assert.equal(findTestCaseHeaderRow([
    ['用户管理系统测试用例', '', ''], ['', '', ''], ['Case ID', 'Title', 'Expected result'], ['TC-1', 'Login', 'Dashboard'],
  ]), 2)
  assert.equal(findTestCaseHeaderRow([['项目', '内容'], ['测试步骤', '按规范执行']]), 0)
})

test('explicit result sheets take precedence without duplicating embedded case statuses', () => {
  const model = buildTestCaseWorkbench([
    { name: 'Cases', header: ['ID', 'Title', 'Status'], rows: [['001', 'Login', 'passed']] },
    { name: 'Results', header: ['Case ID', 'Result'], rows: [['001', 'failed']] },
  ])
  assert.equal(model.cases[0].status, 'failed')
  assert.equal(model.results.length, 1)
})

test('duplicate identifiers never apply execution evidence to an arbitrary case', () => {
  const model = buildTestCaseWorkbench([
    { name: 'Cases', header: ['ID', 'Title'], rows: [['TC-1', 'A'], ['TC-1', 'B']] },
    { name: 'Results', header: ['ID', 'Status'], rows: [['TC-1', 'passed']] },
  ])
  assert.equal(new Set(model.cases.map(item => item.key)).size, 2)
  assert.ok(model.cases.every(item => item.status === 'unknown'))
})

test('actual-result prose alone never fabricates a pass status', () => {
  const model = buildTestCaseWorkbench([{ name: 'Cases', header: ['ID', 'Title', 'Actual result'], rows: [['TC-1', 'Login', '页面已打开']] }])
  assert.equal(model.cases[0].status, 'unknown')
  assert.equal(model.results.length, 0)
  assert.equal(testCaseStatus('未通过'), 'failed')
  assert.equal(testCaseStatus('未执行'), 'not-run')
  assert.equal(testCaseStatus('blocked'), 'blocked')
  assert.equal(testCaseStatus('已跳过'), 'skipped')
  assert.equal(testCaseStatus('PASS'), 'passed')
  assert.equal(testCaseStatus('FAIL'), 'failed')
  assert.equal(testCaseStatus('待确认'), 'unknown')
})

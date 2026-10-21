import type { TestCaseSheet } from './testCaseSheet'

type CaseFieldRole = 'id' | 'title' | 'priority' | 'status' | 'steps' | 'expected' | 'actual' | 'condition' | 'module'
export type TestCaseStatus = 'passed' | 'failed' | 'blocked' | 'skipped' | 'not-run' | 'unknown'
export interface TestCaseField { label: string; value: string }
export interface WorkbenchCase {
  key: string
  id: string
  sourceId: string
  title: string
  priority: string
  status: TestCaseStatus
  statusText: string
  sheetName: string
  details: TestCaseField[]
  resultDetails: TestCaseField[]
  searchText: string
}

const aliases: Record<CaseFieldRole, string[]> = {
  id: ['id', 'caseid', 'testcaseid', 'tcid', '编号', '用例编号', '测试用例编号', '用例id', '案例编号'],
  title: ['title', 'name', 'casename', 'testcase', 'testcasename', 'scenario', 'testscenario', '名称', '用例名称', '用例标题', '测试用例', '测试场景', '场景', '测试项', '测试点', '用例描述'],
  priority: ['priority', 'level', '优先级', '级别', '用例级别', '测试级别'],
  status: ['status', 'result', 'testresult', 'executionresult', '执行状态', '用例状态', '测试状态', '执行结果', '测试结果', '结果', '结论', '执行结论', '是否通过'],
  steps: ['steps', 'teststeps', '操作步骤', '测试步骤', '步骤'],
  expected: ['expected', 'expectedresult', 'expectedresults', '预期结果', '期望结果', '预期'],
  actual: ['actual', 'actualresult', 'actualresults', '实际结果', '实际执行结果'],
  condition: ['precondition', 'preconditions', 'prerequisites', '前置条件', '预置条件', '前提条件'],
  module: ['module', 'feature', '模块', '功能模块', '测试模块', '所属模块'],
}

function normalize(value: string): string { return value.toLowerCase().replace(/[\s_\-:：()（）]/g, '') }
export function testCaseFieldRole(label: string): CaseFieldRole | undefined {
  const key = normalize(label)
  return (Object.keys(aliases) as CaseFieldRole[]).find(role => aliases[role].includes(key))
}

/** Skip decorative workbook titles, while retaining plain key/value sheet headers. */
export function findTestCaseHeaderRow(rows: string[][]): number {
  let best = -1
  let score = 1
  rows.slice(0, 20).forEach((row, index) => {
    const roles = new Set(row.map(testCaseFieldRole).filter(Boolean))
    if (roles.size > score) { best = index; score = roles.size }
  })
  return best >= 0 ? best : Math.max(0, rows.findIndex(row => row.some(cell => cell.trim())))
}

export function testCaseStatus(value: string): TestCaseStatus {
  const status = value.trim().toLowerCase().replace(/[✅❌✔✘]/g, '').trim()
  if (/^(fail(?:ed)?|failure|失败|未通过|不通过|不合格)(\b|\s|[（(:：]|$)/.test(status)) return 'failed'
  if (/^(pass(?:ed)?|success|通过|全部通过|成功)(\b|\s|[（(:：]|$)/.test(status)) return 'passed'
  if (/^(blocked|阻塞|受阻|环境阻塞)(\b|\s|[（(:：]|$)/.test(status)) return 'blocked'
  if (/^(skip(?:ped)?|跳过|已跳过)(\b|\s|[（(:：]|$)/.test(status)) return 'skipped'
  if (/^(not[ _-]?run|pending|未执行|未运行|待执行)$/.test(status)) return 'not-run'
  return 'unknown'
}

export function buildTestCaseWorkbench(sheets: TestCaseSheet[]) {
  const cases: WorkbenchCase[] = []
  const notes: TestCaseSheet[] = []
  const results: WorkbenchCase[] = []
  const embeddedResults: WorkbenchCase[] = []
  for (const sheet of sheets) {
    const roles = sheet.header.map(testCaseFieldRole)
    const isNotes = /说明|概览|简介|环境|readme|overview|notes|summary/i.test(sheet.name)
    const hasCaseFields = roles.some(role => role === 'id' || role === 'title' || role === 'steps' || role === 'expected')
    if (isNotes || (!hasCaseFields && !/用例|案例|场景|cases?|scenarios?/i.test(sheet.name))) { notes.push(sheet); continue }
    const isResults = /执行记录|结果|报告|results?|execution|reports?|runs?/i.test(sheet.name)
    const target = isResults ? results : cases
    const keys = new Map<string, number>()
    sheet.rows.forEach((row, index) => {
      if (row.every(value => !value.trim()) || row.every((value, col) => value === sheet.header[col])) return
      const field = (role: CaseFieldRole) => row[roles.indexOf(role)]?.trim() || ''
      const sourceId = field('id')
      const rawKey = `${sheet.name}:${sourceId || index}`
      const duplicate = keys.get(rawKey) || 0
      keys.set(rawKey, duplicate + 1)
      const title = field('title') || field('steps').split('\n')[0] || row.find((value, col) => value.trim() && !['id', 'priority', 'status'].includes(roles[col] || '')) || ''
      const record: WorkbenchCase = {
        key: `${rawKey}:${duplicate}`, id: sourceId || `#${index + 1}`, sourceId, title,
        priority: field('priority'), status: testCaseStatus(field('status')), statusText: field('status'), sheetName: sheet.name,
        details: row.flatMap((value, col) => value && !['id', 'title', 'priority'].includes(roles[col] || '')
          ? [{ label: sheet.header[col] || `#${col + 1}`, value }] : []),
        resultDetails: [], searchText: row.join(' ').toLowerCase(),
      }
      target.push(record)
      if (!isResults && record.statusText) embeddedResults.push({ ...record, key: `result:${record.key}` })
    })
  }
  const explicitResultIds = new Set(results.map(item => item.sourceId).filter(Boolean))
  results.push(...embeddedResults.filter(item => !item.sourceId || !explicitResultIds.has(item.sourceId)))
  const idCounts = new Map<string, number>()
  cases.forEach(item => { if (item.sourceId) idCounts.set(item.sourceId, (idCounts.get(item.sourceId) || 0) + 1) })
  // Only explicit, unique case identifiers can associate execution evidence.
  // A missing result never implies that a case passed or was not executed.
  for (const result of results) {
    if (!result.sourceId || idCounts.get(result.sourceId) !== 1) continue
    const item = cases.find(item => item.sourceId === result.sourceId)!
    item.status = result.status
    item.statusText = result.statusText
    if (item.sheetName !== result.sheetName) item.resultDetails = result.details
  }
  return { cases, notes, results }
}

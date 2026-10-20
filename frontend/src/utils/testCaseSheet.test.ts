import assert from 'node:assert/strict'
import test from 'node:test'
import * as XLSX from 'xlsx'
import { isSpreadsheetArtifact, parseTestCaseSheets, spreadsheetExt } from './testCaseSheet.ts'

function workbookBlob(rows: string[][], sheetName = 'Sheet1'): Blob {
  const book = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(book, XLSX.utils.aoa_to_sheet(rows), sheetName)
  return new Blob([XLSX.write(book, { type: 'buffer', bookType: 'xlsx' })])
}

test('spreadsheet detection covers excel and delimited case exports only', () => {
  assert.equal(isSpreadsheetArtifact('登录测试用例.xlsx'), true)
  assert.equal(isSpreadsheetArtifact('cases.CSV'), true)
  assert.equal(isSpreadsheetArtifact('cases.tsv'), true)
  assert.equal(isSpreadsheetArtifact('cases.md'), false)
  assert.equal(isSpreadsheetArtifact('cases.docx'), false)
  assert.equal(spreadsheetExt('没有后缀'), '')
})

test('parses an exported case workbook into a header row and its cases', async () => {
  const blob = workbookBlob([
    ['用例编号', '模块', '优先级'],
    ['TC-1', '登录', '高'],
    ['TC-2', '登录', '低'],
    ['', '', ''],
  ])
  const sheets = await parseTestCaseSheets(blob, '登录测试用例.xlsx')
  assert.equal(sheets.length, 1)
  assert.deepEqual(sheets[0].header, ['用例编号', '模块', '优先级'])
  assert.deepEqual(sheets[0].rows, [
    ['TC-1', '登录', '高'],
    ['TC-2', '登录', '低'],
  ])
})

test('parses csv case exports with the same header and rows shape', async () => {
  const blob = new Blob(['用例编号,模块\nTC-1,登录\n'], { type: 'text/csv' })
  const sheets = await parseTestCaseSheets(blob, 'cases.csv')
  assert.deepEqual(sheets[0].header, ['用例编号', '模块'])
  assert.deepEqual(sheets[0].rows, [['TC-1', '登录']])
})

test('a sheet with no case rows is rejected so the panel can show the file instead', async () => {
  await assert.rejects(() => parseTestCaseSheets(workbookBlob([['用例编号', '模块']]), '空用例.xlsx'))
})

test('non-spreadsheet case documents are rejected without parsing', async () => {
  await assert.rejects(() => parseTestCaseSheets(new Blob(['# 用例']), 'cases.md'))
})
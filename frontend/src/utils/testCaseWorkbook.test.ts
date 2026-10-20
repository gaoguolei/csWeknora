import assert from 'node:assert/strict'
import test from 'node:test'
import * as XLSX from 'xlsx'
import { readCaseWorkbook, writeCaseWorkbook } from './testCaseWorkbook.ts'

test('editing a case preserves other sheets, numeric cells, formulas and whitespace', async () => {
  const book = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(book, XLSX.utils.aoa_to_sheet([
    ['ID', 'Steps', 'Count', 'Formula'], ['TC-1', '  first\nsecond  ', 12, { t: 'n', v: 24, f: 'C2*2' }],
  ]), 'Cases')
  XLSX.utils.book_append_sheet(book, XLSX.utils.aoa_to_sheet([['Notes'], ['keep me']]), 'Notes')
  const blob = new Blob([XLSX.write(book, { type: 'array', bookType: 'xlsx' })])
  const loaded = await readCaseWorkbook(blob, 'cases.xlsx')
  assert.equal(loaded.sheets[0].cells[1][1], '  first\nsecond  ')
  const file = await writeCaseWorkbook(loaded.workbook, [
    { sheet: 'Cases', row: 1, col: 1, value: 'updated\nsteps' },
    { sheet: 'Cases', row: 2, col: 0, value: 'TC-2' },
  ], 'cases.xlsx')
  const saved = await readCaseWorkbook(file, file.name)
  assert.deepEqual(saved.workbook.SheetNames, ['Cases', 'Notes'])
  assert.equal(saved.workbook.Sheets.Cases.B2.v, 'updated\nsteps')
  assert.equal(saved.workbook.Sheets.Cases.C2.t, 'n')
  assert.equal(saved.workbook.Sheets.Cases.C2.v, 12)
  assert.equal(saved.workbook.Sheets.Cases.D2.f, 'C2*2')
  assert.equal(saved.workbook.Sheets.Notes.A2.v, 'keep me')
  assert.equal(saved.workbook.Sheets.Cases.A3.v, 'TC-2')
  assert.equal(loaded.workbook.Sheets.Cases.B2.v, '  first\nsecond  ')
})

test('a used range away from A1 keeps its actual cell coordinates', async () => {
  const book = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(book, {
    '!ref': 'C4:D5', C4: { t: 's', v: 'ID' }, D4: { t: 's', v: 'Steps' },
    C5: { t: 's', v: 'TC-1' }, D5: { t: 's', v: 'original' },
  }, 'Cases')
  const loaded = await readCaseWorkbook(new Blob([XLSX.write(book, { type: 'array', bookType: 'xlsx' })]), 'cases.xlsx')
  assert.equal(loaded.sheets[0].startRow, 3)
  assert.equal(loaded.sheets[0].startCol, 2)
  assert.deepEqual(loaded.sheets[0].cells, [['ID', 'Steps'], ['TC-1', 'original']])
})

test('CSV and TSV retain quoted newlines, separators and literal text after editing', async () => {
  for (const [name, delimiter] of [['cases.csv', ','], ['cases.tsv', '\t']]) {
    const loaded = await readCaseWorkbook(new Blob([`ID${delimiter}Steps\n001${delimiter}"first\nsecond"\n`]), name)
    assert.equal(loaded.sheets[0].cells[1][0], '001')
    const value = '=literal\nwith,comma\tand tab'
    const file = await writeCaseWorkbook(loaded.workbook, [{ sheet: loaded.sheets[0].name, row: 1, col: 1, value }], name)
    const saved = await readCaseWorkbook(file, name)
    assert.equal(saved.sheets[0].cells[1][0], '001')
    assert.equal(saved.sheets[0].cells[1][1], value)
  }
})

test('explicitly replacing a formula cell removes its old formula and cached text', async () => {
  const book = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(book, { '!ref': 'A1:A2', A1: { t: 's', v: 'Count' }, A2: { t: 'n', v: 2, f: '1+1', w: '2' } }, 'Cases')
  const file = await writeCaseWorkbook(book, [{ sheet: 'Cases', row: 1, col: 0, value: 'changed' }], 'cases.xlsx')
  const saved = await readCaseWorkbook(file, file.name)
  assert.equal(saved.workbook.Sheets.Cases.A2.f, undefined)
  assert.equal(saved.workbook.Sheets.Cases.A2.v, 'changed')
})

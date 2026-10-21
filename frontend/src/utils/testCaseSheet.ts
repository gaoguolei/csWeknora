import { decodeDelimitedText } from './filePreview'
import { findTestCaseHeaderRow } from './testCaseWorkbench'

/** One worksheet flattened to a header row plus its case rows. */
export interface TestCaseSheet {
  name: string
  header: string[]
  rows: string[][]
}

const SPREADSHEET_EXTS = new Set(['xlsx', 'xls', 'xlsm', 'ods', 'csv', 'tsv'])

export function spreadsheetExt(fileName: string): string {
  const match = /\.([A-Za-z0-9]+)$/.exec(fileName || '')
  return match ? match[1].toLowerCase() : ''
}

/** Whether the file is a spreadsheet the case table can read in the browser. */
export function isSpreadsheetArtifact(fileName: string): boolean {
  return SPREADSHEET_EXTS.has(spreadsheetExt(fileName))
}

function isBlankRow(row: string[]): boolean {
  return !row.some((cell) => cell.trim() !== '')
}

/**
 * Parse an exported case workbook into sheets of header + rows. Throws when the
 * file is not a spreadsheet or holds no data rows, which is the signal for
 * callers to fall back to the plain file view.
 */
export async function parseTestCaseSheets(blob: Blob, fileName: string): Promise<TestCaseSheet[]> {
  const ext = spreadsheetExt(fileName)
  if (!SPREADSHEET_EXTS.has(ext)) throw new Error(`unsupported spreadsheet: ${ext || fileName}`)
  const XLSX = await import('xlsx')
  const buffer = await blob.arrayBuffer()
  const workbook =
    ext === 'csv' || ext === 'tsv'
      ? XLSX.read(decodeDelimitedText(buffer), {
          type: 'string',
          raw: true,
          FS: ext === 'tsv' ? '\t' : ',',
        })
      : XLSX.read(buffer, { type: 'array' })

  const sheets: TestCaseSheet[] = []
  for (const name of workbook.SheetNames) {
    const matrix = XLSX.utils.sheet_to_json<unknown[]>(workbook.Sheets[name], {
      header: 1,
      raw: false,
      defval: '',
    })
    const rows = matrix.map((cells) =>
      (Array.isArray(cells) ? cells : []).map((cell) => String(cell ?? '').trim()),
    )
    while (rows.length && isBlankRow(rows[0])) rows.shift()
    while (rows.length && isBlankRow(rows[rows.length - 1])) rows.pop()
    // 只有表头（或空表）没有可展示的用例，交给文件视图。
    if (rows.length < 2) continue
    const header = findTestCaseHeaderRow(rows)
    if (header >= rows.length - 1) continue
    sheets.push({
      name,
      header: rows[header],
      rows: rows.slice(header + 1).filter((row) => !isBlankRow(row)),
    })
  }
  if (!sheets.length) throw new Error('no test case rows')
  return sheets
}

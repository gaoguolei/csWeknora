import type { WorkBook, BookType } from 'xlsx'
import { decodeDelimitedText } from './filePreview'
import { isSpreadsheetArtifact, spreadsheetExt } from './testCaseSheet'

export interface EditableCaseSheet {
  name: string
  startRow: number
  startCol: number
  cells: string[][]
}

export interface CaseCellEdit {
  sheet: string
  row: number
  col: number
  value: string
}

export async function readCaseWorkbook(blob: Blob, fileName: string) {
  if (!isSpreadsheetArtifact(fileName)) throw new Error('unsupported spreadsheet')
  const XLSX = await import('xlsx')
  const ext = spreadsheetExt(fileName)
  const buffer = await blob.arrayBuffer()
  const workbook = ext === 'csv' || ext === 'tsv'
    ? XLSX.read(decodeDelimitedText(buffer), { type: 'string', FS: ext === 'tsv' ? '\t' : ',', raw: true })
    : XLSX.read(buffer, { type: 'array', cellStyles: true, bookVBA: true })
  const sheets: EditableCaseSheet[] = workbook.SheetNames.map((name) => {
    const sheet = workbook.Sheets[name]
    const range = XLSX.utils.decode_range(sheet['!ref'] || 'A1')
    const cells = XLSX.utils.sheet_to_json<string[]>(sheet, { header: 1, raw: false, defval: '', blankrows: true })
    const rows = cells.map((row) =>
      Array.from({ length: range.e.c - range.s.c + 1 }, (_, col) => String(row[col] ?? '')),
    )
    return { name, startRow: range.s.r, startCol: range.s.c, cells: rows.length ? rows : [['']] }
  })
  return { workbook, sheets }
}

/** Patch only edited cells so untouched numbers, formulas and other sheets survive. */
export async function writeCaseWorkbook(workbook: WorkBook, edits: CaseCellEdit[], fileName: string): Promise<File> {
  const XLSX = await import('xlsx')
  const output: WorkBook = { ...workbook, Sheets: { ...workbook.Sheets } }
  const copied = new Set<string>()
  for (const edit of edits) {
    if (!output.Sheets[edit.sheet]) throw new Error('unknown worksheet')
    if (!copied.has(edit.sheet)) {
      output.Sheets[edit.sheet] = { ...output.Sheets[edit.sheet] }
      copied.add(edit.sheet)
    }
    const sheet = output.Sheets[edit.sheet]
    const address = XLSX.utils.encode_cell({ r: edit.row, c: edit.col })
    const cell = { ...sheet[address], t: 's' as const, v: edit.value }
    delete cell.f
    delete cell.F
    delete cell.w
    delete cell.h
    delete cell.r
    sheet[address] = cell
    const range = XLSX.utils.decode_range(sheet['!ref'] || address)
    range.s.r = Math.min(range.s.r, edit.row)
    range.s.c = Math.min(range.s.c, edit.col)
    range.e.r = Math.max(range.e.r, edit.row)
    range.e.c = Math.max(range.e.c, edit.col)
    sheet['!ref'] = XLSX.utils.encode_range(range)
  }
  const ext = spreadsheetExt(fileName)
  const bookType = (ext === 'xls' ? 'biff8' : ext === 'tsv' ? 'csv' : ext) as BookType
  const data = XLSX.write(output, { type: 'array', bookType, cellStyles: true,
    ...(ext === 'tsv' ? { FS: '\t' } : {}) })
  return new File([data], fileName, { type: 'application/octet-stream' })
}

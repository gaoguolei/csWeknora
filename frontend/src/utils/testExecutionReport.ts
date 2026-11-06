import { isSpreadsheetArtifact, parseTestCaseSheets, type TestCaseSheet } from './testCaseSheet'
import { findTestCaseHeaderRow, testCaseFieldRole } from './testCaseWorkbench'

export function isTabularExecutionReport(fileName: string): boolean {
  return isSpreadsheetArtifact(fileName) || /\.(html?|md)$/i.test(fileName)
}

/** Only explicit tabular, per-case evidence can update the case workbench. */
export async function parseTestExecutionReport(blob: Blob, fileName: string): Promise<TestCaseSheet[]> {
  if (isSpreadsheetArtifact(fileName)) return parseTestCaseSheets(blob, fileName)
  const text = await blob.text()
  const tables: string[][][] = []
  if (/\.html?$/i.test(fileName)) {
    const doc = new DOMParser().parseFromString(text, 'text/html')
    for (const table of Array.from(doc.getElementsByTagName('table'))) {
      tables.push(Array.from(table.getElementsByTagName('tr')).map(row =>
        Array.from(row.childNodes).filter(cell => cell.nodeType === 1 && /^(TH|TD)$/i.test((cell as Element).tagName)).map(cell => cell.textContent?.trim() || ''),
      ))
    }
  } else if (/\.md$/i.test(fileName)) {
    let rows: string[][] = []
    let inFence = false
    for (const line of text.split(/\r?\n/)) {
      if (/^\s*(```|~~~)/.test(line)) { inFence = !inFence; continue }
      if (!inFence && line.includes('|')) {
        const cells = line.trim().replace(/^\|/, '').replace(/\|$/, '').split(/(?<!\\)\|/).map(cell => cell.trim().replace(/\\\|/g, '|').replace(/\*\*|`/g, ''))
        if (!cells.every(cell => /^:?-+:?$/.test(cell))) rows.push(cells)
      } else if (rows.length) { tables.push(rows); rows = [] }
    }
    if (rows.length) tables.push(rows)
  }
  return tables.flatMap((rows, index) => {
    const header = findTestCaseHeaderRow(rows)
    const roles = (rows[header] || []).map(testCaseFieldRole)
    return roles.includes('id') && roles.includes('status')
      ? [{ name: `${fileName} #${index + 1}`, header: rows[header], rows: rows.slice(header + 1) }]
      : []
  })
}

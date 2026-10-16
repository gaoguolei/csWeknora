/**
 * Unified-diff helpers.
 *
 * The agent reports the before/after text of every write and edit, and the
 * workspace code view renders the result inline the way an editor does. jsdiff
 * already produces the hunks with context, so this module only normalises them
 * into typed lines with old/new gutter numbers.
 */
import { structuredPatch } from 'diff'

export interface DiffLine {
  type: 'add' | 'del' | 'context'
  content: string
  oldNumber: number | null
  newNumber: number | null
}

export interface DiffHunk {
  header: string
  lines: DiffLine[]
}

export interface UnifiedDiff {
  hunks: DiffHunk[]
  added: number
  removed: number
  /** True when the two sides are identical (nothing to show). */
  empty: boolean
}

/** Lines of unchanged context kept around each change. */
export const DIFF_CONTEXT_LINES = 3

function hunkHeader(hunk: {
  oldStart: number
  oldLines: number
  newStart: number
  newLines: number
}): string {
  return `@@ -${hunk.oldStart},${hunk.oldLines} +${hunk.newStart},${hunk.newLines} @@`
}

/**
 * Build the unified diff between two full-file strings.
 *
 * Identical sides produce `empty: true`; callers fall back to the plain source
 * view rather than rendering an empty diff.
 */
export function buildUnifiedDiff(
  before: string,
  after: string,
  context: number = DIFF_CONTEXT_LINES,
): UnifiedDiff {
  if (before === after) return { hunks: [], added: 0, removed: 0, empty: true }

  const patch = structuredPatch('a', 'b', before, after, '', '', { context })
  let added = 0
  let removed = 0
  const hunks: DiffHunk[] = patch.hunks.map((hunk) => {
    let oldLine = hunk.oldStart
    let newLine = hunk.newStart
    const lines: DiffLine[] = []
    for (const raw of hunk.lines) {
      // "\ No newline at end of file" is a marker, not a line of the file.
      if (raw.startsWith('\\')) continue
      const marker = raw.charAt(0)
      const content = raw.slice(1)
      if (marker === '+') {
        lines.push({ type: 'add', content, oldNumber: null, newNumber: newLine++ })
        added++
      } else if (marker === '-') {
        lines.push({ type: 'del', content, oldNumber: oldLine++, newNumber: null })
        removed++
      } else {
        lines.push({ type: 'context', content, oldNumber: oldLine++, newNumber: newLine++ })
      }
    }
    return { header: hunkHeader(hunk), lines }
  })

  return { hunks, added, removed, empty: hunks.length === 0 }
}

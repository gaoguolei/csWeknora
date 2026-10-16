import { buildShellExecView } from './shellExecResult'
import { canPreviewSource } from './workspacePreview'
import { workspaceCheckKind } from './workspaceChecks'

export type WorkspaceIntent = 'chat' | 'build' | 'code' | 'test' | 'knowledge'
export type WorkspaceTab = 'source' | 'preview' | 'tests'

export interface WorkspaceSignal {
  tab: WorkspaceTab
  path: string
  refresh: boolean
  tool: string
}

export interface WorkspacePanelDecision {
  tab: WorkspaceTab
  path: string
}

export function workspaceIntent(value: unknown): WorkspaceIntent {
  return typeof value === 'string' && ['build', 'code', 'test', 'knowledge'].includes(value) ? value as WorkspaceIntent : 'chat'
}

export function intentPanelTab(intent: WorkspaceIntent): WorkspaceTab | null {
  return intent === 'build' ? 'preview' : intent === 'code' ? 'source' : intent === 'test' ? 'tests' : null
}

const fileTools = new Set(['write_sandbox_file', 'edit_sandbox_file', 'read_sandbox_file', 'list_sandbox_files'])
const commandTools = new Set(['shell_exec', 'execute_code'])
/** Writes and edits name the artifact a finished turn should show. */
const artifactTools = new Set(['write_sandbox_file', 'edit_sandbox_file'])

/** Writes and edits are the only tools that produce a file diff. */
const mutationTools = new Set(['write_sandbox_file', 'edit_sandbox_file'])

export interface WorkspaceFileChange {
  turnId?: string
  path: string
  type: 'added' | 'modified' | 'deleted'
  /** Pre-write text; empty string for a brand-new file, undefined when the
   *  backend omitted the body because the file was too large to ship. */
  before?: string
  after?: string
  addedLines: number
  removedLines: number
  /** Set by the live stream / change store; merged git rows carry one too. */
  id?: string
  timestamp?: number
  /** True for a change from the live stream, false for history replay. */
  live?: boolean
}

function toCount(value: unknown): number {
  const n = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0
}

/**
 * Every object level a tool result can hide behind: live SSE flattens the
 * result onto `data`, history stores it under `tool_data`, and some shapes wrap
 * it one more time in `data`. Follow the chain so the caller reads the level
 * that actually carries `path` / `diff_*` instead of assuming one shape.
 */
function toolResultLevels(root: any): Array<Record<string, any>> {
  const levels: Array<Record<string, any>> = []
  const seen = new Set<any>()
  let current: any = root
  while (
    current &&
    typeof current === 'object' &&
    !seen.has(current) &&
    levels.length < 6
  ) {
    seen.add(current)
    levels.push(current)
    current = current.tool_data ?? current.data
  }
  return levels
}

/**
 * The file mutation a `tool_result` chunk reports, or null.
 *
 * `before`/`after` are the full-file sides the backend ships for the code view
 * diff; a file past the payload cap reports only the +/- line counts, and the
 * caller still uses the path to move the reader to the file.
 */
export function workspaceFileChange(
  chunk: { response_type?: string; data?: Record<string, any> },
): WorkspaceFileChange | null {
  if (chunk.response_type !== 'tool_result') return null
  const levels = toolResultLevels(chunk.data || {})
  if (levels.some(level => level.success === false)) return null
  const name = String(
    levels.map((level) => level.tool_name).find((value) => typeof value === 'string' && value) || '',
  )
  if (!mutationTools.has(name)) return null
  const result = levels.find((level) => level.path || level.file_path)
  if (!result) return null
  const path = String(result.path || result.file_path || '')
  if (!path) return null
  // Path and diff bodies may be on different wrapper levels. In particular,
  // never substitute the current file for a missing pre-edit snapshot.
  const before = levels.find(level => typeof level.diff_before === 'string')?.diff_before
  const after = levels.find(level => typeof level.diff_after === 'string')?.diff_after
  return {
    path,
    type: name === 'write_sandbox_file' && before === '' ? 'added' : 'modified',
    before,
    after,
    addedLines: toCount(levels.find(level => level.added_lines !== undefined)?.added_lines),
    removedLines: toCount(levels.find(level => level.removed_lines !== undefined)?.removed_lines),
  }
}

/**
 * Every file mutation a loaded conversation recorded, oldest first.
 *
 * Reloading a session replays the persisted tool events, so the diff a turn
 * produced is still there to show after a refresh.
 */
export function collectWorkspaceFileChanges(
  messages: Array<Record<string, any>>,
): WorkspaceFileChange[] {
  const out: WorkspaceFileChange[] = []
  for (const message of messages) {
    for (const event of message.agentEventStream || []) {
      if (event.type !== 'tool_call' || event.pending || event.success === false) continue
      const change = workspaceFileChange({
        response_type: 'tool_result',
        data: { ...(event.tool_data || {}), tool_name: event.tool_name },
      })
      if (change) out.push({ id: '', timestamp: 0, live: false, ...change, turnId: String(message.request_id || message.id || '') })
    }
  }
  return out
}

/** One dirty file from the programming git-status endpoint. */
export interface WorkspaceGitStatusFile {
  path: string
  old_path?: string
  status: string
  added: number
  removed: number
}

function normalizeChangePath(path: string): string {
  const normalized = path.replaceAll('\\', '/')
  return normalized.startsWith('/') ? normalized.slice(1) : normalized
}

function changeMatchesGitPath(changePath: string, gitPath: string, exact: boolean): boolean {
  const change = normalizeChangePath(changePath)
  const target = normalizeChangePath(gitPath)
  if (change === target) return true
  return !exact && (change.endsWith(`/${target}`) || target.endsWith(`/${change}`))
}

/**
 * Pair each recorded change with a git row. Exact paths are claimed first, so
 * a loose suffix match can never take the row that a sibling change names
 * exactly: with both `a.ts` and `src/a.ts` dirty, the log entry for `a.ts`
 * must not be handed `src/a.ts` just because it appears earlier in the list.
 */
function pairChangesWithGitRows(
  changes: WorkspaceFileChange[],
  gitFiles: WorkspaceGitStatusFile[],
): Map<number, number> {
  const paired = new Map<number, number>()
  const taken = new Set<number>()
  for (const exact of [true, false]) {
    changes.forEach((change, changeIndex) => {
      if (paired.has(changeIndex)) return
      const gitIndex = gitFiles.findIndex((gitFile, i) =>
        !taken.has(i) && changeMatchesGitPath(change.path, gitFile.path, exact))
      if (gitIndex < 0) return
      paired.set(changeIndex, gitIndex)
      taken.add(gitIndex)
    })
  }
  return paired
}

/**
 * Merge the session's recorded edits with the workspace's uncommitted changes.
 *
 * The conversation log answers "what did the agent write"; git answers "what
 * is dirty right now". A commit leaves the first untouched but empties the
 * second, so the panel is driven by git and the log only supplies the
 * before/after bodies that let a row show an inline diff.
 *
 * Status/line counts follow git; a recorded entry keeps its before/after and
 * id. Git rows the log never saw (a manual edit in the terminal) get a row
 * without a body — the diff panel falls back to its "open the source" notice.
 */
export function mergeWorkspaceFileChangesWithGitStatus(
  changes: WorkspaceFileChange[],
  gitFiles: WorkspaceGitStatusFile[],
): WorkspaceFileChange[] {
  const pairs = pairChangesWithGitRows(changes, gitFiles)
  const matched = new Set<number>(pairs.values())
  const merged: WorkspaceFileChange[] = []
  changes.forEach((change, changeIndex) => {
    const index = pairs.get(changeIndex)
    if (index === undefined) return
    const gitFile = gitFiles[index]
    merged.push({
      ...change,
      path: gitFile.path,
      type: (gitFile.status === 'deleted' ? 'deleted'
        : gitFile.status === 'added' || gitFile.status === 'untracked' ? 'added'
        : 'modified') as WorkspaceFileChange['type'],
      addedLines: gitFile.added,
      removedLines: gitFile.removed,
    })
  })
  for (const [i, gitFile] of gitFiles.entries()) {
    if (matched.has(i)) continue
    merged.push({
      // A git-only row still needs the store's identity fields to render.
      id: `git:${gitFile.path}`,
      timestamp: 0,
      live: false,
      path: gitFile.path,
      type: (gitFile.status === 'deleted' ? 'deleted'
        : gitFile.status === 'added' || gitFile.status === 'untracked' ? 'added'
        : 'modified') as WorkspaceFileChange['type'],
      addedLines: gitFile.added,
      removedLines: gitFile.removed,
    })
  }
  return merged
}

/** Only real workspace tools drive the panel; ordinary answers/code snippets do not. */
export function workspaceToolSignal(chunk: { response_type?: string; data?: Record<string, any> }): WorkspaceSignal | null {
  if (chunk.response_type !== 'tool_call' && chunk.response_type !== 'tool_result') return null
  const data = chunk.data || {}
  const name = String(data.tool_name || '')
  let args = data.arguments || {}
  if (typeof args === 'string') {
    try { args = JSON.parse(args) } catch { args = {} }
  }
  if (name === 'workspace_tests') return { tab: 'tests', path: '', refresh: chunk.response_type === 'tool_result' && args.action === 'run', tool: name }
  if (!fileTools.has(name) && !commandTools.has(name)) return null
  const result = data.tool_data || data.data || data
  const command = String(args.command || args.code || result.command || '')
  const isTest = commandTools.has(name) && workspaceCheckKind(command) === 'tests'
  const rawPath = String(result.path || result.file_path || args.path || args.file_path || '')
  // A listing names a directory; the tree can only open files.
  const path = name === 'list_sandbox_files' ? '' : rawPath
  return {
    tab: (isTest ? 'tests' : /\.(?:html?|svg)$/i.test(path) ? 'preview' : 'source') as WorkspaceTab,
    path,
    refresh: chunk.response_type === 'tool_result',
    tool: name,
  }
}

/**
 * A finished turn opens the panel on its own result. Artifacts beat plain
 * reads. Writes keep the code view on its changes, including HTML/Markdown;
 * preview is a manual choice after an edit. A turn that only runs tests can
 * show the test results.
 */
export function completedWorkspacePanel(signals: WorkspaceSignal[]): WorkspacePanelDecision | null {
  const written = signals.filter(signal => signal.path && artifactTools.has(signal.tool))
  if (written.length) return { tab: 'source', path: written[written.length - 1].path }
  const files = signals.filter(signal => signal.path)
  const previewable = [...files].reverse().find(signal => canPreviewSource(signal.path))
  if (previewable) return { tab: 'preview', path: previewable.path }
  if (signals.some(signal => signal.tab === 'tests')) return { tab: 'tests', path: '' }
  if (files.length) return { tab: 'source', path: files[files.length - 1].path }
  return signals.length ? { tab: 'source', path: '' } : null
}

export interface WorkspaceCommandRun {
  id: string
  command: string
  output: string
  status: 'running' | 'passed' | 'failed' | 'unknown'
  durationMs?: number
  exitCode?: number | null
}

/** Shared renderer consumes existing SSE/hydrated tool events, including failures. */
export function collectWorkspaceRuns(messages: Array<Record<string, any>>): WorkspaceCommandRun[] {
  return messages.flatMap(message => (message.agentEventStream || [])
    .filter((event: any) => event.type === 'tool_call' && commandTools.has(event.tool_name))
    .map((event: any, index: number): WorkspaceCommandRun => {
      const data = event.tool_data?.tool_data || event.tool_data?.data || event.tool_data || {}
      const view = buildShellExecView(data, event.arguments, event.output || event.error)
      return {
        id: `${message.id}:${event.tool_call_id || event.id || index}`,
        command: view.command || String(event.arguments?.code || event.tool_name),
        output: [view.stdout, view.stderr].filter(Boolean).join('\n'),
        status: event.pending ? 'running'
          : view.killed || (view.exitCode != null && view.exitCode !== 0) || event.success === false ? 'failed'
          : view.exitCode === 0 || event.success === true ? 'passed' : 'unknown',
        durationMs: view.durationMs ?? event.duration_ms ?? event.duration,
        exitCode: view.exitCode,
      }
    })).slice(-20).reverse()
}

/** Restored conversations open preview while retaining the latest workspace path. */
export function lastWorkspaceSignal(messages: Array<Record<string, any>>): WorkspacePanelDecision | null {
  for (const message of [...messages].reverse()) {
    const signals: WorkspaceSignal[] = []
    for (const event of message.agentEventStream || []) {
      if (event.type !== 'tool_call') continue
      const signal = workspaceToolSignal({ response_type: 'tool_result', data: { ...event.tool_data, tool_name: event.tool_name, arguments: event.arguments } })
      if (signal) signals.push(signal)
    }
    const decision = completedWorkspacePanel(signals)
    if (decision) return { ...decision, tab: 'preview' }
  }
  return null
}

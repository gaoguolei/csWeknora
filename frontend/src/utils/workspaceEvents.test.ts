import test from 'node:test'
import assert from 'node:assert/strict'
import { collectWorkspaceFileChanges, collectWorkspaceRuns, completedWorkspacePanel, intentPanelTab, mergeWorkspaceFileChangesWithGitStatus, workspaceFileChange, workspaceIntent, workspaceToolSignal, lastWorkspaceSignal } from './workspaceEvents'
import { resolveWorkspaceAsset, canPreviewSource, PREVIEW_CSP } from './workspacePreview'

test('intent selects the right workspace surface without treating knowledge as coding', () => {
  assert.equal(intentPanelTab(workspaceIntent('build')), 'preview')
  assert.equal(intentPanelTab(workspaceIntent('test')), 'tests')
  assert.equal(intentPanelTab(workspaceIntent('code')), 'source')
  assert.equal(intentPanelTab(workspaceIntent('knowledge')), null)
  assert.equal(workspaceIntent(['build']), 'chat')
  assert.equal(workspaceIntent('unknown'), 'chat')
})

test('only actual workspace tools open the panel and completed tools trigger refresh', () => {
  assert.equal(workspaceToolSignal({ response_type: 'answer', data: { tool_name: 'shell_exec' } }), null)
  assert.equal(workspaceToolSignal({ response_type: 'tool_call', data: { tool_name: 'knowledge_search' } }), null)
  assert.deepEqual(workspaceToolSignal({ response_type: 'tool_call', data: { tool_name: 'write_sandbox_file', arguments: '{"path":"index.html"}' } }), { tab: 'preview', path: 'index.html', refresh: false, tool: 'write_sandbox_file' })
  assert.deepEqual(workspaceToolSignal({ response_type: 'tool_result', data: { tool_name: 'edit_sandbox_file', tool_data: { path: 'src/main.ts' } } }), { tab: 'source', path: 'src/main.ts', refresh: true, tool: 'edit_sandbox_file' })
  assert.equal(workspaceToolSignal({ response_type: 'tool_call', data: { tool_name: 'shell_exec', arguments: { command: 'npm run test' } } })?.tab, 'tests')
  assert.equal(workspaceToolSignal({ response_type: 'tool_call', data: { tool_name: 'shell_exec', arguments: '{incomplete' } })?.tab, 'source')
  // A listing names a directory: it refreshes the tree but never focuses a row.
  assert.equal(workspaceToolSignal({ response_type: 'tool_result', data: { tool_name: 'list_sandbox_files', arguments: '{"path":"todolist"}' } })?.path, '')
})

test('a finished turn opens the panel on its own result', () => {
  const signal = (tool: string, path = '', tab: 'source' | 'preview' | 'tests' = 'source') => ({ tool, path, tab, refresh: true })
  assert.equal(completedWorkspacePanel([]), null)
  assert.deepEqual(completedWorkspacePanel([signal('shell_exec', '', 'tests')]), { tab: 'tests', path: '' })
  assert.deepEqual(completedWorkspacePanel([signal('shell_exec')]), { tab: 'source', path: '' })
  // Writes stay on Changes even for previewable files and later validation.
  assert.deepEqual(completedWorkspacePanel([signal('write_sandbox_file', 'index.html', 'preview'), signal('shell_exec', '', 'tests')]), { tab: 'source', path: 'index.html' })
  assert.deepEqual(completedWorkspacePanel([signal('edit_sandbox_file', 'src/main.ts')]), { tab: 'source', path: 'src/main.ts' })
  assert.deepEqual(completedWorkspacePanel([signal('write_sandbox_file', 'pages/about.html')]), { tab: 'source', path: 'pages/about.html' })
  assert.deepEqual(completedWorkspacePanel([signal('write_sandbox_file', 'notes.md')]), { tab: 'source', path: 'notes.md' })
  // Background verification must not replace the code the user is reviewing.
  assert.deepEqual(completedWorkspacePanel([signal('edit_sandbox_file', 'src/main.ts'), signal('shell_exec', '', 'tests')]), { tab: 'source', path: 'src/main.ts' })
  // An artifact outranks a file the same turn only read, previewable or not.
  assert.deepEqual(completedWorkspacePanel([signal('write_sandbox_file', 'app.ts'), signal('read_sandbox_file', 'README.md')]), { tab: 'source', path: 'app.ts' })
})

test('execution results preserve failures, running state, and output from the conversation', () => {
  const result = collectWorkspaceRuns([{ id: 'm1', agentEventStream: [
    { type: 'tool_call', tool_call_id: 'a', tool_name: 'shell_exec', arguments: { command: 'npm test' }, pending: false, success: false, output: 'FAIL: expected 1', duration_ms: 45 },
    { type: 'tool_call', tool_call_id: 'b', tool_name: 'shell_exec', arguments: { command: 'go test ./...' }, pending: true },
    { type: 'thinking', content: 'run a test' },
  ] }])
  assert.equal(result.length, 2)
  assert.equal(result[0].status, 'running')
  assert.equal(result[1].status, 'failed')
  assert.equal(result[1].output, 'FAIL: expected 1')
  assert.equal(result[1].durationMs, 45)
})

test('asset resolution stays within the workspace and rejects remote URLs and traversal', () => {
  assert.equal(resolveWorkspaceAsset('pages/index.html', '../style.css?v=2'), 'style.css')
  assert.equal(resolveWorkspaceAsset('pages/index.html', '/src/app.js'), 'src/app.js')
  assert.equal(resolveWorkspaceAsset('index.html', './style.css'), 'style.css')
  for (const input of ['../../secrets', '%2e%2e/secrets', 'https://example.com/a.js', '//example.com/a.js', 'data:text/html,test', 'javascript:alert(1)', 'file:///tmp/a', '..\\secret', '%00hidden', '%broken']) {
    assert.equal(resolveWorkspaceAsset('index.html', input), null, input)
  }
})

test('historical coding conversations restore their latest workspace file', () => {
  const messages = [{ id: 'a', agentEventStream: [{ type: 'tool_call', tool_name: 'write_sandbox_file', tool_data: { tool_data: { path: '/workspace/index.html' } } }] }, { id: 'b', agentEventStream: [{ type: 'tool_call', tool_name: 'knowledge_search' }] }]
  assert.equal(lastWorkspaceSignal(messages)?.path, '/workspace/index.html')
  assert.equal(lastWorkspaceSignal(messages)?.tab, 'preview')
  assert.equal(lastWorkspaceSignal([{ content: '```html\n<h1>example</h1>```' }]), null)
})

test('structured command exit codes override tool-level success and absent status stays unknown', () => {
  const runs = collectWorkspaceRuns([{ agentEventStream: [
    { type: 'tool_call', tool_name: 'shell_exec', success: true, arguments: '{"command":"npm test"}', tool_data: { tool_data: { exit_code: 1, stdout: 'FAIL', stderr: 'assertion', duration_ms: 25 } } },
    { type: 'tool_call', tool_name: 'shell_exec', arguments: { command: 'npm test' } },
  ] }])
  assert.equal(runs[0].status, 'unknown')
  assert.equal(runs[1].status, 'failed')
  assert.equal(runs[1].command, 'npm test')
  assert.equal(runs[1].output, 'FAIL\nassertion')
  assert.equal(runs[1].exitCode, 1)
})

test('standalone preview recognizes supported types and denies network, forms, frames, and base changes', () => {
  assert.equal(canPreviewSource('index.html'), true)
  assert.equal(canPreviewSource('README.md'), true)
  assert.equal(canPreviewSource('logo.svg'), true)
  assert.equal(canPreviewSource('src/App.vue'), false)
  assert.equal(canPreviewSource('index.tsx'), false)
  for (const rule of ["default-src 'none'", "connect-src 'none'", "frame-src 'none'", "base-uri 'none'", "form-action 'none'"]) assert.ok(PREVIEW_CSP.includes(rule))
})

test('a write or edit result exposes the before/after sides the code view diffs', () => {
  const edit = workspaceFileChange({ response_type: 'tool_result', data: {
    tool_name: 'edit_sandbox_file',
    tool_data: { path: 'src/main.ts', diff_before: 'a\n', diff_after: 'b\n', added_lines: 1, removed_lines: 1 },
  } })
  assert.deepEqual(edit, { path: 'src/main.ts', type: 'modified', before: 'a\n', after: 'b\n', addedLines: 1, removedLines: 1 })

  // A write with no pre-image is a brand-new (added) file.
  const created = workspaceFileChange({ response_type: 'tool_result', data: {
    tool_name: 'write_sandbox_file',
    tool_data: { path: 'index.html', diff_before: '', diff_after: '<h1>x</h1>', added_lines: 1 },
  } })
  assert.equal(created?.type, 'added')
  assert.equal(created?.after, '<h1>x</h1>')

  // An oversized payload omits both sides but still names the file to follow.
  const capped = workspaceFileChange({ response_type: 'tool_result', data: {
    tool_name: 'write_sandbox_file',
    tool_data: { path: 'big.txt', added_lines: 900, removed_lines: 0 },
  } })
  assert.equal(capped?.before, undefined)
  assert.equal(capped?.after, undefined)
  assert.equal(capped?.addedLines, 900)

  // Reads and non-mutation tools never produce a diff.
  assert.equal(workspaceFileChange({ response_type: 'tool_result', data: { tool_name: 'read_sandbox_file', tool_data: { path: 'a.txt' } } }), null)
  assert.equal(workspaceFileChange({ response_type: 'tool_call', data: { tool_name: 'write_sandbox_file' } }), null)
})

test('loaded history replays every write and edit, oldest first, skipping pending calls', () => {
  const messages = [
    { id: 'a', agentEventStream: [
      { type: 'tool_call', tool_name: 'write_sandbox_file', tool_data: { path: '/workspace/index.html', diff_before: '', diff_after: 'x' } },
      { type: 'tool_call', tool_name: 'read_sandbox_file', tool_data: { path: '/workspace/README.md' } },
    ] },
    { id: 'b', agentEventStream: [
      { type: 'tool_call', tool_name: 'edit_sandbox_file', pending: true, tool_data: { path: '/workspace/app.ts' } },
      { type: 'tool_call', tool_name: 'edit_sandbox_file', tool_data: { path: '/workspace/app.ts', diff_before: 'a', diff_after: 'b' } },
    ] },
  ]
  const changes = collectWorkspaceFileChanges(messages)
  assert.deepEqual(changes.map((change) => change.path), ['/workspace/index.html', '/workspace/app.ts'])
  assert.equal(changes[0].type, 'added')
  assert.equal(changes[1].type, 'modified')
  assert.equal(changes[1].after, 'b')
})

test('a write or edit result is found however the SSE or history nests it', () => {
  const diff = { path: 'src/main.ts', diff_before: 'a\n', diff_after: 'b\n', added_lines: 1, removed_lines: 1 }
  // Live SSE: the result is flat on `data`.
  assert.equal(workspaceFileChange({ response_type: 'tool_result', data: { tool_name: 'edit_sandbox_file', ...diff } })?.path, 'src/main.ts')
  // History: the result sits one level down under `tool_data`.
  assert.equal(workspaceFileChange({ response_type: 'tool_result', data: { tool_name: 'edit_sandbox_file', tool_data: diff } })?.path, 'src/main.ts')
  // Doubly wrapped: `tool_data.tool_data` and `tool_data.data`.
  assert.equal(workspaceFileChange({ response_type: 'tool_result', data: { tool_name: 'edit_sandbox_file', tool_data: { tool_data: diff } } })?.path, 'src/main.ts')
  assert.equal(workspaceFileChange({ response_type: 'tool_result', data: { tool_name: 'edit_sandbox_file', tool_data: { data: diff } } })?.path, 'src/main.ts')
  // The tool name may only live on the inner record.
  assert.equal(workspaceFileChange({ response_type: 'tool_result', data: { tool_data: { tool_name: 'edit_sandbox_file', path: 'src/main.ts', diff_before: 'a', diff_after: 'b' } } })?.after, 'b')
})

test('replayed history unwraps nested tool_data so the diff survives a reload', () => {
  const messages = [
    { id: 'a', agentEventStream: [
      { type: 'tool_call', tool_name: 'edit_sandbox_file', tool_data: { tool_data: { path: '/workspace/app.ts', diff_before: 'a', diff_after: 'b' } } },
      { type: 'tool_call', tool_name: 'write_sandbox_file', tool_data: { data: { path: '/workspace/new.ts', diff_before: '', diff_after: 'x' } } },
    ] },
  ]
  const changes = collectWorkspaceFileChanges(messages)
  assert.deepEqual(changes.map((change) => change.path), ['/workspace/app.ts', '/workspace/new.ts'])
  assert.equal(changes[0].before, 'a')
  assert.equal(changes[1].type, 'added')
})

test('an exact git path is never taken by a sibling change that only suffix-matches it', () => {
  // Both a.ts and src/a.ts are dirty. The log lists src/a.ts first, so a greedy
  // single pass would hand it git's `a.ts` row (suffix match) and leave the
  // exact a.ts change with the wrong counts.
  const merged = mergeWorkspaceFileChangesWithGitStatus(
    [
      { id: 'l-src', path: '/workspace/src/a.ts', type: 'modified', addedLines: 0, removedLines: 0 },
      { id: 'l-root', path: 'a.ts', type: 'modified', addedLines: 0, removedLines: 0 },
    ],
    [
      { path: 'a.ts', status: 'modified', added: 1, removed: 0 },
      { path: 'src/a.ts', status: 'modified', added: 9, removed: 4 },
    ],
  )
  const byId = Object.fromEntries(merged.map((row) => [row.id, row]))
  assert.equal(byId['l-root'].path, 'a.ts')
  assert.equal(byId['l-root'].addedLines, 1)
  assert.equal(byId['l-src'].path, 'src/a.ts')
  assert.equal(byId['l-src'].addedLines, 9)
  assert.equal(merged.length, 2)
})

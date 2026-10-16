import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { mergeWorkspaceFileChangesWithGitStatus } from '../../utils/workspaceEvents.ts'
import { buildUnifiedDiff } from '../../utils/unifiedDiff.ts'
import { formatWorkspaceCode, highlightWorkspaceLines, trimHighlightedIndent } from '../../utils/workspaceCode.ts'
import { computed, effectScope, nextTick, reactive, ref, watch } from 'vue'

// Exercise the component's state transitions without a browser or a sandbox.
// All filesystem and execution calls pass through an explicitly supplied test gateway.
const source = readFileSync(new URL('./ConversationWorkspace.vue', import.meta.url), 'utf8')
const setup = source.split('<script setup lang="ts">')[1].split('</script>')[0].replace(/^import[\s\S]*?from ['"][^'"]+['"];?\n/gm, '')
const compiled = ts.transpile(setup.replace('export interface', 'interface'), { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None })
const file = path => ({ path, content: path, size: path.length, hash: path })
const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b }); return { promise, resolve, reject } }

function mount(gateway = {}, seed = {}) {
  const props = reactive({ sessionId: 'a', tab: 'source', active: false, revision: 0, focusPath: '', agentRuns: [], gateway: { tree: async () => ({ nodes: [], root: '/workspace' }), file: async (_, path) => file(path), run: async () => ({ stdout: '', stderr: '', exit_code: 0, duration_ms: 1, killed: false }), ...gateway } })
  const emitted = [], cleanup = [], notifications = []
  // The chat view provides one change list per conversation; the tests drive it
  // directly so a live edit can be replayed without the SSE handler.
  const changes = seed.changes || ref([]), activeId = seed.activeId || ref(null)
  const scope = effectScope()
  const state = scope.run(() => vm.runInNewContext(`${compiled}; ({ file, nodes, directory, error, busy, openFile, loadDirectory, refresh, reconcileMissingFile, treeRows, toggleDirectory, childrenByPath, expandedPaths, fileGlyph, fileView, showFilePreview, showFileDiff, showViewSwitch, previewHtml, previewLoading, formattedCode, content, highlightedLines, treeWidth, treeStyle, startTreeResize, resizeTree, endTreeResize, TREE_MIN_WIDTH, TREE_MAX_WIDTH, visibleFileChanges, gitStatusFiles, hasChangesPanel, gitLinked, gitRepos, repoChanges, refreshGitStatus, refreshGitRemote, submitCommit, commitMessages, commitPending, commitErrors, ensureBranches, branchOptions, switchBranch, branchSwitching, branchErrors, confirmDiscard, discardForRepo, discardPending, discardError, submitPull, pullPending, pullErrors, remotes })`, {
    MessagePlugin: { success: message => notifications.push(message) },
    computed, ref, watch, defineProps: () => props, withDefaults: value => value,
    defineEmits: () => (...args) => { if (args[0] !== 'viewChange') emitted.push(args) }, useI18n: () => ({ t: (key, values) => key + (values ? JSON.stringify(values) : '') }),
    onBeforeUnmount: fn => cleanup.push(fn), onMounted: fn => cleanup.push(fn), setTimeout, clearTimeout, Blob,
    formatWorkspaceCode: seed.formatWorkspaceCode || (async source => source),
    highlightWorkspaceLines, trimHighlightedIndent,
    canPreviewSource: path => /\.(?:html?|svg|md|markdown)$/i.test(path),
    buildWorkspacePreview: async (path, content) => ({ html: content, warnings: [] }),
    useFileChanges: () => ({ changes, activeId, addChange() {}, clearChanges() {}, hasChanges: () => changes.value.length > 0 }),
    findFileChange: (list, path) => list.find(change => change.path === path || path.endsWith(`/${change.path}`)),
    buildUnifiedDiff,
    mergeWorkspaceFileChangesWithGitStatus: seed.mergeWorkspaceFileChangesWithGitStatus || mergeWorkspaceFileChangesWithGitStatus,
  }))
  return { props, state, emitted, notifications, changes, activeId, unmount: () => { cleanup.forEach(fn => fn()); scope.stop() } }
}

test('rapid file selections cannot show a stale response', async () => {
  const first = deferred(), second = deferred()
  const view = mount({ file: async (_, path) => path === 'first.ts' ? first.promise : second.promise })
  const a = view.state.openFile('first.ts'), b = view.state.openFile('second.ts')
  second.resolve(file('second.ts')); await b
  first.resolve(file('first.ts')); await a
  assert.equal(view.state.file.value.path, 'second.ts')
  view.unmount()
})

test('changing conversation rejects late files and resets workspace state', async () => {
  const pending = deferred(), view = mount({ file: () => pending.promise })
  const request = view.state.openFile('old.ts')
  view.props.sessionId = 'b'; await nextTick()
  pending.resolve(file('old.ts')); await request
  assert.equal(view.state.file.value, null)
  assert.equal(view.state.directory.value, '')
  view.unmount()
})

test('code tree hides the input and output scratch folders and expands folders lazily', async () => {
  const view = mount({ tree: async (_, path) => path
    ? { root: '/workspace', nodes: [{ path: 'src/main.ts', name: 'main.ts', kind: 'file' }] }
    : { root: '/workspace', nodes: [
      { path: 'input', name: 'input', kind: 'directory' },
      { path: 'output', name: 'output', kind: 'directory' },
      { path: 'src', name: 'src', kind: 'directory' },
      { path: 'index.html', name: 'index.html', kind: 'file' },
    ] } })
  await view.state.loadDirectory()
  assert.deepEqual(Array.from(view.state.treeRows.value, row => row.node.name), ['src', 'index.html'])
  await view.state.toggleDirectory({ path: 'src', name: 'src', kind: 'directory' })
  assert.deepEqual(Array.from(view.state.treeRows.value, row => row.node.name), ['src', 'main.ts', 'index.html'])
  assert.deepEqual(Array.from(view.state.expandedPaths.value), ['src'])
  await view.state.toggleDirectory({ path: 'src', name: 'src', kind: 'directory' })
  assert.deepEqual(Array.from(view.state.treeRows.value, row => row.node.name), ['src', 'index.html'])
  view.unmount()
})

test('host workspaces keep project folders named input or output', async () => {
  const view = mount({ tree: async () => ({ root: '/Users/dev/site', origin: 'host', nodes: [
    { path: 'input', name: 'input', kind: 'directory' },
    { path: 'output', name: 'output', kind: 'directory' },
    { path: 'index.html', name: 'index.html', kind: 'file' },
  ] }) })
  await view.state.loadDirectory()
  assert.deepEqual(Array.from(view.state.treeRows.value, row => row.node.name), ['input', 'output', 'index.html'])
  view.unmount()
})

test('a file the backend no longer serves leaves the tree instead of a dead row', async () => {
  let listings = 0
  const view = mount({
    tree: async (_, path) => {
      if (path === 'todolist') {
        listings += 1
        // The first listing is the row the user clicked; the re-listing after
        // the failed read is what a rebuilt sandbox actually returns.
        return { root: '/workspace', nodes: listings === 1 ? [{ path: 'todolist/test_backend.py', name: 'test_backend.py', kind: 'file' }] : [] }
      }
      return { root: '/workspace', nodes: [{ path: 'todolist', name: 'todolist', kind: 'directory' }] }
    },
    file: async () => { throw Error('error code: 1003, error message: file not found') },
  })
  await view.state.loadDirectory()
  await view.state.toggleDirectory({ path: 'todolist', name: 'todolist', kind: 'directory' })
  assert.deepEqual(Array.from(view.state.treeRows.value, row => row.node.name), ['todolist', 'test_backend.py'])

  await view.state.openFile('todolist/test_backend.py')
  await view.state.reconcileMissingFile('todolist/test_backend.py')
  assert.match(view.state.error.value, /workspace\.fileGone/)
  assert.deepEqual(Array.from(view.state.treeRows.value, row => row.node.name), ['todolist'])
  assert.equal(view.state.file.value, null)
  view.unmount()
})

test('the file tree stays visible alongside the per-file toolbar', () => {
  assert.doesNotMatch(source, /workspace-toolbar/)
  assert.doesNotMatch(source, /treeOpen/)
  assert.match(source, /<aside[^>]*class="file-tree"/)
  assert.doesNotMatch(source, /<aside[^>]*v-if/)
})

test('file markers follow Manus: TS/JS text, braces for data, git glyph for git files', () => {
  const view = mount()
  const glyph = name => view.state.fileGlyph(name)
  assert.equal(glyph('vite.config.ts').text, 'TS')
  assert.equal(glyph('App.tsx').text, 'TS')
  assert.equal(glyph('app.jsx').text, 'JS')
  assert.equal(glyph('package.json').text, '{}')
  assert.equal(glyph('pnpm-lock.yaml').text, '{}')
  assert.equal(glyph('.prettierrc').text, '{}')
  assert.equal(glyph('.gitignore').icon, 'git-branch')
  assert.equal(glyph('.gitkeep').icon, 'file')
  assert.equal(glyph('index.html').icon, 'file')
  assert.equal(glyph('notes.md').icon, 'file')
  view.unmount()
})

test('unavailable workspaces surface recovery states without fabricated output', async () => {
  const view = mount({ tree: async () => { throw Error('unavailable') }, run: async () => { throw Error('offline') } })
  await view.state.loadDirectory()
  assert.match(view.state.error.value, /workspace.unavailable/)
  assert.equal(view.state.busy.value, false)
  view.unmount()
})

test('preview frame has an opaque origin and no form, top navigation, or same-origin privilege', () => {
  const sandbox = source.match(/<iframe[^>]*sandbox="([^"]+)"/)?.[1]
  assert.equal(sandbox, 'allow-scripts')
  assert.match(source, /referrerpolicy="no-referrer"/)
})


test('HTML can switch between source and preview inside the code tab', async () => {
  const view = mount()
  await view.state.openFile('index.html')
  assert.equal(view.state.showFilePreview.value, false)
  view.state.fileView.value = 'preview'; await new Promise(resolve => setImmediate(resolve))
  assert.equal(view.state.showFilePreview.value, true)
  assert.equal(view.state.previewHtml.value, 'index.html')
  await view.state.openFile('style.css'); await nextTick()
  assert.equal(view.state.showFilePreview.value, false)
  await view.state.openFile('other.html'); await new Promise(resolve => setImmediate(resolve))
  assert.equal(view.state.showFilePreview.value, true)
  assert.equal(view.state.previewHtml.value, 'other.html')
  view.state.fileView.value = 'source'; await nextTick()
  assert.equal(view.state.showFilePreview.value, false)
  view.unmount()
})

test('file preview selection resets when changing conversations', async () => {
  const view = mount()
  await view.state.openFile('index.html')
  view.state.fileView.value = 'preview'; await nextTick()
  view.props.sessionId = 'b'; await nextTick()
  assert.equal(view.state.fileView.value, 'source')
  assert.equal(view.state.previewHtml.value, '')
  view.unmount()
})

const flush = async () => { await nextTick(); await new Promise(resolve => setImmediate(resolve)); await nextTick() }
const liveChange = (id, path) => ({ id, path, type: 'modified', before: 'one\n', after: 'two\n', addedLines: 1, removedLines: 1, timestamp: 1, live: true })

test('a live edit opens the changed file in the diff view and follows A then B', async () => {
  // The follow watcher only runs on a git-linked workspace. gitLinked is now
  // set by the backend's git-status probe (refreshGitStatus), not a root .git
  // node, so the test drives a refresh to link before the live edit lands.
  const view = mount({
    tree: async () => ({ nodes: [{ path: '.git', name: '.git', kind: 'directory' }], root: '/workspace' }),
    // The live watcher reads visibleFileChanges, which merges the log with git
    // status once it lands — so the mock reports the live paths as dirty.
    gitStatus: async () => ({ available: true, repos: [{ root: '', name: 'workspace', files: [
      { path: 'a.ts', status: 'modified', added: 1, removed: 1 },
      { path: 'src/b.ts', status: 'modified', added: 1, removed: 1 },
    ] }] }),
  })
  await view.state.loadDirectory('')
  await view.state.refresh()
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(view.state.gitLinked.value, true)
  view.changes.value = [liveChange('a-1', '/workspace/a.ts')]
  view.activeId.value = 'a-1'
  await flush()
  assert.equal(view.state.file.value.path, 'a.ts')
  assert.equal(view.state.fileView.value, 'diff')
  // A second file retargets the view instead of leaving the reader on A.
  view.changes.value = [...view.changes.value, liveChange('b-2', '/workspace/src/b.ts')]
  view.activeId.value = 'b-2'
  await flush()
  assert.equal(view.state.file.value.path, 'src/b.ts')
  assert.equal(view.state.fileView.value, 'diff')
  view.unmount()
})

test('a panel that mounts after a live edit still lands on the diff, not the restored file', async () => {
  // The edit happened before the panel was mounted: the immediate watcher and
  // the refresh both have to land on the changed file in diff view. gitLinked
  // comes from the git-status probe, and the watcher reads visibleFileChanges,
  // so the mock git status must carry the live path.
  const changes = ref([liveChange('a-1', '/workspace/a.ts')])
  const view = mount(
    {
      tree: async () => ({ nodes: [{ path: '.git', name: '.git', kind: 'directory' }], root: '/workspace' }),
      gitStatus: async () => ({ available: true, repos: [{ root: '', name: 'workspace', files: [{ path: 'a.ts', status: 'modified', added: 1, removed: 1 }] }] }),
    },
    { changes, activeId: ref('a-1') },
  )
  await view.state.loadDirectory('')
  await view.state.refresh()
  await flush()
  assert.equal(view.state.fileView.value, 'diff')
  assert.equal(view.state.file.value.path, 'a.ts')
  view.unmount()
})

test('restored history alone does not force the diff view open', async () => {
  const changes = ref([{ ...liveChange('h-1', '/workspace/a.ts'), live: false }])
  const view = mount({}, { changes, activeId: ref('h-1') })
  await flush()
  assert.equal(view.state.fileView.value, 'source')
  view.unmount()
})

test('a changed file opens from the tree with the changes panel available', async () => {
  // The user opens the file from the tree instead of waiting for the live
  // watcher. On a git-linked workspace the 变更 panel is available, and the
  // reader can switch the code pane from source to the changes list. gitLinked
  // is set by the git-status probe, so the test refreshes to link first.
  const changes = ref([{ ...liveChange('m-1', 'a.ts'), live: false }])
  const view = mount(
    {
      tree: async () => ({ nodes: [{ path: '.git', name: '.git', kind: 'directory' }], root: '/workspace' }),
      gitStatus: async () => ({ available: true, repos: [{ root: '', name: 'workspace', files: [{ path: 'a.ts', status: 'modified', added: 1, removed: 1 }] }] }),
    },
    { changes, activeId: ref(null), mergeWorkspaceFileChangesWithGitStatus },
  )
  await view.state.loadDirectory('')
  await view.state.refresh()
  await new Promise(resolve => setImmediate(resolve))
  await view.state.openFile('a.ts')
  await nextTick()
  assert.equal(view.state.hasChangesPanel.value, true)
  // a.ts is not previewable, so the switch exists only because 变更 is present.
  assert.equal(view.state.showViewSwitch.value, true)
  assert.equal(view.state.visibleFileChanges.value.some(row => row.path === 'a.ts'), true)
  view.state.fileView.value = 'diff'
  await nextTick()
  assert.equal(view.state.showFileDiff.value, true)
  view.unmount()
})

test('the code pane switches between source, 变更, and preview by capability', () => {
  // The view switch is one group in the file toolbar: source always, 变更 when
  // the workspace is git-linked, preview when the file is previewable. There is
  // no separate sidebar tab for changes (that assertion lives in the
  // SandboxSidePanel source check below).
  assert.match(source, /v-if="hasChangesPanel"[\s\S]*?fileView = 'diff'/)
  assert.match(source, /v-if="canPreviewFile"[\s\S]*?fileView = 'preview'/)
  assert.match(source, /fileView = 'source'/)
})

test('the code side of the panel is resizable by dragging the tree divider', () => {
  // The sandbox panel already resizes; the code/diff side gets the same grip so
  // a wide diff fits without widening the whole panel.
  assert.match(source, /<PanelResizeHandle[\s\S]*?edge="right"/)
  assert.match(source, /:style="treeStyle"/)
  assert.match(source, /max-width: calc\(100% - 200px\)/)
  const view = mount()
  // Nothing stored yet: the responsive CSS default owns the width.
  assert.equal(view.state.treeWidth.value, 0)
  assert.equal(view.state.treeStyle.value, undefined)
  // Deltas are measured from the width the drag started at, matching the
  // pointer handler that feeds this splitter.
  view.state.startTreeResize()
  view.state.resizeTree(48)
  assert.equal(view.state.treeWidth.value, 180)
  // The style object comes from the vm realm, so compare the field, not the prototype.
  assert.equal(view.state.treeStyle.value.width, '180px')
  // Dragging past the ceiling leaves the code pane its minimum room.
  view.state.resizeTree(100000)
  assert.equal(view.state.treeWidth.value, view.state.TREE_MAX_WIDTH)
  view.state.resizeTree(-100000)
  assert.equal(view.state.treeWidth.value, view.state.TREE_MIN_WIDTH)
  view.unmount()
})

test('the diff renders inside the code view, never as a separate sidebar tab', () => {
  const panel = readFileSync(new URL('../chat/SandboxSidePanel.vue', import.meta.url), 'utf8')
  const fileDiff = readFileSync(new URL('./WorkspaceFileDiff.vue', import.meta.url), 'utf8')
  // The per-file colored diff lives in WorkspaceFileDiff, rendered from the
  // 变更 list inside the code pane.
  assert.match(fileDiff, /class="file-diff"/)
  assert.match(source, /showFileDiff/)
  // The sidebar keeps Preview/Source/Tests/Artifacts/Terminal — no Changes tab.
  assert.doesNotMatch(panel, /id: 'changes'/)
  assert.doesNotMatch(panel, /tabChanges|changesTab/)
})


test('late formatting cannot replace the current file or conversation', async () => {
  const pending = deferred()
  const view = mount({}, { formatWorkspaceCode: source => source === 'old.ts' ? pending.promise : Promise.resolve('formatted ' + source) })
  await view.state.openFile('old.ts')
  await view.state.openFile('new.ts')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(view.state.content.value, 'formatted new.ts')
  pending.resolve('stale old file')
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(view.state.content.value, 'formatted new.ts')
  view.props.sessionId = 'b'; await nextTick()
  assert.equal(view.state.content.value, '')
  view.unmount()
})


test('source formatting leaves the original diff and its line numbers intact', async () => {
  const before = '{"count":1}', after = '{"count":2}'
  // Warm the lazy parser so the assertion only waits for the component update.
  await formatWorkspaceCode(before, 'data.json')
  const changes = ref([{ id: 'edit', path: 'data.json', before, after }])
  const view = mount({ file: async () => ({ ...file('data.json'), content: after }) }, { changes, formatWorkspaceCode })
  await view.state.openFile('data.json')
  await new Promise(resolve => setImmediate(resolve))
  assert.match(view.state.content.value, /\n  "count": 2\n/)
  assert.equal(view.state.file.value.content, after)
  const rows = buildUnifiedDiff(before, after).hunks.flatMap(hunk => hunk.lines)
  assert.equal(rows.find(row => row.type === 'add').newNumber, 1)
  assert.equal(rows.find(row => row.type === 'del').content, '{"count":1}')
  assert.equal(rows.find(row => row.type === 'add').content, '{"count":2}')
  view.unmount()
})

test('formatting does not erase whitespace-only diffs', async () => {
  const before = 'const x=1;', after = 'const x = 1;'
  const changes = ref([{ id: 'edit', path: 'app.js', before, after }])
  const view = mount({ file: async () => ({ ...file('app.js'), content: after }) }, { changes, formatWorkspaceCode: async () => after })
  await view.state.openFile('app.js')
  await new Promise(resolve => setImmediate(resolve))
  const rows = buildUnifiedDiff(before, after).hunks.flatMap(hunk => hunk.lines)
  assert.equal(rows.find(row => row.type === 'del').content, before)
  assert.equal(rows.find(row => row.type === 'add').content, after)
  view.unmount()
})

test('git status snapshot empties the panel after a commit', async () => {
  const changes = ref([{ id: 'edit', path: 'a.go', before: 'package a\n', after: 'package b\n', addedLines: 1, removedLines: 1, timestamp: 0, live: true }])
  const view = mount(
    { tree: async () => ({ nodes: [{ path: '.git', name: '.git', kind: 'directory' }], root: '/workspace' }), gitStatus: async () => ({ available: true, repos: [{ root: '', name: 'workspace', files: [] }] }) },
    { changes, mergeWorkspaceFileChangesWithGitStatus },
  )
  await view.state.loadDirectory('')
  await new Promise(resolve => setImmediate(resolve))
  await view.state.refresh()
  await new Promise(resolve => setImmediate(resolve))
  // The status lands as one empty repo group: the log is superseded by git.
  assert.equal(view.state.gitStatusFiles.value.length, 0)
  assert.equal(view.state.visibleFileChanges.value.length, 0)
  // A clean linked workspace keeps the 变更 panel open so the empty state shows.
  assert.equal(view.state.hasChangesPanel.value, true)
  view.unmount()
})

test('git status keeps rows the log recorded and adds dirty paths it never saw', async () => {
  const changes = ref([{ id: 'edit', path: 'a.go', before: 'package a\n', after: 'package b\n', addedLines: 1, removedLines: 1, timestamp: 0, live: true }])
  const view = mount(
    {
      tree: async () => ({ nodes: [{ path: '.git', name: '.git', kind: 'directory' }], root: '/workspace' }),
      gitStatus: async () => ({ available: true, files: [
        { path: 'a.go', status: 'modified', added: 4, removed: 2 },
        { path: 'b.go', status: 'untracked', added: 7, removed: 0 },
      ] }),
    },
    { changes, mergeWorkspaceFileChangesWithGitStatus },
  )
  await view.state.loadDirectory('')
  await view.state.refresh()
  await new Promise(resolve => setImmediate(resolve))
  const rows = view.state.visibleFileChanges.value
  assert.equal(rows.length, 2)
  const a = rows.find(row => row.path === 'a.go')
  assert.equal(a.before, 'package a\n', 'log supplies the diff bodies')
  assert.equal(a.addedLines, 4, 'git supplies the counts')
  const b = rows.find(row => row.path === 'b.go')
  assert.equal(b.type, 'added')
  assert.equal(b.before, undefined, 'a git-only row has no recorded body')
  view.unmount()
})

test('a non-git workspace shows no changes panel at all', async () => {
  // available:false means the backend found no repository. The panel hides
  // entirely — the session log is NOT surfaced as a fallback, because changes
  // only mean something when git can track them.
  const changes = ref([{ id: 'edit', path: 'a.go', before: 'a', after: 'b', addedLines: 1, removedLines: 1, timestamp: 0, live: true }])
  const view = mount(
    { tree: async () => ({ nodes: [], root: '/workspace' }), gitStatus: async () => ({ available: false }) },
    { changes, mergeWorkspaceFileChangesWithGitStatus },
  )
  await view.state.loadDirectory('')
  await view.state.refresh()
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(view.state.gitLinked.value, false)
  assert.equal(view.state.hasChangesPanel.value, false)
  assert.equal(view.state.visibleFileChanges.value.length, 0)
  view.unmount()
})

test('each repository keeps its own recorded diff bodies and workspace paths', async () => {
  const changes = ref([
    { id: 'app', path: '/workspace/app/main.ts', before: 'app before', after: 'app after' },
    { id: 'lib', path: '/workspace/lib/main.ts', before: 'lib before', after: 'lib after' },
  ])
  for (const roots of [['app'], ['app', 'lib']]) {
    const view = mount({ gitStatus: async () => ({ available: true, repos: roots.map(root => ({ root, name: root, files: [{ path: 'main.ts', status: 'modified', added: 1, removed: 1 }] })) }) }, { changes })
    await view.state.loadDirectory()
    await view.state.refreshGitStatus()
    assert.equal(view.state.repoChanges('app')[0].before, 'app before')
    assert.equal(view.state.repoChanges('app')[0].path, 'main.ts')
    assert.equal(view.state.visibleFileChanges.value[0].path, 'app/main.ts')
    if (roots.length > 1) assert.equal(view.state.repoChanges('lib')[0].before, 'lib before')
    view.unmount()
  }
})

test('a late git status snapshot cannot restore changes after a newer clean snapshot', async () => {
  const pending = deferred()
  let calls = 0
  const view = mount({ gitStatus: () => ++calls === 1 ? pending.promise : Promise.resolve({ available: true, files: [] }) })
  const first = view.state.refreshGitStatus()
  await view.state.refreshGitStatus()
  pending.resolve({ available: true, files: [{ path: 'old.ts', status: 'modified', added: 1, removed: 1 }] })
  await first
  assert.equal(view.state.visibleFileChanges.value.length, 0)
  view.unmount()
})

test('late git actions cannot update or retry against a different conversation', async () => {
  const scenarios = [
    ['gitCommit', state => { state.commitMessages.value.app = 'commit'; return state.submitCommit('app') }, { committed: false, needs_identity: true, detail: 'old error' }],
    ['gitBranches', state => state.ensureBranches('app'), { branches: ['old'], current: 'old' }],
    ['gitSwitchBranch', state => state.switchBranch('app', 'old'), { switched: false, detail: 'old error' }],
    ['gitDiscard', state => { state.discardForRepo.value = 'app'; return state.confirmDiscard() }, { discarded: false, detail: 'old error' }],
    ['gitPull', state => state.submitPull('app'), { pulled: false, detail: 'old error' }],
  ]
  for (const [method, run, result] of scenarios) {
    const pending = deferred(), commits = []
    const view = mount({ gitCommit: async session => { commits.push(session); return { committed: true } }, [method]: () => pending.promise })
    const request = run(view.state)
    view.props.sessionId = 'b'; await nextTick()
    pending.resolve(result); await request
    assert.equal(view.state.discardForRepo.value, null, method)
    for (const key of ['commitErrors', 'commitPending', 'branchOptions', 'branchErrors', 'branchSwitching', 'pullErrors', 'pullPending']) {
      assert.equal(Object.keys(view.state[key].value).length, 0, `${method}: ${key}`)
    }
    assert.equal(view.state.discardError.value, '', method)
    assert.equal(view.notifications.length, 0, method)
    assert.equal(commits.length, 0, 'identity must not retry the old commit in the new conversation')
    view.unmount()
  }
})

test('changing conversation resets branch options and open git dialogs', async () => {
  const view = mount()
  view.state.branchOptions.value.app = ['old']
  view.state.branchErrors.value.app = 'old error'
  view.state.discardForRepo.value = 'app'
  view.state.discardError.value = 'old error'
  view.props.sessionId = 'b'; await nextTick()
  assert.equal(Object.keys(view.state.branchOptions.value).length, 0)
  assert.equal(Object.keys(view.state.branchErrors.value).length, 0)
  assert.equal(view.state.discardForRepo.value, null)
  assert.equal(view.state.discardError.value, '')
  view.unmount()
})

test('remote requests keep the newest snapshot for each repository', async () => {
  const pending = deferred()
  let calls = 0
  const view = mount({ gitRemote: () => ++calls === 1 ? pending.promise : Promise.resolve({ available: true, branch: 'new' }) })
  const first = view.state.refreshGitRemote(false, 'app')
  await view.state.refreshGitRemote(true, 'app')
  pending.resolve({ available: true, branch: 'old' }); await first
  assert.equal(view.state.remotes.value.app.branch, 'new')
  view.unmount()
})

test('branch switches, pulls, and discards reload the tree and open file', async () => {
  for (const method of ['gitSwitchBranch', 'gitPull', 'gitDiscard']) {
    let changed = false, treeCalls = 0
    const view = mount({
      tree: async () => { treeCalls++; return { nodes: [], root: '/workspace' } },
      file: async (_, path) => ({ ...file(path), content: changed ? 'new content' : 'old content' }),
      [method]: async () => { changed = true; return { switched: true, pulled: true, discarded: true } },
    })
    await view.state.loadDirectory()
    await view.state.openFile('main.ts')
    if (method === 'gitSwitchBranch') await view.state.switchBranch('', 'feature')
    if (method === 'gitPull') await view.state.submitPull('')
    if (method === 'gitDiscard') { view.state.discardForRepo.value = ''; await view.state.confirmDiscard() }
    await new Promise(resolve => setImmediate(resolve))
    assert.equal(view.state.file.value.content, 'new content', method)
    assert.equal(treeCalls, 2, method)
    view.unmount()
  }
})

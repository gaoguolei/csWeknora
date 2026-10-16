import assert from 'node:assert/strict'
import test from 'node:test'

import {
  emptyFolderLayout,
  hasFolderLayout,
  normalizeFolderLayout,
  resolveFolderLayout,
  toServerLayout,
  type FolderLayout,
} from './sessionFolderState'

const layout = (patch: Partial<FolderLayout> = {}): FolderLayout => ({
  ...emptyFolderLayout(),
  ...patch,
})

const withFolders = (id = 'f1', name = '工作'): FolderLayout =>
  layout({ folders: [{ id, name, collapsed: false }] })

test('normalizes the server shape (snake_case) and the legacy local shape (camelCase)', () => {
  const fromServer = normalizeFolderLayout({
    folders: [{ id: 'f1', name: '工作', collapsed: true }],
    assignments: { s1: 'f1' },
    sort_mode: 'recent',
    projects_collapsed: true,
  })
  const fromLocal = normalizeFolderLayout({
    folders: [{ id: 'f1', name: '工作', collapsed: true }],
    assignments: { s1: 'f1' },
    sortMode: 'recent',
    projectsCollapsed: true,
  })
  assert.deepEqual(fromServer, fromLocal)
  assert.equal(fromServer.sortMode, 'recent')
  assert.equal(fromServer.projectsCollapsed, true)
  assert.equal(fromServer.folders[0].collapsed, true)
})

test('malformed or missing input degrades to an empty layout', () => {
  for (const bad of [null, undefined, {}, 42, 'nope', { folders: 'nope' }]) {
    assert.deepEqual(normalizeFolderLayout(bad), emptyFolderLayout())
  }
  // Entries without an id/name are dropped rather than rendered as blanks.
  assert.deepEqual(
    normalizeFolderLayout({ folders: [{ name: 'no id' }, { id: 'x' }, null] }).folders,
    [],
  )
})

test('a layout counts as present when it has folders or assignments', () => {
  assert.equal(hasFolderLayout(null), false)
  assert.equal(hasFolderLayout(emptyFolderLayout()), false)
  assert.equal(hasFolderLayout(withFolders()), true)
  assert.equal(hasFolderLayout(layout({ assignments: { s1: 'f1' } })), true)
})

test('the server wins when it has content', () => {
  const remote = withFolders('server', '服务端文件夹')
  const local = withFolders('local', '本地文件夹')
  const { layout: resolved, upload } = resolveFolderLayout(remote, local)
  assert.deepEqual(resolved, remote)
  assert.equal(upload, false, 'a device that already has server data must not re-upload')
})

test('an EMPTY server entry never beats a non-empty local one (regression)', () => {
  // This is the shape a partially-failed sync can leave behind. Treating it as
  // truth is what once wiped folders on both sides.
  const emptyRemote = normalizeFolderLayout({ folders: [] })
  const local = withFolders('local', '本地文件夹')
  const { layout: resolved, upload } = resolveFolderLayout(emptyRemote, local)
  assert.deepEqual(resolved, local)
  assert.equal(upload, true, 'local must be written back so the empty server entry self-heals')
})

test('a missing server entry migrates the local layout up', () => {
  const local = withFolders('local', '本地文件夹')
  const { layout: resolved, upload } = resolveFolderLayout(null, local)
  assert.deepEqual(resolved, local)
  assert.equal(upload, true)
})

test('two empty sides stay empty and upload nothing', () => {
  const { layout: resolved, upload } = resolveFolderLayout(
    normalizeFolderLayout({ folders: [] }),
    emptyFolderLayout(),
  )
  assert.deepEqual(resolved, emptyFolderLayout())
  assert.equal(upload, false, 'a fresh account must not push an empty entry')
})

test('the server layout is serialized with the keys the backend expects', () => {
  const serialized = toServerLayout(
    layout({
      folders: [{ id: 'f1', name: '工作', collapsed: true }],
      assignments: { s1: 'f1' },
      sortMode: 'manual',
      projectsCollapsed: true,
    }),
  )
  assert.deepEqual(serialized, {
    folders: [{ id: 'f1', name: '工作', collapsed: true }],
    assignments: { s1: 'f1' },
    sort_mode: 'manual',
    projects_collapsed: true,
  })
  // Round-trip: what we send must survive normalization back into a layout.
  assert.equal(normalizeFolderLayout(serialized).sortMode, 'manual')
  assert.equal(normalizeFolderLayout(serialized).projectsCollapsed, true)
})
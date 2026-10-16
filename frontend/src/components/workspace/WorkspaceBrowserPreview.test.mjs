import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('./WorkspaceBrowserPreview.vue', import.meta.url), 'utf8')

test('preview renders the web view directly with no desktop tab or connection', () => {
  assert.match(source, /<WorkspaceUrlPreview\b/)
  assert.match(source, /:active="active"/)
  assert.match(source, /:key="sessionId"/)
  assert.doesNotMatch(source, /WorkspaceDesktopPreview|remoteDesktop|desktopUnavailable|preview-modes/)
  assert.doesNotMatch(source, /\benabled\b|\bloading\b|\bretry\b/)
})

test('web preview keeps the workspace identity, updates and actions', () => {
  for (const name of ['session-id', 'agent-id', 'agent-source-tenant-id', 'revision']) {
    assert.ok(source.includes(`:${name}=`), name)
  }
  assert.match(source, /@ask="emit\('ask', \$event\)"/)
  assert.match(source, /@open-terminal="emit\('open-terminal'\)"/)
})

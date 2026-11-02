import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./index.vue', import.meta.url), 'utf8')
function harness() {
  const state = {
    props: { embeddedMode: false }, composerMode: { value: 'coding' },
    sandboxPanel: {
      visible: { value: false }, autoOpenAllowed: { value: false }, activeTab: { value: 'preview' },
      open(tab) { this.visible.value = true; this.activeTab.value = tab },
    },
    workspacePath: { value: '' }, workspaceRevision: { value: 0 },
    workspaceFileView: { value: 'source' },
    workspacePanelTabs: ['source', 'preview', 'tests'],
    completedWorkspacePanel: () => ({ tab: 'source', path: 'app.ts' }),
  }
  const arm = source.slice(source.indexOf('  workspaceAutoRevealPending = !props.embeddedMode &&'), source.indexOf('  const intent = workspaceIntentFromPrompt(', source.indexOf('const sendMsg = async')))
  const live = source.slice(source.indexOf('function showLiveWorkspaceDiff('), source.indexOf('function prefillWorkspacePrompt('))
  return { state, run: Function(...Object.keys(state), `let workspaceAutoRevealPending = false;
    let workspaceTurnSignals = []; let workspaceTurnIntent = 'code';
    ${live}
    return { arm() { ${arm} }, live: showLiveWorkspaceDiff, complete: openCompletedWorkspacePanel };`)(...Object.values(state)) }
}

test('each coding send rearms reveal, but does not open before output', () => {
  const { state, run } = harness()
  run.arm()
  assert.equal(state.sandboxPanel.visible.value, false)
  run.live('app.ts')
  assert.equal(state.sandboxPanel.visible.value, true)
  state.sandboxPanel.visible.value = false
  state.sandboxPanel.autoOpenAllowed.value = false
  run.arm()
  run.live('next.ts')
  assert.equal(state.sandboxPanel.visible.value, true)
})

test('command-only output can reveal at completion; manual close suppresses reveal', () => {
  const { state, run } = harness()
  run.arm()
  run.complete()
  assert.equal(state.sandboxPanel.visible.value, true)
  state.sandboxPanel.visible.value = false
  run.arm()
  state.sandboxPanel.autoOpenAllowed.value = false
  run.live('app.ts')
  run.complete()
  assert.equal(state.sandboxPanel.visible.value, false)
})

test('history and ordinary chat do not arm automatic reveal', () => {
  const { state, run } = harness()
  run.live('history.ts')
  run.complete()
  assert.equal(state.sandboxPanel.visible.value, false)
  state.composerMode.value = 'chat'
  run.arm()
  run.complete()
  assert.equal(state.sandboxPanel.visible.value, false)
})

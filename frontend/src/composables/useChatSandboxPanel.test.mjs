import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'
import { ref, watch } from 'vue'

const source = readFileSync(new URL('./useChatSandboxPanel.ts', import.meta.url), 'utf8')
const start = source.indexOf('export function provideChatSandboxPanel(')
const end = source.indexOf('\nexport function useChatSandboxPanel(', start)
assert.ok(start >= 0 && end > start)
const factory = ts.transpile(source.slice(start, end).replace('export function', 'function'))

function createPanel(mode = ref('coding')) {
  return vm.runInNewContext(`${factory}; provideChatSandboxPanel(testMode)`, {
    ref, watch, testMode: mode,
    initialPanelWidth: () => 420,
    provide() {},
    CHAT_SANDBOX_PANEL_KEY: Symbol(),
  })
}

test('clicking the same folder opens, closes, and reopens its artifacts', () => {
  const panel = createPanel()
  panel.toggleArtifacts('a')
  assert.equal(panel.visible.value, true)
  assert.equal(panel.artifactFocus.value.messageId, 'a')
  panel.toggleArtifacts('a')
  assert.equal(panel.visible.value, false)
  assert.equal(panel.artifactFocus.value, null)
  panel.toggleArtifacts('a')
  assert.equal(panel.visible.value, true)
  assert.equal(panel.artifactFocus.value.messageId, 'a')
})

test('chat panels start on artifacts and reject coding-only navigation', () => {
  const panel = createPanel(ref('chat'))
  panel.open()
  assert.equal(panel.activeTab.value, 'artifacts')
  for (const tab of ['preview', 'source', 'tests']) {
    panel.open(tab)
    assert.equal(panel.activeTab.value, 'artifacts')
    panel.activeTab.value = tab
    assert.equal(panel.activeTab.value, 'artifacts', 'history/direct writes follow the same policy')
  }
  panel.open('terminal')
  assert.equal(panel.activeTab.value, 'terminal')
})

test('switching to chat normalizes a hidden tab without reopening the panel', () => {
  const mode = ref('coding')
  const panel = createPanel(mode)
  panel.open('source')
  panel.close()
  mode.value = 'chat'
  assert.equal(panel.activeTab.value, 'artifacts')
  assert.equal(panel.visible.value, false)
  assert.equal(panel.autoOpenAllowed.value, false)
  mode.value = 'coding'
  panel.open('tests')
  assert.equal(panel.activeTab.value, 'tests')
  panel.open('terminal')
  mode.value = 'chat'
  assert.equal(panel.activeTab.value, 'terminal', 'keep an already open sandbox terminal')
})

test('clicking another message folder switches focus without closing', () => {
  const panel = createPanel()
  panel.toggleArtifacts('a')
  panel.toggleArtifacts('b')
  assert.equal(panel.visible.value, true)
  assert.equal(panel.artifactFocus.value.messageId, 'b')
})

test('manual close suppresses automatic reopening until a new turn or explicit open', () => {
  const panel = createPanel()
  assert.equal(panel.autoOpenAllowed.value, true)
  panel.open('source')
  panel.close()
  assert.equal(panel.autoOpenAllowed.value, false)
  panel.open('tests')
  assert.equal(panel.autoOpenAllowed.value, true)
})

test('folder entry switches from terminal or an unfocused header entry to its files', () => {
  const panel = createPanel()
  panel.open()
  panel.toggleArtifacts('a')
  assert.equal(panel.visible.value, true)
  assert.equal(panel.artifactFocus.value.messageId, 'a')
  panel.open('terminal')
  panel.toggleArtifacts('a')
  assert.equal(panel.visible.value, true)
  assert.equal(panel.activeTab.value, 'artifacts')
})

test('inline preview requests stay open even when repeated; folder click closes them', () => {
  const panel = createPanel()
  for (let i = 0; i < 2; i++) {
    panel.open('artifacts', { messageId: 'a', previewIndex: 0 })
    assert.equal(panel.visible.value, true)
    assert.equal(panel.artifactFocus.value.previewIndex, 0)
  }
  panel.toggleArtifacts('a')
  assert.equal(panel.visible.value, false)
})

test('both answer renderers toggle folders while retaining explicit preview opening', () => {
  for (const file of ['botmsg.vue', 'AgentStreamDisplay.vue']) {
    const component = readFileSync(new URL(`../views/chat/components/${file}`, import.meta.url), 'utf8')
    assert.match(component, /if \(previewIndex == null\)\s*\{\s*sandboxPanel\.toggleArtifacts\(messageIdForArtifacts\.value\)/)
    assert.match(component, /else\s*\{\s*sandboxPanel\.open\('artifacts',/)
  }
})


test('new panels open preview by default and reopening keeps the explicit tab', () => {
  const panel = createPanel()
  panel.open()
  assert.equal(panel.activeTab.value, 'preview')
  panel.close()
  panel.open()
  assert.equal(panel.activeTab.value, 'preview')
  panel.open('source')
  panel.close()
  panel.open()
  assert.equal(panel.activeTab.value, 'source')
})

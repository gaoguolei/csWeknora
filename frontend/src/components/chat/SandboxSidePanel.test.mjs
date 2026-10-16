import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { computed, effectScope, nextTick, reactive, ref, watch } from 'vue'

const panel = readFileSync(new URL('./SandboxSidePanel.vue', import.meta.url), 'utf8')

test('preview has one entry before code, artifacts and terminal', () => {
  const tab = id => panel.search(new RegExp(`id: ["']${id}["']`))
  assert.notEqual(tab('preview'), -1)
  assert.ok(tab('preview') < tab('source'))
  assert.ok(tab('source') < tab('artifacts'))
  assert.ok(tab('artifacts') < tab('terminal'))
  assert.doesNotMatch(panel, /id: ["']desktop["']/)
  assert.match(panel, /<ChatArtifactsPanel/)
  assert.doesNotMatch(panel, /chat-sandbox-panel__title|<h3/)
})

test('web preview never queries or depends on desktop capabilities', () => {
  assert.doesNotMatch(panel, /getProgrammingPreviewCapabilities|desktop_enabled|desktopTabVisible|capabilityRefreshRevision/)
  assert.doesNotMatch(panel, /<SandboxDesktop\b|:enabled=|:loading="capabilitiesLoading"/)
  assert.equal((panel.match(/<WorkspaceBrowserPreview\b/g) || []).length, 1)
})

const setup = panel.split('<script setup lang="ts">')[1].split('</script>')[0]
  .replace(/^import[\s\S]*?from ["'][^"']+["'];?\s*$/gm, '')
const compiled = ts.transpile(setup, { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None })
function mount(options = {}) {
  const props = reactive({ sessionId: 'a', workspaceRevision: 0, ...options })
  const ui = { visible: ref(true), activeTab: ref('preview'), width: ref(560), clearArtifactFocus() {} }
  const scope = effectScope()
  const state = scope.run(() => vm.runInNewContext(`${compiled}; ({ isWorkspaceTab, previewMounted, terminalMounted, tabs })`, {
    computed, nextTick, ref, watch, defineAsyncComponent: () => ({}), defineProps: () => props, withDefaults: value => value, defineEmits: () => () => {},
    useI18n: () => ({ t: key => key }), useChatSandboxPanel: () => ui,
  }))
  return { props, ui, state, stop: () => scope.stop() }
}
const flush = async () => { for (let i = 0; i < 6; i++) await nextTick() }

test('web preview is available without a desktop-enabled agent', async () => {
  const view = mount()
  await flush()
  assert.equal(view.state.tabs.value[0].label, 'workspace.preview')
  assert.equal(view.state.tabs.value.filter(tab => ['preview', 'desktop'].includes(tab.id)).length, 1)
  assert.equal(view.state.previewMounted.value, true)
  assert.equal(view.state.isWorkspaceTab.value, false)
  view.ui.activeTab.value = 'source'; await flush()
  assert.equal(view.state.isWorkspaceTab.value, true)
  view.stop()
})

test('closing the panel drops preview and terminal mounts without reconnecting on Files reopen', async () => {
  const view = mount()
  view.ui.activeTab.value = 'terminal'; await flush()
  assert.equal(view.state.terminalMounted.value, true)
  view.ui.visible.value = false; await flush()
  assert.equal(view.state.previewMounted.value, false)
  assert.equal(view.state.terminalMounted.value, false)
  view.ui.activeTab.value = 'source'
  view.ui.visible.value = true; await flush()
  assert.equal(view.state.previewMounted.value, false)
  assert.equal(view.state.terminalMounted.value, false)
  view.stop()
})

test('workspace gateways keep their existing preview path', async () => {
  const view = mount({ workspaceGateway: {} })
  await flush()
  assert.equal(view.state.isWorkspaceTab.value, true)
  assert.match(panel, /v-if="previewMounted && !workspaceGateway"/)
  view.stop()
})

test('panel slide-in is clipped to the viewport so it cannot create a document scrollbar', () => {
  assert.match(panel, /chat-sandbox-panel-clip/)
  assert.match(panel, /sandbox-panel-enter-from \.chat-sandbox-panel/)
  assert.match(panel, /translateX\(100%\)/)
  assert.match(panel, /:duration/)
})

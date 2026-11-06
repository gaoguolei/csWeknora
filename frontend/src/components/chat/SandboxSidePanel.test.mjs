import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { computed, effectScope, nextTick, reactive, ref, watch } from 'vue'
import { testingArtifactKind } from '../../utils/sessionArtifacts.ts'

const panel = readFileSync(new URL('./SandboxSidePanel.vue', import.meta.url), 'utf8')

test('preview has one entry before code, artifacts and terminal', () => {
  const view = mount()
  assert.deepEqual(Array.from(view.state.tabs.value, tab => tab.id), ['preview', 'source', 'tests', 'artifacts'])
  assert.equal(view.state.moreTools.value[0].id, 'terminal')
  assert.doesNotMatch(panel, /id: ["']desktop["']/)
  assert.match(panel, /<ChatArtifactsPanel/)
  assert.doesNotMatch(panel, /chat-sandbox-panel__title|<h3/)
  view.stop()
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
  const props = reactive({ sessionId: 'a', workspaceRevision: 0, artifacts: [], ...options })
  const ui = { mode: ref(options.mode || 'coding'), visible: ref(true), activeTab: ref(options.mode === 'chat' ? 'artifacts' : options.mode === 'testing' ? 'cases' : 'preview'), width: ref(560), clearArtifactFocus() {}, open(tab) { this.activeTab.value = tab } }
  const scope = effectScope()
  const state = scope.run(() => vm.runInNewContext(`${compiled}; ({ isWorkspaceTab, workspaceMounted, displayedArtifacts, artifactEmptyText, artifactCount, openTab, previewMounted, terminalMounted, tabs, moreTools, isCaseTab })`, {
    computed, nextTick, ref, watch, defineAsyncComponent: () => ({}), defineProps: () => props, withDefaults: value => value, defineEmits: () => () => {},
    useI18n: () => ({ t: key => key }), useChatSandboxPanel: () => ui, testingArtifactKind,
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
  assert.match(panel, /v-if="codingMode && previewMounted && !workspaceGateway"/)
  view.stop()
})

test('chat mode exposes only artifacts and sandbox, with no hidden coding views or more menu', async () => {
  const view = mount({ mode: 'chat', workspaceGateway: {} })
  await flush()
  assert.deepEqual(Array.from(view.state.tabs.value, tab => tab.id), ['artifacts', 'terminal'])
  assert.equal(view.state.tabs.value[1].label, 'workspace.sandboxTab')
  assert.equal(view.state.moreTools.value.length, 0)
  assert.equal(view.state.previewMounted.value, false)
  assert.equal(view.state.isWorkspaceTab.value, false)
  assert.match(panel, /v-if="codingMode && workspaceMounted"/)
  assert.match(panel, /v-if="moreTools.length"/)
  view.ui.activeTab.value = 'terminal'; await flush()
  assert.equal(view.state.terminalMounted.value, true)
  view.ui.mode.value = 'coding'; await flush()
  assert.deepEqual(Array.from(view.state.tabs.value, tab => tab.id), ['preview', 'source', 'tests', 'artifacts'])
  assert.equal(view.state.moreTools.value[0].id, 'terminal')
  view.stop()
})

test('panel slide-in is clipped to the viewport so it cannot create a document scrollbar', () => {
  assert.match(panel, /chat-sandbox-panel-clip/)
  assert.match(panel, /sandbox-panel-enter-from \.chat-sandbox-panel/)
  assert.match(panel, /translateX\(100%\)/)
  assert.match(panel, /:duration/)
})

test('testing exposes case documents, reports, artifacts and terminal without code test discovery', async () => {
  const view = mount({ mode: 'testing', workspaceGateway: {} })
  await flush()
  assert.deepEqual(Array.from(view.state.tabs.value, tab => tab.id), ['cases', 'reports', 'artifacts', 'terminal'])
  assert.deepEqual(Array.from(view.state.tabs.value, tab => tab.label), ['workspace.casesTab', 'workspace.reportsTab', 'chat.sandbox.tabArtifacts', 'chat.sandbox.tabTerminal'])
  assert.equal(view.state.moreTools.value.length, 0)
  assert.equal(view.state.workspaceMounted.value, false)
  assert.equal(view.state.isWorkspaceTab.value, false)
  assert.equal(view.state.previewMounted.value, false)
  assert.doesNotMatch(panel, /WorkspaceTestResults|WorkspaceTestTree|cases-only/)
  assert.match(panel, /<TestCasePanel[\s\S]*?:reports="reportArtifacts"/)
  assert.match(panel, /v-show="isCaseTab"/)
  assert.match(panel, /v-show="isArtifactTab && !isCaseTab"/)
  const cases = { messageId: 'a', index: 3, file_name: '登录测试用例.xlsx' }
  const report = { messageId: 'b', index: 7, file_name: 'test-report.html' }
  const other = { messageId: 'a', index: 9, file_name: 'screenshot.png' }
  view.props.artifacts = [cases, report, other]
  await flush()
  assert.equal(view.state.displayedArtifacts.value[0].index, 3)
  assert.equal(view.state.displayedArtifacts.value.length, 1)
  assert.equal(view.state.artifactCount('cases'), 1)
  assert.equal(view.state.artifactEmptyText.value, 'workspace.caseFilesEmpty')
  assert.equal(view.state.isCaseTab.value, true)
  view.ui.activeTab.value = 'reports'; await flush()
  assert.equal(view.state.displayedArtifacts.value[0].messageId, 'b')
  assert.equal(view.state.displayedArtifacts.value[0].index, 7)
  assert.equal(view.state.artifactCount('reports'), 1)
  view.ui.activeTab.value = 'artifacts'; await flush()
  assert.equal(view.state.displayedArtifacts.value.length, 3)
  view.ui.activeTab.value = 'terminal'; await flush()
  assert.equal(view.state.terminalMounted.value, true)
  view.stop()
})


<template>
  <section class="tests-panel" :aria-label="t('workspace.validationTitle')">
    <header class="tests-heading"><h2>{{ t('workspace.validationTitle') }}</h2><span>{{ t('workspace.validationDescription') }}</span></header>
    <div class="test-toolbar">
      <select v-model="selectedProject" :aria-label="t('workspace.validationProject')">
        <option value="">{{ t('workspace.validationAllProjects') }}</option>
        <option v-for="project in discovery.projects" :key="project.id" :value="project.id">{{ project.name }} · {{ project.path || '/' }}</option>
      </select>
      <button type="button" class="run-primary" :disabled="running" @click="start()"><t-icon :name="starting ? 'loading' : 'play-circle'" :class="{ spinning: starting }" />{{ t('workspace.validationRunProject') }}</button>
      <button type="button" :disabled="loading" @click="discover()"><t-icon name="refresh" />{{ t('workspace.validationRediscover') }}</button>
    </div>
    <label class="auto-validation"><input v-model="autoRun" type="checkbox" />{{ t('workspace.validationAutoRun') }}</label>
    <p v-if="discovery.status === 'paused' || discovery.status === 'sandbox_not_running'" class="notice" role="status">{{ t('workspace.validationResumeHint') }}</p>
    <p v-if="loading" class="notice" role="status">{{ t('workspace.testDiscoveryLoading') }}</p>
    <p v-for="warning in discovery.warnings" :key="warning" class="notice">{{ warning }}</p>
    <p v-if="error" class="notice is-error" role="alert">{{ error.startsWith('workspace.') ? t(error) : error }}</p>
    <p v-if="!loading && !projects.length && discovery.status === 'ready'" class="notice">{{ t('workspace.validationNoTargets') }}</p>
    <WorkspaceTestTree :projects="projects" :results="targetResults" :running="running" @run="target => start({ project_id: target.project_id, target_id: target.id })" />
    <div v-if="runs.length" class="history-picker"><label>{{ t('workspace.validationRunHistory') }}<select v-model="selectedRun"><option v-for="run in runs" :key="run.id" :value="run.id">{{ new Date(run.created_at).toLocaleString() }} · {{ t(`workspace.validationState_${testRunDisplayState(run)}`) }}</option></select></label></div>
    <WorkspaceTestRunDetails :run="current" :stale="stale" :busy="running || probing" @cancel="cancel" @rerun="rerunFailed" @ask="askAgent" @report="openReport" @browser="probeBrowser" @run-case="(check, test) => start({ project_id: check.project_id, target_id: check.id.split(':case:')[0], case_name: test.selector, case_file: test.file })" />
    <p class="test-footnote">{{ t('workspace.validationBrowserBoundary') }}</p>
    <ChatArtifactsDrawer v-model:visible="reportVisible" :session-id="sessionId" :message-id="reportMessage" :preview-index="0" />
  </section>
</template>
<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { workspaceTestsAPI, type WorkspaceTestsGateway } from '@/api/workspaceTests'
import { discoverProgrammingWebPreview } from '@/api/programming'
import { resolveProjectPreviewUrl } from '@/utils/projectPreview'
import { testRunDisplayState, useWorkspaceTests } from '@/composables/useWorkspaceTests'
import WorkspaceTestTree from './WorkspaceTestTree.vue'
import WorkspaceTestRunDetails from './WorkspaceTestRunDetails.vue'
import ChatArtifactsDrawer from '@/views/chat/components/ChatArtifactsDrawer.vue'
const props = withDefaults(defineProps<{ sessionId: string; active: boolean; revision?: number; agentId?: string; agentSourceTenantId?: string | number | null; gateway?: WorkspaceTestsGateway }>(), { revision: 0 })
const emit = defineEmits<{ ask: [prompt: string]; reportCreated: [messageId: string] }>()
const { t } = useI18n()
const agent = computed(() => ({ agent_id: props.agentId, ...(props.agentSourceTenantId ? { agent_source_tenant_id: Number(props.agentSourceTenantId) } : {}) }))
const api = props.gateway || workspaceTestsAPI
const { discovery, runs, selectedProject, selectedRun, current, loading, starting, running, error, stale, autoRun, discover, refreshRuns, start, cancel, rerunFailed } = useWorkspaceTests(toRef(props, 'sessionId'), toRef(props, 'active'), toRef(props, 'revision'), api, agent, id => emit('reportCreated', id))
const projects = computed(() => discovery.value.projects.filter(project => !selectedProject.value || project.id === selectedProject.value))
const targetResults = computed(() => Object.fromEntries((current.value?.result.checks || []).map(check => [check.id, check.status])))
const reportVisible = ref(false), reportMessage = ref('')
const probing = ref(false)
async function probeBrowser() {
  const run = current.value, session = props.sessionId
  if (!run || probing.value) return
  probing.value = true
  let outcome = 'unknown', detail = t('workspace.validationBrowserUnknown')
  try {
    const data = (await discoverProgrammingWebPreview(session)).data
    if (data.status === 'ready' && data.url) {
      const url = new URL(resolveProjectPreviewUrl(data.url, window.location.href))
      url.pathname = '/.weknora/preview-health'
      try {
        const response = await fetch(url, { credentials: 'omit', signal: AbortSignal.timeout(8_000) })
        const body = await response.json().catch(() => null)
        outcome = response.ok && body?.status === 'ready' ? 'passed' : 'blocked'
        detail = response.ok && body?.status !== 'ready' ? t('workspace.validationBrowserUnexpected') : t('workspace.validationBrowserHTTP', { code: response.status })
      } catch { outcome = 'blocked'; detail = t('workspace.validationBrowserNetwork') }
    } else { detail = t('workspace.validationBrowserNotReady') }
    const saved = await api.browser(session, run.id, outcome, detail)
    if (props.sessionId === session) { selectedRun.value = saved.id; await refreshRuns() }
  } catch { if (props.sessionId === session) error.value = t('workspace.validationRequestError') }
  finally { probing.value = false }
}
function openReport(id: string) { reportMessage.value = id; reportVisible.value = true }
function askAgent(fix: boolean) {
  const run = current.value
  if (!run) return
  const action = fix ? t('workspace.validationFixPrompt') : t('workspace.validationAnalyzePrompt')
  emit('ask', `${action}\n\n${t('workspace.validationRunId')}: ${run.id}\n${t('workspace.validationProject')}: ${run.project_id || '/'}\n${t('workspace.validationVersion')}: ${run.result.version?.head || '-'}\n${t('workspace.validationUseToolHint')}`)
}
</script>
<style scoped lang="less">
.tests-panel { flex: 1; min-height: 0; min-width: 0; overflow: auto; padding: 18px 16px; color: var(--td-text-color-primary); }
.tests-heading { margin-bottom: 16px; } h2 { margin: 0 0 7px; font-size: 18px; font-weight: 600; } .tests-heading span { color: var(--td-text-color-secondary); font-size: 12px; line-height: 1.6; }
.test-toolbar { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 12px; }
select { min-width: 0; max-width: 100%; flex: 1; border: 1px solid var(--td-component-stroke); background: var(--td-bg-color-container); color: inherit; border-radius: 5px; padding: 6px; font: inherit; font-size: 12px; }
button { display: inline-flex; align-items: center; gap: 5px; border: 1px solid var(--td-component-stroke); border-radius: 5px; background: transparent; color: inherit; padding: 6px 8px; cursor: pointer; font: inherit; font-size: 12px; }
.run-primary { color: var(--app-on-accent, white); background: var(--td-brand-color); border-color: var(--td-brand-color); } button:disabled { opacity: .5; cursor: not-allowed; }
.notice { font-size: 12px; color: var(--td-text-color-secondary); line-height: 1.6; } .is-error { color: var(--td-error-color); }
.auto-validation { display: flex; align-items: center; gap: 6px; font-size: 11px; color: var(--td-text-color-secondary); margin-bottom: 12px; }
.history-picker { margin-top: 18px; padding-top: 12px; border-top: 1px solid var(--td-component-stroke); } .history-picker label { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; font-size: 12px; }
.test-footnote { margin: 20px 0 0; font-size: 11px; color: var(--td-text-color-placeholder); line-height: 1.6; }
.spinning { animation: test-spin 1s linear infinite; } @keyframes test-spin { to { transform: rotate(360deg); } }
button:focus-visible, select:focus-visible { outline: 2px solid var(--td-brand-color); }
@media (prefers-reduced-motion: reduce) { .spinning { animation: none; } }
</style>

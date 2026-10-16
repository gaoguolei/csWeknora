import { computed, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import type { TestDiscovery, TestRequest, TestRun, WorkspaceTestsGateway } from '@/api/workspaceTests'

export function isActiveTestRun(run: TestRun): boolean {
  return ['queued', 'preparing', 'running', 'cancelling'].includes(run.status)
}

export function testRunDisplayState(run: TestRun): string {
  return run.status === 'completed' ? run.result.outcome : run.status
}

export function useWorkspaceTests(session: Ref<string>, active: Ref<boolean>, revision: Ref<number | undefined>,
  api: WorkspaceTestsGateway, agent: Ref<{ agent_id?: string; agent_source_tenant_id?: number }>,
  reportCreated: (messageId: string) => void = () => {}) {
  const discovery = ref<TestDiscovery>({ status: 'loading', projects: [], warnings: [] })
  const runs = ref<TestRun[]>([]), selectedProject = ref(''), selectedRun = ref('')
  const loading = ref(false), starting = ref(false), error = ref('')
  const autoRun = ref(false)
  let pendingAutoRun = false
  let epoch = 0, discoveryEpoch = 0, refreshEpoch = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let discoveryTimer: ReturnType<typeof setTimeout> | undefined
  const notified = new Set<string>()
  const current = computed(() => runs.value.find(run => run.id === selectedRun.value) || runs.value[0])
  const running = computed(() => runs.value.some(isActiveTestRun) || starting.value)
  const stale = computed(() => Boolean(current.value && !isActiveTestRun(current.value) &&
    (current.value.result.version_changed || (current.value.result.version?.fingerprint && discovery.value.version?.fingerprint && current.value.result.version.fingerprint !== discovery.value.version.fingerprint))))

  function explain(e: unknown) {
    const value = e as { error?: string | { message?: string }; message?: string }
    return typeof value?.error === 'string' ? value.error : value?.error?.message || value?.message || 'workspace.validationRequestError'
  }
  async function discover(allowAuto = false) {
    const generation = epoch, request = ++discoveryEpoch, sid = session.value
    if (!sid || !active.value) return
    loading.value = true
    try {
      const data = await api.projects(sid)
      if (generation === epoch && request === discoveryEpoch) {
        const previous = discovery.value.version?.fingerprint
        discovery.value = data
        if (allowAuto && autoRun.value && data.status === 'ready' && previous && data.version?.fingerprint && previous !== data.version.fingerprint) {
          if (running.value) pendingAutoRun = true
          else void start({ source: 'automatic' })
        }
      }
    } catch (e) {
      if (generation === epoch && request === discoveryEpoch) error.value = explain(e)
    } finally {
      if (generation === epoch && request === discoveryEpoch) loading.value = false
    }
  }
  async function refreshRuns() {
    const generation = epoch, request = ++refreshEpoch, sid = session.value
    if (!sid || !active.value) return
    clearTimeout(timer)
    try {
      const data = await api.runs(sid)
      if (generation !== epoch || request !== refreshEpoch) return
      runs.value = data
      let arrived = ''
      for (const run of data) {
        if (run.report_message_id && !notified.has(run.report_message_id)) {
          notified.add(run.report_message_id); arrived = run.report_message_id
        }
      }
      if (arrived) reportCreated(arrived)
      if (pendingAutoRun && !data.some(isActiveTestRun) && autoRun.value && active.value) {
        pendingAutoRun = false; void start({ source: 'automatic' }); return
      }
      if (data.some(isActiveTestRun)) timer = setTimeout(() => { void refreshRuns() }, 2_000)
      else if (data.some(run => !run.report_message_id && !run.report_error)) timer = setTimeout(() => { void refreshRuns() }, 2_000)
    } catch (e) {
      if (generation === epoch && request === refreshEpoch) {
        error.value = explain(e)
        if (active.value) timer = setTimeout(() => { void refreshRuns() }, 5_000)
      }
    }
  }
  async function start(request: Partial<TestRequest> = {}) {
    if (running.value || !session.value) return
    const generation = epoch, sid = session.value
    starting.value = true; error.value = ''
    try {
      const run = await api.start(sid, { project_id: selectedProject.value, source: 'manual', ...agent.value, ...request })
      if (generation !== epoch) return
      runs.value = [run, ...runs.value.filter(old => old.id !== run.id)]; selectedRun.value = run.id
      void refreshRuns()
      // The explicit run may have resumed the sandbox; refresh discovery too.
      clearTimeout(discoveryTimer); discoveryTimer = setTimeout(() => { void discover() }, 2_500)
    } catch (e) { if (generation === epoch) error.value = explain(e) }
    finally { if (generation === epoch) starting.value = false }
  }
  async function cancel() {
    const run = runs.value.find(isActiveTestRun)
    if (!run) return
    const generation = epoch, sid = session.value
    try {
      const updated = await api.cancel(sid, run.id)
      if (generation === epoch) { runs.value = runs.value.map(item => item.id === run.id ? updated : item); void refreshRuns() }
    } catch (e) { if (generation === epoch) error.value = explain(e) }
  }
  function rerunFailed() {
    if (current.value?.request.case_name) { void start({ ...current.value.request, source: 'manual' }); return }
    const failed = current.value?.result.checks.filter(check => ['failed', 'error', 'blocked', 'timed_out'].includes(check.status)) || []
    if (failed.length) void start({ project_id: '', target_ids: failed.map(check => check.id) })
  }
  watch(session, () => {
    epoch++; discoveryEpoch++; refreshEpoch++; clearTimeout(timer); clearTimeout(discoveryTimer)
    runs.value = []; selectedProject.value = ''; selectedRun.value = ''; error.value = ''; starting.value = false; loading.value = false; notified.clear()
    discovery.value = { status: 'loading', projects: [], warnings: [] }
    pendingAutoRun = false
  }, { flush: 'sync' })
  watch(session, () => {
    autoRun.value = typeof localStorage !== 'undefined' && localStorage.getItem(`workspace-test-auto:${session.value}`) === '1'
  }, { immediate: true })
  watch(autoRun, value => {
    if (typeof localStorage !== 'undefined') localStorage.setItem(`workspace-test-auto:${session.value}`, value ? '1' : '0')
    if (!value) pendingAutoRun = false
  })
  watch([session, active], () => {
    clearTimeout(timer)
    if (active.value && session.value) { void discover(); void refreshRuns() }
  }, { immediate: true })
  watch(revision, () => {
    clearTimeout(discoveryTimer)
    if (active.value) discoveryTimer = setTimeout(() => { void discover(true); void refreshRuns() }, 300)
  })
  onBeforeUnmount(() => { epoch++; clearTimeout(timer); clearTimeout(discoveryTimer) })
  return { discovery, runs, selectedProject, selectedRun, current, loading, starting, running, error, stale, autoRun, discover, refreshRuns, start, cancel, rerunFailed }
}

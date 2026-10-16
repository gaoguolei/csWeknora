import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { discoverProgrammingWebPreview, type ProgrammingWebPreview } from '@/api/programming'
import { resolveProjectPreviewUrl } from '@/utils/projectPreview'

/** Discovery follows the active session; polling never starts or resumes a sandbox. */
export function useWorkspaceWebPreview(
  session: Ref<string>, active: Ref<boolean>, revision: Ref<number | undefined>,
  discover = discoverProgrammingWebPreview,
  agent?: Ref<{ agent_id?: string; agent_source_tenant_id?: string | number }>,
) {
  const result = ref<ProgrammingWebPreview | null>(null)
  const currentUrl = ref(''), busy = ref(false), starting = ref(false), frameKey = ref(0)
  let previewID = '', generation = 0, pendingRefresh = false, pendingStart = false
  let timer: ReturnType<typeof setTimeout> | undefined
  function schedule(delay: number) {
    clearTimeout(timer)
    if (active.value && session.value) timer = setTimeout(() => { void refresh() }, delay)
  }
  async function refresh(start = false) {
    if (!session.value || (!active.value && !start)) return
    if (busy.value) { pendingRefresh = true; pendingStart ||= start; return }
    clearTimeout(timer)
    const epoch = generation
    busy.value = true
    starting.value = start
    try {
      const response = await discover(session.value, { start, preview_id: previewID, ...(start ? agent?.value : {}) })
      if (epoch !== generation) return
      const data = response.data
      if (data.status === 'ready' && data.url && data.preview_id) {
        currentUrl.value = resolveProjectPreviewUrl(data.url, window.location.href)
        previewID = data.preview_id
        result.value = data
      } else {
        currentUrl.value = ''
        result.value = data.status === 'ready' ? { status: 'error' } : data
      }
    } catch {
      if (epoch === generation) {
        result.value = { status: 'error' }
        // Avoid destroying a working iframe on a transient control-plane error.
      }
    } finally {
      if (epoch === generation) {
        busy.value = false
        starting.value = false
        const queuedStart = pendingStart
        const queuedRefresh = pendingRefresh
        pendingStart = false; pendingRefresh = false
        if (queuedStart) void refresh(true)
        else if (queuedRefresh || !['sandbox_unconfigured', 'unsupported', 'setup_required'].includes(result.value?.status || ''))
          schedule(queuedRefresh ? 300 : result.value?.status === 'ready' ? 10_000 : 5_000)
      }
    }
  }
  function reload() {
    if (currentUrl.value) frameKey.value++
    void refresh()
  }
  watch(session, () => {
    generation++; clearTimeout(timer)
    previewID = ''; result.value = null; currentUrl.value = ''; busy.value = false; starting.value = false; pendingRefresh = false; pendingStart = false
  }, { flush: 'sync' })
  watch(() => [session.value, active.value, revision.value, agent?.value.agent_id, agent?.value.agent_source_tenant_id], () => {
    clearTimeout(timer)
    if (active.value && session.value) schedule(150)
  }, { immediate: true })
  onBeforeUnmount(() => { generation++; clearTimeout(timer) })
  return { result, currentUrl, busy, starting, frameKey, refresh, reload }
}

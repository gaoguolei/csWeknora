import { onUnmounted, ref, watch } from 'vue'
import { isToolApprovalPending } from '@/api/mcp-service'

// A missing waiter is terminal, not a successful approval/rejection. Keep auth,
// network and unrelated API errors on their normal failure path.
export function isUnavailableToolApproval(error: unknown): boolean {
  const value = error as { status?: number; message?: string; error?: { message?: string } }
  const message = String(value?.error?.message || value?.message || '')
  return (value?.status === 404 && /pending approval.*(?:not found|completed)/i.test(message))
    || ((value?.status === 400 || value?.status === 409) && /pending approval already resolved/i.test(message))
}

export function usePendingToolApproval(id: () => string | undefined, active: () => boolean) {
  const unavailable = ref(false)
  const checking = ref(false)
  let version = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let currentId: string | undefined

  const markUnavailable = () => {
    unavailable.value = true
    checking.value = false
    clearTimeout(timer)
    version++
  }

  watch([id, active] as const, ([pendingId, enabled]) => {
    const requestVersion = ++version
    clearTimeout(timer)
    checking.value = false
    if (pendingId !== currentId) {
      currentId = pendingId
      unavailable.value = false
    }
    if (!pendingId || !enabled || unavailable.value) return

    const check = async () => {
      checking.value = true
      try {
        const pending = await isToolApprovalPending(pendingId)
        if (requestVersion !== version) return
        if (!pending) {
          markUnavailable()
          return
        }
      } catch {
        // A failed status request must never grant permission or silently
        // dismiss a live approval. The submission path reports real failures.
      } finally {
        if (requestVersion === version) checking.value = false
      }
      if (requestVersion === version) timer = setTimeout(check, 15000)
    }
    void check()
  }, { immediate: true })

  onUnmounted(() => {
    version++
    clearTimeout(timer)
  })
  return { unavailable, checking, markUnavailable }
}

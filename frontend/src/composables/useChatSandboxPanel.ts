import { inject, provide, ref, watch, type InjectionKey, type Ref } from 'vue'
import type { ComposerMode } from '@/utils/composerMode'

export type SandboxPanelTab = 'artifacts' | 'terminal' | 'preview' | 'source' | 'tests'

export type ArtifactPanelFocus = {
  messageId: string
  previewIndex?: number | null
}

export type ArtifactPanelFocusState = ArtifactPanelFocus & {
  nonce: number
}

// 面板宽度可拖拽调整，持久化到 localStorage。
export const SANDBOX_PANEL_MIN_WIDTH = 320
export const SANDBOX_PANEL_MAX_WIDTH = 1200
export const SANDBOX_PANEL_DEFAULT_WIDTH = 560

function clampPanelWidth(width: number): number {
  if (typeof window === 'undefined') return SANDBOX_PANEL_DEFAULT_WIDTH
  const viewportCap = Math.max(SANDBOX_PANEL_MIN_WIDTH, window.innerWidth - 620)
  return Math.min(
    SANDBOX_PANEL_MAX_WIDTH,
    viewportCap,
    Math.max(SANDBOX_PANEL_MIN_WIDTH, Math.round(width)),
  )
}

function initialPanelWidth(): number {
  const raw = Number(localStorage.getItem('sandbox_panel_width'))
  return Number.isFinite(raw) && raw > 0 ? clampPanelWidth(raw) : clampPanelWidth(SANDBOX_PANEL_DEFAULT_WIDTH)
}

export type ChatSandboxPanelContext = {
  mode: Readonly<Ref<ComposerMode>>
  visible: Ref<boolean>
  activeTab: Ref<SandboxPanelTab>
  width: Ref<number>
  setWidth: (width: number) => void
  artifactFocus: Ref<ArtifactPanelFocusState | null>
  open: (tab?: SandboxPanelTab, focus?: ArtifactPanelFocus) => void
  toggleArtifacts: (messageId: string) => void
  close: () => void
  clearArtifactFocus: () => void
  autoOpenAllowed: Ref<boolean>
}

const CHAT_SANDBOX_PANEL_KEY: InjectionKey<ChatSandboxPanelContext> = Symbol(
  'chatSandboxPanel',
)

export function provideChatSandboxPanel(mode: Readonly<Ref<ComposerMode>> = ref<ComposerMode>('coding')): ChatSandboxPanelContext {
  const allowedTab = (tab: SandboxPanelTab): SandboxPanelTab =>
    mode.value === 'coding' || tab === 'artifacts' || tab === 'terminal' ? tab : 'artifacts'
  const visible = ref(false)
  const activeTab = ref<SandboxPanelTab>(allowedTab('preview'))
  // Guard direct assignments as well as open(): stream/history callbacks and
  // session changes must never bring a coding-only view into chat mode.
  watch([mode, activeTab], () => {
    const next = allowedTab(activeTab.value)
    if (next !== activeTab.value) activeTab.value = next
  }, { flush: 'sync' })
  const width = ref(initialPanelWidth())
  const artifactFocus = ref<ArtifactPanelFocusState | null>(null)
  const autoOpenAllowed = ref(true)
  let focusNonce = 0

  const setWidth = (next: number) => {
    width.value = clampPanelWidth(next)
    localStorage.setItem('sandbox_panel_width', String(width.value))
  }

  const open = (tab?: SandboxPanelTab, focus?: ArtifactPanelFocus) => {
    autoOpenAllowed.value = true
    // Reopening keeps the selected tab; a new panel starts with preview.
    activeTab.value = allowedTab(tab || activeTab.value)
    if (focus) {
      artifactFocus.value = { ...focus, nonce: ++focusNonce }
    }
    visible.value = true
  }

  const close = () => {
    autoOpenAllowed.value = false
    visible.value = false
    artifactFocus.value = null
  }

  const toggleArtifacts = (messageId: string) => {
    if (
      visible.value &&
      activeTab.value === 'artifacts' &&
      artifactFocus.value?.messageId === messageId
    ) {
      close()
      return
    }
    open('artifacts', { messageId })
  }

  const clearArtifactFocus = () => {
    artifactFocus.value = null
  }

  const ctx: ChatSandboxPanelContext = {
    mode,
    visible,
    activeTab,
    width,
    setWidth,
    artifactFocus,
    open,
    toggleArtifacts,
    close,
    clearArtifactFocus,
    autoOpenAllowed,
  }

  provide(CHAT_SANDBOX_PANEL_KEY, ctx)
  return ctx
}

export function useChatSandboxPanel(): ChatSandboxPanelContext | null {
  return inject(CHAT_SANDBOX_PANEL_KEY, null)
}

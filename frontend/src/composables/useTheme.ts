import { ref } from 'vue'
import {
  loadPreference,
  savePreference,
  migratePreferencesIntoUser,
  readUserId,
  safeGetItem,
  safeSetItem,
} from './preferenceStorage'

export type ThemeMode = 'light' | 'dark' | 'system'

const THEME_KEY = 'theme'

// Device-level mirror of the most recently chosen theme. Only consulted for
// the pre-login "anon" namespace so the login page renders with the same
// light/dark mode the user last saw, instead of flashing light and flipping
// to dark on successful login. Logged-in users keep strict per-user isolation.
const LAST_THEME_KEY = 'WeKnora_last_theme'

function isThemeMode(v: unknown): v is ThemeMode {
  return v === 'light' || v === 'dark' || v === 'system'
}

function loadTheme(): ThemeMode {
  const v = loadPreference(THEME_KEY)
  if (isThemeMode(v)) {
    // Seed the device-level mirror whenever a signed-in user's own
    // preference is read (app start, login, logout), so the pre-login
    // page picks it up without requiring the user to re-toggle theme.
    if (readUserId() !== 'anon') safeSetItem(LAST_THEME_KEY, v)
    return v
  }
  if (readUserId() === 'anon') {
    const last = safeGetItem(LAST_THEME_KEY)
    if (isThemeMode(last)) return last
  }
  return 'light'
}

// Shared reactive state across all consumers
const currentTheme = ref<ThemeMode>(loadTheme())

let lastEffective: 'light' | 'dark' | null = null

function getSystemTheme(): 'light' | 'dark' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** Keep browser and native window backgrounds on the same workspace surface. */
function syncWailsNativeChrome(effective: 'light' | 'dark') {
  const bg = 'var(--td-bg-color-page)'
  document.documentElement.style.background = bg
  document.documentElement.style.minHeight = '100%'
  document.documentElement.style.colorScheme = effective === 'dark' ? 'dark' : 'light'
  if (document.body) {
    document.body.style.background = bg
    document.body.style.minHeight = '100%'
  }
  const appEl = document.getElementById('app')
  if (appEl) {
    appEl.style.background = bg
    appEl.style.minHeight = '100%'
  }
  const w = (window as unknown as {
    runtime?: {
      WindowSetDarkTheme?: () => void
      WindowSetLightTheme?: () => void
      WindowSetBackgroundColour?: (r: number, g: number, b: number, a: number) => void
    }
  }).runtime
  if (!w?.WindowSetBackgroundColour) return
  try {
    if (effective === 'dark') {
      w.WindowSetDarkTheme?.()
    } else {
      w.WindowSetLightTheme?.()
    }
    const rgb = getComputedStyle(document.documentElement).backgroundColor.match(/[\d.]+/g)
    if (rgb && rgb.length >= 3) {
      w.WindowSetBackgroundColour(Number(rgb[0]), Number(rgb[1]), Number(rgb[2]), 255)
    }
  } catch {
    /* 非桌面壳或未注入 runtime */
  }
}

function applyTheme(mode: ThemeMode) {
  const effective = mode === 'system' ? getSystemTheme() : mode
  if (lastEffective === effective) return
  const mutate = () => {
    lastEffective = effective
    document.documentElement.setAttribute('theme-mode', effective)
    syncWailsNativeChrome(effective)
  }
  // 主题切换动效：新界面扫出覆盖旧界面（见 theme.css ::view-transition-*）。
  // 方向：进入深色从左往右，回到浅色从右往左，通过 data-theme-sweep
  // 属性告知 CSS。首次加载、浏览器不支持或用户偏好减弱动效时直接瞬切。
  const doc = document as Document & {
    startViewTransition?: (cb: () => void) => { finished?: Promise<unknown> }
  }
  const animate =
    typeof doc.startViewTransition === 'function' &&
    lastEffective !== null &&
    !window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (!animate) {
    mutate()
    return
  }
  doc.documentElement.setAttribute('data-theme-sweep', effective === 'dark' ? 'ltr' : 'rtl')
  const transition = doc.startViewTransition!(mutate)
  const reset = () => doc.documentElement.removeAttribute('data-theme-sweep')
  if (transition?.finished) {
    transition.finished.then(reset, reset)
  } else {
    reset()
  }
}

export function useTheme() {
  function setTheme(mode: ThemeMode): boolean {
    if (!isThemeMode(mode)) return false
    currentTheme.value = mode
    savePreference(THEME_KEY, mode)
    safeSetItem(LAST_THEME_KEY, mode)
    applyTheme(mode)
    return true
  }

  return { currentTheme, setTheme }
}

/** Call once in main.ts to initialise theme and listen for OS changes. */
export function initTheme() {
  currentTheme.value = loadTheme()
  applyTheme(currentTheme.value)

  // React to OS theme changes when user chose "system"
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (currentTheme.value === 'system') {
      applyTheme('system')
    }
  })
}

/** Re-read preferences from storage (call after login / logout). */
export function reloadThemeFromStorage() {
  migratePreferencesIntoUser()
  currentTheme.value = loadTheme()
  applyTheme(currentTheme.value)
}

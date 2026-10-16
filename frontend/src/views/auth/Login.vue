<template>
  <div class="auth-page">
    <!-- 右上角控制：登录前也要能切语言与主题。语言存 localStorage('locale')，
         主题存 WeKnora_anon_theme（见 useTheme / index.html 的首屏引导），
         登录后由 preferenceStorage 迁移到用户命名空间。 -->
    <div class="auth-toolbar">
      <t-dropdown
        :options="languageOptions"
        trigger="click"
        placement="bottom-right"
        attach="body"
        @click="handleLanguageChange"
      >
        <button
          type="button"
          class="auth-toolbar__btn"
          :aria-label="$t('language.selectLanguage')"
          aria-haspopup="menu"
        >
          <t-icon name="translate" size="16px" />
          <span class="auth-toolbar__label">{{ currentLanguageLabel }}</span>
          <t-icon name="chevron-down" size="12px" />
        </button>
      </t-dropdown>
      <button
        type="button"
        class="auth-toolbar__btn auth-toolbar__btn--icon"
        :aria-label="$t('theme.theme')"
        :aria-pressed="isDark"
        :title="isDark ? $t('theme.light') : $t('theme.dark')"
        @click="toggleTheme"
      >
        <t-icon :name="isDark ? 'sunny' : 'moon'" size="16px" />
      </button>
    </div>

    <!-- 左侧品牌面板 -->
    <aside class="brand-panel">
      <div class="brand-panel__inner">
        <!-- 与 WorkspaceWelcome 同一套三条杠标志，替掉原先只存在于样式里的字标。 -->
        <div class="brand-lockup">
          <span class="brand-mark" aria-hidden="true">
            <span></span><span></span><span></span>
          </span>
          <span class="brand-lockup__name">WeKnora</span>
        </div>

        <div class="brand-copy">
          <h1 class="brand-copy__title">{{ $t('platform.subtitle') }}</h1>
          <p class="brand-copy__desc">{{ $t('platform.description') }}</p>
        </div>

        <ul class="brand-features">
          <li class="brand-features__item">
            <t-icon name="check" class="brand-features__check" size="18px" />
            <span>{{ $t('platform.multimodalParsing') }}</span>
          </li>
          <li class="brand-features__item">
            <t-icon name="check" class="brand-features__check" size="18px" />
            <span>{{ $t('platform.hybridSearchEngine') }}</span>
          </li>
          <li class="brand-features__item">
            <t-icon name="check" class="brand-features__check" size="18px" />
            <span>{{ $t('platform.ragQandA') }}</span>
          </li>
        </ul>

        <div class="brand-tags">
          <span class="brand-tag">{{ $t('platform.rag') }}</span>
          <span class="brand-tag">{{ $t('platform.agent') }}</span>
          <span class="brand-tag">{{ $t('platform.wiki') }}</span>
          <span class="brand-tag">{{ $t('platform.hybridSearch') }}</span>
        </div>
      </div>
    </aside>

    <!-- 右侧表单区 -->
    <main class="form-panel">
      <div class="form-panel__theme">
        <ThemeToggle />
      </div>
      <div class="form-panel__inner">
        <!-- 登录 -->
        <div v-if="!isRegisterMode" class="auth-card">
          <div v-if="inviteLookup" class="invite-banner">
            <t-icon name="link" class="invite-banner__icon" />
            <div class="invite-banner__text">
              <div class="invite-banner__title">
                {{
                  $t('inviteRegister.bannerTitle', {
                    tenant: inviteLookup.tenant_name || '',
                  })
                }}
              </div>
              <div class="invite-banner__hint">
                {{ $t('inviteRegister.bannerHintLogin') }}
              </div>
            </div>
          </div>
          <div
            v-else-if="inviteLookupError"
            class="invite-banner invite-banner--error"
          >
            {{ inviteLookupError }}
          </div>

          <header class="auth-card__header">
            <div class="auth-card__eyebrow">ALLTHINKER WORKSPACE</div>
            <h2 class="auth-card__title">{{ $t('auth.login') }}</h2>
            <p class="auth-card__subtitle">{{ $t('auth.subtitle') }}</p>
          </header>

          <p v-if="registrationEnabled" class="auth-card__hint">
            {{ $t('auth.loginHint') }}
          </p>

          <t-form
            ref="formRef"
            :data="formData"
            :rules="formRules"
            @submit="handleLogin"
            layout="vertical"
            label-align="top"
            class="auth-form"
          >
            <t-form-item :label="$t('auth.email')" name="email">
              <t-input
                v-model="formData.email"
                :placeholder="$t('auth.emailPlaceholder')"
                type="text"
                autocomplete="email"
                size="large"
                :disabled="loading"
              />
            </t-form-item>

            <t-form-item :label="$t('auth.password')" name="password">
              <t-input
                v-model="formData.password"
                :placeholder="$t('auth.passwordPlaceholder')"
                type="password"
                autocomplete="current-password"
                size="large"
                :disabled="loading"
                @enter="handleLogin"
              />
            </t-form-item>

            <t-button
              theme="primary"
              type="submit"
              size="large"
              block
              :loading="loading"
              class="btn btn--submit"
            >
              {{ loading ? $t('auth.loggingIn') : $t('auth.login') }}
            </t-button>

            <t-button
              v-if="registrationEnabled"
              theme="default"
              variant="outline"
              size="large"
              block
              :disabled="loading"
              class="btn"
              @click="toggleMode"
            >
              {{ $t('auth.createAccount') }}
            </t-button>

            <template v-if="oidcEnabled">
              <div class="auth-divider">
                <span>{{ $t('auth.orContinueWith') }}</span>
              </div>
              <t-button
                theme="default"
                size="large"
                block
                :loading="oidcLoading"
                :disabled="loading"
                class="btn"
                @click="handleOIDCLogin"
              >
                {{ oidcLoading ? $t('auth.redirectingToOIDC') : oidcLoginText }}
              </t-button>
            </template>
          </t-form>
        </div>

        <!-- 注册 -->
        <div v-else-if="registrationEnabled" class="auth-card">
          <div v-if="inviteLookup" class="invite-banner">
            <t-icon name="link" class="invite-banner__icon" />
            <div class="invite-banner__text">
              <div class="invite-banner__title">
                {{
                  $t('inviteRegister.bannerTitle', {
                    tenant: inviteLookup.tenant_name || '',
                  })
                }}
              </div>
              <div class="invite-banner__hint">
                {{ $t('inviteRegister.bannerHint') }}
              </div>
            </div>
          </div>
          <div
            v-else-if="inviteLookupError"
            class="invite-banner invite-banner--error"
          >
            {{ inviteLookupError }}
          </div>

          <header class="auth-card__header">
            <div class="auth-card__eyebrow">ALLTHINKER WORKSPACE</div>
            <h2 class="auth-card__title">{{ $t('auth.createAccount') }}</h2>
            <p class="auth-card__subtitle">{{ $t('auth.subtitle') }}</p>
          </header>

          <t-form
            ref="registerFormRef"
            :data="registerData"
            :rules="registerRules"
            @submit="handleRegister"
            layout="vertical"
            label-align="top"
            class="auth-form"
          >
            <t-form-item :label="$t('auth.username')" name="username">
              <t-input
                v-model="registerData.username"
                :placeholder="$t('auth.usernamePlaceholder')"
                size="large"
                :disabled="loading"
              />
            </t-form-item>

            <t-form-item :label="$t('auth.email')" name="email">
              <t-input
                v-model="registerData.email"
                :placeholder="$t('auth.emailPlaceholder')"
                type="text"
                autocomplete="email"
                size="large"
                :disabled="loading"
              />
            </t-form-item>

            <t-form-item :label="$t('auth.password')" name="password">
              <t-input
                v-model="registerData.password"
                :placeholder="$t('auth.passwordPlaceholder')"
                type="password"
                autocomplete="new-password"
                size="large"
                :disabled="loading"
              />
            </t-form-item>

            <t-form-item
              :label="$t('auth.confirmPassword')"
              name="confirmPassword"
            >
              <t-input
                v-model="registerData.confirmPassword"
                :placeholder="$t('auth.confirmPasswordPlaceholder')"
                type="password"
                autocomplete="new-password"
                size="large"
                :disabled="loading"
                @enter="handleRegister"
              />
            </t-form-item>

            <t-button
              theme="primary"
              type="submit"
              size="large"
              block
              :loading="loading"
              class="btn btn--submit"
            >
              {{ loading ? $t('auth.registering') : $t('auth.register') }}
            </t-button>
          </t-form>

          <div class="auth-card__footer">
            <span>{{ $t('auth.haveAccount') }}</span>
            <a href="#" @click.prevent="toggleMode" class="auth-link">
              {{ $t('auth.backToLogin') }}
            </a>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import {
  ref,
  reactive,
  nextTick,
  onMounted,
  onBeforeUnmount,
  computed,
} from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoleLabel } from '@/composables/useRoleLabel'
import { notifyLoginSuccess } from '@/utils/loginNotify'
import { canRegister } from './registrationPolicy'
import { newPasswordRules } from '@/utils/passwordPolicy'
import {
  login,
  register,
  getOIDCAuthorizationURL,
  getOIDCConfig,
  autoSetup,
  getAuthConfig,
  userInfoFromApi,
  getInvitationByToken,
  registerByInvite,
  type InviteLookup,
} from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { useI18n } from 'vue-i18n'
import {
  SUPPORTED_LOCALES,
  type SupportedLocale,
} from '@/i18n/resolveDefaultLocale'
import { useTheme } from '@/composables/useTheme'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t, tm, locale } = useI18n()
const { formatRole, roleIcon } = useRoleLabel()
const { currentTheme, setTheme } = useTheme()

// 登录页也能切语言与主题：语言沿用设置页的写法（locale.value +
// localStorage['locale']，i18n 启动时读取）；主题走 useTheme，落到
// WeKnora_anon_theme，登录时由 preferenceStorage 迁移给用户。
const LOCALE_LABEL_KEYS: Record<SupportedLocale, string> = {
  'zh-CN': 'language.zhCN',
  'zh-TW': 'language.zhTW',
  'en-US': 'language.enUS',
  'ru-RU': 'language.ruRU',
  'ko-KR': 'language.koKR',
  'ja-JP': 'language.jaJP',
}

const languageOptions = computed(() =>
  SUPPORTED_LOCALES.map((code) => ({
    content: t(LOCALE_LABEL_KEYS[code]),
    value: code,
    active: locale.value === code,
  })),
)

const currentLanguageLabel = computed(() => {
  const code = SUPPORTED_LOCALES.find((item) => item === locale.value)
  return code ? t(LOCALE_LABEL_KEYS[code]) : locale.value
})

const handleLanguageChange = (option: { value?: string }) => {
  const code = option?.value
  if (!code || !(SUPPORTED_LOCALES as readonly string[]).includes(code)) return
  locale.value = code
  localStorage.setItem('locale', code)
}

// currentTheme 为 "system" 时 useTheme 不改写它，所以这里自己跟一次媒体查询，
// 否则用户在登录页停留期间系统换主题、图标会停在旧状态。
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)')
const systemDark = ref(prefersDark.matches)
const onSystemThemeChange = (event: MediaQueryListEvent) => {
  systemDark.value = event.matches
}
const isDark = computed(() =>
  currentTheme.value === 'system'
    ? systemDark.value
    : currentTheme.value === 'dark',
)

const toggleTheme = () => {
  setTheme(isDark.value ? 'light' : 'dark')
}

const formRef = ref()
const registerFormRef = ref()

const loading = ref(false)
const oidcLoading = ref(false)
const isRegisterMode = ref(false)
const oidcEnabled = ref(false)
const oidcProviderName = ref('')
// Wait for configuration before showing a registration entry.
const registrationMode = ref('')
const registrationEnabled = computed(() =>
  canRegister(registrationMode.value, !!inviteLookup.value),
)
const complexPasswordEnabled = ref(false)

const inviteToken = ref('')
const inviteLookup = ref<InviteLookup | null>(null)
const inviteLookupError = ref('')
const inviteLookupLoading = ref(false)

const oidcLoginText = computed(() => {
  if (oidcProviderName.value) {
    return t('auth.oidcLoginWithProvider', {
      provider: oidcProviderName.value,
    })
  }
  return t('auth.oidcLogin')
})

const formData = reactive<{ [key: string]: any }>({
  email: '',
  password: '',
})

const registerData = reactive<{ [key: string]: any }>({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
})

const formRules = computed(() => ({
  email: [
    { required: true, message: t('auth.emailRequired'), type: 'error' },
    { email: true, message: t('auth.emailInvalid'), type: 'error' },
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8, message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' },
  ],
}))

const registerRules = computed(() => ({
  username: [
    { required: true, message: t('auth.usernameRequired'), type: 'error' },
    { min: 2, message: t('auth.usernameMinLength'), type: 'error' },
    { max: 20, message: t('auth.usernameMaxLength'), type: 'error' },
    {
      pattern: /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/,
      message: t('auth.usernameInvalid'),
      type: 'error',
    },
  ],
  email: [
    { required: true, message: t('auth.emailRequired'), type: 'error' },
    { email: true, message: t('auth.emailInvalid'), type: 'error' },
  ],
  password: newPasswordRules(t, complexPasswordEnabled.value),
  confirmPassword: [
    {
      required: true,
      message: t('auth.confirmPasswordRequired'),
      type: 'error',
    },
    {
      validator: (val: string) => val === registerData.password,
      message: t('auth.passwordMismatch'),
      type: 'error',
    },
  ],
}))

const toggleMode = () => {
  isRegisterMode.value = !isRegisterMode.value && registrationEnabled.value

  Object.keys(registerData).forEach((key) => {
    ;(registerData as any)[key] = ''
  })
}

const persistLoginResponse = async (response: any, skipRedirect = false) => {
  const activeTenant = response.active_tenant || response.tenant
  if (response.user && response.token) {
    const homeTenantIdRaw = response.user.tenant_id ?? activeTenant?.id ?? ''
    authStore.setUser(userInfoFromApi(response.user, homeTenantIdRaw))
    authStore.setToken(response.token)
    if (response.refresh_token) {
      authStore.setRefreshToken(response.refresh_token)
    }
    if (activeTenant) {
      authStore.setTenant({
        id: String(activeTenant.id) || '',
        name: activeTenant.name || '',
        owner_id: response.user.id || '',
        created_at: activeTenant.created_at || new Date().toISOString(),
        updated_at: activeTenant.updated_at || new Date().toISOString(),
      })
    } else {
      authStore.setTenant(null)
    }
    if (Array.isArray(response.memberships)) {
      authStore.setMemberships(response.memberships)
    }
    const activeIdNum = Number(activeTenant?.id)
    const homeIdNum = Number(homeTenantIdRaw)
    if (
      Number.isFinite(activeIdNum) &&
      Number.isFinite(homeIdNum) &&
      activeIdNum !== homeIdNum
    ) {
      authStore.setSelectedTenant(activeIdNum, activeTenant?.name || null)
    } else {
      authStore.setSelectedTenant(null, null)
    }
  }

  await authStore.refreshFromAuthMe()
  await nextTick()
  if (skipRedirect) return
  router.replace(
    authStore.hasValidTenant ? '/platform/creatChat' : '/onboarding/workspace',
  )
}

const getBackendOIDCRedirectURI = () =>
  `${window.location.origin}/api/v1/auth/oidc/callback`

const loadOIDCConfig = async () => {
  try {
    const response = await getOIDCConfig()
    oidcEnabled.value = !!response.success && !!response.enabled
    oidcProviderName.value = response.provider_display_name || ''
  } catch {
    oidcEnabled.value = false
    oidcProviderName.value = ''
  }
}

// Registration stays hidden if configuration cannot be loaded.
const loadAuthConfig = async () => {
  try {
    const response = await getAuthConfig()
    registrationMode.value = response.success ? response.registration_mode : ''
    complexPasswordEnabled.value = response.complex_password_enabled
  } catch {
    registrationMode.value = ''
    complexPasswordEnabled.value = false
  }
}

const handleOIDCLogin = async () => {
  try {
    oidcLoading.value = true
    const response = await getOIDCAuthorizationURL(getBackendOIDCRedirectURI())
    const authorizationURL = response.authorization_url

    if (!response.success || !authorizationURL) {
      MessagePlugin.error(response.message || t('auth.oidcLoginFailed'))
      return
    }

    if (inviteToken.value) {
      sessionStorage.setItem('weknora_pending_invite_token', inviteToken.value)
    }
    window.location.href = authorizationURL
  } catch (error: any) {
    console.error('OIDC 登录跳转失败:', error)
    MessagePlugin.error(error.message || t('auth.oidcLoginFailed'))
  } finally {
    oidcLoading.value = false
  }
}

const acceptAndEnter = async (token: string) => {
  loading.value = true
  try {
    const result = await authStore.acceptInvitationByTokenAndRefresh(token)
    if (result.ok) {
      MessagePlugin.success(t('inviteRegister.joined'))
    } else {
      MessagePlugin.warning(t('inviteRegister.invalidBody'))
    }
  } catch {
    MessagePlugin.warning(t('inviteRegister.invalidBody'))
  } finally {
    loading.value = false
    await nextTick()
    router.replace('/platform/creatChat')
  }
}

const handleLogin = async () => {
  try {
    const valid = await formRef.value?.validate()
    if (valid !== true) return

    loading.value = true

    const response = await login({
      email: formData.email,
      password: formData.password,
    })

    if (response.success) {
      if (inviteToken.value) {
        await persistLoginResponse(response, true)
        await acceptAndEnter(inviteToken.value)
        return
      }
      await persistLoginResponse(response)
      notifyLoginSuccess(response, t, tm, formatRole, roleIcon)
    } else {
      MessagePlugin.error(response.message || t('auth.loginError'))
    }
  } catch (error: any) {
    console.error('登录错误:', error)
    MessagePlugin.error(error.message || t('auth.loginErrorRetry'))
  } finally {
    loading.value = false
  }
}

const handleRegister = async () => {
  if (!registrationEnabled.value) return
  try {
    const valid = await registerFormRef.value?.validate()
    if (valid !== true) return

    loading.value = true

    if (inviteToken.value) {
      const response = await registerByInvite({
        token: inviteToken.value,
        username: registerData.username,
        email: registerData.email,
        password: registerData.password,
      })
      if (!response.success) {
        MessagePlugin.error(response.message || t('auth.registerFailed'))
        return
      }
      MessagePlugin.success(t('auth.registerSuccess'))
      await persistLoginResponse(response)
      return
    }

    const response = await register({
      username: registerData.username,
      email: registerData.email,
      password: registerData.password,
    })

    if (response.success) {
      MessagePlugin.success(t('auth.registerSuccess'))
      isRegisterMode.value = false
      formData.email = registerData.email
      Object.keys(registerData).forEach((key) => {
        ;(registerData as any)[key] = ''
      })
    } else {
      MessagePlugin.error(response.message || t('auth.registerFailed'))
    }
  } catch (error: any) {
    console.error('注册错误:', error)
    MessagePlugin.error(error.message || t('auth.registerError'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  prefersDark.addEventListener('change', onSystemThemeChange)
  const tokenFromQuery = String(route.query.token || '').trim()
  if (tokenFromQuery) {
    inviteToken.value = tokenFromQuery
    inviteLookupLoading.value = true
    try {
      const resp = await getInvitationByToken(tokenFromQuery)
      if (resp.success && resp.data) {
        inviteLookup.value = resp.data
      } else {
        inviteLookupError.value =
          resp.message || t('inviteRegister.invalidBody')
        loadOIDCConfig()
        loadAuthConfig()
        return
      }
    } catch {
      inviteLookupError.value = t('inviteRegister.invalidBody')
      loadOIDCConfig()
      loadAuthConfig()
      return
    } finally {
      inviteLookupLoading.value = false
    }

    // 2. 已登录则直接兑换 token 进入空间（三种模式通用）。
    if (authStore.isLoggedIn && (await authStore.refreshFromAuthMe())) {
      await acceptAndEnter(tokenFromQuery)
      return
    }

    // 3. Only enabled registration modes may create an invited account.
    await loadAuthConfig()
    isRegisterMode.value = registrationEnabled.value
    loadOIDCConfig()
    return
  }

  if (authStore.isLoggedIn) {
    router.replace('/platform/creatChat')
    return
  }

  localStorage.removeItem('weknora_auto_setup_failed')
  try {
    const response = await autoSetup()
    if (response.success) {
      authStore.setLiteMode(true)
      await persistLoginResponse(response)
      return
    }
  } catch {
    // Auto-setup may be unavailable outside the native Lite shell.
  }

  loadOIDCConfig()
  loadAuthConfig()
})

onBeforeUnmount(() => {
  prefersDark.removeEventListener('change', onSystemThemeChange)
})
</script>

<style lang="less" scoped>
/* ---------- 页面骨架 ---------- */
.auth-page {
  position: relative;
  display: flex;
  min-height: 100dvh;
  width: 100%;
  background: var(--td-bg-color-page);
  font-family: var(--app-font-family);
  overflow: hidden;
}

/* ---------- 右上角：语言 / 主题（登录前也可用）---------- */
.auth-toolbar {
  position: absolute;
  top: var(--app-space-5);
  right: var(--app-space-6);
  z-index: var(--z-sticky);
  display: flex;
  align-items: center;
  gap: var(--app-space-2);
}

.auth-toolbar__btn {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-2);
  padding: var(--app-space-2) var(--app-space-3);
  border: 1px solid transparent;
  border-radius: var(--app-radius-md);
  background: transparent;
  color: var(--td-text-color-secondary);
  font-family: inherit;
  font-size: var(--app-text-md);
  cursor: pointer;
  transition:
    background-color var(--app-motion-fast) ease,
    border-color var(--app-motion-fast) ease,
    color var(--app-motion-fast) ease;

  &:hover {
    background: var(--td-bg-color-container-hover);
    border-color: var(--td-component-stroke);
    color: var(--td-text-color-primary);
  }

  &:focus-visible {
    outline: 2px solid var(--td-text-color-secondary);
    outline-offset: 2px;
  }
}

.auth-toolbar__btn--icon {
  padding: var(--app-space-2);
}

.auth-toolbar__label {
  white-space: nowrap;
}

/* The entry screen shares the welcome page's paper and charcoal surfaces. */
.brand-panel {
  position: relative;
  flex: 0 0 46%;
  min-width: 0;
  background: var(--td-bg-color-sidebar);
  color: var(--td-text-color-primary);
  border-right: 1px solid var(--td-component-stroke);
  display: flex;
  align-items: center;
  padding: 44px clamp(32px, 5vw, 76px);
  box-sizing: border-box;
}

.brand-panel__inner {
  position: relative;
  z-index: 2;
  width: 100%;
  max-width: 460px;
  display: flex;
  flex-direction: column;
  min-height: calc(100dvh - 88px);
}

/* 品牌标记沿用侧栏与「新对话」首页那套三道圆角短线。 */
.brand-lockup {
  display: flex;
  align-items: center;
  gap: var(--app-space-3);
  margin-bottom: var(--app-space-10);
}

.brand-mark {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  height: 28px;
  flex-shrink: 0;
  color: var(--td-text-color-primary);
  transform: rotate(-12deg);

  > span {
    display: block;
    width: 5px;
    background: currentColor;
    border-radius: var(--app-radius-pill);

    &:nth-child(1) {
      height: 18px;
    }

    &:nth-child(2) {
      height: 28px;
    }

    &:last-child {
      height: 14px;
      align-self: flex-end;
    }
  }
}

.brand-lockup__name {
  font-size: var(--app-text-3xl);
  font-weight: 700;
  letter-spacing: -0.3px;
  color: var(--td-text-color-primary);
}

/* 卡片眉标（品牌字标），登录与注册共用。 */
.auth-card__eyebrow {
  font-size: var(--app-text-xs);
  line-height: 1;
  letter-spacing: 0.16em;
  font-weight: 700;
}

.brand-copy {
  margin-bottom: 42px;
}

.brand-copy__title {
  font-size: clamp(34px, 4vw, 48px);
  font-family: var(--app-font-family-display);
  line-height: 1.3;
  font-weight: 500;
  letter-spacing: -1.5px;
  margin: 0 0 18px 0;
  color: var(--td-text-color-primary);
  text-wrap: balance;
}

.brand-copy__desc {
  font-size: var(--app-text-xl);
  line-height: 1.7;
  color: var(--td-text-color-secondary);
  margin: 0;
  max-width: 390px;
}

.brand-features {
  list-style: none;
  padding: 0;
  margin: 0 0 40px 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.brand-features__item {
  display: flex;
  align-items: center;
  gap: var(--app-space-3);
  font-size: var(--app-text-lg);
  color: var(--td-text-color-secondary);
}

.brand-features__check {
  flex-shrink: 0;
  color: var(--td-brand-color);
}

.brand-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: auto;
  margin-bottom: 20px;
}

.brand-tag {
  display: inline-block;
  padding: 7px 12px;
  border-radius: var(--app-radius-md);
  border: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);
  font-size: var(--app-text-md);
  font-weight: 500;
  color: var(--td-text-color-secondary);
}

/* ---------- 右侧表单区 ---------- */
.form-panel {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: var(--app-space-10) clamp(24px, 6vw, 96px);
  box-sizing: border-box;
  background: var(--td-bg-color-page);
}

/* 表单区右上角的主题切换（浅色/深色/跟随系统） */
.form-panel__theme {
  position: absolute;
  top: 20px;
  right: 24px;
  z-index: 2;
}

.form-panel__inner {
  width: 100%;
  max-width: 430px;
  position: relative;
  z-index: 1;
}

/* 暗色下 --td-bg-color-container 与页面同为 #141414，卡片会与背景同色而「消失」。
   这里把卡片表面收进一个组件内变量：浅色仍是白卡，暗色抬到 sidebar 层。
   暗色那一半写在文件末尾的非 scoped 样式块里 —— Vue 的 :global() 表达不了
   「:root[theme-mode] + 后代」这种选择器（会静默丢规则或丢后代）。 */
.auth-card {
  --login-card-surface: var(--td-bg-color-container);

  width: 100%;
  padding: var(--app-space-10) var(--app-space-8) var(--app-space-8);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-2xl);
  background: var(--login-card-surface);
  box-shadow: var(--app-surface-shadow);
  box-sizing: border-box;
}

.auth-card__header {
  margin-bottom: 26px;
}

.auth-card__eyebrow {
  color: var(--td-brand-color);
  margin-bottom: 18px;
}

.auth-card__title {
  font-family: var(--app-font-family-display);
  font-size: clamp(24px, 2.2vw, 30px);
  font-weight: 500;
  letter-spacing: -1px;
  color: var(--td-text-color-primary);
  margin: 0 0 8px 0;
  line-height: 1.25;
}

.auth-card__subtitle {
  font-size: var(--app-text-lg);
  color: var(--td-text-color-secondary);
  margin: 0;
  line-height: 1.5;
}

.auth-card__hint {
  margin: 0 0 24px 0;
  padding: 10px 14px;
  border-radius: var(--app-radius-xl);
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-md);
  line-height: 1.5;
}

.auth-card__footer {
  margin-top: 24px;
  text-align: center;
  font-size: var(--app-text-base);
  color: var(--td-text-color-secondary);
}

.auth-link {
  color: var(--td-brand-color);
  text-decoration: none;
  font-weight: 600;
  margin-left: 4px;

  &:hover {
    text-decoration: underline;
  }
}

/* ---------- 表单 ---------- */
.auth-form {
  :deep(.t-form-item__label) {
    font-size: var(--app-text-base);
    font-weight: 500;
    color: var(--td-text-color-primary);
    margin-bottom: var(--app-space-2);
    padding: 0;
  }

  :deep(.t-form-item) {
    margin-bottom: var(--app-space-5);
  }

  :deep(.t-input) {
    border-radius: var(--app-radius-xl);
    border-color: var(--td-component-border);
    background: var(--td-bg-color-container);
    transition:
      border-color var(--app-motion-fast) ease,
      box-shadow var(--app-motion-fast) ease;

    &:hover {
      border-color: var(--td-text-color-placeholder);
    }

    &:focus-within {
      border-color: var(--td-brand-color);
      box-shadow: 0 0 0 3px var(--td-brand-color-focus);
    }
  }

  :deep(.t-input__inner) {
    font-size: var(--app-text-lg);
  }
}

/* 配色交给 TDesign 与工作区令牌：theme="primary" 取 --td-brand-color
   （浅色炭黑 / 暗色浅灰），文字色由 workspace-theme 的 --app-on-accent 保证对比。
   这里只管尺寸、节奏与焦点态，因此不再需要强制覆盖。 */
.auth-form :deep(.t-button.btn) {
  height: 50px;
  border-radius: var(--app-radius-xl);
  font-size: var(--app-text-lg);
  font-weight: 600;
  transition:
    background-color var(--app-motion-base) ease,
    border-color var(--app-motion-base) ease,
    color var(--app-motion-base) ease,
    transform var(--app-motion-base) ease;

  & + .btn {
    margin-top: var(--app-space-3);
  }

  &:focus-visible {
    outline: 2px solid var(--td-text-color-secondary);
    outline-offset: 2px;
  }
}

.auth-form :deep(.t-button.btn--submit) {
  margin-top: var(--app-space-8);
}

.auth-divider {
  position: relative;
  text-align: center;
  margin: var(--app-space-5) 0 var(--app-space-4);
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-md);

  span {
    position: relative;
    z-index: 1;
    padding: 0 var(--app-space-3);
    /* 必须是卡片表面色，才能遮住下面的分隔线（暗色下卡片不等于 container）。 */
    background: var(--login-card-surface);
  }

  &::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 50%;
    border-top: 1px solid var(--td-component-stroke);
  }
}

/* ---------- 邀请提示 ---------- */
.invite-banner {
  display: flex;
  align-items: flex-start;
  gap: var(--app-space-3);
  padding: var(--app-space-3) var(--app-space-4);
  margin-bottom: var(--app-space-6);
  border-radius: var(--app-radius-lg);
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);
}

.invite-banner__icon {
  margin-top: 2px;
  font-size: var(--app-text-2xl);
  flex-shrink: 0;
  color: var(--td-brand-color);
}

.invite-banner__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.invite-banner__title {
  font-size: var(--app-text-base);
  font-weight: 600;
  line-height: 1.4;
  color: var(--td-text-color-primary);
}

.invite-banner__hint {
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}

.invite-banner--error {
  background: var(--td-error-color-light);
  border-color: color-mix(in srgb, var(--td-error-color) 30%, transparent);
  color: var(--td-error-color);
  font-size: var(--app-text-base);
}

/* ---------- 响应式 ---------- */
@media (max-width: 960px) {
  .auth-page {
    flex-direction: column;
  }

  /* 单列布局下工具栏回到正常流，成为页首一行，不再压在品牌面板上。 */
  .auth-toolbar {
    position: static;
    justify-content: flex-end;
    padding: var(--app-space-3) var(--app-space-5) 0;
  }

  .brand-panel {
    flex: 0 0 auto;
    min-width: 0;
    padding: var(--app-space-8) var(--app-space-8) var(--app-space-10);
  }

  .brand-panel__inner {
    min-height: 0;
  }

  .brand-lockup {
    margin-bottom: var(--app-space-6);
  }

  .brand-copy {
    margin-bottom: var(--app-space-6);

    &__title {
      font-size: var(--app-text-4xl);
    }
  }

  .brand-features {
    margin-bottom: var(--app-space-6);
  }

  .form-panel {
    padding: var(--app-space-10) var(--app-space-6);
    align-items: flex-start;
  }

  .auth-card {
    padding: var(--app-space-8);
  }
}

@media (max-width: 520px) {
  .auth-toolbar {
    padding: var(--app-space-2) var(--app-space-4) 0;
  }

  /* 极窄屏只留图标，避免语言名把工具栏撑到换行。 */
  .auth-toolbar__label {
    display: none;
  }

  .brand-panel {
    padding: var(--app-space-8) var(--app-space-5);
  }

  .brand-panel__inner {
    min-height: auto;
  }

  .brand-lockup {
    margin-bottom: var(--app-space-5);
  }

  .brand-copy__title {
    font-size: var(--app-text-4xl);
  }

  .brand-copy__desc {
    font-size: var(--app-text-base);
  }

  .form-panel {
    padding: var(--app-space-8) var(--app-space-5);
  }

  .auth-card__title {
    font-size: var(--app-text-4xl);
  }

  .auth-card {
    padding: var(--app-space-6) var(--app-space-5);
    border-radius: var(--app-radius-xl);
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-toolbar__btn,
  .auth-form :deep(.t-button.btn) {
    transition: none;
  }
}
</style>

<!--
  非 scoped：只有跨整棵树的 :root[theme-mode="dark"] 前置条件需要它。
  .auth-page 只在本文件使用，选择器仍被这一页收住。
  Vue 的 :global() 无法表达「:root[theme-mode] + 后代」：::global(sel .x) 会被
  静默丢弃，:global(sel) .x 会把 .x 丢掉，两种写法都得不到想要的规则。
-->
<style lang="less">
:root:root[theme-mode='dark'] .auth-page .auth-card {
  --login-card-surface: var(--td-bg-color-sidebar);
}
</style>

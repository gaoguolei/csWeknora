<template>
  <div class="auth-page">
    <!-- 左侧品牌面板 -->
    <aside class="brand-panel">
      <div class="brand-panel__glow brand-panel__glow--top"></div>
      <div class="brand-panel__glow brand-panel__glow--bottom"></div>

      <div class="brand-panel__inner">
        <div class="brand-panel__eyebrow">KNOWLEDGE / AGENT / WORKSPACE</div>

        <div class="brand-copy">
          <h1 class="brand-copy__title">{{ $t("platform.subtitle") }}</h1>
          <p class="brand-copy__desc">{{ $t("platform.description") }}</p>
        </div>

        <ul class="brand-features">
          <li class="brand-features__item">
            <span class="brand-features__check">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="3"
              >
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </span>
            <span>{{ $t("platform.multimodalParsing") }}</span>
          </li>
          <li class="brand-features__item">
            <span class="brand-features__check">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="3"
              >
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </span>
            <span>{{ $t("platform.hybridSearchEngine") }}</span>
          </li>
          <li class="brand-features__item">
            <span class="brand-features__check">
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="3"
              >
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </span>
            <span>{{ $t("platform.ragQandA") }}</span>
          </li>
        </ul>

        <div class="brand-tags">
          <span class="brand-tag">{{ $t("platform.rag") }}</span>
          <span class="brand-tag">{{ $t("platform.agent") }}</span>
          <span class="brand-tag">{{ $t("platform.wiki") }}</span>
          <span class="brand-tag">{{ $t("platform.hybridSearch") }}</span>
        </div>

        <div class="brand-panel__meta">
          <span class="brand-panel__meta-dot"></span>
          <span>Private knowledge, ready to work</span>
        </div>
      </div>
    </aside>

    <!-- 右侧表单区 -->
    <main class="form-panel">
      <div class="form-panel__inner">
        <!-- 登录 -->
        <div v-if="!isRegisterMode" class="auth-card">
          <div v-if="inviteLookup" class="invite-banner">
            <t-icon name="link" class="invite-banner__icon" />
            <div class="invite-banner__text">
              <div class="invite-banner__title">
                {{
                  $t("inviteRegister.bannerTitle", {
                    tenant: inviteLookup.tenant_name || "",
                  })
                }}
              </div>
              <div class="invite-banner__hint">
                {{ $t("inviteRegister.bannerHintLogin") }}
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
            <div class="auth-card__eyebrow">WEKNORA WORKSPACE</div>
            <h2 class="auth-card__title">{{ $t("auth.login") }}</h2>
            <p class="auth-card__subtitle">{{ $t("auth.subtitle") }}</p>
          </header>

          <p v-if="registrationEnabled" class="auth-card__hint">
            {{ $t("auth.loginHint") }}
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
              theme="default"
              type="submit"
              size="large"
              block
              :loading="loading"
              class="btn btn--primary"
            >
              {{ loading ? $t("auth.loggingIn") : $t("auth.login") }}
            </t-button>

            <t-button
              v-if="registrationEnabled"
              theme="default"
              variant="outline"
              size="large"
              block
              :disabled="loading"
              class="btn btn--ghost"
              @click="toggleMode"
            >
              {{ $t("auth.createAccount") }}
            </t-button>

            <template v-if="oidcEnabled">
              <div class="auth-divider">
                <span>{{ $t("auth.orContinueWith") }}</span>
              </div>
              <t-button
                theme="default"
                size="large"
                block
                :loading="oidcLoading"
                :disabled="loading"
                class="btn btn--oidc"
                @click="handleOIDCLogin"
              >
                {{ oidcLoading ? $t("auth.redirectingToOIDC") : oidcLoginText }}
              </t-button>
            </template>
          </t-form>
        </div>

        <!-- 注册 -->
        <div v-else class="auth-card">
          <div v-if="inviteLookup" class="invite-banner">
            <t-icon name="link" class="invite-banner__icon" />
            <div class="invite-banner__text">
              <div class="invite-banner__title">
                {{
                  $t("inviteRegister.bannerTitle", {
                    tenant: inviteLookup.tenant_name || "",
                  })
                }}
              </div>
              <div class="invite-banner__hint">
                {{ $t("inviteRegister.bannerHint") }}
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
            <div class="auth-card__eyebrow">WEKNORA WORKSPACE</div>
            <h2 class="auth-card__title">{{ $t("auth.createAccount") }}</h2>
            <p class="auth-card__subtitle">{{ $t("auth.subtitle") }}</p>
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
              theme="default"
              type="submit"
              size="large"
              block
              :loading="loading"
              class="btn btn--primary"
            >
              {{ loading ? $t("auth.registering") : $t("auth.register") }}
            </t-button>
          </t-form>

          <div class="auth-card__footer">
            <span>{{ $t("auth.haveAccount") }}</span>
            <a href="#" @click.prevent="toggleMode" class="auth-link">
              {{ $t("auth.backToLogin") }}
            </a>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, nextTick, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { MessagePlugin } from "tdesign-vue-next";
import { useRoleLabel } from "@/composables/useRoleLabel";
import { notifyLoginSuccess } from "@/utils/loginNotify";
import { newPasswordRules } from "@/utils/passwordPolicy";
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
} from "@/api/auth";
import { useAuthStore } from "@/stores/auth";
import { useI18n } from "vue-i18n";

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const { t, tm } = useI18n();
const { formatRole, roleIcon } = useRoleLabel();

const formRef = ref();
const registerFormRef = ref();

const loading = ref(false);
const oidcLoading = ref(false);
const isRegisterMode = ref(false);
const oidcEnabled = ref(false);
const oidcProviderName = ref("");
const registrationEnabled = ref(true);
const complexPasswordEnabled = ref(false);

const inviteToken = ref("");
const inviteLookup = ref<InviteLookup | null>(null);
const inviteLookupError = ref("");
const inviteLookupLoading = ref(false);

const oidcLoginText = computed(() => {
  if (oidcProviderName.value) {
    return t("auth.oidcLoginWithProvider", {
      provider: oidcProviderName.value,
    });
  }
  return t("auth.oidcLogin");
});

const formData = reactive<{ [key: string]: any }>({
  email: "",
  password: "",
});

const registerData = reactive<{ [key: string]: any }>({
  username: "",
  email: "",
  password: "",
  confirmPassword: "",
});

const formRules = computed(() => ({
  email: [
    { required: true, message: t("auth.emailRequired"), type: "error" },
    { email: true, message: t("auth.emailInvalid"), type: "error" },
  ],
  password: [
    { required: true, message: t("auth.passwordRequired"), type: "error" },
    { min: 8, message: t("auth.passwordMinLength"), type: "error" },
    { max: 32, message: t("auth.passwordMaxLength"), type: "error" },
  ],
}));

const registerRules = computed(() => ({
  username: [
    { required: true, message: t("auth.usernameRequired"), type: "error" },
    { min: 2, message: t("auth.usernameMinLength"), type: "error" },
    { max: 20, message: t("auth.usernameMaxLength"), type: "error" },
    {
      pattern: /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/,
      message: t("auth.usernameInvalid"),
      type: "error",
    },
  ],
  email: [
    { required: true, message: t("auth.emailRequired"), type: "error" },
    { email: true, message: t("auth.emailInvalid"), type: "error" },
  ],
  password: newPasswordRules(t, complexPasswordEnabled.value),
  confirmPassword: [
    {
      required: true,
      message: t("auth.confirmPasswordRequired"),
      type: "error",
    },
    {
      validator: (val: string) => val === registerData.password,
      message: t("auth.passwordMismatch"),
      type: "error",
    },
  ],
}));

const toggleMode = () => {
  isRegisterMode.value = !isRegisterMode.value;
  Object.keys(registerData).forEach((key) => {
    (registerData as any)[key] = "";
  });
};

const persistLoginResponse = async (response: any, skipRedirect = false) => {
  const activeTenant = response.active_tenant || response.tenant;
  if (response.user && response.token) {
    const homeTenantIdRaw = response.user.tenant_id ?? activeTenant?.id ?? "";
    authStore.setUser(userInfoFromApi(response.user, homeTenantIdRaw));
    authStore.setToken(response.token);
    if (response.refresh_token) {
      authStore.setRefreshToken(response.refresh_token);
    }
    if (activeTenant) {
      authStore.setTenant({
        id: String(activeTenant.id) || "",
        name: activeTenant.name || "",
        owner_id: response.user.id || "",
        created_at: activeTenant.created_at || new Date().toISOString(),
        updated_at: activeTenant.updated_at || new Date().toISOString(),
      });
    } else {
      authStore.setTenant(null);
    }
    if (Array.isArray(response.memberships)) {
      authStore.setMemberships(response.memberships);
    }
    const activeIdNum = Number(activeTenant?.id);
    const homeIdNum = Number(homeTenantIdRaw);
    if (
      Number.isFinite(activeIdNum) &&
      Number.isFinite(homeIdNum) &&
      activeIdNum !== homeIdNum
    ) {
      authStore.setSelectedTenant(activeIdNum, activeTenant?.name || null);
    } else {
      authStore.setSelectedTenant(null, null);
    }
  }

  await authStore.refreshFromAuthMe();
  await nextTick();
  if (skipRedirect) return;
  router.replace(
    authStore.hasValidTenant
      ? "/platform/creatChat"
      : "/onboarding/workspace",
  );
};

const getBackendOIDCRedirectURI = () =>
  `${window.location.origin}/api/v1/auth/oidc/callback`;

const loadOIDCConfig = async () => {
  try {
    const response = await getOIDCConfig();
    oidcEnabled.value = !!response.success && !!response.enabled;
    oidcProviderName.value = response.provider_display_name || "";
  } catch {
    oidcEnabled.value = false;
    oidcProviderName.value = "";
  }
};

const loadAuthConfig = async () => {
  try {
    const response = await getAuthConfig();
    registrationEnabled.value = response.registration_mode !== "invite_only";
    complexPasswordEnabled.value = response.complex_password_enabled;
  } catch {
    registrationEnabled.value = true;
    complexPasswordEnabled.value = false;
  }
};

const handleOIDCLogin = async () => {
  try {
    oidcLoading.value = true;
    const response = await getOIDCAuthorizationURL(getBackendOIDCRedirectURI());
    const authorizationURL = response.authorization_url;

    if (!response.success || !authorizationURL) {
      MessagePlugin.error(response.message || t("auth.oidcLoginFailed"));
      return;
    }

    if (inviteToken.value) {
      sessionStorage.setItem("weknora_pending_invite_token", inviteToken.value);
    }
    window.location.href = authorizationURL;
  } catch (error: any) {
    console.error("OIDC 登录跳转失败:", error);
    MessagePlugin.error(error.message || t("auth.oidcLoginFailed"));
  } finally {
    oidcLoading.value = false;
  }
};

const acceptAndEnter = async (token: string) => {
  loading.value = true;
  try {
    const result = await authStore.acceptInvitationByTokenAndRefresh(token);
    if (result.ok) {
      MessagePlugin.success(t("inviteRegister.joined"));
    } else {
      MessagePlugin.warning(t("inviteRegister.invalidBody"));
    }
  } catch {
    MessagePlugin.warning(t("inviteRegister.invalidBody"));
  } finally {
    loading.value = false;
    await nextTick();
    router.replace("/platform/creatChat");
  }
};

const handleLogin = async () => {
  try {
    const valid = await formRef.value?.validate();
    if (valid !== true) return;

    loading.value = true;

    const response = await login({
      email: formData.email,
      password: formData.password,
    });

    if (response.success) {
      if (inviteToken.value) {
        await persistLoginResponse(response, true);
        await acceptAndEnter(inviteToken.value);
        return;
      }
      await persistLoginResponse(response);
      notifyLoginSuccess(response, t, tm, formatRole, roleIcon);
    } else {
      MessagePlugin.error(response.message || t("auth.loginError"));
    }
  } catch (error: any) {
    console.error("登录错误:", error);
    MessagePlugin.error(error.message || t("auth.loginErrorRetry"));
  } finally {
    loading.value = false;
  }
};

const handleRegister = async () => {
  try {
    const valid = await registerFormRef.value?.validate();
    if (valid !== true) return;

    loading.value = true;

    if (inviteToken.value) {
      const response = await registerByInvite({
        token: inviteToken.value,
        username: registerData.username,
        email: registerData.email,
        password: registerData.password,
      });
      if (!response.success) {
        MessagePlugin.error(response.message || t("auth.registerFailed"));
        return;
      }
      MessagePlugin.success(t("auth.registerSuccess"));
      await persistLoginResponse(response);
      return;
    }

    const response = await register({
      username: registerData.username,
      email: registerData.email,
      password: registerData.password,
    });

    if (response.success) {
      MessagePlugin.success(t("auth.registerSuccess"));
      isRegisterMode.value = false;
      formData.email = registerData.email;
      Object.keys(registerData).forEach((key) => {
        (registerData as any)[key] = "";
      });
    } else {
      MessagePlugin.error(response.message || t("auth.registerFailed"));
    }
  } catch (error: any) {
    console.error("注册错误:", error);
    MessagePlugin.error(error.message || t("auth.registerError"));
  } finally {
    loading.value = false;
  }
};

onMounted(async () => {
  const tokenFromQuery = String(route.query.token || "").trim();
  if (tokenFromQuery) {
    inviteToken.value = tokenFromQuery;
    inviteLookupLoading.value = true;
    try {
      const resp = await getInvitationByToken(tokenFromQuery);
      if (resp.success && resp.data) {
        inviteLookup.value = resp.data;
      } else {
        inviteLookupError.value =
          resp.message || t("inviteRegister.invalidBody");
        loadOIDCConfig();
        loadAuthConfig();
        return;
      }
    } catch {
      inviteLookupError.value = t("inviteRegister.invalidBody");
      loadOIDCConfig();
      loadAuthConfig();
      return;
    } finally {
      inviteLookupLoading.value = false;
    }

    if (authStore.isLoggedIn && (await authStore.refreshFromAuthMe())) {
      await acceptAndEnter(tokenFromQuery);
      return;
    }

    const cfg = await getAuthConfig();
    const inviteOnly = cfg.registration_mode === "invite_only";
    registrationEnabled.value = !inviteOnly;
    isRegisterMode.value = !inviteOnly;
    loadOIDCConfig();
    return;
  }

  if (authStore.isLoggedIn) {
    router.replace("/platform/creatChat");
    return;
  }

  const AUTO_SETUP_FAILED_KEY = "weknora_auto_setup_failed";
  if (localStorage.getItem(AUTO_SETUP_FAILED_KEY) !== "true") {
    try {
      const response = await autoSetup();
      if (response.success) {
        authStore.setLiteMode(true);
        await persistLoginResponse(response);
        return;
      } else {
        localStorage.setItem(AUTO_SETUP_FAILED_KEY, "true");
      }
    } catch {
      localStorage.setItem(AUTO_SETUP_FAILED_KEY, "true");
    }
  }

  loadOIDCConfig();
  loadAuthConfig();
});
</script>

<style lang="less" scoped>
/* ---------- 页面骨架 ---------- */
.auth-page {
  display: flex;
  min-height: 100dvh;
  width: 100%;
  background: #f5f7fb;
  font-family: var(--app-font-family);
  overflow: hidden;
}

/* ---------- 左侧品牌面板 ---------- */
.brand-panel {
  position: relative;
  flex: 0 0 46%;
  min-width: 380px;
  background:
    radial-gradient(
      circle at 82% 14%,
      rgba(80, 122, 255, 0.2),
      transparent 32%
    ),
    radial-gradient(
      circle at 14% 88%,
      rgba(58, 99, 230, 0.16),
      transparent 36%
    ),
    #0b1120;
  color: #f1f5f9;
  overflow: hidden;
  display: flex;
  align-items: center;
  padding: 44px clamp(32px, 5vw, 76px);
  box-sizing: border-box;

  &::before {
    content: "";
    position: absolute;
    inset: 0;
    opacity: 0.18;
    pointer-events: none;
    background-image:
      linear-gradient(rgba(255, 255, 255, 0.08) 1px, transparent 1px),
      linear-gradient(90deg, rgba(255, 255, 255, 0.08) 1px, transparent 1px);
    background-size: 64px 64px;
    mask-image: linear-gradient(to bottom, black, transparent 76%);
  }
}

.brand-panel__glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(120px);
  pointer-events: none;
}

.brand-panel__glow--top {
  width: 480px;
  height: 480px;
  top: -180px;
  right: -160px;
  background: rgba(80, 122, 255, 0.28);
}

.brand-panel__glow--bottom {
  width: 420px;
  height: 420px;
  bottom: -200px;
  left: -120px;
  background: rgba(56, 130, 246, 0.22);
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

.brand-logo {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 22px;
}

.brand-logo__mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: var(--app-radius-xl);
  background: #dbe4ff;
  color: #0b1120;
  font-weight: 800;
  font-size: var(--app-text-4xl);
  letter-spacing: -0.7px;
  box-shadow: 0 0 0 6px rgba(219, 228, 255, 0.08);
}

.brand-logo__text {
  font-size: var(--app-text-4xl);
  font-weight: 700;
  letter-spacing: -0.3px;
  color: #f8fafc;
}

.brand-panel__eyebrow,
.auth-card__eyebrow {
  font-size: var(--app-text-xs);
  line-height: 1;
  letter-spacing: 0.16em;
  font-weight: 700;
}

.brand-panel__eyebrow {
  color: rgba(179, 196, 255, 0.68);
  margin-bottom: 74px;
}

.brand-copy {
  margin-bottom: 42px;
}

.brand-copy__title {
  font-size: clamp(34px, 4vw, 48px);
  line-height: 1.08;
  font-weight: 700;
  letter-spacing: -1.5px;
  margin: 0 0 18px 0;
  color: #ffffff;
  text-wrap: nowrap;
}

.brand-copy__desc {
  font-size: var(--app-text-xl);
  line-height: 1.7;
  color: rgba(241, 245, 249, 0.68);
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
  gap: 12px;
  font-size: var(--app-text-lg);
  color: rgba(241, 245, 249, 0.85);
}

.brand-features__check {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgba(80, 122, 255, 0.16);
  color: #6f92ff;
  flex-shrink: 0;

  svg {
    width: 12px;
    height: 12px;
  }
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
  border: 1px solid rgba(241, 245, 249, 0.16);
  background: rgba(241, 245, 249, 0.06);
  font-size: var(--app-text-md);
  font-weight: 500;
  color: rgba(241, 245, 249, 0.78);
}

.brand-panel__meta {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: var(--app-text-sm);
  color: rgba(241, 245, 249, 0.42);
}

.brand-panel__meta-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #6f92ff;
  box-shadow: 0 0 0 4px rgba(111, 146, 255, 0.1);
}

.brand-footer {
  margin-top: 48px;
  font-size: var(--app-text-md);
  color: rgba(241, 245, 249, 0.38);
}

/* ---------- 右侧表单区 ---------- */
.form-panel {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: 48px clamp(24px, 6vw, 96px);
  box-sizing: border-box;
  background:
    radial-gradient(
      circle at 82% 18%,
      rgba(219, 228, 255, 0.62),
      transparent 28%
    ),
    #f6f8fc;

  &::after {
    content: "";
    position: absolute;
    right: 8%;
    bottom: 10%;
    width: 160px;
    height: 160px;
    border: 1px solid rgba(30, 58, 138, 0.08);
    border-radius: var(--app-radius-xl);
    transform: rotate(18deg);
    pointer-events: none;
  }
}

.form-panel__inner {
  width: 100%;
  max-width: 430px;
  position: relative;
  z-index: 1;
}

.auth-card {
  width: 100%;
  padding: 40px 42px 36px;
  border: 1px solid rgba(23, 37, 84, 0.1);
  border-radius: var(--app-radius-pill);
  background: rgba(255, 255, 255, 0.86);
  box-shadow: 0 22px 60px rgba(15, 23, 42, 0.08);
  backdrop-filter: blur(18px);
  box-sizing: border-box;
}

.auth-card__header {
  margin-bottom: 26px;
}

.auth-card__eyebrow {
  color: #3a63e6;
  margin-bottom: 18px;
}

.auth-card__title {
  font-size: var(--app-text-4xl);
  font-weight: 700;
  letter-spacing: -1px;
  color: #0f172a;
  margin: 0 0 8px 0;
  line-height: 1.25;
}

.auth-card__subtitle {
  font-size: var(--app-text-lg);
  color: #64748b;
  margin: 0;
  line-height: 1.5;
}

.auth-card__hint {
  margin: 0 0 24px 0;
  padding: 10px 14px;
  border-radius: var(--app-radius-xl);
  background: #eef2ff;
  border: 1px solid #d6deff;
  color: #2b4cc0;
  font-size: var(--app-text-md);
  line-height: 1.5;
}

.auth-card__footer {
  margin-top: 24px;
  text-align: center;
  font-size: var(--app-text-base);
  color: #64748b;
}

.auth-link {
  color: #3a63e6;
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
    color: #334155;
    margin-bottom: 6px;
    padding: 0;
  }

  :deep(.t-form-item) {
    margin-bottom: 20px;
  }

  :deep(.t-input) {
    border-radius: var(--app-radius-xl);
    border-color: #dbe2ec;
    background: rgba(255, 255, 255, 0.72);
    transition:
      border-color var(--app-motion-base) ease,
      box-shadow 0.18s ease;

    &:hover {
      border-color: #94a3b8;
    }

    &:focus-within {
      border-color: #507aff;
      box-shadow: 0 0 0 3px rgba(80, 122, 255, 0.14);
    }
  }

  :deep(.t-input__inner) {
    font-size: var(--app-text-lg);
  }
}

.btn {
  height: 50px;
  border-radius: var(--app-radius-xl);
  font-size: var(--app-text-lg);
  font-weight: 600;
  transition: all var(--app-motion-base) ease;

  & + .btn {
    margin-top: 12px;
  }
}

.btn--primary {
  background: #142138 !important;
  border-color: #142138 !important;
  border: none;
  color: #ffffff !important;
  margin-top: 30px;

  &:hover,
  &:focus {
    background: #1e3a5f !important;
    border-color: #1e3a5f !important;
    color: #ffffff !important;
    transform: translateY(-1px);
  }

  &:active {
    background: #0d1a33 !important;
    border-color: #0d1a33 !important;
    color: #ffffff !important;
    transform: translateY(0);
  }
}

.btn--ghost {
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid #dbe2ec;
  color: #0f172a;

  &:hover {
    border-color: #507aff;
    color: #2b4cc0;
    background: #eef2ff;
  }
}

.btn--oidc {
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid #dbe2ec;
  color: #334155;

  &:hover {
    border-color: #94a3b8;
    background: #f8fafc;
  }
}

.auth-divider {
  position: relative;
  text-align: center;
  margin: 22px 0 18px;
  color: #94a3b8;
  font-size: var(--app-text-md);

  span {
    position: relative;
    z-index: 1;
    padding: 0 12px;
    background: #ffffff;
  }

  &::before {
    content: "";
    position: absolute;
    left: 0;
    right: 0;
    top: 50%;
    border-top: 1px solid #e2e8f0;
  }
}

/* ---------- 邀请提示 ---------- */
.invite-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  margin-bottom: 24px;
  border-radius: var(--app-radius-lg);
  background: #eef2ff;
  border: 1px solid #d6deff;
}

.invite-banner__icon {
  margin-top: 2px;
  font-size: var(--app-text-2xl);
  flex-shrink: 0;
  color: #3a63e6;
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
  color: #1e3a8a;
}

.invite-banner__hint {
  font-size: var(--app-text-md);
  color: #2b4cc0;
  line-height: 1.5;
}

.invite-banner--error {
  background: #fef2f2;
  border-color: #fecaca;
  color: #b91c1c;
  font-size: var(--app-text-base);
}

/* ---------- 响应式 ---------- */
@media (max-width: 960px) {
  .auth-page {
    flex-direction: column;
  }

  .brand-panel {
    flex: 0 0 auto;
    min-width: 0;
    padding: 36px 32px 40px;
  }

  .brand-panel__inner {
    min-height: 0;
  }

  .brand-logo {
    margin-bottom: 18px;
  }

  .brand-panel__eyebrow {
    margin-bottom: 42px;
  }

  .brand-copy {
    margin-bottom: 28px;

    &__title {
      font-size: var(--app-text-4xl);
    }
  }

  .brand-features {
    margin-bottom: 24px;
  }

  .brand-footer {
    display: none;
  }

  .form-panel {
    padding: 48px 24px;
    align-items: flex-start;
  }

  .auth-card {
    padding: 34px 32px 30px;
  }
}

@media (max-width: 520px) {
  .brand-panel {
    padding: 32px 20px;
  }

  .brand-panel__inner {
    min-height: auto;
  }

  .brand-panel__eyebrow {
    margin-bottom: 34px;
  }

  .brand-copy__title {
    font-size: var(--app-text-4xl);
  }

  .brand-copy__desc {
    font-size: var(--app-text-base);
  }

  .form-panel {
    padding: 32px 20px;
  }

  .auth-card__title {
    font-size: var(--app-text-4xl);
  }

  .auth-card {
    padding: 30px 22px 26px;
    border-radius: var(--app-radius-xl);
  }
}
</style>

<style lang="less">
html[theme-mode="dark"] {
  .auth-page,
  .form-panel {
    background: #0a0f1a;
  }

  .form-panel {
    background:
      radial-gradient(
        circle at 82% 18%,
        rgba(40, 70, 150, 0.18),
        transparent 28%
      ),
      #0a0f1a;
  }

  .auth-card {
    background: rgba(15, 20, 34, 0.9);
    border-color: rgba(179, 196, 255, 0.14);
    box-shadow: 0 22px 60px rgba(0, 0, 0, 0.24);
  }

  .auth-card__eyebrow {
    color: #b3c4ff;
  }

  .auth-card__title {
    color: #f1f5f9;
  }

  .auth-card__subtitle,
  .auth-card__footer {
    color: #94a3b8;
  }

  .auth-card__hint {
    background: rgba(80, 122, 255, 0.12);
    border-color: rgba(80, 122, 255, 0.3);
    color: #b3c4ff;
  }

  .auth-link {
    color: #6f92ff;
  }

  .auth-divider {
    color: #64748b;

    span {
      background: #0f1420;
    }

    &::before {
      border-color: #1e2530;
    }
  }

  .auth-form .t-form-item__label {
    color: #cbd5e1;
  }

  .auth-form .t-input {
    background: #161a21;
    border-color: #1e2530;
    color: #f1f5f9;

    &:hover {
      border-color: #334155;
    }

    &:focus-within {
      border-color: #507aff;
    }
  }

  .btn--primary {
    background: #dbe4ff !important;
    border-color: #dbe4ff !important;
    color: #0f1e4f !important;

    &:hover,
    &:focus {
      background: #c3d2ff !important;
      border-color: #c3d2ff !important;
      color: #0f1e4f !important;
    }

    &:active {
      background: #a9bcff !important;
      border-color: #a9bcff !important;
      color: #0f1e4f !important;
    }
  }

  .btn--ghost,
  .btn--oidc {
    background: #161a21;
    border-color: #1e2530;
    color: #e2e8f0;

    &:hover {
      border-color: #507aff;
      color: #b3c4ff;
      background: #0f1a2e;
    }
  }

  .invite-banner {
    background: rgba(80, 122, 255, 0.1);
    border-color: rgba(80, 122, 255, 0.28);
  }

  .invite-banner__title {
    color: #dbe4ff;
  }

  .invite-banner__hint {
    color: #b3c4ff;
  }

  .brand-panel__meta {
    color: rgba(241, 245, 249, 0.5);
  }
}
</style>

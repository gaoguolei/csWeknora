<template>
  <IntegrationLandingLayout
    :title="$t('integrations.chrome.title')"
    :subtitle="$t('integrations.chrome.subtitle')"
    variant="chrome"
  >
    <template #tags>
      <span v-for="key in scenarioKeys" :key="key" class="scenario-tag">
        {{ $t(`integrations.chrome.scenarios.${key}`) }}
      </span>
    </template>

    <template #actions>
      <IntegrationExternalCta
        variant="chrome"
        :label="$t('integrations.chrome.installCta')"
        :hint="$t('integrations.chrome.installCtaHint')"
        @click="openChromeStore"
      >
        <template #icon>
          <t-icon name="extension" size="18px" />
        </template>
      </IntegrationExternalCta>
    </template>

    <template #main>
      <div class="landing-group">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">
            {{ $t('integrations.chrome.capabilitiesTitle') }}
            <span class="section-head-extra">{{ capabilityKeys.length }}</span>
          </h4>
          <div class="capability-grid">
            <div v-for="key in capabilityKeys" :key="key" class="capability-card">
              <div class="capability-card__icon">
                <t-icon :name="capabilityIcons[key]" />
              </div>
              <h5 class="capability-card__title">{{ $t(`integrations.chrome.capabilities.${key}.title`) }}</h5>
              <p class="capability-card__desc">{{ $t(`integrations.chrome.capabilities.${key}.desc`) }}</p>
            </div>
          </div>
        </section>
      </div>
    </template>

    <template #aside>
      <div class="landing-group">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t('integrations.chrome.stepsTitle') }}</h4>
          <ol class="landing-steps">
            <li v-for="(step, index) in stepKeys" :key="step" class="landing-step">
              <span class="landing-step-num">{{ index + 1 }}</span>
              <div class="landing-step-body">
                <div class="landing-step-title">{{ $t(`integrations.chrome.steps.${step}.title`) }}</div>
                <p class="landing-step-desc">{{ $t(`integrations.chrome.steps.${step}.desc`) }}</p>
                <t-button
                  v-if="step === 'api' && canOpenApiSettings"
                  size="small"
                  variant="outline"
                  class="landing-step-action"
                  @click="openApiSettings"
                >
                  {{ $t('integrations.chrome.openApiSettings') }}
                </t-button>
                <div v-if="step === 'connect'" class="landing-step-embed credential-row">
                  <div class="api-key-control landing-api-control">
                    <t-input :model-value="apiBaseUrlDisplay" readonly class="mono-text-input" />
                    <t-button
                      size="small"
                      variant="text"
                      :title="$t('integrations.chrome.copy')"
                      @click="copyApiUrl"
                    >
                      <t-icon name="file-copy" size="16px" />
                    </t-button>
                  </div>
                </div>
              </div>
            </li>
          </ol>
        </section>
      </div>
    </template>

    <template #footer>
      <span class="landing-meta">{{ $t('integrations.chrome.storeMeta') }}</span>
    </template>
  </IntegrationLandingLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { copyWithToast } from '@/utils/clipboard'
import { useRouter } from 'vue-router'
import { CHROME_EXTENSION_URL, INTEGRATION_TAB_MIN_ROLE } from '@/config/integrations'
import { useApiBaseUrlDisplay } from '@/composables/useApiBaseUrlDisplay'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import IntegrationLandingLayout from './IntegrationLandingLayout.vue'
import IntegrationExternalCta from './IntegrationExternalCta.vue'

const router = useRouter()
const uiStore = useUIStore()
const authStore = useAuthStore()
const { apiBaseUrlDisplay } = useApiBaseUrlDisplay()

const capabilityKeys = ['qa', 'clip', 'notes', 'shortcuts'] as const
const scenarioKeys = ['research', 'learning', 'tech', 'work'] as const
const stepKeys = ['api', 'port', 'install', 'connect'] as const

const capabilityIcons: Record<(typeof capabilityKeys)[number], string> = {
  qa: 'chat-bubble',
  clip: 'file-copy',
  notes: 'edit',
  shortcuts: 'jump',
}

const openChromeStore = () => {
  window.open(CHROME_EXTENSION_URL, '_blank', 'noopener,noreferrer')
}

// 「API 集成」是 owner 专属（INTEGRATION_TAB_MIN_ROLE.api），普通成员与空间
// 管理员点进去会命中 settings 的 role-denied 兜底，所以只对能看到该页面的人
// 渲染跳转按钮；引导页本身对所有成员开放。
const canOpenApiSettings = computed(() => {
  const min = INTEGRATION_TAB_MIN_ROLE.api
  if (!min) return true
  if (authStore.canAccessAllTenants) return true
  return authStore.hasRole(min)
})

const openApiSettings = () => {
  router.push({ path: '/platform/settings', query: { section: 'integration-api' } })
  uiStore.openSettings('integration-api')
}

const copyApiUrl = async () => {
  await copyWithToast(apiBaseUrlDisplay.value, 'integrations.chrome.copySuccess')
}
</script>

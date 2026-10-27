<template>
  <SpotlightGuide v-model:active="active" :steps="steps" step-i18n-prefix="newUserGuide.steps"
    labels-prefix="newUserGuide" @finish="onFinish" @step-change="onStepChange" />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import SpotlightGuide from '@/components/SpotlightGuide.vue'
import { GLOBAL_USER_GUIDE_KEY, OPEN_NEW_USER_GUIDE_EVENT } from '@/config/contextualGuides'
import { useAuthStore } from '@/stores/auth'
import { useDeploymentCapabilitiesStore } from '@/stores/deploymentCapabilities'
import { useUIStore } from '@/stores/ui'
import type { SpotlightGuideStep } from '@/types/spotlightGuide'

const uiStore = useUIStore()
const authStore = useAuthStore()
const deploymentCapabilities = useDeploymentCapabilitiesStore()
let settingsOpenedByGuide = false

// 引导步骤必须与用户实际能看到的菜单/入口一致：
// - 「智能体」受部署能力 agents 门控（与 menu.ts 同一判断），未部署时侧栏
//   不显示，这里整条剔除，避免进度点位显示一步却指向空气；
// - 「模型」：模型管理已收归平台超级管理员（isSystemAdmin，运营管理员继承），
//   普通成员和空间管理员在设置导航里看不到该入口，深链进入会被 Settings.vue
//   的 canSeeSection 兜底渲染 role-denied 空态，因此不能出现在他们的引导里；
// - 「设置」步骤按角色挑选文案：普通成员只谈个人偏好、空间管理员加成员与
//   数据源、超管/运营再加系统管理，避免向他们描述看不到的入口。
const pickSettingsStepKey = (): 'settingsSuper' | 'settingsWorkspaceAdmin' | 'settingsMember' => {
  if (authStore.isSystemAdmin) return 'settingsSuper'
  if (authStore.canAccessAllTenants || authStore.hasRole('admin')) return 'settingsWorkspaceAdmin'
  return 'settingsMember'
}

const steps = computed<SpotlightGuideStep[]>(() => {
  const list: SpotlightGuideStep[] = [
    { key: 'welcome' },
    {
      key: 'knowledge',
      target: '[data-guide="nav-knowledge-bases"]',
      placement: 'right',
      before: () => uiStore.expandSidebar(),
    },
  ]
  if (deploymentCapabilities.isSupported('agents')) {
    list.push({
      key: 'agents',
      target: '[data-guide="nav-agents"]',
      placement: 'right',
      before: () => uiStore.expandSidebar(),
    })
  }
  list.push(
    {
      key: 'chat',
      target: '[data-guide="nav-creatChat"]',
      placement: 'right',
      before: () => uiStore.expandSidebar(),
    },
    {
      key: pickSettingsStepKey(),
      target: '[data-guide="user-menu"]',
      placement: 'right',
      before: () => uiStore.expandSidebar(),
    },
  )
  if (authStore.isSystemAdmin) {
    list.push({
      key: 'models',
      target: '[data-guide="settings-add-model"], [data-guide="settings-models"]',
      placement: 'left',
      before: () => {
        uiStore.openSettings('models')
        settingsOpenedByGuide = true
      },
    })
  }
  list.push({ key: 'done' })
  return list
})

const active = ref(false)

const closeGuideSettings = () => {
  if (settingsOpenedByGuide) {
    uiStore.closeSettings()
    settingsOpenedByGuide = false
  }
}

const onFinish = () => {
  localStorage.setItem(GLOBAL_USER_GUIDE_KEY, '1')
  closeGuideSettings()
}

const onStepChange = ({ toKey }: { toKey: string }) => {
  if (toKey !== 'models') {
    closeGuideSettings()
  }
}

const open = () => {
  active.value = true
}

const handleOpenEvent = () => {
  if (active.value) return
  open()
}

onMounted(() => {
  window.addEventListener(OPEN_NEW_USER_GUIDE_EVENT, handleOpenEvent)
  if (localStorage.getItem(GLOBAL_USER_GUIDE_KEY) !== '1') {
    window.setTimeout(() => {
      if (localStorage.getItem(GLOBAL_USER_GUIDE_KEY) !== '1') {
        open()
      }
    }, 700)
  }
})

onBeforeUnmount(() => {
  window.removeEventListener(OPEN_NEW_USER_GUIDE_EVENT, handleOpenEvent)
  closeGuideSettings()
})
</script>

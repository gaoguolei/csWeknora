<template>
  <SpotlightGuide v-model:active="active" :steps="guideSteps" step-i18n-prefix="contextualGuide.kbCreate.steps"
    labels-prefix="contextualGuide" @finish="onFinish" @dismiss="onFinish" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import SpotlightGuide from '@/components/SpotlightGuide.vue'
import {
  focusKbEditorSection,
  markContextualGuideDone,
  isContextualGuideDone,
  isGlobalUserGuideDone,
} from '@/config/contextualGuides'
import type { SpotlightGuideStep } from '@/types/spotlightGuide'

const props = defineProps<{
  when: boolean
  isFaq: boolean
}>()

const active = ref(false)

const guideSteps = computed<SpotlightGuideStep[]>(() => {
  const steps: SpotlightGuideStep[] = [
    {
      key: 'type',
      target: '[data-guide="kb-create-type"]',
      placement: 'right',
      before: () => focusKbEditorSection('basic'),
    },
    {
      key: 'name',
      target: '[data-guide="kb-create-name"]',
      placement: 'right',
      before: () => focusKbEditorSection('basic'),
    },
  ]

  if (!props.isFaq) {
    steps.push({
      key: 'indexing',
      target: '[data-guide="kb-create-indexing"]',
      placement: 'right',
      before: () => focusKbEditorSection('basic'),
    })
  }

  if (props.isFaq) {
    // 建库态的 FAQ 索引策略已内联到基本信息里（不再有侧栏 FAQ 分区）。
    steps.push({
      key: 'faq',
      target: '[data-guide="kb-create-faq"]',
      placement: 'right',
      before: () => focusKbEditorSection('basic'),
      optional: true,
    })
  }

  steps.push({
    key: 'submit',
    target: '[data-guide="kb-create-submit"]',
    placement: 'top',
    before: () => focusKbEditorSection('basic'),
  })

  return steps
})

let openTimer: ReturnType<typeof setTimeout> | null = null
let waitGlobalTimer: ReturnType<typeof setTimeout> | null = null

const onFinish = () => {
  markContextualGuideDone('kbCreate')
}

const tryOpen = () => {
  if (!props.when || isContextualGuideDone('kbCreate') || active.value) return
  openTimer = setTimeout(() => {
    if (!props.when || isContextualGuideDone('kbCreate')) return
    active.value = true
  }, 450)
}

const scheduleOpen = () => {
  if (openTimer) {
    clearTimeout(openTimer)
    openTimer = null
  }
  if (waitGlobalTimer) {
    clearTimeout(waitGlobalTimer)
    waitGlobalTimer = null
  }
  if (!props.when || isContextualGuideDone('kbCreate')) return
  if (isGlobalUserGuideDone()) {
    tryOpen()
    return
  }
  const poll = () => {
    if (!props.when || isContextualGuideDone('kbCreate')) return
    if (isGlobalUserGuideDone()) {
      tryOpen()
      return
    }
    waitGlobalTimer = setTimeout(poll, 400)
  }
  waitGlobalTimer = setTimeout(poll, 400)
}

watch(
  () => props.when,
  (val) => {
    if (!val) {
      if (openTimer) clearTimeout(openTimer)
      if (waitGlobalTimer) clearTimeout(waitGlobalTimer)
      openTimer = null
      waitGlobalTimer = null
      active.value = false
      return
    }
    scheduleOpen()
  },
  { immediate: true },
)
</script>

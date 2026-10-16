<template>
  <section v-if="run" class="run-details" :aria-label="t('workspace.validationRunDetails')">
    <div class="run-title"><strong :class="testRunDisplayState(run)">{{ t(`workspace.validationState_${testRunDisplayState(run)}`) }}</strong><time>{{ new Date(run.created_at).toLocaleString() }}</time></div>
    <p v-if="stale" class="notice">{{ t('workspace.validationStale') }}</p>
    <p v-if="run.result.detail" class="notice" role="status">{{ run.result.detail }}</p>
    <p class="run-version"><code>{{ commit || t('workspace.validationWorkingTree') }}</code> · {{ run.request.source === 'ai' ? 'AI' : run.request.source === 'automatic' ? t('workspace.validationAutomatic') : t('workspace.manualRunCase') }} · {{ ((run.result.duration_ms || 0) / 1000).toFixed(1) }}s</p>
    <p v-if="run.request.case_name" class="notice">{{ t('workspace.validationCaseScope') }}：{{ run.request.case_name }}</p>
    <div class="run-actions">
      <button v-if="active" type="button" @click="emit('cancel')">{{ t('workspace.cancelTestRun') }}</button>
      <button v-else-if="hasFailures" type="button" :disabled="busy" @click="emit('rerun')">{{ t('workspace.validationRerunFailed') }}</button>
      <button type="button" @click="emit('ask', false)">{{ t('workspace.analyzeCheckFailure') }}</button>
      <button v-if="!active" type="button" :disabled="busy" @click="emit('ask', true)">{{ t('workspace.validationFixRerun') }}</button>
      <button v-if="run.report_message_id" type="button" @click="emit('report', run.report_message_id)"><t-icon name="file" />{{ t('workspace.validationViewReport') }}</button>
      <button v-if="!active" type="button" :disabled="busy" @click="emit('browser')">{{ t('workspace.validationProbeBrowser') }}</button>
    </div>
    <p v-if="run.report_message_id" class="saved"><t-icon name="check-circle" />{{ t('workspace.validationReportSaved') }}</p>
    <p v-if="run.report_error" role="alert">{{ run.report_error }}</p>
    <details v-for="check in run.result.checks || []" :key="check.id" :open="check.status !== 'passed'" class="check-details">
      <summary><span :class="check.status">{{ t(`workspace.validationState_${check.status}`) }}</span> {{ check.project_name }} / {{ check.file || t(`workspace.validationKind_${check.kind}`) }}<small>{{ check.cases?.length || 0 }} {{ t('workspace.validationReportedCases') }}</small></summary>
      <p v-if="check.detail" class="notice">{{ check.detail }}</p>
      <code v-if="check.command">$ {{ check.command }}</code>
      <ul><li v-for="(test, index) in check.cases || []" :key="index"><span :class="test.status">{{ t(`workspace.validationState_${test.status}`) }}</span> {{ test.name }} <button v-if="test.selector && test.file && ['node-test','vitest','jest'].includes(check.framework)" type="button" :disabled="busy" @click="emit('runCase', check, test)">{{ t('workspace.validationRerunCase') }}</button><pre v-if="test.detail">{{ test.detail }}</pre></li></ul>
    </details>
    <details class="logs" :open="active"><summary>{{ t('workspace.executionDetails') }}</summary><pre>{{ run.result.log || t('workspace.waitingCheckOutput') }}</pre></details>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TestRun, TestCheck, TestCaseResult } from '@/api/workspaceTests'
import { isActiveTestRun, testRunDisplayState } from '@/composables/useWorkspaceTests'
const props = defineProps<{ run?: TestRun; stale: boolean; busy: boolean }>()
const emit = defineEmits<{ cancel: []; rerun: []; ask: [fix: boolean]; report: [messageId: string]; runCase: [check: TestCheck, test: TestCaseResult]; browser: [] }>()
const { t } = useI18n()
const active = computed(() => Boolean(props.run && isActiveTestRun(props.run)))
const commit = computed(() => (props.run?.result.version?.head || Object.values(props.run?.result.project_versions || {}).find(version => version.head)?.head || '').slice(0,8))
const hasFailures = computed(() => props.run?.result.checks.some(check => ['failed', 'error', 'blocked', 'timed_out'].includes(check.status)))
</script>
<style scoped lang="less">
.run-details { margin-top: 20px; padding-top: 14px; border-top: 1px solid var(--td-component-stroke); font-size: 12px; }
.run-title { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
time, .run-version, small { color: var(--td-text-color-placeholder); font-size: 11px; }
.notice { line-height: 1.6; color: var(--td-text-color-secondary); }
.run-actions { display: flex; flex-wrap: wrap; gap: 6px; margin: 12px 0; }
button { border: 1px solid var(--td-component-stroke); border-radius: 5px; padding: 6px 9px; background: transparent; color: inherit; font: inherit; cursor: pointer; }
button:disabled { opacity: .4; cursor: not-allowed; }
.saved, .passed { color: var(--td-success-color); } .saved { display: flex; gap: 5px; align-items: center; }
.failed, .error, .timed_out { color: var(--td-error-color); } .blocked, .unknown, .no_tests { color: var(--td-warning-color); }
summary { cursor: pointer; padding: 8px 0; overflow-wrap: anywhere; }
summary small { margin-left: 8px; }
.check-details { border-top: 1px solid var(--td-component-stroke); }
code, pre { font: 11px/1.6 var(--app-font-family-mono, monospace); white-space: pre-wrap; overflow-wrap: anywhere; }
pre { max-height: 320px; overflow: auto; background: var(--td-bg-color-secondarycontainer); border-radius: 5px; padding: 10px; }
ul { list-style: none; padding: 0; } li { margin: 8px 0; overflow-wrap: anywhere; }
button:focus-visible, summary:focus-visible { outline: 2px solid var(--td-brand-color); }
</style>

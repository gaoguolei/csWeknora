<template>
  <Teleport :to="approvalDock || 'body'" :disabled="!isDocked">
    <div v-if="!unavailable || resolved" class="action-card interaction-inline" :class="{ 'action-pending': !effectiveResolved, 'is-docked': isDocked }">
      <div class="action-header no-results">
        <div class="action-title">
          <t-icon class="action-title-icon" :name="headerIcon" />
          <span class="action-name">{{ mainTitle }}</span>
        </div>
      </div>

      <div class="interaction-status-summary">
        <div class="results-summary-text">
          <span v-if="effectiveResolved" class="status-label">{{ statusLine }}</span>
          <span v-if="!effectiveResolved" class="inline-actions">
            <button type="button" class="inline-action" :disabled="checking || submitting || submitted" @click.stop="submit('reject')">
              {{ $t('agentStream.toolApproval.reject') }}
            </button>
            <span class="inline-dot">·</span>
            <button type="button" class="inline-action is-primary" :disabled="checking || submitting || submitted"
              @click.stop="submit('approve')">
              {{ $t(approvalPlan ? 'agentStream.toolApproval.approvePlan' : 'agentStream.toolApproval.approve') }}
            </button>
          </span>
        </div>
      </div>

      <div v-if="approvalPlan" class="approval-plan-preview">
        <PlanDisplay :data="approvalPlan" />
      </div>
      <div v-if="!effectiveResolved" class="interaction-status-summary">
        <div class="results-summary-text">{{ $t(approvalPlan ? 'agentStream.toolApproval.planApprovalHint' : 'agentStream.toolApproval.approvalHint') }}</div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, shallowRef, computed, onMounted } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { resolveToolApproval } from '@/api/mcp-service'
import { resolveEmbedToolApproval } from '@/api/embed'
import { isUnavailableToolApproval, usePendingToolApproval } from '@/composables/usePendingToolApproval'
import PlanDisplay from './tool-results/PlanDisplay.vue'
import type { PlanData } from '@/types/tool-results'

const props = defineProps<{
  pendingId: string
  serviceName: string
  mcpToolName: string
  description?: string
  argsJson?: string
  timeoutSeconds?: number
  requestedAt?: number
  resolved?: boolean
  approved?: boolean
  resolveReason?: string
  embeddedMode?: boolean
  embedChannelId?: string
  embedToken?: string
  embedSessionId?: string
  embedSessionSig?: string
  embedVisitorId?: string
}>()

const useEmbedApproval = () =>
  props.embeddedMode
  && props.embedChannelId
  && props.embedToken
  && props.embedSessionId
  && props.embedSessionSig
  && props.embedVisitorId

const { t } = useI18n()

const approvalDock = shallowRef<HTMLElement | null>(null)
const submitting = ref(false)
const submitted = ref(false)
const submittedDecision = ref<'approve' | 'reject'>('reject')
const effectiveResolved = computed(() => !!props.resolved || submitted.value)
const effectiveApproved = computed(() => props.resolved ? props.approved : submitted.value && submittedDecision.value === 'approve')
const { unavailable, checking, markUnavailable } = usePendingToolApproval(
  () => props.pendingId, () => !props.embeddedMode && !effectiveResolved.value,
)
const approvalPlan = computed<PlanData | null>(() => {
  if (props.serviceName !== 'WeKnora' || props.mcpToolName !== 'approve_plan') return null
  try {
    const value = JSON.parse(props.argsJson || '{}')
    if (!Array.isArray(value?.steps)) return null
    return { display_type: 'plan', task: typeof value.task === 'string' ? value.task : '', steps: value.steps, total_steps: value.steps.length }
  } catch { return null }
})

const isDocked = computed(() => !!approvalDock.value && !effectiveResolved.value && !props.embeddedMode)

const headerIcon = computed(() => {
  if (!effectiveResolved.value) return 'secured'
  if (effectiveApproved.value) return 'check-circle'
  return 'close-circle'
})

const builtinToolLabels: Record<string, string> = {
  shell_exec: 'agentStream.tools.shellExec',
  write_sandbox_file: 'agentStream.tools.writeSandboxFile',
  edit_sandbox_file: 'agentStream.tools.editSandboxFile',
  edit_knowledge_file: 'agentEditor.tools.editKnowledgeFile',
}
const toolLabel = computed(() => {
  const key = props.serviceName === 'WeKnora' && builtinToolLabels[props.mcpToolName]
  return key ? t(key) : props.mcpToolName
})

const mainTitle = computed(() => {
  if (approvalPlan.value) return t('agentStream.toolApproval.taskPlanTitle')
  if (!effectiveResolved.value) {
    return t('agentStream.toolApproval.waitingStatus')
  }
  return t('agentStream.toolApproval.titleWithTarget', {
    service: props.serviceName,
    tool: toolLabel.value,
  })
})

const statusLine = computed(() => {
  if (!effectiveResolved.value) return t('agentStream.toolApproval.waitingStatus')
  if (effectiveApproved.value) return t('agentStream.toolApproval.approvedTag')
  return t('agentStream.toolApproval.rejectedTag')
})

onMounted(() => {
  if (!props.embeddedMode) approvalDock.value = document.getElementById('chat-tool-approval-dock')
})

const submit = async (decision: 'approve' | 'reject') => {
  if (effectiveResolved.value || checking.value || unavailable.value || submitting.value) return
  submitting.value = true
  try {
    await (useEmbedApproval()
      ? resolveEmbedToolApproval(
        props.embedChannelId!,
        props.embedToken!,
        props.embedSessionId!,
        props.embedSessionSig!,
        props.embedVisitorId!,
        props.pendingId,
        {
          decision,
          reason: decision === 'reject' ? t('agentStream.toolApproval.userRejected') : undefined,
        },
      )
      : resolveToolApproval(props.pendingId, {
        decision,
        reason: decision === 'reject' ? t('agentStream.toolApproval.userRejected') : undefined,
      }))
    submittedDecision.value = decision
    submitted.value = true
    MessagePlugin.success(t('agentStream.toolApproval.submitted'))
  } catch (e: any) {
    if (isUnavailableToolApproval(e)) {
      markUnavailable()
      MessagePlugin.info(t('agentStream.toolApproval.unavailable'))
      return
    }
    const msg = e?.response?.data?.error?.message || e?.message || t('agentStream.toolApproval.submitFailed')
    MessagePlugin.error(msg)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="less">
@import './agent-interaction-card.less';

.approval-plan-preview { max-height: 180px; overflow-y: auto; padding-top: 8px; }

.is-docked {
  .action-title { gap: 8px; flex-wrap: nowrap; }
  .action-title .action-name { white-space: normal; flex-shrink: 1; overflow-wrap: anywhere; }
  .action-title .action-title-icon.t-icon {
    position: static;
    transform: none;
    margin-right: 0;
  }
  .interaction-status-summary { padding-top: 8px; }
  .inline-actions > .inline-dot { display: none; }
  .inline-action {
    padding: 5px 10px;
    border: 1px solid var(--td-component-border);
    border-radius: var(--app-radius-md);
    color: var(--td-text-color-primary);
    &.is-primary {
      border-color: var(--td-brand-color);
      background: var(--td-brand-color);
      color: var(--td-text-color-anti);
    }
  }
  & + .is-docked { margin-top: 16px; }
}
</style>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { resolveToolApproval } from '@/api/mcp-service'
import { isUnavailableToolApproval, usePendingToolApproval } from '@/composables/usePendingToolApproval'
import type { PlanData, PlanStep } from '@/types/tool-results'

interface PlanEvent {
  type?: string
  tool_name?: string
  pending?: boolean
  success?: boolean
  tool_data?: Partial<PlanData> & { data?: Partial<PlanData> }
  mcp_tool_name?: string
  service_name?: string
  pending_id?: string
  args_json?: string
  resolved?: boolean
  approved?: boolean
  tool_call_id?: string
}

const props = defineProps<{
  messages: Array<{ id?: string; role?: string; is_completed?: boolean; agentEventStream?: unknown }>
  running: boolean
  planMode?: boolean
}>()
const { t } = useI18n()
const expanded = ref(false)
const submitting = ref(false)
const submittedId = ref('')
const rejectedPlanId = ref('')

function parseTaskPlan(data: unknown): { task: string; steps: PlanStep[]; blockedReason: string } | null {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const value = data as Partial<PlanData>
  if (!Array.isArray(value.steps)) return null
  const steps = value.steps.filter((step): step is PlanStep => !!step && typeof step.id === 'string'
    && !!step.id.trim() && typeof step.description === 'string' && !!step.description.trim()
    && ['pending', 'in_progress', 'completed', 'skipped'].includes(step.status))
    .map(step => ({ id: step.id, description: step.description, status: step.status }))
  return steps.length ? { task: typeof value.task === 'string' ? value.task : '', steps,
    blockedReason: typeof value.blocked_reason === 'string' ? value.blocked_reason : '' } : null
}

function resultPlan(event: PlanEvent) {
  if (event?.type !== 'tool_call' || event.tool_name !== 'todo_write' || event.pending || event.success === false) return null
  return parseTaskPlan(Array.isArray(event.tool_data?.steps) ? event.tool_data : event.tool_data?.data)
}

// Approved unfinished plans carry across follow-ups while plan mode is selected.
// Other checklists belong to their generating reply and expire when it finishes.
// Completed/rejected approved plans end when the next user request starts.
const planState = computed(() => {
  let snapshot: ReturnType<typeof parseTaskPlan> = null
  let approval: PlanEvent | null = null
  let approvalOwnerCompleted = false
  let key = ''
  for (const message of props.messages) {
    if (message.role === 'user') {
      if (snapshot && (snapshot.steps.every(step => step.status === 'completed')
        || (approval?.resolved && approval.approved === false)
        || (approval && !approval.resolved && approvalOwnerCompleted))) {
        snapshot = null
        approval = null
        key = ''
      }
      continue
    }
    if (message.role !== 'assistant' || !Array.isArray(message.agentEventStream)) continue
    for (const event of message.agentEventStream as PlanEvent[]) {
      if (event.type === 'tool_approval_required' && event.service_name === 'WeKnora'
        && event.mcp_tool_name === 'approve_plan') {
        let proposed: ReturnType<typeof parseTaskPlan> = null
        try { proposed = parseTaskPlan(JSON.parse(event.args_json || '{}')) } catch { /* Incomplete proposal. */ }
        if (proposed && (!approval?.pending_id || approval.pending_id !== event.pending_id)) {
          snapshot = proposed
          approval = event
          approvalOwnerCompleted = !!message.is_completed
          key = event.pending_id || message.id || ''
        }
        continue
      }
      const update = resultPlan(event)
      if (!update) continue
      if (!snapshot) {
        snapshot = update
        key = event.tool_call_id || message.id || ''
        continue
      }
      if (approval && (!approval.resolved || !approval.approved)) continue
      const sameTasks = update.steps.length === snapshot.steps.length
        && update.steps.every((step, index) => step.id === snapshot!.steps[index].id)
      if (!sameTasks || (!approval && update.task !== snapshot.task)) {
        if (!approval) {
          snapshot = update
          key = event.tool_call_id || message.id || ''
        }
        continue
      }
      snapshot.blockedReason = update.blockedReason
      for (let i = 0; i < snapshot.steps.length; i++) {
        const original = snapshot.steps[i]
        const progress = update.steps[i]
        if (original.status !== 'completed' && progress.status !== 'skipped'
          && !(original.status === 'in_progress' && progress.status === 'pending')) original.status = progress.status
      }
    }
    if (message.is_completed && (!approval || !props.planMode)) {
      snapshot = null
      approval = null
      key = ''
    }
  }
  return { plan: snapshot, approval, key }
})
const plan = computed(() => planState.value.plan)
const planApproval = computed(() => planState.value.approval)
const requiresApproval = computed(() => !!planApproval.value?.pending_id && !planApproval.value.resolved
  && props.running && submittedId.value !== planApproval.value.pending_id)
const { unavailable, checking, markUnavailable } = usePendingToolApproval(
  () => planApproval.value?.pending_id, () => requiresApproval.value,
)
const awaitingApproval = computed(() => requiresApproval.value && !unavailable.value)
const rejected = computed(() => (!!planApproval.value?.resolved && planApproval.value.approved === false)
  || (!!planApproval.value?.pending_id && planApproval.value.pending_id === rejectedPlanId.value))
const executing = computed(() => props.running && !awaitingApproval.value && !rejected.value && !plan.value?.blockedReason)
const completed = computed(() => plan.value?.steps.filter(step => step.status === 'completed').length || 0)
const total = computed(() => plan.value?.steps.length || 0)
const allCompleted = computed(() => total.value > 0 && completed.value === total.value && !awaitingApproval.value && !rejected.value && !plan.value?.blockedReason)
const visible = computed(() => !!plan.value && !allCompleted.value && !rejected.value
  && (props.running || (props.planMode && planApproval.value?.resolved && planApproval.value.approved))
  && (!unavailable.value || planApproval.value?.resolved === true))
const currentStep = computed(() => plan.value?.steps.find(step => step.status === 'in_progress'))
watch(() => planState.value.key, () => { expanded.value = false })
watch(() => planApproval.value?.pending_id, (id) => {
  if (id && !planApproval.value?.resolved) expanded.value = true
})

async function submitPlan(decision: 'approve' | 'reject') {
  const id = planApproval.value?.pending_id
  if (!id || !awaitingApproval.value || checking.value || submitting.value || submittedId.value === id) return
  submitting.value = true
  try {
    await resolveToolApproval(id, {
      decision,
      reason: decision === 'reject' ? t('agentStream.toolApproval.userRejected') : undefined,
    })
    submittedId.value = id
    if (decision === 'reject') rejectedPlanId.value = id
  } catch (error: any) {
    if (isUnavailableToolApproval(error)) {
      markUnavailable()
      MessagePlugin.info(t('agentStream.toolApproval.unavailable'))
      return
    }
    MessagePlugin.error(error?.response?.data?.error?.message || error?.message || t('agentStream.toolApproval.submitFailed'))
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <section v-if="visible" class="chat-task-plan" :aria-label="t('agentStream.toolApproval.taskPlanTitle')">
    <div class="task-plan-inner">
      <button type="button" class="task-plan-header" :aria-expanded="expanded" @click="expanded = !expanded">
        <t-icon :name="awaitingApproval ? 'secured' : executing ? 'loading' : 'task'" class="task-plan-icon"
          :class="{ 'is-spinning': executing }" />
        <span class="task-plan-title">{{ t('agentStream.toolApproval.taskPlanTitle') }}</span>
        <span v-if="total" class="task-plan-count">{{ completed }} / {{ total }}</span>
        <span v-if="!expanded && currentStep" class="task-plan-preview" :title="currentStep.description">{{ currentStep.description }}</span>
        <span v-if="awaitingApproval" class="task-plan-state">{{ t('agentStream.toolApproval.planAwaitingApproval') }}</span>
        <span v-else-if="plan?.blockedReason" class="task-plan-state">{{ t('agentStream.toolApproval.planBlocked') }}</span>
        <span v-else-if="total && !running" class="task-plan-state">{{ t('agentStream.toolApproval.planUnfinished') }}</span>
        <t-icon class="task-plan-chevron" :name="expanded ? 'chevron-up' : 'chevron-down'" />
      </button>
      <div v-if="expanded && plan" class="task-plan-body">
        <p v-if="plan.task" class="task-plan-task">{{ plan.task }}</p>
        <p v-if="plan.blockedReason" class="task-plan-blocker" role="status">{{ plan.blockedReason }}</p>
        <ol class="task-plan-steps">
          <li v-for="(step, index) in plan.steps" :key="step.id || index"
            class="task-plan-step" :class="`is-${step.status}`">
            <t-icon :name="step.status === 'completed' ? 'check-circle-filled'
              : step.status === 'in_progress' && executing ? 'loading'
              : step.status === 'skipped' ? 'minus-circle' : 'circle'"
              class="task-plan-step-icon" :class="{ 'is-spinning': step.status === 'in_progress' && executing }" />
            <span class="task-plan-step-text">{{ step.description }}</span>
          </li>
        </ol>
      </div>
      <div v-if="awaitingApproval" class="task-plan-approval">
        <span>{{ t('agentStream.toolApproval.planApprovalHint') }}</span>
        <div class="task-plan-approval-actions">
          <button type="button" :disabled="checking || submitting || submittedId === planApproval?.pending_id" @click="submitPlan('reject')">
            {{ t('agentStream.toolApproval.reject') }}
          </button>
          <button type="button" class="is-primary" :disabled="checking || submitting || submittedId === planApproval?.pending_id" @click="submitPlan('approve')">
            {{ t('agentStream.toolApproval.approvePlan') }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped lang="less">
.chat-task-plan {
  flex: 0 0 auto;
  min-height: 0;
  width: 100%;
  box-sizing: border-box;
  margin-bottom: 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-lg);
  background: var(--td-bg-color-container);
}
.task-plan-inner { min-width: 0; }
.task-plan-header {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 7px 10px;
  border: 0;
  background: transparent;
  color: var(--td-text-color-primary);
  font: inherit;
  font-size: 12px;
  line-height: 16px;
  text-align: left;
  cursor: pointer;
}
.task-plan-title { flex-shrink: 0; font-weight: 500; }
.task-plan-count { flex-shrink: 0; font-variant-numeric: tabular-nums; color: var(--td-text-color-secondary); }
.task-plan-state { font-size: 11px; color: var(--td-text-color-secondary); }
.task-plan-preview { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--td-text-color-secondary); }
.task-plan-icon, .task-plan-chevron { flex-shrink: 0; font-size: 14px; }
.task-plan-chevron { margin-left: auto; color: var(--td-text-color-placeholder); }
.task-plan-body { max-height: min(18vh, 152px); padding: 0 10px 8px; overflow-y: auto; scrollbar-width: thin; }
.task-plan-task { margin: 4px 0 0 20px; font-size: 11px; color: var(--td-text-color-secondary); overflow-wrap: anywhere; }
.task-plan-blocker { margin: 6px 0 0 20px; font-size: 12px; color: var(--td-warning-color); overflow-wrap: anywhere; }
.task-plan-steps { list-style: none; margin: 4px 0 0; padding: 0; }
.task-plan-step {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  padding: 3px 0;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  line-height: 1.4;
  .task-plan-step-icon { flex-shrink: 0; margin-top: 2px; font-size: 13px; }
  .task-plan-step-text { min-width: 0; overflow-wrap: anywhere; }
  &.is-in_progress { color: var(--td-text-color-primary); }
  &.is-completed .task-plan-step-text { text-decoration: line-through; color: var(--td-text-color-placeholder); }
  &.is-completed .task-plan-step-icon { color: var(--td-success-color); }
  &.is-skipped { color: var(--td-text-color-placeholder); }
}
.task-plan-approval {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 7px 10px;
  border-top: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-secondary);
  font-size: 12px;
}
.task-plan-approval-actions {
  display: flex;
  gap: 6px;
  margin-left: auto;
  button {
    border: 1px solid var(--td-component-border);
    border-radius: var(--app-radius-md);
    padding: 4px 8px;
    background: transparent;
    color: var(--td-text-color-primary);
    font: inherit;
    cursor: pointer;
    &:disabled { opacity: .5; cursor: default; }
    &.is-primary { background: var(--td-brand-color); border-color: var(--td-brand-color); color: var(--td-text-color-anti); }
  }
}
.is-spinning { animation: plan-spin 1s linear infinite; }
@keyframes plan-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .is-spinning { animation: none; } }
</style>

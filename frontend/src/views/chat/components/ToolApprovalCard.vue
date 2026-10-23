<template>
  <Teleport :to="approvalDock || 'body'" :disabled="!isDocked">
    <div class="action-card interaction-inline" :class="{ 'action-pending': !resolved, 'is-docked': isDocked }">
      <div class="action-header no-results">
        <div class="action-title">
          <t-icon class="action-title-icon" :name="headerIcon" />
          <span class="action-name">{{ mainTitle }}</span>
        </div>
      </div>

      <div class="interaction-status-summary">
        <div class="results-summary-text">
          <span v-if="resolved" class="status-label">{{ statusLine }}</span>
          <span v-if="!resolved" class="inline-actions">
            <button type="button" class="inline-action" :disabled="submitting || submitted" @click.stop="submit('reject')">
              {{ $t('agentStream.toolApproval.reject') }}
            </button>
            <span class="inline-dot">·</span>
            <button type="button" class="inline-action is-primary" :disabled="submitting || submitted || !isJsonValid"
              @click.stop="submit('approve')">
              {{ $t('agentStream.toolApproval.approve') }}
            </button>
          </span>
        </div>
      </div>

      <div v-if="!resolved && !argsExpanded && (!isJsonValid || argsDirty)" class="interaction-status-summary">
        <div class="results-summary-text">
          <span v-if="!isJsonValid" class="args-status args-invalid">
            <t-icon name="error-circle" />
            {{ $t('agentStream.toolApproval.invalidJson') }}
          </span>
          <span v-else-if="argsDirty" class="args-status args-dirty">
            {{ $t('agentStream.toolApproval.argsModified') }}
          </span>
        </div>
      </div>

      <div v-if="hasArgs" class="interaction-status-summary is-clickable" @click="toggleArgs">
        <div class="results-summary-text args-toggle-row">
          <span>{{ $t('agentStream.toolApproval.argsLabel') }}</span>
          <t-icon class="args-toggle-icon" :name="argsExpanded ? 'chevron-down' : 'chevron-right'" />
        </div>
      </div>

      <div v-if="hasArgs && argsExpanded" class="action-details">
        <div v-if="!resolved && (!isJsonValid || argsDirty)" class="interaction-args-meta">
          <span v-if="!isJsonValid" class="args-status args-invalid">
            <t-icon name="error-circle" />
            {{ $t('agentStream.toolApproval.invalidJson') }}
          </span>
          <span v-else-if="argsDirty" class="args-status args-dirty">
            {{ $t('agentStream.toolApproval.argsModified') }}
          </span>
        </div>

        <div v-if="!resolved" class="interaction-args-block">
          <t-textarea v-model="argsText" class="interaction-args-input" :autosize="{ minRows: 2, maxRows: 8 }"
            placeholder="{}" @click.stop />
        </div>

        <div v-else class="interaction-args-preview">{{ argsText }}</div>
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

function formatJson(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

const initialArgs = formatJson(props.argsJson || '{}')
const argsText = ref(initialArgs)
const argsExpanded = ref(true)
const approvalDock = shallowRef<HTMLElement | null>(null)
const submitting = ref(false)
const submitted = ref(false)

const hasArgs = computed(() => argsText.value.trim().length > 0)

const isJsonValid = computed(() => {
  if (!argsText.value.trim()) return true
  try {
    const value = JSON.parse(argsText.value)
    return value !== null && typeof value === 'object' && !Array.isArray(value)
  } catch {
    return false
  }
})

const argsDirty = computed(() => argsText.value.trim() !== initialArgs.trim())

const isDocked = computed(() => !!approvalDock.value && !props.resolved && !props.embeddedMode)

const headerIcon = computed(() => {
  if (!props.resolved) return 'secured'
  if (props.approved) return 'check-circle'
  return 'close-circle'
})

const targetLabel = computed(() => (
  t('agentStream.toolApproval.targetWithTool', {
    service: props.serviceName,
    tool: props.mcpToolName,
  })
))

const mainTitle = computed(() => {
  if (!props.resolved) {
    return t('agentStream.toolApproval.waiting', { target: targetLabel.value })
  }
  return t('agentStream.toolApproval.titleWithTarget', {
    service: props.serviceName,
    tool: props.mcpToolName,
  })
})

const statusLine = computed(() => {
  if (!props.resolved) return t('agentStream.toolApproval.waitingStatus')
  if (props.approved) return t('agentStream.toolApproval.approvedTag')
  return t('agentStream.toolApproval.rejectedTag')
})

function toggleArgs() {
  argsExpanded.value = !argsExpanded.value
}

onMounted(() => {
  if (!props.embeddedMode) approvalDock.value = document.getElementById('chat-tool-approval-dock')
})

const submit = async (decision: 'approve' | 'reject') => {
  if (props.resolved || submitting.value || submitted.value) return
  submitting.value = true
  try {
    let modified: Record<string, unknown> | undefined
    if (decision === 'approve') {
      try {
        modified = JSON.parse(argsText.value || '{}') as Record<string, unknown>
      } catch {
        argsExpanded.value = true
        MessagePlugin.error(t('agentStream.toolApproval.invalidJson'))
        return
      }
    }
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
          modified_args: decision === 'approve' && argsDirty.value ? modified : undefined,
          reason: decision === 'reject' ? t('agentStream.toolApproval.userRejected') : undefined,
        },
      )
      : resolveToolApproval(props.pendingId, {
        decision,
        modified_args: decision === 'approve' && argsDirty.value ? modified : undefined,
        reason: decision === 'reject' ? t('agentStream.toolApproval.userRejected') : undefined,
      }))
    submitted.value = true
    MessagePlugin.success(t('agentStream.toolApproval.submitted'))
  } catch (e: any) {
    const msg = e?.response?.data?.error?.message || e?.message || t('agentStream.toolApproval.submitFailed')
    MessagePlugin.error(msg)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="less">
@import './agent-interaction-card.less';

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

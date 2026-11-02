<script setup lang="ts">
import { computed, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { updateToolApprovalMode } from '@/api/chat/steer'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'

const props = defineProps<{ disabled?: boolean; sessionId?: string; assistantMessageId?: string }>()
const { t } = useI18n()
const settings = useSettingsStore()
const visible = ref(false)
const submitting = ref(false)
const modes = [
  { value: 'auto', key: 'auto', icon: 'check-circle' },
  { value: 'manual', key: 'manual', icon: 'secured' },
  { value: 'accept_edits', key: 'acceptEdits', icon: 'edit' },
  { value: 'plan', key: 'plan', icon: 'task' },
  { value: 'bypass', key: 'bypass', icon: 'lock-off' },
] as const
const selectedMode = computed(() => modes.find(mode => mode.value === settings.toolApprovalMode) || modes[0])
const modeLabel = computed(() => t('agentStream.toolApproval.modeLabel'))
const select = async (mode: typeof modes[number]['value']) => {
  if (props.disabled || submitting.value) return
  visible.value = false
  if (mode === settings.toolApprovalMode) return
  const sessionId = props.sessionId
  submitting.value = true
  try {
    if (sessionId) await updateToolApprovalMode(sessionId, mode, props.assistantMessageId)
    if (props.sessionId === sessionId) settings.toolApprovalMode = mode
  } catch (error: any) {
    MessagePlugin.error(error?.response?.data?.error?.message || error?.message || t('agentStream.toolApproval.submitFailed'))
  } finally {
    submitting.value = false
  }
}

</script>

<template>
  <t-popup v-model:visible="visible" trigger="click" placement="top-left" :disabled="disabled || submitting"
    :overlay-inner-style="{ padding: '6px', borderRadius: 'var(--app-radius-lg)' }">
    <button type="button" class="approval-trigger" :disabled="disabled || submitting" aria-haspopup="menu"
      :aria-expanded="visible" :aria-label="`${modeLabel}: ${t(`agentStream.toolApproval.${selectedMode.key}Mode`)}`"
      @keydown.esc="visible = false">
      <t-icon :name="selectedMode.icon" size="14px" />
      <span>{{ t(`agentStream.toolApproval.${selectedMode.key}Mode`) }}</span>
      <t-icon name="chevron-down" size="12px" />
    </button>
    <template #content>
      <div class="approval-menu" role="menu" :aria-label="modeLabel"
        @keydown.esc="visible = false">
        <div class="approval-menu-heading">{{ t('agentStream.toolApproval.sessionPermissions') }}</div>
        <button v-for="mode in modes" :key="mode.value" type="button" role="menuitemradio"
          :aria-checked="selectedMode.value === mode.value" :disabled="disabled || submitting"
          class="approval-option" @click="select(mode.value)">
          <span class="option-copy">
            <strong>{{ t(`agentStream.toolApproval.${mode.key}Mode`) }}</strong>
            <span>{{ t(`agentStream.toolApproval.${mode.key}ModeDescription`) }}</span>
          </span>
          <t-icon v-if="selectedMode.value === mode.value" name="check" class="approval-option-check" size="16px" />
        </button>
      </div>
    </template>
  </t-popup>
</template>

<style scoped lang="less">
.approval-trigger {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 6px;
  border: 0;
  border-radius: var(--app-radius-md);
  background: transparent;
  color: var(--td-text-color-secondary);
  font: inherit;
  font-size: 12px;
  white-space: nowrap;
  cursor: pointer;
  &:hover { background: var(--td-bg-color-container-hover); }
  &:disabled { opacity: .5; cursor: default; }
}
.approval-menu { width: min(340px, 80vw); max-height: 70vh; overflow-y: auto; }
.approval-menu-heading { padding: 6px 10px 8px; color: var(--td-text-color-placeholder); font-size: 12px; }
.approval-option-check { flex-shrink: 0; align-self: flex-start; margin-top: 2px; color: var(--td-brand-color); }
.approval-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 10px;
  border: 0;
  border-radius: var(--app-radius-md);
  background: transparent;
  color: var(--td-text-color-primary);
  text-align: left;
  cursor: pointer;
  &:hover { background: var(--td-bg-color-container-hover); }
}
.option-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
  strong { font-size: 13px; font-weight: 500; }
  > span { color: var(--td-text-color-secondary); font-size: 12px; line-height: 1.5; }
}
</style>

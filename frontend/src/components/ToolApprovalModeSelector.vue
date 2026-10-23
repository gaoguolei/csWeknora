<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'

defineProps<{ disabled?: boolean }>()
const { t } = useI18n()
const settings = useSettingsStore()
const visible = ref(false)
const modes = ['auto', 'manual'] as const
const select = (mode: 'auto' | 'manual') => {
  settings.toolApprovalMode = mode
  visible.value = false
}
</script>

<template>
  <t-popup v-model:visible="visible" trigger="click" placement="top-left" :disabled="disabled"
    :overlay-inner-style="{ padding: '6px', borderRadius: 'var(--app-radius-lg)' }">
    <button type="button" class="approval-trigger" :disabled="disabled" aria-haspopup="menu"
      :aria-expanded="visible" :aria-label="`${t('agentStream.toolApproval.modeLabel')}: ${t(`agentStream.toolApproval.${settings.toolApprovalMode}Mode`)}`"
      @keydown.esc="visible = false">
      <t-icon :name="settings.toolApprovalMode === 'manual' ? 'secured' : 'check-circle'" size="14px" />
      <span>{{ t(`agentStream.toolApproval.${settings.toolApprovalMode}Mode`) }}</span>
      <t-icon name="chevron-down" size="12px" />
    </button>
    <template #content>
      <div class="approval-menu" role="menu" :aria-label="t('agentStream.toolApproval.modeLabel')"
        @keydown.esc="visible = false">
        <button v-for="mode in modes" :key="mode" type="button" role="menuitemradio"
          :aria-checked="settings.toolApprovalMode === mode" :disabled="disabled"
          class="approval-option" @click="select(mode)">
          <span class="option-copy">
            <strong>{{ t(`agentStream.toolApproval.${mode}Mode`) }}</strong>
            <span>{{ t(`agentStream.toolApproval.${mode}ModeDescription`) }}</span>
          </span>
          <t-icon v-if="settings.toolApprovalMode === mode" name="check" size="16px" />
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
.approval-menu { width: min(300px, 80vw); }
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
  &:hover, &[aria-checked='true'] { background: var(--td-bg-color-container-hover); }
}
.option-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
  strong { font-size: 13px; font-weight: 500; }
  > span { color: var(--td-text-color-secondary); font-size: 12px; line-height: 1.5; }
}
</style>

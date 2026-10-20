<template>
  <div class="section-header">
    <h2>{{ $t('agentEditor.mcp.label') }}</h2>
    <p class="section-description">{{ $t('agentEditor.mcp.desc') }}</p>
  </div>

  <div class="settings-group">
    <!-- MCP 服务选择 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.mcp.label') }}</label>
        <p class="desc">{{ $t('agentEditor.mcp.desc') }}</p>
      </div>
      <div class="setting-control">
        <t-radio-group v-model="selectionMode">
          <t-radio-button value="all">{{ $t('agentEditor.selection.all') }}</t-radio-button>
          <t-radio-button value="selected">{{ $t('agentEditor.selection.selected') }}</t-radio-button>
          <t-radio-button value="none">{{ $t('agentEditor.selection.disabled') }}</t-radio-button>
        </t-radio-group>
      </div>
    </div>

    <!-- 选择指定 MCP 服务 -->
    <div v-if="selectionMode === 'selected' && showServiceSelect" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.mcp.selectLabel') }}</label>
        <p class="desc">{{ $t('agentEditor.mcp.selectDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-select v-model="config.mcp_services" multiple
          :placeholder="$t('agentEditor.mcp.selectPlaceholder')" filterable>
          <t-option v-for="mcp in mcpOptions" :key="mcp.value" :value="mcp.value" :label="mcp.label"
            :disabled="mcp.disabled" />
        </t-select>
      </div>
    </div>

    <!-- 授权等待超时：对话中触发 OAuth 授权时的等待秒数 -->
    <div v-if="selectionMode !== 'none'" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.mcp.authWaitTimeout') }}</label>
        <p class="desc">{{ $t('agentEditor.mcp.authWaitTimeoutDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-input-number v-model="config.mcp_auth_wait_timeout" :min="5" :max="3600" theme="column"
          :placeholder="$t('agentEditor.mcp.authWaitTimeoutPlaceholder')" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { CustomAgentConfig } from '@/api/agent'

// MCP 服务分区（需求 I）。编辑器与平台「智能体配置」共用本组件。
//
// selectionMode 由父级持有：编辑器里它是一个本地 ref，并被一个 watch 同步到
// config.mcp_selection_mode；这里用 v-model 透传，保持那条 watch 仍然生效。
const props = defineProps<{
  config: CustomAgentConfig
  selectionMode: string
  /** 选「指定服务」时是否展示服务下拉（没有可用服务时不展示）。 */
  showServiceSelect: boolean
  mcpOptions: Array<{ value: string; label: string; disabled?: boolean }>
}>()

const emit = defineEmits<{ (e: 'update:selectionMode', v: string): void }>()

const selectionMode = computed({
  get: () => props.selectionMode,
  set: (v: string) => emit('update:selectionMode', v),
})
</script>

<style scoped lang="less">
@import './agentSectionStyles.less';
</style>
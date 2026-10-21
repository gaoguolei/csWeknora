<template>
  <div class="section-header">
    <h2>{{ $t('agent.editor.conversationSettings') }}</h2>
    <p class="section-description">{{ conversationSectionDesc }}</p>
  </div>

  <div class="settings-group">
    <!-- 多轮对话开关（仅普通模式：Agent 模式由 EnsureDefaults 强制开启，
         展示可关闭的开关只会被服务端改回去） -->
    <div v-if="!isAgentMode" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.multiTurn') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.multiTurn') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.multi_turn_enabled" />
      </div>
    </div>

    <!-- 保留轮数（仅普通模式：Agent 模式按上下文窗口加载历史、超出时压缩成
         摘要，见 session_agent_qa.go -> LoadAgentHistory，不读 history_turns） -->
    <div v-if="!isAgentMode && config.multi_turn_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.historyTurns') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.historyRounds') }}</p>
      </div>
      <div class="setting-control">
        <t-input-number v-model="config.history_turns" :min="1" :max="100" theme="column" />
      </div>
    </div>

    <!-- 跨轮保留检索结果。只有 agent 链路读取该值（internal/agent/observe.go），
         且改写的只是 KB/Wiki 这八个工具的历史结果，所以没有知识库时不展示。 -->
    <div v-if="isAgentMode && hasKnowledgeBase" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.retainRetrievalHistory') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.retainRetrievalHistory') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.retain_retrieval_history" />
      </div>
    </div>

    <!-- 问题改写（仅多轮对话开启且普通模式时显示） -->
    <div v-if="config.multi_turn_enabled && !isAgentMode" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.enableRewrite') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.rewrite') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.enable_rewrite" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CustomAgentConfig } from '@/api/agent'

// 多轮对话分区（需求 B）。编辑器与平台「智能体配置」共用本组件。
//
// conversationSectionDesc 只被本分区的副标题用，随模板一起搬了进来。
const props = defineProps<{
  config: CustomAgentConfig
  isAgentMode: boolean
  hasKnowledgeBase: boolean
}>()

const { t } = useI18n()

// Agent 模式下本组只剩「保留轮数」（以及有知识库时的检索保留），
// 默认文案里的「问题改写」并不展示，故按模式分开。
const conversationSectionDesc = computed(() =>
  props.isAgentMode
    ? t('agentEditor.desc.conversationSectionAgent')
    : t('agentEditor.desc.conversationSection'),
)
</script>

<style scoped lang="less">
@import './agentSectionStyles.less';
</style>
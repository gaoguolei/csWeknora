<template>
  <div class="section-header">
    <h2>{{ $t('agent.editor.retrievalStrategy') }}</h2>
    <p class="section-description">{{ $t('agentEditor.desc.retrievalSection') }}</p>
  </div>

  <div class="settings-group">
    <!-- 查询扩展（仅普通模式） -->
    <div v-if="!isAgentMode" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.enableQueryExpansion') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.queryExpansion') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.enable_query_expansion" />
      </div>
    </div>

    <!-- 向量召回TopK -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.embeddingTopK') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.embeddingTopK') }}</p>
      </div>
      <div class="setting-control">
        <t-input-number v-model="config.embedding_top_k" :min="1" :max="50" theme="column" />
      </div>
    </div>

    <!-- 关键词阈值 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.keywordThreshold') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.keywordThreshold') }}</p>
      </div>
      <div class="setting-control">
        <div class="slider-wrapper">
          <t-slider v-model="config.keyword_threshold" :min="0" :max="1" :step="0.01" />
          <span class="slider-value">{{ config.keyword_threshold?.toFixed(2) }}</span>
        </div>
      </div>
    </div>

    <!-- 向量阈值 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.vectorThreshold') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.vectorThreshold') }}</p>
      </div>
      <div class="setting-control">
        <div class="slider-wrapper">
          <t-slider v-model="config.vector_threshold" :min="0" :max="1" :step="0.01" />
          <span class="slider-value">{{ config.vector_threshold?.toFixed(2) }}</span>
        </div>
      </div>
    </div>

    <!-- 重排TopK（仅在配置了 Rerank 模型时展示） -->
    <div v-if="config.rerank_model_id" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.rerankTopK') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.rerankTopK') }}</p>
      </div>
      <div class="setting-control">
        <t-input-number v-model="config.rerank_top_k" :min="1" :max="20" theme="column" />
      </div>
    </div>

    <!-- 重排阈值（仅在配置了 Rerank 模型时展示） -->
    <div v-if="config.rerank_model_id" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.rerankThreshold') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.rerankThreshold') }}</p>
      </div>
      <div class="setting-control">
        <div class="slider-wrapper">
          <t-slider v-model="config.rerank_threshold" :min="-10" :max="10" :step="0.01" />
          <span class="slider-value">{{ config.rerank_threshold?.toFixed(1) }}</span>
        </div>
      </div>
    </div>

    <!-- FAQ 优先策略（关联 FAQ 类型知识库时显示） -->
    <div v-if="hasFaqKnowledgeBase" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.faq.enableLabel') }}</label>
        <p class="desc">{{ $t('agentEditor.faq.enableDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.faq_priority_enabled" />
      </div>
    </div>

    <div v-if="hasFaqKnowledgeBase && config.faq_priority_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.faq.thresholdLabel') }}</label>
        <p class="desc">{{ $t('agentEditor.faq.thresholdDesc') }}</p>
      </div>
      <div class="setting-control">
        <div class="slider-wrapper">
          <t-slider v-model="config.faq_direct_answer_threshold" :min="0.7" :max="1" :step="0.05" />
          <span class="slider-value">{{ config.faq_direct_answer_threshold?.toFixed(2) }}</span>
        </div>
      </div>
    </div>

    <div v-if="hasFaqKnowledgeBase && config.faq_priority_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.faq.boostLabel') }}</label>
        <p class="desc">{{ $t('agentEditor.faq.boostDesc') }}</p>
      </div>
      <div class="setting-control">
        <div class="slider-wrapper">
          <t-slider v-model="config.faq_score_boost" :min="1" :max="2" :step="0.1" />
          <span class="slider-value">{{ config.faq_score_boost?.toFixed(1) }}x</span>
        </div>
      </div>
    </div>

    <!-- 表格数据分析（仅普通模式，命中 CSV/Excel 时会多一次 LLM 调用生成 SQL） -->
    <div v-if="!isAgentMode" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agentEditor.dataAnalysis.enableLabel') }}</label>
        <p class="desc">{{ $t('agentEditor.dataAnalysis.enableDesc') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.data_analysis_enabled" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { CustomAgentConfig } from '@/api/agent'

// 检索策略分区（需求 E）。编辑器与平台「智能体配置」共用本组件。
//
// config 是父级 formData.config 的同一个对象引用，直接改它的字段即可 ——
// 这与抽取前内联写法完全一致（改 formData.config.X），保证是纯搬运。
defineProps<{
  config: CustomAgentConfig
  /** Agent（smart-reasoning）模式下检索策略由全局配置接管，部分行不展示。 */
  isAgentMode: boolean
  /** 关联了 FAQ 类型知识库时才展示 FAQ 优先策略。 */
  hasFaqKnowledgeBase: boolean
}>()
</script>

<style scoped lang="less">
@import './agentSectionStyles.less';
</style>
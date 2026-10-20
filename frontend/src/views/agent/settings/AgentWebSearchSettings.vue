<template>
  <div class="section-header">
    <h2>{{ $t('agent.editor.webSearchConfig') }}</h2>
    <p class="section-description">{{ $t('agent.editor.webSearchConfigDesc') }}</p>
  </div>

  <div class="settings-group">
    <!-- 网络搜索 -->
    <div class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.webSearch') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.webSearch') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.web_search_enabled" />
      </div>
    </div>

    <!-- 搜索引擎选择 -->
    <div v-if="config.web_search_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.webSearchProvider') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.webSearchProvider') }}</p>
      </div>
      <div class="setting-control">
        <t-select v-model="config.web_search_provider_id" clearable
          :placeholder="$t('agent.editor.webSearchProviderPlaceholder')" style="width: 240px;">
          <t-option v-for="p in webSearchProviderList" :key="p.id" :value="p.id" :label="p.name">
            <span>{{ p.name }}</span>
            <t-tag v-if="p.is_default" theme="primary" size="small" style="margin-left: 6px;">{{
              $t('common.default')
              }}</t-tag>
          </t-option>
        </t-select>
      </div>
    </div>

    <!-- 网络搜索最大结果数 -->
    <div v-if="config.web_search_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.webSearchMaxResults') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.webSearchMaxResults') }}</p>
      </div>
      <div class="setting-control">
        <div class="slider-wrapper">
          <t-slider v-model="config.web_search_max_results" :min="1" :max="10" />
          <span class="slider-value">{{ config.web_search_max_results }}</span>
        </div>
      </div>
    </div>

    <!-- 自动抓取页面内容 -->
    <div v-if="config.web_search_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.webFetchEnabled') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.webFetchEnabled') }}</p>
      </div>
      <div class="setting-control">
        <t-switch v-model="config.web_fetch_enabled" />
      </div>
    </div>

    <!-- 抓取页面数 -->
    <div v-if="config.web_search_enabled && config.web_fetch_enabled" class="setting-row">
      <div class="setting-info">
        <label>{{ $t('agent.editor.webFetchTopN') }}</label>
        <p class="desc">{{ $t('agentEditor.desc.webFetchTopN') }}</p>
      </div>
      <div class="setting-control">
        <div class="slider-wrapper">
          <t-slider v-model="config.web_fetch_top_n" :min="1" :max="10" />
          <span class="slider-value">{{ config.web_fetch_top_n }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { CustomAgentConfig } from '@/api/agent'

// 网络搜索分区（需求 F）。编辑器与平台「智能体配置」共用本组件。
//
// 需求里这一项是"由超级管理员统一配置，并且自动获取运维部署时配置的搜索引擎"，
// 所以搜索引擎列表由父级传入（部署级配置），本组件只负责选择。
defineProps<{
  config: CustomAgentConfig
  /** 可选的搜索引擎列表（来自部署级「网络搜索」配置）。id 可能缺失（旧数据），
   *  与编辑器里的实体类型保持一致，所以这里声明为可选。 */
  webSearchProviderList: Array<{ id?: string; name: string; is_default?: boolean }>
}>()
</script>

<style scoped lang="less">
@import './agentSectionStyles.less';
</style>
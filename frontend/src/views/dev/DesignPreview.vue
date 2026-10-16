<template>
  <div class="design-preview">
    <aside class="preview-nav">
      <strong>WeKnora</strong>
      <p>界面预览 · 示例数据</p>
      <button v-for="item in pages" :key="item.key" type="button" :aria-pressed="page === item.key" @click="page = item.key">
        <t-icon :name="item.icon" />{{ item.label }}
      </button>
      <div class="preview-theme">
        <button type="button" @click="setTheme('light')">浅色模式</button>
        <button type="button" @click="setTheme('dark')">暗色模式</button>
        <button type="button" @click="settingsOpen = true">打开设置</button>
      </div>
    </aside>
    <WorkspaceWelcome v-if="page === 'welcome'" :mode="intent" @select="intent = $event">
      <div class="preview-composer"><t-textarea placeholder="描述你的想法，开始一段新对话…" :autosize="{ minRows: 3 }" /><t-button theme="primary">开始对话</t-button></div>
    </WorkspaceWelcome>
    <main v-else class="preview-page">
      <header class="header">
        <div class="header-title"><div class="title-row"><h2>{{ pages.find(p => p.key === page)?.label }}</h2><t-button theme="primary">新建知识库</t-button></div><p class="header-subtitle">整理知识、连接工具，让每一次探索都有据可循。</p></div>
      </header>
      <div class="preview-toolbar"><div class="preview-filters"><button v-for="item in ['全部', '收藏', '最近', '我的空间']" :key="item" :aria-pressed="filter === item" @click="filter = item">{{ item }}</button></div><t-input class="preview-search" v-model="query" placeholder="搜索知识库" clearable><template #prefix-icon><t-icon name="search" /></template></t-input></div>
      <div class="preview-content">
        <div v-if="page === 'resources'" class="preview-grid">
          <article v-for="item in resources.filter(r => r.title.includes(query))" :key="item.title" class="preview-card" tabindex="0">
            <div class="card-header"><t-icon :name="item.icon" size="24px" /><span class="card-title">{{ item.title }}</span></div>
            <div class="card-content"><p class="card-description">{{ item.description }}</p></div>
            <div class="card-bottom"><t-tag theme="default">{{ item.count }} 个文档</t-tag><span>刚刚更新</span></div>
          </article>
        </div>
        <template v-else>
          <t-table :data="resources" :columns="columns" row-key="title" hover />
          <EmptyState icon="folder-open" title="为下一次探索积累知识" description="上传文档后，便可在对话中引用、检索和整理。"><t-button variant="outline">上传文档</t-button></EmptyState>
        </template>
      </div>
    </main>
    <SettingsModalShell :visible="settingsOpen" v-model="section" title="设置" :nav-groups="groups" @close="settingsOpen = false">
      <div class="preview-settings">
        <header class="section-header"><h2>通用设置</h2><p class="section-description">让工作空间符合你的使用习惯。</p></header>
        <div class="setting-row"><div class="setting-info"><label>外观</label><p class="desc">浅色、暗色，或跟随系统。</p></div><div class="setting-control"><t-select :value="currentTheme" :options="themes" @change="(value: unknown) => setTheme(value as ThemeMode)" /></div></div>
        <div class="setting-row"><div class="setting-info"><label>自动保存</label><p class="desc">保留未完成的输入，继续上次的思路。</p></div><div class="setting-control"><t-switch v-model="enabled" /></div></div>
        <div class="setting-row"><div class="setting-info"><label>工作空间名称</label><p class="desc">示例表单，展示输入和校验状态。</p></div><div class="setting-control"><t-input value="我的工作空间" /></div></div>
        <t-alert theme="warning" message="示例提示：请先完成模型配置。" />
        <div class="preview-statuses"><t-input status="error" placeholder="请填写必填项" /><t-input disabled placeholder="不可编辑" /><t-select placeholder="选择语言" :options="[{ label: '简体中文', value: 'zh' }, { label: 'English', value: 'en' }]" /></div>
      </div>
      <template #footer><t-button variant="outline" @click="settingsOpen = false">取消</t-button><t-button theme="primary" @click="settingsOpen = false">完成</t-button></template>
    </SettingsModalShell>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import WorkspaceWelcome from '@/components/workspace/WorkspaceWelcome.vue'
import SettingsModalShell from '@/components/SettingsModalShell.vue'
import EmptyState from '@/components/EmptyState.vue'
import { useTheme, type ThemeMode } from '@/composables/useTheme'
import type { WorkspaceIntent } from '@/utils/workspaceEvents'
const { currentTheme, setTheme } = useTheme()
const page = ref('resources'), query = ref(''), filter = ref('全部'), section = ref('general')
const settingsOpen = ref(false), enabled = ref(true), intent = ref<WorkspaceIntent>('chat')
const pages = [{ key: 'welcome', label: '新对话', icon: 'chat' }, { key: 'resources', label: '知识库', icon: 'folder' }, { key: 'documents', label: '文档与空状态', icon: 'file' }]
const resources = [
  { title: '产品手册', description: '功能说明、操作指南与常见问题，汇集到一个地方。', count: 12, icon: 'book-open' },
  { title: '团队知识', description: '保存讨论结论，记录每个项目的重要决定。', count: 8, icon: 'usergroup' },
  { title: '设计资料', description: '从灵感到实现，一起探索更好的工作方式。', count: 21, icon: 'image' },
]
const columns = [{ colKey: 'title', title: '名称' }, { colKey: 'description', title: '说明', ellipsis: true }, { colKey: 'count', title: '文档', width: 80 }]
const themes = [{ label: '浅色', value: 'light' }, { label: '暗色', value: 'dark' }, { label: '跟随系统', value: 'system' }]
const groups = [{ key: 'preferences', label: '偏好设置', items: [{ key: 'general', label: '通用设置', icon: 'setting' }, { key: 'models', label: '模型设置', icon: 'control-platform' }] }]
</script>

<style scoped lang="less">
@import (reference) '@/components/css/resource-card.less';
@import (reference) '@/components/css/artifact-filter-tabs.less';
@import (reference) '@/components/css/settings-section.less';
.design-preview { display: flex; height: 100dvh; background: var(--td-bg-color-container); }
.preview-nav { width: 200px; flex-shrink: 0; display: flex; flex-direction: column; padding: 28px 16px; gap: 8px; background: var(--td-bg-color-sidebar); border-right: 1px solid var(--td-component-stroke); box-sizing: border-box; strong { font-size: 22px; } p { font-size: 12px; color: var(--td-text-color-secondary); } button { display: flex; align-items: center; gap: 8px; width: 100%; padding: 10px; border: 1px solid transparent; border-radius: var(--app-radius-md); background: transparent; color: var(--td-text-color-primary); text-align: left; cursor: pointer; &[aria-pressed='true'], &:hover { background: var(--app-selection-bg); border-color: var(--td-component-stroke); } } }
.preview-theme { margin-top: auto; }
.preview-page { .workspace-page(); }
.resource-list-header();
.header-title { display: flex; flex-direction: column; }.title-row { display: flex; align-items: center; }.header-subtitle { margin: 0; }
.preview-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 16px; padding-bottom: 16px; border-bottom: 1px solid var(--td-component-stroke); > .preview-search { width: 240px; margin-left: auto; } }
.preview-filters { .artifact-filter-tabs(); }
.preview-content { .resource-list-main(); }
.preview-grid { .resource-card-grid(); }
.preview-card { .resource-card(); .card-bottom { color: var(--td-text-color-secondary); font-size: 12px; } }
.preview-settings { padding: 32px; }.section-header { .settings-section-header(); }.setting-row { .setting-row(); }.setting-info { .setting-info(); }.setting-control { .setting-control(); }.preview-statuses { display: grid; gap: 16px; margin-top: 24px; }
.preview-composer { border: 1px solid var(--td-component-stroke); padding: 16px; border-radius: var(--app-radius-2xl); box-shadow: var(--app-surface-shadow); > .t-button { margin-top: 12px; } }
@media (max-width: 680px) { .preview-nav { width: 132px; padding: 16px 8px; } .preview-settings { padding: 20px; } }
</style>

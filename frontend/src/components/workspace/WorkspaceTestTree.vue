<template>
  <div class="test-tree">
    <details v-for="project in projects" :key="project.id" open>
      <summary><strong>{{ project.name }}</strong><span>{{ project.path || '/' }} · {{ project.framework }}</span></summary>
      <p v-if="!project.targets.length">{{ t('workspace.validationNoTargets') }}</p>
      <div v-for="target in project.targets" :key="target.id" class="test-target">
        <t-icon :name="target.kind === 'tests' ? 'task' : 'check-circle'" size="14px" />
        <div><span>{{ target.file || (target.id.includes(':configured:') ? target.name : t(`workspace.validationKind_${target.kind}`)) }}</span><small v-if="target.scope === 'package'">{{ t('workspace.validationPackageScope') }}</small><small v-if="results[target.id]">{{ t(`workspace.validationState_${results[target.id]}`) }}</small></div>
        <button type="button" :disabled="running" :aria-label="t('workspace.run') + ' ' + target.name" @click="emit('run', target)"><t-icon name="play-circle" size="16px" /></button>
      </div>
    </details>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { TestProject, TestTarget } from '@/api/workspaceTests'
defineProps<{ projects: TestProject[]; results: Record<string, string>; running: boolean }>()
const emit = defineEmits<{ run: [target: TestTarget] }>()
const { t } = useI18n()
</script>
<style scoped lang="less">
details { border-bottom: 1px solid var(--td-component-stroke); padding: 9px 0; }
summary { cursor: pointer; font-size: 12px; padding: 3px 0; }
summary span { margin-left: 8px; color: var(--td-text-color-placeholder); font-size: 11px; }
.test-target { display: flex; align-items: center; gap: 8px; min-height: 34px; padding-left: 12px; font-size: 12px; }
.test-target > div { flex: 1; min-width: 0; overflow-wrap: anywhere; }
small { margin-left: 8px; color: var(--td-text-color-secondary); }
button { border: 0; background: transparent; color: var(--td-text-color-secondary); padding: 5px; cursor: pointer; }
button:disabled { opacity: .4; cursor: not-allowed; }
button:focus-visible, summary:focus-visible { outline: 2px solid var(--td-brand-color); }
</style>

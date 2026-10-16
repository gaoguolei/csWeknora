<template>
  <section class="workspace-browser">
    <WorkspaceUrlPreview :key="sessionId" :session-id="sessionId" :agent-id="agentId" :agent-source-tenant-id="agentSourceTenantId" :revision="revision" :active="active" @ask="emit('ask', $event)" @open-terminal="emit('open-terminal')" />
  </section>
</template>

<script setup lang="ts">
import WorkspaceUrlPreview from './WorkspaceUrlPreview.vue'
defineProps<{ sessionId: string; agentId?: string; agentSourceTenantId?: string | number | null; active: boolean; revision?: number }>()
const emit = defineEmits<{ ask: [prompt: string]; 'open-terminal': [] }>()
</script>

<style scoped lang="less">
.workspace-browser { display: flex; flex: 1; flex-direction: column; min-width: 0; min-height: 0; height: 100%; background: var(--td-bg-color-container); }
</style>

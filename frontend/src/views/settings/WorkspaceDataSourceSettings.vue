<template>
  <div class="workspace-ds-settings">
    <div class="section-header">
      <h2>{{ $t('datasource.title') }}</h2>
      <p class="section-description">{{ $t('datasource.description') }}</p>
    </div>

    <!-- 数据源挂在具体知识库下，因此先选目标库，再复用知识库设置里的数据源管理 UI。 -->
    <div class="workspace-ds-bar">
      <label class="workspace-ds-bar__label">{{ $t('kbDefaults.datasource.targetKb') }}</label>
      <t-select
        v-model="selectedKbId"
        class="workspace-ds-bar__select"
        filterable
        :placeholder="$t('kbDefaults.datasource.selectKb')"
      >
        <t-option v-for="kb in kbList" :key="kb.id" :value="kb.id" :label="kb.name" />
      </t-select>
    </div>
    <p class="workspace-ds-hint">{{ $t('kbDefaults.datasource.hint') }}</p>

    <t-loading :loading="loading" size="small">
      <DataSourceSettings v-if="selectedKbId" :key="selectedKbId" :kb-id="selectedKbId" />
      <p v-else class="workspace-ds-empty">{{ $t('kbDefaults.datasource.selectKb') }}</p>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import DataSourceSettings from '@/views/knowledge/settings/DataSourceSettings.vue'
import { listKnowledgeBases } from '@/api/knowledge-base'

// 空间级数据源管理：由空间管理员配置，对本空间所有知识库生效。
// 数据源本身仍归属具体知识库，这里只提供集中入口。
const loading = ref(false)
const kbList = ref<{ id: string; name: string }[]>([])
const selectedKbId = ref('')

async function loadKBs() {
  loading.value = true
  try {
    const res: any = await listKnowledgeBases()
    const arr = res?.data && Array.isArray(res.data) ? res.data : []
    kbList.value = arr.map((kb: any) => ({ id: kb.id, name: kb.name }))
    if (!selectedKbId.value && kbList.value.length) {
      selectedKbId.value = kbList.value[0].id
    }
  } catch (e) {
    console.error('workspaceDataSource: failed to load knowledge bases', e)
    kbList.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadKBs()
})
</script>

<style lang="less" scoped>
.workspace-ds-settings {
  width: 100%;
  padding-bottom: 24px;
}

.section-header {
  margin-bottom: 20px;

  h2 {
    font-size: var(--app-text-3xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 6px 0;
  }

  .section-description {
    font-size: var(--app-text-base);
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.workspace-ds-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;

  &__label {
    font-size: var(--app-text-lg);
    font-weight: 500;
    color: var(--td-text-color-primary);
    flex: 0 0 auto;
  }

  &__select {
    flex: 0 0 320px;
    max-width: 320px;
  }
}

.workspace-ds-hint,
.workspace-ds-empty {
  font-size: var(--app-text-base);
  color: var(--td-text-color-secondary);
  margin: 0 0 12px 0;
  line-height: 1.5;
}
</style>

<template>
  <div
    class="theme-toggle-btn"
    role="button"
    tabindex="0"
    :aria-label="t('theme.theme')"
    :title="t('theme.theme')"
    @click="toggleTheme"
    @keydown.enter.prevent="toggleTheme"
    @keydown.space.prevent="toggleTheme"
  >
    <!-- 空心线框版：浅色太阳 / 深色月亮，与登录页同款同尺寸。 -->
    <t-icon :name="isDark ? 'moon' : 'sunny'" size="18px" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTheme } from '@/composables/useTheme'

const { t } = useI18n()
const { effectiveTheme, setTheme } = useTheme()

// 只有浅/深两态。历史遗留的 “system” 选择在点击时按当前生效值
// 归一化为 light/dark，之后不再回到 system。
// 图标必须读 effectiveTheme（响应式）：切到“跟随系统”或系统换主题时
// theme-mode 属性变化不会触发重算，读 DOM 会让图标停在旧状态。
const isDark = computed(() => effectiveTheme.value === 'dark')

function toggleTheme() {
  setTheme(isDark.value ? 'light' : 'dark')
}
</script>

<style scoped>
.theme-toggle-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: var(--app-radius-sm);
  color: var(--td-text-color-secondary);
  cursor: pointer;
  flex-shrink: 0;
  transition: background var(--app-motion-fast) ease, color var(--app-motion-fast) ease;
}

.theme-toggle-btn:hover {
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-primary);
}

.theme-toggle-btn:focus-visible {
  outline: 2px solid var(--td-brand-color);
  outline-offset: 1px;
}
</style>

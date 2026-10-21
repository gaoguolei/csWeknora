<template>
  <section class="workspace-welcome">
    <!-- <header class="welcome-topbar">
      <span class="workspace-wordmark">WeKnora <span>Workspace</span></span>
      <button type="button" class="knowledge-link" @click="$emit('knowledge')"><t-icon name="folder" />{{
        t('workspace.knowledgeLink') }}<t-icon name="arrow-right-up" /></button>
    </header> -->
    <main class="welcome-content">
      <div class="welcome-intro">
        <img class="welcome-symbol" :src="thinkerflowMark" alt="" aria-hidden="true" />
        <h1>{{ t("workspace.title") }}</h1>
        <div class="welcome-mode-bar">
          <div class="welcome-modes" role="group" :aria-label="t('workspace.modeLabel')">
            <!-- 选中滑块：随激活项左右平滑滑动，代替固定背景 -->
            <span class="welcome-mode-thumb" :class="{ 'is-code': activeMode === 'coding', 'is-testing': activeMode === 'testing' }" aria-hidden="true"></span>
            <button type="button" :aria-pressed="activeMode === 'chat'"
              :disabled="disabled" @click="$emit('select', 'chat')">
              <t-icon name="chat" size="16px" />{{ t('workspace.modeChat') }}
            </button>
            <button type="button" :aria-pressed="activeMode === 'coding'"
              :disabled="disabled" @click="$emit('select', 'code')">
              <t-icon name="code" size="16px" />{{ t('workspace.modeCoding') }}
            </button>
            <button type="button" :aria-pressed="activeMode === 'testing'"
              :disabled="disabled" @click="$emit('select', 'test')">
              <t-icon name="check-circle" size="16px" />{{ t('workspace.modeTesting') }}
            </button>
          </div>
        </div>
      </div>
      <div class="welcome-composer">
        <slot />
      </div>
      <div class="welcome-suggestions">
        <slot name="suggestions" />
      </div>
    </main>
    <!-- <footer class="welcome-footer">WeKnora <span>·</span> {{ t('workspace.panelHint') }}</footer> -->
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import thinkerflowMark from "@/assets/img/thinkerflow-mark.svg";
import type { WorkspaceIntent } from "@/utils/workspaceEvents";
import { composerModeFromIntent } from "@/utils/composerMode";
const props = defineProps<{ mode?: WorkspaceIntent; disabled?: boolean }>();
defineEmits<{ select: [intent: WorkspaceIntent]; knowledge: [] }>();
const { t } = useI18n();
const activeMode = computed(() => composerModeFromIntent(props.mode || 'chat'));
</script>

<style scoped lang="less">
.workspace-welcome {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: auto;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
}

.welcome-topbar {
  min-height: 68px;
  padding: 0 32px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

// .workspace-wordmark {
//   font-size: var(--app-text-2xl);
//   font-weight: 650;
//   letter-spacing: -.6px;

//   span {
//     font-size: var(--app-text-sm);
//     color: var(--td-text-color-placeholder);
//     font-weight: 400;
//     margin-left: 10px;
//     letter-spacing: 0;
//   }
// }

.knowledge-link {
  display: flex;
  gap: 8px;
  align-items: center;
  border: 0;
  background: transparent;
  padding: 8px;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-sm);
  border-radius: var(--app-radius-md);

  &:hover {
    background: var(--td-bg-color-container-hover);
  }
}

.welcome-content {
  width: calc(100% - 64px);
  max-width: 760px;
  /* 垂直居中基础上整体上移：下边距大于上边距，视觉重心靠上一些 */
  margin: 18vh auto 18vh;
  padding: 46px 0 0;
}

.welcome-intro {
  text-align: center;
  margin-bottom: 32px;

  h1 {
    font-family: var(--app-font-family-display);
    font-size: clamp(28px, 3vw, 40px);
    font-weight: 500;
    letter-spacing: -1.3px;
    line-height: 1.4;
    margin: 12px 0;
    text-wrap: balance;
  }
}

.welcome-symbol {
  /* 品牌标记：与侧边栏同用 thinkerflow-mark.svg（透明底蓝色双条，左高右矮、右条偏下）。
     SVG 是竖长比例（viewBox 21×55），固定成正方形会被居中留出两侧空白，
     所以用 height 定尺寸、width 自适应。 */
  display: block;
  height: 40px;
  width: auto;
  /* 眉标行去掉后补一点间距，保持标记与标题原来的呼吸感 */
  margin: 0 auto 28px;
}

.welcome-mode-bar {
  display: flex;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 20px;
  animation: welcome-rise .45s cubic-bezier(.22, .8, .3, 1) both;
}

.welcome-modes {
  position: relative;
  isolation: isolate;
  display: inline-grid;
  grid-template-columns: repeat(3, 1fr);
  padding: 3px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-pill);
  background: var(--td-bg-color-secondarycontainer);

  button {
    position: relative;
    z-index: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    min-width: 84px;
    padding: 9px 16px;
    border: 0;
    border-radius: var(--app-radius-pill);
    background: transparent;
    color: var(--td-text-color-secondary);
    font: inherit;
    font-size: var(--app-text-sm);
    cursor: pointer;
    transition: color .25s ease, transform .15s ease;

    &:hover:not([aria-pressed="true"]):not(:disabled) {
      color: var(--td-text-color-primary);
    }

    &[aria-pressed="true"] {
      /* 背景交给滑块，选中项文字用反白 */
      color: #fff;
    }

    &:active {
      transform: scale(.96);
    }

    &:focus-visible {
      outline: 2px solid var(--td-text-color-secondary);
      outline-offset: 3px;
    }

    &:disabled { cursor: default; opacity: .6; }
  }
}

.welcome-mode-thumb {
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 3px;
  width: calc((100% - 6px) / 3);
  border-radius: var(--app-radius-pill);
  /* 品牌蓝滑块，深色/浅色模式下都与底色拉开对比 */
  background: var(--brand-main);
  box-shadow: 0 2px 10px color-mix(in srgb, var(--brand-main) 40%, transparent);
  transition: transform .3s cubic-bezier(.22, .8, .3, 1);

  &.is-code {
    transform: translateX(100%);
  }

  &.is-testing {
    transform: translateX(200%);
  }
}

.welcome-composer {
  position: relative;
  animation: welcome-rise .45s cubic-bezier(.22, .8, .3, 1) .08s both;

  :deep(.answers-input) {
    width: 100%;
    max-width: none;
    position: static;
    transform: none;
  }
}

@keyframes welcome-rise {
  from {
    opacity: 0;
    transform: translateY(14px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.welcome-suggestions:empty {
  display: none;
}

// .welcome-footer {
//   text-align: center;
//   color: var(--td-text-color-placeholder);
//   font-size: var(--app-text-2xs);
//   padding: 16px;

//   span {
//     margin: 0 10px;
//   }
// }

@media (max-width: 680px) {
  .welcome-topbar {
    padding: 0 16px;
  }

  .workspace-wordmark span {
    display: none;
  }

  .welcome-content {
    width: calc(100% - 32px);
    padding-top: 24px;
  }

  .welcome-intro h1 {
    letter-spacing: -0.5px;
  }

}
</style>

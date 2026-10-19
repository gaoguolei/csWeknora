<template>
  <section class="workspace-welcome">
    <!-- <header class="welcome-topbar">
      <span class="workspace-wordmark">WeKnora <span>Workspace</span></span>
      <button type="button" class="knowledge-link" @click="$emit('knowledge')"><t-icon name="folder" />{{
        t('workspace.knowledgeLink') }}<t-icon name="arrow-right-up" /></button>
    </header> -->
    <div class="welcome-mode-bar">
      <div class="welcome-modes" role="group" :aria-label="t('workspace.modeLabel')">
        <button type="button" :aria-pressed="composerModeFromIntent(mode || 'chat') === 'chat'"
          :disabled="disabled" @click="$emit('select', 'chat')">
          <t-icon name="chat" size="16px" />{{ t('workspace.modeChat') }}
        </button>
        <button type="button" :aria-pressed="composerModeFromIntent(mode || 'chat') === 'coding'"
          :disabled="disabled" @click="$emit('select', 'code')">
          <t-icon name="code" size="16px" />{{ t('workspace.modeCoding') }}
        </button>
      </div>
    </div>
    <main class="welcome-content">
      <div class="welcome-intro">
        <div class="welcome-symbol" aria-hidden="true">
          <span></span><span></span><span></span>
        </div>
        <p class="welcome-eyebrow">{{ t("workspace.eyebrow") }}</p>
        <h1>{{ t("workspace.title") }}</h1>
        <p class="welcome-description">{{ t("workspace.description") }}</p>
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
import { useI18n } from "vue-i18n";
import type { WorkspaceIntent } from "@/utils/workspaceEvents";
import { composerModeFromIntent } from "@/utils/composerMode";
defineProps<{ mode?: WorkspaceIntent; disabled?: boolean }>();
defineEmits<{ select: [intent: WorkspaceIntent]; knowledge: [] }>();
const { t } = useI18n();
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
  margin: auto;
  padding: 46px 0 64px;
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
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  height: 36px;
  margin: 0 auto 20px;
  transform: rotate(-12deg);

  span {
    display: block;
    width: 7px;
    height: 27px;
    background: var(--td-text-color-primary);
    border-radius: var(--app-radius-sm);

    &:nth-child(2) {
      height: 38px;
    }

    &:last-child {
      height: 19px;
      align-self: flex-end;
    }
  }
}

.welcome-eyebrow {
  font-size: var(--app-text-xs);
  letter-spacing: 2px;
  color: var(--td-text-color-placeholder);
}

.welcome-description {
  font-size: var(--app-text-base);
  color: var(--td-text-color-secondary);
  margin: 0;
}

.welcome-mode-bar {
  position: sticky;
  top: 0;
  z-index: 5;
  display: flex;
  justify-content: center;
  flex-shrink: 0;
  padding: 18px 16px;
  background: var(--td-bg-color-container);
}

.welcome-modes {
  display: inline-flex;
  padding: 3px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-pill);
  background: var(--td-bg-color-secondarycontainer);

  button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    min-width: 96px;
    padding: 9px 18px;
    border: 0;
    border-radius: var(--app-radius-pill);
    background: transparent;
    color: var(--td-text-color-secondary);
    font: inherit;
    font-size: var(--app-text-sm);
    cursor: pointer;

    &[aria-pressed="true"] {
      background: var(--td-bg-color-container);
      color: var(--td-text-color-primary);
    }

    &:focus-visible {
      outline: 2px solid var(--td-text-color-secondary);
      outline-offset: 3px;
    }

    &:disabled { cursor: default; opacity: .6; }
  }
}

.welcome-composer {
  position: relative;

  :deep(.answers-input) {
    width: 100%;
    max-width: none;
    position: static;
    transform: none;
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

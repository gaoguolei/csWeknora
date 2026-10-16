<template>
  <section class="workspace-welcome">
    <!-- <header class="welcome-topbar">
      <span class="workspace-wordmark">WeKnora <span>Workspace</span></span>
      <button type="button" class="knowledge-link" @click="$emit('knowledge')"><t-icon name="folder" />{{
        t('workspace.knowledgeLink') }}<t-icon name="arrow-right-up" /></button>
    </header> -->
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
      <nav class="workspace-actions" :aria-label="t('workspace.recipes')">
        <button
          v-for="action in actions"
          :key="action.id"
          type="button"
          :class="{ selected: mode === action.id }"
          :aria-pressed="mode === action.id"
          @click="$emit('select', action.id)"
        >
          <t-icon :name="action.icon" size="17px" /><span>{{
            t(`workspace.${action.id}`)
          }}</span>
        </button>
      </nav>
      <p class="welcome-hint">{{ t("workspace.hint") }}</p>
      <div class="welcome-suggestions">
        <slot name="suggestions" />
      </div>
      <div class="welcome-recipes">
        <button type="button" @click="$emit('select', 'build')">
          <span
            class="recipe-illustration recipe-illustration--web"
            aria-hidden="true"
            ><i></i><b></b><em></em></span
          ><span
            ><strong>{{ t("workspace.recipeBuild") }}</strong
            ><small>{{ t("workspace.recipeBuildDesc") }}</small></span
          ><t-icon name="arrow-right-up" />
        </button>
        <button type="button" @click="$emit('select', 'knowledge')">
          <span
            class="recipe-illustration recipe-illustration--book"
            aria-hidden="true"
            ><i></i><b></b><em></em></span
          ><span
            ><strong>{{ t("workspace.recipeKnowledge") }}</strong
            ><small>{{ t("workspace.recipeKnowledgeDesc") }}</small></span
          ><t-icon name="arrow-right-up" />
        </button>
      </div>
    </main>
    <!-- <footer class="welcome-footer">WeKnora <span>·</span> {{ t('workspace.panelHint') }}</footer> -->
  </section>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { WorkspaceIntent } from "@/utils/workspaceEvents";
defineProps<{ mode?: WorkspaceIntent }>();
defineEmits<{ select: [intent: WorkspaceIntent]; knowledge: [] }>();
const { t } = useI18n();
const actions = [
  { id: "build", icon: "code" },
  { id: "code", icon: "code-1" },
  { id: "test", icon: "check-circle" },
  { id: "knowledge", icon: "book-open" },
] as const;
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

button {
  font: inherit;
  cursor: pointer;
  color: inherit;
  transition:
    background var(--app-motion-base) ease,
    transform 0.18s ease;

  &:focus-visible {
    outline: 2px solid var(--td-text-color-secondary);
    outline-offset: 3px;
  }

  &:active {
    transform: translateY(1px);
  }
}

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

.welcome-composer {
  position: relative;

  :deep(.answers-input) {
    width: 100%;
    max-width: none;
    position: static;
    transform: none;
  }
}

.workspace-actions {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 20px;

  button {
    display: flex;
    align-items: center;
    gap: 8px;
    border: 1px solid var(--td-component-stroke);
    background: transparent;
    border-radius: var(--app-radius-pill);
    padding: 9px 16px;
    font-size: var(--app-text-sm);

    &:hover,
    &.selected {
      background: var(--td-bg-color-secondarycontainer);
      border-color: var(--td-component-border);
    }
  }
}

.welcome-hint {
  text-align: center;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-xs);
  margin: 20px 0 0;
}

.welcome-suggestions:empty {
  display: none;
}

.welcome-recipes {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-top: 46px;

  button {
    display: flex;
    align-items: center;
    gap: 16px;
    text-align: left;
    border: 1px solid var(--td-component-stroke);
    border-radius: var(--app-radius-xl);
    background: transparent;
    padding: 20px 18px;

    &:hover {
      background: var(--td-bg-color-container-hover);
    }

    > .t-icon {
      margin-left: auto;
      flex-shrink: 0;
      color: var(--td-text-color-placeholder);
    }

    strong {
      display: block;
      font-size: var(--app-text-md);
      font-weight: 500;
    }

    small {
      display: block;
      font-size: var(--app-text-xs);
      margin-top: 6px;
      color: var(--td-text-color-placeholder);
    }
  }
}

.recipe-illustration {
  position: relative;
  flex-shrink: 0;
  width: 44px;
  height: 40px;
  color: var(--td-text-color-secondary);

  i,
  b,
  em {
    position: absolute;
    display: block;
    border: 1px solid currentColor;
    border-radius: var(--app-radius-xs);
  }

  &--web {
    i {
      inset: 4px 0 4px;
      transform: rotate(-7deg);
    }

    b {
      width: 19px;
      height: 16px;
      left: 6px;
      top: 12px;
    }

    em {
      width: 9px;
      height: 1px;
      right: 5px;
      top: 17px;
      border-width: 1px 0 0;
      box-shadow: 0 5px currentColor;
    }
  }

  &--book {
    i {
      inset: 2px 9px 2px 3px;
      transform: rotate(-12deg);
      opacity: 0.4;
    }

    b {
      inset: 3px 3px 2px 10px;
      background: var(--td-bg-color-container);
      transform: rotate(8deg);
    }

    em {
      width: 15px;
      left: 16px;
      top: 14px;
      border-width: 1px 0 0;
      border-radius: 0;
      transform: rotate(8deg);
      box-shadow: 0 6px currentColor;
    }
  }
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

  .welcome-recipes {
    grid-template-columns: 1fr;
    margin-top: 28px;
  }

  .welcome-intro h1 {
    letter-spacing: -0.5px;
  }

  .workspace-actions {
    gap: 8px;

    button {
      padding: 8px 12px;
    }
  }
}

@media (prefers-reduced-motion: reduce) {
  button {
    transition: none;
  }
}
</style>

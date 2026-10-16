<template>
  <Transition name="sandbox-panel" :duration="{ enter: 240, leave: 300 }">
    <div v-show="panel?.visible.value" class="chat-sandbox-panel-clip">
      <aside
        class="chat-sandbox-panel"
        :class="{ 'is-shifted': shifted, 'is-resizing': resizing }"
        :style="{
          width: `${panel?.width.value ?? 420}px`,
          '--references-shift': `${shiftWidth}px`,
        }"
        role="complementary"
        :aria-label="t('workspace.panelTitle')"
        @keydown.esc.stop="panel?.close()"
      >
        <!-- 左缘拖拽把手：按住向左/右拖动调整面板宽度。 -->
        <PanelResizeHandle
          edge="left"
          :label="t('knowledgeStages.resizeDrawer')"
          :value="panel?.width.value ?? 560"
          :min="SANDBOX_PANEL_MIN_WIDTH"
          :max="SANDBOX_PANEL_MAX_WIDTH"
          @start="startResize"
          @resize="resizePanel"
          @end="resizing = false"
        />
        <div class="chat-sandbox-panel__tabs">
          <div class="chat-sandbox-panel__tablist" role="tablist">
            <button
              v-for="tab in tabs"
              :key="tab.id"
              type="button"
              class="chat-sandbox-panel__tab"
              :class="{ 'is-active': panel?.activeTab.value === tab.id }"
              role="tab"
              :aria-selected="panel?.activeTab.value === tab.id"
              @click="panel?.open(tab.id)"
            >
              <t-icon :name="tab.icon" size="16px" />
              <span>{{ tab.label }}</span>
              <span
                v-if="tab.id === 'artifacts' && artifacts.length"
                class="chat-sandbox-panel__tab-count"
                aria-hidden="true"
                >{{ artifacts.length }}</span
              >
            </button>
          </div>
          <t-dropdown
            :options="moreOptions"
            trigger="click"
            placement="bottom-right"
            attach="body"
            @click="openMoreTool"
          >
            <button
              type="button"
              class="chat-sandbox-panel__tab chat-sandbox-panel__more"
              :class="{ 'is-active': Boolean(activeMoreTool) }"
              :aria-label="t('workspace.moreTools')"
              aria-haspopup="menu"
            >
              <t-icon :name="activeMoreTool?.icon || 'ellipsis'" size="16px" />
              <span>{{
                activeMoreTool?.label || t("workspace.moreTools")
              }}</span>
              <t-icon v-if="activeMoreTool" name="chevron-down" size="12px" />
            </button>
          </t-dropdown>
          <button
            type="button"
            class="chat-sandbox-panel__close"
            :aria-label="t('common.close')"
            @click="panel?.close()"
          >
            <t-icon name="close" size="16px" />
          </button>
        </div>

        <div
          class="chat-sandbox-panel__body"
          :class="{
            'is-flush': ['artifacts', 'preview', 'source', 'tests'].includes(
              panel?.activeTab.value || '',
            ),
          }"
        >
          <ConversationWorkspace
            v-if="workspaceMounted"
            v-show="isWorkspaceTab"
            :session-id="sessionId"
            :tab="workspaceTab"
            :active="Boolean(panel?.visible.value && isWorkspaceTab)"
            :revision="workspaceRevision"
            :focus-path="workspacePath"
            :agent-runs="agentRuns"
            :agent-id="agentId"
            :agent-source-tenant-id="agentSourceTenantId"
            :gateway="workspaceGateway"
            @ask="emit('ask', $event)"
            @report-created="emit('refreshArtifacts', $event)"
            @view-change="emit('workspaceViewChange', $event)"
          />
          <ChatArtifactsPanel
            v-show="panel?.activeTab.value === 'artifacts'"
            class="chat-sandbox-panel__artifacts"
            :session-id="sessionId"
            :items="artifacts"
            :collecting="artifactsCollecting"
            :active="
              Boolean(
                panel?.visible.value && panel?.activeTab.value === 'artifacts',
              )
            "
            @deleted="emit('artifactDeleted', $event)"
          />

          <!-- 终端：首次激活时惰性挂载；切 tab 用 v-show 保留实例（不丢 PTY）。 -->
          <SandboxTerminal
            v-if="terminalMounted"
            v-show="panel?.activeTab.value === 'terminal'"
            :key="sessionId"
            ref="terminalRef"
            :session-id="sessionId"
            :agent-id="agentId"
            :agent-source-tenant-id="agentSourceTenantId"
            class="chat-sandbox-panel__terminal"
          />
          <div
            v-else-if="panel?.activeTab.value === 'terminal'"
            class="chat-sandbox-panel__placeholder"
          >
            <t-skeleton
              animation="gradient"
              :row-col="[{ width: '100%', height: '100%', type: 'rect' }]"
            />
          </div>

          <!-- Web preview does not require remote desktop capabilities. -->
          <WorkspaceBrowserPreview
            v-if="previewMounted && !workspaceGateway"
            v-show="panel?.activeTab.value === 'preview'"
            :key="sessionId"
            :session-id="sessionId"
            :agent-id="agentId"
            :agent-source-tenant-id="agentSourceTenantId"
            :active="
              Boolean(
                panel?.visible.value && panel?.activeTab.value === 'preview',
              )
            "
            :revision="workspaceRevision"
            class="chat-sandbox-panel__preview"
            @ask="emit('ask', $event)"
            @open-terminal="panel?.open('terminal')"
          />
        </div>
      </aside>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, h, nextTick, ref, watch } from "vue";
import { Icon as TIcon } from "tdesign-vue-next";
import type { WorkspaceGateway } from "@/components/workspace/ConversationWorkspace.vue";
import type {
  WorkspaceTab,
  WorkspaceCommandRun,
} from "@/utils/workspaceEvents";
const ConversationWorkspace = defineAsyncComponent(
  () => import("@/components/workspace/ConversationWorkspace.vue"),
);
import { useI18n } from "vue-i18n";
import {
  useChatSandboxPanel,
  SANDBOX_PANEL_MIN_WIDTH,
  SANDBOX_PANEL_MAX_WIDTH,
  type SandboxPanelTab,
} from "@/composables/useChatSandboxPanel";
import SandboxTerminal from "@/views/chat/components/SandboxTerminal.vue";
import WorkspaceBrowserPreview from "@/components/workspace/WorkspaceBrowserPreview.vue";
import ChatArtifactsPanel from "@/views/chat/components/ChatArtifactsPanel.vue";
import PanelResizeHandle from "@/components/PanelResizeHandle.vue";
import type { SessionArtifactItem } from "@/utils/sessionArtifacts";

const props = withDefaults(
  defineProps<{
    sessionId: string;
    /** 当前会话选中的 agent（首次连接时按其配置自动创建沙箱）。 */
    agentId?: string;
    /** 共享智能体来源空间，缺省表示本空间自有 agent。 */
    agentSourceTenantId?: string | number | null;
    /** 参考来源面板同开时整体左移，避免两块 fixed 面板重叠。 */
    shifted?: boolean;
    /** 参考来源面板当前宽度（查看原文时会变宽），左移的距离。 */
    shiftWidth?: number;
    artifacts?: SessionArtifactItem[];
    artifactsCollecting?: boolean;
    workspaceRevision?: number;
    workspacePath?: string;
    agentRuns?: WorkspaceCommandRun[];
    workspaceGateway?: WorkspaceGateway;
  }>(),
  {
    artifacts: () => [],
    artifactsCollecting: false,
    shiftWidth: 420,
  },
);

// The artifact list is owned by the chat view (a computed over the loaded
// history), so a delete inside the panel has to travel back up to it.
const emit = defineEmits<{
  (e: "refreshArtifacts", messageId?: string): void;
  (e: "artifactDeleted", payload: { messageId: string; index: number }): void;
  (e: "ask", prompt: string): void;
  (e: "workspaceViewChange", view: "source" | "diff" | "preview"): void;
}>();

const { t } = useI18n();
const panel = useChatSandboxPanel();
watch(
  () => [props.sessionId, panel?.visible.value, panel?.activeTab.value],
  () => {
    if (
      props.sessionId &&
      panel?.visible.value &&
      panel.activeTab.value === "artifacts"
    )
      emit("refreshArtifacts");
  },
);
const workspaceMounted = ref(false);
const isWorkspaceTab = computed(
  () =>
    ["source", "tests"].includes(panel?.activeTab.value || "") ||
    (Boolean(props.workspaceGateway) && panel?.activeTab.value === "preview"),
);
const workspaceTab = computed(() =>
  isWorkspaceTab.value ? (panel!.activeTab.value as WorkspaceTab) : "source",
);
watch(
  () => [panel?.visible.value, isWorkspaceTab.value],
  ([visible, workspace]) => {
    if (visible && workspace) workspaceMounted.value = true;
  },
  { immediate: true },
);

const tabs = computed(() => {
  const list: Array<{ id: SandboxPanelTab; icon: string; label: string }> = [
    { id: "preview", icon: "desktop", label: t("workspace.preview") },
    { id: "source", icon: "code", label: t("workspace.source") },
    { id: "tests", icon: "check-circle", label: t("workspace.tests") },
    { id: "artifacts", icon: "folder", label: t("chat.sandbox.tabArtifacts") },
  ];
  return list;
});

// Less frequent tools remain available without occupying the default tab bar.
const moreTools = computed(() => [
  { id: "terminal", icon: "terminal", label: t("chat.sandbox.tabTerminal") },
]);
const activeMoreTool = computed(() =>
  moreTools.value.find((tool) => tool.id === panel?.activeTab.value),
);
const moreOptions = computed(() =>
  moreTools.value.map((tool) => ({
    content: tool.label,
    prefixIcon: () => h(TIcon, { name: tool.icon, size: "16px" }),
    value: tool.id,
    active: tool.id === panel?.activeTab.value,
  })),
);
function openMoreTool(option: { value?: unknown }) {
  if (option.value === "terminal") panel?.open(option.value);
}

// 终端 / 预览惰性挂载（首次切到对应 tab 时），面板关闭即销毁（v-if），
// 与 ChatReferencesDrawer 的开合行为一致；会话切换时由 :key 重建。
const terminalMounted = ref(false);
const terminalRef = ref<{ focus?: () => void } | null>(null);
const previewMounted = ref(false);

watch(
  () => [panel?.visible.value, panel?.activeTab.value] as const,
  ([visible, tab]) => {
    if (!visible) {
      // Drop the lazy-mount flags so reopening on Files does not remount
      // either panel (which would connect and refresh TTL).
      terminalMounted.value = false;
      previewMounted.value = false;
      return;
    }
    if (tab === "terminal") {
      terminalMounted.value = true;
      void nextTick(() => terminalRef.value?.focus?.());
      return;
    }
    if (tab === "preview") {
      // The web preview checks the workspace without starting a sandbox.
      previewMounted.value = true;
    }
  },
  { immediate: true },
);

watch(
  () => props.sessionId,
  () => {
    panel?.clearArtifactFocus();
  },
);

// --- 左缘拖拽调宽 -------------------------------------------------------
const resizing = ref(false);
let resizeStartWidth = 0;
function startResize() {
  if (!panel) return;
  resizeStartWidth = panel.width.value;
  resizing.value = true;
}
function resizePanel(delta: number) {
  panel?.setWidth(resizeStartWidth - delta);
}
</script>

<style scoped lang="less">
.chat-sandbox-panel-clip {
  position: fixed;
  inset: 0;
  z-index: 1201;
  overflow: hidden;
  pointer-events: none;
}

.chat-sandbox-panel {
  pointer-events: auto;
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: min(420px, 100vw);
  max-width: 100vw;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-container);
  border-left: 1px solid var(--td-component-stroke);
  box-shadow: -6px 0 24px rgba(0, 0, 0, 0.025);

  @media (max-width: 959px) {
    width: 100% !important;
    max-width: 100%;
  }

  &.is-shifted {
    @media (min-width: 1400px) {
      right: var(--references-shift, 420px);
    }
  }

  // 拖拽调宽期间关闭过渡与文本选择，保证跟手。
  &.is-resizing {
    transition: none;
    user-select: none;

    .chat-sandbox-panel__body,
    .chat-sandbox-panel__tabs {
      pointer-events: none;
    }
  }
}

.chat-sandbox-panel__close {
  border: 0;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  width: 28px;
  height: 28px;
  border-radius: var(--app-radius-md);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition:
    background var(--app-motion-fast) ease,
    color var(--app-motion-fast) ease;

  &:hover {
    background: color-mix(
      in srgb,
      var(--td-text-color-primary) 8%,
      var(--td-bg-color-secondarycontainer)
    );
    color: var(--td-text-color-primary);
  }
}

.chat-sandbox-panel__tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  height: var(--app-chat-header-height);
  box-sizing: border-box;
  padding: 0 12px;
  border-bottom: 1px solid var(--td-component-stroke);
  flex-shrink: 0;
}

.chat-sandbox-panel__tablist {
  display: flex;
  gap: 4px;
  min-width: 0;
  flex: 1;
  overflow-x: auto;
}

.chat-sandbox-panel__tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 8px;
  line-height: 20px;
  border: 0;
  border-radius: var(--app-radius-sm);
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-md);
  cursor: pointer;
  white-space: nowrap;
  transition:
    background-color var(--app-motion-fast) ease,
    color var(--app-motion-fast) ease;

  &:hover {
    color: var(--td-text-color-primary);
    background: var(--td-bg-color-container-hover);
  }

  &.is-active {
    color: var(--td-text-color-primary);
    background: var(--td-bg-color-secondarycontainer);
    font-weight: 500;
  }

  &:focus-visible {
    outline: 2px solid var(--td-text-color-secondary);
    outline-offset: -2px;
  }
}

.chat-sandbox-panel__tab-count {
  min-width: 16px;
  height: 16px;
  padding: 0 5px;
  border-radius: var(--app-radius-md);
  background: color-mix(in srgb, var(--td-text-color-primary) 8%, transparent);
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-xs);
  line-height: 16px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.chat-sandbox-panel__more {
  flex-shrink: 0;
}

.chat-sandbox-panel__body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 8px;

  &.is-flush {
    padding: 0;
  }
}

.chat-sandbox-panel__terminal,
.chat-sandbox-panel__preview,
.chat-sandbox-panel__artifacts {
  flex: 1;
  min-height: 0;
  min-width: 0;
}

.chat-sandbox-panel__placeholder {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-md);

  p {
    margin: 0;
  }
}

.sandbox-panel-enter-active .chat-sandbox-panel {
  transition:
    transform 0.24s cubic-bezier(0.22, 0.61, 0.36, 1),
    opacity 0.24s cubic-bezier(0.22, 0.61, 0.36, 1);
}

.sandbox-panel-leave-active .chat-sandbox-panel {
  transition:
    transform 0.3s cubic-bezier(0.22, 0.61, 0.36, 1),
    opacity 0.3s cubic-bezier(0.22, 0.61, 0.36, 1);
}

.sandbox-panel-enter-from .chat-sandbox-panel,
.sandbox-panel-leave-to .chat-sandbox-panel {
  transform: translateX(100%);
  opacity: 0.6;
}

:global(:root[theme-mode="dark"]) .chat-sandbox-panel {
  box-shadow: -6px 0 24px rgba(0, 0, 0, 0.12);
}
</style>

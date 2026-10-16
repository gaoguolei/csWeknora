<template>
  <div
    class="workspace-fixture"
    :style="{
      '--fixture-panel': panel.visible.value ? `${panel.width.value}px` : '0px',
    }"
  >
    <aside class="fixture-nav">
      <div class="fixture-brand">WeKnora<span>Workspace</span></div>
      <button
        type="button"
        class="new-task"
        @click="
          home = true;
          panel.close();
        "
      >
        <t-icon name="add" />新建对话</button
      ><button type="button" @click="knowledgeOpen = !knowledgeOpen">
        <t-icon name="folder" />知识库
      </button>
      <div v-if="knowledgeOpen" class="fixture-knowledge">
        开发验收页使用示例数据。正式知识库入口位于登录后的侧栏，支持上传、选择和引用。
      </div>
      <span class="nav-caption">开发验收</span
      ><button type="button" @click="showConversation">对话与工作面板</button
      ><button
        type="button"
        @click="
          session = 'empty';
          showConversation();
        "
      >
        空工作区</button
      ><button
        type="button"
        @click="
          session = 'error';
          showConversation();
        "
      >
        读取失败</button
      ><button
        type="button"
        @click="
          session = 'sample';
          showConversation();
        "
      >
        示例工作区
      </button>
      <footer>
        <span class="fixture-avatar">W</span
        ><span>本地预览<small>仅开发环境可访问</small></span>
      </footer>
    </aside>
    <div class="fixture-main">
      <WorkspaceWelcome
        v-if="home"
        :mode="intent"
        @select="selectIntent"
        @knowledge="knowledgeOpen = !knowledgeOpen"
        ><form class="fixture-composer" @submit.prevent="showConversation">
          <textarea
            v-model="prompt"
            aria-label="描述任务"
            placeholder="把任务交给 WeKnora，或者从一个问题开始…"
          ></textarea>
          <div>
            <button
              type="button"
              aria-label="查看知识库说明"
              @click="knowledgeOpen = !knowledgeOpen"
            >
              <t-icon name="add" /></button
            ><span>智能推理 <t-icon name="chevron-down" /></span
            ><button type="submit" aria-label="打开示例对话" class="send">
              <t-icon name="arrow-up" />
            </button>
          </div></form
      ></WorkspaceWelcome>
      <main v-else class="fixture-chat">
        <header>
          <span>做一个轻量的灵感记录网站</span
          ><button
            type="button"
            @click="panel.open('preview')"
            :aria-label="t('workspace.openPanel')"
          >
            <t-icon name="view-column" />
          </button>
        </header>
        <div class="fixture-messages">
          <div class="sample-user">
            帮我做一个简洁的灵感记录网站，支持记录今天的一条灵感。
          </div>
          <div class="sample-assistant">
            <span class="assistant-mark">W</span>
            <div>
              <p>我会做一个安静、专注的小页面，包含灵感卡片和记录按钮。</p>
              <div class="sample-step">
                <t-icon name="check" />已生成页面结构与样式
                <span>3 个示例文件</span>
              </div>
              <p>
                页面已在右侧打开。可以切换到「代码」查看文件，或进入「测试」体验成功与失败状态。
              </p>
              <button
                type="button"
                class="sample-artifact"
                @click="panel.open('preview')"
              >
                <t-icon name="code" /><span
                  >Fieldnotes · 灵感记录<small
                    >index.html · 页面预览</small
                  ></span
                ><t-icon name="arrow-right-up" />
              </button>
            </div>
          </div>
        </div>
        <form
          class="fixture-composer chat-composer"
          @submit.prevent="panel.open('tests')"
        >
          <textarea
            v-model="prompt"
            aria-label="继续描述任务"
            placeholder="继续描述你想调整的地方…"
          ></textarea>
          <div>
            <span>示例交互 · 不调用模型</span
            ><button type="submit" aria-label="打开测试面板" class="send">
              <t-icon name="arrow-up" />
            </button>
          </div>
        </form>
      </main>
    </div>
    <SandboxSidePanel
      :session-id="session"
      :workspace-gateway="gateway"
      @ask="prompt = $event"
    />
    <div class="fixture-disclaimer">{{ t("workspace.fixture") }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { files } from "./workspaceFixture";
import { useI18n } from "vue-i18n";
import WorkspaceWelcome from "@/components/workspace/WorkspaceWelcome.vue";
import SandboxSidePanel from "@/components/chat/SandboxSidePanel.vue";
import type { WorkspaceGateway } from "@/components/workspace/ConversationWorkspace.vue";
import { provideFileChanges } from "@/composables/useFileChanges";
import { provideChatSandboxPanel } from "@/composables/useChatSandboxPanel";
import { intentPanelTab, type WorkspaceIntent } from "@/utils/workspaceEvents";
const { t } = useI18n();
const panel = provideChatSandboxPanel();
const fileChanges = provideFileChanges();
const home = ref(true),
  knowledgeOpen = ref(false),
  intent = ref<WorkspaceIntent>("chat"),
  prompt = ref(""),
  session = ref("sample");
function selectIntent(value: WorkspaceIntent) {
  intent.value = value;
  if (value !== "chat") prompt.value = t(`workspace.${value}Prompt`);
}
function showConversation() {
  fileChanges.clearChanges();
  if (session.value === "sample") {
    for (const path of ["index.html", "src/main.ts", "app.js"]) {
      const after = files[path];
      const lines = after.split("\n");
      const before = lines.map((line, index) => index === 2 ? "// Previous example content" : line).join("\n");
      const intermediate = lines.slice(0, -1).join("\n");
      fileChanges.addChange({ id: `${path}:1`, path, type: 'modified', before, after: intermediate,
        addedLines: 1, removedLines: 2, timestamp: Date.now(), live: false, turnId: 'sample-turn' });
      fileChanges.addChange({ id: `${path}:2`, path, type: 'modified', before: intermediate, after,
        addedLines: 1, removedLines: 0, timestamp: Date.now(), live: false, turnId: 'sample-turn' });
    }
  }
  home.value = false;
  panel.open(intentPanelTab(intent.value) || "preview");
}

const gateway: WorkspaceGateway = {
  async tree(id, path) {
    if (id === "error") throw new Error("Fixture failure");
    if (id === "empty")
      return { root: "/workspace", origin: "sandbox" as const, nodes: [] };
    const prefix = path ? `${path}/` : "",
      dirs = new Set<string>(),
      leaves: string[] = [];
    for (const key of Object.keys(files)) {
      if (!key.startsWith(prefix)) continue;
      const rest = key.slice(prefix.length),
        slash = rest.indexOf("/");
      if (slash === -1) leaves.push(rest);
      else dirs.add(rest.slice(0, slash));
    }
    return {
      root: "/workspace",
      origin: "sandbox" as const,
      nodes: [
        ...[...dirs].sort().map((name) => ({
          path: `${prefix}${name}`,
          name,
          kind: "directory" as const,
        })),
        ...leaves.sort().map((name) => ({
          path: `${prefix}${name}`,
          name,
          kind: "file" as const,
        })),
      ],
    };
  },
  async file(_id, path) {
    if (!(path in files)) throw new Error("Missing fixture file");
    return {
      path,
      content: files[path],
      size: new Blob([files[path]]).size,
      hash: "fixture",
    };
  },
  async run(_id, command) {
    await new Promise((resolve) => setTimeout(resolve, 650));
    return {
      stdout:
        "Development fixture: simulated command output.\n" +
        (command.includes("fail")
          ? "FAIL: example assertion\nExpected: 2\nReceived: 1"
          : "PASS: example assertion"),
      stderr: "",
      exit_code: command.includes("fail") ? 1 : 0,
      duration_ms: 650,
      killed: false,
    };
  },
};
</script>

<style scoped lang="less">
.workspace-fixture {
  display: flex;
  width: 100%;
  height: 100dvh;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
}
.fixture-nav {
  width: 224px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-sidebar);
  padding: 28px 14px;
  border-right: 1px solid var(--td-component-stroke);
  gap: 5px;
  button {
    display: flex;
    align-items: center;
    gap: 10px;
    text-align: left;
    padding: 12px;
    background: transparent;
    border: 0;
    color: inherit;
    font: inherit;
    font-size: var(--app-text-sm);
    cursor: pointer;
    border-radius: var(--app-radius-md);
    &:hover {
      background: var(--td-bg-color-container-hover);
    }
  }
  .new-task {
    background: var(--td-bg-color-container);
    border: 1px solid var(--td-component-stroke);
    margin-bottom: 14px;
  }
  footer {
    margin-top: auto;
    display: flex;
    gap: 10px;
    font-size: var(--app-text-sm);
    align-items: center;
    small {
      display: block;
      color: var(--td-text-color-placeholder);
      font-size: var(--app-text-2xs);
      margin-top: 4px;
    }
  }
}
.fixture-brand {
  font-size: var(--app-text-4xl);
  letter-spacing: -1px;
  font-weight: 600;
  margin: 0 10px 30px;
  span {
    font-size: var(--app-text-2xs);
    font-weight: 400;
    color: var(--td-text-color-placeholder);
    letter-spacing: 2px;
    display: block;
    margin-top: 6px;
  }
}
.nav-caption {
  font-size: var(--app-text-2xs);
  color: var(--td-text-color-placeholder);
  margin: 30px 12px 10px;
}
.fixture-avatar {
  background: var(--td-bg-color-component);
  padding: 8px 11px;
  border-radius: 50%;
}
.fixture-main {
  display: flex;
  flex: 1;
  min-width: 0;
  padding-right: var(--fixture-panel);
}
.fixture-composer {
  border: 1px solid var(--td-component-border);
  border-radius: var(--app-radius-2xl);
  padding: 18px;
  background: var(--td-bg-color-container);
  box-shadow: 0 5px 24px #302d2606;
  textarea {
    width: 100%;
    height: 65px;
    resize: none;
    background: transparent;
    border: 0;
    outline: none;
    font: inherit;
    font-size: var(--app-text-md);
    line-height: 1.8;
    color: inherit;
  }
  > div {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 12px;
    color: var(--td-text-color-secondary);
    font-size: var(--app-text-xs);
  }
  button {
    border: 0;
    width: 28px;
    height: 28px;
    border-radius: 50%;
    cursor: pointer;
    color: inherit;
    background: var(--td-bg-color-secondarycontainer);
    &.send {
      margin-left: auto;
      color: var(--td-bg-color-container);
      background: var(--td-text-color-primary);
    }
  }
}
.fixture-disclaimer {
  position: fixed;
  bottom: 0;
  padding: 5px 12px;
  text-align: center;
  font-size: var(--app-text-2xs);
  width: 100%;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-placeholder);
  z-index: 1300;
}
.fixture-chat {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  > header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 26px;
    font-size: var(--app-text-sm);
    button {
      background: transparent;
      border: 0;
      color: inherit;
      cursor: pointer;
    }
  }
}
.fixture-messages {
  flex: 1;
  overflow: auto;
  padding: 40px 28px;
  font-size: var(--app-text-md);
  line-height: 1.9;
}
.sample-user {
  padding: 14px 18px;
  border-radius: var(--app-radius-xl);
  background: var(--td-bg-color-secondarycontainer);
  margin: 0 0 38px 30px;
}
.sample-assistant {
  display: flex;
  gap: 14px;
}
.assistant-mark {
  font-family: Georgia, serif;
  font-size: var(--app-text-4xl);
  padding-top: 8px;
}
.sample-step {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 0;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-secondary);
  span {
    color: var(--td-text-color-placeholder);
  }
}
.sample-artifact {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid var(--td-component-stroke);
  background: transparent;
  color: inherit;
  padding: 14px;
  border-radius: var(--app-radius-lg);
  text-align: left;
  cursor: pointer;
  small {
    display: block;
    font-size: var(--app-text-2xs);
    color: var(--td-text-color-placeholder);
  }
  > .t-icon:last-child {
    margin-left: auto;
  }
}
.chat-composer {
  margin: 12px 24px 40px;
}
.fixture-knowledge {
  padding: 12px;
  font-size: var(--app-text-xs);
  line-height: 1.8;
  color: var(--td-text-color-secondary);
}
@media (max-width: 959px) {
  .fixture-main {
    padding-right: 0;
  }
  .fixture-nav {
    width: 180px;
  }
}
@media (max-width: 680px) {
  .fixture-nav {
    display: none;
  }
}
</style>

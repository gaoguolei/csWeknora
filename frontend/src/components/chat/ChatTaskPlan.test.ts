import assert from 'node:assert/strict'
import { after, before, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer, type ViteDevServer } from 'vite'
import vue from '@vitejs/plugin-vue'
import { createSSRApp, h, type Component } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createI18n } from 'vue-i18n'
import zh from '../../i18n/locales/zh-CN'

let server: ViteDevServer
let TaskPlan: Component

before(async () => {
  const mocks: Record<string, string> = {
    '@/api/mcp-service': 'export async function resolveToolApproval() {}',
    '@/composables/usePendingToolApproval': `
      import { ref } from 'vue';
      export const isUnavailableToolApproval = () => false;
      export const usePendingToolApproval = () => ({ unavailable: ref(false), checking: ref(false), markUnavailable() {} });
    `,
    'tdesign-vue-next': 'export const MessagePlugin = { info() {}, error() {} };',
  }
  server = await createServer({
    configFile: false,
    optimizeDeps: { noDiscovery: true, entries: [] },
    plugins: [{
      name: 'task-plan-test', enforce: 'pre',
      resolveId(id) { if (id.startsWith('\0mock:')) return id },
      load(id) { if (id.startsWith('\0mock:')) return mocks[id.slice(6)] },
    }, vue()],
    resolve: { alias: [
      ...Object.keys(mocks).map(find => ({ find, replacement: '\0mock:' + find })),
      { find: '@', replacement: fileURLToPath(new URL('../../', import.meta.url)) },
    ] },
    server: { middlewareMode: true, hmr: false }, appType: 'custom',
  })
  TaskPlan = (await server.ssrLoadModule('/src/components/chat/ChatTaskPlan.vue')).default
})
after(async () => { await server?.close() })

const steps = [
  { id: '1', description: '设计页面', status: 'completed' },
  { id: '2', description: '编写文件', status: 'in_progress' },
  { id: '3', description: '验证页面', status: 'pending' },
  { id: '4', description: '说明结果', status: 'pending' },
]
const progress = (planSteps = steps) => ({
  type: 'tool_call', tool_name: 'todo_write', pending: false, success: true,
  tool_call_id: 'todo', tool_data: { task: '页面制作', steps: planSteps },
})
const approval = (resolved = true, approved = true) => ({
  type: 'tool_approval_required', service_name: 'WeKnora', mcp_tool_name: 'approve_plan',
  pending_id: 'plan-approval', resolved, approved,
  args_json: JSON.stringify({ task: '页面制作', steps: steps.map(step => ({ ...step, status: 'pending' })) }),
})

type Message = { id?: string; role: string; is_completed?: boolean; agentEventStream?: unknown[] }
async function render(messages: Message[], running: boolean, planMode = false) {
  const app = createSSRApp({ render: () => h(TaskPlan, { messages, running, planMode }) })
  app.use(createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zh } }))
  app.component('t-icon', { render: () => null })
  return renderToString(app)
}

test('ordinary progress is visible while its reply is running', async () => {
  const html = await render([{ role: 'assistant', is_completed: false, agentEventStream: [progress()] }], true)
  assert.match(html, /class="chat-task-plan"/)
  assert.match(html, /1 \/ 4/)
})

test('ordinary progress disappears after reply completion even if todo statuses are stale', async () => {
  for (const running of [false, true]) {
    const html = await render([{ role: 'assistant', is_completed: true, agentEventStream: [progress()] }], running)
    assert.doesNotMatch(html, /class="chat-task-plan"/, 'completed replies must not leave a docked checklist')
  }
  assert.equal(steps[1].status, 'in_progress', 'hiding progress must not mark unverified work complete')
})

test('ordinary progress is hidden when execution stops before completion arrives', async () => {
  const html = await render([{ role: 'assistant', is_completed: false, agentEventStream: [progress()] }], false)
  assert.doesNotMatch(html, /class="chat-task-plan"/)
})

test('a new request does not revive ordinary progress from a completed reply', async () => {
  const html = await render([
    { role: 'assistant', is_completed: true, agentEventStream: [progress()] },
    { role: 'user' },
    { role: 'assistant', is_completed: false, agentEventStream: [] },
  ], true)
  assert.doesNotMatch(html, /class="chat-task-plan"/)
})

test('a new ordinary checklist starts with its own statuses', async () => {
  const html = await render([
    { role: 'assistant', is_completed: true, agentEventStream: [progress()] },
    { role: 'user' },
    { role: 'assistant', is_completed: false, agentEventStream: [progress(steps.map(step => ({ ...step, status: 'pending' })))] },
  ], true)
  assert.match(html, /0 \/ 4/)
})

test('approved unfinished plans remain visible across completed replies and follow-ups', async () => {
  const html = await render([
    { role: 'assistant', is_completed: true, agentEventStream: [approval(), progress()] },
    { role: 'user' },
    { role: 'assistant', is_completed: true, agentEventStream: [] },
  ], false, true)
  assert.match(html, /class="chat-task-plan"/)
  assert.match(html, /1 \/ 4/)
  assert.match(html, /还有未完成的任务/)
})

test('leaving plan mode dismisses an approved historical checklist', async () => {
  for (const completed of [true, false]) {
    const html = await render([
      { role: 'assistant', is_completed: completed, agentEventStream: [approval(), progress()] },
    ], false, false)
    assert.doesNotMatch(html, /class="chat-task-plan"/)
  }
})

test('a non-plan follow-up starts a fresh checklist after an approved plan', async () => {
  const html = await render([
    { role: 'assistant', is_completed: true, agentEventStream: [approval(), progress()] },
    { role: 'user' },
    { role: 'assistant', is_completed: false, agentEventStream: [progress(steps.map(step => ({ ...step, status: 'pending' })))] },
  ], true, false)
  assert.match(html, /0 \/ 4/)
})

test('pending approvals still render controls while rejected and completed plans stay hidden', async () => {
  const pending = await render([{ role: 'assistant', is_completed: false, agentEventStream: [approval(false)] }], true)
  assert.match(pending, /批准并执行/)
  for (const events of [
    [approval(true, false)],
    [approval(), progress(steps.map(step => ({ ...step, status: 'completed' })))],
  ]) {
    const html = await render([{ role: 'assistant', is_completed: true, agentEventStream: events }], false, true)
    assert.doesNotMatch(html, /class="chat-task-plan"/)
  }
})

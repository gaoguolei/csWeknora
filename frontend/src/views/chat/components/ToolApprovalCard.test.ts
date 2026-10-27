import assert from 'node:assert/strict'
import { after, before, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer, type ViteDevServer } from 'vite'
import vue from '@vitejs/plugin-vue'
import { createSSRApp, h, type Component, type VNode } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createI18n } from 'vue-i18n'
import zh from '../../../i18n/locales/zh-CN'

let server: ViteDevServer
let Card: Component
let approvals: { requests: Array<{ pendingId: string; body: Record<string, unknown> }> }
before(async () => {
  const mocks: Record<string, string> = {
    '@/api/mcp-service': 'export const requests = []; export async function resolveToolApproval(pendingId, body) { requests.push({pendingId, body}); }',
    '@/api/embed': 'export async function resolveEmbedToolApproval() { throw new Error("Unexpected embedded request"); }',
    '@/composables/usePendingToolApproval': `
      import { ref } from 'vue';
      export const isUnavailableToolApproval = () => false;
      export const usePendingToolApproval = () => ({ unavailable: ref(false), checking: ref(false), markUnavailable() {} });
    `,
    'tdesign-vue-next': 'export const MessagePlugin = { success() {}, error(message) { throw new Error(message); } };',
  }
  server = await createServer({
    configFile: false, optimizeDeps: { noDiscovery: true, entries: [] },
    plugins: [{ name: 'approval-test', enforce: 'pre',
      resolveId(id) { if (id.startsWith('\0mock:')) return id },
      load(id) { if (id.startsWith('\0mock:')) return mocks[id.slice(6)] },
    }, vue()],
    resolve: { alias: [
      ...Object.keys(mocks).map(find => ({ find, replacement: '\0mock:' + find })),
      { find: '@', replacement: fileURLToPath(new URL('../../../', import.meta.url)) },
    ] },
    server: { middlewareMode: true, hmr: false }, appType: 'custom',
  })
  Card = (await server.ssrLoadModule('/src/views/chat/components/ToolApprovalCard.vue')).default
  approvals = await server.ssrLoadModule('\0mock:@/api/mcp-service') as typeof approvals
})
after(async () => { await server?.close() })

async function render(props: Record<string, unknown> = {}) {
  let vnode!: VNode
  const app = createSSRApp({ render: () => (vnode = h(Card, {
    pendingId: 'pending', serviceName: 'WeKnora', mcpToolName: 'shell_exec',
    argsJson: '{"command":"rm -rf private-path"}', ...props,
  })) })
  app.use(createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zh } }))
  app.component('t-icon', { render: () => null })
  const html = await renderToString(app)
  return { html, state: (vnode.component as any).setupState }
}

test('approval shows a readable action and decision buttons without commands or an editor', async () => {
  const { html } = await render()
  assert.match(html, /等待审核|执行命令/)
  assert.match(html, /通过并执行/)
  assert.match(html, /拒绝/)
  assert.doesNotMatch(html, /rm -rf|private-path|textarea|调用参数|已修改/)
  const resolved = await render({ resolved: true, approved: true })
  assert.match(resolved.html, /已通过/)
  assert.doesNotMatch(resolved.html, /rm -rf|private-path|<button/)
})

test('approval and rejection submit decisions without replacing original arguments', async () => {
  approvals.requests.length = 0
  const approved = await render()
  await approved.state.submit('approve')
  assert.deepEqual(approvals.requests[0], {
    pendingId: 'pending', body: { decision: 'approve', reason: undefined },
  })
  await approved.state.submit('approve')
  assert.equal(approvals.requests.length, 1, 'duplicate submissions are ignored')
  const rejected = await render()
  await rejected.state.submit('reject')
  assert.equal(approvals.requests[1].body.decision, 'reject')
  assert.equal(Object.hasOwn(approvals.requests[1].body, 'modified_args'), false)
})

test('built-in approval records omit the service name without leaving a dangling separator', async () => {
  for (const approved of [true, false]) {
    const { html } = await render({ resolved: true, approved })
    assert.match(html, /人工审核 · 执行沙箱命令/)
    assert.doesNotMatch(html, /WeKnora|›|\{service\}/)
  }
  const pending = await render()
  assert.match(pending.html, /等待审核/)
  assert.doesNotMatch(pending.html, /WeKnora|›|\{service\}/)
  const external = await render({ serviceName: 'Orders', mcpToolName: 'lookup', resolved: true, approved: true })
  assert.match(external.html, /Orders › lookup/)
})

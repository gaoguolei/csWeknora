import test from 'node:test'
import assert from 'node:assert/strict'
import { createRenderer, nextTick, ref } from 'vue'
import type { discoverProgrammingWebPreview, ProgrammingWebPreview } from '../api/programming'
import { useWorkspaceWebPreview } from './useWorkspaceWebPreview'

const ready: ProgrammingWebPreview = { status: 'ready', url: 'http://wkpreview-demo.localhost:8080/', preview_id: 'demo' }
const response = (data: ProgrammingWebPreview) => ({ success: true, data, $httpStatus: 200 })

function mount(discover: typeof discoverProgrammingWebPreview) {
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { location: { href: 'http://localhost:5173/' } } })
  const session = ref('a')
  let controls!: ReturnType<typeof useWorkspaceWebPreview>
  const renderer = createRenderer<any, any>({ patchProp() {}, insert() {}, remove() {}, setText() {}, setElementText() {}, createElement: () => ({}), createText: () => ({}), createComment: () => ({}), parentNode: () => null, nextSibling: () => null })
  const app = renderer.createApp({ setup() { controls = useWorkspaceWebPreview(session, ref(true), ref(0), discover); return () => null } })
  app.mount({})
  return { session, controls, close: () => { app.unmount(); Reflect.deleteProperty(globalThis, 'window') } }
}

test('status checks do not build; toolbar refresh requests a new build', async () => {
  const starts: boolean[] = []
  const view = mount(async (_session, options) => {
    starts.push(Boolean(options?.start))
    return response(options?.start ? { status: 'starting' } : ready)
  })
  try {
    await view.controls.refresh()
    assert.deepEqual(starts, [false])
    assert.equal(view.controls.currentUrl.value, ready.url)
    view.controls.reload()
    await nextTick()
    assert.deepEqual(starts, [false, true])
    assert.equal(view.controls.currentUrl.value, '')
    assert.equal(view.controls.result.value?.status, 'starting')
  } finally { view.close() }
})

test('rebuilding hides the old artifact immediately and failure keeps it hidden', async () => {
  let resolve!: (value: ReturnType<typeof response>) => void
  const view = mount(async (_session, options) => options?.start ? new Promise(r => { resolve = r }) : response(ready))
  try {
    await view.controls.refresh()
    const rebuilding = view.controls.refresh(true)
    assert.equal(view.controls.currentUrl.value, '')
    assert.equal(view.controls.result.value?.status, 'starting')
    resolve(response({ status: 'start_failed', detail: 'build failed' }))
    await rebuilding
    assert.equal(view.controls.currentUrl.value, '')
    assert.equal(view.controls.result.value?.status, 'start_failed')
  } finally { view.close() }
})

test('a late build response cannot replace another conversation preview', async () => {
  let resolve!: (value: ReturnType<typeof response>) => void
  const view = mount(() => new Promise(r => { resolve = r }))
  try {
    const request = view.controls.refresh(true)
    view.session.value = 'b'
    await nextTick()
    resolve(response(ready))
    await request
    assert.equal(view.controls.currentUrl.value, '')
    assert.equal(view.controls.result.value, null)
  } finally { view.close() }
})

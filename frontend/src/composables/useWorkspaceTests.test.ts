import test from 'node:test'
import assert from 'node:assert/strict'
import { createRenderer, nextTick, ref } from 'vue'
import { useWorkspaceTests, testRunDisplayState } from './useWorkspaceTests.ts'
import type { TestRun, WorkspaceTestsGateway } from '../api/workspaceTests.ts'

function result(id = 'run-a', session = 'a'): TestRun {
  return { id, session_id: session, project_id: '', status: 'completed', request: { project_id: '', source: 'manual' }, result: { status: 'completed', outcome: 'no_tests', checks: [], version_changed: false, version: { fingerprint: 'v1' } }, report_message_id: 'report-a', created_at: '2026-10-13T01:00:00Z', updated_at: '2026-10-13T01:00:00Z' }
}
function mount(gateway: Partial<WorkspaceTestsGateway> = {}, active = true) {
  const session = ref('a'), enabled = ref(active), revision = ref(0)
  const reports: string[] = []
  const api: WorkspaceTestsGateway = { projects: async () => ({ status: 'ready', projects: [], warnings: [], version: { fingerprint: 'v1' } }), runs: async () => [], start: async () => ({ ...result(), status: 'queued', report_message_id: undefined }), cancel: async () => ({ ...result(), status: 'cancelling', report_message_id: undefined }), browser: async () => result(), ...gateway }
  let controls!: ReturnType<typeof useWorkspaceTests>
  const renderer = createRenderer<any, any>({ patchProp() {}, insert() {}, remove() {}, setText() {}, setElementText() {}, createElement: () => ({}), createText: () => ({}), createComment: () => ({}), parentNode: () => null, nextSibling: () => null })
  const app = renderer.createApp({ setup() { controls = useWorkspaceTests(session, enabled, revision, api, ref({}), id => reports.push(id)); return () => null } })
  app.mount({})
  return { session, enabled, revision, controls, reports, close: () => app.unmount() }
}

test('opening the test panel never starts or resumes a sandbox', async () => {
  let starts = 0
  const view = mount({ projects: async () => ({ status: 'paused', projects: [], warnings: [] }), start: async () => { starts++; return result() } })
  try { await nextTick(); assert.equal(starts, 0); assert.equal(view.controls.discovery.value.status, 'paused') }
  finally { view.close() }
})

test('duplicate start is prevented and a late result cannot leak into another session', async () => {
  let resolve!: (value: TestRun) => void, starts = 0
  const view = mount({ start: () => { starts++; return new Promise(r => { resolve = r }) } })
  try {
    const first = view.controls.start(); await view.controls.start()
    assert.equal(starts, 1)
    view.session.value = 'b'; await nextTick()
    resolve(result('old', 'a')); await first
    assert.equal(view.controls.runs.value.length, 0)
    assert.equal(view.controls.starting.value, false)
    assert.equal(view.reports.length, 0)
  } finally { view.close() }
})

test('persisted results and report notifications restore on reopening without rerunning', async () => {
  let starts = 0
  const view = mount({ runs: async () => [result()], start: async () => { starts++; return result() } })
  try {
    await nextTick(); await view.controls.refreshRuns()
    assert.equal(view.controls.current.value?.result.outcome, 'no_tests')
    assert.deepEqual(view.reports, ['report-a'])
    await view.controls.refreshRuns(); assert.deepEqual(view.reports, ['report-a'])
    assert.equal(starts, 0)
  } finally { view.close() }
})

test('a changed source version makes a completed report stale', async () => {
  const view = mount({ runs: async () => [result()], projects: async () => ({ status: 'ready', projects: [], warnings: [], version: { fingerprint: 'v2' } }) })
  try { await nextTick(); await view.controls.refreshRuns(); await view.controls.discover(); assert.equal(view.controls.stale.value, true) }
  finally { view.close() }
})

test('cancelled and timed-out runs never display an unknown outcome as a successful result', () => {
  assert.equal(testRunDisplayState({ ...result(), status: 'cancelled' }), 'cancelled')
  assert.equal(testRunDisplayState({ ...result(), status: 'timed_out' }), 'timed_out')
  assert.equal(testRunDisplayState(result()), 'no_tests')
})

test('automatic verification requires opt-in and a changed source, without discovery loops', async () => {
  let fingerprint = 'v1'
  const sources: string[] = []
  const view = mount({ projects: async () => ({ status: 'ready', projects: [], warnings: [], version: { fingerprint } }), start: async (_session, request) => { sources.push(request.source || 'manual'); return result() } })
  try {
    await nextTick(); await view.controls.discover()
    fingerprint = 'v2'; await view.controls.discover(true)
    assert.deepEqual(sources, [])
    view.controls.autoRun.value = true; await nextTick()
    fingerprint = 'v3'; await view.controls.discover()
    assert.deepEqual(sources, [], 'manual discovery must not start a run')
    fingerprint = 'v4'; await view.controls.discover(true); await nextTick()
    assert.deepEqual(sources, ['automatic'])
    await view.controls.discover(true)
    assert.deepEqual(sources, ['automatic'], 'an unchanged fingerprint must not repeat a run')
  } finally { view.close() }
})

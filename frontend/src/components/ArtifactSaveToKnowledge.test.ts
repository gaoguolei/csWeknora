import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { nextTick, ref, watch } from 'vue'
import { permissionCanEditKB } from '../utils/kbPermission.ts'

const source = readFileSync(new URL('./ArtifactSaveToKnowledge.vue', import.meta.url), 'utf8')
const script = ts.transpile(source.slice(source.indexOf('const visible = ref'), source.indexOf('</script>')), { target: ts.ScriptTarget.ES2022 })

async function settlePicker() {
  await nextTick()
  await new Promise(resolve => setImmediate(resolve))
  await nextTick()
}

function setup(uploadError?: unknown, linkError?: unknown) {
  const uploads: any[] = []
  const downloads: any[] = []
  const routes: string[] = []
  const messages: string[] = []
  const links: any[] = []
  const bindings: any[] = []
  const props = { sessionId: 'session', messageId: 'message', artifactIndex: 2, fileName: '登录用例.xlsx' }
  const resources = {
    ensureKnowledgeBases: async () => {},
    rawKnowledgeBases: [
      { id: 'own', name: 'Owned', creator_id: 'user' },
      { id: 'other', name: 'Other owner', creator_id: 'someone-else' },
      { id: 'faq', name: 'FAQ', type: 'faq', creator_id: 'user' },
      { id: 'readonly', name: 'Read only', creator_id: 'user', my_permission: 'viewer' },
    ],
  }
  const state = vm.runInNewContext(`${script}; ({ open, save, visible, options, kbId, saved, existingKnowledgeId })`, {
    props, ref, watch, permissionCanEditKB, resources, File,
    organization: { canEditKB: () => false, sharedKnowledgeBases: [
      { permission: 'editor', knowledge_base: { id: 'shared', name: 'Shared' } },
      { permission: 'viewer', knowledge_base: { id: 'viewer', name: 'Viewer share' } },
    ] },
    auth: { user: { id: 'user' }, hasRole: () => false },
    t: (key: string) => key,
    router: { push: async (route: string) => routes.push(route) },
    MessagePlugin: { success: (message: string) => messages.push(message), error: (message: string) => messages.push(message) },
    downloadArtifact: async (...args: any[]) => { downloads.push(args); return new Blob(['original Excel bytes']) },
    uploadKnowledgeFile: async (...args: any[]) => { uploads.push(args); if (uploadError) throw uploadError; return { data: { id: 'knowledge' } } },
    listKnowledgeFiles: async () => ({ data: [] }),
    linkKnowledgeArtifact: async (...args: any[]) => { links.push(args); if (linkError) throw linkError },
    emit: (_: string, binding: any) => bindings.push(binding),
  })
  return { state, uploads, downloads, routes, messages, links, bindings }
}

test('knowledge picker offers writable document bases and editor shares', async () => {
  const { state } = setup()
  await state.open()
  assert.deepEqual(Array.from(state.options.value, (option: any) => option.value), ['own', 'shared'])
})

test('saving uploads the original artifact with its filename into the selected base', async () => {
  const { state, uploads, downloads, routes, links } = setup()
  await state.open()
  state.kbId.value = 'shared'
  await settlePicker()
  await state.save()
  assert.deepEqual(downloads, [['session', 'message', 2]])
  assert.equal(uploads[0][0], 'shared')
  assert.equal(uploads[0][1].file.name, '登录用例.xlsx')
  assert.equal(await uploads[0][1].file.text(), 'original Excel bytes')
  assert.equal(state.saved.value, true)
  assert.equal(state.visible.value, false)
  await state.open()
  assert.deepEqual(routes, ['/platform/knowledge-bases/shared'])
  assert.equal(uploads.length, 1)
  assert.equal(links[0][0], 'knowledge')
  assert.equal(links[0][1].session_id, 'session')
  assert.equal(links[0][1].index, 2)
})

test('existing edited knowledge is linked without uploading the original snapshot over it', async () => {
  const { state, downloads, uploads, links } = setup()
  await state.open()
  await settlePicker()
  state.existingKnowledgeId.value = 'edited-knowledge'
  await state.save()
  assert.equal(downloads.length, 0)
  assert.equal(uploads.length, 0)
  assert.equal(links[0][0], 'edited-knowledge')
  assert.equal(state.saved.value, true)
})

test('a binding failure is not reported as a successful synchronized save', async () => {
  const { state, bindings } = setup(undefined, new Error('binding failed'))
  await state.open()
  await settlePicker()
  await state.save()
  assert.equal(state.saved.value, false)
  assert.equal(state.visible.value, true)
  assert.equal(bindings.length, 0)
})

test('upload failure leaves the selection open for retry', async () => {
  const { state, messages } = setup({ message: 'storage unavailable' })
  await state.open()
  await settlePicker()
  await state.save()
  assert.equal(state.saved.value, false)
  assert.equal(state.visible.value, true)
  assert.equal(messages.at(-1), 'storage unavailable')
})

test('an existing identical file counts as saved only when the server identifies it', async () => {
  for (const [error, expected] of [
    [{ code: 'duplicate_file', data: { id: 'existing' } }, true],
    [{ code: 'duplicate_file' }, false],
  ] as const) {
    const { state } = setup(error)
    await state.open()
    await settlePicker()
    await state.save()
    assert.equal(state.saved.value, expected)
  }
})

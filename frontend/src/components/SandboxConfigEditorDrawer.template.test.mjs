import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import ts from 'typescript'

const source = readFileSync(
  new URL('./SandboxConfigEditorDrawer.vue', import.meta.url), 'utf8')

// Execute the component's actual loader with a rejected HTTP request; this
// catches silent failures without needing a browser or a live Cube cluster.
function failingTemplateLoader(error) {
  const tag = '<script setup lang="ts">'
  const script = source.slice(source.indexOf(tag) + tag.length, source.indexOf('</script>'))
  const ast = ts.createSourceFile('drawer.ts', script, ts.ScriptTarget.Latest, true)
  const loader = ast.statements.find((node) => ts.isFunctionDeclaration(node) && node.name?.text === 'loadTemplates')
  assert.ok(loader)
  const js = ts.transpileModule(loader.getText(ast), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
  }).outputText
  const messages = []
  const state = {
    hasImageCatalog: { value: true },
    connectionReady: () => true,
    templatesLoading: { value: false },
    templatesError: { value: '' },
    querySandboxTemplates: async () => { throw error },
    collectPayload: () => ({}),
    effectiveRecord: { value: null },
    t: () => '模板列表加载失败',
    MessagePlugin: { error: (message) => messages.push(message) },
  }
  const load = new Function(...Object.keys(state), `${js}; return loadTemplates`)(...Object.values(state))
  return { load, state, messages }
}

test('catalog authentication failure is visible and releases the loading state', async () => {
  const message = '沙箱后端认证失败：请填写有效的 API Key'
  const { load, state, messages } = failingTemplateLoader({ status: 400, message })
  assert.equal(await load(), false)
  assert.equal(state.templatesError.value, message)
  assert.equal(state.templatesLoading.value, false)
  assert.deepEqual(messages, [message])
  const template = source.slice(source.indexOf("currentStepKey === 'template'"), source.indexOf("currentStepKey === 'runtime'"))
  assert.ok(template.indexOf('v-if="templatesError"') < template.indexOf('class="template-list"'),
    'the persistent error must appear above the potentially long template list')
})

test('background catalog polling keeps the error visible without repeating notifications', async () => {
  const { load, state, messages } = failingTemplateLoader({ message: '认证失败' })
  assert.equal(await load({ silent: true }), false)
  assert.equal(state.templatesError.value, '认证失败')
  assert.deepEqual(messages, [])
})

test('catalog failures without a message still notify with the localized fallback', async () => {
  const { load, state, messages } = failingTemplateLoader({})
  assert.equal(await load(), false)
  assert.equal(state.templatesError.value, '模板列表加载失败')
  assert.deepEqual(messages, ['模板列表加载失败'])
})

test('sandbox settings no longer offer or submit remote desktop capabilities', () => {
  assert.doesNotMatch(source, /desktop_enabled|desktop_template_image|ensure_desktop|replace_desktop|createDesktopTemplate|customDesktopEnabled/)
  assert.match(source, /ensure_standard:\s*Boolean\(opts\.ensureStandard\)/)
  assert.match(source, /createStandardTemplate/)
})

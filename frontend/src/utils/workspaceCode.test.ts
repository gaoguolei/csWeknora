import assert from 'node:assert/strict'
import test from 'node:test'
import { formatWorkspaceCode, highlightWorkspaceLines, workspaceCodeLanguage } from './workspaceCode'

for (const [path, language, source] of [
  ['app.tsx', 'typescript', 'const App = () => <div>Hello</div>'],
  ['app.jsx', 'javascript', 'const App = () => <div>Hello</div>'],
  ['app.mts', 'typescript', 'const count: number = 1'],
  ['App.vue', 'xml', '<template><div>Hello</div></template>'],
  ['Dockerfile', 'dockerfile', 'FROM node:22\nRUN npm install'],
  ['main.rs', 'rust', 'fn main() { let x = 1; }'],
  ['config.toml', 'ini', '[app]\nport = 8080'],
  ['main.PY', 'python', 'def hello():\n    return True'],
]) {
  test(`recognizes and highlights ${path}`, () => {
    assert.equal(workspaceCodeLanguage(path), language)
    assert.match(highlightWorkspaceLines(source, path).join('\n'), /class="hljs-/)
  })
}

test('multiline tokens remain highlighted in individually balanced rows', () => {
  const lines = highlightWorkspaceLines('/* first\nsecond\nthird */\nconst x = 1', 'main.js')
  assert.equal(lines.length, 4)
  for (const line of lines) {
    assert.equal((line.match(/<span\b/g) || []).length, (line.match(/<\/span>/g) || []).length)
  }
  assert.match(lines[1], /^<span class="hljs-comment">second<\/span>$/)
})

test('unknown source is escaped and files over the old 60 KB cutoff still highlight', () => {
  assert.equal(highlightWorkspaceLines('<script>alert("x")</script>&', 'unknown.xyz')[0], '&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt;&amp;')
  const lines = highlightWorkspaceLines('const value = 123;\n'.repeat(4000), 'large.js')
  assert.match(lines[3999], /hljs-keyword/)
})

for (const [path, source] of [
  ['data.json', '{"a":1,"b":{"c":2}}'],
  ['app.tsx', 'export const App=()=>{return <div><p>Hello</p><p>World</p></div>}'],
  ['index.html', '<html><head><style>body{color:red;margin:0}</style></head><body><h1>Hello</h1><script>const x={a:1};</script></body></html>'],
  ['App.vue', '<template><div>Hello</div></template><script setup lang="ts">const x:number=1</script><style>div{color:red}</style>'],
  ['style.scss', '.app{color:red;.item{margin:0}}'],
]) {
  test(`formats ${path} and is stable on a second pass`, async () => {
    const formatted = await formatWorkspaceCode(source, path)
    assert.notEqual(formatted, source)
    assert.ok(formatted.split('\n').length > 2)
    assert.equal(await formatWorkspaceCode(formatted, path), formatted)
  })
}

test('unsupported, incomplete and oversized source falls back without data loss', async () => {
  for (const [path, source] of [['main.py', 'def f():\n return 1'], ['app.ts', 'const broken = {'], ['large.js', ' '.repeat(200_001) + 'let x=1;']]) {
    assert.equal(await formatWorkspaceCode(source, path), source)
  }
})

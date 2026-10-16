// Opt-in browser regression test with local fixtures; no account or real sandbox.
// Set BROWSERSKILL_TEST_PLAYWRIGHT and BROWSERSKILL_TEST_CHROMIUM, then run with node.
import assert from 'node:assert/strict'
import { createServer as httpServer } from 'node:http'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createServer } from 'vite'
import vue from '@vitejs/plugin-vue'

const { chromium } = await import(pathToFileURL(process.env.BROWSERSKILL_TEST_PLAYWRIGHT).href)
const root = fileURLToPath(new URL('../', import.meta.url))
const cacheDir = await mkdtemp(join(tmpdir(), 'weknora-workspace-preview-'))
const entry = `
import { createApp, h, ref } from 'vue';
import { createPinia } from 'pinia';
import TDesign from 'tdesign-vue-next';
import 'tdesign-vue-next/es/style/index.css';
import '/src/assets/theme/theme.css';
import '/src/assets/theme/workspace-theme.css';
import { createI18n } from 'vue-i18n';
import { workspaceEnUS, workspaceZhCN } from '/src/i18n/workspace.ts';
import Code from '/src/components/workspace/ConversationWorkspace.vue';
import Browser from '/src/components/workspace/WorkspaceBrowserPreview.vue';
const files = {
 'index.html': '<!doctype html><html><head><link rel="stylesheet" href="style.css"></head><body><h1>File preview</h1><button id="count">Count 0</button><script src="main.js"></scr'+'ipt></body></html>',
 'style.css': 'h1{color:rgb(10,100,50)}body{font:16px system-ui;padding:24px}',
 'main.js': 'let n=0;document.querySelector("#count").onclick=()=>document.querySelector("#count").textContent="Count "+(++n);',
 'drawing.svg': '<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect width="100" height="100" fill="green"/></svg>',
 'notes.md': '# Markdown preview',
};
const gateway = {
 tree: async () => ({root:'/workspace',nodes:Object.keys(files).map(path=>({path,name:path,kind:'file'}))}),
 file: async (_,path) => ({path,content:files[path],size:files[path].length,hash:path}),
 run: async()=>({stdout:'',stderr:'',exit_code:0,duration_ms:0,killed:false}),
};
const session = ref('a'), view = ref('code');
const app = createApp({setup(){return()=>h('main',{style:'width:min(900px,100%);height:700px;display:flex;flex-direction:column;border:1px solid #ddd;margin:20px auto'},[
 h('nav',[h('button',{onClick:()=>view.value='code'},'Code fixture'),h('button',{onClick:()=>view.value='url'},'URL fixture'),h('button',{onClick:()=>session.value='b'},'Switch session')]),
 view.value==='code'?h(Code,{sessionId:session.value,tab:'source',active:true,gateway}):h(Browser,{sessionId:session.value,enabled:false,loading:false,active:true}),
])}});
app.use(createPinia()).use(createI18n({legacy:false,locale:'en-US',messages:{'en-US':{workspace:workspaceEnUS,preview:{fullscreen:'Fullscreen',exitFullscreen:'Exit fullscreen'}},'zh-CN':{workspace:workspaceZhCN}}})).use(TDesign).mount('#app');
`
const server = await createServer({
  root, cacheDir, configFile: false, plugins: [vue(), {
    name: 'workspace-preview-fixture',
    resolveId(id) { if (id === '/preview-fixture.js') return '\0preview-fixture' },
    load(id) { if (id === '\0preview-fixture') return entry },
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (req.url === '/preview-fixture.html') {
          res.setHeader('Content-Type', 'text/html')
          res.end(await server.transformIndexHtml(req.url, '<!doctype html><html><body style="margin:0"><div id="app"></div><script type="module" src="/preview-fixture.js"></script></body></html>'))
        } else next()
      })
    },
  }], optimizeDeps: { entries: [] }, resolve: { alias: { '@': root + 'src', '@vue-office/pptx': root + 'node_modules/@vue-office/pptx/lib/v3/index.js' } }, server: { host: '127.0.0.1', port: 0 },
})
const project = httpServer((req, res) => {
  res.setHeader('Content-Type', 'text/html')
  res.end('<!doctype html><html><body><h1>Running project</h1><button onclick="this.textContent=\'Clicked\'">Interact</button><script type="module">document.body.dataset.module="ready";fetch("/api").then(()=>document.body.dataset.api="ready")</script></body></html>')
})
let browser
try {
  await server.listen()
  await new Promise(resolve => project.listen(0, '127.0.0.1', resolve))
  browser = await chromium.launch({ executablePath: process.env.BROWSERSKILL_TEST_CHROMIUM, headless: true })
  const page = await browser.newPage({ viewport: { width: 1100, height: 850 } })
  const errors = [], apiCalls = []
  page.on('pageerror', error => errors.push(error.message))
  page.on('request', req => { if (req.url().includes('/api/v1/')) apiCalls.push(req.url()) })
  await page.goto(server.resolvedUrls.local[0] + 'preview-fixture.html')
  await page.locator('.source-code').waitFor()
  await page.getByRole('button', { name: 'Preview', exact: true }).click()
  const frame = page.frameLocator('.preview-stage iframe')
  await frame.getByRole('heading', { name: 'File preview' }).waitFor()
  assert.equal(await frame.locator('h1').evaluate(el => getComputedStyle(el).color), 'rgb(10, 100, 50)')
  await frame.getByRole('button', { name: 'Count 0' }).click()
  await frame.getByRole('button', { name: 'Count 1' }).waitFor()
  await page.locator('.file-tree__row', { hasText: 'notes.md' }).click()
  await frame.getByRole('heading', { name: 'Markdown preview' }).waitFor()
  await page.locator('.file-tree__row', { hasText: 'drawing.svg' }).click()
  await frame.locator('svg rect').waitFor()
  await page.locator('.file-tree__row', { hasText: 'style.css' }).click()
  await page.locator('.source-code').waitFor()
  assert.equal(await page.locator('.preview-stage iframe').count(), 0)
  await page.locator('.file-tree__row', { hasText: 'index.html' }).click()
  await frame.getByRole('heading', { name: 'File preview' }).waitFor()
  await page.locator('main').evaluate(el => { el.style.width = '560px' })
  assert.ok(await page.locator('main').evaluate(el => el.scrollWidth <= el.clientWidth), 'file toolbar fits the side panel')
  await page.screenshot({ path: join(tmpdir(), 'weknora-file-preview.png') })
  await page.getByRole('button', { name: 'Code', exact: true }).click()
  await page.locator('.source-code').waitFor()
  await page.screenshot({ path: join(tmpdir(), 'weknora-file-source.png') })
  await page.getByRole('button', { name: 'URL fixture' }).click()
  await page.getByRole('heading', { name: 'Start your project and paste its URL' }).waitFor()
  await page.getByRole('button', { name: 'Remote desktop · Unavailable', exact: true }).click()
  await page.getByRole('heading', { name: 'Remote desktop unavailable' }).waitFor()
  assert.equal(await page.locator('.workspace-desktop input').count(), 0, 'the desktop has no project address bar')
  assert.equal(await page.locator('.workspace-desktop .url-devices').count(), 0)
  assert.equal(await page.locator('.sandbox-desktop').count(), 0, 'unsupported sandboxes do not connect')
  await page.getByRole('button', { name: 'Web preview', exact: true }).click()
  assert.equal(await page.locator('.sandbox-desktop').count(), 0)
  const input = page.getByRole('textbox')
  const projectUrl = 'http://127.0.0.1:' + project.address().port
  await input.fill(projectUrl)
  await input.press('Enter')
  const web = page.frameLocator('.url-stage iframe')
  await web.getByRole('heading', { name: 'Running project' }).waitFor()
  await web.locator('body[data-module="ready"][data-api="ready"]').waitFor()
  await web.getByRole('button', { name: 'Interact' }).click()
  await web.getByRole('button', { name: 'Clicked' }).waitFor()
  await page.getByRole('button', { name: 'Remote desktop · Unavailable', exact: true }).click()
  await page.getByRole('heading', { name: 'Remote desktop unavailable' }).waitFor()
  await page.getByRole('button', { name: 'Web preview', exact: true }).click()
  await web.getByRole('button', { name: 'Clicked' }).waitFor()
  await page.getByRole('button', { name: 'Refresh page' }).click()
  await web.getByRole('button', { name: 'Interact' }).waitFor()
  await page.getByRole('button', { name: 'Mobile view' }).click()
  assert.ok((await page.locator('.url-stage iframe').boundingBox()).width <= 392)
  await input.fill('javascript:alert(1)'); await input.press('Enter')
  await page.getByRole('alert').waitFor()
  assert.equal(await page.locator('.url-stage iframe').getAttribute('src'), projectUrl + '/')
  await input.fill('/nested'); await input.press('Enter')
  await web.getByRole('heading', { name: 'Running project' }).waitFor()
  assert.equal(await page.locator('.url-stage iframe').getAttribute('src'), projectUrl + '/nested')
  assert.ok(await page.locator('main').evaluate(el => el.scrollWidth <= el.clientWidth), 'URL toolbar fits the side panel')
  await page.screenshot({ path: join(tmpdir(), 'weknora-url-preview.png') })
  await page.getByRole('button', { name: 'Switch session' }).click()
  assert.equal(await page.locator('.url-stage iframe').count(), 0)
  assert.equal(await input.inputValue(), '')
  assert.deepEqual(apiCalls, [], 'basic previews never call a sandbox or desktop API')
  assert.deepEqual(errors, [])
  console.log('PASS: HTML/CSS/JS, Markdown, SVG, source switching, URL iframe modules/fetch/interaction, refresh, mobile, validation, relative routes and session reset.')
} finally {
  await browser?.close()
  await server.close()
  await new Promise(resolve => project.close(resolve))
  await rm(cacheDir, { recursive: true, force: true })
}

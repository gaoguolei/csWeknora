import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { computed, ref } from 'vue'
import { TOOLBOX_ITEMS, canAccessToolboxSection, isToolboxSection, toolboxLocation } from './toolbox'
import zhCN from '../i18n/locales/zh-CN'

const roles = ['viewer', 'contributor', 'admin', 'owner']
const access = (role: string | null, unsupported: string[] = [], superuser = false, ops = false, sysadmin = false) => ({
  currentTenantRole: role,
  canAccessAllTenants: superuser,
  isOperationsAdmin: ops,
  isSystemAdmin: sysadmin || ops,
  hasRole: (min: string) => roles.indexOf(role || '') >= roles.indexOf(min),
  isSupported: (capability?: string) => !capability || !unsupported.includes(capability),
})

test('toolbox contains only the agreed tools; infrastructure and secrets stay in settings', () => {
  assert.deepEqual(TOOLBOX_ITEMS.map((item) => item.key), ['skills', 'mcp', 'browserconnection'])
  const lookup = (key: string) => key.split('.').reduce<any>((node, part) => node?.[part], zhCN)
  for (const item of TOOLBOX_ITEMS) {
    for (const key of [item.title, item.description, 'help' in item ? item.help : undefined,
      'action' in item ? item.action : undefined]) {
      if (key) assert.equal(typeof lookup(key), 'string', key)
    }
  }
  for (const key of ['sandbox', 'websearch', 'envvars', 'integration-api', undefined, 'unknown']) {
    assert.equal(isToolboxSection(key), false)
  }
})

test('toolbox gates skills/mcp to operations admins; browserconnection stays on the role ladder', () => {
  // skills / mcp are operations-admin-only (like ollama): every tenant role,
  // plain system admins, and cross-tenant superusers are all locked out.
  for (const role of ['viewer', 'contributor', 'admin', 'owner']) {
    assert.equal(canAccessToolboxSection('skills', access(role)), false)
    assert.equal(canAccessToolboxSection('mcp', access(role)), false)
  }
  // Neither the cross-tenant superuser bypass nor the system-admin flag applies.
  assert.equal(canAccessToolboxSection('mcp', access(null, [], true)), false)
  assert.equal(canAccessToolboxSection('skills', access('owner', [], false, false, true)), false)
  // A tenantless operations admin still gets them (isOperationsAdmin wins).
  assert.equal(canAccessToolboxSection('skills', access(null, [], false, true)), true)
  assert.equal(canAccessToolboxSection('mcp', access(null, [], false, true)), true)
  // Deployment capability still gates ahead of the role check.
  assert.equal(canAccessToolboxSection('skills', access('owner', ['settings.sandbox'], false, true)), false)
  assert.equal(canAccessToolboxSection('mcp', access('owner', ['settings.mcp'], false, true)), false)
  // browserconnection is NOT operations-only: it follows the tenant role ladder.
  for (const role of ['viewer', 'contributor', 'admin', 'owner']) {
    assert.equal(canAccessToolboxSection('browserconnection', access(role)), true)
  }
  assert.equal(canAccessToolboxSection('browserconnection', access(null)), false)
  // Reactivity: an owner who is not an operations admin never unlocks skills.
  const role = ref('owner')
  const allowed = computed(() => canAccessToolboxSection('skills', access(role.value)))
  assert.equal(allowed.value, false)
})

test('skill shortcut retains its target sandbox without leaking it to other tools', () => {
  assert.deepEqual(toolboxLocation('skills', 'sandbox-123'), {
    path: '/platform/toolbox/skills', query: { sandboxId: 'sandbox-123' },
  })
  assert.deepEqual(toolboxLocation('browserconnection', 'sandbox-123'), {
    path: '/platform/toolbox/browserconnection', query: {},
  })
  assert.deepEqual(toolboxLocation(), { path: '/platform/toolbox', query: {} })
})

const settings = readFileSync(new URL('../views/settings/Settings.vue', import.meta.url), 'utf8')
const script = settings.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)![1]!

test('settings keeps skills/mcp inline and only forwards to the toolbox', () => {
  // 技能 / MCP 在设置弹窗里内嵌渲染（保留侧边导航入口），同时仍是工具箱的
  // 工具；browserconnection 只属于工具箱。
  assert.match(settings, /<SkillSettings :initial-sandbox-id="currentSubSection" \/>/)
  assert.match(settings, /<McpSettings \/>/)
  assert.equal(settings.includes('<BrowserConnectionSettings'), false)
  assert.match(settings, /\{ key: "skills", icon: SKILL_ICON/)
  assert.match(settings, /\{ key: "mcp", icon: "tools"/)
  // 旧快捷入口（智能体编辑器 / 沙箱卡片等）仍然走 openSettings + subSection
  // 预选目标沙箱，不再改写路由到工具箱。
  assert.equal(script.includes('redirectToToolbox'), false)
  assert.match(script, /normalizedSection === "skills"[\s\S]{0,80}uiStore\.settingsInitialSubSection/)
})

test('inline skill/mcp panels get a Teleport target so unmount cannot crash', () => {
  // 面板里的搜索框 <Teleport to="#toolbox-toolbar" defer>：设置弹窗必须提供
  // 同名容器，否则脱离 Toolbox.vue 渲染时卸载会对 null 取 parentNode 报错。
  assert.match(settings, /id="toolbox-toolbar"/)
  const router = readFileSync(new URL('../router/index.ts', import.meta.url), 'utf8')
  assert.equal(router.includes("isToolboxSection(to.query.section)"), false)
  const page = readFileSync(new URL('../views/toolbox/Toolbox.vue', import.meta.url), 'utf8')
  assert.match(page, /<div id="toolbox-toolbar" class="toolbox-toolbar" \/>/)
  assert.match(page, /:initial-sandbox-id="sandboxId"/)
})

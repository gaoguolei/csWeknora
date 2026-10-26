import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { computed } from 'vue'
import { INTEGRATION_PREVIEW_ITEMS } from '../../config/integrations'
import { integrationSectionKey } from '../../config/settingsRoute'

// Exercise the actual sidebar grouping without mounting the settings panels.
const source = readFileSync(new URL('./Settings.vue', import.meta.url), 'utf8')
const script = source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)![1]!
const ast = ts.createSourceFile('Settings.ts', script, ts.ScriptTarget.Latest, true)
// The sidebar is one flat list split into「个人设置 / 平台设置 / 系统管理」sections:
// `navGroups` builds the grouped layout and stamps each section title onto its first
// visible group (the title doubles as the collapse toggle). Transpile it so the
// grouping stays assertable without mounting the panels.
const grouping = ast.statements
  .filter((statement) =>
    ts.isVariableStatement(statement) &&
    statement.declarationList.declarations.some(
      (declaration) =>
        ts.isIdentifier(declaration.name) &&
        declaration.name.text === 'navGroups',
    ),
  )
  .map((statement) => statement.getText(ast))
  .join('\n')
assert.ok(grouping, 'Settings must define sidebar groups')
const compiled = ts.transpileModule(grouping, {
  compilerOptions: { target: ts.ScriptTarget.ES2022 },
}).outputText

type SidebarGroup = {
  key: string
  scope: string
  items: Array<{ key: string }>
}

function sidebarGroups(visibleKeys: string[]): SidebarGroup[] {
  return runInNewContext(`${compiled}\nnavGroups.value`, {
    computed,
    navItems: { value: visibleKeys.map((key) => ({ key })) },
    t: (key: string) => key,
    integrationSectionKey,
    INTEGRATION_PREVIEW_ITEMS,
    // KB / agent default sub-sections have their own access tests; they are
    // irrelevant here and only need to exist for the layout to evaluate.
    KB_DEFAULTS_SUBSECTIONS: [],
    AGENT_DEFAULTS_SUBSECTIONS: [],
  }) as SidebarGroup[]
}

function integrationMenu(visibleKeys: string[]): string[] {
  const integrationGroup = sidebarGroups(visibleKeys).find((group) => group.key === 'integrations')
  return Array.from(integrationGroup?.items ?? [], (item) => item.key)
}

const integrationKeys = INTEGRATION_PREVIEW_ITEMS.map((item) => integrationSectionKey(item.key))

test('settings sidebar includes every registered integration, including CLI, in navigation order', () => {
  assert.deepEqual(integrationMenu(['general', ...integrationKeys]), integrationKeys)
})

test('settings sidebar preserves visibility filtering without hiding CLI', () => {
  const visible = integrationKeys.filter((key) => key !== 'integration-api' && key !== 'integration-im')
  assert.deepEqual(integrationMenu(visible), visible)
  assert.deepEqual(integrationMenu(['general']), [])
})

test('integrations belong to the personal settings scope, ahead of the platform block', () => {
  // API / MCP server / Chrome extension are workspace- or member-level entries, so
  // the group is stamped with the personal scope and rendered before the platform
  // groups (role visibility still comes from canSeeSection, not from the scope).
  const groups = sidebarGroups(['general', 'models', ...integrationKeys])
  const integrationIndex = groups.findIndex((group) => group.key === 'integrations')
  assert.ok(integrationIndex >= 0, 'integrations group must be present')
  assert.equal(groups[integrationIndex]!.scope, 'personal')
  const platformIndex = groups.findIndex((group) => group.scope === 'platform')
  assert.ok(platformIndex > integrationIndex, 'platform groups must follow integrations')
})

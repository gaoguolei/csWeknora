import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./AgentEditorModal.vue', import.meta.url), 'utf8')

test('editing an agent closes the editor after a successful save', () => {
  assert.match(
    source,
    /await updateAgent\(formData\.value\.id, payload\);\s*MessagePlugin\.success\(t\('agent\.messages\.updated'\)\);\s*emit\('success'\);\s*handleClose\(\);/
  )
})

test('the first successful create stays open for integration setup', () => {
  const createBranch = source.match(
    /if \(editorMode\.value === 'create'\) \{([\s\S]*?)^\s{4}\} else \{/m
  )?.[1]

  assert.ok(createBranch, 'expected to find the create branch')
  assert.doesNotMatch(createBranch, /handleClose\(\)/)
  assert.match(createBranch, /savedAgent\.value = created;/)
})

test('save button labels distinguish create from save-and-close', () => {
  assert.match(
    source,
    /const saveButtonLabel = computed\(\(\) =>\s*editorMode\.value === 'create'\s*\? t\('agent\.editor\.buttons\.create'\)\s*: t\('agent\.editor\.buttons\.saveAndClose'\)\s*\)/
  )
  assert.match(source, /saveButtonLabel/)
})

test('shows a post-create hint after the first successful save', () => {
  assert.match(source, /const isPostCreateSession = computed\(\(\) => !!savedAgent\.value\)/)
  assert.match(source, /settings-footer-note/)
  assert.match(source, /agent\.editor\.postCreateHint\.title/)
})

test('platform-level sections are not exposed in the agent editor', () => {
  // 模型 / 多轮对话 / 检索策略 / 网络搜索 / 附件上传已收归「设置 → 智能体配置」，
  // 编辑器对任何角色都不再展示（项目经理第 11/12 条）。
  const navItems = source.match(/const navItems = computed\(\(\) => \{([\s\S]*?)^\}\);/m)?.[1]
  assert.ok(navItems, 'expected to find the nav items computed')
  for (const key of ['model', 'conversation', 'retrieval', 'websearch', 'multimodal']) {
    assert.doesNotMatch(navItems, new RegExp(`key: '${key}'`), `${key} must not be a nav entry`)
    assert.doesNotMatch(source, new RegExp(`currentSection === '${key}'`), `${key} section must be gone`)
  }
  // 问题推荐仍是用户级配置，必须在编辑器里。
  assert.match(navItems, /key: 'suggestions'/)
  assert.match(source, /v-show="currentSection === 'suggestions'"/)
})



test('skills and sandbox share one editor section', () => {
  const navItems = source.match(/const navItems = computed\(\(\) => \{([\s\S]*?)^\}\);/m)?.[1]
  assert.ok(navItems, 'expected to find the nav items computed')
  assert.match(navItems, /key: 'skills'/)
  assert.match(navItems, /icon: SKILL_ICON/)
  assert.doesNotMatch(navItems, /key: 'sandbox'/)

  const capabilityGroup = source.match(/pickItems\(\['tools', 'mcp', 'skills'\]\)/)
  assert.ok(capabilityGroup, 'expected the capability group to list skills without a separate sandbox tab')

  assert.match(source, /v-show="currentSection === 'skills' && isAgentMode"/)
  assert.doesNotMatch(source, /currentSection === 'sandbox' && isAgentMode/)
  assert.match(source, /sandbox: 'skills'/)
  assert.match(source, /formData\.config\.sandbox_config_id/)
  assert.match(source, /:disabled="!canEnableSkills"/)
  assert.match(source, /sandbox-option/)
  assert.doesNotMatch(source, /skill-info-box/)
})

test('agent skill picker uses the catalog and only enables ready installs', () => {
  assert.match(source, /function autoBindSoleSandbox\(/)
  assert.match(source, /canEnableSkills/)
  assert.match(source, /catalogSkillRows/)
  assert.match(source, /showCatalogSkillList/)
  assert.match(source, /skillsSelectionMode\.value !== 'none'/)
  assert.match(source, /:disabled="!skill\.selectable"/)
  assert.match(source, /catalogSkillGroups/)
  assert.match(source, /installPartial/)
  assert.match(source, /installCatalogToCurrent/)
  assert.match(source, /agent\.editor\.installToThisSandbox/)
  assert.match(source, /skill-pick-list/)
  assert.match(source, /skill-pick-group/)
  assert.match(source, /skill-pick__badge/)
  assert.match(source, /skillStatusIcon/)
  assert.match(source, /isSkillBusy/)
  assert.match(source, /viewInstallProgress/)
  assert.match(source, /openSkillInstallProgress/)
  assert.doesNotMatch(source, /await openSkillInstallProgress\(skill\)/)
  assert.match(source, /SandboxSkillsPanel/)
  assert.match(source, /focus-skill-id/)
  assert.match(source, /skillsGroupUnavailable/)
  assert.match(source, /selectedSandboxSummary/)
  assert.match(source, /line-clamp: 2/)
  assert.doesNotMatch(source, /skillsSelectionMode === 'selected' && catalogSkillRows/)
  assert.doesNotMatch(source, /skill-list-summary/)
  assert.doesNotMatch(source, /skill-ready-stat/)
})

test('agent skill picker offers upgrades to admins only and keeps upgrading skills selected', () => {
  assert.match(source, /from '@\/utils\/skillUpgrade'/)
  assert.match(source, /const upgradable = Boolean\(inst && installUpgradable\(item, inst\)\)/)
  assert.match(source, /canInstallSkills\.value && hasSandboxSelected\.value && skill\.upgradable/)
  assert.match(source, /v-if="canUpgradeSkillRow\(skill\)"/)
  assert.match(source, /v-else-if="canUpgradeSkillRow\(skill\)"/)
  assert.match(source, /agent\.editor\.upgradeOnThisSandbox/)
  // The progress drawer reads admin-only endpoints, so it is not offered to others.
  assert.match(source, /v-else-if="canInstallSkills && isSkillBusy\(skill\)"/)
  assert.match(source, /skill\.selectable \|\| \(skill\.installed && isSkillBusy\(skill\)\)/)
})

test('a skill whose previous version still runs stays selectable during and after an upgrade', () => {
  assert.match(source, /installEnabled && \(installStatus === 'ready' \|\| Boolean\(servedNote\)\)/)
  assert.match(source, /v-if="skill\.selectable && skill\.servedNote"/)
  // Installing the catalog version over a failed upgrade is the upgrade retried.
  assert.match(source, /installsAnUpgrade\(skill\) \? \$t\('settings\.skills\.upgrade'\) : \$t\('agent\.editor\.installShort'\)/)
  assert.match(source, /return skill\.upgradable \|\| Boolean\(skill\.servedNote\)/)
})

test('retrieval strategy now lives in the platform agent settings', () => {
  // search_knowledge reads the global retrieval configuration; the per-agent
  // thresholds were removed from the editor together with the other
  // platform-level sections.
  assert.doesNotMatch(source, /currentSection === 'retrieval'/)
  assert.doesNotMatch(source, /items\.push\(\{ key: 'retrieval'/)
})

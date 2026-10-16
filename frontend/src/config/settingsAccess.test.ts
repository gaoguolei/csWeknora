import assert from 'node:assert/strict'
import test from 'node:test'

import {
  SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE,
  SETTINGS_SECTION_MIN_ROLE,
  SYSTEM_ADMIN_SETTINGS_SECTIONS,
  GLOBAL_MODEL_ADMIN_SECTIONS,
  OPERATIONS_ADMIN_ONLY_SECTIONS,
} from './settingsAccess'

test('management shortcuts are stricter than read-only settings pages', () => {
  assert.equal(SETTINGS_SECTION_MIN_ROLE.members, 'viewer')
  assert.equal(SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE.members, 'owner')
})

test('ops-admin runtime tools left the tenant role ladder for isOperationsAdmin', () => {
  // skills / sandbox (and the other infra pages) are no longer tenant `admin`;
  // they are operations-admin-only, exactly like `ollama`, so they are absent
  // from MIN_ROLE and present in OPERATIONS_ADMIN_ONLY_SECTIONS.
  for (const key of ['mcp', 'vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'websearch']) {
    assert.equal(SETTINGS_SECTION_MIN_ROLE[key], undefined)
    assert.ok(OPERATIONS_ADMIN_ONLY_SECTIONS.has(key))
  }
  // Skills live in the sidebar toolbox, so the avatar menu has no shortcut.
  assert.equal(Object.prototype.hasOwnProperty.call(SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE, 'skills'), false)
})

test('personal skill environment variables are visible to every member', () => {
  assert.equal(SETTINGS_SECTION_MIN_ROLE.envvars, 'viewer')
  // Workspace-wide skill env values live on the Admin+ skills page; a
  // management shortcut on the avatar menu would only duplicate that entrance.
  assert.equal(
    Object.prototype.hasOwnProperty.call(SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE, 'envvars'),
    false,
  )
})

test('system administration settings stay explicitly system-admin-only', () => {
  assert.deepEqual(
    [...SYSTEM_ADMIN_SETTINGS_SECTIONS],
    ['system-global', 'model-catalog', 'runtime-queues', 'platform-api-keys', 'system-audit-log'],
  )
})

test('model config is reclaimed to the platform super admin; Ollama to the operations admin', () => {
  // Both are gated off the per-tenant role ladder, so they are absent from
  // MIN_ROLE. `models` stays isSystemAdmin (GLOBAL_MODEL_ADMIN_SECTIONS);
  // `ollama` moved to OPERATIONS_ADMIN_ONLY_SECTIONS (isOperationsAdmin only).
  assert.equal(SETTINGS_SECTION_MIN_ROLE.models, undefined)
  assert.equal(SETTINGS_SECTION_MIN_ROLE.ollama, undefined)
  assert.ok(GLOBAL_MODEL_ADMIN_SECTIONS.has('models'))
  assert.equal(GLOBAL_MODEL_ADMIN_SECTIONS.has('ollama'), false)
  assert.ok(OPERATIONS_ADMIN_ONLY_SECTIONS.has('ollama'))
})

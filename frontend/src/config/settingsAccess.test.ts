import assert from 'node:assert/strict'
import test from 'node:test'

import {
  SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE,
  SETTINGS_SECTION_MIN_ROLE,
  SYSTEM_ADMIN_SETTINGS_SECTIONS,
  GLOBAL_MODEL_ADMIN_SECTIONS,
  OPERATIONS_ADMIN_ONLY_SECTIONS,
  SUPER_ADMIN_ONLY_SECTIONS,
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
    ['system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log'],
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

test('model catalogue is operations-admin-only', () => {
  // The catalogue is shared by every workspace and its edits apply globally, so
  // the surface is reserved to operations admins: a plain system admin keeps the
  // `models` page but does not see (or deep-link into) the catalogue.
  assert.equal(SETTINGS_SECTION_MIN_ROLE['model-catalog'], undefined)
  assert.ok(OPERATIONS_ADMIN_ONLY_SECTIONS.has('model-catalog'))
  assert.equal(SYSTEM_ADMIN_SETTINGS_SECTIONS.has('model-catalog'), false)
  assert.equal(GLOBAL_MODEL_ADMIN_SECTIONS.has('model-catalog'), false)
  assert.equal(SUPER_ADMIN_ONLY_SECTIONS.has('model-catalog'), false)
})

test('platform knowledge base defaults are super-admin-only', () => {
  // `kb-defaults` is the platform-wide default KB configuration page backed by
  // system_settings "knowledge_base.default_config" (PUT is SystemAdmin-gated).
  // It is gated strictly on isSystemAdmin, so it is absent from the per-tenant
  // role ladder and lives only in SUPER_ADMIN_ONLY_SECTIONS — operations admins
  // still see it because they inherit isSystemAdmin.
  assert.equal(SETTINGS_SECTION_MIN_ROLE['kb-defaults'], undefined)
  assert.ok(SUPER_ADMIN_ONLY_SECTIONS.has('kb-defaults'))
  assert.equal(OPERATIONS_ADMIN_ONLY_SECTIONS.has('kb-defaults'), false)
  assert.equal(GLOBAL_MODEL_ADMIN_SECTIONS.has('kb-defaults'), false)
})

test('long-term memory settings are super-admin-only', () => {
  // `memory` is the platform-wide long-term memory configuration backed by
  // system_settings "memory.global_config" (PUT is SystemAdmin-gated). Members
  // and workspace admins/owners no longer see the entry; operations admins
  // still do because they inherit isSystemAdmin.
  assert.equal(SETTINGS_SECTION_MIN_ROLE.memory, undefined)
  assert.ok(SUPER_ADMIN_ONLY_SECTIONS.has('memory'))
  assert.equal(OPERATIONS_ADMIN_ONLY_SECTIONS.has('memory'), false)
  assert.equal(GLOBAL_MODEL_ADMIN_SECTIONS.has('memory'), false)
})

test('promoted kb-config sub-sections keep the same role split as the inner nav', () => {
  // 6 super-admin sections (models / chunking / image / audio / graph / advanced)
  // stay isSystemAdmin; the 3 ops-only ones (vectorStore / parser / storage)
  // are isOperationsAdmin-only, exactly as the old inner nav gated. FAQ is no
  // longer a platform default: it is chosen per KB at creation time, so it has
  // no settings section of its own.
  for (const key of ['kb-models', 'kb-chunking', 'kb-image', 'kb-audio', 'kb-graph', 'kb-advanced']) {
    assert.equal(SETTINGS_SECTION_MIN_ROLE[key], undefined)
    assert.ok(SUPER_ADMIN_ONLY_SECTIONS.has(key))
    assert.equal(OPERATIONS_ADMIN_ONLY_SECTIONS.has(key), false)
  }
  for (const key of ['kb-vectorStore', 'kb-parser', 'kb-storage']) {
    assert.equal(SETTINGS_SECTION_MIN_ROLE[key], undefined)
    assert.ok(OPERATIONS_ADMIN_ONLY_SECTIONS.has(key))
    assert.equal(SUPER_ADMIN_ONLY_SECTIONS.has(key), false)
  }
})

test('workspace data sources are configured by the workspace admin', () => {
  // Data sources are a tenant-wide concern (configured once, effective for every
  // knowledge base in the workspace), so `kb-datasource` sits on the tenant role
  // ladder at admin+ instead of being operations-admin-only. Operations admins
  // and cross-tenant superusers still pass through hasRole/ bypass.
  assert.equal(SETTINGS_SECTION_MIN_ROLE['kb-datasource'], 'admin')
  assert.equal(OPERATIONS_ADMIN_ONLY_SECTIONS.has('kb-datasource'), false)
  assert.equal(SUPER_ADMIN_ONLY_SECTIONS.has('kb-datasource'), false)
  assert.equal(GLOBAL_MODEL_ADMIN_SECTIONS.has('kb-datasource'), false)
  assert.equal(SYSTEM_ADMIN_SETTINGS_SECTIONS.has('kb-datasource'), false)
})

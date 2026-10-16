export type SettingsRoleKey = 'viewer' | 'contributor' | 'admin' | 'owner'

/**
 * Workspace-scoped settings access policy.
 *
 * Keep this as the single frontend source of truth for both the complete
 * Settings navigation and any shortcuts that lead into it. Backend route
 * guards remain authoritative.
 */
export const SETTINGS_SECTION_MIN_ROLE: Record<string, SettingsRoleKey> = {
  general: 'viewer',
  weknoracloud: 'admin',
  chathistory: 'admin',
  // NOTE: websearch / vectorstore / parser / storage / sandbox / skills / mcp
  // were reclaimed to operations-admin-only (see OPERATIONS_ADMIN_ONLY_SECTIONS)
  // — the exact same treatment as `ollama`. They are deliberately absent here so
  // the nav filter + the role-denied guard in Settings.vue hide them from every
  // non-operations-admin, including plain system admins and workspace admins.
  system: 'viewer',
  userprofile: 'viewer',
  browserconnection: 'viewer',
  tenant: 'viewer',
  members: 'viewer',
  mymemory: 'viewer',
  // Long-term memory is now a platform-wide setting owned by the super admin
  // (system setting memory.global_config). The page is visible to every member
  // so they can see the platform config, but every control is disabled unless
  // isSystemAdmin, and the backend PUT is gated to SystemAdmin. Entry stays
  // viewer; editing is gated on isSystemAdmin inside the component.
  memory: 'viewer',
  // Every member fills in their own environment variables; the workspace-wide
  // values stay on the Admin+ skills page.
  // NOTE: superseded — 沙箱密钥 is now a platform-wide super-admin store
  // (sandbox.global_env_vars). The page is gated to isSystemAdmin via
  // SUPER_ADMIN_ONLY_SECTIONS below, so this viewer entry never takes effect.
  envvars: 'viewer',
}

/**
 * A management-labelled avatar shortcut has a stricter threshold than the
 * corresponding read-only Settings page.
 */
export const SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE = {
  members: 'owner',
  models: 'admin',
} as const satisfies Record<string, SettingsRoleKey>

export const SYSTEM_ADMIN_SETTINGS_SECTIONS = new Set([
  'system-global',
  'model-catalog',
  'runtime-queues',
  'platform-api-keys',
  'system-audit-log',
])

/**
 * Sections that model management has been reclaimed to: online-model
 * configuration (`models`) is now a platform super-administrator concern.
 * Unlike the per-tenant role ladder in SETTINGS_SECTION_MIN_ROLE, these are
 * gated strictly on `isSystemAdmin` — cross-tenant superusers
 * (canAccessAllTenants) do NOT bypass them, matching the server-side
 * RequireSystemAdmin guards on the model mutation routes. Members and
 * workspace admins still consume the shared model catalogue from KB / agent /
 * memory pickers; they just cannot configure it.
 *
 * NOTE: `ollama` used to live here. It has moved to
 * OPERATIONS_ADMIN_ONLY_SECTIONS — the Ollama settings page is now a
 * deployment/operations concern shown only to operations admins. The Ollama
 * backend endpoints stay SystemAdmin-gated because the model editor (a system
 * admin surface) reuses them to list/download local models.
 */
export const GLOBAL_MODEL_ADMIN_SECTIONS = new Set(['models'])

/**
 * Sections owned exclusively by the operations administrator (the platform
 * role one level above system admin). Unlike the SystemAdmin-gated sets, these
 * are NOT visible to plain system admins, workspace admins/owners, or
 * cross-tenant superusers — only to `isOperationsAdmin`.
 *
 * These are all system-level / deployment-time concerns provisioned in advance
 * by ops staff (via .env or platform configuration), so the settings surface is
 * reserved to the operations administrator:
 *   - `ollama`      : local Ollama service address + model downloads
 *   - `mcp`         : MCP servers
 *   - `vectorstore` : vector database engines
 *   - `parser`      : document parsing engines
 *   - `storage`     : object/file storage engines
 *   - `sandbox`     : sandbox runtime configuration
 *   - `skills`      : skill management / install (writes into the sandbox image)
 *   - `websearch`   : network search providers
 *
 * Visibility is enforced purely on the frontend (nav filter + Settings.vue
 * role-denied guard + per-page isOperationsAdmin gate), mirroring how `ollama`
 * was handled: the underlying backend routes are left at their current guards
 * because other admin surfaces may still reuse them.
 */
export const OPERATIONS_ADMIN_ONLY_SECTIONS = new Set([
  'ollama',
  'mcp',
  'vectorstore',
  'parser',
  'storage',
  'sandbox',
  'skills',
  'websearch',
])

/**
 * Sections that are a pure platform super-administrator concern and must not be
 * visible to anyone else — not to workspace admins/owners, and not to
 * cross-tenant superusers (canAccessAllTenants). Gated strictly on
 * `isSystemAdmin`, mirroring the server-side RequireSystemAdmin guards.
 *
 * 沙箱密钥 moved here: a super admin configures one encrypted global secret
 * store that every workspace's runs read from; members and workspace admins no
 * longer see the page at all.
 */
export const SUPER_ADMIN_ONLY_SECTIONS = new Set(['envvars'])

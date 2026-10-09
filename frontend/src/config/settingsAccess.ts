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
  websearch: 'admin',
  chathistory: 'admin',
  vectorstore: 'admin',
  parser: 'admin',
  storage: 'admin',
  sandbox: 'admin',
  // Install writes a root shell into the sandbox image every session of
  // that config boots. Same Admin+ bar as the sandbox editor itself.
  skills: 'admin',
  mcp: 'admin',
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
  'runtime-queues',
  'platform-api-keys',
  'system-audit-log',
])

/**
 * Sections that model management has been reclaimed to: online-model
 * configuration (`models`) and the local/Ollama service (`ollama`) are now a
 * platform super-administrator concern. Unlike the per-tenant role ladder in
 * SETTINGS_SECTION_MIN_ROLE, these are gated strictly on `isSystemAdmin` —
 * cross-tenant superusers (canAccessAllTenants) do NOT bypass them, matching
 * the server-side RequireSystemAdmin guards on the model / Ollama mutation
 * routes. Members and workspace admins still consume the shared model
 * catalogue from KB / agent / memory pickers; they just cannot configure it.
 */
export const GLOBAL_MODEL_ADMIN_SECTIONS = new Set(['models', 'ollama'])

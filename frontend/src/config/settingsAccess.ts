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
  // 数据源是空间级（tenant-wide）配置：数据源挂在本空间各知识库下，配置一次
  // 对本空间所有知识库生效，所以它归空间管理员/所有者（admin 及以上），不再
  // 是运营管理员专属。入口位于「设置 → 个人设置 → 空间」分组。
  'kb-datasource': 'admin',
  mymemory: 'viewer',
  // NOTE: `memory`(长期记忆)不再按租户角色判断——它是平台级配置
  // (system setting memory.global_config)，入口收归超管（运营管理员继承）。
  // 见 SUPER_ADMIN_ONLY_SECTIONS。
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
 *   - `model-catalog`: the built-in model catalogue (context windows, thinking
 *                     levels, input modalities, candidate visibility), whose
 *                     changes apply to every workspace
 *   - `mcp`         : MCP servers
 *   - `vectorstore` : vector database engines
 *   - `parser`      : document parsing engines
 *   - `storage`     : object/file storage engines
 *   - `sandbox`     : sandbox runtime configuration
 *   - `skills`      : skill management / install (writes into the sandbox image)
 *   - `websearch`   : network search providers
 *
 * NOTE: `kb-datasource` used to live here. Data sources are a workspace-level
 * concern configured by workspace admins (see SETTINGS_SECTION_MIN_ROLE).
 *
 * Visibility is enforced purely on the frontend (nav filter + Settings.vue
 * role-denied guard + per-page isOperationsAdmin gate), mirroring how `ollama`
 * was handled: the underlying backend routes are left at their current guards
 * because other admin surfaces may still reuse them.
 */
export const OPERATIONS_ADMIN_ONLY_SECTIONS = new Set([
  'ollama',
  'model-catalog',
  'mcp',
  'vectorstore',
  'parser',
  'storage',
  'sandbox',
  'skills',
  'websearch',
  // 知识库配置（平台默认）中收归运营管理员的三个分区（连超管也不显示），
  // 与同名基础设施页一致：向量存储 / 解析引擎 / 存储引擎。
  // 数据源不在此列：它是空间级配置，由空间管理员维护。
  'kb-vectorStore',
  'kb-parser',
  'kb-storage',
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
 *
 * kb-defaults: the platform-wide default knowledge-base configuration (models,
 * chunking, image, audio, graph, advanced, FAQ) that the KB editor no longer
 * exposes. Backed by system_settings "knowledge_base.default_config", whose
 * PUT route is SystemAdmin-gated; operations admins also see it because they
 * inherit isSystemAdmin.
 */
export const SUPER_ADMIN_ONLY_SECTIONS = new Set([
  'envvars',
  'kb-defaults',
  // 长期记忆（全平台开关 / 写入方式 / 召回 / Embedding 模型）是平台级配置，
  // memory.global_config 的 PUT 由服务端 RequireSystemAdmin 门控。普通成员、
  // 空间管理员/所有者都不再看到入口，只有超管（运营管理员继承）可见并在
  // MemoryWorkspaceSettings 内编辑；深链进入由 Settings.vue 的 canSeeSection
  // 兜底渲染「无权限」空态。
  'memory',
  // 知识库配置（平台默认）提升到设置主侧栏后拆成的独立分区键：模型 / 分块 /
  // 图像 / 音频 / 图谱 / 高级 / FAQ，仅超管（及其继承者运营管理员）可见。
  'kb-models',
  'kb-chunking',
  'kb-image',
  'kb-audio',
  'kb-graph',
  'kb-advanced',
  'kb-faq',
  // 智能体配置（平台默认）分区键，与知识库同样是超管专有：模型 / 多轮对话 /
  // 检索策略 / 网络搜索 / 附件上传。
  // 需求把这些全部划给超级管理员（没有给运营管理员的项），所以统一走
  // isSystemAdmin；「知识库检索」和「问题推荐」留给用户，不在这个集合里。
  // 工具配置 / MCP 服务 / 技能是用户级配置，在创建智能体时设置，不属于平台默认。
  'agent-model',
  'agent-conversation',
  'agent-retrieval',
  'agent-websearch',
  'agent-multimodal',
])

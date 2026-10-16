import type { DeploymentCapabilityKey } from './deploymentCapabilities'

export const CHROME_EXTENSION_URL =
  'https://chromewebstore.google.com/detail/jpemjbopikggjlmikmclgbmkhhopjdgd?utm_source=item-share-cb'

export const CLAWHUB_SKILL_URL = 'https://clawhub.ai/lyingbug/weknora'

export type IntegrationTab = 'im' | 'embed' | 'api' | 'mcpserver' | 'cli' | 'chrome' | 'claw'

export const INTEGRATION_TABS: IntegrationTab[] = ['im', 'embed', 'api', 'mcpserver', 'cli', 'chrome', 'claw']

/** Aligns with Settings.vue SECTION_MIN_ROLE.api and router.go g.Owner() on /api-principal-config. */
export type IntegrationTabRole = 'viewer' | 'contributor' | 'admin' | 'owner'

export const INTEGRATION_TAB_MIN_ROLE: Partial<Record<IntegrationTab, IntegrationTabRole>> = {
  api: 'owner',
}

/**
 * Integration tabs that are a pure platform super-administrator concern. Unlike
 * the per-tenant role ladder in INTEGRATION_TAB_MIN_ROLE, these are gated
 * strictly on `isSystemAdmin` — cross-tenant superusers (canAccessAllTenants)
 * do NOT bypass them, mirroring how GLOBAL_MODEL_ADMIN_SECTIONS / 
 * SUPER_ADMIN_ONLY_SECTIONS gate the model / sandbox-secret pages.
 * chrome: 「Chrome 插件」引导页对全部成员开放（普通成员、空间管理员、owner
 * 均可见），因此不在本集合内；页面内跳转「API 集成」的入口仍按 owner-only 控制。
 */
export const SYSTEM_ADMIN_INTEGRATION_TABS = new Set<IntegrationTab>([])

export const INTEGRATION_TAB_CAPABILITY: Partial<Record<IntegrationTab, DeploymentCapabilityKey>> = {
  im: 'integrations.im',
  embed: 'integrations.embed',
  api: 'integrations.api',
  mcpserver: 'integrations.mcpserver',
}

export type IntegrationPreviewIcon =
  | { type: 'icon'; name: string }
  | { type: 'emoji'; value: string }

/** Sidebar hover preview + Integrations modal nav — add new entries here. */
export const INTEGRATION_PREVIEW_ITEMS: Array<{
  key: IntegrationTab
  icon: IntegrationPreviewIcon
}> = [
  // { key: 'im', icon: { type: 'icon', name: 'chat-message' } },
  // { key: 'embed', icon: { type: 'icon', name: 'code' } },
  { key: 'api', icon: { type: 'icon', name: 'secured' } },
  { key: 'mcpserver', icon: { type: 'icon', name: 'tools' } },
  // { key: 'cli', icon: { type: 'icon', name: 'code' } },
  { key: 'chrome', icon: { type: 'icon', name: 'extension' } },
  // { key: 'claw', icon: { type: 'emoji', value: '🦞' } },
]

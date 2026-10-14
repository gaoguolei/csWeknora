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
 *
 * chrome: 浏览器插件的 API 凭证下发收归平台超管统一配置，普通用户与空间
 * owner/admin 不再看到该引导页。
 */
export const SYSTEM_ADMIN_INTEGRATION_TABS = new Set<IntegrationTab>(['chrome'])

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

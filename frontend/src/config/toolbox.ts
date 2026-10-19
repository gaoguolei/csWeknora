import { SETTINGS_SECTION_MIN_ROLE, OPERATIONS_ADMIN_ONLY_SECTIONS, type SettingsRoleKey } from './settingsAccess'
import { SETTINGS_SECTION_CAPABILITY, type DeploymentCapabilityKey } from './deploymentCapabilities'
import { SKILL_ICON } from '../types/mention'

export const TOOLBOX_ITEMS = [
  {
    key: 'skills',
    icon: SKILL_ICON,
    title: 'settings.skills.title',
    description: 'settings.skills.description',
    help: 'settings.skills.helpTooltip',
    action: 'settings.skills.addSkill',
  },
  {
    key: 'mcp',
    icon: 'tools',
    title: 'settings.mcpService',
    description: 'mcpSettings.description',
    action: 'mcpSettings.addService',
  },
  {
    key: 'browserconnection',
    title: 'localBrowser.settingsTitle',
    description: 'localBrowser.settingsDescription',
  },
] as const

export type ToolboxSection = typeof TOOLBOX_ITEMS[number]['key']

export function isToolboxSection(section: unknown): section is ToolboxSection {
  return TOOLBOX_ITEMS.some((item) => item.key === section)
}

export function toolboxLocation(section?: ToolboxSection, sandboxId?: string) {
  return {
    path: section ? `/platform/toolbox/${section}` : '/platform/toolbox',
    query: section === 'skills' && sandboxId ? { sandboxId } : {},
  }
}

/** Use the same role and deployment gates as the original settings panels. */
export function canAccessToolboxSection(
  section: ToolboxSection,
  access: {
    currentTenantRole: string | null | undefined
    canAccessAllTenants: boolean
    isOperationsAdmin: boolean
    isSystemAdmin: boolean
    hasRole: (role: SettingsRoleKey) => boolean
    isSupported: (capability?: DeploymentCapabilityKey) => boolean
  },
): boolean {
  if (!access.isSupported(SETTINGS_SECTION_CAPABILITY[section])) return false
  // Operations-admin-only tools (skills / mcp — the same set as `ollama`, see
  // OPERATIONS_ADMIN_ONLY_SECTIONS) are hidden from every other role, with no
  // tenant-role or cross-tenant-superuser bypass. Keeping this in sync with the
  // Settings.vue canSeeSection guard means the toolbox and the settings nav
  // never disagree about who can see these entries.
  if (OPERATIONS_ADMIN_ONLY_SECTIONS.has(section)) return access.isOperationsAdmin
  if (!access.currentTenantRole && !access.canAccessAllTenants) return false
  return access.canAccessAllTenants || access.hasRole(SETTINGS_SECTION_MIN_ROLE[section] ?? 'viewer')
}

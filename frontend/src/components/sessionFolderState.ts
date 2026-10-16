import type { SessionFolderState as ServerFolderState } from '@/api/auth'

/**
 * 侧栏「项目」文件夹组织在 localStorage 写穿缓存与服务端 preferences 之间的
 * 归一化与取舍。
 *
 * 这块逻辑出过一次数据丢失：早先的实现把「服务端没有这个空间的记录」和
 * 「记录是空的」当成同一件事，于是空状态会被当成真相应用、并写回另一侧，
 * 两边一起销毁——文件夹消失，而 assignments 还在时所有会话会落回外层。
 * 因此取舍规则单独抽到这里并配测试：空状态永远不许覆盖非空状态。
 */

export type ConversationFolder = {
  id: string
  name: string
  collapsed: boolean
}

export type FolderLayout = {
  folders: ConversationFolder[]
  assignments: Record<string, string>
  projectsCollapsed: boolean
  sortMode: 'recent' | 'manual'
}

export const emptyFolderLayout = (): FolderLayout => ({
  folders: [],
  assignments: {},
  projectsCollapsed: false,
  sortMode: 'manual',
})

/**
 * 同时接受服务端（snake_case）与旧 localStorage（camelCase）两种形状，
 * 使升级前的本地数据可以直接迁移，无需一次性转换脚本。
 */
export const normalizeFolderLayout = (saved: unknown): FolderLayout => {
  const raw = (saved ?? {}) as Record<string, any>
  return {
    folders: Array.isArray(raw.folders)
      ? raw.folders
          .filter(
            (folder: any) =>
              folder &&
              typeof folder.id === 'string' &&
              typeof folder.name === 'string',
          )
          .map((folder: any) => ({
            id: folder.id,
            name: folder.name,
            collapsed: Boolean(folder.collapsed),
          }))
      : [],
    assignments:
      raw.assignments && typeof raw.assignments === 'object'
        ? (raw.assignments as Record<string, string>)
        : {},
    projectsCollapsed:
      raw.projectsCollapsed === true || raw.projects_collapsed === true,
    sortMode:
      (raw.sortMode ?? raw.sort_mode) === 'recent' ? 'recent' : 'manual',
  }
}

/** 有实际内容：至少一个文件夹，或至少一条会话归属。 */
export const hasFolderLayout = (
  layout: FolderLayout | null,
): layout is FolderLayout =>
  !!layout &&
  (layout.folders.length > 0 || Object.keys(layout.assignments).length > 0)

export type FolderLayoutResolution = {
  layout: FolderLayout
  /** true = 服务端没有可用内容而本地有，应把本地补写上去（迁移 / 自愈）。 */
  upload: boolean
}

/**
 * 服务端与本地二选一。服务端有内容时以它为准（换设备后它才是真相），
 * 否则本地非空就用本地并要求补写；两侧都空才真正当作「没组织过」。
 */
export const resolveFolderLayout = (
  remote: FolderLayout | null,
  local: FolderLayout | null,
): FolderLayoutResolution => {
  if (hasFolderLayout(remote)) return { layout: remote, upload: false }
  if (hasFolderLayout(local)) return { layout: local, upload: true }
  return { layout: emptyFolderLayout(), upload: false }
}

/** 转成服务端形状；每次只用于一个空间，后端按空间键合并。 */
export const toServerLayout = (layout: FolderLayout): ServerFolderState => ({
  folders: layout.folders.map((folder) => ({
    id: folder.id,
    name: folder.name,
    collapsed: folder.collapsed,
  })),
  assignments: layout.assignments,
  sort_mode: layout.sortMode,
  projects_collapsed: layout.projectsCollapsed,
})
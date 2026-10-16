import { get, post, put } from '@/utils/request'

export interface ProgrammingNode {
  path: string
  name: string
  kind: 'file' | 'directory'
  size?: number
  modified?: string
}

export interface ProgrammingTree {
  root: string
  origin: 'host' | 'sandbox'
  /** "expired" means the session sandbox was reclaimed and its files are gone. */
  workspace_state?: 'live' | 'expired'
  nodes: ProgrammingNode[]
}

export interface ProgrammingFile {
  path: string
  content: string
  hash: string
  size: number
  modified?: string
  /** Raster images use base64; text files keep the original UTF-8 content. */
  encoding?: 'base64'
  mime_type?: string
}

export interface ProgrammingCommandResult {
  stdout: string
  stderr: string
  exit_code: number
  duration_ms: number
  killed: boolean
}

export interface ProgrammingGitStatusFile {
  path: string
  old_path?: string
  status: string
  added: number
  removed: number
}

/** One git repository discovered in the workspace, with its own change list. */
export interface ProgrammingGitRepo {
  /** Workspace-relative repo root; "" for a repo at the workspace root. */
  root: string
  name: string
  files: ProgrammingGitStatusFile[]
  /** Cached local branch/upstream metadata; does not wait for network fetch. */
  remote?: ProgrammingGitRemote
}

export interface ProgrammingGitStatus {
  available: boolean
  /** Present on the multi-repo shape. One entry per discovered repository. */
  repos?: ProgrammingGitRepo[]
  /** Legacy single-repo shape, kept for backward compatibility. */
  files?: ProgrammingGitStatusFile[]
  message?: string
}
const base = (sessionId: string) => `/api/v1/sessions/${encodeURIComponent(sessionId)}/programming`

export function getProgrammingTree(sessionId: string, path = '') {
  const query = path ? `?path=${encodeURIComponent(path)}` : ''
  return get<{ success: boolean; data: ProgrammingTree }>(`${base(sessionId)}/tree${query}`)
}

export function getProgrammingGitStatus(sessionId: string) {
  return get<{ success: boolean; data: ProgrammingGitStatus }>(`${base(sessionId)}/git-status`)
}

export interface ProgrammingGitCommitResult {
  committed: boolean
  /** git's own stderr/stdout when the commit was refused (nothing staged, missing identity, hook). */
  detail?: string
  /** True when the refusal is a missing user.name/user.email the panel can offer to fix. */
  needs_identity?: boolean
}

export function setProgrammingGitIdentity(sessionId: string, data: { name: string; email: string; repo?: string }) {
  return post<{ success: boolean; data: { saved: boolean } }>(`${base(sessionId)}/git-identity`, data, { timeout: 30_000 })
}

export interface ProgrammingGitBranches {
  branches: string[]
  /** Remote tracking refs, e.g. origin/feature; excludes origin/HEAD aliases. */
  remote_branches?: string[]
  current: string
}

export function getProgrammingGitBranches(sessionId: string, repo = '') {
  const query = repo ? `?repo=${encodeURIComponent(repo)}` : ''
  return get<{ success: boolean; data: ProgrammingGitBranches }>(`${base(sessionId)}/git-branches${query}`, { timeout: 20_000 })
}

export interface ProgrammingGitHistoryCommit {
  sha: string
  short_sha: string
  author: string
  committed_at: string
  subject: string
  body: string
  parents: string[]
  refs: string
  status: 'outgoing' | 'incoming' | 'synced' | 'unknown'
  /** Verified against the current branch; incoming/stale/published targets are excluded. */
  can_reset?: boolean
}

export interface ProgrammingGitHistory {
  commits: ProgrammingGitHistoryCommit[]
  head: string
  /** Upstream object ID anchors the same history snapshot across pages. */
  upstream: string
  has_more: boolean
}

export function getProgrammingGitHistory(sessionId: string, repo = '', page: { skip?: number; head?: string; upstream?: string } = {}) {
  const params = new URLSearchParams({ limit: '30' })
  if (repo) params.set('repo', repo)
  if (page.skip) params.set('skip', String(page.skip))
  if (page.head) {
    params.set('head', page.head)
    params.set('upstream', page.upstream ?? '')
  }
  return get<{ success: boolean; data: ProgrammingGitHistory }>(`${base(sessionId)}/git-history?${params}`, { timeout: 75_000 })
}

export interface ProgrammingGitHistoryFile {
  path: string
  old_path?: string
  status: string
}

export interface ProgrammingGitHistoryFiles {
  files: ProgrammingGitHistoryFile[]
  parent: string
  patch?: string
  patch_truncated?: boolean
}

export function getProgrammingGitHistoryFiles(sessionId: string, commit: string, repo = '', path?: string) {
  const params = new URLSearchParams({ commit })
  if (repo) params.set('repo', repo)
  if (path !== undefined) params.set('path', path)
  return get<{ success: boolean; data: ProgrammingGitHistoryFiles }>(`${base(sessionId)}/git-history-files?${params}`, { timeout: 75_000 })
}

export interface ProgrammingGitActionResult {
  switched?: boolean
  discarded?: boolean
  branch?: string
  /** git's own stderr/stdout when the action was refused (dirty tree, unknown branch). */
  detail?: string
}

export function switchProgrammingGitBranch(sessionId: string, branch: string, repo = '', remote = false) {
  return post<{ success: boolean; data: ProgrammingGitActionResult }>(`${base(sessionId)}/git-switch-branch`, { branch, repo, remote }, { timeout: 90_000 })
}

export function discardProgrammingChanges(sessionId: string, repo = '') {
  return post<{ success: boolean; data: ProgrammingGitActionResult }>(`${base(sessionId)}/git-discard`, { repo }, { timeout: 45_000 })
}

export interface ProgrammingGitResetResult {
  reset: boolean
  reason?: 'not_on_branch' | 'already_at_commit' | 'published_history' | 'dirty_tree' | 'git_error'
  /** git's own stderr/stdout when the reset was refused (published commit, dirty tree). */
  detail?: string
}

/** Moves the branch back to an unpushed commit, keeping its changes in the tree. */
export function resetProgrammingGitCommit(sessionId: string, commit: string, repo = '') {
  return post<{ success: boolean; data: ProgrammingGitResetResult }>(`${base(sessionId)}/git-reset`, { repo, commit }, { timeout: 45_000 })
}

export function commitProgrammingChanges(sessionId: string, message: string, repo = '', paths?: string[]) {
  return post<{ success: boolean; data: ProgrammingGitCommitResult }>(`${base(sessionId)}/git-commit`, { message, repo, paths }, { timeout: 120_000 })
}

export interface ProgrammingGitRemoteIncoming {
  sha: string
  subject: string
}

export interface ProgrammingGitRemote {
  available: boolean
  branch: string
  /** Empty when the branch has no upstream configured. */
  upstream: string
  ahead: number
  behind: number
  /** True only when this read actually refreshed refs from the remote. */
  fetched: boolean
  incoming: ProgrammingGitRemoteIncoming[]
}

export function getProgrammingGitRemote(sessionId: string, refresh = false, repo = '') {
  const params = new URLSearchParams()
  if (refresh) params.set('refresh', '1')
  if (repo) params.set('repo', repo)
  const query = params.toString() ? `?${params}` : ''
  return get<{ success: boolean; data: ProgrammingGitRemote }>(`${base(sessionId)}/git-remote${query}`, { timeout: 75_000 })
}

export interface ProgrammingGitPullResult {
  pulled: boolean
  /** git's own stderr/stdout on a refusal (diverged branch, no upstream, offline). */
  detail?: string
  /** True when the local commits were unwound first to let the pull fast-forward. */
  aligned?: boolean
  conflicts?: boolean
  restore_failed?: boolean
  rollback_failed?: boolean
  backup_ref?: string
  stash_ref?: string
}

/** realign unwinds local commits (keeping their changes) so a diverged branch can pull. */
export function pullProgrammingChanges(sessionId: string, repo = '', realign = false) {
  return post<{ success: boolean; data: ProgrammingGitPullResult }>(`${base(sessionId)}/git-pull`, { repo, realign }, { timeout: realign ? 240_000 : 70_000 })
}

export interface ProgrammingGitPushCredentials {
  username?: string
  password?: string
  remote_url: string
  use_ssh?: boolean
}

export interface ProgrammingGitPushResult {
  pushed: boolean
  auth_required?: boolean
  auth_failed?: boolean
  auth_mode?: 'http' | 'ssh'
  username?: string
  credentials_saved?: boolean
  remote_url?: string
  /** A refusal or authentication failure; local commits remain available. */
  detail?: string
}

export function pushProgrammingChanges(sessionId: string, repo = '', credentials?: ProgrammingGitPushCredentials) {
  return post<{ success: boolean; data: ProgrammingGitPushResult }>(`${base(sessionId)}/git-push`, { repo, ...credentials }, { timeout: 120_000 })
}

export function getProgrammingFile(sessionId: string, path: string) {
  return get<{ success: boolean; data: ProgrammingFile }>(`${base(sessionId)}/file?path=${encodeURIComponent(path)}`)
}

export function saveProgrammingFile(sessionId: string, data: { path: string; content: string; base_hash?: string }) {
  return put<{ success: boolean; data: ProgrammingFile }>(`${base(sessionId)}/file`, data)
}

export function createProgrammingFile(sessionId: string, path: string) {
  return post<{ success: boolean; data: ProgrammingFile }>(`${base(sessionId)}/file`, { path })
}

export function moveProgrammingFile(sessionId: string, source: string, target: string) {
  return post<{ success: boolean; data: ProgrammingFile }>(`${base(sessionId)}/file/move`, { source, target })
}

export function deleteProgrammingFile(sessionId: string, path: string) {
  return post<{ success: boolean; data: { path: string } }>(`${base(sessionId)}/file/delete`, { path })
}

export function runProgrammingCommand(sessionId: string, data: { command: string; path?: string; timeout?: number }) {
  return post<{ success: boolean; data: ProgrammingCommandResult }>(`${base(sessionId)}/command`, data, { timeout: 70_000 })
}

export interface ProgrammingWebPreview {
  status: 'ready' | 'not_running' | 'sandbox_not_running' | 'sandbox_unconfigured' | 'sandbox_start_failed' | 'project_missing' | 'starting' | 'start_failed' | 'paused' | 'unsupported' | 'setup_required' | 'unreachable' | 'error'
  url?: string
  preview_id?: string
  port?: number
  detail?: string
}
export function discoverProgrammingWebPreview(sessionId: string, data: { start?: boolean; preview_id?: string; agent_id?: string; agent_source_tenant_id?: string | number } = {}) {
  return post<{ success: boolean; data: ProgrammingWebPreview }>(`${base(sessionId)}/web-preview`, data, { timeout: data.start ? 150_000 : 45_000 })
}

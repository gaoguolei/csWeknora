<template>
  <section class="git-history" :aria-label="t('workspace.commitHistory')">
    <header class="git-history__header">
      <button type="button" class="git-history__toggle" :aria-expanded="!collapsed" @click="collapsed = !collapsed">
        <t-icon :name="collapsed ? 'chevron-right' : 'chevron-down'" size="13px" />
        <t-icon name="history" size="14px" />
        <span>{{ t('workspace.commitHistory') }}</span>
        <span v-if="report?.commits.length" class="git-history__count">{{ report.commits.length }}</span>
      </button>
      <button type="button" class="git-history__action" :disabled="loading" :title="t('workspace.historyRefresh')"
        :aria-label="t('workspace.historyRefresh')" @click="emit('refresh')">
        <t-icon name="refresh" size="13px" :class="{ 'is-spinning': loading }" />
      </button>
    </header>
    <div v-show="!collapsed" class="git-history__content" :aria-busy="loading">
      <p v-if="error" class="git-history__notice is-error" role="alert">{{ error }}</p>
      <div v-if="loading && !report?.commits.length" class="git-history__skeleton" role="status" :aria-label="t('workspace.historyLoading')">
        <div v-for="row in 3" :key="row" class="git-history__skeleton-row"><i /><span /><small /></div>
      </div>
      <p v-else-if="!report?.commits.length && !error" class="git-history__notice">{{ t('workspace.historyEmpty') }}</p>
      <ol v-else-if="report?.commits.length" class="git-history__list">
        <li v-for="commit in report.commits" :key="commit.sha" class="git-history__item" :class="{ 'is-head': commit.sha === report.head }">
          <details class="git-history__entry" @toggle="onCommitToggle($event, commit.sha)">
            <summary class="git-history__row" :title="`${commit.subject}\n${commit.author} · ${formatMessageTimestamp(commit.committed_at)}\n${commit.sha}`">
              <span class="git-history__node" aria-hidden="true"><t-icon v-if="commit.parents?.length > 1" name="git-merge" size="13px" /><i v-else /></span>
              <span class="git-history__subject">{{ commit.subject }}</span>
              <span class="git-history__author">{{ commit.author }}</span>
              <button v-if="validations[commit.sha]" class="git-history__validation" :class="validations[commit.sha].result.outcome" type="button" :title="t('workspace.validationViewReport')" @click.stop.prevent="openValidationReport(validations[commit.sha].report_message_id)">
                <t-icon :name="validations[commit.sha].result.outcome === 'passed' ? 'check-circle' : 'error-circle'" size="12px" />{{ t(`workspace.validationState_${validations[commit.sha].result.outcome}`) }}
              </button>
              <span v-if="commit.status === 'outgoing' || commit.status === 'incoming'" class="git-history__state" :class="`is-${commit.status}`">
                {{ t(commit.status === 'outgoing' ? 'workspace.historyOutgoing' : 'workspace.historyIncoming') }}
              </span>
              <span v-for="decoration in decorations(commit.refs)" :key="decoration.label" class="git-history__ref" :class="{ 'is-local': decoration.local }" :title="decoration.label">
                <t-icon :name="decoration.icon" size="11px" /><span>{{ decoration.label }}</span>
              </span>
              <span class="git-history__actions">
                <button v-if="commit.can_reset && commit.sha !== report.head" type="button" class="git-history__action"
                  :title="t('workspace.historyRevert')" :aria-label="t('workspace.historyRevert')" @click.stop.prevent="emit('revert', commit)"><t-icon name="rollback" size="13px" /></button>
                <button type="button" class="git-history__action" :title="t('workspace.copyCommitHash')" :aria-label="t('workspace.copyCommitHash')"
                  @click.stop.prevent="copyWithToast(commit.sha, 'workspace.commitHashCopied')"><t-icon name="copy" size="13px" /></button>
              </span>
            </summary>
            <div class="git-history__expanded">
              <p v-if="filesLoading[commit.sha]" class="git-history__notice" role="status"><t-icon name="loading" size="12px" class="is-spinning" /> {{ t('workspace.historyFilesLoading') }}</p>
              <p v-else-if="filesErrors[commit.sha]" class="git-history__notice is-error" role="alert">
                {{ filesErrors[commit.sha] }} <button type="button" @click="loadCommitFiles(commit.sha, true)">{{ t('workspace.retry') }}</button>
              </p>
              <p v-else-if="files[commit.sha] && !files[commit.sha].files.length" class="git-history__notice">{{ t('workspace.historyFilesEmpty') }}</p>
              <ul v-else class="git-history__files">
                <li v-for="changed in files[commit.sha]?.files ?? []" :key="changed.path">
                  <button type="button" class="git-history__file" :class="{ 'is-selected': selectedFiles[commit.sha] === changed.path }"
                    :title="changed.old_path ? `${changed.old_path} → ${changed.path}` : changed.path"
                    :aria-expanded="selectedFiles[commit.sha] === changed.path" @click="toggleFile(commit.sha, changed)">
                    <span class="git-history__file-icon" :class="fileBadge(changed.path).kind" aria-hidden="true">{{ fileBadge(changed.path).label }}</span>
                    <span class="git-history__filename">{{ basename(changed.path) }}</span>
                    <span class="git-history__directory">{{ directory(changed.path) }}</span>
                    <span class="git-history__file-status" :class="`is-${changed.status}`" :title="fileStatusTitle(changed.status)">{{ changed.status }}</span>
                  </button>
                  <div v-if="selectedFiles[commit.sha] === changed.path" class="git-history__diff">
                    <p v-if="patchLoading[patchKey(commit.sha, changed.path)]" class="git-history__notice" role="status"><t-icon name="loading" size="12px" class="is-spinning" /> {{ t('workspace.historyDiffLoading') }}</p>
                    <p v-else-if="patchErrors[patchKey(commit.sha, changed.path)]" class="git-history__notice is-error" role="alert">
                      {{ patchErrors[patchKey(commit.sha, changed.path)] }} <button type="button" @click="loadPatch(commit.sha, changed.path)">{{ t('workspace.retry') }}</button>
                    </p>
                    <template v-else-if="patches[patchKey(commit.sha, changed.path)]">
                      <p v-if="patches[patchKey(commit.sha, changed.path)].patch_truncated" class="git-history__notice">{{ t('workspace.historyDiffTruncated') }}</p>
                      <p v-if="!patches[patchKey(commit.sha, changed.path)].patch" class="git-history__notice">{{ t('workspace.noDiff') }}</p>
                      <div v-else class="git-history__patch" :aria-label="t('workspace.diff')">
                        <div v-for="(line, index) in patchLines(patches[patchKey(commit.sha, changed.path)].patch || '')" :key="index" class="git-history__patch-line" :class="`is-${line.kind}`">
                          <span class="git-history__gutter" aria-hidden="true">{{ line.old }}</span><span class="git-history__gutter" aria-hidden="true">{{ line.next }}</span>
                          <span class="git-history__sign" aria-hidden="true">{{ line.kind === 'add' ? '+' : line.kind === 'del' ? '-' : '' }}</span><code>{{ line.text || ' ' }}</code>
                        </div>
                      </div>
                    </template>
                  </div>
                </li>
              </ul>
              <div class="git-history__details">
                <p v-if="commit.body">{{ commit.body }}</p>
                <span>{{ formatMessageTimestamp(commit.committed_at) }}</span><code>{{ commit.short_sha }}</code>
              </div>
            </div>
          </details>
        </li>
      </ol>
      <button v-if="report?.has_more" type="button" class="git-history__more" :disabled="loading" @click="emit('loadMore')">
        <t-icon v-if="loading" name="loading" size="12px" class="is-spinning" />{{ t(loading ? 'workspace.historyLoading' : 'workspace.historyLoadMore') }}
      </button>
    </div>
    <ChatArtifactsDrawer v-model:visible="validationReportVisible" :session-id="sessionId || ''" :message-id="validationReportMessage" :preview-index="0" />
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { workspaceTestsAPI, type TestRun } from '@/api/workspaceTests'
import { commitValidationRuns } from '@/utils/workspaceValidation'
import ChatArtifactsDrawer from '@/views/chat/components/ChatArtifactsDrawer.vue'
import { useI18n } from 'vue-i18n'
import type { ProgrammingGitHistory, ProgrammingGitHistoryCommit, ProgrammingGitHistoryFile, ProgrammingGitHistoryFiles } from '@/api/programming'
import { copyWithToast } from '@/utils/clipboard'
import { formatMessageTimestamp } from '@/utils/messageTimestamp'

const props = defineProps<{
  report?: ProgrammingGitHistory | null; loading?: boolean; error?: string; sessionId?: string; repo?: string;
  loadFiles?: (commit: string, path?: string) => Promise<ProgrammingGitHistoryFiles>;
}>()
const emit = defineEmits<{ refresh: []; loadMore: []; revert: [commit: ProgrammingGitHistoryCommit] }>()
const { t } = useI18n()
const validations = ref<Record<string,TestRun>>({})
const validationReportVisible = ref(false), validationReportMessage = ref('')
let validationEpoch = 0
watch(() => [props.sessionId, props.repo, props.report?.head], async () => {
  const epoch = ++validationEpoch
  validations.value = {}
  if (!props.sessionId) return
  try {
    const runs = await workspaceTestsAPI.runs(props.sessionId)
    if (epoch === validationEpoch) validations.value = commitValidationRuns(runs,props.repo || '')
  } catch { /* Validation metadata is optional; Git history stays usable. */ }
}, { immediate:true })
function openValidationReport(id?:string) { if(id) { validationReportMessage.value=id; validationReportVisible.value=true } }
const collapsed = ref(false)
const files = ref<Record<string, ProgrammingGitHistoryFiles>>({})
const filesLoading = ref<Record<string, boolean>>({})
const filesErrors = ref<Record<string, string>>({})
const selectedFiles = ref<Record<string, string | null>>({})
const patches = ref<Record<string, ProgrammingGitHistoryFiles>>({})
const patchLoading = ref<Record<string, boolean>>({})
const patchErrors = ref<Record<string, string>>({})
let generation = 0
watch(() => [props.sessionId, props.repo], () => {
  generation++
  files.value = {}; filesLoading.value = {}; filesErrors.value = {}; selectedFiles.value = {}
  patches.value = {}; patchLoading.value = {}; patchErrors.value = {}
})
onBeforeUnmount(() => { generation++ })
function onCommitToggle(event: Event, sha: string) {
  if ((event.target as HTMLDetailsElement).open) void loadCommitFiles(sha)
}
async function loadCommitFiles(sha: string, force = false) {
  if (filesLoading.value[sha] || (files.value[sha] && !force)) return
  if (!props.loadFiles) { filesErrors.value[sha] = t('workspace.historyFilesFailed'); return }
  const epoch = generation
  filesLoading.value[sha] = true; filesErrors.value[sha] = ''
  try {
    const data = await props.loadFiles(sha)
    if (epoch === generation) files.value[sha] = data
  } catch {
    if (epoch === generation) filesErrors.value[sha] = t('workspace.historyFilesFailed')
  } finally {
    if (epoch === generation) filesLoading.value[sha] = false
  }
}
function patchKey(sha: string, path: string) { return `${sha}:${path}` }
async function toggleFile(sha: string, file: ProgrammingGitHistoryFile) {
  if (selectedFiles.value[sha] === file.path) { selectedFiles.value[sha] = null; return }
  selectedFiles.value[sha] = file.path
  if (!patches.value[patchKey(sha, file.path)]) await loadPatch(sha, file.path)
}
async function loadPatch(sha: string, path: string) {
  const key = patchKey(sha, path), epoch = generation
  if (patchLoading.value[key] || !props.loadFiles) return
  patchLoading.value[key] = true; patchErrors.value[key] = ''
  try {
    const data = await props.loadFiles(sha, path)
    if (epoch === generation) patches.value[key] = data
  } catch {
    if (epoch === generation) patchErrors.value[key] = t('workspace.historyDiffFailed')
  } finally {
    if (epoch === generation) patchLoading.value[key] = false
  }
}
function basename(path: string) { return path.slice(path.lastIndexOf('/') + 1) }
function directory(path: string) { return path.slice(0, Math.max(0, path.lastIndexOf('/'))) }
function fileBadge(path: string) {
  const extension = basename(path).split('.').pop()?.toLowerCase()
  if (extension === 'vue') return { label: 'V', kind: 'is-vue' }
  if (extension === 'ts' || extension === 'tsx') return { label: 'TS', kind: 'is-ts' }
  if (extension === 'js' || extension === 'jsx') return { label: 'JS', kind: 'is-js' }
  if (extension === 'json') return { label: '{}', kind: 'is-json' }
  if (extension === 'ico') return { label: '★', kind: 'is-js' }
  if (extension === 'md') return { label: 'M↓', kind: 'is-md' }
  return { label: extension?.slice(0, 3).toUpperCase() || '◇', kind: 'is-other' }
}
function fileStatusTitle(status: string) {
  const labels: Record<string, string> = { A: 'historyFileAdded', M: 'historyFileModified', D: 'historyFileDeleted', R: 'historyFileRenamed', C: 'historyFileCopied', T: 'historyFileModified' }
  return labels[status] ? t(`workspace.${labels[status]}`) : status
}
function decorations(refs: string) {
  return refs.split(', ').filter(Boolean).map(ref => ({
    label: ref.replace(/^HEAD -> /, '').replace(/^tag: /, ''),
    local: ref.startsWith('HEAD -> ') || ref === 'HEAD',
    icon: ref.startsWith('tag: ') ? 'tag' : ref.includes('/') ? 'cloud' : 'git-branch',
  }))
}
function patchLines(patch: string) {
  let old = 0, next = 0, inHunk = false
  return patch.replace(/\n$/, '').split('\n').map(text => {
    const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(text)
    if (hunk) { old = Number(hunk[1]); next = Number(hunk[2]); inHunk = true; return { text, kind: 'hunk', old: '', next: '' } }
    if (text.startsWith('diff --git ')) inHunk = false
    if (inHunk && text.startsWith('+')) return { text: text.slice(1), kind: 'add', old: '', next: next++ }
    if (inHunk && text.startsWith('-')) return { text: text.slice(1), kind: 'del', old: old++, next: '' }
    if (inHunk && text.startsWith(' ')) return { text: text.slice(1), kind: 'context', old: old++, next: next++ }
    return { text, kind: 'meta', old: '', next: '' }
  })
}
</script>

<style scoped lang="less">
.git-history { --git-history-accent: var(--td-success-color); flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; margin-top: 8px; }
.git-history__header { display: flex; align-items: center; gap: 8px; padding: 6px 0; }
.git-history__toggle { display: flex; flex: 1; align-items: center; gap: 6px; min-width: 0; border: 0; padding: 2px 0; background: transparent; color: var(--td-text-color-secondary); font: inherit; font-size: 12px; font-weight: 600; text-align: left; cursor: pointer; }
.git-history__count { padding: 0 5px; border-radius: 4px; background: var(--td-bg-color-secondarycontainer); font-size: 10px; line-height: 17px; font-variant-numeric: tabular-nums; }
.git-history__action { display: inline-flex; align-items: center; justify-content: center; flex: none; width: 24px; height: 24px; padding: 0; border: 0; border-radius: 4px; background: transparent; color: var(--td-text-color-placeholder); cursor: pointer; }
.git-history__action:hover:not(:disabled) { background: var(--td-bg-color-secondarycontainer); color: var(--td-text-color-primary); }
.git-history button:disabled { opacity: .5; cursor: not-allowed; }
.git-history button:focus-visible, .git-history summary:focus-visible { outline: 2px solid var(--td-brand-color); outline-offset: -2px; }
.git-history__content { flex: 1 1 auto; min-height: 0; overflow: auto; scrollbar-width: thin; }
.git-history__list, .git-history__files { margin: 0; padding: 0; list-style: none; }
.git-history__row { position: relative; display: flex; align-items: center; gap: 7px; min-height: 36px; padding: 2px 4px 2px 0; border-radius: 4px; cursor: pointer; list-style: none; }
.git-history__row::before, .git-history__row::after { position: absolute; z-index: 1; left: 11px; width: 1px; background: var(--git-history-accent); opacity: .7; content: ''; pointer-events: none; }
.git-history__row::before { top: 0; bottom: calc(50% + 10px); }
.git-history__row::after { top: calc(50% + 10px); bottom: 0; }
.git-history__item:first-child .git-history__row::before, .git-history__item:last-child .git-history__row::after { display: none; }
.git-history__row::-webkit-details-marker { display: none; }
.git-history__row:hover, .git-history__entry[open] > .git-history__row { background: var(--td-bg-color-container-hover); }
.git-history__node { position: relative; z-index: 2; display: flex; align-items: center; justify-content: center; flex: none; width: 23px; height: 24px; color: var(--git-history-accent); }
.git-history__node i { width: 12px; height: 12px; border-radius: 50%; background: var(--git-history-accent); }
.git-history__entry[open] .git-history__node i { width: 14px; height: 14px; }
.git-history__item.is-head .git-history__node i { width: 16px; height: 16px; border: 2px solid var(--git-history-accent); background: var(--td-bg-color-container); }
.git-history__subject { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; font-weight: 500; color: var(--td-text-color-primary); }
.git-history__author { min-width: 0; max-width: 100px; flex-shrink: 2; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--td-text-color-placeholder); font-size: 12px; }
.git-history__validation { display:inline-flex; align-items:center; gap:3px; padding:2px 5px; border:0; background:var(--td-bg-color-secondarycontainer); border-radius:4px; color:var(--td-warning-color); font:inherit; font-size:10px; cursor:pointer; white-space:nowrap; }
.git-history__validation.passed { color:var(--td-success-color); } .git-history__validation.failed { color:var(--td-error-color); }
.git-history__actions { display: inline-flex; margin-left: auto; flex: none; opacity: 0; }
.git-history__row:hover .git-history__actions, .git-history__row:focus-within .git-history__actions, .git-history__entry[open] .git-history__actions { opacity: 1; }
.git-history__ref { display: inline-flex; align-items: center; gap: 3px; flex: none; max-width: 105px; padding: 1px 5px; border-radius: 8px; color: var(--td-text-color-secondary); background: var(--td-bg-color-secondarycontainer); font-size: 10px; }
.git-history__ref span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.git-history__ref.is-local { color: var(--td-brand-color); background: var(--td-brand-color-light); }
.git-history__state { flex: none; font-size: 10px; color: var(--td-text-color-placeholder); }
.git-history__state.is-outgoing { color: var(--td-warning-color); }
.git-history__state.is-incoming { color: var(--td-brand-color); }
.git-history__expanded { position: relative; margin-left: 11px; border-left: 2px solid transparent; padding: 2px 0 4px 17px; }
.git-history__file { display: flex; align-items: center; gap: 7px; width: 100%; min-height: 31px; border: 0; border-radius: 3px; padding: 3px 8px 3px 0; background: transparent; color: var(--td-text-color-primary); font: inherit; text-align: left; cursor: pointer; }
.git-history__file:hover, .git-history__file.is-selected { background: var(--td-bg-color-container-hover); }
.git-history__file-icon { display: inline-flex; align-items: center; justify-content: center; flex: none; width: 18px; height: 19px; font-size: 10px; font-weight: 700; color: var(--td-text-color-placeholder); }
.git-history__file-icon.is-vue { font-size: 19px; color: var(--td-success-color); }
.git-history__file-icon.is-ts { color: var(--td-brand-color); background: var(--td-brand-color-light); }
.git-history__file-icon.is-js, .git-history__file-icon.is-json { color: var(--td-warning-color); }
.git-history__filename { min-width: 0; flex-shrink: 0; max-width: 55%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }
.git-history__directory { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--td-text-color-placeholder); font-size: 11px; }
.git-history__file-status { flex: none; width: 14px; text-align: right; font: 600 11px var(--app-font-family-mono, monospace); color: var(--td-brand-color); }
.git-history__file-status.is-D { color: var(--td-error-color); }
.git-history__file-status.is-A { color: var(--td-success-color); }
.git-history__file-status.is-R, .git-history__file-status.is-C { color: var(--td-warning-color); }
.git-history__details { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 12px; padding: 7px 4px; color: var(--td-text-color-placeholder); font-size: 10px; }
.git-history__details p { flex-basis: 100%; margin: 0 0 4px; color: var(--td-text-color-secondary); white-space: pre-wrap; overflow-wrap: anywhere; font-size: 12px; }
.git-history__details code { font-family: var(--app-font-family-mono, monospace); }
.git-history__notice { display: flex; align-items: center; flex-wrap: wrap; gap: 5px; margin: 5px 0; padding: 6px 4px; color: var(--td-text-color-placeholder); font-size: 12px; line-height: 1.5; }
.git-history__notice.is-error { color: var(--td-error-color); }
.git-history__notice button { border: 0; background: transparent; padding: 0; color: var(--td-brand-color); cursor: pointer; font: inherit; }
.git-history__diff { margin: 2px 0 8px; border: 1px solid var(--td-component-stroke); border-radius: 4px; overflow: hidden; }
.git-history__patch { max-height: 360px; overflow: auto; font: 11px/1.7 var(--app-font-family-mono, monospace); }
.git-history__patch-line { display: grid; grid-template-columns: 3em 3em 1.4em minmax(max-content, 1fr); min-height: 19px; }
.git-history__patch-line code { padding: 0 7px; white-space: pre; font: inherit; }
.git-history__gutter { padding: 0 4px; text-align: right; color: var(--td-text-color-placeholder); user-select: none; font-variant-numeric: tabular-nums; }
.git-history__sign { text-align: center; font-weight: 600; user-select: none; }
.is-add .git-history__sign { color: var(--td-success-color); }
.is-del .git-history__sign { color: var(--td-error-color); }
.git-history__patch-line.is-add { background: color-mix(in srgb, var(--td-success-color) 12%, transparent); }
.git-history__patch-line.is-del { background: color-mix(in srgb, var(--td-error-color) 12%, transparent); }
.git-history__patch-line.is-hunk { background: var(--td-bg-color-secondarycontainer); color: var(--td-brand-color); }
.git-history__patch-line.is-meta { color: var(--td-text-color-placeholder); }
.git-history__more { display: flex; align-items: center; justify-content: center; gap: 6px; width: 100%; margin-top: 6px; padding: 8px; border: 0; border-radius: 4px; background: transparent; color: var(--td-text-color-secondary); font: inherit; font-size: 11px; cursor: pointer; }
.git-history__more:hover:not(:disabled) { background: var(--td-bg-color-container-hover); }
.git-history__skeleton-row { display: flex; align-items: center; gap: 10px; height: 32px; }
.git-history__skeleton-row i { width: 8px; height: 8px; margin: 0 7px; border-radius: 50%; background: var(--td-bg-color-component); }
.git-history__skeleton-row span { width: 52%; height: 9px; border-radius: 3px; background: var(--td-bg-color-secondarycontainer); }
.git-history__skeleton-row small { width: 14%; height: 8px; border-radius: 3px; background: var(--td-bg-color-secondarycontainer); }
.is-spinning { animation: history-spin 1s linear infinite; }
@keyframes history-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .is-spinning { animation: none; } }
</style>

<template>
  <details class="file-diff" @toggle="onToggle">
    <summary class="file-diff__header">
      <span v-if="selectable" class="file-diff__selection" @click.stop @keydown.stop>
        <t-checkbox :checked="selected" :disabled="selectionDisabled"
          :aria-label="t('workspace.selectCommitFile', { path: change.path })"
          @change="(checked: boolean) => emit('selection-change', checked)">
          <span class="file-diff__selection-label">{{ t('workspace.selectCommitFile', { path: change.path }) }}</span>
        </t-checkbox>
      </span>
      <t-icon :name="expanded ? 'chevron-down' : 'chevron-right'" size="14px" />
      <t-icon name="file-code" size="17px" />
      <span class="file-diff__path" :title="change.path"><span>{{ directory }}</span><strong>{{ name }}</strong></span>
      <span class="file-diff__added">+{{ change.addedLines }}</span>
      <span class="file-diff__removed">−{{ change.removedLines }}</span>
    </summary>
    <div v-if="expanded" class="file-diff__body">
      <div class="file-diff__actions"><button type="button" @click="emit('open-source')">{{ t('workspace.viewFullSource') }} <t-icon name="arrow-right" /></button></div>
      <p v-if="!complete" class="file-diff__notice">{{ t('workspace.diffUnavailable') }}</p>
      <p v-else-if="!sections.length" class="file-diff__notice">{{ t('workspace.noDiff') }}</p>
      <template v-for="(section, index) in sections" :key="index">
        <div v-if="section.skipped" class="file-diff__gap">{{ t('workspace.unchangedLines', { count: section.skipped }) }}</div>
        <div v-for="(line, lineIndex) in section.lines" :key="lineIndex" class="diff-row" :class="`is-${line.type}`">
          <span class="diff-gutter" aria-hidden="true">{{ line.type === 'del' ? line.oldNumber : line.newNumber }}</span>
          <span class="diff-sign" aria-hidden="true">{{ line.type === 'add' ? '+' : line.type === 'del' ? '-' : '' }}</span>
          <code class="diff-code" v-html="line.highlighted || '&#8203;'" />
        </div>
      </template>
      <div v-if="trailingLines" class="file-diff__gap">{{ t('workspace.unchangedLines', { count: trailingLines }) }}</div>
    </div>
  </details>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FileChange } from '@/composables/useFileChanges'
import { buildUnifiedDiff } from '@/utils/unifiedDiff'
import { highlightWorkspaceLines, trimHighlightedIndent } from '@/utils/workspaceCode'

const props = defineProps<{ change: FileChange; selectable?: boolean; selected?: boolean; selectionDisabled?: boolean }>()
const emit = defineEmits<{ 'open-source': []; 'selection-change': [checked: boolean] }>()
const { t } = useI18n()
const expanded = ref(false)
function onToggle(event: Event) {
  expanded.value = (event.target as HTMLDetailsElement).open
}
const name = computed(() => props.change.path.replaceAll('\\', '/').split('/').pop() || props.change.path)
const directory = computed(() => props.change.path.slice(0, -name.value.length))
const complete = computed(() => props.change.before !== undefined && props.change.after !== undefined)
// Collapsed files do no diff/highlighting work. Updating a file preserves its open state.
const sections = computed(() => {
  if (!expanded.value || !complete.value) return []
  const { before = '', after = '', path } = props.change
  const diff = buildUnifiedDiff(before, after)
  const oldLines = highlightWorkspaceLines(before, path)
  const newLines = highlightWorkspaceLines(after, path)
  let previousLine = 0
  return diff.hunks.map(hunk => {
    const first = hunk.lines.find(line => line.newNumber !== null)?.newNumber
    const skipped = Math.max(0, (first ?? previousLine + 1) - previousLine - 1)
    for (const line of hunk.lines) if (line.newNumber !== null) previousLine = line.newNumber
    const indents = hunk.lines.filter(line => line.content.trim()).map(line => /^[\t ]*/.exec(line.content)![0])
    let commonIndent = indents[0] || ''
    for (const indent of indents) {
      while (commonIndent && !indent.startsWith(commonIndent)) commonIndent = commonIndent.slice(0, -1)
    }
    return { skipped, lines: hunk.lines.map(line => ({
      ...line,
      highlighted: trimHighlightedIndent(
        (line.type === 'del' ? oldLines[(line.oldNumber || 1) - 1] : newLines[(line.newNumber || 1) - 1]) || '',
        commonIndent.length,
      ),
    })) }
  })
})
const trailingLines = computed(() => {
  if (!sections.value.length) return 0
  const rows = sections.value.flatMap(section => section.lines)
  const last = [...rows].reverse().find(line => line.newNumber !== null)?.newNumber || 0
  const after = props.change.after || ''
  const total = after ? after.split('\n').length - (after.endsWith('\n') ? 1 : 0) : 0
  return Math.max(0, total - last)
})
</script>

<style scoped lang="less">
.file-diff { border-bottom: 1px solid var(--td-component-stroke); --diff-add: #1a7f37; --diff-del: #cf222e; }
.file-diff__header { display: flex; align-items: center; gap: 7px; padding: 14px 2px; cursor: pointer; list-style: none; font-size: 12px; }
.file-diff__header::-webkit-details-marker { display: none; }
.file-diff__selection { display: inline-flex; align-items: center; justify-content: center; flex: none; padding: 3px 0; }
.file-diff__selection-label { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
.file-diff__header > :not(.file-diff__path) { flex-shrink: 0; }
.file-diff__header:focus-visible { outline: 2px solid var(--td-brand-color); outline-offset: -2px; border-radius: 6px; }
.file-diff__path { flex: 1; min-width: 0; overflow-wrap: anywhere; color: var(--td-text-color-secondary); }
.file-diff__path strong { font-weight: 500; color: var(--td-text-color-primary); }
.file-diff__added { color: var(--diff-add); font-variant-numeric: tabular-nums; }
.file-diff__removed { color: var(--diff-del); font-variant-numeric: tabular-nums; }
.file-diff__body { padding-bottom: 12px; }
.file-diff__actions { display: flex; justify-content: flex-end; padding: 0 2px 8px; }
.file-diff__actions button { display: inline-flex; align-items: center; gap: 5px; border: 0; background: transparent; color: var(--td-text-color-secondary); font: inherit; font-size: 12px; cursor: pointer; padding: 4px; }
.file-diff__actions button:hover { color: var(--td-text-color-primary); }
.file-diff__gap, .file-diff__notice { margin: 4px 0; padding: 9px 12px; border-radius: 8px; background: var(--td-bg-color-secondarycontainer); color: var(--td-text-color-secondary); font-size: 12px; }
.diff-row { display: grid; grid-template-columns: 4em 1.4em minmax(0, 1fr); font: 13px/1.65 var(--app-font-family-mono, monospace); border-left: 2px solid transparent; }
.diff-row.is-add { background: color-mix(in srgb, var(--diff-add) 11%, transparent); border-left-color: var(--diff-add); }
.diff-row.is-del { background: color-mix(in srgb, var(--diff-del) 11%, transparent); border-left-color: var(--diff-del); }
.diff-gutter { padding: 0 8px 0 2px; text-align: right; color: var(--td-text-color-placeholder); user-select: none; }
.is-add .diff-gutter { color: var(--diff-add); }
.is-del .diff-gutter { color: var(--diff-del); }
.diff-sign { text-align: center; font-weight: 600; user-select: none; }
.is-add .diff-sign { color: var(--diff-add); }
.is-del .diff-sign { color: var(--diff-del); }
.diff-code { display: block; min-width: 0; padding: 0 8px; border-left: 1px solid var(--td-component-stroke); font: inherit; white-space: pre-wrap; overflow-wrap: anywhere; tab-size: 2; }
:global(:root[theme-mode="dark"] .file-diff) { --diff-add: #63c984; --diff-del: #ff8c86; }
.diff-code {

  :deep(.hljs-keyword),
  :deep(.hljs-doctag),
  :deep(.hljs-meta) {
    color: var(--ws-code-keyword);
  }

  :deep(.hljs-string),
  :deep(.hljs-regexp) {
    color: var(--ws-code-string);
  }

  :deep(.hljs-comment),
  :deep(.hljs-quote) {
    color: var(--ws-code-comment);
  }

  :deep(.hljs-number),
  :deep(.hljs-literal),
  :deep(.hljs-operator),
  :deep(.hljs-punctuation),
  :deep(.hljs-params) {
    color: var(--ws-code-accent);
  }

  :deep(.hljs-title),
  :deep(.hljs-name),
  :deep(.hljs-built_in),
  :deep(.hljs-type),
  :deep(.hljs-section),
  :deep(.hljs-selector-tag),
  :deep(.hljs-selector-id),
  :deep(.hljs-selector-pseudo),
  :deep(.hljs-selector-class),
  :deep(.hljs-variable) {
    color: var(--ws-code-entity);
  }

  :deep(.hljs-attr),
  :deep(.hljs-attribute),
  :deep(.hljs-property),
  :deep(.hljs-tag) {
    color: var(--ws-code-accent);
  }
}

</style>

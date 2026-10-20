import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';
import { computed, reactive, ref } from 'vue';
import { resolveKnowledgeSpreadsheetFileName } from './knowledgeDownloadFileName.ts';

const card = {
  id: 'cases', type: 'file', parse_status: 'completed', file_name: '登录测试用例',
  original_file_name: '登录测试用例.xlsx', file_type: 'XLSX',
};

test('the document menu exposes spreadsheet editing for extensionless display names', () => {
  const source = readFileSync(new URL('./components/DocumentActionMenu.vue', import.meta.url), 'utf8');
  const line = source.split('\n').find((line) => line.startsWith('const canEditSpreadsheet ='))!;
  assert.ok(line);
  const props = reactive({ item: card, canDownload: true });
  const canEdit = vm.runInNewContext(`${ts.transpile(line)}; canEditSpreadsheet`, {
    props, computed, resolveKnowledgeSpreadsheetFileName,
  });
  assert.equal(canEdit.value, true);
  props.canDownload = false;
  assert.equal(canEdit.value, false);
  assert.match(source, /v-if="item.type === 'manual' \|\| canEditSpreadsheet"/);
});

test('the knowledge page opens the spreadsheet editor with the source filename', () => {
  const source = readFileSync(new URL('./KnowledgeBase.vue', import.meta.url), 'utf8');
  const start = source.indexOf('const caseEditorVisible =');
  const end = source.indexOf('\n// Opens ONLY', start);
  assert.ok(start >= 0 && end > start);
  const manualCalls: unknown[] = [];
  const canEdit = ref(true);
  const canDownloadKnowledge = ref(true);
  const isCardDetails = ref(true);
  const state = vm.runInNewContext(`${ts.transpile(source.slice(start, end))}; ({ handleManualEdit, caseEditorVisible, caseEditorDocument })`, {
    ref, canEdit, canDownloadKnowledge, isCardDetails, resolveKnowledgeSpreadsheetFileName,
    isFAQ: ref(false), cardList: ref([card]), kbId: ref('kb'), manualEditorSuccess: () => {},
    uiStore: { openManualEditor: (value: unknown) => manualCalls.push(value) },
  });
  state.handleManualEdit(0, card);
  assert.equal(state.caseEditorVisible.value, true);
  assert.equal(state.caseEditorDocument.value.fileName, '登录测试用例.xlsx');
  assert.equal(isCardDetails.value, false);
  assert.equal(manualCalls.length, 0);
  assert.match(source, /<TestCaseKnowledgeEditor v-if="caseEditorVisible"/);
  assert.doesNotMatch(source, /<TestCaseKnowledgeEditor v-model:visible/);
  state.caseEditorVisible.value = false;
  canEdit.value = false;
  state.handleManualEdit(0, card);
  assert.equal(state.caseEditorVisible.value, false);
  canEdit.value = true;
  canDownloadKnowledge.value = false;
  state.handleManualEdit(0, card);
  assert.equal(state.caseEditorVisible.value, false);
});

test('document details offer an explicit edit button and keep read-only documents protected', () => {
  const source = readFileSync(new URL('../../components/doc-content.vue', import.meta.url), 'utf8');
  const start = source.indexOf('const canEditDocument =');
  const end = source.indexOf('\n\n', start);
  const props = reactive({ details: { id: 'cases', type: 'file', title: '登录测试用例.xlsx' }, canEditKB: true, canDownloadKB: true });
  const canEdit = vm.runInNewContext(`${ts.transpile(source.slice(start, end))}; canEditDocument`, {
    props, computed, resolveKnowledgeSpreadsheetFileName,
  });
  assert.equal(canEdit.value, true);
  props.canEditKB = false;
  assert.equal(canEdit.value, false);
  assert.match(source, /v-if="canEditDocument"/);
  assert.match(source, /@click="emit\('editDocument', details\)"/);
});

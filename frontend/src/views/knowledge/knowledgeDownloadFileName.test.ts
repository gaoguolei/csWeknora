import assert from 'node:assert/strict';
import test from 'node:test';

import { isBatchDownloadableKnowledge, resolveKnowledgeDownloadFileName, resolveKnowledgeSpreadsheetFileName } from './knowledgeDownloadFileName.ts';

test('prefers the original filename over the extensionless display name', () => {
  assert.equal(resolveKnowledgeDownloadFileName({
    id: 'knowledge-1',
    original_file_name: 'quarterly-report.pdf',
    file_name: 'quarterly-report',
    type: 'file',
  }), 'quarterly-report.pdf');
});

test('falls back to the backend filename when no original filename is present', () => {
  assert.equal(resolveKnowledgeDownloadFileName({
    id: 'knowledge-2',
    file_name: 'notes.txt',
    type: 'file',
  }), 'notes.txt');
});

test('batch download skips web pages without an original file', () => {
  assert.equal(isBatchDownloadableKnowledge({ type: 'manual' }), true);
  assert.equal(isBatchDownloadableKnowledge({ type: 'file', file_path: 'stored' }), true);
  assert.equal(isBatchDownloadableKnowledge({ type: 'url' }), false);
  assert.equal(isBatchDownloadableKnowledge({ type: 'url', file_path: 'snapshot.html' }), true);
  assert.equal(isBatchDownloadableKnowledge(undefined), false);
});

test('adds the markdown extension to manual documents exactly once', () => {
  assert.equal(resolveKnowledgeDownloadFileName({
    id: 'knowledge-3',
    title: 'meeting-notes',
    type: 'manual',
  }), 'meeting-notes.md');
  assert.equal(resolveKnowledgeDownloadFileName({
    id: 'knowledge-4',
    title: 'meeting-notes.md',
    type: 'manual',
  }), 'meeting-notes.md');
});

test('spreadsheet editing uses the original filename from a normalized knowledge list card', () => {
  assert.equal(resolveKnowledgeSpreadsheetFileName({
    id: 'cases', type: 'file', original_file_name: '登录测试用例.xlsx',
    file_name: '登录测试用例', file_type: 'XLSX',
  }), '登录测试用例.xlsx');
});

test('spreadsheet editing falls back to the file type when only the display name is present', () => {
  assert.equal(resolveKnowledgeSpreadsheetFileName({
    id: 'cases', type: 'file', file_name: '登录.v2', file_type: 'XLSX',
  }), '登录.v2.xlsx');
});

test('spreadsheet editing recognizes the filename used by document details', () => {
  assert.equal(resolveKnowledgeSpreadsheetFileName({
    id: 'cases', type: 'file', title: '登录测试用例.csv', file_type: 'csv',
  }), '登录测试用例.csv');
  assert.equal(resolveKnowledgeSpreadsheetFileName({
    id: 'cases', type: 'manual', title: '登录测试用例.xlsx',
  }), null);
  assert.equal(resolveKnowledgeSpreadsheetFileName({
    id: 'pdf', type: 'file', file_name: '需求', file_type: 'PDF',
  }), null);
});

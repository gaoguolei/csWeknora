import { isSpreadsheetArtifact } from '../../utils/testCaseSheet';

export interface KnowledgeDownloadItem {
  id: string;
  original_file_name?: string;
  file_name?: string;
  title?: string;
  type?: string;
  file_type?: string;
}

export function resolveKnowledgeDownloadFileName(item: KnowledgeDownloadItem): string {
  const baseName = item.original_file_name || item.file_name || item.title || item.id;
  if (item.type === 'manual' && !baseName.toLowerCase().endsWith('.md')) {
    return `${baseName}.md`;
  }
  return baseName;
}

/** Knowledge list names omit the extension; editing must use the stored source name. */
export function resolveKnowledgeSpreadsheetFileName(item: KnowledgeDownloadItem): string | null {
  if (item.type !== 'file') return null;
  const fileName = resolveKnowledgeDownloadFileName(item);
  if (isSpreadsheetArtifact(fileName)) return fileName;
  const ext = item.file_type?.toLowerCase().replace(/^\./, '');
  return ext && isSpreadsheetArtifact(`document.${ext}`) ? `${fileName}.${ext}` : null;
}

export function isBatchDownloadableKnowledge(item?: { type?: string; file_path?: string } | null): boolean {
  if (!item) return false;
  if (item.type === 'manual') return true;
  return Boolean(item.file_path);
}

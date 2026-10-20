export const KNOWLEDGE_FILE_UPDATED_EVENT = 'weknora:knowledge-file-updated'

export function notifyKnowledgeFileUpdated(knowledgeId: string) {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(KNOWLEDGE_FILE_UPDATED_EVENT, { detail: { knowledgeId } }))
  }
}

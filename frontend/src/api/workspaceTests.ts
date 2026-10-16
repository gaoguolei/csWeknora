import { get, post } from '@/utils/request'

export interface TestTarget { id: string; project_id: string; kind: 'tests' | 'build' | 'types' | 'lint' | 'connectivity' | 'custom' | 'browser'; name: string; file?: string; scope?: string }
export interface TestProject { id: string; name: string; path: string; language: string; framework: string; manager: string; targets: TestTarget[] }
export interface TestDiscovery { status: string; projects: TestProject[]; warnings: string[]; version?: Record<string, string> }
export interface TestCaseResult { name: string; file?: string; status: string; duration_ms?: number; detail?: string; selector?: string }
export interface TestCheck extends TestTarget { project_name: string; project_path: string; framework: string; status: string; command?: string; exit_code?: number; duration_ms?: number; detail?: string; cases: TestCaseResult[] }
export interface TestResult { status: string; outcome: string; checks: TestCheck[]; detail?: string; log?: string; duration_ms?: number; version?: Record<string, string>; project_versions?: Record<string, Record<string, string>>; version_changed: boolean; command?: string }
export interface TestRequest { project_id: string; target_id?: string; target_ids?: string[]; case_name?: string; case_file?: string; source?: 'manual' | 'ai' | 'automatic'; timeout?: number; agent_id?: string; agent_source_tenant_id?: number }
export interface TestRun { id: string; session_id: string; project_id: string; sandbox_id?: string; status: string; request: TestRequest; result: TestResult; report_message_id?: string; report_error?: string; created_at: string; updated_at: string }
const base = (session: string) => `/api/v1/sessions/${encodeURIComponent(session)}/programming`
export const workspaceTestsAPI = {
  projects: async (session: string): Promise<TestDiscovery> => (await get<{ data: TestDiscovery }>(`${base(session)}/test-projects`)).data,
  runs: async (session: string): Promise<TestRun[]> => (await get<{ data: TestRun[] }>(`${base(session)}/test-runs`)).data || [],
  start: async (session: string, request: TestRequest): Promise<TestRun> => (await post<{ data: TestRun }>(`${base(session)}/test-runs`, request)).data,
  cancel: async (session: string, id: string): Promise<TestRun> => (await post<{ data: TestRun }>(`${base(session)}/test-runs/${encodeURIComponent(id)}/cancel`, {}, { timeout: 90_000 })).data,
  browser: async (session: string, id: string, outcome: string, detail: string): Promise<TestRun> => (await post<{data:TestRun}>(`${base(session)}/test-runs/browser-check`,{run_id:id,outcome,detail})).data,
}
export type WorkspaceTestsGateway = typeof workspaceTestsAPI

import { del, get, put } from '@/utils/request'

/**
 * One platform-wide sandbox/skill secret as the super admin may see it.
 *
 * There is deliberately no value field: the endpoint reports whether a secret is
 * set and when it last changed, never what is stored. Values are write-only —
 * encrypted at rest and injected into every workspace's runs.
 */
export interface GlobalSandboxEnvView {
  name: string
  has_value: boolean
  updated_at?: string
}

/** Lists every global secret (name + set/unset + updated_at only). */
export function listGlobalSandboxEnv(): Promise<{ data: GlobalSandboxEnvView[] }> {
  return get('/api/v1/system/admin/sandbox-env') as unknown as Promise<{
    data: GlobalSandboxEnvView[]
  }>
}

/** Stores or rotates one global secret; the value is encrypted server-side. */
export function setGlobalSandboxEnv(
  name: string,
  value: string,
): Promise<{ success: boolean }> {
  return put('/api/v1/system/admin/sandbox-env', { name, value }) as unknown as Promise<{
    success: boolean
  }>
}

/** Removes one global secret. The name travels in a JSON body, matching the PUT. */
export function deleteGlobalSandboxEnv(name: string): Promise<{ success: boolean }> {
  return del('/api/v1/system/admin/sandbox-env', { name }) as unknown as Promise<{
    success: boolean
  }>
}

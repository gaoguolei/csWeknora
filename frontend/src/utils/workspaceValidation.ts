import type { TestRun } from '@/api/workspaceTests'

/** A partial run or dirty tree never certifies a Git commit. Latest run wins. */
export function commitValidationRuns(runs: TestRun[], repo: string): Record<string, TestRun> {
  const result: Record<string, TestRun> = {}
  const path = repo === '.' ? '' : repo.replace(/^\.\//, '').replace(/\/$/, '')
  for (const run of runs) {
    if (run.status !== 'completed' || run.result.version_changed || run.request.target_id || run.request.target_ids?.length || run.request.case_name || !run.result.checks.length) continue
    const versions = Object.values(run.result.project_versions || {})
    if (!versions.length && run.result.version) versions.push(run.result.version)
    for (const version of versions) {
      if (!version.head || version.dirty !== 'false' || version.complete === 'false' || (version.repo_path || '') !== path) continue
      if (!result[version.head]) result[version.head] = run
    }
  }
  return result
}

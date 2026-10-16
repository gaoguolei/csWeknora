import test from 'node:test'
import assert from 'node:assert/strict'
import { commitValidationRuns } from './workspaceValidation.ts'
import type { TestRun } from '../api/workspaceTests.ts'

const run = (id: string, outcome = 'passed'): TestRun => ({ id,session_id:'session',project_id:'project',status:'completed',request:{project_id:'project'},result:{status:'completed',outcome,version_changed:false,project_versions:{project:{head:'commit',dirty:'false',complete:'true',repo_path:'demo'}},checks:[{id:'tests',project_id:'project',kind:'tests',name:'Tests',project_name:'demo',project_path:'demo',framework:'node-test',status:outcome,cases:[]}]},created_at:'2026-10-13',updated_at:'2026-10-13' })
test('latest full verification of a clean commit wins, including a later failure',()=>{
  assert.equal(commitValidationRuns([run('new','failed'),run('old')],'demo').commit.id,'new')
  assert.deepEqual(commitValidationRuns([run('one')],'another-repo'),{})
})
test('dirty, partial and changed-source results do not certify a commit',()=>{
  const dirty=run('dirty');dirty.result.project_versions!.project.dirty='true'
  const partial=run('partial');partial.request.target_id='file'
  const changed=run('changed');changed.result.version_changed=true
  assert.deepEqual(commitValidationRuns([dirty,partial,changed],'demo'),{})
})

test('root repository identity accepts the Git dot path',()=>{
  const root = run('root'); root.result.project_versions!.project.repo_path = ''
  assert.equal(commitValidationRuns([root],'.').commit.id,'root')
})

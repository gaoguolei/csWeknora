import test from 'node:test'
import assert from 'node:assert/strict'
import { buildUnifiedDiff } from './unifiedDiff'

test('identical sides produce an empty diff so the view falls back to source', () => {
  const result = buildUnifiedDiff('a\nb\n', 'a\nb\n')
  assert.equal(result.empty, true)
  assert.equal(result.hunks.length, 0)
  assert.equal(result.added, 0)
  assert.equal(result.removed, 0)
})

test('a modified line yields one add and one del with old/new gutter numbers', () => {
  const before = 'one\ntwo\nthree\nfour\nfive\n'
  const after = 'one\ntwo\nTHREE\nfour\nfive\n'
  const result = buildUnifiedDiff(before, after)
  assert.equal(result.empty, false)
  const lines = result.hunks.flatMap((hunk) => hunk.lines)
  const added = lines.filter((line) => line.type === 'add')
  const removed = lines.filter((line) => line.type === 'del')
  assert.equal(added.length, 1)
  assert.equal(removed.length, 1)
  assert.equal(added[0].content, 'THREE')
  assert.equal(added[0].newNumber, 3)
  assert.equal(added[0].oldNumber, null)
  assert.equal(removed[0].content, 'three')
  assert.equal(removed[0].oldNumber, 3)
  assert.equal(removed[0].newNumber, null)
  // The hunk header carries the coordinate ranges an editor shows.
  assert.match(result.hunks[0].header, /^@@ -\d+,\d+ \+\d+,\d+ @@$/)
})

test('a brand-new file reads as one added block from an empty before side', () => {
  const result = buildUnifiedDiff('', 'a\nb\nc\n')
  assert.equal(result.empty, false)
  const lines = result.hunks.flatMap((hunk) => hunk.lines)
  assert.equal(lines.every((line) => line.type === 'add'), true)
  assert.equal(result.added, 3)
  assert.equal(result.removed, 0)
  assert.equal(lines[0].newNumber, 1)
  assert.equal(lines[2].newNumber, 3)
})

test('context lines are kept around a change and numbered on both sides', () => {
  const before = Array.from({ length: 20 }, (_, i) => `line ${i}`).join('\n')
  const after = before.replace('line 10', 'line ten')
  const result = buildUnifiedDiff(before, after)
  const lines = result.hunks.flatMap((hunk) => hunk.lines)
  const context = lines.filter((line) => line.type === 'context')
  assert.ok(context.length > 0)
  assert.ok(context.every((line) => line.oldNumber !== null && line.newNumber !== null))
  // Only a window of context is kept, not the whole file.
  assert.ok(context.length <= 6)
})

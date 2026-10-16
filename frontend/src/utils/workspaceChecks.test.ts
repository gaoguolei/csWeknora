import test from 'node:test'
import assert from 'node:assert/strict'
import { describeWorkspaceCheck, workspaceCheckKind, type WorkspaceCheckKind } from './workspaceChecks.ts'

const cases: Array<[WorkspaceCheckKind, string]> = [
  // Tests, the common shapes an agent actually runs.
  ['tests', 'npm test'],
  ['tests', 'npm run test'],
  ['tests', 'npm run test:unit'],
  ['tests', 'pnpm test'],
  ['tests', 'yarn run e2e'],
  ['tests', 'bun test'],
  ['tests', 'npx vitest run'],
  ['tests', 'pnpm exec playwright test'],
  ['tests', 'pnpm dlx jest --ci'],
  ['tests', 'uv run pytest -q'],
  ['tests', 'poetry run pytest'],
  ['tests', 'pytest'],
  ['tests', 'python -m pytest ./tests'],
  ['tests', 'python3 -m unittest discover'],
  ['tests', 'python test.py'],
  ['tests', 'node --test'],
  ['tests', 'node test/smoke.js'],
  ['tests', 'go test -v ./...'],
  ['tests', 'make test'],
  ['tests', 'make -j4 test'],
  ['tests', 'bash run_tests.sh'],
  ['tests', './run_tests.sh'],
  ['tests', 'sh ./scripts/test.sh'],
  ['tests', './gradlew test'],
  ['tests', 'mvn -q test'],
  ['tests', 'cargo test'],
  ['tests', 'dotnet test'],
  ['tests', 'deno test'],
  ['tests', 'uvx pytest'],
  ['tests', 'sudo npm test'],
  ['tests', 'timeout 60 go test ./...'],
  ['tests', 'cd frontend && npm run test'],
  // Build.
  ['build', 'npm run build'],
  ['build', 'npm run build:prod'],
  ['build', 'vite build'],
  ['build', 'go build ./...'],
  ['build', 'cargo build'],
  ['build', 'make build'],
  // Types.
  ['types', 'npm run type-check'],
  ['types', 'npm run check-types'],
  ['types', 'vue-tsc --noEmit'],
  ['types', 'mypy src'],
  ['types', 'deno check main.ts'],
  // Lint.
  ['lint', 'npm run lint'],
  ['lint', 'npm run lint:fix'],
  ['lint', 'eslint .'],
  ['lint', 'ruff check .'],
  ['lint', 'golangci-lint run'],
  ['lint', 'go vet ./...'],
  ['lint', 'make lint'],
  // Unrelated work must never inflate the check counts.
  ['other', 'npm run dev'],
  ['other', 'npm run preview'],
  ['other', 'cat README.md'],
  ['other', 'echo hello'],
  ['other', 'ls -la'],
  ['other', 'git status'],
  ['other', 'python scripts/seed.py'],
  ['other', 'npm install'],
]

test('workspaceCheckKind classifies the commands an agent runs', () => {
  for (const [kind, command] of cases) {
    assert.equal(workspaceCheckKind(command), kind, command)
  }
})

test('workspaceCheckKind never classifies from output words', () => {
  // Only the command decides the kind; "passed"/"pytest" in the echoed output must not matter.
  assert.equal(workspaceCheckKind('echo "10 tests passed"'), 'other')
  assert.equal(workspaceCheckKind('cat pytest.ini'), 'other')
})

test('describeWorkspaceCheck carries the kind, log, and evidence for a real run', () => {
  const check = describeWorkspaceCheck({
    id: 'm1',
    command: 'make test',
    output: '\u001b[32mok  \tgithub.com/example/pkg\t0.012s\u001b[0m\nFAIL\tgithub.com/example/pkg\t0.200s',
    status: 'failed',
    durationMs: 1200,
    exitCode: 1,
  })
  assert.equal(check.kind, 'tests')
  assert.equal(check.log.includes('\u001b['), false)
  assert.ok(check.highlights.some(line => /FAIL/.test(line)))
})

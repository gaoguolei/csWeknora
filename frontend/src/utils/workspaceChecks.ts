import type { WorkspaceCommandRun } from './workspaceEvents'

export type WorkspaceCheckKind = 'tests' | 'build' | 'types' | 'lint' | 'other'
export type WorkspaceCheck = WorkspaceCommandRun & {
  kind: WorkspaceCheckKind
  highlights: string[]
  log: string
  logTruncated: boolean
}

type KnownKind = Exclude<WorkspaceCheckKind, 'other'>

const PACKAGE_MANAGERS = new Set(['npm', 'pnpm', 'yarn', 'bun'])
const MAKE_TOOLS = new Set(['make', 'gmake', 'just'])
const SHELLS = new Set(['bash', 'sh', 'zsh', 'dash', 'ksh', 'fish', 'pwsh', 'powershell'])
// Tools that merely launch another program: unwrap them and classify the target.
const LAUNCH_WRAPPERS = new Set(['npx', 'bunx', 'pnpx', 'uvx', 'uv', 'poetry', 'pipenv', 'pdm', 'rye', 'hatch', 'pipx'])
const PASSTHROUGH_WRAPPERS = new Set(['sudo', 'env', 'command', 'time', 'nohup', 'stdbuf', 'exec'])
const ENV_PREFIX = /^(?:(?:[A-Za-z_][A-Za-z_\d]*)=(?:"[^"]*"|'[^']*'|\S+)\s+)+/
const SCRIPT_EXTENSION = /\.(?:exe|cmd|bat|ps1)$/i

// Script/target names: test, test:unit, run-e2e, coverage, build, type-check…
const SCRIPT_TEST = /(?:^|[-_:.])(?:test|tests|e2e|spec|specs|coverage)(?:$|[-_:.])/
const SCRIPT_BUILD = /(?:^|[-_:.])(?:build|bundle|compile)(?:$|[-_:.])/
const SCRIPT_TYPES = /(?:^|[-_:.])(?:type-?check|typecheck|check-?types|types)(?:$|[-_:.])/
const SCRIPT_LINT = /(?:^|[-_:.])(?:lint|eslint|stylelint)(?:$|[-_:.])/
// File names: run_tests.sh, test.py, foo.test.ts, bar.spec.js…
const TEST_FILE = /(?:^|[-_.])(?:test|tests|spec|specs)(?:$|[-_.])|\.(?:test|spec)\./i

/** Executable name with any directory and a Windows extension removed. */
function fileName(token: string): string {
  return (token.replace(/\\/g, '/').split('/').pop() || token).replace(SCRIPT_EXTENSION, '')
}

function isTestScript(token: string): boolean {
  const segments = token.replace(/\\/g, '/').split('/').filter(Boolean)
  if (segments.some(segment => /^(?:tests?|specs?)$/i.test(segment))) return true
  return TEST_FILE.test(fileName(token).replace(/^\.+/, ''))
}

/** Classify an npm/pnpm/yarn `run <script>` name or a `make` target. */
function scriptKind(raw: string): KnownKind | null {
  const name = fileName(raw).toLowerCase().replace(/^\.+/, '')
  if (!name || name === 'run' || name === 'exec') return null
  if (SCRIPT_TEST.test(name)) return 'tests'
  if (SCRIPT_BUILD.test(name)) return 'build'
  if (SCRIPT_TYPES.test(name) || name === 'tsc' || name === 'vue-tsc') return 'types'
  if (SCRIPT_LINT.test(name)) return 'lint'
  return null
}

/** Classify a bare executable invocation, e.g. `go test`, `vitest run`, `node --test`. */
function directKind(tokens: string[]): KnownKind | null {
  // Drop leading NAME=value assignments a wrapper may expose (env FOO=1 npm test).
  while (tokens.length && /^[A-Za-z_][A-Za-z_\d]*=(?:"[^"]*"|'[^']*'|\S+)$/.test(tokens[0])) tokens = tokens.slice(1)
  if (!tokens.length) return null
  const rawHead = fileName(tokens[0]).toLowerCase()
  const bin = rawHead.replace(/[\d.]+$/, '')
  const args = tokens.slice(1)
  const arg1 = (args[0] || '').toLowerCase()

  if (LAUNCH_WRAPPERS.has(bin)) {
    const index = args.findIndex(token => token !== 'run' && token !== 'exec' && !token.startsWith('-'))
    return index >= 0 ? directKind(args.slice(index)) : null
  }
  if (PASSTHROUGH_WRAPPERS.has(bin)) return directKind(args)
  if (bin === 'timeout') {
    const index = args.findIndex(token => !/^\d+(?:\.\d+)?[smhd]?$/.test(token) && !token.startsWith('-'))
    return index >= 0 ? directKind(args.slice(index)) : null
  }
  if (PACKAGE_MANAGERS.has(bin)) return packageManagerKind(tokens)
  if (MAKE_TOOLS.has(bin)) return makeTargetKind(args)

  switch (bin) {
    // Shell conditions inspect files/values; they do not run project tests.
    case 'test':
    case '[':
    case '[[':
      return null
    case 'go':
      if (arg1 === 'test') return 'tests'
      if (arg1 === 'vet') return 'lint'
      if (arg1 === 'build') return 'build'
      return null
    case 'cargo':
    case 'dotnet':
      if (arg1 === 'test') return 'tests'
      if (arg1 === 'build') return 'build'
      return null
    case 'deno':
      if (arg1 === 'test') return 'tests'
      if (arg1 === 'lint') return 'lint'
      if (arg1 === 'check') return 'types'
      return null
    case 'node':
    case 'tsx':
    case 'ts-node':
      if (args.some(token => /^--test(?:=|$)/.test(token)) || arg1 === '-t') return 'tests'
      if (isTestScript(args[0] || '')) return 'tests'
      return null
    case 'vite':
    case 'next':
    case 'nuxt':
      if (arg1 === 'build') return 'build'
      return null
    case 'tsc':
    case 'vue-tsc':
    case 'mypy':
    case 'pyright':
    case 'flow':
      return 'types'
    case 'eslint':
    case 'stylelint':
    case 'flake8':
    case 'pylint':
    case 'golangci-lint':
      return 'lint'
    case 'ruff':
      return arg1 === 'check' ? 'lint' : null
    case 'pytest':
    case 'vitest':
    case 'jest':
    case 'mocha':
    case 'ava':
    case 'tap':
    case 'rspec':
    case 'phpunit':
    case 'nose2':
    case 'cypress':
      return 'tests'
    case 'playwright':
      return arg1 === 'test' ? 'tests' : null
  }

  if (bin === 'gradle' || bin === 'gradlew' || bin === 'mvn' || bin === 'mvnw') {
    return args.some(token => token.toLowerCase() === 'test' || /(?:^|:)test$/i.test(token)) ? 'tests' : null
  }
  if (bin === 'python' || bin === 'pythonw') {
    if (/\s-m\s+(?:pytest|unittest)\b/.test(tokens.join(' '))) return 'tests'
    return args.some(token => isTestScript(token)) ? 'tests' : null
  }
  if (SHELLS.has(rawHead)) {
    return args.some(token => isTestScript(token)) ? 'tests' : null
  }
  // A bare executable script that names itself a test, e.g. ./run_tests.sh.
  return isTestScript(rawHead) ? 'tests' : null
}

function makeTargetKind(targets: string[]): KnownKind | null {
  return targets.reduce<KnownKind | null>(
    (found, target) => found || (target.startsWith('-') || target.includes('=') ? null : scriptKind(target)),
    null,
  )
}

function packageManagerKind(tokens: string[]): KnownKind | null {
  const sub = (tokens[1] || '').toLowerCase()
  if (sub === 'exec' || sub === 'dlx' || sub === 'x') return directKind(tokens.slice(2))
  const script = tokens.slice(1).find(token => token !== 'run' && !token.startsWith('-'))
  return script ? scriptKind(script) : null
}

// Classify the executed command, never words found in its output. Unknown
// commands remain available as other operations and do not count as tests.
export function workspaceCheckKind(command: string): WorkspaceCheckKind {
  for (const raw of command.split(/&&|\|\||[;\n]/)) {
    let segment = raw.trim().replace(/^[({[]\s*/, '').replace(/[\s)}\]]+$/, '')
    segment = segment.replace(ENV_PREFIX, '')
    if (!segment) continue
    const tokens = segment.split(/\s+/)
    const head = fileName(tokens[0]).toLowerCase()
    const kind = PACKAGE_MANAGERS.has(head)
      ? packageManagerKind(tokens)
      : MAKE_TOOLS.has(head)
        ? makeTargetKind(tokens.slice(1))
        : directKind(tokens)
    if (kind) return kind
  }
  return 'other'
}

export function describeWorkspaceCheck(run: WorkspaceCommandRun): WorkspaceCheck {
  const log = run.output.replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, '').replace(/\r(?!\n)/g, '\n')
  const tail = log.slice(-100_000)
  const lines = tail.split('\n').map(line => line.trim()).filter(Boolean)
  const summaries = lines.filter(line => /^(?:Tests?:|Test Suites:|Test Files\s|Tests\s|# (?:tests|pass|fail)|=+.*\d+ (?:passed|failed)|\d+ (?:passing|failing|passed|failed)|ok\s+\S+|test result:|✓.*built in|Successfully compiled)/i.test(line))
  const failures = lines.filter(line => /(?:\b(?:AssertionError|TypeError|ReferenceError|SyntaxError|Error:|error TS\d+|FAIL(?:ED)?\b|Expected:|Received:)|^[×✕❌])/.test(line))
  // Excerpts are evidence, not inferred test-case counts or invented explanations.
  const highlights = [...new Set(run.status === 'failed' ? [...failures, ...summaries] : summaries)].slice(0, 3).map(line => line.slice(0, 400))
  return { ...run, kind: workspaceCheckKind(run.command), highlights, log: tail, logTruncated: log.length > tail.length }
}

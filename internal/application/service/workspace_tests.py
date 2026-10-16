"""Session-local project discovery and detached validation runs. Python stdlib only."""
import hashlib
import html
import http.client
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import signal
import subprocess
import sys
import time
import threading
from urllib.parse import urlsplit
import xml.etree.ElementTree as ET

SKIP = {'.git', 'node_modules', 'vendor', 'dist', 'build', 'coverage', '.next', '.nuxt', '.venv', 'venv', 'target', '.tox', '__pycache__', 'input', 'output'}
LIMIT = 100000

NODE_REPORTER = r'''import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { inspect } from 'node:util';
export default async function* reporter(source) {
  const cases = [];
  for await (const event of source) {
    const data = event.data || {};
    if (event.type === 'test:stdout' || event.type === 'test:stderr') yield data.message || '';
    if (event.type !== 'test:pass' && event.type !== 'test:fail') continue;
    yield event.type + ' ' + data.name + '\n';
    // Node generates a successful file wrapper even for a file declaring no
    // tests. A suite or that wrapper is not an assertion case.
    if (data.details?.type === 'suite' || (data.file && resolve(data.name) === resolve(data.file))) continue;
    cases.push({ name: data.name, file: data.file || '',
      selector: data.nesting === 0 ? data.name : '',
      status: data.skip || data.todo ? 'skipped' : event.type === 'test:pass' ? 'passed' : 'failed',
      duration_ms: Math.round(data.details?.duration_ms || 0),
      detail: data.details?.error ? inspect(data.details.error, {depth: 4}).slice(0, 4000) : '' });
  }
  writeFileSync(process.env.WEKNORA_NODE_REPORT, JSON.stringify({cases}));
}
'''


def load(path, default=None):
    try:
        if path.stat().st_size > 1024 * 1024:
            return default
        return json.loads(path.read_text())
    except (OSError, ValueError):
        return default


def save(path, value):
    tmp = path.with_suffix('.tmp')
    tmp.write_text(json.dumps(value, ensure_ascii=False))
    tmp.replace(path)


def inside(root, relative):
    path = (root / relative).resolve()
    if path != root and root not in path.parents:
        raise ValueError('Path is outside the current workspace')
    return path


def project_id(relative):
    return hashlib.sha256(relative.encode()).hexdigest()[:16]


def discover(root):
    root = root.resolve()
    projects, warnings = [], []
    folders = []
    for folder, dirs, files in os.walk(root, followlinks=False):
        directory = Path(folder)
        dirs[:] = sorted(d for d in dirs if d not in SKIP and not d.startswith('.') and not (directory / d).is_symlink())
        folders.append((directory, files))
        if len(folders) >= 500:
            warnings.append('Project discovery reached the 500-directory limit; results may be incomplete.')
            break
        if len(directory.relative_to(root).parts) >= 6:
            if dirs:
                warnings.append('Project discovery reached its depth limit; results may be incomplete.')
            dirs[:] = []
    for directory, files in folders:
        package = load(directory / 'package.json') if 'package.json' in files else None
        language = 'node' if isinstance(package, dict) else 'go' if 'go.mod' in files else 'python' if any(n in files for n in ('pyproject.toml', 'pytest.ini', 'setup.cfg')) else ''
        if not language:
            continue
        relative = directory.relative_to(root).as_posix()
        relative = '' if relative == '.' else relative
        pid = project_id(relative)
        scripts = package.get('scripts', {}) if package else {}
        if not isinstance(scripts, dict):
            scripts = {}
        deps = {**package.get('dependencies', {}), **package.get('devDependencies', {})} if package else {}
        test_script = scripts.get('test', '')
        framework = 'pytest' if language == 'python' else 'go' if language == 'go' else 'vitest' if 'vitest' in test_script else 'jest' if re.search(r'\bjest\b', test_script) else 'playwright' if 'playwright test' in test_script else 'node-test' if '--test' in test_script else 'custom'
        if language == 'node' and not test_script:
            framework = next((f for f in ('vitest', 'jest') if f in deps), 'custom')
        manager = str(package.get('packageManager', '')).split('@')[0] if package else ''
        if manager not in ('npm', 'pnpm', 'yarn', 'bun'):
            manager = 'pnpm' if (directory / 'pnpm-lock.yaml').exists() else 'yarn' if (directory / 'yarn.lock').exists() else 'bun' if any((directory / n).exists() for n in ('bun.lock', 'bun.lockb')) else 'npm'
        targets, go_packages = [], set()
        def add(kind, name, file=''):
            targets.append({'id': f'{pid}:{kind}:{file or name}', 'kind': kind, 'name': name, 'file': file, 'project_id': pid})
        if language in ('python', 'go') or test_script or framework != 'custom':
            add('tests', 'Tests')
        for kind, names in [('build', ('build',)), ('types', ('type-check', 'typecheck', 'check-types')), ('lint', ('lint',))]:
            script = next((name for name in names if name in scripts), '')
            if script:
                add(kind, script)
        if language == 'go':
            add('build', 'Build')
        for subdir, subfiles in folders:
            if directory != subdir and directory not in subdir.parents:
                continue
            if directory != subdir and any(marker in subfiles for marker in ('package.json', 'go.mod', 'pyproject.toml')):
                continue
            # A nested manifest is another project's execution root.
            if any((ancestor / marker).is_file() for ancestor in list(subdir.parents)[:max(0, len(subdir.relative_to(directory).parts) - 1)] for marker in ('package.json', 'go.mod', 'pyproject.toml')):
                continue
            for name in subfiles:
                file = subdir / name
                if file.is_symlink():
                    continue
                rel = file.relative_to(directory).as_posix()
                is_test = bool(re.search(r'(?:^test_.*\.py$|.*_test\.go$|.*\.(?:test|spec)\.[cm]?[jt]sx?$|test.*\.[cm]?[jt]sx?$)', name)) or ('__tests__' in file.parts and file.suffix in ('.js', '.ts', '.tsx', '.jsx', '.mjs', '.cjs'))
                if is_test and len(targets) < 300 and framework != 'custom':
                    if language == 'go':
                        package = Path(rel).parent.as_posix()
                        if package not in go_packages:
                            go_packages.add(package); add('tests', package, package); targets[-1]['scope'] = 'package'
                    else:
                        add('tests', name, rel)
        config = load(directory / '.weknora/validation.json', {}) or {}
        if config.get('services'):
            add('connectivity', 'Service connectivity')
        for index, check in enumerate(config.get('checks', [])[:30]):
            kind = check.get('kind', 'custom')
            if kind not in ('tests', 'build', 'types', 'lint', 'custom') or not isinstance(check.get('command'), str) or len(check['command']) > 8000:
                continue
            target = {'id':f'{pid}:configured:{index}', 'project_id':pid, 'kind':kind, 'name':str(check.get('name', 'Custom validation'))[:200], 'file':'', 'command':check['command']}
            targets.append(target)
        projects.append({'id': pid, 'name': (package or {}).get('name') or directory.name, 'path': relative, 'language': language, 'framework': framework, 'manager': manager, 'scripts': scripts, 'targets': targets, 'services': config.get('services', []), 'test_script': test_script, 'has_dependencies': bool(deps)})
    return {'projects': projects, 'warnings': list(dict.fromkeys(warnings)), 'version': version(root)}


def version(root, workspace=None):
    workspace = workspace or root
    def git(*args):
        try:
            p = subprocess.run(['git', '-C', str(root), *args], capture_output=True, timeout=8)
            return p.stdout if p.returncode == 0 else b''
        except (OSError, subprocess.TimeoutExpired):
            return b''
    top = git('rev-parse','--show-toplevel').decode().strip()
    if top:
        repo = Path(top).resolve()
        if repo != root and (repo == workspace or workspace in repo.parents):
            return version(repo,workspace)
    head = git('rev-parse', 'HEAD').decode().strip()
    digest = hashlib.sha256(git('status', '--porcelain', '--untracked-files=no'))
    changed = git('diff', 'HEAD', '--name-only', '--relative', '-z') + git('ls-files', '--others', '--exclude-standard', '-z')
    for raw in changed.split(b'\0')[:500]:
        if raw:
            if b'.weknora/test-runs/' in raw or raw.startswith((b'input/', b'output/')):
                continue
            try:
                p = inside(root, os.fsdecode(raw))
                if not p.is_symlink() and p.is_file() and p.stat().st_size <= 1024 * 1024:
                    digest.update(raw); digest.update(p.read_bytes())
            except (OSError, ValueError):
                pass
    count, total, complete, directory_count = 0, 0, True, 0
    if not head:
        for folder, dirs, files in os.walk(root, followlinks=False):
            directory_count += 1
            if directory_count > 500:
                complete = False; break
            dirs[:] = sorted(d for d in dirs if d not in SKIP and not d.startswith('.') and not (Path(folder) / d).is_symlink())
            for name in sorted(files):
                count += 1
                file = Path(folder) / name
                if file.is_symlink():
                    continue
                try:
                    if file.stat().st_size <= 1024 * 1024:
                        data = file.read_bytes(); total += len(data)
                        digest.update(str(file.relative_to(root)).encode()); digest.update(data)
                except OSError:
                    pass
                if count >= 1000 or total >= 8 * 1024 * 1024:
                    complete = False; break
            if not complete:
                break
    dirty = bool(git('status','--porcelain')) if head else True
    relative = root.relative_to(workspace).as_posix() if root == workspace or workspace in root.parents else ''
    return {'head': head, 'fingerprint': digest.hexdigest(), 'complete': str(complete).lower(), 'dirty':str(dirty).lower(), 'repo_path':'' if relative == '.' else relative}


def state_dir(root, rid, host=False):
    if not re.fullmatch(r'[a-f0-9-]{36}', rid):
        raise ValueError('Invalid test run ID')
    cache = root / '.weknora/test-runs' if host else Path.home() / '.cache/weknora-tests' / project_id(str(root))
    base = cache / rid
    base.mkdir(parents=True, exist_ok=True)
    return base


def read_log(path):
    try:
        with path.open('rb') as f:
            f.seek(0, 2); size = f.tell(); f.seek(max(0, size - LIMIT))
            return f.read().decode('utf-8', 'replace')
    except OSError:
        return ''


def execute(command, cwd, run_dir, state, log, timeout, env=None):
    if (run_dir / 'cancel').exists():
        return None, 'cancelled', ''
    state['command'] = shlex.join(command)
    save(run_dir / 'state.json', state)
    started = time.monotonic()
    try:
        process = subprocess.Popen(command, cwd=cwd, stdout=log, stderr=log, stdin=subprocess.DEVNULL, start_new_session=True, env=env or {**os.environ, 'CI': 'true', 'BROWSER': 'none'})
    except FileNotFoundError:
        return None, 'blocked', 'Required runtime or package manager is not installed.'
    except OSError as e:
        return None, 'error', str(e)
    reason = ''
    while process.poll() is None:
        if (run_dir / 'cancel').exists():
            reason = 'cancelled'
        elif time.monotonic() - started > timeout:
            reason = 'timed_out'
        elif (run_dir / 'output.log').stat().st_size > 8 * 1024 * 1024:
            reason = 'error'
        if reason:
            break
        time.sleep(.2)
    # Stop this task's descendants, never the sandbox or existing project servers.
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait()
    # A child may ignore SIGTERM after its leader exits. Confirm cleanup of
    # this task's group before declaring cancellation complete.
    try:
        os.killpg(process.pid, 0)
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    return process.returncode, reason, 'Output exceeded the 8 MiB limit.' if reason == 'error' else ''


def test_command(project, target, directory, report):
    framework, file = project['framework'], target.get('file', '')
    selector = '^' + re.escape(target['case_name']) + '$' if target.get('case_name') else ''
    binary = directory / 'node_modules/.bin'
    if framework == 'node-test':
        script = project.get('test_script', '')
        # Keep existing loaders/Node options; replace only the test file selection.
        tokens = shlex.split(script)
        if not tokens or tokens[0] not in ('node', 'tsx') or any(t in ('&&', ';', '|', '||') for t in tokens):
            return [project['manager'], 'run', 'test'], 'custom'
        executable = str(binary / 'tsx') if tokens[0] == 'tsx' else 'node'
        options, files = [], []
        args = iter(tokens[1:])
        for token in args:
            if token.startswith('--test-reporter'):
                if '=' not in token: next(args, None)
            elif token.startswith('-'):
                options.append(token)
                if token in ('--import', '--require', '-r', '--loader', '--conditions', '--test-name-pattern', '--test-concurrency', '--test-timeout'):
                    value = next(args, None)
                    if value is not None: options.append(value)
            else:
                files.append(token)
        if file: files = [file]
        if selector: options += ['--test-name-pattern=' + selector]
        # Expand shell globs ourselves: argv execution deliberately uses no shell.
        expanded = []
        for pattern in files:
            matches = sorted(directory.glob(pattern)) if any(c in pattern for c in '*?[') else []
            expanded.extend(str(p.relative_to(directory)) for p in matches if p.is_file()) if matches else expanded.append(pattern)
        reporter = report.parent / 'node-reporter.mjs'
        reporter.write_text(NODE_REPORTER)
        return [executable, *options, '--test-reporter=' + str(reporter), *expanded], 'node-json'
    if framework == 'vitest':
        return [str(binary / 'vitest'), 'run', '--reporter=json', '--outputFile=' + str(report), *([file] if file else []), *(['-t',selector] if selector else [])], 'json'
    if framework == 'jest':
        return [str(binary / 'jest'), '--runInBand', '--json', '--outputFile=' + str(report), *(['--runTestsByPath', file] if file else []), *(['--testNamePattern=' + selector] if selector else [])], 'json'
    if framework == 'playwright':
        return [str(binary / 'playwright'), 'test', '--reporter=json', *([file] if file else [])], 'playwright'
    if framework == 'pytest':
        return [sys.executable, '-m', 'pytest', '--junitxml=' + str(report), *([file] if file else [])], 'junit'
    if framework == 'go':
        package = './' + (file if target.get('scope') == 'package' else str(Path(file).parent)) if file else './...'
        return ['go', 'test', '-json', package], 'go'
    return [project['manager'], 'run', 'test'], 'custom'


def normalize_case_durations(cases):
    # Native runners such as Vitest report fractional milliseconds. The API
    # stores integer milliseconds; apply the same conversion to old snapshots
    # so a run already in progress can still be polled or cancelled after an
    # upgrade. Invalid timing metadata must not discard assertion evidence.
    for case in cases:
        try:
            case['duration_ms'] = max(0, int(case.get('duration_ms') or 0))
        except (TypeError, ValueError, OverflowError):
            case['duration_ms'] = 0
    return cases


def parse_results(format, report, output):
    cases = []
    if format == 'node-json':
        cases = (load(report, {}) or {}).get('cases', [])
    elif format == 'json':
        data = load(report, {}) or {}
        for suite in data.get('testResults', []):
            for case in suite.get('assertionResults', []):
                status = case.get('status', 'unknown')
                status = 'skipped' if status in ('pending', 'todo', 'disabled') else status
                cases.append({'name': case.get('fullName') or case.get('title', ''), 'selector':case.get('fullName') or case.get('title',''), 'file': suite.get('name', ''), 'status': status, 'duration_ms': case.get('duration') or 0, 'detail': '\n'.join(case.get('failureMessages', []))[:4000]})
    elif format == 'junit':
        try:
            for case in ET.parse(report).iter('testcase'):
                failure = case.find('failure')
                error = case.find('error')
                cases.append({'name': case.get('classname', '') + '::' + case.get('name', ''), 'file': case.get('file', ''), 'status': 'failed' if failure is not None or error is not None else 'skipped' if case.find('skipped') is not None else 'passed', 'duration_ms': int(float(case.get('time', '0')) * 1000), 'detail': ((failure.text or '') if failure is not None else (error.text or '') if error is not None else '')[:4000]})
        except (OSError, ET.ParseError, ValueError):
            pass
    elif format == 'go':
        for line in output.splitlines():
            try:
                event = json.loads(line)
                if event.get('Test') and event.get('Action') in ('pass', 'fail', 'skip'):
                    cases.append({'name': event['Test'], 'file': event.get('Package', ''), 'status': {'pass': 'passed', 'fail': 'failed', 'skip': 'skipped'}[event['Action']], 'duration_ms': int(event.get('Elapsed', 0) * 1000)})
            except (ValueError, TypeError):
                pass
    elif format == 'tap':
        # Only TAP result records count; arbitrary prose containing "passed" does not.
        for line in output.splitlines():
            match = re.match(r'^(\s*)(not ok|ok)\s+\d+\s+(?:-\s*)?(.+)', line)
            if match:
                name = match[3]
                cases.append({'name': name.split(' # ')[0], 'status': 'skipped' if re.search(r'#\s*(?:SKIP|TODO)', name, re.I) else 'failed' if match[2] == 'not ok' else 'passed'})
    elif format == 'playwright':
        data = load(report, {}) or {}
        def walk(suite):
            for spec in suite.get('specs', []):
                for case in spec.get('tests', []):
                    results = case.get('results', [])
                    result = results[-1] if results else {}
                    cases.append({'name': spec.get('title', ''), 'file': spec.get('file', ''), 'status': 'passed' if case.get('status') == 'expected' else 'skipped' if case.get('status') == 'skipped' else 'failed', 'duration_ms': result.get('duration', 0), 'detail': '\n'.join(e.get('message', '') for e in result.get('errors', []))[:4000]})
            for child in suite.get('suites', []):
                walk(child)
        for suite in data.get('suites', []):
            walk(suite)
    return normalize_case_durations(cases[:1000])


def connectivity(services):
    results = []
    for service in services[:16]:
        url = urlsplit(str(service.get('url', '')))
        if url.scheme != 'http' or url.hostname not in ('localhost', '127.0.0.1') or not url.port or url.port < 1024 or url.port in (49983, 5900, 6080, 9222) or url.username or url.password:
            results.append({'name': service.get('name', 'Service'), 'status': 'blocked', 'detail': 'Service checks require a sandbox-local HTTP URL on a project port.'})
            continue
        connection = http.client.HTTPConnection(url.hostname, url.port, timeout=5)
        try:
            connection.request('GET', url.path or '/')
            response = connection.getresponse(); response.read(1024)
            expected = service.get('status', 200)
            results.append({'name': service.get('name') or url.geturl(), 'status': 'passed' if response.status == expected else 'failed', 'detail': f'HTTP {response.status}; expected {expected}'})
        except (OSError, http.client.HTTPException) as e:
            results.append({'name': service.get('name') or url.geturl(), 'status': 'blocked', 'detail': str(e)})
        finally:
            connection.close()
    return results


def worker(options):
    root = Path(options['root']).resolve()
    run_dir = state_dir(root, options['run_id'], options.get('host', False))
    state = {'status': 'preparing', 'outcome': 'unknown', 'checks': [], 'started_at': time.time(), 'version': version(root), 'command': ''}
    save(run_dir / 'state.json', state)
    deadline = time.monotonic() + min(max(int(options.get('timeout', 300)), 10), 900)
    with (run_dir / 'output.log').open('w') as log:
        try:
            projects = discover(root)['projects']
            selected = [p for p in projects if not options.get('project_id') or p['id'] == options['project_id']]
            if not selected:
                raise ValueError('No runnable project was found. Configure a test or build script first.')
            state['project_versions'] = {p['id']:version(inside(root,p['path']),root) for p in selected}
            matched = False
            for project in selected:
                directory = inside(root, project['path'])
                targets = [t for t in project['targets'] if (t['id'] == options['target_id'] if options.get('target_id') else not t['file'])]
                if options.get('target_ids'):
                    targets = [t for t in project['targets'] if t['id'] in options['target_ids']]
                if options.get('files'):
                    targets = [t for t in project['targets'] if t.get('file') in options['files']]
                if options.get('case_name'):
                    if project['framework'] not in ('node-test','vitest','jest') or len(targets) != 1 or targets[0]['kind'] != 'tests':
                        raise ValueError('This runner cannot reliably select that case; run its file or suite instead.')
                    chosen = inside(directory, options.get('case_file',''))
                    relative = chosen.relative_to(directory).as_posix()
                    if relative not in {t.get('file') for t in project['targets']}:
                        raise ValueError('The case file is no longer a discovered test target.')
                    target = dict(targets[0])
                    target.update(file=relative, case_name=options['case_name'], id=target['id']+':case:'+hashlib.sha256(options['case_name'].encode()).hexdigest()[:12], name=options['case_name'])
                    targets = [target]
                if not targets:
                    if not options.get('target_id') and not options.get('target_ids') and not options.get('files'):
                        matched = True
                        state['checks'].append({'id':project['id']+':unconfigured', 'project_id':project['id'], 'project_name':project['name'], 'project_path':project['path'], 'kind':'tests', 'name':'Validation configuration', 'status':'blocked', 'cases':[], 'detail':'No test or validation script is configured for this project.'})
                    continue
                matched = True
                needs_install = project['language'] == 'node' and project['has_dependencies'] and not (directory / 'node_modules').is_dir()
                if needs_install:
                    manager = project['manager']
                    install = [manager, 'ci' if manager == 'npm' and (directory / 'package-lock.json').exists() else 'install']
                    if manager == 'npm':
                        install += ['--no-audit', '--no-fund']
                    code, reason, detail = execute(install, directory, run_dir, state, log, max(1, deadline - time.monotonic()))
                    if reason or code != 0:
                        state.update(status=reason if reason in ('cancelled', 'timed_out') else 'completed', outcome='blocked', detail=detail or 'Dependency preparation failed. Check the installation output.')
                        break
                state['status'] = 'running'
                for target in targets:
                    if (run_dir / 'cancel').exists():
                        state.update(status='cancelled', outcome='unknown'); break
                    check = {**target, 'project_name': project['name'], 'project_path': project['path'], 'framework': project['framework'], 'status': 'running', 'cases': []}
                    state['checks'].append(check); save(run_dir / 'state.json', state)
                    start = time.monotonic()
                    report = run_dir / ('native-' + hashlib.sha256(target['id'].encode()).hexdigest()[:12] + ('.xml' if project['framework'] == 'pytest' else '.json'))
                    offset = (run_dir / 'output.log').stat().st_size
                    if target['kind'] == 'connectivity':
                        check['cases'] = connectivity(project['services'])
                        check['exit_code'] = 1 if any(c['status'] != 'passed' for c in check['cases']) else 0
                        reason, detail, format = '', '', 'connectivity'
                    else:
                        if target.get('command'):
                            command, format = ['sh', '-c', target['command']], 'custom'
                        elif target['kind'] == 'tests':
                            command, format = test_command(project, target, directory, report)
                        elif project['language'] == 'go':
                            command, format = ['go', 'build', './...'], ''
                        else:
                            command, format = [project['manager'], 'run', target['name']], ''
                        env = {**os.environ, 'CI': 'true', 'BROWSER': 'none', 'PLAYWRIGHT_JSON_OUTPUT_NAME': str(report), 'WEKNORA_NODE_REPORT': str(report)}
                        code, reason, detail = execute(command, directory, run_dir, state, log, max(1, deadline - time.monotonic()), env)
                        check.update(command=shlex.join(command), exit_code=code)
                        if not reason:
                            # Parse this check's bounded output, not previous commands.
                            with (run_dir / 'output.log').open('rb') as f:
                                f.seek(offset); check_output = f.read(4 * 1024 * 1024).decode('utf-8', 'replace')
                            check['cases'] = parse_results(format, report, check_output)
                    check['duration_ms'] = int((time.monotonic() - start) * 1000)
                    code = check['exit_code']
                    cases = check['cases']
                    check['detail'] = detail
                    if reason:
                        check['status'] = reason
                    elif code != 0:
                        check['status'] = 'no_tests' if project['framework'] == 'pytest' and code == 5 else 'error' if target['kind'] == 'tests' and not cases and format != 'custom' else 'failed'
                    elif target['kind'] != 'tests':
                        check['status'] = 'passed'
                    elif not cases:
                        check['status'] = 'unknown' if format == 'custom' else 'no_tests'
                    else:
                        check['status'] = 'failed' if any(c['status'] == 'failed' for c in cases) else 'no_tests' if all(c['status'] == 'skipped' for c in cases) else 'passed'
                    save(run_dir / 'state.json', state)
                    if reason in ('cancelled', 'timed_out'):
                        state.update(status=reason, outcome='unknown'); break
                if state['status'] in ('cancelled', 'timed_out', 'completed'):
                    break
            if not matched:
                raise ValueError('The selected test target no longer exists. Refresh the project list.')
            if state['status'] not in ('cancelled', 'timed_out', 'completed'):
                results = [c['status'] for c in state['checks']]
                outcome = 'failed' if 'failed' in results else 'blocked' if any(s in ('blocked', 'error') for s in results) else 'unknown' if 'unknown' in results else 'no_tests' if 'no_tests' in results else 'passed'
                state.update(status='completed', outcome=outcome)
        except Exception as e:
            state.update(status='error', outcome='blocked', detail=str(e))
        finally:
            state['finished_at'] = time.time()
            state['duration_ms'] = int((state['finished_at'] - state['started_at']) * 1000)
            state['version_changed'] = version(root) != state['version']
            state['version_changed'] = state['version_changed'] or any(version(inside(root,p['path']),root) != state.get('project_versions',{}).get(p['id']) for p in locals().get('selected',[]))
            save(run_dir / 'state.json', state)


def main(options):
    root = Path(options['root']).resolve()
    action = options['action']
    if action == 'discover':
        return discover(root)
    run_dir = state_dir(root, options['run_id'], options.get('host', False))
    if action == 'start':
        helper = run_dir / 'runner.py'
        helper.write_text(options.pop('script'))
        options['action'] = 'worker'
        save(run_dir / 'state.json', {'status': 'queued', 'outcome': 'unknown', 'checks': []})
        with (run_dir / 'worker.log').open('w') as log:
            process = subprocess.Popen([sys.executable, str(helper), json.dumps(options)], stdin=subprocess.DEVNULL, stdout=log, stderr=log, start_new_session=True)
        threading.Thread(target=process.wait, daemon=True).start()
        save(run_dir / 'worker.json', {'pid': process.pid})
        return {'status': 'queued', 'outcome': 'unknown', 'checks': []}
    if action == 'worker':
        worker(options); return {}
    if action == 'cancel':
        (run_dir / 'cancel').touch()
    state = load(run_dir / 'state.json', {'status': 'error', 'outcome': 'unknown', 'detail': 'Run state is unavailable.'})
    for check in state.get('checks', []):
        normalize_case_durations(check.get('cases', []))
    state['log'] = read_log(run_dir / 'output.log')
    if (run_dir / 'cancel').exists() and state.get('status') in ('queued', 'preparing', 'running'):
        state['status'] = 'cancelling'
    if state.get('status') in ('queued', 'preparing', 'running', 'cancelling'):
        pid = (load(run_dir / 'worker.json', {}) or {}).get('pid')
        if pid:
            try:
                os.kill(pid, 0)
                if Path(f'/proc/{pid}/stat').exists() and Path(f'/proc/{pid}/stat').read_text().split(') ', 1)[1].startswith('Z'):
                    raise ProcessLookupError()
            except ProcessLookupError:
                state.update(status='error', outcome='unknown', detail='The test runner stopped without a complete report.')
    return state


if __name__ == '__main__':
    try:
        print(json.dumps(main(json.loads(sys.argv[1])), ensure_ascii=False))
    except Exception as e:
        print(json.dumps({'status': 'error', 'outcome': 'blocked', 'detail': str(e)}))

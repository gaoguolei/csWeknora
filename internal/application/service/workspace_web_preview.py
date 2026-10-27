"""Loaded after workspace_project.py's helpers; starts project HTTP servers."""
import secrets
import signal


# This process installs/builds before replacing itself with the production
# server. Its PID stays stable while discovery polls, even for long builds.
BUILD_SERVER_SCRIPT = r'''
import http.server
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import urlsplit

options = json.loads(sys.argv[1])
plan = options['plan']
if options['install']:
    print('Installing project dependencies...', flush=True)
    subprocess.run(options['install'], check=True, timeout=180)
    Path(options['pending']).unlink(missing_ok=True)
if plan.get('build'):
    print('Building project for preview...', flush=True)
    subprocess.run(plan['build'], check=True, timeout=600)
os.environ['NODE_ENV'] = 'production'
os.environ['PORT'] = str(options['port'])
command = plan.get('serve')
if command:
    command = list(command)
    script = plan['serve_script']
    if re.search(r'\bvite\b(?!\s+preview\b)|\b(?:next|nuxt|astro)\s+dev\b|\b(?:npm|pnpm|yarn)\s+(?:run\s+)?dev\b|\bwebpack(?:-dev-server|\s+serve)\b|\breact-scripts\s+start\b|\bvue-cli-service\s+serve\b', script):
        sys.exit('The preview/start script launches a development server. Configure it to serve the production build instead.')
    if re.search(r'\bvite\s+preview\b', script):
        command += (['--'] if plan['manager'] == 'npm' else []) + ['--host', '127.0.0.1', '--port', str(options['port']), '--strictPort']
    elif re.search(r'\bnext\s+start\b', script):
        command += (['--'] if plan['manager'] == 'npm' else []) + ['--hostname', '127.0.0.1', '--port', str(options['port'])]
    print('Starting production preview (PORT=%d)...' % options['port'], flush=True)
    os.execvpe(command[0], command, os.environ)

root = Path.cwd().resolve()
if not plan.get('static'):
    root = next((root / name for name in ('dist', 'build', 'out') if (root / name / 'index.html').is_file() and not (root / name).is_symlink()), None)
    if root is None:
        sys.exit('Build produced no static dist/build/out/index.html. Add a production preview or start script that listens on PORT and serves the built app and its API routes.')
os.chdir(root)

class StaticPreview(http.server.SimpleHTTPRequestHandler):
    def list_directory(self, path):
        self.send_error(404)
    def send_head(self):
        path = urlsplit(self.path).path
        # Client-side routes use the built index; missing assets remain 404s.
        if not Path(self.translate_path(path)).exists() and not Path(path).suffix and not path.startswith(('/api', '/files', '/mcp/')):
            self.path = '/index.html'
        return super().send_head()
    def end_headers(self):
        self.send_header('Cache-Control', 'no-store')
        super().end_headers()

print('Serving built files from %s on port %d' % (root, options['port']), flush=True)
http.server.ThreadingHTTPServer(('127.0.0.1', options['port']), StaticPreview).serve_forever()
'''


def project_server_environment():
    # A project script may enable --open itself. Keep project servers disconnected
    # from the user's desktop even when the sandbox has a graphical session.
    env = dict(os.environ)
    for name in ('DISPLAY', 'WAYLAND_DISPLAY', 'XAUTHORITY', 'DBUS_SESSION_BUS_ADDRESS'):
        env.pop(name, None)
    env['BROWSER'] = 'none'
    return env


def project_start_command(directory, plan, state_dir, port):
    install = None
    pending = state_dir / ('dependencies-' + hashlib.sha256(str(directory).encode()).hexdigest()[:16] + '.pending')
    if plan.get('manager') and (pending.exists() or not (Path(directory) / 'node_modules').is_dir()):
        manager = plan['manager']
        install = [manager, 'ci' if manager == 'npm' and (Path(directory) / 'package-lock.json').exists() else 'install']
        if manager == 'npm':
            install += ['--no-audit', '--no-fund']
        pending.write_text(manager)
    return [sys.executable, '-u', '-c', BUILD_SERVER_SCRIPT, json.dumps({'install': install, 'plan': plan, 'port': port, 'pending': str(pending)})]


def stop_preview_server(pid):
    # Only stop the process group we created, including its build/server children.
    try:
        if alive(pid) and os.getpgid(int(pid)) == int(pid):
            os.killpg(int(pid), signal.SIGTERM)
            for _ in range(20):
                if not alive(pid):
                    return
                time.sleep(.1)
            os.killpg(int(pid), signal.SIGKILL)
    except ProcessLookupError:
        pass


def web_preview(options):
    root = Path(options['root']).resolve()
    state_dir = Path.home() / '.cache/weknora-preview' / hashlib.sha256(str(root).encode()).hexdigest()[:16]
    state_dir.mkdir(parents=True, exist_ok=True)
    with (state_dir / 'web.lock').open('w') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return {'status': 'starting'}
        # Reuse authenticated relays only for the managed production port.
        relays = []
        for config_file in state_dir.glob('relay-*.json'):
            try:
                relays.append(json.loads(config_file.read_text()))
            except (OSError, ValueError):
                pass
        # Never discover arbitrary listeners: a running dev server must not win
        # over a build in progress or a failed build. Legacy server.pid is ignored.
        server_file = state_dir / 'build-server.json'
        try:
            server = json.loads(server_file.read_text())
        except (OSError, ValueError):
            server = {}
        pid = server.get('pid')
        url = 'http://127.0.0.1:%d/' % server['port'] if server.get('port') else ''
        running = alive(pid)
        ready = running and serves_html(url)
        timed_out = running and not ready and time.time() - server.get('started_at', 0) > 900
        log_path = state_dir / 'server.log'
        if options.get('start') and (not running or ready or timed_out):
            project = project_command(root)
            if not project:
                return {'status': 'project_missing'}
            stop_preview_server(pid)
            for relay in relays:
                stop_preview_server(relay.get('pid'))
                (state_dir / ('relay-%d.json' % relay['port'])).unlink(missing_ok=True)
            directory, plan = project
            with socket.socket() as sock:
                sock.bind(('127.0.0.1', 0))
                port = sock.getsockname()[1]
            with log_path.open('w') as log:
                process = subprocess.Popen(project_start_command(directory, plan, state_dir, port), cwd=directory, stdin=subprocess.DEVNULL, stdout=log, stderr=log, start_new_session=True, env=project_server_environment())
            server_file.write_text(json.dumps({'pid': process.pid, 'port': port, 'started_at': time.time()}))
            return {'status': 'starting', 'detail': log_tail(log_path)}
        if not ready:
            if timed_out:
                return {'status': 'start_failed', 'detail': log_tail(log_path) + '\nBuild or preview startup timed out. Production preview scripts must listen on the PORT environment variable.'}
            if running:
                return {'status': 'starting', 'detail': log_tail(log_path)}
            if pid and log_path.exists():
                return {'status': 'start_failed', 'detail': log_tail(log_path)}
            return {'status': 'project_missing' if not project_command(root) else 'not_running'}
        port = urlsplit(url).port
        config_file = state_dir / ('relay-%d.json' % port)
        config = next((item for item in relays if item['port'] == port and alive(item.get('pid'))), None)
        if config is None:
            with socket.socket() as sock:
                sock.bind(('0.0.0.0', 0))
                relay_port = sock.getsockname()[1]
            config = {'port': port, 'relay_port': relay_port, 'secret': secrets.token_hex(32)}
            process = subprocess.Popen([sys.executable, '-u', '-c', options['relay_script'], json.dumps(config)], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
            config['pid'] = process.pid
            config_file.write_text(json.dumps(config))
            config_file.chmod(0o600)
        # Do not report ready before the forwarding listener has started.
        for _ in range(30):
            try:
                with socket.create_connection(('127.0.0.1', config['relay_port']), timeout=.1):
                    return {'status': 'ready', 'url': url, **config}
            except OSError:
                time.sleep(.1)
        return {'status': 'start_failed', 'detail': 'Could not start the project relay.'}

if __name__ == '__main__':
    try:
        print(json.dumps(web_preview(json.loads(sys.argv[1]))))
    except Exception as error:
        print(json.dumps({'status': 'start_failed', 'detail': str(error)}))

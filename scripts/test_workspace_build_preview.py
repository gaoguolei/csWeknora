"""Production preview regressions, including real build and HTTP integration."""
import http.client
import http.server
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
import types
import unittest
from unittest.mock import patch

REPO = Path(__file__).resolve().parents[1]
preview = types.ModuleType('build_preview')
for name in ('workspace_project.py', 'workspace_web_preview.py'):
    path = REPO / 'internal/application/service' / name
    exec(compile(path.read_text(), str(path), 'exec'), preview.__dict__)


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def get(port, path='/'):
    connection = http.client.HTTPConnection('127.0.0.1', port, timeout=2)
    try:
        connection.request('GET', path)
        response = connection.getresponse()
        return response.status, response.read()
    finally:
        connection.close()


class BuildPreviewTests(unittest.TestCase):
    def package(self, root, scripts):
        root.mkdir(parents=True, exist_ok=True)
        (root / 'package.json').write_text(json.dumps({'type': 'module', 'scripts': scripts}))
        (root / 'node_modules').mkdir(exist_ok=True)

    def state(self, root, data):
        directory = root / '.cache/weknora-preview' / preview.hashlib.sha256(str(root.resolve()).encode()).hexdigest()[:16]
        directory.mkdir(parents=True)
        (directory / 'build-server.json').write_text(json.dumps(data))
        (directory / 'server.log').write_text('Build failed: missing import')
        return directory

    def test_discovery_prefers_frontend_build_and_never_selects_dev(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.package(root / 'backend', {'build': 'tsc', 'start': 'node dist/server.js'})
            self.package(root / 'frontend', {'build': 'vite build', 'preview': 'vite preview', 'dev': 'vite'})
            self.package(root / 'node_modules/frontend', {'build': 'untrusted'})
            directory, plan = preview.project_command(root)
            self.assertEqual(directory, str(root / 'frontend'))
            self.assertEqual(plan['build'], ['npm', 'run', 'build'])
            self.assertEqual(plan['serve'], ['npm', 'run', 'preview'])

    def test_unbuilt_source_and_external_symlinks_are_not_static_projects(self):
        with tempfile.TemporaryDirectory() as tmp, tempfile.TemporaryDirectory() as outside:
            root = Path(tmp)
            self.package(root, {'dev': 'vite'})
            (root / 'index.html').write_text('<script src="/src/main.ts"></script>')
            (root / 'public').mkdir()
            (root / 'public/index.html').write_text('source template')
            (Path(outside) / 'index.html').write_text('outside')
            (root / 'linked').symlink_to(outside, target_is_directory=True)
            self.assertIsNone(preview.project_command(root))

    def test_lookup_ignores_existing_development_server_without_starting(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.package(root, {'build': 'vite build'})
            with patch.object(preview.Path, 'home', return_value=root), patch.object(preview, 'serves_html', return_value=True) as probe, patch.object(preview.subprocess, 'Popen') as spawn:
                self.assertEqual(preview.web_preview({'root': tmp})['status'], 'not_running')
                probe.assert_not_called()
                spawn.assert_not_called()

    def test_build_in_progress_does_not_start_twice_or_show_dev_page(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.state(root, {'pid': 123, 'port': 4173, 'started_at': time.time()})
            with patch.object(preview.Path, 'home', return_value=root), patch.object(preview, 'alive', return_value=True), patch.object(preview, 'serves_html', return_value=False), patch.object(preview.subprocess, 'Popen') as spawn:
                self.assertEqual(preview.web_preview({'root': tmp, 'start': True})['status'], 'starting')
                spawn.assert_not_called()

    def test_failed_build_reports_logs_instead_of_a_running_dev_page(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.state(root, {'pid': 123, 'port': 4173})
            with patch.object(preview.Path, 'home', return_value=root), patch.object(preview, 'alive', return_value=False), patch.object(preview, 'serves_html', return_value=True):
                result = preview.web_preview({'root': tmp})
                self.assertEqual(result['status'], 'start_failed')
                self.assertIn('missing import', result['detail'])
                self.assertNotIn('url', result)

    def test_ready_preview_refresh_rebuilds_on_a_new_managed_port(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.package(root, {'build': 'vite build'})
            state = self.state(root, {'pid': 123, 'port': 4173})
            with patch.object(preview.Path, 'home', return_value=root), patch.object(preview, 'alive', return_value=True), patch.object(preview, 'serves_html', return_value=True), patch.object(preview, 'stop_preview_server') as stop, patch.object(preview.subprocess, 'Popen') as spawn:
                spawn.return_value.pid = 456
                self.assertEqual(preview.web_preview({'root': tmp, 'start': True})['status'], 'starting')
                stop.assert_called_once_with(123)
                self.assertEqual(json.loads((state / 'build-server.json').read_text())['pid'], 456)
                options = json.loads(spawn.call_args.args[0][-1])
                self.assertEqual(options['plan']['build'], ['npm', 'run', 'build'])
                self.assertNotEqual(options['port'], 4173)

    def launch(self, root, plan, port):
        log = (root / 'preview.log').open('w+')
        process = subprocess.Popen(preview.project_start_command(root, plan, root, port), cwd=root, stdout=log, stderr=log, start_new_session=True, env=preview.project_server_environment())
        def cleanup():
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            process.wait(timeout=5)
            log.close()
        self.addCleanup(cleanup)
        return process, log

    def wait_ready(self, process, log, port):
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            if preview.serves_html('http://127.0.0.1:%d/' % port):
                return
            if process.poll() is not None:
                break
            time.sleep(.1)
        log.seek(0)
        self.fail('Preview did not start: ' + log.read())

    def test_real_build_serves_only_artifacts_and_supports_spa_routes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'index.html').write_text('<html>UNBUILT SOURCE</html>')
            build = "from pathlib import Path; p=Path('dist'); p.mkdir(exist_ok=True); (p/'index.html').write_text('<html>BUILT ARTIFACT</html>')"
            plan = {'build': [sys.executable, '-c', build]}
            port = free_port()
            process, log = self.launch(root, plan, port)
            self.wait_ready(process, log, port)
            self.assertEqual(get(port), (200, b'<html>BUILT ARTIFACT</html>'))
            self.assertEqual(get(port, '/settings/profile?x=1'), get(port))
            self.assertEqual(get(port, '/missing.js')[0], 404)
            self.assertEqual(get(port, '/api/users')[0], 404)
            self.assertEqual(get(port, '/package.json')[0], 404)

    def test_failed_real_build_never_serves_stale_dist(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'dist').mkdir()
            (root / 'dist/index.html').write_text('<html>STALE</html>')
            port = free_port()
            process, _ = self.launch(root, {'build': [sys.executable, '-c', 'raise SystemExit(1)']}, port)
            self.assertNotEqual(process.wait(timeout=5), 0)
            self.assertFalse(preview.serves_html('http://127.0.0.1:%d/' % port))

    def test_preview_script_cannot_disguise_a_development_server(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            plan = {'serve': ['does-not-run'], 'serve_script': 'vite --host', 'manager': 'npm'}
            process, log = self.launch(root, plan, free_port())
            self.assertNotEqual(process.wait(timeout=5), 0)
            log.seek(0)
            self.assertIn('development server', log.read())

    @unittest.skipUnless(shutil.which('npm') and (REPO / 'frontend/node_modules/vite/bin/vite.js').is_file(), 'requires local Vite installation')
    def test_real_vite_build_preserves_production_api_proxy(self):
        class Backend(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(b'{"source":"backend"}')
            def log_message(self, *args):
                pass
        backend = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Backend)
        threading.Thread(target=backend.serve_forever, daemon=True).start()
        self.addCleanup(backend.server_close)
        self.addCleanup(backend.shutdown)
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.package(root, {'build': 'vite build', 'preview': 'vite preview', 'dev': 'vite'})
            (root / 'node_modules').rmdir()
            (root / 'node_modules').symlink_to(REPO / 'frontend/node_modules', target_is_directory=True)
            (root / 'index.html').write_text('<html>BUILT VITE<script type="module" src="/main.js"></script></html>')
            (root / 'main.js').write_text('console.log("production asset")')
            (root / 'vite.config.js').write_text("export default { preview: { proxy: { '/api': 'http://127.0.0.1:%d' } } }" % backend.server_port)
            _, plan = preview.project_command(root)
            port = free_port()
            process, log = self.launch(root, plan, port)
            self.wait_ready(process, log, port)
            status, body = get(port)
            self.assertEqual(status, 200)
            self.assertIn(b'/assets/', body)
            self.assertNotIn(b'/@vite/client', body)
            self.assertNotIn(b'/main.js', body)
            self.assertEqual(get(port, '/api/data'), (200, b'{"source":"backend"}'))
            self.assertEqual(get(port, '/settings')[0], 200)


if __name__ == '__main__':
    unittest.main()

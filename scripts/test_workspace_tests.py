"""Real local runner regressions; no production account, service or sandbox."""
import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time
import unittest
import uuid
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / 'internal/application/service/workspace_tests.py'
spec = importlib.util.spec_from_file_location('validation', SCRIPT)
validation = importlib.util.module_from_spec(spec)
spec.loader.exec_module(validation)


class WorkspaceTests(unittest.TestCase):
    def project(self, root, script='node --test tests/*.test.mjs', source=None):
        (root / 'package.json').write_text(json.dumps({'name': 'demo', 'scripts': {'test': script}}))
        (root / 'tests').mkdir(exist_ok=True)
        if source is not None:
            (root / 'tests/unit.test.mjs').write_text(source)

    def run_project(self, root, **kwargs):
        rid = str(uuid.uuid4())
        validation.worker({'root': str(root), 'run_id': rid, 'timeout': 10, 'host': True, **kwargs})
        return json.loads((root / '.weknora/test-runs' / rid / 'state.json').read_text())

    def test_native_node_reports_actual_cases_and_no_false_source_change(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.project(root, source="import test from 'node:test';import assert from 'node:assert/strict';test('add',()=>assert.equal(1+1,2));test('subtract',()=>assert.equal(3-1,2));")
            state = self.run_project(root)
            self.assertEqual(state['status'], 'completed')
            self.assertEqual(state['outcome'], 'passed')
            self.assertEqual([c['name'] for c in state['checks'][0]['cases']], ['add', 'subtract'])
            self.assertFalse(state['version_changed'])

    def test_failed_assertion_is_different_from_missing_runtime(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.project(root, source="import test from 'node:test';import assert from 'node:assert/strict';test('fails',()=>assert.equal(1,2));")
            state = self.run_project(root)
            self.assertEqual(state['outcome'], 'failed')
            self.assertEqual(state['checks'][0]['cases'][0]['status'], 'failed')
            rid = str(uuid.uuid4())
            folder = validation.state_dir(root, rid, True)
            with (folder / 'output.log').open('w') as log:
                code, reason, _ = validation.execute(['/missing-runtime'], root, folder, {}, log, 2)
            self.assertIsNone(code)
            self.assertEqual(reason, 'blocked')

    def test_zero_actual_tests_is_not_a_pass(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.project(root, source='// no tests')
            state = self.run_project(root)
            self.assertEqual(state['outcome'], 'no_tests')
            self.assertEqual(state['checks'][0]['exit_code'], 0)

    def test_discovery_separates_frontend_backend_and_skips_dependencies(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for folder in ('frontend', 'backend', 'node_modules/unrelated'):
                project = root / folder; project.mkdir(parents=True)
                self.project(project, source="import test from 'node:test';test('smoke',()=>{});")
            projects = validation.discover(root)['projects']
            self.assertEqual({p['path'] for p in projects}, {'frontend', 'backend'})
            self.assertTrue(all(len(p['targets']) == 2 for p in projects))

    def test_file_selection_does_not_run_sibling_test(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.project(root, source="import test from 'node:test';test('chosen',()=>{});")
            (root / 'tests/other.test.mjs').write_text("import test from 'node:test';test('other',()=>{throw Error('should not run')});")
            project = validation.discover(root)['projects'][0]
            target = next(t for t in project['targets'] if t['file'] == 'tests/unit.test.mjs')
            state = self.run_project(root, project_id=project['id'], target_id=target['id'])
            self.assertEqual(state['outcome'], 'passed')
            self.assertEqual([c['name'] for c in state['checks'][0]['cases']], ['chosen'])

    def test_actual_case_selector_does_not_run_a_different_assertion(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            self.project(root, source="import test from 'node:test';test('chosen',()=>{});test('other',()=>{throw Error('must not run')});")
            project=validation.discover(root)['projects'][0]
            state=self.run_project(root,project_id=project['id'],target_id=project['targets'][0]['id'],case_name='chosen',case_file=str(root/'tests/unit.test.mjs'))
            self.assertEqual(state['outcome'],'passed')
            self.assertTrue(any(case['name']=='chosen' and case['status']=='passed' for case in state['checks'][0]['cases']))
            self.assertFalse(any(case['name']=='other' and case['status']=='failed' for case in state['checks'][0]['cases']))

    def test_detached_run_cancellation_stops_only_its_process_group(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.project(root, source="import test from 'node:test';import{writeFileSync}from'node:fs';test('slow',async()=>{writeFileSync('test-pid',String(process.pid));await new Promise(r=>setTimeout(r,30000))});")
            server = subprocess.Popen(['sleep', '30'])
            try:
                rid = str(uuid.uuid4())
                opts = {'root': str(root), 'run_id': rid, 'timeout': 15, 'host': True}
                validation.main({**opts, 'action': 'start', 'script': SCRIPT.read_text()})
                deadline = time.monotonic() + 8
                while not (root / 'test-pid').exists() and time.monotonic() < deadline:
                    time.sleep(.05)
                self.assertTrue((root / 'test-pid').exists())
                pid = int((root / 'test-pid').read_text())
                validation.main({**opts, 'action': 'cancel'})
                while time.monotonic() < deadline:
                    state = validation.main({**opts, 'action': 'status'})
                    if state['status'] == 'cancelled': break
                    time.sleep(.05)
                self.assertEqual(state['status'], 'cancelled')
                self.assertNotEqual(state['outcome'], 'passed')
                self.assertIsNone(server.poll(), 'existing project services must survive cancellation')
                with self.assertRaises(ProcessLookupError): os.kill(pid, 0)
            finally:
                server.terminate(); server.wait()

    def test_structured_json_results_and_html_payload_remain_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            report = Path(tmp) / 'result.json'
            report.write_text(json.dumps({'testResults': [{'name': 'test.ts', 'assertionResults': [{'fullName': 'case', 'status': 'failed', 'failureMessages': ['<script>alert(1)</script>']}]}]}))
            cases = validation.parse_results('json', report, '10 tests passed')
            self.assertEqual(cases[0]['status'], 'failed')
            self.assertEqual(cases[0]['detail'], '<script>alert(1)</script>')
            self.assertEqual(validation.parse_results('tap', report, '10 tests passed'), [])

    def test_fractional_native_durations_are_integer_milliseconds(self):
        with tempfile.TemporaryDirectory() as tmp:
            report = Path(tmp) / 'result.json'
            fixtures = {
                'json': {'testResults': [{'name': 'frontend/test.ts', 'assertionResults': [{'fullName': 'case', 'status': 'passed', 'duration': 328.5659089999999}]}]},
                'node-json': {'cases': [{'name': 'case', 'status': 'passed', 'duration_ms': 328.5659089999999}]},
                'playwright': {'suites': [{'specs': [{'title': 'case', 'tests': [{'status': 'expected', 'results': [{'duration': 328.5659089999999}]}]}]}]},
            }
            for format, payload in fixtures.items():
                with self.subTest(format=format):
                    report.write_text(json.dumps(payload))
                    case = validation.parse_results(format, report, '')[0]
                    self.assertEqual(case['duration_ms'], 328)
                    self.assertIs(type(case['duration_ms']), int)
                    self.assertEqual(case['status'], 'passed')

    def test_status_and_cancel_can_read_old_fractional_duration_snapshots(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            rid = str(uuid.uuid4())
            folder = validation.state_dir(root, rid, True)
            snapshot = {'status': 'running', 'outcome': 'unknown', 'checks': [{'cases': [{'name': 'completed case', 'status': 'passed', 'duration_ms': 328.5659089999999}]}]}
            (folder / 'state.json').write_text(json.dumps(snapshot))
            options = {'root': str(root), 'run_id': rid, 'host': True}
            for action, status in [('status', 'running'), ('cancel', 'cancelling')]:
                with self.subTest(action=action):
                    state = validation.main({**options, 'action': action})
                    self.assertEqual(state['status'], status)
                    self.assertEqual(state['checks'][0]['cases'][0]['duration_ms'], 328)
                    self.assertIs(type(state['checks'][0]['cases'][0]['duration_ms']), int)
            self.assertTrue((folder / 'cancel').exists())


if __name__ == '__main__':
    unittest.main()

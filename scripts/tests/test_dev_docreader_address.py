"""Exercise dev-app environment loading without starting Go, Air or containers."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "dev.sh"


class DocReaderDevelopmentAddressTest(unittest.TestCase):
    def start_app_address(self, values):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".env").write_text("DB_DRIVER=postgres\n" + values)
            # Source the real entry point in help mode; replace only unrelated
            # runtime/build capabilities. Air reports the actual exported env.
            command = r'''
set -- help
source "$DEV_SCRIPT" >/dev/null
PROJECT_ROOT="$TEST_PROJECT_ROOT"
go() { :; }
enable_anydoc_build_tag() { :; }
add_go_bin_to_path() { :; }
air() { printf 'RESULT=%s|%s\n' "$DOCREADER_ADDR" "$DOCREADER_TRANSPORT"; }
start_app
'''
            env = {key: value for key, value in os.environ.items()
                   if not key.startswith(("DOCREADER_", "DEV_REMOTE_", "GRPC_"))}
            env.update(DEV_SCRIPT=str(SCRIPT), TEST_PROJECT_ROOT=directory)
            result = subprocess.run(["bash", "-c", command], env=env,
                                    capture_output=True, text=True, check=True)
            return next(line.removeprefix("RESULT=") for line in result.stdout.splitlines()
                        if line.startswith("RESULT="))

    def test_explicit_remote_grpc_endpoint_survives_env_loading(self):
        self.assertEqual(self.start_app_address(
            "DOCREADER_ADDR=47.92.240.13:5051\nDOCREADER_TRANSPORT=grpc\n"),
            "47.92.240.13:5051|grpc")

    def test_unset_and_empty_addresses_use_local_default(self):
        for values in ["", "DOCREADER_ADDR=\n"]:
            with self.subTest(values=values):
                self.assertEqual(self.start_app_address(values), "127.0.0.1:50051|grpc")

    def test_compose_default_is_mapped_to_the_published_local_port(self):
        self.assertEqual(self.start_app_address(
            "DOCREADER_ADDR=docreader:50051\nDOCREADER_PORT=5051\n"),
            "127.0.0.1:5051|grpc")

    def test_explicit_loopback_and_http_addresses_are_preserved(self):
        self.assertEqual(self.start_app_address("DOCREADER_ADDR=127.0.0.1:15051\n"),
                         "127.0.0.1:15051|grpc")
        self.assertEqual(self.start_app_address(
            "DOCREADER_ADDR=https://parser.example.com\nDOCREADER_TRANSPORT=https\n"),
            "https://parser.example.com|https")


if __name__ == "__main__":
    unittest.main()

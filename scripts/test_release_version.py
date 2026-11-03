"""Release numbering and retry behavior, independent of a live GitHub repository."""
from pathlib import Path
import os
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("release-version.py").resolve()

class ReleaseVersionTest(unittest.TestCase):
    def test_first_release_patch_and_retry(self):
        with tempfile.TemporaryDirectory() as directory:
            def git(*args):
                return subprocess.check_output(
                    ["git", *args], cwd=directory, stderr=subprocess.DEVNULL,
                    env={**os.environ, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"},
                )
            def version():
                return subprocess.check_output(["python3", str(SCRIPT)], cwd=directory, text=True).strip()
            git("init")
            git("config", "user.email", "test@example.com")
            git("config", "user.name", "Test")
            git("commit", "--allow-empty", "-m", "first")
            self.assertEqual(version(), "v1.0.0")
            git("tag", "v1.0.0")
            self.assertEqual(version(), "v1.0.0")
            git("commit", "--allow-empty", "-m", "next")
            git("tag", "v99.0.0-beta")
            self.assertEqual(version(), "v1.0.1")
            git("tag", "v1.0.1")
            self.assertEqual(version(), "v1.0.1")

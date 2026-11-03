"""Hermetic retry/finish regressions; no credentials, services or app builds."""
import hashlib
import importlib.util
import io
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import urllib.error

SCRIPT = Path(__file__).with_name("verify-release.py").resolve()
SPEC = importlib.util.spec_from_file_location("verify_release", SCRIPT)
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)
PUBLISHER_HELPER = release.publisher
REPO = SCRIPT.parent.parent
TARGETS = "linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 darwin-universal.dmg"
TAG = "v1.0.0"


class ReleaseRetryTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.assets = self.directory / "assets"
        self.assets.mkdir()
        self.remote = {}
        for target in TARGETS.split():
            name = f"oh-my-disk-{target}"
            content = name.encode()
            (self.assets / name).write_bytes(content)
            self.remote[name] = content
        self.remote["RELEASE"] = f"oh-my-disk {TAG}\n".encode()
        self.sign()
        self.remote["latest"] = TAG.encode() + b"\n"
        self.requests = []
        for context in (
            patch.object(release, "ASSETS", self.assets),
            patch.dict(os.environ, RELEASE_VERSION=TAG),
            patch.object(release, "publisher", side_effect=self.publisher),
            patch.object(release.urllib.request, "urlopen", side_effect=self.urlopen),
        ):
            context.start()
            self.addCleanup(context.stop)

    def sign(self):
        self.remote["SHA256SUMS"] = "".join(
            f"{hashlib.sha256(content).hexdigest()}  {name}\n"
            for name, content in self.remote.items()
            if name not in {"SHA256SUMS", "SHA256SUMS.sig", "latest"}
        ).encode()
        self.remote["SHA256SUMS.sig"] = self.remote["SHA256SUMS"] + b"test-signature"

    def publisher(self, command, *args):
        if command.startswith("pkg_known"):
            return TARGETS
        if command.startswith("pkg_field"):
            return "oh-my-disk"
        if command.startswith("config_get"):
            return "https://example.invalid"
        self.assertEqual(command, 'sig_verify "$1"')
        self.assertEqual(self.requests, ["latest", "SHA256SUMS", "SHA256SUMS.sig"])
        directory = Path(args[0])
        if (directory / "SHA256SUMS.sig").read_bytes() != (
            directory / "SHA256SUMS"
        ).read_bytes() + b"test-signature":
            raise subprocess.CalledProcessError(1, "sig_verify")
        return ""

    def urlopen(self, url, timeout):
        self.assertEqual(timeout, 60)
        name = url.rsplit("/", 1)[1]
        self.requests.append(name)
        content = self.remote.get(name, 404)
        if isinstance(content, int):
            raise urllib.error.HTTPError(url, content, "fixture failure", {}, None)
        if isinstance(content, Exception):
            raise content
        return io.BytesIO(content)

    def test_completed_retry_reads_all_five_assets(self):
        self.assertTrue(release.run("detect"))
        self.assertTrue({path.name for path in self.assets.iterdir()} <= set(self.requests))

    def test_same_latest_metadata_verified_for_finish(self):
        self.assertTrue(release.run("verify", self.directory))
        self.assertEqual((self.directory / "RELEASE").read_bytes(), self.remote["RELEASE"])

    def test_historical_retry_verifies_immutable_version_without_latest(self):
        self.remote["latest"] = b"v1.0.10\n"
        self.assertTrue(release.run("detect"))
        self.requests.clear()
        self.assertFalse(release.run("verify", self.directory))

    def test_older_or_missing_latest_resumes_publish(self):
        for latest in (b"v0.9.9\n", 404):
            self.remote["latest"] = latest
            self.assertFalse(release.run("detect"))
        self.assertEqual(self.requests, ["latest", "latest"])

    def test_bad_signature_fails_before_asset_downloads(self):
        self.remote["SHA256SUMS.sig"] = b"forged"
        with self.assertRaises(subprocess.CalledProcessError):
            release.run("detect")

    def test_missing_extra_duplicate_and_malformed_manifest_fail(self):
        original = self.remote["SHA256SUMS"]
        for manifest in (
            original.split(b"\n", 1)[1],
            original + b"0" * 64 + b"  extra\n",
            original + original.splitlines(keepends=True)[0],
            original.replace(b"  oh-my", b" *oh-my", 1),
        ):
            self.requests.clear()
            self.remote["SHA256SUMS"] = manifest
            self.remote["SHA256SUMS.sig"] = manifest + b"test-signature"
            with self.assertRaises(ValueError):
                release.run("detect")

    def test_signed_wrong_package_or_tag_fails(self):
        for binding in (b"other v1.0.0\n", b"oh-my-disk v1.0.1\n"):
            self.requests.clear()
            self.remote["RELEASE"] = binding
            self.sign()
            with self.assertRaises(ValueError):
                release.run("detect")

    def test_remote_or_draft_mismatch_fails(self):
        name = "oh-my-disk-darwin-universal.dmg"
        self.remote[name] = b"tampered"
        with self.assertRaises(ValueError):
            release.run("detect")
        self.requests.clear()
        self.sign()
        with self.assertRaises(ValueError):
            release.run("detect")

    def test_network_and_missing_remote_assets_fail_closed(self):
        for name, failure in (("latest", 500), ("latest", OSError("offline")),
                              ("oh-my-disk-linux-arm64", 404)):
            with self.subTest(name=name, failure=failure):
                self.requests.clear()
                previous = self.remote[name]
                self.remote[name] = failure
                with self.assertRaises(OSError):
                    release.run("detect")
                self.remote[name] = previous

    def test_missing_local_asset_and_forged_marker_fail(self):
        (self.assets / "oh-my-disk-linux-arm64").unlink()
        (self.assets / "verified").write_text("true")
        with self.assertRaises(ValueError):
            release.run("detect")
        self.assertEqual(self.requests, [])

    def test_finish_refuses_older_latest(self):
        self.remote["latest"] = b"v0.9.9\n"
        with self.assertRaises(ValueError):
            release.run("verify", self.directory)

    def test_publisher_helper_sources_trusted_common(self):
        trusted = Path(os.environ.get("RELEASE_TEST_PUBLISHER", REPO / ".release-publisher"))
        if not (trusted / "lib/common.sh").is_file():
            self.skipTest("CI checks out publisher; locally set RELEASE_TEST_PUBLISHER")
        key = self.directory / "fixture-key"
        subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(key)], check=True)
        manifest = self.directory / "SHA256SUMS"
        manifest.write_bytes(self.remote["SHA256SUMS"])
        with patch.object(release, "PUBLISHER", trusted):
            namespace = PUBLISHER_HELPER('printf "%s" "$SIGN_NAMESPACE"')
            subprocess.run(["ssh-keygen", "-Y", "sign", "-f", str(key), "-n", namespace,
                            str(manifest)], check=True, capture_output=True)
            command = 'PUBKEY_FILE=$2; sig_verify "$1"'
            PUBLISHER_HELPER(command, self.directory, f"{key}.pub")
            manifest.write_bytes(manifest.read_bytes() + b"tampered")
            with self.assertRaises(subprocess.CalledProcessError):
                PUBLISHER_HELPER(command, self.directory, f"{key}.pub")

    def test_finish_never_uploads_or_exposes_failed_verification(self):
        commands = self.directory / "bin"
        commands.mkdir()
        (commands / "python3").write_text("#!/bin/sh\nexit 1\n")
        (commands / "gh").write_text('#!/bin/sh\necho called >"$GH_LOG"\n')
        for command in commands.iterdir():
            command.chmod(0o755)
        log = self.directory / "gh.log"
        result = subprocess.run(
            ["sh", str(SCRIPT.with_name("publish-github-release.sh")), "finish"],
            cwd=REPO, env={**os.environ, "PATH": f"{commands}:{os.environ["PATH"]}",
                           "GH_LOG": str(log), "GH_REPO": "fixture/repo",
                           "RELEASE_COMMIT": "fixture"}, capture_output=True,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(log.exists())

    def test_workflow_retry_contract(self):
        workflow = (REPO / ".github/workflows/release.yml").read_text()
        self.assertEqual(workflow.count("repository: fum4/releases"), 1)
        self.assertRegex(workflow, r"ref: [a-f0-9]{40}\n")
        self.assertIn("uses: ./.release-publisher", workflow)
        self.assertIn("run: ./.release-publisher/bin/publish --index-only", workflow)
        self.assertIn("if: steps.published.outputs.done != 'true'", workflow)
        self.assertIn("working_directory: .release-publisher", workflow)
        self.assertIn("install_args: rclone", workflow)
        self.assertNotIn("dry-run:", workflow)
        self.assertIn("path: dist/release/*", workflow)
        self.assertNotIn("--ignore-missing", workflow)

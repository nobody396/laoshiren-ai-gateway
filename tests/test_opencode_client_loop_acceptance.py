from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "opencode_client_loop_acceptance.py"
SPEC = importlib.util.spec_from_file_location("opencode_client_loop_acceptance_test", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class OpenCodeClientLoopAcceptanceTest(unittest.TestCase):
    def test_exact_version_and_all_protocol_configs_are_secret_free(self):
        self.assertEqual(MODULE.CLIENT_VERSION, "1.18.15")
        for protocol in sorted(MODULE.PROTOCOLS):
            config = MODULE.build_config("alpha", protocol)
            rendered = json.dumps(config)
            self.assertEqual(
                config["provider"][MODULE.provider_id(protocol)]["options"]["apiKey"],
                "{env:LAOSHIRENAI_OPENCODE_KEY}",
            )
            self.assertNotIn("sk-", rendered)
            self.assertEqual(
                config["provider"][MODULE.provider_id(protocol)]["npm"],
                MODULE.PROVIDER_PACKAGES[protocol],
            )

    def test_temporary_home_config_never_contains_child_key(self):
        with tempfile.TemporaryDirectory() as directory:
            home = Path(directory)
            path = MODULE.write_config(home, "alpha", "responses")
            content = path.read_text()
            self.assertNotIn("sk-owned-secret-value", content)
            self.assertIn("{env:LAOSHIRENAI_OPENCODE_KEY}", content)
            self.assertTrue(str(path).startswith(str(home)))

    def test_child_env_drops_unrelated_credentials_and_keeps_key_only_in_env(self):
        with tempfile.TemporaryDirectory() as directory:
            env = MODULE.child_env(Path(directory), "sk-owned-secret-value", {
                "PATH": "/bin", "UNRELATED_API_KEY": "sk-do-not-forward",
                "AUTH_TOKEN": "token-value", "LANG": "C",
            })
            self.assertEqual(env[MODULE.KEY_ENV], "sk-owned-secret-value")
            self.assertEqual(env["PATH"], "/bin")
            self.assertEqual(env["LANG"], "C")
            self.assertNotIn("UNRELATED_API_KEY", env)
            self.assertNotIn("AUTH_TOKEN", env)

    def test_parser_requires_exact_target_protocol(self):
        self.assertEqual(
            MODULE.parse_target("alpha:6:responses"),
            ("alpha", 6, "responses"),
        )
        with self.assertRaises(Exception):
            MODULE.parse_target("alpha:6:unknown")

    def test_json_stream_parser_requires_real_successful_read_and_shell(self):
        file_marker = "FILE_MARKER"
        final_marker = "FINAL_MARKER"
        events = [
            {"type": "tool_use", "part": {"tool": "read", "state": {"status": "completed", "output": file_marker}}},
            {"type": "tool_use", "part": {"tool": "bash", "state": {"status": "completed", "output": "ok"}}},
            {"type": "text", "part": {"type": "text", "text": final_marker}},
        ]
        raw = " ".join(json.dumps(event) for event in events).encode()
        parsed = MODULE.parse_events(raw, file_marker, final_marker)
        self.assertTrue(parsed["tool_use_observed"])
        self.assertTrue(parsed["tool_result_observed"])
        self.assertTrue(parsed["read_tool_verified"])
        self.assertTrue(parsed["shell_tool_verified"])
        self.assertTrue(parsed["file_marker_verified"])
        self.assertTrue(parsed["final_marker_verified"])
        self.assertNotIn(file_marker, json.dumps(parsed))

    def test_usage_attribution_is_exact_per_protocol(self):
        class Controller:
            def admin_cli_json(self, _args):
                return {"data": {"items": [{
                    "id": 1,
                    "created_at": "2026-09-01T00:00:01+00:00",
                    "api_key_id": 128,
                    "group_id": 6,
                    "model": "alpha",
                    "requested_model": "alpha",
                    "inbound_endpoint": "/v1/responses",
                    "upstream_endpoint": "/v1/responses",
                    "user_agent": "opencode/1.18.15",
                    "input_tokens": 1,
                    "output_tokens": 2,
                }]}}

        rows, upstream = MODULE.usage_rows(
            Controller(), model_id="alpha", group_id=6,
            protocol="responses", started_at="2026-09-01T00:00:00+00:00",
        )
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["inbound_endpoint"], "/v1/responses")
        self.assertEqual(upstream, "/v1/responses")

    def test_chat_attribution_accepts_one_exact_responses_bridge(self):
        class Controller:
            def admin_cli_json(self, _args):
                return {"data": {"items": [{
                    "id": 1, "created_at": "2026-09-01T00:00:01+00:00",
                    "api_key_id": 128, "group_id": 34, "model": "grok-4.5",
                    "inbound_endpoint": "/v1/chat/completions", "upstream_endpoint": "/v1/responses",
                    "user_agent": "opencode/1.18.15", "input_tokens": 1, "output_tokens": 2,
                }]}}

        rows, upstream = MODULE.usage_rows(
            Controller(), model_id="grok-4.5", group_id=34,
            protocol="chat_completions", started_at="2026-09-01T00:00:00+00:00",
        )
        self.assertEqual(len(rows), 1)
        self.assertEqual(upstream, "/v1/responses")

    def test_atomic_receipt_rejects_secret_and_has_canonical_sha(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "receipt.json"
            MODULE.atomic_json(path, {"kind": "fixture", "secret_free": True})
            receipt = json.loads(path.read_text())
            embedded = receipt.pop("artifact_sha256")
            self.assertEqual(embedded, MODULE.canonical_sha(receipt))
            with self.assertRaisesRegex(RuntimeError, "secret-shaped"):
                MODULE.atomic_json(path, {"value": "sk-owned-secret-value"})

    def test_dual_live_gate_stops_before_controller_or_subprocess(self):
        completed = subprocess.run(
            [
                sys.executable, str(SCRIPT),
                "--target", "alpha:6:responses",
                "--output-dir", "/tmp/opencode-acceptance-test-no-run",
            ],
            cwd=ROOT, text=True, capture_output=True, check=False,
        )
        self.assertNotEqual(completed.returncode, 0)
        self.assertIn("live run requires", completed.stderr)

    def test_version_parser_is_exact(self):
        self.assertTrue(MODULE.version_ok(b"opencode 1.18.15\n"))
        self.assertFalse(MODULE.version_ok(b"opencode 1.18.14\n"))
        self.assertFalse(MODULE.version_ok(b"opencode 11.18.150\n"))

    def test_finally_restores_group_six_when_loop_raises(self):
        class Controller:
            def __init__(self): self.switches = []
            def owned_key(self):
                return {"id": 128, "user_id": 2, "group_id": 6, "key": "sk-fixture-secret-value"}
            def switch_group(self, group_id):
                self.switches.append(group_id)
                return {"group_id": group_id}

        controller = Controller()
        old_load, old_run = MODULE.load_controller, MODULE.run_one
        MODULE.load_controller = lambda: controller
        MODULE.run_one = lambda *_args, **_kwargs: (_ for _ in ()).throw(RuntimeError("boom"))
        try:
            with self.assertRaisesRegex(RuntimeError, "boom"):
                MODULE.main([
                    "--target", "alpha:6:responses",
                    "--output-dir", "/tmp/opencode-acceptance-test-restore",
                    "--execute", "--acknowledge-paid-probes",
                ])
            self.assertEqual(controller.switches[-1], 6)
        finally:
            MODULE.load_controller, MODULE.run_one = old_load, old_run


if __name__ == "__main__":
    unittest.main()

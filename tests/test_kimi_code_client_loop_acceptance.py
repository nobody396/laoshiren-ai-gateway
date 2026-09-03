from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "kimi_code_client_loop_acceptance.py"


def load_module():
    spec = importlib.util.spec_from_file_location("kimi_code_acceptance_test", SCRIPT)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


MODULE = load_module()


class FakeController:
    def __init__(self):
        self.switches: list[int] = []

    def owned_key(self):
        return {"id": 128, "user_id": 2, "group_id": 6, "key": "test-secret-value"}

    def switch_group(self, group_id: int):
        self.switches.append(group_id)
        return {"id": 128, "user_id": 2, "status": "active", "group_id": group_id}

    def discover(self, _base_url: str, _key: str, _timeout: int):
        return {"http_status": 200, "model_count": 1, "model_ids": ["model-test"]}

    def admin_cli_json(self, _args):
        return {"data": {"items": []}}


class KimiCodeAcceptanceTest(unittest.TestCase):
    def test_requires_both_live_switches_before_controller_load(self):
        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(
            MODULE, "load_controller",
        ) as controller:
            with self.assertRaisesRegex(SystemExit, "requires --execute"):
                MODULE.main([
                    "--target", "model-test:7:responses",
                    "--output-dir", temporary,
                    "--execute",
                ])
            controller.assert_not_called()

    def test_target_and_protocol_contract(self):
        self.assertEqual(
            MODULE.parse_target("model-test:57:generate_content"),
            ("model-test", 57, "generate_content"),
        )

    def test_exact_client_version_gate(self):
        def exact(*_args, **_kwargs):
            return subprocess.CompletedProcess([], 0, b"0.38.0\n", b"")

        def drifted(*_args, **_kwargs):
            return subprocess.CompletedProcess([], 0, b"0.39.1\n", b"")

        self.assertEqual(MODULE.executable_version(exact), "0.38.0")
        with self.assertRaisesRegex(RuntimeError, "requires 0.38.0"):
            MODULE.executable_version(drifted)
        with self.assertRaisesRegex(ValueError, "invalid target"):
            MODULE.parse_target("model-test:57:images")
        with self.assertRaisesRegex(ValueError, "invalid target"):
            MODULE.parse_target("model/test:57:responses")
        with self.assertRaisesRegex(ValueError, "invalid target"):
            MODULE.parse_target("model-test:0:responses")
        self.assertEqual(
            {key: value["provider_type"] for key, value in MODULE.PROTOCOL_CONFIG.items()},
            {
                "responses": "openai_responses",
                "chat_completions": "openai",
                "messages": "anthropic",
                "generate_content": "google-genai",
            },
        )

    def test_ephemeral_config_never_contains_key_and_protocol_is_explicit_env(self):
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary) / "home"
            home.mkdir()
            config = MODULE.write_ephemeral_config(home)
            secret = "key-that-must-stay-in-env"
            self.assertEqual(MODULE.secret_file_matches(Path(temporary), secret), [])
            self.assertEqual(stat.S_IMODE(config.stat().st_mode), 0o600)
            for protocol, contract in MODULE.PROTOCOL_CONFIG.items():
                env = MODULE.runtime_environment(
                    home=home, key=secret, model_id="model-test", protocol=protocol,
                    context_size=262144, max_output_tokens=131072,
                    source={
                        "PATH": "/bin", "LANG": "C",
                        "UNRELATED_API_KEY": "must-not-forward",
                        "AUTH_TOKEN": "must-not-forward",
                    },
                )
                self.assertEqual(env["KIMI_MODEL_API_KEY"], secret)
                self.assertEqual(env["KIMI_MODEL_PROVIDER_TYPE"], contract["provider_type"])
                self.assertEqual(env["KIMI_MODEL_BASE_URL"], contract["base_url"])
                self.assertEqual(env["PATH"], "/bin")
                self.assertEqual(env["LANG"], "C")
                self.assertNotIn("UNRELATED_API_KEY", env)
                self.assertNotIn("AUTH_TOKEN", env)
                self.assertEqual(MODULE.secret_file_matches(Path(temporary), secret), [])

    def test_secret_file_match_reports_metadata_not_secret(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            secret = "sensitive-test-value"
            path = root / "config.toml"
            path.write_text(f'api_key = "{secret}"\n')
            matches = MODULE.secret_file_matches(root, secret)
            self.assertEqual(matches[0]["path"], "config.toml")
            self.assertNotIn(secret, json.dumps(matches))

    def test_parse_events_requires_non_user_file_marker_and_assistant_final(self):
        file_marker = "FILE_MARKER_123"
        final_marker = "FINAL_MARKER_456"
        raw = b"\n".join([
            json.dumps({"role": "user", "type": "message", "content": final_marker}).encode(),
            json.dumps({"role": "assistant", "type": "tool_call", "name": "read_file"}).encode(),
            json.dumps({"role": "tool", "type": "tool_result", "content": file_marker}).encode(),
            json.dumps({"role": "assistant", "type": "tool_call", "name": "shell"}).encode(),
            json.dumps({"role": "tool", "type": "tool_result", "content": "ok"}).encode(),
            json.dumps({"role": "assistant", "type": "result", "content": final_marker}).encode(),
        ])
        parsed = MODULE.parse_events(
            raw, file_marker=file_marker, final_marker=final_marker,
        )
        self.assertTrue(parsed["tool_use_observed"])
        self.assertTrue(parsed["tool_result_observed"])
        self.assertTrue(parsed["file_marker_observed_after_tool"])
        self.assertTrue(parsed["final_marker_verified"])

    def test_usage_attribution_requires_two_exact_protocol_rows(self):
        expected = "/v1/responses"
        rows = [
            {
                "id": value, "api_key_id": 128, "group_id": 7,
                "model": "model-test", "requested_model": "model-test",
                "inbound_endpoint": expected, "upstream_endpoint": expected,
                "user_agent": "kimi-code/0.38.0", "input_tokens": 10,
                "output_tokens": 3, "created_at": f"2026-09-01T00:00:0{value}Z",
            }
            for value in (2, 3)
        ]
        controller = mock.Mock()
        controller.admin_cli_json.return_value = {"data": {"items": rows}}
        _, selected, verified = MODULE.attributed_usage(
            controller, model_id="model-test", group_id=7, protocol="responses",
            before_ids={1}, attempts=1, sleep=lambda _seconds: None,
        )
        self.assertTrue(verified)
        self.assertEqual([row["id"] for row in selected], [2, 3])

    def test_atomic_receipt_is_mode_0600_and_self_authenticating(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "receipt.json"
            MODULE.atomic_json(path, {"schema_version": 1, "passed": False})
            value = json.loads(path.read_text())
            digest = value.pop("artifact_sha256")
            self.assertEqual(digest, MODULE.canonical_sha(value))
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            with self.assertRaisesRegex(RuntimeError, "secret-shaped"):
                MODULE.atomic_json(path, {"value": "sk-" + "not-allowed-123456"})

    def test_run_case_refuses_plaintext_persistence_and_cleans_home(self):
        controller = FakeController()
        created_roots = set(Path(tempfile.gettempdir()).glob("kimi-code-matrix-loop-*"))

        def runner(command, *, cwd, env, **_kwargs):
            secret = env["KIMI_MODEL_API_KEY"]
            leak = Path(env["KIMI_CODE_HOME"]) / "provider.json"
            leak.write_text(json.dumps({"api_key": secret}))
            prompt = command[command.index("-p") + 1]
            shell_marker = re.search(r"KIMI_MATRIX_FILE_[0-9a-f]+", prompt).group(0)
            final_marker = re.search(r"KIMI_MATRIX_AGENT_OK_[0-9a-f]+", prompt).group(0)
            (Path(cwd) / "result.txt").write_text(shell_marker + "\n")
            events = [
                {"role": "assistant", "type": "tool_call", "name": "read_file"},
                {"role": "tool", "type": "tool_result", "content": "fixture"},
                {"role": "assistant", "type": "tool_call", "name": "shell"},
                {"role": "tool", "type": "tool_result", "content": "ok"},
                {"role": "assistant", "type": "result", "content": final_marker},
            ]
            return subprocess.CompletedProcess(
                command, 0, "\n".join(json.dumps(row) for row in events).encode(),
                secret.encode(),
            )

        with mock.patch.object(
            MODULE, "attributed_usage",
            return_value=("/admin/usage", [{"id": 1}, {"id": 2}], True),
        ):
            receipt, policy_blocked = MODULE.run_case(
                controller, "test-secret-value", model_id="model-test", group_id=7,
                protocol="responses", timeout=1, context_size=100,
                max_output_tokens=20, runner=runner,
            )
        self.assertTrue(policy_blocked)
        self.assertFalse(receipt["passed"])
        self.assertEqual(receipt["credential_policy"]["blocker"], MODULE.POLICY_BLOCKER)
        self.assertTrue(receipt["credential_policy"]["plaintext_stream_echo_detected"])
        self.assertNotIn("test-secret-value", json.dumps(receipt))
        remaining = set(Path(tempfile.gettempdir()).glob("kimi-code-matrix-loop-*"))
        self.assertEqual(remaining, created_roots)

    def test_main_restores_group_six_and_writes_failure_receipt(self):
        controller = FakeController()
        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(
            MODULE, "executable_version", return_value="0.38.0",
        ), mock.patch.object(
            MODULE, "load_controller", return_value=controller,
        ), mock.patch.object(
            MODULE, "run_case", side_effect=RuntimeError("offline simulated failure"),
        ):
            result = MODULE.main([
                "--target", "model-test:7:chat_completions",
                "--output-dir", temporary,
                "--execute", "--acknowledge-paid-probes",
            ])
            self.assertEqual(result, 2)
            self.assertEqual(controller.switches[-1], 6)
            receipts = list(Path(temporary).glob("*.json"))
            self.assertEqual(len(receipts), 1)
            receipt = json.loads(receipts[0].read_text())
            self.assertEqual(receipt["restored_group_id"], 6)
            self.assertFalse(receipt["passed"])

    def test_main_policy_blocker_returns_three_and_restores_group_six(self):
        controller = FakeController()
        blocked = {
            "schema_version": 1,
            "kind": "real_client_loop",
            "secret_free": True,
            "model_id": "model-test",
            "protocol": "responses",
            "group_id": 7,
            "credential_policy": {"blocker": MODULE.POLICY_BLOCKER},
            "passed": False,
        }
        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(
            MODULE, "executable_version", return_value="0.38.0",
        ), mock.patch.object(
            MODULE, "load_controller", return_value=controller,
        ), mock.patch.object(
            MODULE, "run_case", return_value=(blocked, True),
        ):
            result = MODULE.main([
                "--target", "model-test:7:responses",
                "--output-dir", temporary,
                "--execute", "--acknowledge-paid-probes",
            ])
            self.assertEqual(result, 3)
            self.assertEqual(controller.switches[-1], 6)
            receipt = json.loads(next(Path(temporary).glob("*.json")).read_text())
            self.assertEqual(receipt["credential_policy"]["blocker"], MODULE.POLICY_BLOCKER)
            self.assertEqual(receipt["restored_group_id"], 6)


if __name__ == "__main__":
    unittest.main()

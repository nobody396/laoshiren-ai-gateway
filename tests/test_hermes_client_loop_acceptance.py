from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

from scripts.model_evidence_receipt_import import client_loop


SCRIPT = Path(__file__).parents[1] / "scripts" / "hermes_client_loop_acceptance.py"
SPEC = importlib.util.spec_from_file_location("hermes_client_loop_acceptance", SCRIPT)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class FakeController:
    def __init__(self) -> None:
        self.switches: list[int] = []

    def owned_key(self) -> dict:
        return {"id": 128, "user_id": 2, "group_id": 6, "key": "unit-secret-value"}

    def switch_group(self, group_id: int) -> dict:
        self.switches.append(group_id)
        return {"group_id": group_id}

    def admin_cli_json(self, _args: list[str]) -> dict:
        return {"data": {"items": []}}


class HermesClientLoopAcceptanceTests(unittest.TestCase):
    def test_double_live_gate_precedes_controller_access(self) -> None:
        base = ["--model", "gpt-5.6-sol", "--group-id", "6", "--output", "/tmp/x.json"]
        for switches in ([], ["--execute"], ["--acknowledge-paid-probes"]):
            with self.subTest(switches=switches), mock.patch.object(MODULE, "load_controller") as load:
                with self.assertRaisesRegex(SystemExit, "requires --execute and --acknowledge"):
                    MODULE.main(base + switches)
                load.assert_not_called()

    def test_child_environment_isolated_and_does_not_mutate_parent(self) -> None:
        before = os.environ.get("OPENAI_API_KEY")
        with tempfile.TemporaryDirectory() as temporary:
            env = MODULE.child_environment(Path(temporary), "unit-secret-value")
        self.assertEqual(env["OPENAI_API_KEY"], "unit-secret-value")
        self.assertEqual(env["OPENAI_BASE_URL"], MODULE.BASE_URL)
        self.assertEqual(os.environ.get("OPENAI_API_KEY"), before)

    def test_atomic_receipt_embeds_hash_and_no_secret(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "receipt.json"
            MODULE.atomic_receipt(path, {"kind": "test", "secret_free": True})
            loaded = json.loads(path.read_text())
            embedded = loaded.pop("artifact_sha256")
            self.assertEqual(embedded, MODULE.canonical_sha(loaded))
            self.assertNotIn("unit-secret-value", path.read_text())
            self.assertFalse(any(path.parent.glob(f".{path.name}.*.tmp")))

    def test_finally_restores_group_six_and_receipt_stays_secret_free(self) -> None:
        controller = FakeController()
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "receipt.json"
            argv = [
                "--model", "gpt-5.6-sol", "--group-id", "7", "--output", str(output),
                "--execute", "--acknowledge-paid-probes",
            ]
            with (
                mock.patch.object(MODULE, "load_controller", return_value=controller),
                mock.patch.object(MODULE, "execute_child", side_effect=RuntimeError("synthetic")),
                mock.patch.object(MODULE.time, "sleep", return_value=None),
            ):
                self.assertEqual(MODULE.main(argv), 2)
            receipt = json.loads(output.read_text())
            imported = client_loop(output)
        self.assertEqual(controller.switches, [7, 6])
        self.assertEqual(receipt["restoration"], {"group_id": 6, "verified": True})
        self.assertEqual(receipt["restored_group_id"], 6)
        self.assertFalse(receipt["tool_use_observed"])
        self.assertFalse(receipt["tool_result_observed"])
        self.assertEqual(len(imported), 2)
        self.assertEqual(receipt["run_error_type"], "RuntimeError")
        self.assertFalse(receipt["passed"])
        self.assertNotIn("unit-secret-value", json.dumps(receipt))

    def test_usage_attribution_requires_new_exact_responses_rows(self) -> None:
        rows = [
            {
                "id": 2, "api_key_id": 128, "group_id": 7,
                "model": "gpt-5.6-sol", "inbound_endpoint": "/v1/responses",
                "upstream_endpoint": "/v1/responses", "user_agent": "OpenAI/Python 2.24.0",
                "input_tokens": 20, "output_tokens": 5,
            },
            {
                "id": 3, "api_key_id": 128, "group_id": 7,
                "model": "gpt-5.6-sol", "inbound_endpoint": "/v1/chat/completions",
                "upstream_endpoint": "/v1/chat/completions", "input_tokens": 20, "output_tokens": 5,
            },
        ]
        accepted = MODULE.accepted_usage_rows(
            rows, baseline_ids={1}, model_id="gpt-5.6-sol", group_id=7,
        )
        self.assertEqual([row["id"] for row in accepted], [2])


if __name__ == "__main__":
    unittest.main()

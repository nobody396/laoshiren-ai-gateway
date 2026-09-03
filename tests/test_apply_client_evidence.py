from __future__ import annotations

import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "apply_client_evidence.py"
SPEC = importlib.util.spec_from_file_location("apply_client_evidence", SCRIPT)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def evidence(status: str = "blocked") -> dict:
    return {
        "status": status,
        "observed_at": "2026-08-30",
        "source_ref": "fixture://legacy",
        **({"artifact_sha256": "0" * 64} if status in {"verified", "unsupported"} else {}),
    }


def matrix() -> dict:
    features = {
        "minimal_text": "unverified",
        "streaming_terminal": "unverified",
        "tool_call": "unverified",
        "tool_result_continuation": "unverified",
        "usage": "unverified",
        "invalid_request": "unverified",
    }
    os_rows = []
    for os_id in ("macos", "linux", "windows"):
        os_rows.append({
            "os": os_id,
            "support": "documented",
            "config_files": [{"path": f"~/.fixture/{os_id}.json"}],
            "evidence": evidence(),
        })
    return {
        "schema_version": 2,
        "kind": "client_matrix",
        "clients": [{
            "id": "fixture-client",
            "name": "Fixture Client",
            "client_protocol": {
                "protocols": [
                    {"protocol": "responses", "support": "unknown",
                     "client_transport_features": dict(features), "evidence": evidence()},
                    {"protocol": "chat_completions", "support": "unknown",
                     "client_transport_features": dict(features), "evidence": evidence()},
                ]
            },
            "client_reasoning": {
                "control_kind": "fixed",
                "level_control": {"values": ["low", "high"]},
                "evidence": evidence(),
            },
            "client_config_os": {
                "release": {"version_key": "cli:1.0.0", "display": "1.0.0"},
                "os_support": os_rows,
            },
        }],
    }


def receipt(
    evidence_id: str, *, evidence_type: str, feature: str,
    result: str = "pass", protocol: str | None = "responses",
    os_id: str = "macos", version: str = "1.0.0",
    observed_at: str = "2026-08-31T10:00:00+00:00",
    reasoning_level: str | None = None, digest: str = "a" * 64,
    artifact_uri: str = "artifact.json",
) -> dict:
    target = {
        "client_id": "fixture-client", "client_version": version,
        "os": os_id, "feature": feature,
    }
    if protocol is not None:
        target["protocol"] = protocol
    if reasoning_level is not None:
        target["reasoning_level"] = reasoning_level
    return {
        "evidence_id": evidence_id,
        "evidence_type": evidence_type,
        "target": target,
        "result": result,
        "observed_at": observed_at,
        "artifact_uri": artifact_uri,
        "artifact_sha256": digest,
        "versions": {"client": version, "os": os_id},
        "summary": "secret-free fixture receipt",
        "secret_free": True,
    }


def index(*rows: dict) -> dict:
    return {"schema_version": 1, "kind": "test_evidence_matrix", "rows": list(rows)}


class ApplyClientEvidenceTest(unittest.TestCase):
    def client(self, projected):
        return projected["clients"][0]

    def protocol(self, projected, protocol):
        return next(
            row for row in self.client(projected)["client_protocol"]["protocols"]
            if row["protocol"] == protocol
        )

    def test_terminal_provider_negative_is_valid_but_cannot_touch_client_matrix(self):
        provider = {
            "evidence_id": "provider-negative", "evidence_type": "live_protocol_probe",
            "target": {"model_id": "alpha", "protocol": "responses", "feature": "streaming_terminal"},
            "result": "unsupported", "observed_at": "2026-08-31T10:00:00+00:00",
            "artifact_uri": "artifact.json", "artifact_sha256": "a" * 64,
            "versions": {}, "summary": "terminal provider negative", "secret_free": True,
        }
        rows = MODULE.receipt_rows(index(provider), artifact_root=ROOT, verify_artifacts=False)
        projected, events = MODULE.project(matrix(), rows)
        self.assertEqual(events, [])
        self.assertEqual(projected, matrix())

    def test_newer_failure_on_another_model_does_not_revoke_protocol_presence(self):
        passing = receipt(
            "model-a-pass", evidence_type="real_client_loop", feature="agent_loop",
            observed_at="2026-08-31T10:00:00+00:00",
        )
        passing["target"]["model_id"] = "model-a"
        failing = receipt(
            "model-b-fail", evidence_type="real_client_loop", feature="agent_loop", result="fail",
            observed_at="2026-08-31T11:00:00+00:00",
        )
        failing["target"]["model_id"] = "model-b"
        presence = receipt(
            "model-a-loop", evidence_type="real_client_loop", feature="agent_loop",
            observed_at="2026-08-31T09:00:00+00:00",
        )
        presence["target"]["model_id"] = "model-a"
        rows = MODULE.receipt_rows(index(presence, passing, failing), artifact_root=ROOT, verify_artifacts=False)
        projected, _ = MODULE.project(matrix(), rows)
        responses = self.protocol(projected, "responses")
        self.assertEqual(responses["support"], "supported")
        self.assertEqual(responses["evidence"]["status"], "verified")

    def test_global_terminal_negative_marks_protocol_unsupported(self):
        negative = receipt(
            "global-unsupported", evidence_type="regression_test",
            feature="agent_loop", result="unsupported",
            observed_at="2026-09-01T00:00:00+00:00",
        )
        negative["target"].pop("model_id", None)
        rows = MODULE.receipt_rows(index(negative), artifact_root=ROOT, verify_artifacts=False)
        projected, events = MODULE.project(matrix(), rows)
        responses = self.protocol(projected, "responses")
        self.assertEqual(responses["support"], "unsupported")
        self.assertEqual(responses["evidence"]["status"], "unsupported")
        self.assertTrue(all(value == "not_applicable" for value in responses["client_transport_features"].values()))
        self.assertTrue(any(event["matrix"] == "client_protocol" for event in events))

    def test_real_loop_proves_only_exact_protocol_presence(self):
        rows = MODULE.receipt_rows(
            index(
                receipt("loop", evidence_type="real_client_loop", feature="agent_loop"),
                receipt("minimal", evidence_type="regression_test", feature="minimal_text"),
            ),
            artifact_root=ROOT, verify_artifacts=False,
        )
        projected, events = MODULE.project(matrix(), rows)
        responses = self.protocol(projected, "responses")
        chat = self.protocol(projected, "chat_completions")
        self.assertEqual(responses["support"], "supported")
        self.assertEqual(responses["evidence"]["status"], "verified")
        self.assertEqual(responses["client_transport_features"]["minimal_text"], "verified")
        self.assertEqual(responses["client_transport_features"]["tool_call"], "unverified")
        self.assertEqual(chat["support"], "unknown")
        self.assertEqual(self.client(projected)["client_reasoning"]["evidence"]["status"], "blocked")
        self.assertTrue(any(event["matrix"] == "client_protocol" for event in events))

    def test_loop_cannot_upgrade_config_or_another_os(self):
        rows = MODULE.receipt_rows(
            index(*[
                receipt(f"loop-{feature}", evidence_type="real_client_loop", feature=feature)
                for feature in MODULE.CONFIG_FEATURES
            ]),
            artifact_root=ROOT, verify_artifacts=False,
        )
        projected, _ = MODULE.project(matrix(), rows)
        os_rows = self.client(projected)["client_config_os"]["os_support"]
        self.assertTrue(all(row["evidence"]["status"] == "blocked" for row in os_rows))

    def test_model_protocol_probe_cannot_upgrade_any_client_matrix(self):
        row = receipt("probe", evidence_type="live_protocol_probe", feature="agent_loop")
        rows = MODULE.receipt_rows(index(row), artifact_root=ROOT, verify_artifacts=False)
        projected, events = MODULE.project(matrix(), rows)
        self.assertEqual(projected, matrix())
        self.assertEqual(events, [])

    def test_model_only_regression_is_valid_evidence_but_not_client_authority(self):
        row = receipt("model-regression", evidence_type="regression_test", feature="minimal_text")
        row["target"] = {
            "model_id": "model-a", "protocol": "responses", "feature": "minimal_text",
        }
        row["versions"] = {"provider_model": "model-a"}
        rows = MODULE.receipt_rows(index(row), artifact_root=ROOT, verify_artifacts=False)
        projected, events = MODULE.project(matrix(), rows)
        self.assertEqual(projected, matrix())
        self.assertEqual(events, [])

    def test_reasoning_requires_exact_level_and_transport(self):
        ignored = receipt(
            "ordinary-loop", evidence_type="real_client_loop", feature="agent_loop",
            reasoning_level="high",
        )
        wrong_level = receipt(
            "wrong-level", evidence_type="regression_test", feature="reasoning_transport",
            reasoning_level="ultra",
        )
        rows = MODULE.receipt_rows(index(ignored, wrong_level), artifact_root=ROOT, verify_artifacts=False)
        projected, _ = MODULE.project(matrix(), rows)
        self.assertEqual(self.client(projected)["client_reasoning"]["evidence"]["status"], "blocked")

        exact = receipt(
            "exact-reasoning", evidence_type="regression_test", feature="reasoning_transport",
            reasoning_level="high", observed_at="2026-08-31T11:00:00+00:00",
        )
        rows = MODULE.receipt_rows(index(ignored, wrong_level, exact), artifact_root=ROOT, verify_artifacts=False)
        projected, events = MODULE.project(matrix(), rows)
        self.assertEqual(self.client(projected)["client_reasoning"]["evidence"]["status"], "verified")
        reasoning_event = next(event for event in events if event["matrix"] == "client_reasoning")
        self.assertEqual(reasoning_event["protocol"], "responses")
        self.assertEqual(reasoning_event["reasoning_level"], "high")

    def test_reasoning_failure_on_other_model_does_not_erase_exact_transport_pass(self):
        passing = receipt(
            "reasoning-a-pass", evidence_type="regression_test",
            feature="reasoning_transport", reasoning_level="high",
            observed_at="2026-08-31T10:00:00+00:00",
        )
        passing["target"]["model_id"] = "model-a"
        failing = receipt(
            "reasoning-b-fail", evidence_type="regression_test",
            feature="reasoning_transport", reasoning_level="high", result="fail",
            observed_at="2026-08-31T11:00:00+00:00",
        )
        failing["target"]["model_id"] = "model-b"
        presence = receipt(
            "model-a-loop", evidence_type="real_client_loop", feature="agent_loop",
            observed_at="2026-08-31T09:00:00+00:00",
        )
        presence["target"]["model_id"] = "model-a"
        rows = MODULE.receipt_rows(index(presence, passing, failing), artifact_root=ROOT, verify_artifacts=False)
        projected, _ = MODULE.project(matrix(), rows)
        self.assertEqual(self.client(projected)["client_reasoning"]["evidence"]["status"], "verified")

    def test_config_requires_safe_write_backup_and_idempotency_on_exact_os(self):
        rows = [
            receipt(f"qa-{feature}", evidence_type="config_qa", feature=feature, protocol=None)
            for feature in MODULE.CONFIG_FEATURES
        ]
        parsed = MODULE.receipt_rows(index(*rows), artifact_root=ROOT, verify_artifacts=False)
        projected, events = MODULE.project(matrix(), parsed)
        os_rows = {row["os"]: row for row in self.client(projected)["client_config_os"]["os_support"]}
        self.assertEqual(os_rows["macos"]["support"], "verified")
        self.assertEqual(os_rows["macos"]["evidence"]["status"], "verified")
        self.assertEqual(os_rows["linux"]["evidence"]["status"], "blocked")
        self.assertEqual(os_rows["windows"]["evidence"]["status"], "blocked")
        self.assertEqual(len([event for event in events if event["matrix"] == "client_config_os"]), 1)

    def test_incomplete_config_receipt_cannot_upgrade(self):
        rows = MODULE.receipt_rows(
            index(receipt("safe", evidence_type="config_qa", feature="config_safe_write", protocol=None)),
            artifact_root=ROOT, verify_artifacts=False,
        )
        projected, events = MODULE.project(matrix(), rows)
        mac = self.client(projected)["client_config_os"]["os_support"][0]
        self.assertEqual(mac["support"], "documented")
        self.assertEqual(mac["evidence"]["status"], "blocked")
        self.assertFalse(any(event["matrix"] == "client_config_os" for event in events))

    def test_latest_exact_os_feature_result_wins(self):
        earlier = receipt(
            "earlier-pass", evidence_type="regression_test", feature="minimal_text",
            observed_at="2026-08-31T10:00:00+00:00",
        )
        later = receipt(
            "later-fail", evidence_type="regression_test", feature="minimal_text", result="fail",
            observed_at="2026-08-31T11:00:00+00:00",
        )
        rows = MODULE.receipt_rows(index(earlier, later), artifact_root=ROOT, verify_artifacts=False)
        projected, _ = MODULE.project(matrix(), rows)
        responses = self.protocol(projected, "responses")
        self.assertEqual(responses["client_transport_features"]["minimal_text"], "unverified")
        self.assertEqual(responses["evidence"]["status"], "blocked")

    def test_failure_of_one_feature_does_not_erase_other_feature_protocol_presence(self):
        rows = MODULE.receipt_rows(
            index(
                receipt(
                    "minimal-pass", evidence_type="regression_test", feature="minimal_text",
                    observed_at="2026-08-31T10:00:00+00:00",
                ),
                receipt(
                    "tool-fail", evidence_type="regression_test", feature="tool_call", result="fail",
                    observed_at="2026-08-31T11:00:00+00:00",
                ),
            ),
            artifact_root=ROOT, verify_artifacts=False,
        )
        projected, _ = MODULE.project(matrix(), rows)
        responses = self.protocol(projected, "responses")
        self.assertEqual(responses["support"], "supported")
        self.assertEqual(responses["evidence"]["status"], "verified")
        self.assertEqual(responses["client_transport_features"]["minimal_text"], "verified")
        self.assertEqual(responses["client_transport_features"]["tool_call"], "unverified")

    def test_version_and_os_mismatch_are_ignored(self):
        rows = MODULE.receipt_rows(
            index(
                receipt("wrong-version", evidence_type="real_client_loop", feature="agent_loop", version="1.0.1"),
                receipt("wrong-os", evidence_type="real_client_loop", feature="agent_loop", os_id="android"),
            ),
            artifact_root=ROOT, verify_artifacts=False,
        )
        projected, events = MODULE.project(matrix(), rows)
        self.assertEqual(projected, matrix())
        self.assertEqual(events, [])

    def test_conflicting_latest_receipts_fail_closed(self):
        rows = MODULE.receipt_rows(
            index(
                receipt("pass", evidence_type="regression_test", feature="minimal_text", result="pass"),
                receipt("fail", evidence_type="regression_test", feature="minimal_text", result="fail"),
            ),
            artifact_root=ROOT, verify_artifacts=False,
        )
        with self.assertRaisesRegex(ValueError, "conflicting latest receipts"):
            MODULE.project(matrix(), rows)

    def test_plan_apply_are_atomic_backed_up_and_idempotent(self):
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            matrix_path = tmp / "client-matrix.json"
            evidence_path = tmp / "evidence.json"
            artifact_path = tmp / "artifact.json"
            report_path = tmp / "report.json"
            backup_dir = tmp / "backups"
            artifact_path.write_text('{"secret_free":true}\n', encoding="utf-8")
            digest = hashlib.sha256(artifact_path.read_bytes()).hexdigest()
            row = receipt(
                "loop", evidence_type="real_client_loop", feature="agent_loop",
                digest=digest, artifact_uri="artifact.json",
            )
            matrix_path.write_text(json.dumps(matrix()), encoding="utf-8")
            evidence_path.write_text(json.dumps(index(row)), encoding="utf-8")
            original = hashlib.sha256(matrix_path.read_bytes()).hexdigest()
            base = [
                sys.executable, str(SCRIPT), "plan", "--client-matrix", str(matrix_path),
                "--evidence", str(evidence_path), "--artifact-root", str(tmp),
                "--backup-dir", str(backup_dir), "--report", str(report_path),
            ]
            planned = subprocess.run(base, text=True, capture_output=True, check=False)
            self.assertEqual(planned.returncode, 0, planned.stderr)
            self.assertEqual(hashlib.sha256(matrix_path.read_bytes()).hexdigest(), original)
            self.assertFalse(backup_dir.exists())

            base[2] = "apply"
            applied = subprocess.run(base, text=True, capture_output=True, check=False)
            self.assertEqual(applied.returncode, 0, applied.stderr)
            after = hashlib.sha256(matrix_path.read_bytes()).hexdigest()
            self.assertNotEqual(after, original)
            self.assertEqual(len(list(backup_dir.glob("*.json"))), 1)
            self.assertTrue(json.loads(report_path.read_text())["client_matrix_write"])

            again = subprocess.run(base, text=True, capture_output=True, check=False)
            self.assertEqual(again.returncode, 0, again.stderr)
            self.assertEqual(hashlib.sha256(matrix_path.read_bytes()).hexdigest(), after)
            self.assertEqual(len(list(backup_dir.glob("*.json"))), 1)
            self.assertFalse(json.loads(report_path.read_text())["client_matrix_write"])

    def test_secret_shaped_receipt_is_rejected(self):
        bad = receipt("bad", evidence_type="real_client_loop", feature="agent_loop")
        bad["summary"] = "api_key=abcdefghijk"
        with self.assertRaisesRegex(ValueError, "secret-shaped"):
            MODULE.receipt_rows(index(bad), artifact_root=ROOT, verify_artifacts=False)

    def test_malformed_exact_client_receipt_is_rejected_not_ignored(self):
        bad = receipt("bad-shape", evidence_type="real_client_loop", feature="agent_loop")
        bad["target"].pop("os")
        bad["versions"].pop("os")
        with self.assertRaisesRegex(ValueError, "requires target.os"):
            MODULE.receipt_rows(index(bad), artifact_root=ROOT, verify_artifacts=False)


if __name__ == "__main__":
    unittest.main()

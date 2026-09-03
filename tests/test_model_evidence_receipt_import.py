import json
import hashlib
from pathlib import Path
import tempfile
import unittest

from scripts.model_evidence_receipt_import import (
    canonical_sha256, client_loop, group_discovery, group_smoke, immediately_after, merge, owned_e2e, provider_live,
)


def write_group_artifact(path: Path, results: list[dict], *, key_id: int = 205) -> Path:
    value = {
        "schema_version": 1,
        "kind": "owned_group_model_discovery",
        "dry_run": False,
        "production_write": True,
        "identity": {"user_id": 1, "key_id": key_id, "key_name": "owned matrix key"},
        "planned_groups": [],
        "results": results,
        "restored_group_id": 6,
        "secret_free": True,
        "all_passed": False,
    }
    value["artifact_sha256"] = canonical_sha256(value)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    return path


def group_result(*, probes: list[dict] | None = None) -> dict:
    return {
        "observed_at": "2026-09-01T00:00:00+00:00",
        "group": {"id": 6, "name": "GPT 标准线路"},
        "key_readback": {"id": 205, "user_id": 1, "status": "active", "group_id": 6},
        "discovery": {"http_status": 200, "error": None, "model_ids": ["alpha", "beta"]},
        "smoke_probes": probes or [],
    }


def probe(model: str, *, http_status: int = 200, completed: bool = True,
          passed: bool = True, error=None) -> dict:
    return {
        "model_id": model,
        "protocol": "responses",
        "http_status": http_status,
        "completed": completed,
        "passed": passed,
        "error": error,
        "body_sha256": hashlib.sha256(model.encode()).hexdigest(),
    }


class ModelEvidenceReceiptImportTests(unittest.TestCase):
    def test_derived_terminal_receipt_can_order_after_the_same_artifact_failure(self):
        self.assertEqual(
            "2026-09-01T00:00:00.000001Z",
            immediately_after("2026-09-01T00:00:00Z"),
        )

    def test_group_discovery_is_idempotent_and_not_capability_proof(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = root / "group.json"
            item = group_result()
            item["discovery"]["model_ids"] = ["alpha"]
            write_group_artifact(artifact, [item])
            rows = group_discovery(artifact)
            self.assertEqual(rows[0]["target"]["feature"], "model_discovery")
            self.assertIn("not capability proof", rows[0]["summary"])
            index = root / "index.json"
            self.assertEqual(len(merge(index, rows)["rows"]), 1)
            self.assertEqual(len(merge(index, rows)["rows"]), 1)

    def test_owned_e2e_failure_stays_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "e2e.json"
            artifact.write_text(json.dumps({
                "identity": {"group_id": 5}, "finished_at_utc": "2026-08-31T00:00:00+00:00",
                "results": [{"model": "claude-test", "http_status": 400, "completed": False, "passed": False}],
                "attribution": {"usage_rows": []},
            }))
            rows = owned_e2e(artifact, "messages")
            self.assertTrue(rows)
            self.assertEqual({item["result"] for item in rows}, {"fail"})

    def test_group_smoke_preserves_pass_and_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "smoke.json"
            write_group_artifact(artifact, [group_result(probes=[
                probe("alpha"),
                probe("beta", http_status=503, completed=False, passed=False,
                      error={"type": "overloaded_error"}),
            ])])
            rows = group_smoke(artifact)
            # two discovery rows plus route_call/minimal_text for two probes
            self.assertEqual(len(rows), 6)
            self.assertEqual({item["result"] for item in rows}, {"pass", "fail"})

    def test_http_200_without_terminal_is_a_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "empty.json"
            write_group_artifact(artifact, [group_result(probes=[
                probe("alpha", http_status=200, completed=False, passed=False),
            ])])
            rows = [
                row for row in group_smoke(artifact)
                if row["target"].get("feature") in {"route_call", "minimal_text"}
            ]
            self.assertEqual({row["result"] for row in rows}, {"fail"})
            self.assertTrue(all("completed=False" in row["summary"] for row in rows))

    def test_http_200_completed_but_explicit_empty_shape_is_a_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "empty-shape.json"
            item = probe("alpha")
            item["response_shape"] = {
                "output_count": 0, "choices_count": 0,
                "candidates_count": 0, "content_count": 0,
            }
            write_group_artifact(artifact, [group_result(probes=[item])])
            route_rows = [
                row for row in group_smoke(artifact)
                if row["target"].get("feature") == "route_call"
            ]
            self.assertEqual([row["result"] for row in route_rows], ["fail"])

    def test_key_205_artifact_is_supported_and_tampering_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "key205.json"
            write_group_artifact(artifact, [group_result(probes=[probe("alpha")])], key_id=205)
            self.assertTrue(group_smoke(artifact))
            value = json.loads(artifact.read_text())
            value["results"][0]["smoke_probes"][0]["completed"] = False
            artifact.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, "SHA-256"):
                group_smoke(artifact)

    def test_dry_run_and_secret_shaped_artifacts_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "unsafe.json"
            write_group_artifact(artifact, [group_result()])
            value = json.loads(artifact.read_text())
            value["dry_run"] = True
            value["artifact_sha256"] = canonical_sha256({k: v for k, v in value.items() if k != "artifact_sha256"})
            artifact.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, "dry-run"):
                group_discovery(artifact)

            value["dry_run"] = False
            value["debug"] = "Bearer abcdefghijklmnop"
            value["artifact_sha256"] = canonical_sha256({k: v for k, v in value.items() if k != "artifact_sha256"})
            artifact.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, "secret-shaped"):
                group_discovery(artifact)

    def test_client_loop_keeps_exact_client_dimensions(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "loop.json"
            artifact.write_text(json.dumps({
                "kind": "real_client_loop", "secret_free": True, "client_id": "hermes-agent",
                "client_version": "0.20.0", "os": "macos", "architecture": "arm64",
                "model_id": "alpha", "protocol": "responses", "observed_at": "2026-08-31T00:00:00+00:00",
                "exit_code": 0, "timed_out": False, "passed": True,
                "tool_use_observed": True, "tool_result_observed": True,
                "file_marker_verified": True, "final_marker_verified": True,
                "stdout_bytes": 42, "stdout_sha256": "a" * 64,
            }))
            rows = client_loop(artifact)
            self.assertEqual(rows[0]["evidence_type"], "real_client_loop")
            self.assertEqual(rows[0]["target"]["client_version"], "0.20.0")
            self.assertEqual(rows[0]["target"]["protocol"], "responses")
            self.assertEqual(rows[0]["result"], "pass")

    def test_client_loop_pass_flag_without_terminal_markers_stays_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "loop.json"
            artifact.write_text(json.dumps({
                "kind": "real_client_loop", "secret_free": True, "client_id": "claude-code",
                "client_version": "2.1.251", "os": "macos", "architecture": "arm64",
                "model_id": "alpha", "protocol": "messages", "observed_at": "2026-09-01T00:00:00Z",
                "exit_code": 0, "timed_out": False, "passed": True,
                "tool_use_observed": True, "tool_result_observed": True,
                "file_marker_verified": False, "final_marker_verified": False,
                "stdout_bytes": 42, "stdout_sha256": "a" * 64,
            }))
            self.assertEqual(client_loop(artifact)[0]["result"], "fail")

    def test_client_loop_links_exact_protocol_attribution(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            attribution = root / "attribution.json"
            attribution_value = {
                "schema_version": 1, "kind": "client_usage_attribution", "secret_free": True,
                "client_id": "claude-code", "client_version": "2.1.251", "group_id": 5,
                "observed_at": "2026-09-01T00:00:00Z",
                "rows": [{
                    "requested_model": "alpha", "model": "alpha",
                    "inbound_endpoint": "/v1/messages", "upstream_endpoint": "/v1/messages",
                    "input_tokens": 1, "output_tokens": 2,
                }],
            }
            attribution_value["artifact_sha256"] = canonical_sha256(attribution_value)
            attribution.write_text(json.dumps(attribution_value, ensure_ascii=False, indent=2) + "\n")
            attribution_digest = hashlib.sha256(attribution.read_bytes()).hexdigest()
            loop = root / "loop.json"
            loop.write_text(json.dumps({
                "kind": "real_client_loop", "secret_free": True, "client_id": "claude-code",
                "client_version": "2.1.251", "os": "macos", "architecture": "arm64",
                "model_id": "alpha", "protocol": "messages", "group_id": 5,
                "observed_at": "2026-09-01T00:00:00Z", "exit_code": 0, "timed_out": False,
                "tool_use_observed": True, "tool_result_observed": True,
                "file_marker_verified": True, "final_marker_verified": True,
                "stdout_bytes": 42, "stdout_sha256": "a" * 64, "passed": True,
                "protocol_attribution": {
                    "artifact_uri": str(attribution), "artifact_sha256": attribution_digest,
                    "usage_rows": 1, "verified": True,
                    "inbound_endpoint": "/v1/messages", "upstream_endpoint": "/v1/messages",
                },
            }))
            rows = client_loop(loop)
            self.assertEqual(len(rows), 2)
            self.assertEqual(rows[0]["result"], "pass")
            self.assertEqual(rows[1]["evidence_type"], "billing_reconciliation")
            self.assertEqual(rows[0]["evidence_refs"], [rows[1]["evidence_id"]])

    def test_generate_content_attribution_accepts_normalized_protocol_endpoint(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            attribution = root / "attribution.json"
            attribution_value = {
                "schema_version": 1, "kind": "client_usage_attribution", "secret_free": True,
                "client_id": "gemini-cli", "client_version": "0.57.0", "group_id": 57,
                "observed_at": "2026-09-01T00:00:00Z",
                "rows": [{
                    "model": "gemini-3.1-pro",
                    "inbound_endpoint": "/v1beta/models", "upstream_endpoint": "/v1beta/models",
                    "input_tokens": 10, "output_tokens": 2,
                }],
            }
            attribution_value["artifact_sha256"] = canonical_sha256(attribution_value)
            attribution.write_text(json.dumps(attribution_value, ensure_ascii=False, indent=2) + "\n")
            attribution_digest = hashlib.sha256(attribution.read_bytes()).hexdigest()
            loop = root / "loop.json"
            loop.write_text(json.dumps({
                "kind": "real_client_loop", "secret_free": True, "client_id": "gemini-cli",
                "client_version": "0.57.0", "os": "macos", "architecture": "arm64",
                "model_id": "gemini-3.1-pro", "protocol": "generate_content", "group_id": 57,
                "observed_at": "2026-09-01T00:00:00Z", "exit_code": 0, "timed_out": False,
                "tool_use_observed": True, "tool_result_observed": True,
                "file_marker_verified": True, "final_marker_verified": True,
                "stdout_bytes": 42, "stdout_sha256": "a" * 64, "passed": True,
                "protocol_attribution": {
                    "artifact_uri": str(attribution), "artifact_sha256": attribution_digest,
                    "usage_rows": 1, "verified": True,
                    "inbound_endpoint": "/v1beta/models", "upstream_endpoint": "/v1beta/models",
                },
            }))
            rows = client_loop(loop)
            self.assertEqual([row["result"] for row in rows], ["pass", "pass"])

    def test_chat_client_attribution_preserves_verified_responses_bridge(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            attribution = root / "attribution.json"
            attribution_value = {
                "schema_version": 1, "kind": "client_usage_attribution", "secret_free": True,
                "client_id": "opencode", "client_version": "cli:1.18.15", "group_id": 34,
                "observed_at": "2026-09-01T00:00:00Z",
                "rows": [{
                    "model": "grok-4.5",
                    "inbound_endpoint": "/v1/chat/completions", "upstream_endpoint": "/v1/responses",
                    "input_tokens": 10, "output_tokens": 2,
                }],
            }
            attribution_value["artifact_sha256"] = canonical_sha256(attribution_value)
            attribution.write_text(json.dumps(attribution_value, ensure_ascii=False, indent=2) + "\n")
            attribution_digest = hashlib.sha256(attribution.read_bytes()).hexdigest()
            loop = root / "loop.json"
            loop.write_text(json.dumps({
                "kind": "real_client_loop", "secret_free": True, "client_id": "opencode",
                "client_version": "cli:1.18.15", "os": "macos", "architecture": "arm64",
                "model_id": "grok-4.5", "protocol": "chat_completions", "group_id": 34,
                "observed_at": "2026-09-01T00:00:00Z", "exit_code": 0, "timed_out": False,
                "tool_use_observed": True, "tool_result_observed": True,
                "file_marker_verified": True, "final_marker_verified": True,
                "stdout_bytes": 42, "stdout_sha256": "a" * 64, "passed": True,
                "protocol_attribution": {
                    "artifact_uri": str(attribution), "artifact_sha256": attribution_digest,
                    "usage_rows": 1, "verified": True,
                    "inbound_endpoint": "/v1/chat/completions", "upstream_endpoint": "/v1/responses",
                },
            }))
            rows = client_loop(loop)
            self.assertEqual([row["result"] for row in rows], ["pass", "pass"])
            self.assertEqual(rows[1]["target"]["upstream_endpoint"], "/v1/responses")

    def test_live_provider_case_imports_exact_p_case_and_rejects_tampering(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "provider.json"
            value = {
                "schema_version": 1,
                "kind": "provider_contract_live_case",
                "secret_free": True,
                "case": {
                    "p_id": "P-05", "model_id": "gpt-test", "protocol": "responses",
                    "group_id": 6, "name": "tool_result_continuation",
                },
                "result": "pass",
                "classification": "verified",
                "offline_verifier": {"status": "passed", "reason": None},
                "response": {"http_status": 200},
                "observed_at": "2026-09-01T00:00:00Z",
            }
            value["artifact_sha256"] = canonical_sha256(value)
            artifact.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
            rows = provider_live(artifact)
            self.assertEqual(rows[0]["result"], "pass")
            self.assertEqual(rows[0]["target"]["feature"], "tool_result_continuation")
            self.assertEqual(rows[0]["target"]["group_id"], "6")
            self.assertEqual(rows[1]["target"]["feature"], "tool_call")
            self.assertEqual(rows[1]["result"], "pass")

            value["result"] = "blocked"
            artifact.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, "SHA-256"):
                provider_live(artifact)

    def test_provider_usage_case_emits_linked_billing_reconciliation(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "usage.json"
            value = {
                "schema_version": 1, "kind": "provider_contract_live_case", "secret_free": True,
                "case": {"p_id": "P-12", "model_id": "gpt-test", "protocol": "responses", "group_id": 6, "name": "usage"},
                "result": "pass", "classification": "verified",
                "offline_verifier": {"status": "passed", "reason": None},
                "response": {"http_status": 200},
                "billing_attribution": {"status": "verified", "usage_row_ids": ["1"], "actual_cost": 0.1, "total_cost": 0.2},
                "observed_at": "2026-09-01T00:00:00Z",
            }
            value["artifact_sha256"] = canonical_sha256(value)
            artifact.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
            rows = provider_live(artifact)
            self.assertEqual([item["target"]["feature"] for item in rows], ["usage", "billing"])
            self.assertNotIn("evidence_refs", rows[0])
            self.assertEqual(rows[1]["evidence_type"], "billing_reconciliation")
            self.assertEqual(rows[1]["result"], "pass")

            value["result"] = "blocked"
            value["classification"] = "contract_failed"
            value["offline_verifier"] = {"status": "failed", "reason": "usage mismatch"}
            value["finished_at"] = "2026-09-01T00:00:01Z"
            value["artifact_sha256"] = canonical_sha256({k: v for k, v in value.items() if k != "artifact_sha256"})
            artifact.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
            blocked_rows = provider_live(artifact)
            self.assertEqual(blocked_rows[0]["result"], "fail")
            self.assertEqual(blocked_rows[1]["result"], "pass")

    def test_error_passthrough_also_closes_the_protocol_invalid_request_check(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "invalid.json"
            value = {
                "schema_version": 1, "kind": "provider_contract_live_case", "secret_free": True,
                "case": {"p_id": "P-15", "model_id": "gpt-test", "protocol": "responses", "group_id": 6, "name": "error_passthrough"},
                "result": "pass", "classification": "verified",
                "offline_verifier": {"status": "passed", "reason": None},
                "response": {"http_status": 400}, "observed_at": "2026-09-01T00:00:00Z",
            }
            value["artifact_sha256"] = canonical_sha256(value)
            artifact.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
            rows = provider_live(artifact)
            self.assertEqual(
                [item["target"]["feature"] for item in rows],
                ["error_passthrough", "invalid_request"],
            )

    def test_deterministic_valid_capability_failure_emits_terminal_unsupported_receipt(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "unsupported.json"
            value = {
                "schema_version": 1, "kind": "provider_contract_live_case", "secret_free": True,
                "case": {"p_id": "P-04", "model_id": "chat-model", "protocol": "chat_completions", "group_id": 6, "name": "tool_call"},
                "result": "blocked", "classification": "contract_failed",
                "reason": "tool_call.name is required",
                "offline_verifier": {"status": "failed", "reason": "tool_call.name is required"},
                "response": {"http_status": 200, "complete": True},
                "observed_at": "2026-09-01T00:00:00Z", "finished_at": "2026-09-01T00:00:01Z",
            }
            value["artifact_sha256"] = canonical_sha256(value)
            artifact.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
            rows = provider_live(artifact)
            self.assertEqual([item["result"] for item in rows], ["fail", "unsupported"])
            self.assertEqual(rows[1]["target"]["component"], "terminal_negative_capability")

    def test_reasoning_transport_emits_exact_model_level_receipt(self):
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "reasoning.json"
            value = {
                "schema_version": 1, "kind": "provider_contract_live_case", "secret_free": True,
                "case": {"p_id": "P-06", "model_id": "kimi", "protocol": "chat_completions", "group_id": 62, "name": "reasoning_transport", "reasoning_level": "low"},
                "result": "pass", "classification": "verified",
                "offline_verifier": {"status": "passed", "reason": None},
                "response": {"http_status": 200}, "observed_at": "2026-09-01T00:00:00Z",
            }
            value["artifact_sha256"] = canonical_sha256(value)
            artifact.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
            rows = provider_live(artifact)
            self.assertEqual(rows[-1]["target"]["feature"], "reasoning_level")
            self.assertEqual(rows[-1]["target"]["level"], "low")


if __name__ == "__main__":
    unittest.main()

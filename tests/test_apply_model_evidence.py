import hashlib
from pathlib import Path
import tempfile
import unittest

from scripts.apply_model_evidence import (
    project_contract, receipt_rows, stable_evidence_id,
)


class ApplyModelEvidenceTests(unittest.TestCase):
    def test_group_requires_discovery_and_route_call(self):
        contract = {"model": {"id": "alpha"}, "test_matrix": {"group_access": {"Group A": {"status": "blocked", "evidence": "gap", "protocols": ["responses"]}}, "protocols": {}}}
        discovery = {"evidence_id": "e1", "result": "pass", "secret_free": True, "target": {"model_id": "alpha", "group_id": "6", "feature": "model_discovery"}}
        partial, _ = project_contract(contract, [discovery], {"Group A": "6"}, {})
        self.assertEqual(partial["test_matrix"]["group_access"]["Group A"]["status"], "blocked")
        route = {"evidence_id": "e2", "result": "pass", "secret_free": True, "target": {"model_id": "alpha", "group_id": "6", "feature": "route_call"}}
        route["target"]["protocol"] = "responses"
        minimal = {"evidence_id": "e3", "result": "pass", "secret_free": True, "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "minimal_text"}}
        complete, _ = project_contract(contract, [discovery, route, minimal], {"Group A": "6"}, {})
        self.assertEqual(complete["test_matrix"]["group_access"]["Group A"]["status"], "verified")
        self.assertEqual(complete["test_matrix"]["group_access"]["Group A"]["protocol_evidence"]["responses"]["status"], "verified")

    def test_group_is_verified_per_protocol_and_missing_protocol_stays_blocked(self):
        contract = {"model": {"id": "alpha"}, "test_matrix": {"group_access": {"Group A": {"status": "verified", "evidence": "old", "protocols": ["responses", "chat_completions"]}}, "protocols": {}}}
        rows = [
            {"evidence_id": "d", "result": "pass", "target": {"model_id": "alpha", "group_id": "6", "feature": "model_discovery"}},
            {"evidence_id": "r", "result": "pass", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "route_call"}},
            {"evidence_id": "m", "result": "pass", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "minimal_text"}},
        ]
        projected, _ = project_contract(contract, rows, {"Group A": "6"}, {})
        cell = projected["test_matrix"]["group_access"]["Group A"]
        self.assertEqual(cell["status"], "blocked")
        self.assertEqual(cell["protocol_evidence"]["responses"]["status"], "verified")
        self.assertEqual(cell["protocol_evidence"]["chat_completions"]["status"], "blocked")

    def test_group_is_unsupported_when_route_is_terminally_unsupported(self):
        contract = {"model": {"id": "alpha"}, "test_matrix": {"group_access": {"Group A": {"status": "blocked", "evidence": "gap", "protocols": ["responses"]}}, "protocols": {}}}
        rows = [
            {"evidence_id": "d", "result": "pass", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "feature": "model_discovery"}},
            {"evidence_id": "r", "result": "unsupported", "observed_at": "2026-09-01T00:01:00Z", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "route_call"}},
            {"evidence_id": "m", "result": "unsupported", "observed_at": "2026-09-01T00:01:00Z", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "minimal_text"}},
        ]
        projected, _ = project_contract(contract, rows, {"Group A": "6"}, {})
        cell = projected["test_matrix"]["group_access"]["Group A"]
        self.assertEqual(cell["status"], "unsupported")
        self.assertEqual(cell["protocol_evidence"]["responses"]["status"], "unsupported")

    def test_latest_transient_failure_is_preserved_as_blocked_not_unsupported(self):
        contract = {"model": {"id": "alpha"}, "test_matrix": {"group_access": {"Group A": {"status": "verified", "evidence": "old", "protocols": ["responses"]}}, "protocols": {}}}
        rows = [
            {"evidence_id": "d", "result": "pass", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "feature": "model_discovery"}},
            {"evidence_id": "r1", "result": "pass", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "route_call"}},
            {"evidence_id": "m1", "result": "pass", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "minimal_text"}},
            {"evidence_id": "r2", "result": "fail", "observed_at": "2026-09-01T01:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "route_call"}},
            {"evidence_id": "m2", "result": "fail", "observed_at": "2026-09-01T01:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "minimal_text"}},
        ]
        projected, _ = project_contract(contract, rows, {"Group A": "6"}, {})
        cell = projected["test_matrix"]["group_access"]["Group A"]
        self.assertEqual(cell["status"], "blocked")
        self.assertEqual(cell["protocol_evidence"]["responses"]["status"], "blocked")
        self.assertEqual(cell["protocol_evidence"]["responses"]["failure_evidence_ids"], ["m2", "r2"])

    def test_protocol_latest_failure_blocks_a_legacy_verified_cell(self):
        contract = {"model": {"id": "alpha"}, "test_matrix": {"group_access": {}, "protocols": {"responses": {"minimal_text": {"status": "blocked", "evidence": "gap"}, "usage": {"status": "verified", "evidence": "old"}}}}}
        rows = [
            {"evidence_id": "e1", "result": "pass", "secret_free": True, "target": {"model_id": "alpha", "protocol": "responses", "feature": "minimal_text"}},
            {"evidence_id": "e2", "result": "fail", "secret_free": True, "target": {"model_id": "alpha", "protocol": "responses", "feature": "usage"}},
        ]
        projected, _ = project_contract(contract, rows, {}, {})
        self.assertEqual(projected["test_matrix"]["protocols"]["responses"]["minimal_text"]["status"], "verified")
        self.assertEqual(projected["test_matrix"]["protocols"]["responses"]["usage"]["status"], "blocked")
        self.assertEqual(projected["test_matrix"]["protocols"]["responses"]["usage"]["failure_evidence_ids"], ["e2"])

    def test_protocol_feature_and_tool_receipts_project_from_the_same_m8_source(self):
        contract = {
            "model": {"id": "alpha"},
            "test_matrix": {
                "group_access": {}, "protocols": {},
                "protocol_features": {"responses": {"error_passthrough": {"status": "blocked", "evidence": "gap"}}},
                "tools": {"responses": {"structured_output": {"status": "blocked", "evidence": "gap"}}},
            },
        }
        rows = [
            {"evidence_id": "e1", "result": "pass", "target": {"model_id": "alpha", "protocol": "responses", "feature": "error_passthrough"}},
            {"evidence_id": "e2", "result": "pass", "target": {"model_id": "alpha", "protocol": "responses", "feature": "structured_output"}},
        ]
        projected, _ = project_contract(contract, rows, {}, {})
        self.assertEqual(projected["test_matrix"]["protocol_features"]["responses"]["error_passthrough"]["status"], "verified")
        self.assertEqual(projected["test_matrix"]["tools"]["responses"]["structured_output"]["status"], "verified")

    def test_later_terminal_unsupported_receipt_closes_the_feature(self):
        contract = {"model": {"id": "alpha"}, "test_matrix": {"group_access": {}, "protocols": {"chat_completions": {"tool_call": {"status": "blocked", "evidence": "gap"}}}}}
        rows = [
            {"evidence_id": "fail", "result": "fail", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "protocol": "chat_completions", "feature": "tool_call"}},
            {"evidence_id": "unsupported", "result": "unsupported", "observed_at": "2026-09-01T00:00:01Z", "target": {"model_id": "alpha", "protocol": "chat_completions", "feature": "tool_call", "component": "terminal_negative_capability"}},
        ]
        projected, _ = project_contract(contract, rows, {}, {})
        self.assertEqual(projected["test_matrix"]["protocols"]["chat_completions"]["tool_call"]["status"], "unsupported")

    def test_semantic_not_applicable_is_not_overwritten_by_an_invalid_wire_probe(self):
        contract = {"model": {"id": "alpha"}, "test_matrix": {"group_access": {}, "protocols": {}, "protocol_features": {"chat_completions": {"reasoning": {"status": "not_applicable", "evidence": "always on"}}}}}
        rows = [{"evidence_id": "bad-wire", "result": "fail", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "protocol": "chat_completions", "feature": "reasoning"}}]
        projected, _ = project_contract(contract, rows, {}, {})
        self.assertEqual(projected["test_matrix"]["protocol_features"]["chat_completions"]["reasoning"]["status"], "not_applicable")

    def test_blocked_protocol_promotes_when_text_works_and_all_base_checks_are_terminal(self):
        checks = {
            "minimal_text": {"status": "verified", "evidence_ids": ["m"]},
            "streaming_terminal": {"status": "unsupported", "evidence_ids": ["s"]},
            "tool_call": {"status": "verified", "evidence_ids": ["t"]},
        }
        contract = {
            "model": {"id": "alpha"},
            "protocols": [{"name": "responses", "status": "blocked", "evidence": "gap"}],
            "test_matrix": {"group_access": {}, "protocols": {"responses": checks}},
        }
        projected, events = project_contract(contract, [], {}, {})
        self.assertEqual(projected["protocols"][0]["status"], "verified")
        self.assertEqual(projected["protocols"][0]["evidence_ids"], ["m", "s", "t"])
        self.assertTrue(any(event["path"] == "protocols/responses" for event in events))

    def test_candidate_protocol_checks_are_materialized_from_exact_receipts(self):
        contract = {
            "model": {"id": "alpha"},
            "protocols": [{"name": "responses", "status": "blocked", "evidence": "candidate"}],
            "test_matrix": {"group_access": {}, "protocols": {}},
        }
        rows = []
        for feature in (
            "minimal_text", "streaming_terminal", "tool_call",
            "tool_result_continuation", "usage", "invalid_request",
        ):
            rows.append({
                "evidence_id": feature,
                "result": "pass" if feature != "streaming_terminal" else "unsupported",
                "observed_at": "2026-09-01T00:00:01Z",
                "target": {"model_id": "alpha", "protocol": "responses", "feature": feature},
            })
        projected, _ = project_contract(contract, rows, {}, {})
        self.assertEqual(set(projected["test_matrix"]["protocols"]["responses"]), {
            "minimal_text", "streaming_terminal", "tool_call",
            "tool_result_continuation", "usage", "invalid_request",
        })
        self.assertEqual(projected["protocols"][0]["status"], "verified")

    def test_exact_reasoning_level_receipt_projects_model_reasoning_cell(self):
        contract = {"model": {"id": "kimi"}, "test_matrix": {"group_access": {}, "protocols": {}, "reasoning": {"model_levels": {"low": {"status": "blocked", "evidence": "gap"}}}}}
        rows = [{"evidence_id": "low-pass", "result": "pass", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "kimi", "protocol": "chat_completions", "feature": "reasoning_level", "level": "low"}}]
        projected, _ = project_contract(contract, rows, {}, {})
        self.assertEqual(projected["test_matrix"]["reasoning"]["model_levels"]["low"]["status"], "verified")

    def test_real_client_loop_projects_exact_cell_and_public_coverage(self):
        contract = {"model": {"id": "alpha"}, "clients": [], "client_coverage": [], "test_matrix": {"group_access": {}, "protocols": {}, "clients": {"Hermes Agent": {"responses": {"macos": {"client_version": "cli:1", "status": "blocked", "evidence": "gap"}}}}}}
        receipts = [{"evidence_id": "e1", "evidence_type": "real_client_loop", "result": "pass", "observed_at": "2026-08-31T00:00:00+00:00", "target": {"model_id": "alpha", "client_id": "hermes-agent", "client_version": "cli:1", "protocol": "responses", "os": "macos", "feature": "agent_loop"}}]
        clients = {"Hermes Agent": {"id": "hermes-agent", "verification_os": ["macos"], "client_protocol": {"protocols": [{"protocol": "responses", "support": "supported"}]}, "client_config_os": {"release": {"display": "1"}}}}
        projected, _ = project_contract(contract, receipts, {}, clients)
        self.assertEqual(projected["test_matrix"]["clients"]["Hermes Agent"]["responses"]["macos"]["status"], "verified")
        self.assertEqual(projected["client_coverage"][0]["status"], "verified")
        self.assertEqual(projected["clients"][0]["protocol"], "responses")

    def test_display_version_receipt_matches_exact_canonical_version_key(self):
        contract = {"model": {"id": "alpha"}, "clients": [], "client_coverage": [], "test_matrix": {"group_access": {}, "protocols": {}, "clients": {"Claude Code": {"messages": {"macos": {"client_version": "cli:2.1.251", "status": "blocked", "evidence": "gap"}}}}}}
        receipts = [
            {"evidence_id": "old-fail", "evidence_type": "real_client_loop", "result": "fail", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "client_id": "claude-code", "client_version": "2.1.251", "protocol": "messages", "os": "macos", "feature": "agent_loop"}},
            {"evidence_id": "new-pass", "evidence_type": "real_client_loop", "result": "pass", "observed_at": "2026-09-01T01:00:00Z", "target": {"model_id": "alpha", "client_id": "claude-code", "client_version": "2.1.251", "protocol": "messages", "os": "macos", "feature": "agent_loop"}},
        ]
        clients = {"Claude Code": {"id": "claude-code", "verification_os": ["macos"], "client_protocol": {"protocols": [{"protocol": "messages", "support": "supported"}]}, "client_config_os": {"release": {"display": "2.1.251", "version_key": "cli:2.1.251"}}}}
        projected, _ = project_contract(contract, receipts, {}, clients)
        cell = projected["test_matrix"]["clients"]["Claude Code"]["messages"]["macos"]
        self.assertEqual(cell["status"], "verified")
        self.assertEqual(cell["failure_evidence_ids"], ["old-fail"])
        self.assertEqual(projected["client_coverage"][0]["status"], "verified")

    def test_unverified_coverage_is_removed_from_public_clients(self):
        contract = {"model": {"id": "alpha"}, "verification": {"gateway_e2e": True}, "clients": [{"name": "Hermes Agent"}], "client_coverage": [{"name": "Hermes Agent", "status": "verified", "protocols": ["responses"], "evidence": "legacy"}], "test_matrix": {"group_access": {}, "protocols": {}, "clients": {"Hermes Agent": {"responses": {"macos": {"client_version": "cli:1", "status": "blocked", "evidence": "gap"}}}}}}
        clients = {"Hermes Agent": {"id": "hermes-agent", "verification_os": ["macos"], "client_protocol": {"protocols": [{"protocol": "responses", "support": "supported"}]}, "client_config_os": {"release": {"display": "1"}}}}
        receipts = [{"evidence_id": "fail", "evidence_type": "real_client_loop", "result": "fail", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "client_id": "hermes-agent", "client_version": "cli:1", "protocol": "responses", "os": "macos", "feature": "agent_loop"}}]
        projected, _ = project_contract(contract, receipts, {}, clients)
        self.assertEqual(projected["client_coverage"][0]["status"], "unsupported")
        self.assertEqual(projected["clients"], [])
        self.assertFalse(projected["verification"]["gateway_e2e"])

    def test_receipt_artifact_sha_and_immutable_id_are_verified(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifact = root / "artifact.json"
            artifact.write_text('{"safe":true}\n')
            digest = hashlib.sha256(artifact.read_bytes()).hexdigest()
            target = {
                "model_id": "alpha", "group_id": "6",
                "protocol": "responses", "feature": "route_call",
            }
            evidence_id = stable_evidence_id("live_protocol_probe", target, digest)
            index = {"rows": [{
                "evidence_id": evidence_id,
                "evidence_type": "live_protocol_probe",
                "target": target,
                "result": "pass",
                "artifact_uri": "artifact.json",
                "artifact_sha256": digest,
                "secret_free": True,
            }]}
            self.assertEqual(
                receipt_rows(index, artifact_root=root, verify_artifacts=True)[0]["evidence_id"],
                evidence_id,
            )
            artifact.write_text('{"safe":false}\n')
            with self.assertRaisesRegex(ValueError, "SHA-256 mismatch"):
                receipt_rows(index, artifact_root=root, verify_artifacts=True)

    def test_projection_is_idempotent(self):
        contract = {"model": {"id": "alpha"}, "test_matrix": {"group_access": {"Group A": {"status": "blocked", "evidence": "gap", "protocols": ["responses"]}}, "protocols": {}}}
        rows = [
            {"evidence_id": "d", "result": "pass", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "feature": "model_discovery"}},
            {"evidence_id": "r", "result": "pass", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "route_call"}},
            {"evidence_id": "m", "result": "pass", "observed_at": "2026-09-01T00:00:00Z", "target": {"model_id": "alpha", "group_id": "6", "protocol": "responses", "feature": "minimal_text"}},
        ]
        once, first_events = project_contract(contract, rows, {"Group A": "6"}, {})
        twice, second_events = project_contract(once, rows, {"Group A": "6"}, {})
        self.assertTrue(first_events)
        self.assertEqual(second_events, [])
        self.assertEqual(twice, once)


if __name__ == "__main__":
    unittest.main()

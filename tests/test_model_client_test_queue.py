import hashlib
from pathlib import Path
import tempfile
import unittest

from scripts.model_client_test_queue import build_queue, receipt_matches


class ModelClientTestQueueTests(unittest.TestCase):
    def test_receipt_matching_fails_closed_on_missing_dimensions(self):
        receipt = {"target": {"model_id": "alpha", "group_id": "6", "feature": "model_discovery"}}
        reasoning = {"case_type": "model_reasoning", "target": {"model_id": "alpha", "reasoning_level": "high"}}
        group_without_id = {"case_type": "group_access", "target": {"model_id": "alpha", "group_name": "Group A"}}
        self.assertFalse(receipt_matches(reasoning, receipt))
        self.assertFalse(receipt_matches(group_without_id, receipt))

    def test_collapses_duplicate_cell_failures_and_keeps_runtime_gates(self):
        gaps = {"matrices": {
            "group_access": {"failures": [
                "alpha/group_access/Group A: status is blocked",
                "alpha/group_access/Group A: public access must be verified",
            ]},
            "model_price": {"failures": [
                "alpha/pricing/Group A/input_price: mismatch",
                "alpha/pricing/Group A/output_price: mismatch",
            ]},
            "test_evidence": {"failures": [
                "alpha/clients/Client A/responses/macos: missing receipt",
                "alpha/Client A: client status is not final",
            ]},
        }}
        client_matrix = {"clients": [{
            "id": "client-a", "name": "Client A",
            "client_protocol": {"protocols": [{"protocol": "responses", "support": "supported"}]},
            "client_config_os": {"release": {"version_key": "cli:1"}, "os_support": [{"os": "macos"}]},
        }]}
        scope = {"clients": [{"client_id": "client-a", "scope_status": "existing_contract"}, {
            "client_id": "client-b", "scope_status": "planned_identity_frozen_version_pending", "version_key": None,
        }]}
        inventory = {"evidence": [{
            "evidence_id": "candidate-1", "target": {"model_id": "alpha", "client_id": "Client A", "protocol": "responses", "os": "macos"},
            "proof": {"reusable_as_terminal_evidence": False},
        }]}
        result = build_queue(gaps, client_matrix, scope, inventory, [], {"rows": []}, {})
        keys = [row["case_key"] for row in result["queue"]]
        self.assertEqual(len(keys), len(set(keys)))
        self.assertIn("group_access:alpha/group_access/Group A", keys)
        self.assertIn("model_price:alpha/pricing/Group A", keys)
        self.assertIn("test_evidence:alpha/clients/Client A/responses/macos", keys)
        self.assertIn("client_protocol:client-a:cli:1:responses", keys)
        self.assertIn("client_config_os:client-a:cli:1:macos", keys)
        self.assertIn("client_identity:client-b", keys)
        self.assertEqual(result["derived_failure_count"], 1)
        self.assertEqual(result["terminal_reuse_count"], 0)
        self.assertLessEqual(result["execution_batch_count"], result["queue_count"])
        self.assertTrue(result["coverage_audit"]["complete"])
        self.assertEqual(result["coverage_audit"]["unmapped_actionable_failures"], [])

    def test_semantic_dedupe_prefers_runtime_gate_over_raw_client_audit(self):
        clients = []
        failures = []
        for index in range(17):
            client_id = f"client-{index}"
            name = f"Client {index}"
            clients.append({
                "id": client_id, "name": name,
                "client_protocol": {"protocols": [{"protocol": "responses", "support": "supported"}]},
                "client_config_os": {
                    "release": {"version_key": "cli:1"},
                    "os_support": [{"os": "macos"}, {"os": "linux"}, {"os": "windows"}],
                },
            })
            for os_id in ("macos", "linux", "windows"):
                failures.append(f"client-matrix/{name}/{os_id}: evidence is non-terminal (blocked)")
        result = build_queue(
            {"matrices": {"client_config_os": {"failures": failures}}},
            {"clients": clients}, {"clients": []}, {"evidence": []}, [], {"rows": []}, {},
        )
        config_rows = [row for row in result["queue"] if row["case_type"] == "config_qa"]
        self.assertEqual(len(config_rows), 17 * 3)
        semantic = [
            (row["target"]["client_id"], row["target"]["client_version"], row["target"]["os"])
            for row in config_rows
        ]
        self.assertEqual(len(semantic), len(set(semantic)))
        self.assertTrue(all(row["source"] == "client_matrix_runtime_gate" for row in config_rows))
        self.assertTrue(all(len(row["merged_case_keys"]) == 2 for row in config_rows))
        self.assertEqual(result["semantic_dedupe"]["duplicates_removed"], 17 * 3)
        self.assertGreater(result["semantic_dedupe"]["batch_work_items_before"], result["semantic_dedupe"]["batch_work_items_after"])
        self.assertEqual(result["coverage_audit"]["config_qa_target_count"], 51)
        self.assertEqual(result["coverage_audit"]["unique_config_qa_target_count"], 51)

    def test_semantic_dedupe_applies_across_provider_and_client_dimensions(self):
        gaps = {"matrices": {
            "model_protocol": {"failures": [
                "alpha/responses/minimal_text: blocked",
                "alpha/responses/minimal_text: missing evidence",
            ]},
            "client_protocol": {"failures": [
                "client-matrix/Client A/responses: blocked",
            ]},
            "test_evidence": {"failures": [
                "alpha/clients/Client A/responses/macos: blocked",
                "alpha/clients/Client A/responses/macos: version mismatch",
            ]},
        }}
        client_matrix = {"clients": [{
            "id": "client-a", "name": "Client A",
            "client_protocol": {"protocols": [{"protocol": "responses", "support": "supported"}]},
            "client_config_os": {"release": {"version_key": "cli:1"}, "os_support": [{"os": "macos"}]},
        }]}
        contract = {
            "model": {"id": "alpha"},
            "test_matrix": {
                "protocols": {"responses": {"minimal_text": {"status": "verified"}}},
                "clients": {"Client A": {"responses": {"macos": {"status": "verified", "client_version": "cli:1"}}}},
            },
        }
        result = build_queue(gaps, client_matrix, {"clients": []}, {"evidence": []}, [contract], {"rows": []}, {})
        provider = [row for row in result["queue"] if row["case_type"] == "provider_contract" and row["target"].get("test_case_id") == "minimal_text"]
        client_protocol = [row for row in result["queue"] if row["case_type"] == "client_protocol_runtime"]
        loops = [row for row in result["queue"] if row["case_type"] == "real_client_loop"]
        self.assertEqual(len(provider), 1)
        self.assertEqual(len(client_protocol), 1)
        self.assertEqual(len(loops), 1)
        self.assertGreaterEqual(result["semantic_dedupe"]["duplicates_removed"], 1)

    def test_exact_display_version_and_latest_pass_close_old_client_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            old_artifact = root / "old.json"
            new_artifact = root / "new.json"
            old_artifact.write_text("old")
            new_artifact.write_text("new")
            def receipt(evidence_id, result, observed_at, version, artifact):
                return {
                    "evidence_id": evidence_id, "evidence_type": "real_client_loop",
                    "result": result, "observed_at": observed_at, "secret_free": True,
                    "artifact_uri": str(artifact),
                    "artifact_sha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
                    "target": {
                        "model_id": "alpha", "client_id": "claude-code",
                        "client_version": version, "protocol": "messages", "os": "macos",
                        "feature": "agent_loop",
                    },
                }
            contract = {
                "model": {"id": "alpha"},
                "test_matrix": {"clients": {"Claude Code": {"messages": {"macos": {
                    "status": "verified", "client_version": "cli:2.1.251",
                }}}}},
            }
            evidence = {"rows": [
                receipt("old-fail", "fail", "2026-08-31T00:00:00Z", "cli:2.1.251", old_artifact),
                receipt("new-pass", "pass", "2026-09-01T00:00:00Z", "2.1.251", new_artifact),
            ]}
            result = build_queue(
                {"matrices": {}}, {"clients": []}, {"clients": []}, {"evidence": []},
                [contract], evidence, {},
            )
            self.assertFalse(any(item["case_type"] == "real_client_loop" for item in result["queue"]))
            self.assertEqual(result["shortest_path_policy"]["removed_redundant_cartesian_client_loops"], 1)

    def test_client_protocol_pass_is_not_revoked_by_another_models_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            good = root / "good.json"; good.write_text("good")
            bad = root / "bad.json"; bad.write_text("bad")
            def evidence(eid, model, result, observed, path):
                return {
                    "evidence_id": eid, "evidence_type": "real_client_loop", "result": result,
                    "observed_at": observed, "secret_free": True,
                    "artifact_uri": str(path), "artifact_sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                    "target": {"model_id": model, "client_id": "Codex", "client_version": "cli:1", "protocol": "responses", "os": "macos", "feature": "agent_loop"},
                }
            client_matrix = {"clients": [{
                "id": "codex", "name": "Codex",
                "client_protocol": {"protocols": [{"protocol": "responses", "support": "supported"}]},
                "client_config_os": {"release": {"version_key": "cli:1"}, "os_support": []},
            }]}
            result = build_queue(
                {"matrices": {}}, client_matrix, {"clients": []}, {"evidence": []}, [],
                {"rows": [
                    evidence("pass-alpha", "alpha", "pass", "2026-09-01T00:00:00Z", good),
                    evidence("fail-beta", "beta", "fail", "2026-09-01T01:00:00Z", bad),
                ]}, {},
            )
            row = next(item for item in result["queue"] if item["case_type"] == "client_protocol_runtime")
            self.assertEqual(row["status"], "satisfied")
            self.assertEqual(row["terminal_results"], ["pass"])
            self.assertEqual(row["latest_terminal_evidence_ids"], ["pass-alpha"])

    def test_blocked_provider_features_are_planned_instead_of_hidden(self):
        contract = {
            "model": {"id": "alpha"},
            "test_matrix": {"protocol_features": {"responses": {
                "prompt_cache": {"status": "blocked"},
                "timeout": {"status": "blocked"},
            }}},
        }
        result = build_queue(
            {"matrices": {}}, {"clients": []}, {"clients": []}, {"evidence": []},
            [contract], {"rows": []}, {},
        )
        rows = [row for row in result["queue"] if row["case_type"] == "provider_contract"]
        self.assertEqual({row["target"]["test_case_id"] for row in rows}, {"prompt_cache", "timeout"})
        self.assertTrue(all(row["status"] == "planned" for row in rows))

    def test_config_qa_requires_safe_write_backup_and_idempotency(self):
        matrix = {"clients": [{
            "id": "codex", "name": "Codex",
            "client_protocol": {"protocols": []},
            "client_config_os": {"release": {"version_key": "cli:1"}, "os_support": [{"os": "macos"}]},
        }]}
        result = build_queue(
            {"matrices": {}}, matrix, {"clients": []}, {"evidence": []}, [], {"rows": []}, {},
        )
        row = next(item for item in result["queue"] if item["case_type"] == "config_qa")
        self.assertEqual(row["required_features"], ["safe_write", "backup_recovery", "idempotency"])
        self.assertEqual(row["status"], "planned")


if __name__ == "__main__":
    unittest.main()

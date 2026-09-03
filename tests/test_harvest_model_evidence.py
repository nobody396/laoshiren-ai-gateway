import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).parents[1] / "scripts" / "harvest_model_evidence.py"
SPEC = importlib.util.spec_from_file_location("harvest_model_evidence", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class HarvestModelEvidenceTest(unittest.TestCase):
    def write_contract(self, root: Path) -> Path:
        contracts = root / "contracts"
        contracts.mkdir()
        contract = {
            "schema_version": 1,
            "model": {"id": "qwen-test-1"},
            "protocols": [
                {
                    "name": "responses",
                    "status": "verified",
                    "evidence": "2026-08-30 Responses stream returned response.completed",
                }
            ],
            "clients": [
                {
                    "name": "Codex",
                    "version": "0.151.0",
                    "protocol": "responses",
                    "status": "verified",
                    "evidence": "real tool-result loop completed and exited 0",
                }
            ],
            "test_matrix": {
                "protocols": {
                    "responses": {
                        "tool_result_continuation": {
                            "status": "verified",
                            "verified_at": "2026-08-30",
                            "evidence": "tool result continuation returned the exact marker",
                        }
                    }
                },
                "clients": {
                    "Codex": {
                        "responses": {
                            "macos": {
                                "status": "verified",
                                "evidence": "real shell loop completed",
                                "verified_at": "2026-08-31",
                                "model_id": "qwen-test-1",
                                "protocol": "responses",
                                "client_version": "0.151.0",
                                "os": "macos",
                            }
                        }
                    }
                },
            },
        }
        path = contracts / "qwen-test-1.json"
        path.write_text(json.dumps(contract), encoding="utf-8")
        # Non-contract JSON files in the same directory must be ignored.
        (contracts / "matrix-schema.json").write_text(json.dumps({"type": "object"}), encoding="utf-8")
        return contracts

    def test_contract_harvest_keeps_only_explicit_coordinates(self):
        with tempfile.TemporaryDirectory() as directory:
            contracts = self.write_contract(Path(directory))
            inventory = MODULE.build_inventory(contracts, [], [])
        self.assertEqual(inventory["kind"], "historical_model_evidence_inventory")
        self.assertRegex(inventory["artifact_id"], r"^evidence-inventory-[a-f0-9]{20}$")
        self.assertRegex(inventory["sha256"], r"^[a-f0-9]{64}$")
        self.assertEqual(inventory["summary"]["terminal_records"], 0)

        client_claim = next(
            item for item in inventory["evidence"]
            if item["target"].get("client_id") == "Codex" and item["summary"].startswith("real tool-result")
        )
        self.assertEqual(client_claim["target"]["client_version"], "0.151.0")
        self.assertNotIn("os", client_claim["target"])
        self.assertNotIn("observed_at", client_claim)
        self.assertFalse(client_claim["proof"]["exact_terminal_proof"])

        exact_cell = next(item for item in inventory["evidence"] if item["summary"] == "real shell loop completed")
        self.assertEqual(
            exact_cell["target"],
            {
                "model_id": "qwen-test-1",
                "protocol": "responses",
                "client_id": "Codex",
                "client_version": "0.151.0",
                "os": "macos",
            },
        )
        self.assertEqual(exact_cell["observed_at"], "2026-08-31")

    def test_log_redacts_credentials_and_uses_literal_mentions(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contracts = self.write_contract(root)
            log = root / "log.md"
            log.write_text(
                "- qwen-test-1 Responses Codex 0.151.0 on macOS passed 2026-08-31; "
                "Authorization: Bearer secret-value-12345 api_key=also-secret token=query-secret\n"
                "- qwen-test-1 and another model had a generic result\n",
                encoding="utf-8",
            )
            inventory = MODULE.build_inventory(contracts, [log], [])
        serialized = json.dumps(inventory)
        self.assertNotIn("secret-value-12345", serialized)
        self.assertNotIn("also-secret", serialized)
        self.assertNotIn("query-secret", serialized)
        self.assertIn("[REDACTED]", serialized)
        log_record = next(
            item for item in inventory["evidence"]
            if item["source_type"] == "log_md" and "Codex" in item["summary"]
        )
        self.assertEqual(
            log_record["target"],
            {
                "model_id": "qwen-test-1",
                "protocol": "responses",
                "client_id": "Codex",
                "client_version": "0.151.0",
                "os": "macos",
            },
        )
        self.assertEqual(log_record["observed_at"], "2026-08-31")
        self.assertGreater(inventory["summary"]["redactions"], 0)

    def test_rollout_summary_is_never_terminal_proof(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contracts = self.write_contract(root)
            summary = root / "rollout.md"
            summary.write_text(
                "- qwen-test-1 Responses Codex 0.151.0 macOS completed the tool loop and exited 0 on 2026-08-31.\n",
                encoding="utf-8",
            )
            inventory = MODULE.build_inventory(contracts, [], [summary])
        record = next(item for item in inventory["evidence"] if item["source_type"] == "rollout_summary")
        self.assertEqual(record["proof"]["classification"], "summary_only")
        self.assertFalse(record["proof"]["exact_terminal_proof"])
        self.assertFalse(record["proof"]["reusable_as_terminal_evidence"])
        source = next(item for item in inventory["sources"] if item["source_type"] == "rollout_summary")
        self.assertTrue(source["summary_only"])

    def test_ambiguous_summary_does_not_create_cartesian_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contracts = self.write_contract(root)
            second = json.loads((contracts / "qwen-test-1.json").read_text(encoding="utf-8"))
            second["model"]["id"] = "qwen-test-2"
            (contracts / "qwen-test-2.json").write_text(json.dumps(second), encoding="utf-8")
            log = root / "log.md"
            log.write_text("qwen-test-1 and qwen-test-2 Responses passed", encoding="utf-8")
            inventory = MODULE.build_inventory(contracts, [log], [])
        self.assertFalse(any(item["source_type"] == "log_md" for item in inventory["evidence"]))

    def test_write_is_byte_idempotent(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contracts = self.write_contract(root)
            output = root / "inventory.json"
            first = subprocess.run(
                [sys.executable, str(SCRIPT), "write", "--contracts", str(contracts), "--output", str(output)],
                check=False, capture_output=True, text=True,
            )
            self.assertEqual(first.returncode, 0, first.stderr)
            first_bytes = output.read_bytes()
            first_report = json.loads(first.stdout)
            self.assertTrue(first_report["changed"])
            second = subprocess.run(
                [sys.executable, str(SCRIPT), "write", "--contracts", str(contracts), "--output", str(output)],
                check=False, capture_output=True, text=True,
            )
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertEqual(output.read_bytes(), first_bytes)
            self.assertFalse(json.loads(second.stdout)["changed"])

    def test_plan_prints_machine_json_and_never_uses_network(self):
        with tempfile.TemporaryDirectory() as directory:
            contracts = self.write_contract(Path(directory))
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "plan", "--contracts", str(contracts)],
                check=False, capture_output=True, text=True,
            )
        self.assertEqual(result.returncode, 0, result.stderr)
        inventory = json.loads(result.stdout)
        self.assertEqual(inventory["network_execution"], "disabled")
        self.assertTrue(inventory["secret_free"])


if __name__ == "__main__":
    unittest.main()

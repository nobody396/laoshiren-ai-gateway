from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "model_matrix_backfill.py"
MATRIX_SCRIPT = ROOT / "scripts" / "model_doc_matrix.py"


def load_module(path: Path, name: str):
    spec = importlib.util.spec_from_file_location(name, path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


BACKFILL = load_module(SCRIPT, "model_matrix_backfill_test_module")
MATRIX = load_module(MATRIX_SCRIPT, "model_doc_matrix_backfill_test_auditor")


def client_matrix() -> dict:
    evidence = {
        "status": "verified",
        "observed_at": "2026-08-31",
        "source_ref": "fixture.json",
        "artifact_sha256": "a" * 64,
    }
    unsupported_evidence = {**evidence, "status": "unsupported"}
    protocol_rows = []
    for protocol in ("responses", "chat_completions", "messages", "generate_content"):
        supported = protocol == "responses"
        protocol_rows.append({
            "protocol": protocol,
            "support": "supported" if supported else "unsupported",
            "client_transport_features": {
                name: "verified" if supported else "not_applicable"
                for name in BACKFILL.BASE_PROTOCOL_CHECKS
            },
            "evidence": evidence if supported else unsupported_evidence,
        })
    return {
        "schema_version": 2,
        "protocol_ids": ["responses", "chat_completions", "messages", "generate_content"],
        "client_transport_feature_ids": list(BACKFILL.BASE_PROTOCOL_CHECKS),
        "clients": [{
            "id": "fixture-client",
            "name": "Fixture Client",
            "client_protocol": {"protocols": protocol_rows},
            "client_reasoning": {
                "control_kind": "model_defined",
                "level_control": {"values": []},
                "notes": "model defined",
                "evidence": evidence,
            },
            "client_config_os": {
                "release": {
                    "version_key": "cli:1.2.3",
                    "display": "1.2.3",
                    "components": [{"component": "cli", "version": "1.2.3"}],
                    "evidence": evidence,
                },
                "os_support": [{
                    "os": "macos",
                    "support": "documented",
                    "config_files": [{"path": "~/.fixture/config.json"}],
                    "evidence": evidence,
                }],
                "endpoint": {"base_url_rule": "root", "credential_location": "file"},
                "mutation": {"merge_strategy": "merge"},
                "verification_contract": "tool loop",
                "verification_commands": [{"commands": ["fixture --version"]}],
            },
            "verification_os": ["macos"],
        }],
    }


def contract() -> dict:
    return {
        "schema_version": 1,
        "model": {
            "id": "alpha",
            "display_name": "Alpha",
            "family": "test",
            "context_window": 128000,
            "max_output_tokens": 8192,
            "input_modalities": ["text", "image"],
            "output_modalities": ["text"],
        },
        "access": {
            "base_url": "https://api.example.test/v1",
            "groups": [{"name": "Test", "multiplier": 0.5}],
        },
        "protocols": [{
            "name": "responses",
            "status": "verified",
            "evidence": (
                "2026-08-30 minimal text, SSE terminal response.completed, function calling, "
                "tool-result continuation, usage, native web_search and structured invalid_request passed"
            ),
        }],
        "recommended_protocol": "responses",
        "recommended_protocol_reason": "native path",
        "clients": [{
            "name": "Fixture Client",
            "version": "1.2.3",
            "protocol": "responses",
            "status": "verified",
            "recommended": True,
            "evidence": "2026-08-30 real Agent tool loop exited 0",
        }],
        "client_coverage": [{
            "name": "Fixture Client",
            "protocols": ["responses"],
            "status": "verified",
            "evidence": "2026-08-30 real Agent tool loop exited 0",
        }],
        "reasoning": {
            "model_levels": ["low", "high"],
            "client_levels": [],
            "client_mappings": [],
        },
        "verification": {
            "official_spec_url": "https://example.test/alpha",
            "verified_at": "2026-08-30",
            "limits_source": "official",
            "modalities": {"text": "verified", "image": "verified", "video": "unsupported"},
            "gateway_e2e": True,
        },
    }


def inventory() -> dict:
    return {
        "updated_at": "2026-08-31T00:00:00Z",
        "currency": "CNY",
        "unit": "per_1m_tokens",
        "groups": [{
            "group_id": 7,
            "name": "Test",
            "platform": "openai",
            "rate_multiplier": 0.5,
            "models": [{
                "model": "alpha",
                "input_price": 1,
                "output_price": 4,
                "cache_write_price": None,
                "cache_read_price": 0.25,
            }],
        }],
    }


class ModelMatrixBackfillTest(unittest.TestCase):
    def fixture_dir(self, directory: Path) -> tuple[Path, Path, Path]:
        contracts = directory / "contracts"
        contracts.mkdir()
        schema = json.loads((ROOT / "model-doc-contracts" / "matrix-schema.json").read_text())
        (contracts / "matrix-schema.json").write_text(json.dumps(schema), encoding="utf-8")
        contract_path = contracts / "alpha.json"
        contract_path.write_text(json.dumps(contract()), encoding="utf-8")
        client_path = contracts / "client-matrix.json"
        client_path.write_text(json.dumps(client_matrix()), encoding="utf-8")
        inventory_path = directory / "inventory.json"
        inventory_path.write_text(json.dumps(inventory()), encoding="utf-8")
        return contracts, client_path, inventory_path

    def run_cli(self, command: str, contracts: Path, client_path: Path,
                inventory_path: Path, report_path: Path | None = None):
        args = [
            sys.executable, str(SCRIPT), command,
            "--contracts", str(contracts),
            "--client-matrix", str(client_path),
            "--inventory-json", str(inventory_path),
            "--as-of", "2026-08-31",
        ]
        if report_path:
            args += ["--report", str(report_path)]
        completed = subprocess.run(args, cwd=ROOT, text=True, capture_output=True, check=False)
        self.assertEqual(completed.stderr, "")
        return completed, json.loads(completed.stdout)

    def test_plan_copies_only_direct_facts_and_expands_unknowns_as_gaps(self):
        with tempfile.TemporaryDirectory() as raw:
            tmp = Path(raw)
            contracts, clients, inventory_path = self.fixture_dir(tmp)
            original = (contracts / "alpha.json").read_text()
            report_path = tmp / "report.json"
            completed, report = self.run_cli("plan", contracts, clients, inventory_path, report_path)
            self.assertEqual(completed.returncode, 0)
            self.assertEqual((contracts / "alpha.json").read_text(), original)
            self.assertEqual(json.loads(report_path.read_text()), report)
            self.assertEqual(report["network_execution"], "disabled")
            self.assertEqual(report["before"]["with_test_matrix"], 0)
            self.assertEqual(report["after"]["with_test_matrix"], 1)
            self.assertGreater(report["backfilled"]["cells_added"], 0)
            self.assertGreater(report["remaining"]["migration_gap_cells"], 0)

            migrated, _ = BACKFILL.migrate_contract(contract(), client_matrix())
            matrix = migrated["test_matrix"]
            self.assertEqual(matrix["limits"]["context_window"]["status"], "verified")
            self.assertEqual(matrix["modalities"]["video"]["status"], "unsupported")
            self.assertEqual(matrix["protocols"]["responses"]["tool_call"]["status"], "verified")
            self.assertEqual(matrix["protocol_features"]["responses"]["web_search"]["status"], "verified")
            self.assertEqual(matrix["protocol_features"]["responses"]["prompt_cache"]["status"], "blocked")
            self.assertEqual(matrix["reasoning"]["model_levels"]["high"]["status"], "verified")
            self.assertEqual(
                matrix["reasoning"]["clients"]["Fixture Client"]["responses"]["status"],
                "not_exposed",
            )
            client_cell = matrix["clients"]["Fixture Client"]["responses"]["macos"]
            self.assertEqual(client_cell["status"], "blocked")
            self.assertIn("does not state exact OS", client_cell["evidence"])
            self.assertEqual(client_cell["client_version"], "cli:1.2.3")
            self.assertEqual(matrix["group_access"]["Test"]["status"], "blocked")
            self.assertEqual(matrix["pricing"]["Test"]["status"], "blocked")
            self.assertNotIn("input_price", matrix["pricing"]["Test"])

    def test_apply_is_atomic_backed_up_and_idempotent(self):
        with tempfile.TemporaryDirectory() as raw:
            tmp = Path(raw)
            contracts, clients, inventory_path = self.fixture_dir(tmp)
            original = json.loads((contracts / "alpha.json").read_text())
            completed, first = self.run_cli("apply", contracts, clients, inventory_path)
            self.assertEqual(completed.returncode, 0)
            applied = json.loads((contracts / "alpha.json").read_text())
            self.assertIn("test_matrix", applied)
            backup = contracts / ".model-matrix-backfill-backups" / "alpha.json.bak"
            self.assertEqual(json.loads(backup.read_text()), original)
            digest = backup.read_bytes()

            completed, second = self.run_cli("apply", contracts, clients, inventory_path)
            self.assertEqual(completed.returncode, 0)
            self.assertEqual(second["backfilled"]["cells_added"], 0)
            self.assertEqual(second["backfilled"]["contracts_changed"], 0)
            self.assertEqual(backup.read_bytes(), digest)
            self.assertEqual(json.loads((contracts / "alpha.json").read_text()), applied)
            self.assertGreater(first["backfilled"]["cells_added"], 0)

    def test_generated_contract_is_readable_by_nine_matrix_plan(self):
        migrated, _ = BACKFILL.migrate_contract(contract(), client_matrix())
        sections = MATRIX.audit_contract_sections(
            migrated,
            client_matrix(),
            MATRIX.normalize_inventory(inventory())["models"]["alpha"],
            currency="CNY",
            unit="per_1m_tokens",
            as_of=BACKFILL.date(2026, 8, 31),
        )
        self.assertEqual(set(sections), set(BACKFILL.EXPECTED_MATRICES))
        self.assertTrue(any("status must be" in item for item in sections["test_evidence"]))

    def test_existing_test_matrix_cells_are_never_overwritten(self):
        source = contract()
        source["test_matrix"] = {
            "protocol_features": {
                "responses": {
                    "prompt_cache": {"status": "unsupported", "evidence": "owned negative probe"}
                }
            }
        }
        migrated, _ = BACKFILL.migrate_contract(source, client_matrix())
        self.assertEqual(
            migrated["test_matrix"]["protocol_features"]["responses"]["prompt_cache"],
            {"status": "unsupported", "evidence": "owned negative probe"},
        )

    def test_http_pricing_url_is_rejected_without_network_access(self):
        with tempfile.TemporaryDirectory() as raw:
            tmp = Path(raw)
            contracts, clients, _ = self.fixture_dir(tmp)
            completed = subprocess.run(
                [
                    sys.executable, str(SCRIPT), "plan",
                    "--contracts", str(contracts),
                    "--client-matrix", str(clients),
                    "--pricing-url", "https://example.test/pricing",
                ],
                cwd=ROOT, text=True, capture_output=True, check=False,
            )
            self.assertEqual(completed.returncode, 2)
            report = json.loads(completed.stdout)
            self.assertIn("network pricing URLs are disabled", report["error"])
            self.assertEqual(completed.stderr, "")

    def test_file_pricing_url_is_supported_offline(self):
        with tempfile.TemporaryDirectory() as raw:
            tmp = Path(raw)
            contracts, clients, inventory_path = self.fixture_dir(tmp)
            completed = subprocess.run(
                [
                    sys.executable, str(SCRIPT), "plan",
                    "--contracts", str(contracts),
                    "--client-matrix", str(clients),
                    "--pricing-url", inventory_path.as_uri(),
                    "--as-of", "2026-08-31",
                ],
                cwd=ROOT, text=True, capture_output=True, check=False,
            )
            self.assertEqual(completed.returncode, 0, completed.stdout)
            self.assertEqual(json.loads(completed.stdout)["network_execution"], "disabled")


if __name__ == "__main__":
    unittest.main()

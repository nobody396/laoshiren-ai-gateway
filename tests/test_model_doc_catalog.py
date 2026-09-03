from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import sys
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "model_doc_catalog.py"


def load_module(path: Path, name: str):
    spec = importlib.util.spec_from_file_location(name, path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


CATALOG = load_module(SCRIPT, "model_doc_catalog_test_module")


class ModelDocCatalogTest(unittest.TestCase):
    def test_loads_every_contract_including_drafts(self):
        contracts = CATALOG.load_contracts()
        expected_ids = {
            json.loads(path.read_text(encoding="utf-8"))["model"]["id"]
            for path in CATALOG.CONTRACT_DIR.glob("*.json")
            if path not in {CATALOG.CLIENT_MATRIX, CATALOG.CONTRACT_DIR / "matrix-schema.json"}
        }

        self.assertEqual({item["model"]["id"] for item in contracts}, expected_ids)
        self.assertGreaterEqual(len(expected_ids), 34)
        self.assertTrue({
            "claude-opus-4-5",
            "gpt-5.3-codex-spark",
            "gpt-daybreak-blue-latest",
            "minimax-m3",
        }.issubset(expected_ids))
        self.assertTrue(any(not item["publication"]["publishable"] for item in contracts))
        self.assertTrue(all(
            item["publication"]["status"]
            == ("publishable" if item["publication"]["publishable"] else "draft")
            for item in contracts
        ))

    def test_projection_exposes_admin_compatibility_rows(self):
        contracts = CATALOG.load_contracts()
        grok = next(item for item in contracts if item["model"]["id"] == "grok-4.6")
        compatibility = grok["compatibility"]

        self.assertTrue(any(row["feature"] == "web_search" for row in compatibility["protocol_features"]))
        self.assertTrue(any(row["check"] == "tool_call" for row in compatibility["protocol_checks"]))
        self.assertTrue(any(
            row["client"] == "Grok Build"
            and row["protocol"] == "chat_completions"
            and row["os"] == "macos"
            for row in compatibility["exact_clients"]
        ))
        self.assertEqual(
            {row["group"] for row in compatibility["access"]},
            {group["name"] for group in grok["access"]["groups"]},
        )
        self.assertEqual(
            {row["group"] for row in compatibility["pricing"]},
            {group["name"] for group in grok["access"]["groups"]},
        )
        self.assertEqual(
            [row["level"] for row in compatibility["reasoning"]["model_levels"]],
            grok["reasoning"]["model_levels"],
        )

    def test_missing_evidence_counts_cells_separately_from_audit_messages(self):
        contracts = CATALOG.load_contracts()
        expected_blocked_cells = 0
        for path in CATALOG.CONTRACT_DIR.glob("*.json"):
            if path in {CATALOG.CLIENT_MATRIX, CATALOG.CONTRACT_DIR / "matrix-schema.json"}:
                continue
            raw = json.loads(path.read_text(encoding="utf-8"))
            expected_blocked_cells += CATALOG._blocked_cell_count(raw.get("test_matrix"))
        blocked_cells = sum(
            item["publication"]["missing_evidence"]["blocked_cells"]
            for item in contracts
        )
        audit_failures = sum(
            item["publication"]["missing_evidence"]["audit_failures"]
            for item in contracts
        )

        self.assertEqual(blocked_cells, 0)
        self.assertEqual(audit_failures, 0)
        for contract in contracts:
            publication = contract["publication"]
            if publication["publishable"]:
                self.assertEqual(publication["missing_evidence"]["audit_failures"], 0)
                self.assertFalse(publication["missing_evidence"]["validation_errors"])

    def test_generated_types_are_explicit_for_admin_rows(self):
        rendered = CATALOG.render(CATALOG.load_contracts())
        self.assertIn("export interface ModelDocCompatibility", rendered)
        self.assertIn("protocol_features:", rendered)
        self.assertIn("tools:", rendered)
        self.assertIn("exact_clients:", rendered)
        self.assertIn("pricing: ModelDocPricingRow[]", rendered)
        self.assertIn("gateway_e2e: boolean", rendered)
        self.assertIn("evidence_ids?: string[]", rendered)
        self.assertIn("pass_evidence_ids?: string[]", rendered)
        self.assertIn("protocol_evidence?: Partial<Record", rendered)

    def test_full_matrix_is_backend_only_and_public_projection_is_not_reconstructable(self):
        contracts = CATALOG.load_contracts()
        public = CATALOG.render(contracts)
        admin = json.loads(CATALOG.render_admin_payload(contracts))

        self.assertNotIn('"test_matrix"', public)
        self.assertNotIn('"client_matrix"', public)
        self.assertEqual(admin["counts"], {"models": 34, "clients": 14, "intersections": 476})
        self.assertEqual(len(admin["contracts"]), 34)
        self.assertEqual(len(admin["client_matrix"]["clients"]), 14)
        self.assertIn("test_matrix", admin["contracts"][0])

    def test_public_client_rows_strip_artifact_locations_and_failure_history(self):
        contract = next(
            item for item in CATALOG.load_contracts()
            if item["model"]["id"] == "gemini-3.1-pro"
        )
        projected = CATALOG.project_public_contract(contract)
        rendered = json.dumps({
            "clients": projected["clients"],
            "client_coverage": projected["client_coverage"],
        })
        self.assertNotIn("artifact_uri", rendered)
        self.assertNotIn("artifact_sha256", rendered)
        self.assertNotIn("failure_evidence_ids", rendered)


if __name__ == "__main__":
    unittest.main()

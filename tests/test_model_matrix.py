import importlib.util
from datetime import date
import json
import os
from pathlib import Path
import unittest


ROOT = Path(__file__).parents[1]
# Skill Hub tests normally exercise the synchronized Skill copy. During source
# development MODEL_DOC_MATRIX_SCRIPT lets the exact project gate be tested
# before Skill synchronization.
SCRIPT = Path(os.environ.get("MODEL_DOC_MATRIX_SCRIPT", ROOT / "scripts" / "model_doc_matrix.py"))
EXAMPLE = ROOT / "tests" / "fixtures" / "model-contracts" / "example.json"
CLIENTS = ROOT / "tests" / "fixtures" / "model-contracts" / "client-matrix.json"
spec = importlib.util.spec_from_file_location("model_doc_matrix", SCRIPT)
module = importlib.util.module_from_spec(spec)
assert spec.loader
spec.loader.exec_module(module)


class ModelMatrixTest(unittest.TestCase):
    AS_OF = date(2026, 8, 31)
    GROUP = "GLM 企业高速线路"

    def test_exact_nine_matrix_names(self):
        self.assertEqual(module.MATRIX_NAMES, (
            "public_model", "model_protocol", "model_reasoning",
            "client_protocol", "client_reasoning", "group_access",
            "client_config_os", "test_evidence", "model_price",
        ))

    def test_client_protocol_expansion_uses_declared_snapshot(self):
        matrix = json.loads(CLIENTS.read_text())
        for client in matrix["clients"]:
            self.assertIn("chat_completions", module.client_protocols(client))

    def complete_contract(self):
        contract = json.loads(EXAMPLE.read_text(encoding="utf-8"))
        clients = json.loads(CLIENTS.read_text(encoding="utf-8"))
        contract["recommended_protocol_reason"] = (
            "Chat Completions has complete tool continuation; Responses Web Search is unsupported."
        )
        client_rows = [client for client in clients["clients"] if "chat_completions" in module.client_protocols(client)]
        contract["client_coverage"] = [
            {
                "name": client["name"], "protocols": ["chat_completions"],
                "status": "verified", "evidence": "all configured OS Agent loops passed",
            }
            for client in client_rows
        ]
        contract["test_matrix"] = {
            "limits": {
                "context_window": {"status": "verified", "evidence": "official spec"},
                "max_output_tokens": {"status": "verified", "evidence": "official spec"},
            },
            "modalities": {
                name: {"status": status, "evidence": "official spec and owned probe"}
                for name, status in contract["verification"]["modalities"].items()
            },
            "protocols": {
                "chat_completions": {
                    check: {"status": "verified", "evidence": "provider contract regression"}
                    for check in module.REQUIRED_PROTOCOL_CHECKS
                }
            },
            "protocol_features": {
                "chat_completions": {
                    feature: {
                        "status": "unsupported" if feature == "web_search" else "verified",
                        "evidence": "feature-specific positive or negative probe",
                    }
                    for feature in module.REQUIRED_PROTOCOL_FEATURES
                }
            },
            "clients": {},
            "reasoning": {"model_levels": {}, "clients": {}},
            "group_access": {
                self.GROUP: {
                    "status": "verified", "evidence": "redacted group-key model discovery and call",
                    "verified_at": "2026-08-31", "model_id": "glm-5.3",
                    "base_url": contract["access"]["base_url"], "multiplier": 1,
                    "protocols": ["chat_completions"],
                    "recommended_protocol": "chat_completions",
                    "recommended_protocol_reason": "complete Agent loop; no server Web Search dependency",
                }
            },
            "pricing": {
                self.GROUP: {
                    "status": "verified", "evidence": "public price row reconciled to billing",
                    "verified_at": "2026-08-31", "currency": "CNY", "unit": "per_1m_tokens",
                    "input_price": 8, "output_price": 28,
                    "cache_write_price": None, "cache_read_price": 2,
                }
            },
        }
        for level in contract["reasoning"]["model_levels"]:
            contract["test_matrix"]["reasoning"]["model_levels"][level] = {
                "status": "verified", "evidence": "reasoning probe",
            }
        for client in client_rows:
            name = client["name"]
            contract["test_matrix"]["clients"][name] = {"chat_completions": {}}
            for os_name in module.client_os_names(client):
                contract["test_matrix"]["clients"][name]["chat_completions"][os_name] = {
                    "status": "verified", "evidence": "real tool-result Agent loop",
                    "verified_at": "2026-08-31", "model_id": "glm-5.3",
                    "protocol": "chat_completions", "client_version": module.client_version_key(client), "os": os_name,
                }
            reasoning_contract = module.client_reasoning_contract(client)
            fixed_levels = reasoning_contract["levels"] if reasoning_contract["mode"] == "fixed" else []
            unsupported = set(fixed_levels) - set(contract["reasoning"]["model_levels"])
            mappings = []
            for source in sorted(unsupported):
                mappings.append({
                    "client": name, "protocol": "chat_completions", "from": source,
                    "to": "high" if source == "medium" else "max",
                })
            contract["test_matrix"]["reasoning"]["clients"][name] = {
                "chat_completions": {
                    "status": "verified" if fixed_levels else "not_exposed",
                    "client_levels": fixed_levels, "mappings": mappings,
                    "evidence": "protocol-specific client reasoning regression",
                }
            }
        inventory_model = {
            "groups": {
                self.GROUP: {
                    "rate_multiplier": 1,
                    "price": {
                        "input_price": 8, "output_price": 28,
                        "cache_write_price": None, "cache_read_price": 2,
                    },
                }
            }
        }
        return contract, clients, inventory_model

    def audit(self, contract, clients, inventory):
        return module.audit_contract(
            contract, clients, inventory, currency="CNY", unit="per_1m_tokens",
            as_of=self.AS_OF,
        )

    def test_complete_nine_matrix_contract_passes(self):
        contract, clients, inventory = self.complete_contract()
        self.assertEqual(self.audit(contract, clients, inventory), [])

    def test_unsupported_web_search_does_not_demote_protocol(self):
        contract, clients, inventory = self.complete_contract()
        self.assertEqual(
            contract["test_matrix"]["protocol_features"]["chat_completions"]["web_search"]["status"],
            "unsupported",
        )
        self.assertEqual(self.audit(contract, clients, inventory), [])

    def test_recommended_protocol_must_be_verified_and_explained(self):
        contract, clients, inventory = self.complete_contract()
        contract["recommended_protocol"] = "responses"
        contract["recommended_protocol_reason"] = ""
        failures = self.audit(contract, clients, inventory)
        self.assertTrue(any("recommended_protocol must reference a verified" in failure for failure in failures))
        self.assertTrue(any("recommended_protocol_reason is required" in failure for failure in failures))

    def test_missing_protocol_feature_fails(self):
        contract, clients, inventory = self.complete_contract()
        del contract["test_matrix"]["protocol_features"]["chat_completions"]["billing"]
        self.assertTrue(any("billing" in f for f in self.audit(contract, clients, inventory)))

    def test_reasoning_mapping_requires_exact_protocol_and_client(self):
        contract, clients, inventory = self.complete_contract()
        mappings = contract["test_matrix"]["reasoning"]["clients"]["Kimi Code"]["chat_completions"]["mappings"]
        mappings[0].pop("protocol")
        self.assertTrue(any("exact client and protocol" in f for f in self.audit(contract, clients, inventory)))

    def test_legacy_flat_reasoning_is_a_migration_gap(self):
        contract, clients, inventory = self.complete_contract()
        nested = contract["test_matrix"]["reasoning"]["clients"]["Kimi Code"]
        contract["test_matrix"]["reasoning"]["clients"]["Kimi Code"] = nested["chat_completions"]
        self.assertTrue(any("legacy reasoning entry" in f for f in self.audit(contract, clients, inventory)))

    def test_client_version_drift_fails_exact_evidence(self):
        contract, clients, inventory = self.complete_contract()
        cell = contract["test_matrix"]["clients"]["OpenCode"]["chat_completions"]["macos"]
        cell["client_version"] = "old"
        self.assertTrue(any("client_version must exactly match" in f for f in self.audit(contract, clients, inventory)))

    def test_required_os_evidence_cannot_be_omitted(self):
        contract, clients, inventory = self.complete_contract()
        clients["clients"][-1]["verification_os"] = ["macos"]
        del contract["test_matrix"]["clients"]["ZCode"]["chat_completions"]["macos"]
        self.assertTrue(any("ZCode/chat_completions/macos" in f for f in self.audit(contract, clients, inventory)))

    def test_stale_evidence_is_marked(self):
        contract, clients, inventory = self.complete_contract()
        cell = contract["test_matrix"]["clients"]["Grok Build"]["chat_completions"]["macos"]
        cell["verified_at"] = "2025-01-01"
        self.assertTrue(any("stale evidence" in f for f in self.audit(contract, clients, inventory)))

    def test_unpublished_limit_is_terminal_without_guessing_a_number(self):
        contract, clients, inventory = self.complete_contract()
        contract["model"]["max_output_tokens"] = None
        contract["test_matrix"]["limits"]["max_output_tokens"] = {
            "status": "not_published",
            "evidence": "official model page omits a maximum output limit",
        }
        failures = self.audit(contract, clients, inventory)
        self.assertFalse(any("limits/max_output_tokens" in failure for failure in failures))

    def test_group_access_must_match_public_multiplier(self):
        contract, clients, inventory = self.complete_contract()
        contract["test_matrix"]["group_access"][self.GROUP]["multiplier"] = 0.5
        self.assertTrue(any("multiplier conflicts" in f for f in self.audit(contract, clients, inventory)))

    def test_price_matrix_must_match_public_customer_price(self):
        contract, clients, inventory = self.complete_contract()
        contract["test_matrix"]["pricing"][self.GROUP]["output_price"] = 99
        self.assertTrue(any("output_price: conflicts" in f for f in self.audit(contract, clients, inventory)))

    def test_canonical_scoped_price_matrix_supersedes_legacy_contract_projection(self):
        contract, clients, inventory = self.complete_contract()
        contract["test_matrix"]["pricing"][self.GROUP]["output_price"] = 999
        canonical_rows = [{
            "model_id": contract["model"]["id"],
            "group_name": self.GROUP,
            "price_scope": "group_customer",
            "status": "not_exposed",
            "component_status": {
                "input": "verified", "cached_input": "verified",
                "cache_write": "not_applicable", "output": "verified",
                "long_context": "not_exposed",
            },
        }]
        failures = module.audit_contract(
            contract, clients, inventory, currency="CNY", unit="per_1m_tokens",
            as_of=self.AS_OF, canonical_price_rows=canonical_rows,
        )
        self.assertFalse(any("pricing" in failure for failure in failures))

    def test_client_config_os_matrix_requires_both_platforms(self):
        _, clients, _ = self.complete_contract()
        clients["clients"][0]["client_config_os"]["os_support"] = [
            row for row in clients["clients"][0]["client_config_os"]["os_support"]
            if row["os"] != "windows"
        ]
        sections = module.audit_client_matrix(clients)
        self.assertTrue(any("missing OS contracts: windows" in f for f in sections["client_config_os"]))

    def test_inventory_preserves_group_price_cells(self):
        inventory = module.normalize_inventory({
            "currency": "CNY", "unit": "per_1m_tokens",
            "groups": [{
                "group_id": 1, "name": "G", "rate_multiplier": 0.5,
                "models": [{"model": "m", "input_price": 1, "output_price": 2,
                            "cache_write_price": None, "cache_read_price": 0.1}],
            }],
        })
        self.assertEqual(inventory["models"]["m"]["groups"]["G"]["rate_multiplier"], 0.5)
        self.assertEqual(inventory["models"]["m"]["groups"]["G"]["price"]["output_price"], 2)


if __name__ == "__main__":
    unittest.main()

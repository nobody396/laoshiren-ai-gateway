import copy
import importlib.util
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).parents[1]
SCRIPT = ROOT / "scripts" / "model_doc_contract.py"
EXAMPLE = ROOT / "tests" / "fixtures" / "model-contracts" / "example.json"
CLIENTS = ROOT / "tests" / "fixtures" / "model-contracts" / "client-matrix.json"
spec = importlib.util.spec_from_file_location("model_doc_contract", SCRIPT)
module = importlib.util.module_from_spec(spec)
assert spec.loader
spec.loader.exec_module(module)


class ModelDocContractTest(unittest.TestCase):
    def setUp(self):
        module.DEFAULT_CLIENT_MATRIX_CANDIDATES = (CLIENTS,)

    def example(self):
        return json.loads(EXAMPLE.read_text(encoding="utf-8"))

    def test_structure_without_resolved_matrix_cannot_publish(self):
        contract = module.validate(self.example())
        self.assertFalse(module.is_publishable(contract))

    def test_rejects_inferred_public_protocol(self):
        contract = self.example()
        contract["protocols"][0]["status"] = "inferred"
        with self.assertRaisesRegex(ValueError, "verified, blocked, or unsupported"):
            module.validate(contract)

    def test_rejects_client_without_verified_protocol(self):
        contract = self.example()
        contract["clients"][0]["protocol"] = "responses"
        with self.assertRaisesRegex(ValueError, "verified protocol"):
            module.validate(contract)

    def test_rejects_recommended_protocol_without_verified_evidence(self):
        contract = self.example()
        contract["recommended_protocol"] = "responses"
        with self.assertRaisesRegex(ValueError, "recommended_protocol"):
            module.validate(contract)

    def test_rejects_missing_protocol_compatible_client(self):
        contract = self.example()
        contract["client_coverage"] = [
            item for item in contract["client_coverage"] if item["name"] != "OpenCode"
        ]
        with self.assertRaisesRegex(ValueError, "missing protocol-compatible clients"):
            module.validate(contract)

    def test_validator_uses_canonical_client_matrix(self):
        contract = self.example()
        client_matrix = json.loads(CLIENTS.read_text(encoding="utf-8"))
        client_matrix["clients"].append({
            "name": "Matrix Canary",
            "protocols": ["chat_completions"],
        })
        with self.assertRaisesRegex(ValueError, "Matrix Canary"):
            module.validate(contract, client_matrix)

    def test_rejects_coverage_verified_without_matching_client(self):
        contract = self.example()
        contract["clients"] = [
            item for item in contract["clients"] if item["name"] != "OpenCode"
        ]
        with self.assertRaisesRegex(ValueError, "must match"):
            module.validate(contract)

    def test_blocked_client_keeps_valid_contract_out_of_public_catalog(self):
        contract = self.example()
        contract["clients"] = [
            item for item in contract["clients"] if item["name"] != "OpenCode"
        ]
        for item in contract["client_coverage"]:
            if item["name"] == "OpenCode":
                item["status"] = "blocked"
                item["evidence"] = "owned gateway loop not completed"
        validated = module.validate(contract)
        self.assertFalse(module.is_publishable(validated))

    def test_rejects_modality_claim_without_matching_evidence(self):
        contract = self.example()
        contract["model"]["input_modalities"].append("image")
        with self.assertRaisesRegex(ValueError, "conflicts"):
            module.validate(contract)

    def test_rejects_secrets(self):
        contract = self.example()
        contract["access"]["api_key"] = "not-allowed"
        with self.assertRaisesRegex(ValueError, "secret-shaped"):
            module.validate(contract)

    def test_allows_honest_draft_without_verified_client(self):
        contract = self.example()
        contract["verification"]["gateway_e2e"] = False
        contract["clients"] = []
        for item in contract["client_coverage"]:
            item["status"] = "blocked"
            item["protocols"] = []
            item["evidence"] = "exact client loop is still missing"
        validated = module.validate(contract)
        self.assertFalse(module.is_publishable(validated))

    def test_unknown_max_output_requires_blocked_limit_cell(self):
        contract = self.example()
        contract["verification"]["gateway_e2e"] = False
        contract["model"]["max_output_tokens"] = None
        contract["test_matrix"] = {"limits": {}}
        contract["test_matrix"]["limits"]["max_output_tokens"] = {
            "status": "blocked",
            "evidence": "official deployment does not publish a maximum",
        }
        validated = module.validate(contract)
        self.assertFalse(module.is_publishable(validated))

    def test_officially_unpublished_max_output_is_terminal_and_publishable(self):
        contract = self.example()
        contract["model"]["max_output_tokens"] = None
        contract.setdefault("test_matrix", {}).setdefault("limits", {})["max_output_tokens"] = {
            "status": "not_published",
            "evidence": "official model page does not publish this limit",
        }
        isolated_client_matrix = {
            "clients": [
                {"name": item["name"], "protocols": ["chat_completions"]}
                for item in contract["client_coverage"]
            ]
        }
        validated = module.validate(contract, isolated_client_matrix)
        self.assertFalse(module.is_publishable(validated))


if __name__ == "__main__":
    unittest.main()

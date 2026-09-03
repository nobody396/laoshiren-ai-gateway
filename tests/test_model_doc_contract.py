import importlib.util
import json
from datetime import datetime, timezone
from pathlib import Path
import unittest


ROOT = Path(__file__).parents[1]
SPEC = importlib.util.spec_from_file_location("project_model_doc_contract", ROOT / "scripts" / "model_doc_contract.py")
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(MODULE)
MATRIX_SPEC = importlib.util.spec_from_file_location("project_model_doc_matrix", ROOT / "scripts" / "model_doc_matrix.py")
MATRIX = importlib.util.module_from_spec(MATRIX_SPEC)
assert MATRIX_SPEC.loader
MATRIX_SPEC.loader.exec_module(MATRIX)


class ModelDocContractTerminalLimitTests(unittest.TestCase):
    def fixture(self):
        from test_model_matrix import ModelMatrixTest
        contract, clients, _ = ModelMatrixTest().complete_contract()
        return contract, clients

    def test_matrix_default_date_uses_beijing_not_utc_day(self):
        self.assertEqual(
            "2026-09-01",
            MATRIX.beijing_today(datetime(2026, 8, 31, 16, 30, tzinfo=timezone.utc)).isoformat(),
        )

    def test_officially_unpublished_max_output_is_a_valid_terminal_fact(self):
        contract, clients = self.fixture()
        contract["model"]["max_output_tokens"] = None
        contract["test_matrix"]["limits"]["max_output_tokens"] = {"status":"not_published", "evidence":"official negative fixture"}
        for feature in ("reasoning", "image_input"):
            contract["test_matrix"]["protocol_features"]["chat_completions"][feature] = {"status":"not_published", "evidence":"official negative fixture"}
        validated = MODULE.validate(contract, clients)
        self.assertIsNone(validated["model"]["max_output_tokens"])
        self.assertEqual(
            "not_published",
            validated["test_matrix"]["limits"]["max_output_tokens"]["status"],
        )

    def test_unpublished_optional_feature_is_terminal_without_inventing_support(self):
        contract, clients = self.fixture()
        contract["model"]["max_output_tokens"] = None
        contract["test_matrix"]["limits"]["max_output_tokens"] = {"status":"not_published", "evidence":"official negative fixture"}
        for feature in ("reasoning", "image_input"):
            contract["test_matrix"]["protocol_features"]["chat_completions"][feature] = {"status":"not_published", "evidence":"official negative fixture"}
        failures = MATRIX.audit_contract_sections(contract, clients)["model_protocol"]
        self.assertFalse(any("features/reasoning" in failure for failure in failures), failures)
        self.assertFalse(any("features/image_input" in failure for failure in failures), failures)

    def test_not_exposed_and_not_applicable_features_are_terminal(self):
        grok, clients = self.fixture()
        kimi, _ = self.fixture()
        grok["test_matrix"]["protocol_features"]["chat_completions"]["web_search"] = {"status":"not_exposed", "evidence":"negative fixture"}
        kimi["test_matrix"]["protocol_features"]["chat_completions"]["reasoning"] = {"status":"not_applicable", "evidence":"negative fixture"}
        grok_failures = MATRIX.audit_contract_sections(grok, clients)["model_protocol"]
        kimi_failures = MATRIX.audit_contract_sections(kimi, clients)["model_protocol"]
        self.assertFalse(any("features/web_search" in failure for failure in grok_failures), grok_failures)
        self.assertFalse(any("features/reasoning" in failure for failure in kimi_failures), kimi_failures)


if __name__ == "__main__":
    unittest.main()

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import sys
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "model_doc_matrix.py"
MATRIX = ROOT / "model-doc-contracts" / "client-matrix.json"


def load_module(path: Path, name: str):
    spec = importlib.util.spec_from_file_location(name, path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


AUDITOR = load_module(SCRIPT, "model_doc_matrix_client_states_test_module")


class ClientStateAuditTest(unittest.TestCase):
    def test_all_unsupported_client_and_none_reasoning_are_valid_contract_states(self) -> None:
        matrix = json.loads(MATRIX.read_text(encoding="utf-8"))
        client = next(client for client in matrix["clients"] if all(
            row["support"] == "unsupported" for row in client["client_protocol"]["protocols"]
        ))
        fixture = {
            **{key: value for key, value in matrix.items() if key != "clients"},
            "clients": [client],
        }

        sections = AUDITOR.audit_client_matrix(fixture)

        self.assertEqual([], sections["client_protocol"])
        self.assertEqual([], sections["client_reasoning"])
        self.assertEqual([], sections["client_config_os"])


if __name__ == "__main__":
    unittest.main()

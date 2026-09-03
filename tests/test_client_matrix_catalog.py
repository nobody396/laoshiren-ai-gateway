from __future__ import annotations

import importlib.util
import copy
import json
from pathlib import Path
import sys
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "client_matrix_catalog.py"


def load_module(path: Path, name: str):
    spec = importlib.util.spec_from_file_location(name, path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


CATALOG = load_module(SCRIPT, "client_matrix_catalog_test_module")


class ClientMatrixCatalogTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads(CATALOG.SOURCE.read_text(encoding="utf-8"))

    def test_public_projection_contains_every_canonical_client(self):
        projected = [CATALOG.project_client(client) for client in self.matrix["clients"]]
        self.assertEqual(
            {client["slug"] for client in projected},
            {client["slug"] for client in self.matrix["clients"]},
        )
        self.assertIn("integration-qoder", {client["slug"] for client in projected})
        self.assertIn("integration-minimax-code", {client["slug"] for client in projected})

    def test_projection_preserves_manual_only_states_without_fake_disabled_clients(self):
        projected = [CATALOG.project_client(client) for client in self.matrix["clients"]]
        disabled = [client for client in projected if client["one_click_status"] == "disabled"]
        prototypes = [client for client in projected if client["one_click_status"] == "prototype"]
        self.assertEqual([], disabled)
        self.assertTrue(prototypes)
        self.assertTrue(all(client["config_contract"]["verification_commands"] for client in prototypes))

    def test_generated_types_cover_canonical_enum_values(self):
        rendered = CATALOG.render(self.matrix)
        self.assertIn("| 'none'", rendered)
        self.assertIn("| 'not_applicable'", rendered)
        self.assertIn("| 'disabled'", rendered)

    def test_setup_projection_is_fail_closed_until_client_and_os_are_ready(self):
        client = copy.deepcopy(next(item for item in self.matrix["clients"] if item["one_click_status"] == "prototype"))
        projected = CATALOG.project_setup_contract(client)
        self.assertEqual(projected["one_click_status"], "prototype")
        self.assertFalse(any(cell["ready"] for cell in projected["os"].values()))

        client["one_click_status"] = "ready"
        client["client_config_os"]["os_support"][0]["support"] = "verified"
        client["client_config_os"]["os_support"][0]["evidence"]["status"] = "verified"
        ready = CATALOG.project_setup_contract(client)
        self.assertTrue(ready["os"][client["client_config_os"]["os_support"][0]["os"]]["ready"])

    def test_go_projection_contains_canonical_clients_and_model_protocols(self):
        rendered = CATALOG.render_go(self.matrix)
        self.assertIn("var generatedClientSetupContracts", rendered)
        self.assertIn('"claude-code": {', rendered)
        self.assertIn('OneClickStatus: "ready"', rendered)
        self.assertIn("var generatedClientSetupModelProtocols", rendered)
        self.assertIn('"gpt-5.6-sol"', rendered)
        self.assertIn('{"responses": true}', rendered)


if __name__ == "__main__":
    unittest.main()

import json
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCHEMA_PATH = ROOT / "model-doc-contracts" / "matrix-schema.json"

MATRIX_NAMES = {
    "public_model",
    "model_protocol",
    "model_reasoning",
    "client_protocol",
    "client_reasoning",
    "group_access",
    "client_config_os",
    "test_evidence",
    "model_price",
}


class MatrixSchemaTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        cls.matrices = cls.schema["properties"]["matrices"]
        cls.defs = cls.schema["$defs"]

    def test_is_draft_2020_12_json_schema(self):
        self.assertEqual(
            self.schema["$schema"],
            "https://json-schema.org/draft/2020-12/schema",
        )
        self.assertEqual(self.schema["type"], "object")
        self.assertFalse(self.schema["additionalProperties"])

    def test_has_exactly_nine_matrices(self):
        properties = self.matrices["properties"]
        self.assertEqual(set(properties), MATRIX_NAMES)
        self.assertEqual(set(self.matrices["required"]), MATRIX_NAMES)
        self.assertFalse(self.matrices["additionalProperties"])
        self.assertNotIn("protocol_feature", properties)

    def test_every_matrix_declares_a_valid_primary_key(self):
        for name, matrix in self.matrices["properties"].items():
            with self.subTest(matrix=name):
                primary_key = matrix.get("x-primary-key")
                self.assertIsInstance(primary_key, list)
                self.assertTrue(primary_key)
                row_ref = matrix["properties"]["rows"]["items"]["$ref"]
                row = self._resolve_local_ref(row_ref)
                self.assertTrue(set(primary_key).issubset(row["required"]))
                self.assertTrue(set(primary_key).issubset(row["properties"]))

    def test_all_local_refs_resolve(self):
        refs = []

        def visit(value):
            if isinstance(value, dict):
                if "$ref" in value and value["$ref"].startswith("#/"):
                    refs.append(value["$ref"])
                for child in value.values():
                    visit(child)
            elif isinstance(value, list):
                for child in value:
                    visit(child)

        visit(self.schema)
        self.assertTrue(refs)
        for ref in refs:
            with self.subTest(ref=ref):
                self.assertIsNotNone(self._resolve_local_ref(ref))

    def test_model_protocol_owns_features_and_recommendation_reason(self):
        row = self.defs["modelProtocolRow"]
        self.assertIn("features", row["required"])
        self.assertIn("recommendation", row["required"])
        self.assertIn("recommendation_reason", row["required"])
        self.assertEqual(
            row["properties"]["features"]["items"]["$ref"],
            "#/$defs/featureResult",
        )

    def test_schema_covers_version_os_date_and_durable_evidence(self):
        evidence = self.defs["testEvidenceRow"]
        self.assertTrue(
            {
                "observed_at",
                "artifact_uri",
                "artifact_sha256",
                "versions",
                "secret_free",
            }.issubset(evidence["required"])
        )
        self.assertEqual(evidence["properties"]["secret_free"]["const"], True)
        self.assertIn(
            "real_client_protocol",
            evidence["properties"]["evidence_type"]["enum"],
        )
        self.assertIn("unsupported", evidence["properties"]["result"]["enum"])
        self.assertEqual(
            evidence["properties"]["evidence_refs"]["$ref"],
            "#/$defs/evidenceIds",
        )

        client_evidence_rule = evidence["allOf"][0]
        self.assertEqual(
            set(
                client_evidence_rule["then"]["properties"]["target"]["required"]
            ),
            {"client_id", "client_version", "os"},
        )

        config = self.defs["clientConfigOsRow"]
        self.assertTrue(
            {"client_version", "os", "architecture", "observed_at"}.issubset(
                config["required"]
            )
        )

    def test_price_has_explicit_units_currency_cache_and_long_context(self):
        price = self.defs["modelPriceRow"]
        required = {
            "currency",
            "unit_tokens",
            "input_price",
            "cached_input_price",
            "cache_write_price",
            "output_price",
            "long_context",
            "component_status",
            "effective_from",
        }
        self.assertTrue(required.issubset(price["required"]))
        self.assertEqual(price["properties"]["unit_tokens"]["const"], 1_000_000)
        self.assertEqual(
            set(price["properties"]["price_scope"]["enum"]),
            {"provider_public", "gateway_base", "group_customer"},
        )
        self.assertEqual(
            set(self.defs["priceComponentStatus"]["enum"]),
            {"verified", "unknown", "blocked", "not_applicable", "not_published", "not_exposed"},
        )

        long_context = self.defs["longContextPrice"]
        self.assertTrue(
            {
                "threshold_tokens",
                "input_multiplier",
                "cached_input_multiplier",
                "output_multiplier",
            }.issubset(long_context["required"])
        )

    def test_cell_status_has_non_final_final_and_stale_states(self):
        statuses = set(self.defs["cellStatus"]["enum"])
        self.assertEqual(
            statuses,
            {
                "unknown", "planned", "verified", "unsupported", "blocked",
                "not_published", "not_exposed", "not_applicable", "stale",
            },
        )
        evidence_rule = self.defs["observedEvidenceRule"]
        self.assertEqual(
            evidence_rule["then"]["properties"]["evidence_ids"]["minItems"], 1
        )

    def _resolve_local_ref(self, ref):
        self.assertTrue(ref.startswith("#/"), ref)
        value = self.schema
        for component in ref[2:].split("/"):
            component = component.replace("~1", "/").replace("~0", "~")
            value = value[component]
        return value


if __name__ == "__main__":
    unittest.main()

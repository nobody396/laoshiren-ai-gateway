import argparse
import copy
import importlib.util
from pathlib import Path
import unittest

SCRIPT = Path(__file__).parents[1] / "scripts" / "model_release.py"
spec = importlib.util.spec_from_file_location("model_release", SCRIPT)
module = importlib.util.module_from_spec(spec)
assert spec.loader
spec.loader.exec_module(module)


class ModelReleaseTest(unittest.TestCase):
    def valid_manifest(self):
        """Legacy v1 manifest kept as a compatibility fixture."""
        return {
            "schema_version": 1,
            "model": {
                "id": "grok-4.6",
                "upstream_id": "grok-4.6",
                "display_name": "Grok 4.6",
                "platform": "grok",
                "context_window": 500000,
                "max_output_tokens": 128000,
            },
            "capabilities": {
                "protocol": "responses",
                "streaming": True,
                "tools": True,
                "image_input": True,
                "cache_read": True,
                "reasoning_efforts": ["low", "medium", "high", "xhigh"],
            },
            "pricing": {
                "input_per_mtok_usd": 2,
                "cached_input_per_mtok_usd": 0.5,
                "output_per_mtok_usd": 6,
                "evidence_url": "https://provider.example/pricing",
                "long_context": {
                    "input_threshold": 200000,
                    "input_multiplier": 2,
                    "output_multiplier": 2,
                },
            },
            "production": {
                "account_id": 29,
                "group_id": 34,
                "group_name": "Grok 4.6 分组",
                "group_description": "",
                "client_default": True,
                "aliases": [],
                "replace_model_id": "grok-4.5",
            },
            "verification": {
                "expected_text": "OK",
                "owned_e2e": True,
                "require_usage_attribution": True,
                "require_accounting_reconciliation": True,
            },
        }

    def v2_manifest(self):
        manifest = self.valid_manifest()
        manifest["schema_version"] = 2
        manifest["lifecycle"] = {
            "public_status": "public",
            "introduced_at": "2026-08-31",
            "deprecated_at": None,
            "replacement_model_id": None,
        }
        manifest["evidence"] = [
            {
                "id": "official-price",
                "kind": "official_docs",
                "url": "https://provider.example/pricing",
                "observed_at": "2026-08-31T10:20:30Z",
                "subject_version": "grok-4.6/2026-08-31",
                "result": "pass",
                "artifact_sha256": None,
            },
            {
                "id": "responses-contract",
                "kind": "live_probe",
                "url": None,
                "observed_at": "2026-08-31T10:30:00Z",
                "subject_version": "grok-4.6/gateway-a1b2c3",
                "result": "pass",
                "artifact_sha256": "a" * 64,
            },
            {
                "id": "billing-sample",
                "kind": "billing_reconciliation",
                "url": None,
                "observed_at": "2026-08-31T10:35:00Z",
                "subject_version": "grok-4.6/gateway-a1b2c3",
                "result": "pass",
                "artifact_sha256": "b" * 64,
            },
        ]
        manifest["protocol_matrix"] = [
            {
                "protocol": "responses",
                "support": "supported",
                "recommendation": "preferred",
                "recommendation_reason": "Native reasoning and complete stream events.",
                "evidence_ids": ["responses-contract"],
            },
            {
                "protocol": "chat_completions",
                "support": "unsupported",
                "recommendation": "not_applicable",
                "recommendation_reason": "Provider rejects this model on the chat endpoint.",
                "evidence_ids": ["responses-contract"],
            },
        ]
        features = {name: "pass" for name in module.REQUIRED_PROTOCOL_FEATURES}
        features["web_search"] = "fail"
        manifest["protocol_feature_matrix"] = [
            {
                "protocol": "responses",
                "features": features,
                "evidence_ids": ["responses-contract", "billing-sample"],
            }
        ]
        manifest["reasoning_matrix"] = [
            {
                "protocol": "responses",
                "effort": effort,
                "support": "supported",
                "wire_value": effort,
                "evidence_ids": ["responses-contract"],
            }
            for effort in ("low", "medium", "high", "xhigh")
        ]
        manifest["price_matrix"] = [
            {
                "scope": "default",
                "currency": "USD",
                "unit": "per_mtok",
                "input": 2,
                "output": 6,
                "cached_input": 0.5,
                "cache_write": None,
                "long_context": copy.deepcopy(manifest["pricing"]["long_context"]),
                "effective_from": "2026-08-31",
                "evidence_ids": ["official-price"],
            }
        ]
        return manifest

    def test_v1_manifest_remains_valid(self):
        module.validate(self.valid_manifest())

    def test_v1_plan_reports_every_v2_migration_gap(self):
        gaps = module.migration_gaps(self.valid_manifest())
        for section in module.V2_SECTIONS:
            self.assertTrue(any(section in gap for gap in gaps), section)

    def test_valid_v2_manifest_is_release_complete(self):
        manifest = self.v2_manifest()
        module.validate(manifest)
        self.assertEqual([], module.contract_gaps(manifest))

    def test_rejects_secret_shaped_field(self):
        manifest = self.v2_manifest()
        manifest["evidence"][0]["authorization"] = "not-allowed"
        with self.assertRaisesRegex(ValueError, "credential-shaped"):
            module.validate(manifest)

    def test_rejects_generic_latest_alias(self):
        manifest = self.valid_manifest()
        manifest["production"]["aliases"] = ["grok-latest"]
        with self.assertRaisesRegex(ValueError, "explicit rollout"):
            module.validate(manifest)

    def test_accepts_zero_cache_price(self):
        manifest = self.v2_manifest()
        manifest["pricing"]["cached_input_per_mtok_usd"] = 0
        manifest["price_matrix"][0]["cached_input"] = 0
        module.validate(manifest)

    def test_openai_default_requires_provider_codex_entry(self):
        manifest = self.valid_manifest()
        manifest["model"]["platform"] = "openai"
        with self.assertRaisesRegex(ValueError, "codex_catalog_entry"):
            module.validate(manifest)

    def test_rejects_duplicate_protocol_rows(self):
        manifest = self.v2_manifest()
        manifest["protocol_matrix"].append(copy.deepcopy(manifest["protocol_matrix"][0]))
        with self.assertRaisesRegex(ValueError, "duplicate protocol_matrix"):
            module.validate(manifest)

    def test_rejects_dangling_evidence_id(self):
        manifest = self.v2_manifest()
        manifest["reasoning_matrix"][0]["evidence_ids"] = ["missing"]
        with self.assertRaisesRegex(ValueError, "unknown evidence ids"):
            module.validate(manifest)

    def test_rejects_price_projection_drift(self):
        manifest = self.v2_manifest()
        manifest["price_matrix"][0]["output"] = 7
        with self.assertRaisesRegex(ValueError, "legacy pricing projection"):
            module.validate(manifest)

    def test_rejects_supported_reasoning_without_wire_value(self):
        manifest = self.v2_manifest()
        manifest["reasoning_matrix"][0]["wire_value"] = None
        with self.assertRaisesRegex(ValueError, "wire_value is required"):
            module.validate(manifest)

    def test_rejects_feature_row_for_undeclared_protocol(self):
        manifest = self.v2_manifest()
        row = copy.deepcopy(manifest["protocol_feature_matrix"][0])
        row["protocol"] = "messages"
        manifest["protocol_feature_matrix"].append(row)
        with self.assertRaisesRegex(ValueError, "must reference protocol_matrix"):
            module.validate(manifest)

    def test_reasoning_feature_requires_supported_effort(self):
        manifest = self.v2_manifest()
        manifest["reasoning_matrix"] = []
        with self.assertRaisesRegex(ValueError, "requires a supported effort"):
            module.validate(manifest)

    def test_rejects_evidence_without_url_or_digest(self):
        manifest = self.v2_manifest()
        manifest["evidence"][1]["artifact_sha256"] = None
        with self.assertRaisesRegex(ValueError, "requires url or artifact_sha256"):
            module.validate(manifest)

    def test_draft_template_exposes_untested_contract_gaps(self):
        args = argparse.Namespace(platform="gemini", model_id="gemini-3", display_name="Gemini 3")
        manifest = module.template(args)
        self.assertEqual(2, manifest["schema_version"])
        self.assertEqual("generate_content", manifest["capabilities"]["protocol"])
        self.assertEqual("untested", manifest["protocol_matrix"][0]["support"])
        gaps = module.contract_gaps(manifest)
        self.assertTrue(any("basic_request is untested" in gap for gap in gaps))
        self.assertTrue(any("placeholder URL" in gap for gap in gaps))
        self.assertTrue(any("not public" in gap for gap in gaps))

    def test_deprecated_lifecycle_requires_date(self):
        manifest = self.v2_manifest()
        manifest["lifecycle"]["public_status"] = "deprecated"
        with self.assertRaisesRegex(ValueError, "deprecated_at"):
            module.validate(manifest)


if __name__ == "__main__":
    unittest.main()

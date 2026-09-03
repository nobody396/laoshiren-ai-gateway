#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("model_catalog.py")
SPEC = importlib.util.spec_from_file_location("model_catalog", SCRIPT)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def grok_default_row(catalog: dict) -> dict:
    return next(row for row in catalog["models"] if row["id"] == "grok-4.6")


class ModelCatalogTest(unittest.TestCase):
    def test_repository_catalog_is_valid_and_deterministic(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        self.assertEqual(catalog["pricing_contract"]["default_scope"], "provider_public")
        self.assertEqual(
            {row["id"] for row in catalog["models"] if row.get("public_price_visibility") == "hidden"},
            {"deepseek-v4-flash", "glm-5.3-flash"},
        )
        spark_gap = next(row for row in catalog["price_gaps"] if row["model_id"] == "gpt-5.3-codex-spark")
        self.assertEqual(spark_gap["status"], "not_published")
        self.assertIn("not final", spark_gap["reason"])
        self.assertTrue(spark_gap["evidence_url"].startswith("https://help.openai.com/"))
        gemini_pro = next(row for row in catalog["models"] if row["id"] == "gemini-3.1-pro")
        gemini_flash = next(row for row in catalog["models"] if row["id"] == "gemini-3.7-flash")
        self.assertEqual(gemini_pro["pricing"]["cached_input_per_mtok_usd"], 0)
        self.assertEqual(gemini_pro["provider_pricing"]["cached_input_per_mtok_usd"], 0.2)
        self.assertEqual(gemini_flash["provider_pricing"]["cached_input_per_mtok_usd"], 0.075)
        codex_models = [
            model["slug"]
            for model in json.loads(MODULE.render_codex_client_catalog(catalog))["models"]
        ]
        self.assertIn("grok-4.6", MODULE.render_go(catalog))
        self.assertIn('"id": "grok-4.6"', MODULE.render_ts(catalog))
        self.assertIn("export const codexClientModels", MODULE.render_ts(catalog))
        self.assertIn("export const clientAutoConfigDefaults", MODULE.render_ts(catalog))
        self.assertIn('"anthropic": "claude-opus-5"', MODULE.render_ts(catalog))
        self.assertNotIn("gpt-5.6-luna", codex_models)
        self.assertNotIn("gpt-5.4-mini", codex_models)
        self.assertIn("$CatalogGrokDefaultModel = 'grok-4.6'", MODULE.render_powershell_block(catalog))
        self.assertIn("CATALOG_GROK_DEFAULT_MODEL='grok-4.6'", MODULE.render_shell_block(catalog))
        self.assertEqual(
            MODULE.installer_model_values(catalog)["grok"]["managed_models"],
            [
                {"id": "grok-4.5", "display_name": "Grok 4.5", "context_window": 500000},
                {"id": "grok-4.6", "display_name": "Grok 4.6", "context_window": 500000},
            ],
        )
        self.assertEqual(
            MODULE.render_codex_client_catalog(catalog),
            MODULE.CODEX_CLIENT_OUTPUT.read_text(encoding="utf-8"),
        )
        self.assertEqual(MODULE.render_go(catalog), MODULE.render_go(catalog))

    def test_rejects_duplicate_defaults(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        duplicate = json.loads(json.dumps(catalog))
        row = json.loads(json.dumps(grok_default_row(duplicate)))
        row["id"] = "grok-next"
        row["upstream_id"] = "grok-next"
        duplicate["models"].append(row)
        with self.assertRaisesRegex(ValueError, "multiple client defaults"):
            MODULE.validate_catalog(duplicate)

    def test_rejects_secret_shaped_fields(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        catalog["api_key"] = "not-allowed"
        with self.assertRaisesRegex(ValueError, "credential-shaped"):
            MODULE.validate_catalog(catalog)

    def test_rejects_null_price_claimed_as_verified(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        gemini = next(row for row in catalog["models"] if row["id"] == "gemini-3.1-pro")
        gemini["provider_pricing"]["cached_input_per_mtok_usd"] = None
        gemini["provider_pricing"]["component_status"]["cached_input"] = "verified"
        with self.assertRaisesRegex(ValueError, "cannot be verified when price is null"):
            MODULE.validate_catalog(catalog)

    def test_rejects_grok_predecessor_without_explicit_model_metadata(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        grok_default_row(catalog)["client_config"]["managed_predecessor_models"] = []
        with self.assertRaisesRegex(ValueError, "must describe every Grok predecessor"):
            MODULE.validate_catalog(catalog)

    def test_manifest_updates_catalog_without_losing_legacy_group(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        manifest = {
            "model": {
                "id": "grok-4.6",
                "upstream_id": "grok-4.6",
                "display_name": "Grok 4.6",
                "platform": "grok",
                "context_window": 500000,
                "max_output_tokens": 128000,
            },
            "pricing": {
                "input_per_mtok_usd": 2,
                "cached_input_per_mtok_usd": 0.5,
                "output_per_mtok_usd": 6,
                "evidence_url": "https://example.com/pricing",
                "long_context": {
                    "input_threshold": 200000,
                    "input_multiplier": 2,
                    "output_multiplier": 2,
                },
            },
            "production": {
                "group_name": "Grok 分组",
                "client_default": True,
            },
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(manifest), encoding="utf-8")
            merged = MODULE.merge_manifest(catalog, path)
        row = grok_default_row(merged)
        self.assertEqual(row["public_group"]["preferred_name"], "Grok")
        self.assertEqual(row["public_group"]["legacy_names"], ["Grok 4.6", "Grok 4.5", "Grok"])

    def test_new_default_replaces_prior_platform_default(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        manifest = {
            "model": {
                "id": "grok-4.7",
                "upstream_id": "grok-4.7",
                "display_name": "Grok 4.7",
                "platform": "grok",
                "context_window": 500000,
                "max_output_tokens": 128000,
            },
            "pricing": {
                "input_per_mtok_usd": 2,
                "cached_input_per_mtok_usd": None,
                "output_per_mtok_usd": 6,
                "evidence_url": "https://example.com/pricing",
                "long_context": None,
            },
            "production": {
                "group_name": "Grok 4.7",
                "legacy_group_names": ["Grok 4.6", "Grok 4.5"],
                "client_default": True,
            },
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(manifest), encoding="utf-8")
            merged = MODULE.merge_manifest(catalog, path)
        defaults = [row["id"] for row in merged["models"] if row["client_default"]]
        self.assertEqual(defaults, ["gemini-3.7-flash", "grok-4.7"])
        self.assertEqual(
            merged["client_auto_config_version"],
            MODULE.bump_patch(catalog["client_auto_config_version"]),
        )
        self.assertEqual(
            MODULE.installer_model_values(merged)["grok"]["managed_ids"],
            ["grok-4.5", "grok-4.6", "grok-4.7"],
        )

    def test_reapplying_same_default_does_not_bump_installer_version(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        current = grok_default_row(catalog)
        manifest = {
            "model": {
                "id": current["id"],
                "upstream_id": current["upstream_id"],
                "display_name": current["display_name"],
                "platform": current["platform"],
                "context_window": current["context_window"],
                "max_output_tokens": current["max_output_tokens"],
            },
            "pricing": {
                "input_per_mtok_usd": current["pricing"]["input_per_mtok_usd"],
                "cached_input_per_mtok_usd": current["pricing"]["cached_input_per_mtok_usd"],
                "output_per_mtok_usd": current["pricing"]["output_per_mtok_usd"],
                "evidence_url": current["pricing"]["evidence_url"],
                "long_context": {
                    "input_threshold": current["pricing"]["long_context_input_threshold"],
                    "input_multiplier": current["pricing"]["long_context_input_multiplier"],
                    "output_multiplier": current["pricing"]["long_context_output_multiplier"],
                },
            },
            "production": {
                "group_name": current["public_group"]["preferred_name"],
                "client_default": True,
            },
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(manifest), encoding="utf-8")
            merged = MODULE.merge_manifest(catalog, path)
        self.assertEqual(merged["client_auto_config_version"], catalog["client_auto_config_version"])

    def test_generated_block_replacement_is_bounded(self) -> None:
        source = "before\n# BEGIN\nold\n# END\nafter\n"
        result = MODULE.replace_generated_block(source, "# BEGIN", "# END", "# BEGIN\nnew\n# END")
        self.assertEqual(result, "before\n# BEGIN\nnew\n# END\nafter\n")

    def test_openai_manifest_drives_installer_and_codex_catalog(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        manifest = {
            "model": {
                "id": "gpt-next",
                "upstream_id": "gpt-next",
                "display_name": "GPT Next",
                "platform": "openai",
                "context_window": 300000,
                "max_output_tokens": 128000,
                "codex_catalog_entry": {
                    "slug": "gpt-next",
                    "display_name": "GPT Next",
                    "base_instructions": "You are Codex.",
                    "supports_reasoning_summaries": True,
                    "visibility": "list",
                    "context_window": 300000,
                    "max_context_window": 300000,
                    "auto_compact_token_limit": 270000,
                },
            },
            "pricing": {
                "input_per_mtok_usd": 2,
                "cached_input_per_mtok_usd": 0.2,
                "output_per_mtok_usd": 8,
                "evidence_url": "https://example.com/pricing",
                "long_context": None,
            },
            "production": {
                "group_name": "GPT Next",
                "legacy_group_names": ["GPT 5.6"],
                "client_default": True,
            },
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(manifest), encoding="utf-8")
            merged = MODULE.merge_manifest(catalog, path)
        self.assertEqual(MODULE.installer_model_values(merged)["openai"]["id"], "gpt-next")
        codex_catalog = json.loads(MODULE.render_codex_client_catalog(merged))
        self.assertEqual(codex_catalog["models"][0]["slug"], "gpt-next")

    def test_anthropic_manifest_drives_installer_default(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        manifest = {
            "model": {
                "id": "claude-next",
                "upstream_id": "claude-next",
                "display_name": "Claude Next",
                "platform": "anthropic",
                "context_window": 250000,
                "max_output_tokens": 64000,
            },
            "pricing": {
                "input_per_mtok_usd": 5,
                "cached_input_per_mtok_usd": 0.5,
                "output_per_mtok_usd": 25,
                "evidence_url": "https://example.com/pricing",
                "long_context": None,
            },
            "production": {
                "group_name": "Claude Next",
                "client_default": True,
            },
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(manifest), encoding="utf-8")
            merged = MODULE.merge_manifest(catalog, path)
        self.assertEqual(MODULE.installer_model_values(merged)["anthropic"]["id"], "claude-next")

    def test_gemini_installer_values_render_in_generated_blocks(self) -> None:
        catalog = MODULE.load_catalog(MODULE.DEFAULT_CATALOG)
        values = MODULE.installer_model_values(catalog)
        self.assertEqual(values["gemini"]["id"], "gemini-3.7-flash")
        self.assertEqual(
            values["gemini"]["managed_ids"],
            ["gemini-3.1-pro", "gemini-3.7-flash"],
        )
        shell_block = MODULE.render_shell_block(catalog)
        self.assertIn("CATALOG_GEMINI_DEFAULT_MODEL='gemini-3.7-flash'", shell_block)
        self.assertIn(
            "CATALOG_GEMINI_MANAGED_MODELS='gemini-3.1-pro gemini-3.7-flash'",
            shell_block,
        )
        powershell_block = MODULE.render_powershell_block(catalog)
        self.assertIn("$CatalogGeminiDefaultModel = 'gemini-3.7-flash'", powershell_block)
        self.assertIn(
            "$CatalogGeminiManagedModels = @('gemini-3.1-pro', 'gemini-3.7-flash')",
            powershell_block,
        )


if __name__ == "__main__":
    unittest.main()

from __future__ import annotations

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "model_price_matrix.py"


def catalog(*models: dict) -> dict:
    return {"schema_version": 1, "models": list(models)}


def model(model_id: str, *, group: str = "Test", input_price: float = 2,
          output_price: float = 8, cache_read: float | None = 0.5,
          long_context: tuple[int, float, float] | None = None) -> dict:
    threshold, input_multiplier, output_multiplier = long_context or (None, None, None)
    return {
        "id": model_id,
        "platform": "openai",
        "public_group": {"preferred_name": group, "legacy_names": []},
        "pricing": {
            "input_per_mtok_usd": input_price,
            "output_per_mtok_usd": output_price,
            "cached_input_per_mtok_usd": cache_read,
            "long_context_input_threshold": threshold,
            "long_context_input_multiplier": input_multiplier,
            "long_context_output_multiplier": output_multiplier,
            "evidence_url": f"https://example.test/{model_id}",
        },
    }


def inventory(groups: list[dict], *, unit: str = "per_1m_tokens") -> dict:
    return {
        "updated_at": "2026-08-31T00:00:00Z",
        "currency": "CNY",
        "unit": unit,
        "groups": groups,
    }


def group(models: list[dict], *, group_id: int = 7, name: str = "Test",
          multiplier: float = 0.5, image: dict | None = None) -> dict:
    result = {
        "group_id": group_id,
        "name": name,
        "platform": "openai",
        "rate_multiplier": multiplier,
        "is_exclusive": False,
        "subscription_type": "standard",
        "models": models,
    }
    if image is not None:
        result["image_generation"] = image
    return result


class ModelPriceMatrixTest(unittest.TestCase):
    def run_cli(self, command: str, catalog_data: dict, public_data: dict,
                db_data: dict | None = None, *, via_url: bool = False):
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            catalog_path = tmp / "catalog.json"
            inventory_path = tmp / "inventory.json"
            catalog_path.write_text(json.dumps(catalog_data), encoding="utf-8")
            inventory_path.write_text(json.dumps(public_data), encoding="utf-8")
            args = [sys.executable, str(SCRIPT), command, "--catalog", str(catalog_path)]
            server = None
            thread = None
            if via_url:
                body = inventory_path.read_bytes()

                class Handler(BaseHTTPRequestHandler):
                    def do_GET(self):
                        self.send_response(200)
                        self.send_header("Content-Type", "application/json")
                        self.end_headers()
                        self.wfile.write(body)

                    def log_message(self, _format, *args):
                        return

                server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
                thread = threading.Thread(target=server.serve_forever, daemon=True)
                thread.start()
                args.extend(["--pricing-url", f"http://127.0.0.1:{server.server_port}/pricing"])
            else:
                args.extend(["--inventory-json", str(inventory_path)])
            if db_data is not None:
                db_path = tmp / "db.json"
                db_path.write_text(json.dumps(db_data), encoding="utf-8")
                args.extend(["--db-export", str(db_path)])
            try:
                completed = subprocess.run(args, cwd=ROOT, text=True, capture_output=True, check=False)
            finally:
                if server:
                    server.shutdown()
                    server.server_close()
                if thread:
                    thread.join(timeout=2)
            self.assertEqual(completed.stderr, "")
            return completed, json.loads(completed.stdout)

    def test_audit_normalizes_token_long_context_cache_and_image_prices(self):
        catalog_data = catalog(model("alpha", long_context=(200_000, 2, 1.5)))
        public_data = inventory([
            group(
                [{
                    "model": "alpha",
                    "input_price": 1,
                    "output_price": 4,
                    "cache_read_price": 0.25,
                    "cache_write_price": 0.625,
                    "long_context": {
                        "input_threshold": 200_000,
                        "input_multiplier": 2,
                        "output_multiplier": 1.5,
                    },
                }],
                image={"mode": "fixed_per_image", "price_per_image": 0.3},
            )
        ])
        completed, report = self.run_cli("audit", catalog_data, public_data, via_url=True)
        self.assertEqual(completed.returncode, 0)
        self.assertTrue(report["complete"])
        self.assertEqual(report["kind"], "model_price_matrix")
        self.assertEqual(report["schema_version"], 2)
        self.assertEqual(report["summary"]["status_counts"], {"not_applicable": 1, "verified": 3})
        alpha = next(row for row in report["rows"] if row["model_id"] == "alpha" and row["price_scope"] == "group_customer")
        self.assertEqual(alpha["cache_write"], 0.625)
        self.assertEqual(alpha["long_context_threshold"], 200_000)
        self.assertEqual(alpha["evidence"]["catalog"]["url"], "https://example.test/alpha")
        image_row = next(row for row in report["rows"] if row["model_id"] == "gpt-image-2")
        self.assertEqual(image_row["image_price_per_image"], 0.3)
        self.assertEqual(image_row["status"], "not_applicable")

    def test_plan_reports_missing_conflict_and_disabled_without_failing(self):
        catalog_data = catalog(model("alpha"), model("catalog-only"))
        public_data = inventory([
            group([
                {"model": "alpha", "input_price": 9, "output_price": 4, "cache_read_price": 0.25},
                {"model": "uncataloged", "input_price": 1, "output_price": 2},
                {"model": "retired", "disabled": True, "input_price": 1, "output_price": 2},
            ])
        ])
        completed, report = self.run_cli("plan", catalog_data, public_data)
        self.assertEqual(completed.returncode, 0)
        self.assertFalse(report["complete"])
        self.assertEqual(set(report["summary"]["drift_columns"]), {"model_id", "provider_public"})
        alpha_scopes = {row["price_scope"] for row in report["rows"] if row["model_id"] == "alpha"}
        self.assertEqual(alpha_scopes, {"provider_public", "gateway_base", "group_customer"})
        self.assertEqual({item["type"] for item in report["issues"]},
                         {"blocked", "missing_public"})

    def test_provider_and_customer_scope_difference_is_not_a_conflict(self):
        completed, report = self.run_cli(
            "audit",
            catalog(model("alpha")),
            inventory([group([{"model": "alpha", "input_price": 1.1,
                                        "output_price": 4, "cache_read_price": 0.25}])]),
        )
        self.assertEqual(completed.returncode, 0)
        self.assertTrue(report["complete"])
        self.assertEqual(report["issues"], [])
        provider = next(row for row in report["rows"] if row["price_scope"] == "provider_public")
        gateway = next(row for row in report["rows"] if row["price_scope"] == "gateway_base")
        customer = next(row for row in report["rows"] if row["price_scope"] == "group_customer")
        self.assertEqual(provider["input"], 2)
        self.assertEqual(gateway["input"], 2.2)
        self.assertEqual(customer["input"], 1.1)

    def test_database_export_per_token_is_normalized_and_compared(self):
        public_data = inventory([group([
            {"model": "alpha", "input_price": 1, "output_price": 4, "cache_read_price": 0.25}
        ])])
        db_data = {
            "currency": "CNY",
            "unit": "per_token",
            "rows": [{
                "model_id": "alpha",
                "group_id": 7,
                "group_name": "Test",
                "platform": "openai",
                "rate_multiplier": 0.5,
                "input": 0.000001,
                "output": 0.000004,
                "cache_read": 0.00000025,
            }],
        }
        completed, report = self.run_cli("audit", catalog(model("alpha")), public_data, db_data)
        self.assertEqual(completed.returncode, 0)
        self.assertTrue(report["complete"])
        self.assertEqual(report["summary"]["database_rows"], 1)
        customer = next(row for row in report["rows"] if row["price_scope"] == "group_customer")
        self.assertEqual(customer["evidence"]["database"]["source"].split("/")[-1], "db.json")

        db_data["rows"][0]["output"] = 0.000005
        completed, report = self.run_cli("audit", catalog(model("alpha")), public_data, db_data)
        self.assertEqual(completed.returncode, 2)
        self.assertEqual(next(item for item in report["issues"] if item["column"] == "output")["expected"], 5.0)

        db_data["rows"][0]["output"] = 0.000004
        db_data["rows"][0]["rate_multiplier"] = 0.7
        completed, report = self.run_cli("plan", catalog(model("alpha")), public_data, db_data)
        self.assertEqual(completed.returncode, 0)
        self.assertEqual(next(item for item in report["issues"] if item["column"] == "rate_multiplier")["expected"], 0.7)

    def test_zero_customer_cache_is_not_null_provider_cache(self):
        gemini = model("gemini", cache_read=None)
        gemini["pricing"]["component_status"] = {"cached_input": "unknown"}
        completed, report = self.run_cli(
            "plan",
            catalog(gemini),
            inventory([group([{
                "model": "gemini", "input_price": 1, "output_price": 4,
                "cache_read_price": 0,
            }])]),
        )
        self.assertEqual(completed.returncode, 0)
        provider = next(row for row in report["rows"] if row["price_scope"] == "provider_public")
        customer = next(row for row in report["rows"] if row["price_scope"] == "group_customer")
        self.assertIsNone(provider["cache_read"])
        self.assertEqual(provider["component_status"]["cached_input"], "unknown")
        self.assertEqual(customer["cache_read"], 0)
        self.assertEqual(customer["component_status"]["cached_input"], "verified")

    def test_provider_long_context_does_not_conflict_with_unknown_runtime_tier(self):
        completed, report = self.run_cli(
            "plan",
            catalog(model("tiered", long_context=(200_000, 2, 1.5))),
            inventory([group([{
                "model": "tiered", "input_price": 1, "output_price": 4,
                "cache_read_price": 0.25,
            }])]),
        )
        self.assertEqual(completed.returncode, 0)
        self.assertFalse(report["complete"])
        self.assertFalse(any(item["type"] == "conflict" for item in report["issues"]))
        problem = next(item for item in report["issues"] if item["column"] == "long_context")
        self.assertEqual(problem["type"], "unknown")
        self.assertIn("not exposed", problem["message"])

    def test_price_gap_blocks_provider_scope_without_guessing_model_catalog(self):
        catalog_data = catalog()
        catalog_data["price_gaps"] = [{
            "model_id": "spark", "price_scope": "provider_public", "status": "blocked",
            "reason": "max_output_tokens is not verified; do not guess it",
        }]
        completed, report = self.run_cli(
            "plan", catalog_data,
            inventory([group([{
                "model": "spark", "input_price": 0.625, "output_price": 5,
                "cache_read_price": 0.0625,
            }])]),
        )
        self.assertEqual(completed.returncode, 0)
        blocked = [row for row in report["rows"] if row["model_id"] == "spark" and row["price_scope"] == "provider_public"]
        self.assertEqual(len(blocked), 1)
        self.assertEqual(blocked[0]["status"], "blocked")
        self.assertIn("do not guess", blocked[0]["evidence"]["catalog"]["gap_reason"])
        self.assertEqual(sum(item["type"] == "blocked" for item in report["issues"]), 1)

    def test_official_not_published_price_is_terminal_without_a_numeric_guess(self):
        catalog_data = catalog()
        catalog_data["price_gaps"] = [{
            "model_id": "spark", "price_scope": "provider_public", "status": "not_published",
            "reason": "Official research-preview pricing is not final",
            "evidence_url": "https://example.test/spark-pricing-status",
        }]
        completed, report = self.run_cli(
            "audit", catalog_data,
            inventory([group([{
                "model": "spark", "input_price": 0.625, "output_price": 5,
                "cache_read_price": 0.0625,
            }])]),
        )
        self.assertEqual(completed.returncode, 0)
        self.assertTrue(report["complete"])
        provider = next(row for row in report["rows"] if row["price_scope"] == "provider_public")
        self.assertEqual(provider["status"], "not_published")
        self.assertIsNone(provider["input"])
        self.assertIn("not final", provider["component_evidence"]["input"])
        self.assertEqual(report["issues"], [])

    def test_non_openai_runtime_long_context_is_terminal_not_exposed(self):
        tiered = model("tiered", long_context=(200_000, 2, 1.5))
        tiered["platform"] = "gemini"
        public = inventory([group([{
            "model": "tiered", "input_price": 1, "output_price": 4,
            "cache_read_price": 0.25,
        }])])
        public["groups"][0]["platform"] = "gemini"
        completed, report = self.run_cli("audit", catalog(tiered), public)
        self.assertEqual(completed.returncode, 0)
        self.assertTrue(report["complete"])
        customer = next(row for row in report["rows"] if row["price_scope"] == "group_customer")
        self.assertEqual(customer["component_status"]["long_context"], "not_exposed")
        self.assertEqual(customer["status"], "not_exposed")
        self.assertIn("publicLongContextPricing", customer["component_evidence"]["long_context"])

    def test_provider_cache_override_and_free_customer_cache_are_both_terminal(self):
        gemini = model("gemini", cache_read=0)
        gemini["platform"] = "gemini"
        gemini["provider_pricing"] = {
            **gemini["pricing"],
            "price_scope": "provider_public",
            "cached_input_per_mtok_usd": 0.075,
            "component_status": {"cached_input": "verified"},
        }
        public = inventory([group([{
            "model": "gemini", "input_price": 1, "output_price": 4,
            "cache_read_price": 0,
        }])])
        public["groups"][0]["platform"] = "gemini"
        completed, report = self.run_cli("audit", catalog(gemini), public)
        self.assertEqual(completed.returncode, 0)
        provider = next(row for row in report["rows"] if row["price_scope"] == "provider_public")
        customer = next(row for row in report["rows"] if row["price_scope"] == "group_customer")
        self.assertEqual(provider["cache_read"], 0.075)
        self.assertEqual(customer["cache_read"], 0)
        self.assertEqual(report["issues"], [])

    def test_explicit_hidden_catalog_model_needs_no_public_row(self):
        hidden = model("hidden")
        hidden["public_price_visibility"] = "hidden"
        completed, report = self.run_cli("audit", catalog(hidden), inventory([]))
        self.assertEqual(completed.returncode, 0)
        self.assertTrue(report["complete"])
        self.assertEqual(report["rows"][0]["public_price_visibility"], "hidden")
        self.assertEqual(report["rows"][0]["price_scope"], "provider_public")

    def test_invalid_inventory_emits_machine_readable_failure(self):
        completed, report = self.run_cli(
            "plan",
            catalog(model("alpha")),
            {"currency": "CNY", "unit": "bananas", "groups": []},
        )
        self.assertEqual(completed.returncode, 2)
        self.assertFalse(report["complete"])
        self.assertEqual(report["issues"][0]["type"], "input_error")
        self.assertIn("unsupported price unit", report["issues"][0]["message"])


if __name__ == "__main__":
    unittest.main()

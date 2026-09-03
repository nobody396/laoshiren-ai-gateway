import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).parents[1] / "scripts" / "provider_contract_runner.py"
SPEC = importlib.util.spec_from_file_location("provider_contract_runner", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class ProviderContractRunnerTest(unittest.TestCase):
    def manifest(self):
        return {
            "schema_version": 1,
            "model": {
                "id": "test-code-1",
                "upstream_id": "provider-test-code-1",
                "display_name": "Test Code 1",
                "platform": "openai",
                "context_window": 128000,
                "max_output_tokens": 8192,
            },
            "capabilities": {
                "protocols": ["chat_completions", "responses", "messages", "generate_content"],
                "streaming": True,
                "tools": True,
                "image_input": True,
                "cache_read": True,
                "reasoning_efforts": ["low", "high"],
            },
            "pricing": {
                "input_per_mtok_usd": 2,
                "cached_input_per_mtok_usd": 0.5,
                "output_per_mtok_usd": 6,
                "evidence_url": "https://example.test/pricing",
                "long_context": None,
            },
            "production": {"group_name": "Test"},
            "verification": {"expected_text": "CONTRACT_OK"},
        }

    def observation(self, case):
        base = {
            "evidence": {"source": "offline_fixture", "id": f"ev-{case.case_id}"},
            "observed_at": "2026-08-31T08:00:00+08:00",
        }
        capability = case.capability
        if capability == "request":
            base.update(status_code=200, text="CONTRACT_OK")
        elif capability == "usage":
            base["usage"] = {"input_tokens": 100, "output_tokens": 20, "total_tokens": 120}
        elif capability == "billing_receipt":
            base.update(
                usage={"input_tokens": 100, "output_tokens": 20, "cached_input_tokens": 0, "total_tokens": 120},
                billed_usd=0.00032,
                request_id="req-1",
                usage_id="usage-1",
                accounting_command_id="acct-1",
            )
        elif capability == "error_passthrough":
            base.update(status_code=400, error={"code": "invalid_request", "message": "bad field"})
        elif capability == "interrupted_stream":
            base.update(complete=False, classification="upstream_stream_interrupted")
        elif capability == "timeout":
            base.update(timed_out=True, classification="upstream_timeout")
        elif capability == "retry":
            base.update(attempts=2, final_status_code=200)
        elif capability == "streaming_terminal":
            terminal = {
                "responses": "response.completed",
                "chat_completions": "[DONE]",
                "messages": "message_stop",
                "generate_content": "STOP",
            }[case.protocol]
            base.update(events=[{"type": "chunk"}, {"type": terminal}], complete=True, text="CONTRACT_OK")
        elif capability == "tool_call":
            base["tool_call"] = {"name": "get_weather", "arguments": {"city": "Beijing"}}
        elif capability == "tool_result_continuation":
            base.update(correlated=True, final_text="tool result accepted")
        elif capability == "prompt_cache":
            base["usage"] = {"input_tokens": 100, "output_tokens": 20, "cached_input_tokens": 80, "total_tokens": 120}
        elif capability == "image_input":
            base.update(accepted=True, final_text="image understood")
        elif capability.startswith("reasoning."):
            base.update(effort=capability.split(".", 1)[1], reasoning={"summary": "bounded evidence"})
        elif capability == "context_window":
            base.update(declared=128000, source_kind="official")
        elif capability == "max_output_tokens":
            base.update(declared=8192, source_kind="official")
        elif capability == "web_search":
            base.update(citations=["https://example.test/source"], final_text="search result")
        elif capability == "structured_output":
            base.update(schema_valid=True, output={"ok": True})
        else:
            self.fail(f"fixture factory missed {capability}")
        return base

    def fixture(self, manifest=None):
        manifest = manifest or self.manifest()
        plan = MODULE.build_plan(manifest)
        observations = {}
        for item in plan["cases"]:
            case = MODULE.ContractCase(item["id"], item["protocol"], item["capability"], item["expectation"])
            observations[case.case_id] = self.observation(case)
        return {"schema_version": 1, "model_id": manifest["model"]["id"], "observations": observations}

    def v2_manifest(self):
        manifest = self.manifest()
        manifest["schema_version"] = 2
        all_pass = {name: "pass" for name in MODULE.V2_FEATURES}
        manifest.update(
            protocol_matrix=[
                {
                    "protocol": "responses", "support": "supported", "recommendation": "preferred",
                    "recommendation_reason": "native contract", "evidence_ids": ["ev-pass"],
                },
                {
                    "protocol": "messages", "support": "unsupported", "recommendation": "not_applicable",
                    "recommendation_reason": "provider does not expose it", "evidence_ids": ["ev-fail"],
                },
            ],
            protocol_feature_matrix=[
                {"protocol": "responses", "features": all_pass, "evidence_ids": ["ev-pass"]},
                {
                    "protocol": "messages",
                    "features": {name: "not_applicable" for name in MODULE.V2_FEATURES},
                    "evidence_ids": ["ev-fail"],
                },
            ],
            reasoning_matrix=[
                {
                    "protocol": "responses", "effort": "high", "support": "supported",
                    "wire_value": "high", "evidence_ids": ["ev-pass"],
                }
            ],
            price_matrix=[
                {
                    "scope": "default", "currency": "USD", "unit": "per_mtok", "input": 2,
                    "output": 6, "cached_input": 0.5, "cache_write": None, "long_context": None,
                    "effective_from": "2026-08-31", "evidence_ids": ["ev-pass"],
                }
            ],
            lifecycle={
                "public_status": "draft", "introduced_at": None, "deprecated_at": None,
                "replacement_model_id": None,
            },
            evidence=[
                {
                    "id": "ev-pass", "kind": "release_artifact", "url": None,
                    "observed_at": "2026-08-31T08:00:00+08:00", "subject_version": "fixture-v1",
                    "result": "pass", "artifact_sha256": "a" * 64,
                },
                {
                    "id": "ev-fail", "kind": "official_docs", "url": "https://example.test/unsupported",
                    "observed_at": "2026-08-31T08:00:00+08:00", "subject_version": "docs-v1",
                    "result": "fail", "artifact_sha256": None,
                },
            ],
        )
        return manifest

    def test_plan_covers_every_protocol_and_claimed_capability(self):
        plan = MODULE.build_plan(self.manifest())
        ids = {case["id"] for case in plan["cases"]}
        for protocol in ("chat_completions", "responses", "messages", "generate_content"):
            for capability in (
                "request", "streaming_terminal", "tool_call", "tool_result_continuation",
                "prompt_cache", "image_input", "reasoning.low", "reasoning.high",
                "error_passthrough", "interrupted_stream", "timeout", "retry", "usage", "billing_receipt",
            ):
                self.assertIn(f"{protocol}.{capability}", ids)
        self.assertEqual(plan["network_execution"], "disabled")

    def test_valid_complete_fixture_passes(self):
        manifest = self.manifest()
        receipt = MODULE.run_fixture(manifest, self.fixture(manifest))
        self.assertEqual(receipt["status"], "passed")
        self.assertEqual(receipt["summary"]["failed"], 0)
        self.assertEqual(receipt["summary"]["planned"], receipt["summary"]["passed"])
        self.assertNotIn("observations", receipt)

    def test_missing_evidence_fails_closed(self):
        manifest = self.manifest()
        fixture = self.fixture(manifest)
        fixture["observations"].pop("responses.tool_call")
        receipt = MODULE.run_fixture(manifest, fixture)
        failed = {item["id"]: item["reason"] for item in receipt["results"] if item["status"] == "failed"}
        self.assertEqual(failed["responses.tool_call"], "missing observation")

    def test_incomplete_stream_and_bad_billing_fail(self):
        manifest = self.manifest()
        fixture = self.fixture(manifest)
        fixture["observations"]["responses.streaming_terminal"]["events"] = [{"type": "response.output_text.delta"}]
        fixture["observations"]["responses.billing_receipt"]["billed_usd"] = 999
        receipt = MODULE.run_fixture(manifest, fixture)
        failed = {item["id"] for item in receipt["results"] if item["status"] == "failed"}
        self.assertIn("responses.streaming_terminal", failed)
        self.assertIn("responses.billing_receipt", failed)

    def test_billing_applies_long_context_multipliers(self):
        manifest = self.manifest()
        manifest["pricing"]["long_context"] = {
            "input_threshold": 50, "input_multiplier": 2, "output_multiplier": 3,
        }
        fixture = self.fixture(manifest)
        for case_id, observation in fixture["observations"].items():
            if case_id.endswith(".billing_receipt"):
                observation["billed_usd"] = 0.00076
        receipt = MODULE.run_fixture(manifest, fixture)
        self.assertEqual(receipt["status"], "passed")

    def test_capability_false_omits_feature_but_not_reliability_baseline(self):
        manifest = self.manifest()
        manifest["capabilities"].update(streaming=False, tools=False, image_input=False, cache_read=False, reasoning_efforts=[])
        ids = {case["id"] for case in MODULE.build_plan(manifest)["cases"]}
        self.assertNotIn("responses.streaming_terminal", ids)
        self.assertNotIn("responses.tool_call", ids)
        self.assertNotIn("responses.image_input", ids)
        self.assertIn("responses.timeout", ids)
        self.assertIn("responses.billing_receipt", ids)

    def test_rejects_secret_in_manifest_or_fixture(self):
        manifest = self.manifest()
        manifest["provider_api_key"] = "not-even-a-real-key"
        with self.assertRaisesRegex(MODULE.ContractError, "credential-shaped"):
            MODULE.build_plan(manifest)
        manifest = self.manifest()
        fixture = self.fixture(manifest)
        fixture["authorization"] = "redacted"
        with self.assertRaisesRegex(MODULE.ContractError, "credential-shaped"):
            MODULE.run_fixture(manifest, fixture)

    def test_v2_protocol_objects_and_features_are_supported(self):
        manifest = self.v2_manifest()
        ids = {case["id"] for case in MODULE.build_plan(manifest)["cases"]}
        self.assertIn("responses.reasoning.high", ids)
        self.assertIn("responses.tool_call", ids)
        self.assertIn("responses.prompt_cache", ids)
        self.assertIn("responses.web_search", ids)
        self.assertIn("responses.structured_output", ids)
        self.assertFalse(any(case_id.startswith("messages.") for case_id in ids))

    def test_complete_v2_fixture_passes_and_preserves_manifest_evidence_ids(self):
        manifest = self.v2_manifest()
        receipt = MODULE.run_fixture(manifest, self.fixture(manifest))
        self.assertEqual(receipt["status"], "passed")
        self.assertEqual(receipt["manifest_evidence_ids"], ["ev-fail", "ev-pass"])

    def test_unknown_or_implicit_v2_claim_fails_closed(self):
        manifest = self.v2_manifest()
        del manifest["protocol_feature_matrix"][0]["features"]["usage"]
        with self.assertRaisesRegex(MODULE.ContractError, "explicitly cover"):
            MODULE.build_plan(manifest)

    def test_cli_returns_one_for_failed_contract_and_writes_json(self):
        manifest = self.manifest()
        fixture = self.fixture(manifest)
        fixture["observations"].pop("responses.request")
        with tempfile.TemporaryDirectory() as directory:
            manifest_path = Path(directory) / "manifest.json"
            fixture_path = Path(directory) / "fixture.json"
            output_path = Path(directory) / "receipt.json"
            manifest_path.write_text(json.dumps(manifest))
            fixture_path.write_text(json.dumps(fixture))
            proc = subprocess.run(
                [sys.executable, str(SCRIPT), "run-fixture", str(manifest_path), str(fixture_path), "--output", str(output_path)],
                check=False,
                capture_output=True,
                text=True,
            )
            self.assertEqual(proc.returncode, 1)
            self.assertEqual(json.loads(output_path.read_text())["status"], "failed")


if __name__ == "__main__":
    unittest.main()

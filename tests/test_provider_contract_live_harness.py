from __future__ import annotations

import importlib.util
import base64
import json
from pathlib import Path
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "provider_contract_live_harness.py"
SPEC = importlib.util.spec_from_file_location("provider_contract_live_harness_test", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def inventory() -> dict:
    return {
        "groups": [{
            "group_id": 6, "name": "Test", "models": [{"model": "alpha"}],
        }]
    }


def contract() -> dict:
    feature = lambda status: {"status": status, "evidence": "fixture"}
    return {
        "schema_version": 1,
        "model": {
            "id": "alpha", "display_name": "Alpha", "context_window": 128000,
            "max_output_tokens": 8192,
        },
        "access": {"base_url": "https://api.example.test/v1", "groups": [{"name": "Test", "multiplier": 1}]},
        "protocols": [{"name": "responses", "status": "verified", "evidence": "fixture"}],
        "reasoning": {"model_levels": ["low", "medium", "high"], "default_level": "medium"},
        "verification": {"modalities": {"text": "verified", "image": "verified", "video": "unsupported"}},
        "test_matrix": {
            "group_access": {"Test": {"protocols": ["responses"]}},
            "protocol_features": {"responses": {
                "reasoning": feature("blocked"), "prompt_cache": feature("blocked"),
                "image_input": feature("verified"), "web_search": feature("verified"),
            }},
            "tools": {"responses": {"structured_output": feature("blocked")}},
        },
    }


class FakeController:
    def __init__(self):
        self.switches = []
        self.restored = False

    def open(self):
        return "sk-fixture-secret-value"

    def switch(self, group_id):
        self.switches.append(group_id)

    def restore(self):
        self.restored = True

    def usage(self, case, started_at, user_agent):
        return [{
            "id": 1, "api_key_id": 128, "group_id": case.group_id,
            "model": case.model_id, "requested_model": case.model_id,
            "input_tokens": 1, "output_tokens": 1, "total_cost": 0.01,
            "actual_cost": 0.01, "created_at": started_at, "user_agent": user_agent,
        }]


class ProviderContractLiveHarnessTest(unittest.TestCase):
    def test_opus55_tool_probe_uses_auto_and_preserves_thinking(self):
        case = MODULE.LiveCase("opus55", "P-05", "tool_result_continuation", 0,
                               "direct", "claude-opus-5-5", "messages",
                               "https://api.example.test", "MARKER")
        payload = MODULE.request_payload(case)
        self.assertEqual({"type": "auto"}, payload["tool_choice"])
        content = [{"type": "thinking", "thinking": "", "signature": "fixture"},
                   {"type": "tool_use", "id": "call", "name": "echo_contract", "input": {"value": "MARKER"}}]
        tool, raw = MODULE.extract_tool_call("messages", {"content": content})
        continuation = MODULE.continuation_payload(case, {}, tool, raw)
        self.assertEqual(content, continuation["messages"][1]["content"])
        structured = MODULE.LiveCase(**{**case.__dict__, "p_id": "P-10"})
        payload = MODULE.request_payload(structured)
        self.assertEqual("json_schema", payload["output_config"]["format"]["type"])
        self.assertEqual(["marker"], payload["output_config"]["format"]["schema"]["required"])


    def fixtures(self, root: Path):
        contracts = root / "contracts"
        contracts.mkdir()
        (contracts / "alpha.json").write_text(json.dumps(contract()))
        pricing = root / "pricing.json"
        pricing.write_text(json.dumps(inventory()))
        return contracts, pricing

    def test_plan_covers_priority_and_declared_optional_cases(self):
        with tempfile.TemporaryDirectory() as directory:
            contracts, pricing = self.fixtures(Path(directory))
            cases = MODULE.build_cases(contracts, pricing)
            self.assertEqual({case.p_id for case in cases}, {
                "P-01", "P-02", "P-03", "P-04", "P-05", "P-06", "P-07",
                "P-08", "P-10", "P-11", "P-12", "P-15",
            })
            reasoning = next(case for case in cases if case.p_id == "P-06")
            self.assertEqual(reasoning.reasoning_level, "medium")
            report = MODULE.plan_report(cases, Path(directory) / "receipts")
            self.assertEqual(report["network_execution"], "disabled")
            self.assertEqual(report["paid_request_upper_bound"], len(cases) + 2)
            self.assertNotIn("sk-", json.dumps(report))

    def test_image_and_cache_fixtures_are_above_provider_minimums(self):
        png = base64.b64decode(MODULE.ONE_PIXEL_PNG)
        self.assertEqual(b"\x89PNG\r\n\x1a\n", png[:8])
        self.assertGreaterEqual(len(png), 90)
        case = MODULE.LiveCase(
            "6:alpha:responses:P-07:default", "P-07", "prompt_cache", 6, "Test",
            "alpha", "responses", "https://api.example.test/v1", "MARKER",
        )
        self.assertGreater(len(MODULE.request_payload(case, cache=True)["input"]), 16_000)

        reasoning_heavy = MODULE.LiveCase(
            "63:minimax-m3:responses:P-01:default", "P-01", "minimal_text", 63, "Test",
            "minimax-m3", "responses", "https://api.example.test/v1", "MARKER",
        )
        self.assertEqual(4096, MODULE.request_payload(reasoning_heavy)["max_output_tokens"])

    def test_queue_model_protocol_and_case_filters_intersect(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contracts, pricing = self.fixtures(root)
            queue = root / "queue.json"
            queue.write_text(json.dumps({
                "kind": "model_client_unique_test_queue",
                "queue": [
                    {"case_type": "provider_contract", "case_key": "q1", "status": "planned", "target": {"model_id": "alpha", "protocol": "responses", "test_case_id": "usage"}},
                    {"case_type": "provider_contract", "case_key": "q2", "status": "satisfied", "target": {"model_id": "alpha", "protocol": "responses", "test_case_id": "minimal_text"}},
                ],
            }))
            cases = MODULE.build_cases(
                contracts, pricing, queue_path=queue,
                models={"alpha"}, protocols={"responses"}, group_ids={6},
                p_ids={"P-01", "P-12"},
            )
            self.assertEqual([case.p_id for case in cases], ["P-12"])
            self.assertEqual(cases[0].source_case_keys, ("q1",))

    def test_blocked_protocol_candidate_requires_explicit_opt_in(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contracts, pricing = self.fixtures(root)
            value = json.loads((contracts / "alpha.json").read_text())
            value["protocol_candidates"] = [{"name": "messages", "status": "blocked", "evidence": "official candidate"}]
            (contracts / "alpha.json").write_text(json.dumps(value))
            default = MODULE.build_cases(
                contracts, pricing, protocols={"messages"}, group_ids={6}, p_ids={"P-01"},
            )
            self.assertEqual([], default)
            opted_in = MODULE.build_cases(
                contracts, pricing, protocols={"messages"}, group_ids={6}, p_ids={"P-01"},
                include_candidate_protocols=True,
            )
            self.assertEqual(["messages"], [case.protocol for case in opted_in])

    def test_p06_can_expand_selected_exact_reasoning_levels(self):
        with tempfile.TemporaryDirectory() as directory:
            contracts, pricing = self.fixtures(Path(directory))
            cases = MODULE.build_cases(
                contracts, pricing, protocols={"responses"}, group_ids={6},
                p_ids={"P-06"}, reasoning_levels={"low", "high"},
            )
            self.assertEqual(["high", "low"], sorted(case.reasoning_level for case in cases))

    def test_http_200_empty_terminal_and_503_are_blocked(self):
        case = MODULE.LiveCase(
            "6:alpha:responses:P-01:default", "P-01", "minimal_text", 6, "Test",
            "alpha", "responses", "https://api.example.test/v1", "MARKER",
        )
        empty = MODULE.response_shape("responses", MODULE.HTTPResult(200, {}, b'{"status":"completed","output":[]}', None), {"status": "completed", "output": []})
        empty["marker_present"] = False
        self.assertEqual(MODULE.blocked_classification(empty), "http_200_empty_or_incomplete_terminal")
        overloaded = MODULE.response_shape("responses", MODULE.HTTPResult(503, {}, b'{"error":{"type":"overloaded_error","message":"busy"}}', None), {"error": {"type": "overloaded_error", "message": "busy"}})
        self.assertEqual(MODULE.blocked_classification(overloaded), "upstream_overloaded")

    def test_non_error_case_cannot_pass_offline_verifier_on_http_400(self):
        case = MODULE.LiveCase(
            "6:alpha:responses:P-06:low", "P-06", "reasoning_transport", 6, "Test",
            "alpha", "responses", "https://api.example.test/v1", "MARKER",
            reasoning_level="low",
        )

        class Transport:
            def post(self, *_args):
                return MODULE.HTTPResult(400, {}, b'{"error":{"type":"invalid_request_error","message":"bad"}}', None)

        class Controller:
            def usage(self, *_args): return []

        receipt = MODULE.execute_case(case, "sk-fixture-secret-value", Transport(), Controller(), 1, "run")
        self.assertEqual("blocked", receipt["result"])
        self.assertEqual("contract_failed", receipt["classification"])
        self.assertEqual("blocked", receipt["result"])

    def test_p02_requires_real_sse_framing(self):
        body = b'data: {"type":"response.output_text.delta","delta":"x"}\n\ndata: {"type":"response.completed"}\n\n'
        shape = MODULE.response_shape(
            "responses",
            MODULE.HTTPResult(200, {"Content-Type": "text/event-stream"}, body, None),
            None, streaming=True,
        )
        self.assertTrue(shape["sse_framing"])
        self.assertTrue(shape["complete"])
        self.assertEqual(shape["sse_parse_errors"], 0)

    def test_tool_call_and_continuation_are_parsed_for_responses(self):
        case = MODULE.LiveCase(
            "6:alpha:responses:P-05:default", "P-05", "tool_result_continuation",
            6, "Test", "alpha", "responses", "https://api.example.test/v1", "MARKER",
        )
        first = json.dumps({
            "id": "resp-1", "status": "completed",
            "output": [{"type": "function_call", "name": "echo_contract", "arguments": '{"value":"MARKER"}', "call_id": "call-1"}],
            "usage": {"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
        }).encode()
        second = json.dumps({
            "id": "resp-2", "status": "completed", "output_text": MODULE.tool_result_marker(case),
            "output": [{"type": "message", "content": [{"type": "output_text", "text": MODULE.tool_result_marker(case)}]}],
            "usage": {"input_tokens": 2, "output_tokens": 1, "total_tokens": 3},
        }).encode()

        class Transport:
            def __init__(self): self.results = [first, second]
            def post(self, *_args): return MODULE.HTTPResult(200, {}, self.results.pop(0), None)

        shape, details, usage = MODULE.run_http_case(case, "sk-fixture-secret-value", Transport(), 1, "ua")
        self.assertTrue(details["correlated"])
        self.assertTrue(shape["marker_present"])
        self.assertEqual(details["attempts"], 2)
        self.assertEqual(usage["input_tokens"], 2)

        payload = MODULE.continuation_payload(
            case,
            json.loads(first),
            {"name": "echo_contract", "arguments": '{"value":"MARKER"}', "call_id": "call-1"},
            json.loads(first)["output"][0],
        )
        self.assertNotIn("previous_response_id", payload)
        self.assertEqual("function_call", payload["input"][0]["type"])
        self.assertEqual("function_call_output", payload["input"][1]["type"])

    def test_invalid_request_payload_uses_a_wrong_type_not_a_clampable_limit(self):
        for protocol, field in (
            ("responses", "input"),
            ("chat_completions", "messages"),
            ("messages", "messages"),
            ("generate_content", "contents"),
        ):
            case = MODULE.LiveCase(
                f"6:alpha:{protocol}:P-15:default", "P-15", "error_passthrough",
                6, "Test", "alpha", protocol, "https://api.example.test/v1", "MARKER",
            )
            payload = MODULE.request_payload(case, invalid=True)
            self.assertNotIsInstance(payload[field], (str, list))

    def test_web_search_uses_each_native_protocol_tool_shape(self):
        messages = MODULE.LiveCase(
            "5:alpha:messages:P-11:default", "P-11", "web_search", 5, "Test",
            "alpha", "messages", "https://api.example.test", "MARKER",
        )
        gemini = MODULE.LiveCase(
            "57:alpha:generate_content:P-11:default", "P-11", "web_search", 57, "Test",
            "alpha", "generate_content", "https://api.example.test", "MARKER",
        )
        messages_payload = MODULE.request_payload(messages)
        self.assertEqual("web_search_20250305", messages_payload["tools"][0]["type"])
        self.assertEqual({"type": "any"}, messages_payload["tool_choice"])
        self.assertEqual({}, MODULE.request_payload(gemini)["tools"][0]["googleSearch"])

    def test_web_search_requires_protocol_native_tool_or_grounding_evidence(self):
        self.assertTrue(MODULE.server_web_search_observed("responses", {
            "output": [{"type": "web_search_call", "status": "completed"}],
        }))
        self.assertTrue(MODULE.server_web_search_observed("messages", {
            "content": [{"type": "server_tool_use", "name": "web_search"}],
        }))
        self.assertTrue(MODULE.server_web_search_observed("generate_content", {
            "candidates": [{"groundingMetadata": {"groundingChunks": [{"web": {"uri": "https://example.test"}}]}}],
        }))
        self.assertFalse(MODULE.server_web_search_observed("chat_completions", {
            "choices": [{"message": {"content": "https://example.test"}}],
        }))

    def test_p04_extracts_nonempty_tool_name_and_arguments(self):
        decoded = {"output": [{"type": "function_call", "name": "echo_contract", "arguments": '{"value":"x"}', "call_id": "c1"}]}
        tool, _ = MODULE.extract_tool_call("responses", decoded)
        self.assertEqual(tool, {"name": "echo_contract", "arguments": '{"value":"x"}', "call_id": "c1"})

    def test_each_case_is_atomic_resumable_and_restore_runs_on_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contracts, pricing = self.fixtures(root)
            case = MODULE.build_cases(contracts, pricing, p_ids={"P-01"})[0]
            controller = FakeController()

            def fake_execute(case, key, transport, controller_arg, timeout, run_id):
                self.assertEqual(key, "sk-fixture-secret-value")
                return MODULE.with_artifact_sha({
                    "schema_version": 1, "kind": "provider_contract_live_case",
                    "case_fingerprint": MODULE.digest_json(MODULE.asdict(case)),
                    "case": MODULE.asdict(case), "result": "pass", "classification": "verified",
                    "secret_free": True,
                })

            original = MODULE.execute_case
            MODULE.execute_case = fake_execute
            try:
                summary = MODULE.run_cases(
                    [case], root / "out", timeout=1,
                    controller_factory=lambda: controller,
                    transport_factory=lambda: object(),
                )
                self.assertEqual(summary["status_counts"], {"pass": 1})
                self.assertTrue(controller.restored)
                self.assertTrue(MODULE.receipt_path(root / "out", case).is_file())
                # Legacy receipts lack provenance and must not be resumed.
                controller2 = FakeController()
                second = MODULE.run_cases(
                    [case], root / "out", timeout=1,
                    controller_factory=lambda: controller2,
                    transport_factory=lambda: object(),
                )
                self.assertEqual(second["status_counts"], {"pass": 1})
                self.assertTrue(controller2.restored)
            finally:
                MODULE.execute_case = original

    def test_run_requires_two_explicit_live_gates(self):
        parser = MODULE.parser()
        args = parser.parse_args(["run", "--output-dir", "/tmp/x"])
        self.assertFalse(args.execute)
        self.assertFalse(args.acknowledge_paid_probes)

    def test_restore_group_six_runs_when_case_execution_raises(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contracts, pricing = self.fixtures(root)
            case = MODULE.build_cases(contracts, pricing, p_ids={"P-01"})[0]
            controller = FakeController()
            original = MODULE.execute_case
            MODULE.execute_case = lambda *_args, **_kwargs: (_ for _ in ()).throw(RuntimeError("boom"))
            try:
                with self.assertRaisesRegex(RuntimeError, "boom"):
                    MODULE.run_cases(
                        [case], root / "out", timeout=1,
                        controller_factory=lambda: controller,
                        transport_factory=lambda: object(),
                    )
                self.assertTrue(controller.restored)
                state = json.loads((root / "out" / "run-state.json").read_text())
                self.assertEqual(state["restored_group_id"], 6)
            finally:
                MODULE.execute_case = original

    def test_usage_readback_retries_transient_admin_failure_without_aborting(self):
        case = MODULE.LiveCase(
            "6:alpha:responses:P-01:default", "P-01", "minimal_text", 6, "Test",
            "alpha", "responses", "https://api.example.test/v1", "MARKER",
        )
        started = "2026-09-01T00:00:00+00:00"
        user_agent = "live-harness-test"

        class AdminModule:
            def __init__(self): self.calls = 0
            def admin_cli_json(self, _args):
                self.calls += 1
                if self.calls == 1:
                    raise RuntimeError("temporary admin fetch failure")
                return {"items": [{
                    "id": 1, "api_key_id": 128, "group_id": 6,
                    "model": "alpha", "created_at": started,
                    "user_agent": user_agent,
                }]}

        controller = MODULE.ProductionController.__new__(MODULE.ProductionController)
        controller.module = AdminModule()
        original_sleep = MODULE.time.sleep
        MODULE.time.sleep = lambda _seconds: None
        try:
            rows = controller.usage(case, started, user_agent)
        finally:
            MODULE.time.sleep = original_sleep
        self.assertEqual([1], [row["id"] for row in rows])
        self.assertEqual(2, controller.module.calls)

    def test_group_switch_retries_transient_admin_failure_and_verifies_readback(self):
        class GroupModule:
            def __init__(self): self.calls = 0
            def switch_group(self, group_id):
                self.calls += 1
                if self.calls == 1:
                    raise RuntimeError("temporary admin failure")
                return {"group_id": group_id}

        controller = MODULE.ProductionController.__new__(MODULE.ProductionController)
        controller.module = GroupModule()
        original_sleep = MODULE.time.sleep
        MODULE.time.sleep = lambda _seconds: None
        try:
            controller.switch(59)
        finally:
            MODULE.time.sleep = original_sleep
        self.assertEqual(2, controller.module.calls)

    def test_owned_key_open_retries_transient_admin_failure(self):
        class OwnedModule:
            def __init__(self): self.calls = 0
            def owned_key(self):
                self.calls += 1
                if self.calls == 1:
                    raise RuntimeError("temporary admin failure")
                return {"id": 128, "user_id": 2, "group_id": 6, "key": "sk-fixture-secret-value"}

        controller = MODULE.ProductionController.__new__(MODULE.ProductionController)
        controller.module = OwnedModule()
        original_sleep = MODULE.time.sleep
        MODULE.time.sleep = lambda _seconds: None
        try:
            self.assertEqual("sk-fixture-secret-value", controller.open())
        finally:
            MODULE.time.sleep = original_sleep
        self.assertEqual(2, controller.module.calls)


if __name__ == "__main__":
    unittest.main()

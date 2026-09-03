#!/usr/bin/env python3
"""Plan and verify provider contracts from a secret-free model manifest.

The runner deliberately has no HTTP client and never resolves environment
variables.  ``plan`` describes the controlled live probes an external harness
must perform.  ``run-fixture`` verifies captured, non-sensitive observations so
ordinary CI remains deterministic and cannot accidentally call a provider.
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from datetime import datetime
import hashlib
import json
import math
from pathlib import Path
import re
import sys
from typing import Any, Callable


PROTOCOLS = {"chat_completions", "responses", "messages", "generate_content"}
V2_FEATURES = {
    "basic_request",
    "streaming",
    "terminal_event",
    "tool_calls",
    "tool_result_round_trip",
    "reasoning",
    "prompt_cache",
    "image_input",
    "context_window",
    "error_passthrough",
    "disconnect",
    "timeout",
    "retry",
    "usage",
    "billing",
    "web_search",
    "structured_output",
}
V2_RESULTS = {"pass", "fail", "untested", "not_applicable"}
PROTOCOL_ALIASES = {
    "chat": "chat_completions",
    "chatcompletion": "chat_completions",
    "chat_completions": "chat_completions",
    "openai_chat_completions": "chat_completions",
    "responses": "responses",
    "openai_responses": "responses",
    "message": "messages",
    "messages": "messages",
    "anthropic_messages": "messages",
    "generatecontent": "generate_content",
    "generate_content": "generate_content",
    "gemini_generate_content": "generate_content",
}
SECRET_KEY = re.compile(
    r"(?:^|[_-])(api[_-]?key|token|password|credential|secret|authorization|cookie)(?:$|[_-])",
    re.I,
)
SECRET_VALUE = re.compile(
    r"(?:Bearer\s+[A-Za-z0-9._~+/=-]{8,}|sk-[A-Za-z0-9_-]{8,}|AIza[A-Za-z0-9_-]{12,})",
    re.I,
)
MODEL_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]*$")


class ContractError(ValueError):
    """A manifest or fixture violates the provider-contract boundary."""


@dataclass(frozen=True)
class ContractCase:
    case_id: str
    protocol: str | None
    capability: str
    expectation: dict[str, Any]

    def as_dict(self) -> dict[str, Any]:
        return {
            "id": self.case_id,
            "protocol": self.protocol,
            "capability": self.capability,
            "expectation": self.expectation,
        }


def _fail(message: str) -> None:
    raise ContractError(message)


def _walk_secret_free(value: Any, path: str = "$") -> None:
    if isinstance(value, dict):
        for key, item in value.items():
            if SECRET_KEY.search(str(key)):
                _fail(f"forbidden credential-shaped field: {path}.{key}")
            _walk_secret_free(item, f"{path}.{key}")
    elif isinstance(value, list):
        for index, item in enumerate(value):
            _walk_secret_free(item, f"{path}[{index}]")
    elif isinstance(value, str) and SECRET_VALUE.search(value):
        _fail(f"secret-shaped value at {path}")


def _object(value: Any, path: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        _fail(f"{path} must be an object")
    return value


def _positive_int(value: Any, path: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
        _fail(f"{path} must be a positive integer")
    return value


def _claim(value: Any, path: str, *, default: bool | None = None) -> bool:
    """Read a capability claim without allowing truthy/unknown values."""
    if value is None and default is not None:
        return default
    if isinstance(value, bool):
        return value
    if isinstance(value, dict):
        state = value.get("status", value.get("supported"))
        if state in (True, "supported", "verified", "claimed"):
            return True
        if state in (False, "unsupported"):
            return False
    _fail(f"{path} must explicitly claim supported or unsupported")


def _normal_protocol(value: Any, path: str) -> str:
    if not isinstance(value, str):
        _fail(f"{path} must be a protocol name")
    key = re.sub(r"[\s-]+", "_", value.strip().lower())
    protocol = PROTOCOL_ALIASES.get(key)
    if protocol not in PROTOCOLS:
        _fail(f"{path} has unsupported protocol: {value}")
    return protocol


def _normal_protocols(capabilities: dict[str, Any]) -> list[str]:
    raw = capabilities.get("protocols", capabilities.get("protocol"))
    if isinstance(raw, str):
        return [_normal_protocol(raw, "capabilities.protocol")]
    if not isinstance(raw, list) or not raw:
        _fail("capabilities.protocol or capabilities.protocols is required")
    protocols: list[str] = []
    for index, item in enumerate(raw):
        if isinstance(item, dict):
            status = item.get("status", item.get("supported"))
            if status in (False, "unsupported"):
                continue
            if status not in (True, "supported", "verified", "claimed"):
                _fail(f"capabilities.protocols[{index}].status must be explicit")
            item = item.get("name", item.get("protocol"))
        protocol = _normal_protocol(item, f"capabilities.protocols[{index}]")
        if protocol not in protocols:
            protocols.append(protocol)
    if not protocols:
        _fail("at least one protocol must be claimed as supported")
    return protocols


def _evidence_index(manifest: dict[str, Any]) -> dict[str, dict[str, Any]]:
    raw = manifest.get("evidence")
    if not isinstance(raw, list):
        _fail("schema v2 evidence must be an array")
    result: dict[str, dict[str, Any]] = {}
    for index, item in enumerate(raw):
        item = _object(item, f"evidence[{index}]")
        evidence_id = item.get("id")
        if not isinstance(evidence_id, str) or not evidence_id.strip() or evidence_id in result:
            _fail(f"evidence[{index}].id must be unique and non-empty")
        if item.get("kind") not in {
            "official_docs", "provider_api", "live_probe", "billing_reconciliation", "release_artifact"
        }:
            _fail(f"evidence[{index}].kind is invalid")
        url = item.get("url")
        digest = item.get("artifact_sha256")
        if url is not None and (not isinstance(url, str) or not url.startswith("https://")):
            _fail(f"evidence[{index}].url must be null or HTTPS")
        if digest is not None and (not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest)):
            _fail(f"evidence[{index}].artifact_sha256 must be null or lowercase SHA-256")
        if url is None and digest is None:
            _fail(f"evidence[{index}] requires url or artifact_sha256")
        if item.get("result") not in {"pass", "fail", "pending"}:
            _fail(f"evidence[{index}].result is invalid")
        if not isinstance(item.get("observed_at"), str) or not isinstance(item.get("subject_version"), str) or not item["subject_version"]:
            _fail(f"evidence[{index}] requires observed_at and subject_version")
        try:
            datetime.fromisoformat(item["observed_at"].replace("Z", "+00:00"))
        except ValueError:
            _fail(f"evidence[{index}].observed_at must be ISO-8601")
        result[evidence_id] = item
    return result


def _matrix_evidence_ids(
    raw: Any,
    path: str,
    evidence: dict[str, dict[str, Any]],
    *,
    required_results: set[str] | None = None,
) -> list[str]:
    if not isinstance(raw, list) or not all(isinstance(item, str) and item for item in raw):
        _fail(f"{path} must be a string array")
    unknown = sorted(set(raw) - evidence.keys())
    if unknown:
        _fail(f"{path} references unknown evidence: {', '.join(unknown)}")
    for result in required_results or set():
        if not raw or not any(evidence[item]["result"] == result for item in raw):
            _fail(f"{path} requires {result} evidence")
    return list(dict.fromkeys(raw))


def _v2_matrix_view(manifest: dict[str, Any]) -> dict[str, Any]:
    evidence = _evidence_index(manifest)
    lifecycle = _object(manifest.get("lifecycle"), "lifecycle")
    if lifecycle.get("public_status") not in {"draft", "public", "hidden", "deprecated"}:
        _fail("lifecycle.public_status is invalid")
    for field in ("introduced_at", "deprecated_at"):
        value = lifecycle.get(field)
        if value is not None and (not isinstance(value, str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", value)):
            _fail(f"lifecycle.{field} must be null or YYYY-MM-DD")
    replacement = lifecycle.get("replacement_model_id")
    if replacement is not None and (not isinstance(replacement, str) or not MODEL_ID.fullmatch(replacement)):
        _fail("lifecycle.replacement_model_id must be null or a safe model ID")
    if lifecycle.get("public_status") == "deprecated" and lifecycle.get("deprecated_at") is None:
        _fail("deprecated lifecycle requires lifecycle.deprecated_at")
    raw_protocols = manifest.get("protocol_matrix")
    if not isinstance(raw_protocols, list) or not raw_protocols:
        _fail("schema v2 protocol_matrix must be a non-empty array")
    protocols: list[str] = []
    protocol_evidence: dict[str, list[str]] = {}
    seen: set[str] = set()
    for index, item in enumerate(raw_protocols):
        item = _object(item, f"protocol_matrix[{index}]")
        protocol = _normal_protocol(item.get("protocol"), f"protocol_matrix[{index}].protocol")
        if protocol in seen:
            _fail(f"protocol_matrix contains duplicate protocol: {protocol}")
        seen.add(protocol)
        support = item.get("support")
        if support not in {"supported", "unsupported", "untested"}:
            _fail(f"protocol_matrix[{index}].support is invalid")
        if item.get("recommendation") not in {"preferred", "allowed", "discouraged", "not_applicable"}:
            _fail(f"protocol_matrix[{index}].recommendation is invalid")
        if not isinstance(item.get("recommendation_reason"), str) or not item["recommendation_reason"].strip():
            _fail(f"protocol_matrix[{index}].recommendation_reason is required")
        ids = _matrix_evidence_ids(
            item.get("evidence_ids"), f"protocol_matrix[{index}].evidence_ids", evidence,
            required_results={"pass"} if support == "supported" else set(),
        )
        if support == "supported":
            protocols.append(protocol)
            protocol_evidence[protocol] = ids
    if not protocols:
        _fail("schema v2 must contain at least one supported protocol")

    raw_features = manifest.get("protocol_feature_matrix")
    if not isinstance(raw_features, list):
        _fail("schema v2 protocol_feature_matrix must be an array")
    feature_status: dict[str, dict[str, str]] = {}
    feature_evidence: dict[str, list[str]] = {}
    for index, item in enumerate(raw_features):
        item = _object(item, f"protocol_feature_matrix[{index}]")
        protocol = _normal_protocol(item.get("protocol"), f"protocol_feature_matrix[{index}].protocol")
        if protocol in feature_status:
            _fail(f"protocol_feature_matrix contains duplicate protocol: {protocol}")
        features = _object(item.get("features"), f"protocol_feature_matrix[{index}].features")
        if set(features) != V2_FEATURES or any(value not in V2_RESULTS for value in features.values()):
            _fail(f"protocol_feature_matrix[{index}].features must explicitly cover every known feature")
        ids = _matrix_evidence_ids(
            item.get("evidence_ids"), f"protocol_feature_matrix[{index}].evidence_ids", evidence,
            required_results=(
                ({"pass"} if any(value == "pass" for value in features.values()) else set())
            ),
        )
        feature_status[protocol] = dict(features)
        feature_evidence[protocol] = ids
    missing = sorted(set(protocols) - feature_status.keys())
    if missing:
        _fail("protocol_feature_matrix is missing supported protocols: " + ", ".join(missing))
    unknown_feature_protocols = sorted(set(feature_status) - seen)
    if unknown_feature_protocols:
        _fail("protocol_feature_matrix references protocols absent from protocol_matrix: " + ", ".join(unknown_feature_protocols))
    for protocol in protocols:
        if feature_status[protocol]["basic_request"] != "pass":
            _fail(f"supported protocol {protocol} requires basic_request=pass")
        if feature_status[protocol]["streaming"] == "pass" and feature_status[protocol]["terminal_event"] != "pass":
            _fail(f"supported streaming protocol {protocol} requires terminal_event=pass")

    raw_reasoning = manifest.get("reasoning_matrix")
    if not isinstance(raw_reasoning, list):
        _fail("schema v2 reasoning_matrix must be an array")
    efforts: dict[str, list[str]] = {protocol: [] for protocol in protocols}
    reasoning_evidence: dict[tuple[str, str], list[str]] = {}
    for index, item in enumerate(raw_reasoning):
        item = _object(item, f"reasoning_matrix[{index}]")
        protocol = _normal_protocol(item.get("protocol"), f"reasoning_matrix[{index}].protocol")
        if protocol not in seen:
            _fail(f"reasoning_matrix[{index}] references a protocol absent from protocol_matrix")
        effort = item.get("effort")
        if not isinstance(effort, str) or not effort.strip():
            _fail(f"reasoning_matrix[{index}].effort is required")
        support = item.get("support")
        if support not in {"supported", "unsupported", "untested"}:
            _fail(f"reasoning_matrix[{index}].support is invalid")
        wire_value = item.get("wire_value")
        if wire_value is not None and (not isinstance(wire_value, str) or not wire_value):
            _fail(f"reasoning_matrix[{index}].wire_value must be null or non-empty")
        if support == "supported" and wire_value is None:
            _fail(f"reasoning_matrix[{index}].wire_value is required for a supported effort")
        ids = _matrix_evidence_ids(
            item.get("evidence_ids"), f"reasoning_matrix[{index}].evidence_ids", evidence,
            required_results={"pass"} if support == "supported" else set(),
        )
        key = (protocol, effort)
        if key in reasoning_evidence:
            _fail(f"reasoning_matrix contains duplicate entry: {protocol}/{effort}")
        reasoning_evidence[key] = ids
        if protocol in efforts and support == "supported":
            efforts[protocol].append(effort)
    for protocol in protocols:
        if feature_status[protocol]["reasoning"] == "pass" and not efforts[protocol]:
            _fail(f"reasoning feature pass requires supported efforts for {protocol}")

    raw_prices = manifest.get("price_matrix")
    if not isinstance(raw_prices, list):
        _fail("schema v2 price_matrix must be an array")
    default_price: dict[str, Any] | None = None
    for index, item in enumerate(raw_prices):
        item = _object(item, f"price_matrix[{index}]")
        ids = _matrix_evidence_ids(
            item.get("evidence_ids"), f"price_matrix[{index}].evidence_ids", evidence, required_results={"pass"}
        )
        if item.get("currency") != "USD" or item.get("unit") != "per_mtok":
            _fail(f"price_matrix[{index}] must use USD per_mtok")
        if not isinstance(item.get("scope"), str) or not item["scope"]:
            _fail(f"price_matrix[{index}].scope is required")
        effective = item.get("effective_from")
        if effective is not None and (not isinstance(effective, str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", effective)):
            _fail(f"price_matrix[{index}].effective_from must be null or YYYY-MM-DD")
        if item.get("scope") == "default":
            if default_price is not None:
                _fail("price_matrix contains duplicate default scope")
            default_price = {
                "input_per_mtok_usd": item.get("input"),
                "output_per_mtok_usd": item.get("output"),
                "cached_input_per_mtok_usd": item.get("cached_input"),
                "long_context": item.get("long_context"),
                "evidence_ids": ids,
            }
    if default_price is None:
        _fail("price_matrix requires a default scope")
    for name in ("input_per_mtok_usd", "output_per_mtok_usd"):
        value = default_price[name]
        if isinstance(value, bool) or not isinstance(value, (int, float)) or value < 0:
            _fail(f"default price {name} must be non-negative")
    cached_price = default_price["cached_input_per_mtok_usd"]
    if cached_price is not None and (
        isinstance(cached_price, bool) or not isinstance(cached_price, (int, float)) or cached_price < 0
    ):
        _fail("default cached input price must be null or non-negative")
    return {
        "protocols": protocols,
        "protocol_features": feature_status,
        "protocol_efforts": efforts,
        "protocol_evidence": protocol_evidence,
        "feature_evidence": feature_evidence,
        "reasoning_evidence": reasoning_evidence,
        "pricing": default_price,
        "evidence": evidence,
    }


def _feature(capabilities: dict[str, Any], name: str, *aliases: str, default: bool | None = None) -> bool:
    features = capabilities.get("features", {})
    if features is not None and not isinstance(features, dict):
        _fail("capabilities.features must be an object")
    for key in (name, *aliases):
        if key in capabilities:
            return _claim(capabilities[key], f"capabilities.{key}", default=default)
        if key in features:
            return _claim(features[key], f"capabilities.features.{key}", default=default)
    return _claim(None, f"capabilities.features.{name}", default=default)


def _reasoning_efforts(manifest: dict[str, Any], capabilities: dict[str, Any]) -> list[str]:
    raw: Any = capabilities.get("reasoning_efforts")
    reasoning = capabilities.get("reasoning", manifest.get("reasoning"))
    if raw is None and isinstance(reasoning, dict):
        raw = reasoning.get("efforts", reasoning.get("model_levels"))
    if raw is None:
        return []
    if not isinstance(raw, list) or not all(isinstance(item, str) and item.strip() for item in raw):
        _fail("reasoning efforts must be a string array")
    return list(dict.fromkeys(item.strip() for item in raw))


def _manifest_view(manifest: dict[str, Any]) -> dict[str, Any]:
    _walk_secret_free(manifest)
    version = manifest.get("schema_version")
    if version not in {1, 2}:
        _fail("schema_version must be 1 or 2")
    model = _object(manifest.get("model"), "model")
    model_id = model.get("id")
    if not isinstance(model_id, str) or not MODEL_ID.fullmatch(model_id):
        _fail("model.id must be an installer-safe identifier")
    context_window = _positive_int(model.get("context_window"), "model.context_window")
    max_output = _positive_int(model.get("max_output_tokens"), "model.max_output_tokens")
    capabilities = _object(manifest.get("capabilities"), "capabilities")
    matrix = _v2_matrix_view(manifest) if version == 2 else None
    protocols = matrix["protocols"] if matrix else _normal_protocols(capabilities)
    efforts = _reasoning_efforts(manifest, capabilities)
    features = {
        "streaming": _feature(capabilities, "streaming"),
        "tools": _feature(capabilities, "tools", "tool_calls"),
        "prompt_cache": _feature(capabilities, "prompt_cache", "cache_read"),
        "image_input": _feature(capabilities, "image_input", "images"),
        "reasoning": bool(efforts),
    }
    pricing = matrix["pricing"] if matrix else _object(manifest.get("pricing"), "pricing")
    required_prices = ("input_per_mtok_usd", "output_per_mtok_usd")
    for field in required_prices:
        value = pricing.get(field)
        if isinstance(value, bool) or not isinstance(value, (int, float)) or value < 0:
            _fail(f"pricing.{field} must be a non-negative number")
    cache_price = pricing.get("cached_input_per_mtok_usd")
    if cache_price is not None and (
        isinstance(cache_price, bool) or not isinstance(cache_price, (int, float)) or cache_price < 0
    ):
        _fail("pricing.cached_input_per_mtok_usd must be null or non-negative")
    long_context = pricing.get("long_context")
    if long_context is not None:
        long_context = _object(long_context, "pricing.long_context")
        for field in ("input_threshold", "input_multiplier", "output_multiplier"):
            value = long_context.get(field)
            if isinstance(value, bool) or not isinstance(value, (int, float)) or value <= 0:
                _fail(f"pricing.long_context.{field} must be positive")
    expected = _object(manifest.get("verification"), "verification").get("expected_text")
    if not isinstance(expected, str) or not expected:
        _fail("verification.expected_text is required")
    return {
        "schema_version": version,
        "model_id": model_id,
        "protocols": protocols,
        "features": features,
        "reasoning_efforts": efforts,
        "context_window": context_window,
        "max_output_tokens": max_output,
        "expected_text": expected,
        "pricing": pricing,
        "protocol_features": matrix["protocol_features"] if matrix else None,
        "protocol_efforts": matrix["protocol_efforts"] if matrix else None,
        "matrix": matrix,
    }


def _case(protocol: str | None, capability: str, expectation: dict[str, Any]) -> ContractCase:
    case_id = f"{protocol}.{capability}" if protocol else f"model.{capability}"
    return ContractCase(case_id, protocol, capability, expectation)


def build_plan(manifest: dict[str, Any]) -> dict[str, Any]:
    """Return the complete controlled probe plan selected by capability claims."""
    view = _manifest_view(manifest)
    cases: list[ContractCase] = []
    for protocol in view["protocols"]:
        matrix_features = view["protocol_features"].get(protocol) if view["protocol_features"] else None
        enabled = lambda v2, v1: matrix_features[v2] == "pass" if matrix_features else view["features"].get(v1, True)
        case_evidence = (
            view["matrix"]["feature_evidence"].get(protocol, [])
            if view["matrix"] else []
        )

        def expectation(value: dict[str, Any], evidence_ids: list[str] | None = None) -> dict[str, Any]:
            value = dict(value)
            if view["schema_version"] == 2:
                value["manifest_evidence_ids"] = evidence_ids if evidence_ids is not None else case_evidence
            return value

        cases.append(_case(protocol, "request", expectation({"http_status": 200, "expected_text": view["expected_text"]})))
        if enabled("usage", "baseline"):
            cases.append(_case(protocol, "usage", expectation({"token_counts": "non_negative_and_total_consistent"})))
        if enabled("billing", "baseline"):
            cases.append(_case(protocol, "billing_receipt", expectation({"prices": "manifest_pricing", "tolerance_usd": 1e-9})))
        if enabled("error_passthrough", "baseline"):
            cases.append(_case(protocol, "error_passthrough", expectation({"http_status": "4xx_or_5xx", "provider_error": True})))
        if enabled("disconnect", "baseline"):
            cases.append(_case(protocol, "interrupted_stream", expectation({"complete": False, "classified": True})))
        if enabled("timeout", "baseline"):
            cases.append(_case(protocol, "timeout", expectation({"timed_out": True, "classified": True})))
        if enabled("retry", "baseline"):
            cases.append(_case(protocol, "retry", expectation({"attempts": ">=2", "final_status": 200})))
        if enabled("streaming", "streaming"):
            cases.append(_case(protocol, "streaming_terminal", expectation({"terminal": _terminal_for(protocol)})))
        if enabled("tool_calls", "tools"):
            cases.append(_case(protocol, "tool_call", expectation({"name_and_arguments": True})))
        if enabled("tool_result_round_trip", "tools"):
            cases.append(_case(protocol, "tool_result_continuation", expectation({"correlated": True, "final_text": True})))
        if enabled("prompt_cache", "prompt_cache"):
            cases.append(_case(protocol, "prompt_cache", expectation({"cached_input_tokens": ">0"})))
        if enabled("image_input", "image_input"):
            cases.append(_case(protocol, "image_input", expectation({"accepted": True, "final_text": True})))
        protocol_efforts = view["protocol_efforts"].get(protocol, []) if view["protocol_efforts"] else view["reasoning_efforts"]
        for effort in protocol_efforts:
            evidence_ids = view["matrix"]["reasoning_evidence"].get((protocol, effort), []) if view["matrix"] else None
            cases.append(_case(protocol, f"reasoning.{effort}", expectation({"effort": effort, "reasoning_observed": True}, evidence_ids)))
        if matrix_features and matrix_features["web_search"] == "pass":
            cases.append(_case(protocol, "web_search", expectation({"citations": True})))
        if matrix_features and matrix_features["structured_output"] == "pass":
            cases.append(_case(protocol, "structured_output", expectation({"schema_valid": True})))
    limits_enabled = not view["protocol_features"] or any(
        features["context_window"] == "pass" for protocol, features in view["protocol_features"].items()
        if protocol in view["protocols"]
    )
    if limits_enabled:
        cases.extend(
            [
                _case(None, "context_window", {"declared": view["context_window"], "source": "official_or_live"}),
                _case(None, "max_output_tokens", {"declared": view["max_output_tokens"], "source": "official_or_live"}),
            ]
        )
    return {
        "receipt_schema_version": 1,
        "mode": "controlled_probe_plan",
        "network_execution": "disabled",
        "model_id": view["model_id"],
        "manifest_schema_version": view["schema_version"],
        "manifest_sha256": _digest(manifest),
        "manifest_evidence_ids": sorted(view["matrix"]["evidence"]) if view["matrix"] else [],
        "cases": [case.as_dict() for case in cases],
        "pending_cases": (
            [{"protocol":p["protocol"], "feature":"protocol_availability", "status":"untested"}
             for p in manifest.get("protocol_matrix", []) if p.get("support") == "untested"]
            + [{"protocol":row["protocol"], "feature":feature, "status":"untested"}
               for row in manifest.get("protocol_feature_matrix", [])
               for feature, status in row["features"].items() if status == "untested"]
        ),
        "release_authorization": False,
    }


def _terminal_for(protocol: str) -> str:
    return {
        "responses": "response.completed",
        "chat_completions": "[DONE] or finish_reason",
        "messages": "message_stop",
        "generate_content": "finishReason",
    }[protocol]


def _digest(value: Any) -> str:
    payload = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(payload).hexdigest()


def _evidence(observation: dict[str, Any]) -> None:
    evidence = observation.get("evidence")
    if not isinstance(evidence, dict):
        _fail("evidence must be an object")
    if not isinstance(evidence.get("source"), str) or not evidence["source"].strip():
        _fail("evidence.source is required")
    if not isinstance(evidence.get("id"), str) or not evidence["id"].strip():
        _fail("evidence.id is required")
    observed_at = observation.get("observed_at")
    if not isinstance(observed_at, str):
        _fail("observed_at is required")
    try:
        datetime.fromisoformat(observed_at.replace("Z", "+00:00"))
    except ValueError:
        _fail("observed_at must be ISO-8601")


def _receipt_evidence(observation: dict[str, Any], model_id: str) -> dict[str, Any]:
    """Render a passed observation in the manifest-v2 evidence shape."""
    # This function is used ONLY by run-fixture, not by the live harness.
    # Preserve the non-production origin even when fixture input claims live.
    kind = "offline_fixture"
    payload = {key: value for key, value in observation.items() if key != "evidence"}
    return {
        "id": observation["evidence"]["id"],
        "kind": kind,
        "url": None,
        "observed_at": observation["observed_at"],
        "subject_version": model_id,
        "result": "pass",
        "artifact_sha256": _digest(payload),
    }


def _event_type(event: Any) -> str | None:
    if isinstance(event, str):
        return event
    if isinstance(event, dict):
        for key in ("type", "event", "finish_reason", "finishReason"):
            value = event.get(key)
            if isinstance(value, str):
                return value
    return None


def _text_present(observation: dict[str, Any], expected: str | None = None) -> bool:
    candidates = [observation.get("text"), observation.get("final_text")]
    body = observation.get("body")
    if isinstance(body, dict):
        candidates.extend([body.get("text"), body.get("output_text")])
    texts = [item for item in candidates if isinstance(item, str)]
    return bool(texts) if expected is None else any(expected in item for item in texts)


def _number(value: Any) -> float | None:
    return float(value) if not isinstance(value, bool) and isinstance(value, (int, float)) and math.isfinite(float(value)) else None


def _usage(observation: dict[str, Any]) -> tuple[float, float, float, float]:
    usage = observation.get("usage")
    if not isinstance(usage, dict):
        _fail("usage object is required")
    inp = _number(usage.get("input_tokens", usage.get("prompt_tokens")))
    out = _number(usage.get("output_tokens", usage.get("completion_tokens")))
    cached = _number(usage.get("cached_input_tokens", usage.get("cache_read_input_tokens", 0)))
    total = _number(usage.get("total_tokens"))
    if inp is None or out is None or cached is None or min(inp, out, cached) < 0:
        _fail("usage token fields must be non-negative numbers")
    expected_total = inp + out
    if total is None:
        total = expected_total
    if total != expected_total:
        _fail("usage.total_tokens must equal input_tokens + output_tokens")
    if cached > inp:
        _fail("cached_input_tokens cannot exceed input_tokens")
    return inp, out, cached, total


def _validate_case(case: ContractCase, observation: dict[str, Any], view: dict[str, Any]) -> None:
    _walk_secret_free(observation, f"fixture.observations.{case.case_id}")
    _evidence(observation)
    capability = case.capability
    status = observation.get("status_code")
    if capability == "request":
        if status != 200 or not _text_present(observation, view["expected_text"]):
            _fail("request must return HTTP 200 and the expected text")
    elif capability == "streaming_terminal":
        events = observation.get("events")
        if not isinstance(events, list) or not events:
            _fail("streaming_terminal requires events")
        kinds = [_event_type(event) for event in events]
        final_kind = kinds[-1]
        terminal = {
            "responses": final_kind == "response.completed",
            "messages": final_kind == "message_stop",
            "generate_content": final_kind in {"STOP", "MAX_TOKENS", "finishReason"},
            "chat_completions": final_kind == "[DONE]" or final_kind in {"stop", "length", "tool_calls"},
        }[case.protocol or "responses"]
        if not terminal or observation.get("complete") is not True or not _text_present(observation, view["expected_text"]):
            _fail(f"missing complete {_terminal_for(case.protocol or 'responses')} terminal")
    elif capability == "tool_call":
        tool = observation.get("tool_call")
        if not isinstance(tool, dict) or not isinstance(tool.get("name"), str) or not tool["name"]:
            _fail("tool_call.name is required")
        arguments = tool.get("arguments")
        if isinstance(arguments, str):
            try:
                arguments = json.loads(arguments)
            except json.JSONDecodeError:
                _fail("tool_call.arguments must be valid JSON")
        if not isinstance(arguments, dict) or not arguments:
            _fail("tool_call.arguments must be a non-empty object")
        expected_name = case.expectation.get("tool_name")
        if expected_name and tool["name"] != expected_name:
            _fail("tool_call.name does not match the requested tool")
        expected_arguments = case.expectation.get("arguments")
        if expected_arguments is not None and arguments != expected_arguments:
            _fail("tool_call.arguments do not match the requested arguments")
        if case.protocol != "generate_content" and not isinstance(tool.get("call_id"), str):
            _fail("tool_call.call_id is required")
        if case.protocol != "generate_content" and not tool["call_id"]:
            _fail("tool_call.call_id must not be empty")
    elif capability == "tool_result_continuation":
        if observation.get("correlated") is not True or not _text_present(observation):
            _fail("tool result must be correlated and continue to final text")
    elif capability.startswith("reasoning."):
        effort = capability.split(".", 1)[1]
        if observation.get("effort") != effort:
            _fail(f"reasoning observation must use effort {effort}")
        reasoning = observation.get("reasoning")
        if not isinstance(reasoning, (str, dict, list)) or not reasoning:
            _fail("reasoning evidence is required")
    elif capability == "prompt_cache":
        _, _, cached, _ = _usage(observation)
        if cached <= 0:
            _fail("prompt cache claim requires cached_input_tokens > 0")
    elif capability == "image_input":
        if observation.get("accepted") is not True or not _text_present(observation):
            _fail("image input must be accepted and produce final text")
    elif capability in {"context_window", "max_output_tokens"}:
        expected = view[capability]
        if observation.get("declared") != expected or observation.get("source_kind") not in {"official", "live"}:
            _fail(f"{capability} must match manifest and have official/live evidence")
    elif capability == "error_passthrough":
        error = observation.get("error")
        if not isinstance(status, int) or not 400 <= status <= 599:
            _fail("error passthrough requires a 4xx/5xx status")
        if not isinstance(error, dict) or not all(isinstance(error.get(key), str) and error[key] for key in ("code", "message")):
            _fail("error passthrough requires provider code and message")
    elif capability == "interrupted_stream":
        if observation.get("complete") is not False or not isinstance(observation.get("classification"), str):
            _fail("interrupted stream must remain incomplete and classified")
    elif capability == "timeout":
        if observation.get("timed_out") is not True or not isinstance(observation.get("classification"), str):
            _fail("timeout must be observed and classified")
    elif capability == "retry":
        attempts = observation.get("attempts")
        if isinstance(attempts, bool) or not isinstance(attempts, int) or attempts < 2 or observation.get("final_status_code") != 200:
            _fail("retry must record at least two attempts and a final HTTP 200")
    elif capability == "usage":
        _usage(observation)
    elif capability == "billing_receipt":
        inp, out, cached, _ = _usage(observation)
        prices = view["pricing"]
        cache_price = prices.get("cached_input_per_mtok_usd")
        if cache_price is None:
            cache_price = prices["input_per_mtok_usd"]
        uncached = inp - cached
        input_multiplier = 1.0
        output_multiplier = 1.0
        long_context = prices.get("long_context")
        if isinstance(long_context, dict) and inp > long_context.get("input_threshold", float("inf")):
            input_multiplier = float(long_context.get("input_multiplier", 1))
            output_multiplier = float(long_context.get("output_multiplier", 1))
        expected = (
            uncached * prices["input_per_mtok_usd"] * input_multiplier
            + cached * cache_price * input_multiplier
            + out * prices["output_per_mtok_usd"] * output_multiplier
        ) / 1_000_000
        billed = _number(observation.get("billed_usd"))
        if not math.isfinite(expected) or billed is None or billed < 0 or abs(billed - expected) > 1e-9:
            _fail(f"billing receipt mismatch: expected {expected:.12g}")
        for key in ("request_id", "usage_id", "accounting_command_id"):
            if not isinstance(observation.get(key), str) or not observation[key]:
                _fail(f"billing receipt requires {key}")
    elif capability == "web_search":
        citations = observation.get("citations")
        if not isinstance(citations, list) or not citations or not all(isinstance(item, str) and item for item in citations):
            _fail("web search requires at least one citation")
        if not _text_present(observation):
            _fail("web search requires final text")
    elif capability == "structured_output":
        if observation.get("schema_valid") is not True or not isinstance(observation.get("output"), dict):
            _fail("structured output must contain schema-valid object output")
    else:  # pragma: no cover - construction and validation are deliberately closed
        _fail(f"unknown contract capability: {capability}")


def run_fixture(manifest: dict[str, Any], fixture: dict[str, Any]) -> dict[str, Any]:
    """Verify a captured fixture and return a machine-readable fail-closed receipt."""
    view = _manifest_view(manifest)
    _walk_secret_free(fixture, "fixture")
    if fixture.get("schema_version") != 1:
        _fail("fixture.schema_version must be 1")
    if fixture.get("model_id") != view["model_id"]:
        _fail("fixture.model_id must match manifest model.id")
    observations = _object(fixture.get("observations"), "fixture.observations")
    plan = build_plan(manifest)
    results: list[dict[str, Any]] = []
    for item in plan["cases"]:
        case = ContractCase(item["id"], item["protocol"], item["capability"], item["expectation"])
        observation = observations.get(case.case_id)
        if not isinstance(observation, dict):
            results.append({"id": case.case_id, "status": "failed", "reason": "missing observation"})
            continue
        try:
            _validate_case(case, observation, view)
        except ContractError as exc:
            results.append({"id": case.case_id, "status": "failed", "reason": str(exc)})
        else:
            results.append(
                {
                    "id": case.case_id,
                    "status": "passed",
                    "evidence": _receipt_evidence(observation, view["model_id"]),
                }
            )
    unexpected = sorted(set(observations) - {item["id"] for item in plan["cases"]})
    if unexpected:
        results.extend({"id": item, "status": "failed", "reason": "unplanned observation"} for item in unexpected)
    passed = sum(result["status"] == "passed" for result in results)
    failed = len(results) - passed
    return {
        "receipt_schema_version": 1,
        "mode": "offline_fixture",
        "network_execution": "disabled",
        "model_id": view["model_id"],
        "manifest_evidence_ids": plan["manifest_evidence_ids"],
        "manifest_sha256": plan["manifest_sha256"],
        "fixture_sha256": _digest(fixture),
        "status": "passed" if failed == 0 else "failed",
        "summary": {"planned": len(plan["cases"]), "passed": passed, "failed": failed},
        "results": results,
    }


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        _fail(f"{path} root must be an object")
    return value


def _write(value: dict[str, Any], output: Path | None) -> None:
    rendered = json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    if output is None:
        sys.stdout.write(rendered)
    else:
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(rendered, encoding="utf-8")


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    sub = root.add_subparsers(dest="command", required=True)
    plan = sub.add_parser("plan", help="emit a controlled live-probe plan; never performs network I/O")
    plan.add_argument("manifest", type=Path)
    plan.add_argument("--output", type=Path)
    run = sub.add_parser("run-fixture", help="verify offline observations and emit a receipt")
    run.add_argument("manifest", type=Path)
    run.add_argument("fixture", type=Path)
    run.add_argument("--output", type=Path)
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        manifest = load_json(args.manifest)
        if args.command == "plan":
            receipt = build_plan(manifest)
        else:
            receipt = run_fixture(manifest, load_json(args.fixture))
        _write(receipt, args.output)
        return 0 if receipt.get("status", "passed") == "passed" else 1
    except (OSError, json.JSONDecodeError, ContractError) as exc:
        error = {
            "receipt_schema_version": 1,
            "status": "failed",
            "error": str(exc),
            "network_execution": "disabled",
        }
        _write(error, getattr(args, "output", None))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

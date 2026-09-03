#!/usr/bin/env python3
"""Secret-safe, opt-in live harness for provider contract cases.

The existing :mod:`provider_contract_runner` remains the offline verifier and
CI default.  This companion command only performs network I/O through the
explicit ``run --execute --acknowledge-paid-probes`` gate.  ``plan`` is fully
offline and is the intended default.

Live execution is restricted to the Agent Switch secret
``LAOSHIRENAI_MODEL_MATRIX_TEST_KEY`` and owned API key 205.  The harness
switches that key group-by-group and restores group 6 in ``finally``.  Every
case is written atomically as a secret-free immutable receipt before the run
checkpoint advances, so interrupted batches resume without repeating passed
paid probes.
"""

from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import sys
import tempfile
import time
from typing import Any, Callable, Iterable
import urllib.error
import urllib.parse
import urllib.request


ROOT = Path(__file__).resolve().parents[1]
OFFLINE_RUNNER_PATH = ROOT / "scripts" / "provider_contract_runner.py"
GROUP_PROBE_PATH = (
    Path.home() / "laoshirenai" / ".codex" / "skills"
    / "laoshirenai-account-ops" / "scripts" / "group_matrix_probe.py"
)
SECRET_NAME = "LAOSHIRENAI_MODEL_MATRIX_TEST_KEY_USER2"
OWNED_USER_ID = 2
OWNED_KEY_ID = 128
RESTORE_GROUP_ID = 6
PROTOCOLS = {"responses", "chat_completions", "messages", "generate_content"}
BASE_CASES = ("P-01", "P-02", "P-03", "P-04", "P-05", "P-06", "P-12", "P-15")
OPTIONAL_CASES = {
    "prompt_cache": "P-07",
    "image_input": "P-08",
    "structured_output": "P-10",
    "web_search": "P-11",
}
CASE_NAMES = {
    "P-01": "minimal_text",
    "P-02": "streaming_sse",
    "P-03": "streaming_terminal",
    "P-04": "tool_call",
    "P-05": "tool_result_continuation",
    "P-06": "reasoning_transport",
    "P-07": "prompt_cache",
    "P-08": "image_input",
    "P-10": "structured_output",
    "P-11": "web_search",
    "P-12": "usage",
    "P-15": "error_passthrough",
}
OFFLINE_CAPABILITIES = {
    "P-01": "request",
    "P-03": "streaming_terminal",
    "P-04": "tool_call",
    "P-05": "tool_result_continuation",
    "P-07": "prompt_cache",
    "P-08": "image_input",
    "P-10": "structured_output",
    "P-11": "web_search",
    "P-12": "usage",
    "P-15": "error_passthrough",
}
QUEUE_CASE_NAMES = {
    "minimal_text": "P-01",
    "streaming_sse": ("P-02",),
    "streaming_terminal": ("P-02", "P-03"),
    "tool_call": "P-04",
    "tool.function_calling": "P-04",
    "tool_result_continuation": "P-05",
    "reasoning": "P-06",
    "reasoning_transport": "P-06",
    "prompt_cache": "P-07",
    "image_input": "P-08",
    "structured_output": "P-10",
    "web_search": "P-11",
    "usage": "P-12",
    "error_passthrough": "P-15",
    "invalid_request": "P-15",
    "billing": "P-12",
}
SECRET_PATTERNS = (
    re.compile(r"\bBearer\s+[A-Za-z0-9._~+/-]{8,}={0,2}", re.I),
    re.compile(r"\b(?:sk|rk|pk)-[A-Za-z0-9_-]{8,}\b", re.I),
    re.compile(r"AIza[A-Za-z0-9_-]{12,}"),
    re.compile(r"-----BEGIN [^-]*PRIVATE KEY-----", re.I),
)
SAFE_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]*$")
ONE_PIXEL_PNG = (
    # A real 32x32 RGB PNG. Some providers reject 1x1/transparent probes as an
    # invalid image even though the container is syntactically valid.
    "iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAIAAAD8GO2jAAAAJ0lEQVR42u3NsQkAAAjAsP7/"
    "tF7hIASyp6lTCQQCgUAgEAgEgi/BAjLD/C5w/SM9AAAAAElFTkSuQmCC"
)


class HarnessError(ValueError):
    """The harness input or a safety gate is invalid."""


@dataclass(frozen=True)
class LiveCase:
    case_key: str
    p_id: str
    name: str
    group_id: int
    group_name: str
    model_id: str
    protocol: str
    base_url: str
    marker: str
    reasoning_level: str | None = None
    declared_status: str | None = None
    source_case_keys: tuple[str, ...] = ()
    context_window: int = 1
    max_output_tokens: int = 1


@dataclass
class HTTPResult:
    status: int
    headers: dict[str, str]
    body: bytes
    error: dict[str, Any] | None
    timed_out: bool = False


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds")


def digest_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def digest_json(value: Any) -> str:
    payload = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
    return digest_bytes(payload)


def assert_secret_free(value: Any, label: str = "receipt") -> None:
    rendered = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False)
    if any(pattern.search(rendered) for pattern in SECRET_PATTERNS):
        raise HarnessError(f"secret-shaped value found in {label}")


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def atomic_json(path: Path, value: Any) -> None:
    assert_secret_free(value, str(path))
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(value, handle, ensure_ascii=False, indent=2, sort_keys=True)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def with_artifact_sha(value: dict[str, Any]) -> dict[str, Any]:
    result = dict(value)
    result.pop("artifact_sha256", None)
    result["artifact_sha256"] = digest_json(result)
    return result


def verify_receipt(path: Path, case_fingerprint: str) -> dict[str, Any] | None:
    if not path.is_file():
        return None
    try:
        receipt = load_json(path)
    except (OSError, json.JSONDecodeError):
        return None
    if not isinstance(receipt, dict) or receipt.get("case_fingerprint") != case_fingerprint:
        return None
    embedded = receipt.get("artifact_sha256")
    body = dict(receipt)
    body.pop("artifact_sha256", None)
    if embedded != digest_json(body):
        return None
    assert_secret_free(receipt, str(path))
    return receipt


def inventory_groups(raw: Any) -> dict[str, dict[str, Any]]:
    while isinstance(raw, dict) and isinstance(raw.get("data"), dict):
        raw = raw["data"]
    if not isinstance(raw, dict) or not isinstance(raw.get("groups"), list):
        raise HarnessError("pricing inventory must contain groups")
    result = {}
    for group in raw["groups"]:
        if isinstance(group, dict) and isinstance(group.get("name"), str) and isinstance(group.get("group_id"), int):
            result[group["name"]] = group
    return result


def verified_protocols(contract: dict[str, Any]) -> set[str]:
    return {
        row["name"] for row in contract.get("protocols", [])
        if isinstance(row, dict) and row.get("status") == "verified" and row.get("name") in PROTOCOLS
    }


def candidate_protocols(contract: dict[str, Any]) -> set[str]:
    rows = [
        *contract.get("protocols", []),
        *contract.get("protocol_candidates", []),
    ]
    return {
        row["name"] for row in rows
        if isinstance(row, dict)
        and row.get("status") in {"verified", "blocked"}
        and row.get("name") in PROTOCOLS
    }


def status_for(contract: dict[str, Any], protocol: str, feature: str) -> str | None:
    value = contract.get("test_matrix", {}).get("protocol_features", {}).get(protocol, {}).get(feature)
    return value.get("status") if isinstance(value, dict) else None


def structured_status(contract: dict[str, Any], protocol: str) -> str | None:
    value = contract.get("test_matrix", {}).get("tools", {}).get(protocol, {}).get("structured_output")
    if isinstance(value, dict):
        return value.get("status")
    value = contract.get("test_matrix", {}).get("protocol_features", {}).get(protocol, {}).get("structured_output")
    return value.get("status") if isinstance(value, dict) else None


def representative_reasoning(contract: dict[str, Any]) -> str | None:
    reasoning = contract.get("reasoning")
    if not isinstance(reasoning, dict):
        return None
    default = reasoning.get("default_level")
    levels = [value for value in reasoning.get("model_levels", []) if isinstance(value, str)]
    if isinstance(default, str) and default in levels:
        return default
    preferred = ("medium", "high", "adaptive", "low", "max", "minimal", "none", "disabled")
    return next((value for value in preferred if value in levels), levels[0] if levels else None)


def queue_index(raw: Any, include_satisfied: bool = False) -> dict[tuple[str, str, str], list[str]]:
    if raw is None:
        return {}
    if not isinstance(raw, dict) or raw.get("kind") != "model_client_unique_test_queue":
        raise HarnessError("--queue must be a model_client_unique_test_queue artifact")
    result: dict[tuple[str, str, str], list[str]] = {}
    for row in raw.get("queue", []):
        if not isinstance(row, dict) or row.get("case_type") not in {"provider_contract", "provider_tool_contract"}:
            continue
        if row.get("status") == "satisfied" and not include_satisfied:
            continue
        target = row.get("target")
        if not isinstance(target, dict):
            continue
        model, protocol, test_id = target.get("model_id"), target.get("protocol"), target.get("test_case_id")
        mapped = QUEUE_CASE_NAMES.get(test_id)
        p_ids = mapped if isinstance(mapped, tuple) else ((mapped,) if mapped else ())
        if isinstance(model, str) and protocol in PROTOCOLS:
            for p_id in p_ids:
                result.setdefault((model, protocol, p_id), []).append(str(row.get("case_key")))
    return result


def parse_csv(value: str | None) -> set[str]:
    return {item.strip() for item in (value or "").split(",") if item.strip()}


def build_cases(
    contracts_dir: Path,
    pricing_inventory: Path,
    *,
    queue_path: Path | None = None,
    models: set[str] | None = None,
    protocols: set[str] | None = None,
    group_ids: set[int] | None = None,
    p_ids: set[str] | None = None,
    include_satisfied: bool = False,
    include_candidate_protocols: bool = False,
    reasoning_levels: set[str] | None = None,
) -> list[LiveCase]:
    groups = inventory_groups(load_json(pricing_inventory))
    qindex = queue_index(load_json(queue_path), include_satisfied) if queue_path else {}
    result: list[LiveCase] = []
    requested = p_ids or set(BASE_CASES) | set(OPTIONAL_CASES.values())
    unknown = requested - set(CASE_NAMES)
    if unknown:
        raise HarnessError("unknown case IDs: " + ", ".join(sorted(unknown)))
    for path in sorted(contracts_dir.glob("*.json")):
        if path.name in {"client-matrix.json", "matrix-schema.json"}:
            continue
        contract = load_json(path)
        if not isinstance(contract, dict):
            continue
        model_id = contract.get("model", {}).get("id")
        if not isinstance(model_id, str) or not SAFE_ID.fullmatch(model_id):
            continue
        if models and model_id not in models:
            continue
        supported = verified_protocols(contract)
        selectable_protocols = candidate_protocols(contract) if include_candidate_protocols else supported
        access_groups = {
            row.get("name"): row for row in contract.get("access", {}).get("groups", [])
            if isinstance(row, dict) and isinstance(row.get("name"), str)
        }
        group_cells = contract.get("test_matrix", {}).get("group_access", {})
        for group_name, access in sorted(access_groups.items()):
            group = groups.get(group_name)
            if not group:
                continue
            group_id = group["group_id"]
            if group_ids and group_id not in group_ids:
                continue
            cell = group_cells.get(group_name) if isinstance(group_cells, dict) else None
            declared = cell.get("protocols") if isinstance(cell, dict) else None
            selected_protocols = set(declared or selectable_protocols).intersection(selectable_protocols)
            if include_candidate_protocols and protocols:
                selected_protocols.update(protocols.intersection(selectable_protocols))
            for protocol in sorted(selected_protocols):
                if protocols and protocol not in protocols:
                    continue
                selected: list[tuple[str, str | None, str | None]] = [
                    (p_id, None, None) for p_id in BASE_CASES if p_id in requested
                ]
                if "P-06" in requested:
                    # Replace the generic P-06 tuple with its representative
                    # exact model level; unsupported reasoning is omitted.
                    selected = [item for item in selected if item[0] != "P-06"]
                    if status_for(contract, protocol, "reasoning") != "unsupported":
                        declared_levels = {
                            level for level in contract.get("reasoning", {}).get("model_levels", [])
                            if isinstance(level, str)
                        }
                        if reasoning_levels:
                            for effort in sorted(reasoning_levels.intersection(declared_levels)):
                                selected.append(("P-06", effort, status_for(contract, protocol, "reasoning")))
                        else:
                            effort = representative_reasoning(contract)
                            if effort:
                                selected.append(("P-06", effort, status_for(contract, protocol, "reasoning")))
                for feature, p_id in OPTIONAL_CASES.items():
                    if p_id not in requested:
                        continue
                    status = structured_status(contract, protocol) if feature == "structured_output" else status_for(contract, protocol, feature)
                    if feature == "image_input" and contract.get("verification", {}).get("modalities", {}).get("image") == "unsupported":
                        continue
                    if status is not None and status != "unsupported":
                        selected.append((p_id, None, status))
                for p_id, reasoning, declared_status in selected:
                    source_keys = tuple(sorted(qindex.get((model_id, protocol, p_id), [])))
                    if queue_path and not source_keys:
                        continue
                    marker_seed = f"{group_id}:{model_id}:{protocol}:{p_id}:{reasoning or ''}"
                    marker = "LSR_CONTRACT_" + digest_bytes(marker_seed.encode())[:16].upper()
                    case_key = ":".join((str(group_id), model_id, protocol, p_id, reasoning or "default"))
                    result.append(LiveCase(
                        case_key=case_key,
                        p_id=p_id,
                        name=CASE_NAMES[p_id],
                        group_id=group_id,
                        group_name=group_name,
                        model_id=model_id,
                        protocol=protocol,
                        base_url=contract.get("access", {}).get("base_url") or "https://api.laoshirenai.com",
                        marker=marker,
                        reasoning_level=reasoning,
                        declared_status=declared_status,
                        source_case_keys=source_keys,
                        context_window=int(contract.get("model", {}).get("context_window") or 1),
                        max_output_tokens=int(contract.get("model", {}).get("max_output_tokens") or 1),
                    ))
    unique = {case.case_key: case for case in result}
    return [unique[key] for key in sorted(unique, key=lambda item: (int(item.split(":", 1)[0]), item))]


def plan_report(cases: list[LiveCase], output_dir: Path) -> dict[str, Any]:
    status_counts = {"planned": 0, "resumable": 0}
    rows = []
    for case in cases:
        fingerprint = digest_json(asdict(case))
        path = receipt_path(output_dir, case)
        prior = verify_receipt(path, fingerprint)
        state = "resumable" if prior and prior.get("result") == "pass" else "planned"
        status_counts[state] += 1
        rows.append({**asdict(case), "case_fingerprint": fingerprint, "state": state, "receipt": str(path)})
    return {
        "schema_version": 1,
        "kind": "provider_contract_live_plan",
        "network_execution": "disabled",
        "secret_source": f"Agent Switch name only: {SECRET_NAME}",
        "owned_identity": {"user_id": OWNED_USER_ID, "key_id": OWNED_KEY_ID, "restore_group_id": RESTORE_GROUP_ID},
        "case_count": len(cases),
        "paid_request_upper_bound": sum(2 if case.p_id in {"P-05", "P-07"} else 1 for case in cases),
        "status_counts": status_counts,
        "cases": rows,
        "plan_sha256": digest_json([asdict(case) for case in cases]),
    }


def endpoint(base_url: str, protocol: str, model_id: str, *, stream: bool = False) -> str:
    base = base_url.rstrip("/")
    if protocol == "responses":
        return base + ("/responses" if base.endswith("/v1") else "/v1/responses")
    if protocol == "chat_completions":
        return base + ("/chat/completions" if base.endswith("/v1") else "/v1/chat/completions")
    if protocol == "messages":
        return base + ("/messages" if base.endswith("/v1") else "/v1/messages")
    root = base[:-3] if base.endswith("/v1") else base
    suffix = ":streamGenerateContent?alt=sse" if stream else ":generateContent"
    return f"{root}/v1beta/models/{urllib.parse.quote(model_id, safe='._:-')}{suffix}"


def headers_for(protocol: str, key: str, user_agent: str) -> dict[str, str]:
    base = {"Accept": "application/json", "Content-Type": "application/json", "User-Agent": user_agent}
    if protocol == "messages":
        base.update({"x-api-key": key, "anthropic-version": "2023-06-01"})
    elif protocol == "generate_content":
        base["x-goog-api-key"] = key
    else:
        base["Authorization"] = "Bearer " + key
    return base


def request_payload(case: LiveCase, *, stream: bool = False, invalid: bool = False, cache: bool = False) -> dict[str, Any]:
    prompt = f"Reply with exactly {case.marker}."
    output_budget = 4096 if case.model_id in {"minimax-m3", "kimi-k2.7-code", "claude-fable-5"} else 96
    if cache:
        # Stay comfortably above provider cache-minimum thresholds; a 4K-char
        # prefix can tokenize to roughly the 1K boundary and miss by rounding.
        prompt = "CACHE_PREFIX_" + ("0123456789abcdef" * 1024) + "\n" + prompt
    if case.protocol == "responses":
        payload: dict[str, Any] = {"model": case.model_id, "input": prompt, "max_output_tokens": output_budget, "stream": stream}
        if case.p_id in {"P-04", "P-05"}:
            payload.update({
                "input": f"Call echo_contract with value {case.marker}.",
                "tools": [{"type": "function", "name": "echo_contract", "description": "Echo a value", "parameters": {"type": "object", "properties": {"value": {"type": "string"}}, "required": ["value"], "additionalProperties": False}}],
                "tool_choice": {"type": "function", "name": "echo_contract"},
            })
        elif case.p_id == "P-08":
            payload["input"] = [{"role": "user", "content": [{"type": "input_text", "text": prompt}, {"type": "input_image", "image_url": "data:image/png;base64," + ONE_PIXEL_PNG}]}]
        elif case.p_id == "P-10":
            payload["input"] = f'Return JSON with marker exactly "{case.marker}".'
            payload["text"] = {"format": {"type": "json_schema", "name": "contract", "strict": True, "schema": {"type": "object", "properties": {"marker": {"type": "string"}}, "required": ["marker"], "additionalProperties": False}}}
        elif case.p_id == "P-11":
            payload["input"] = f"Search the web for the official OpenAI homepage, cite it, then include marker {case.marker}."
            payload["tools"] = [{"type": "web_search"}]
        elif case.p_id == "P-06" and case.reasoning_level:
            payload["reasoning"] = {"effort": case.reasoning_level}
        if cache:
            payload["prompt_cache_key"] = digest_bytes(prompt.encode())[:32]
    elif case.protocol == "chat_completions":
        content: Any = prompt
        if case.p_id == "P-08":
            content = [{"type": "text", "text": prompt}, {"type": "image_url", "image_url": {"url": "data:image/png;base64," + ONE_PIXEL_PNG}}]
        payload = {"model": case.model_id, "messages": [{"role": "user", "content": content}], "max_tokens": output_budget, "stream": stream}
        if case.p_id in {"P-04", "P-05"}:
            payload.update({"messages": [{"role": "user", "content": f"Call echo_contract with value {case.marker}."}], "tools": [{"type": "function", "function": {"name": "echo_contract", "description": "Echo a value", "parameters": {"type": "object", "properties": {"value": {"type": "string"}}, "required": ["value"], "additionalProperties": False}}}], "tool_choice": {"type": "function", "function": {"name": "echo_contract"}}})
        elif case.p_id == "P-10":
            payload["messages"] = [{"role": "user", "content": f'Return JSON with marker exactly "{case.marker}".'}]
            payload["response_format"] = {"type": "json_schema", "json_schema": {"name": "contract", "strict": True, "schema": {"type": "object", "properties": {"marker": {"type": "string"}}, "required": ["marker"], "additionalProperties": False}}}
        elif case.p_id == "P-06" and case.reasoning_level:
            payload["reasoning_effort"] = case.reasoning_level
    elif case.protocol == "messages":
        content = prompt
        if case.p_id == "P-08":
            content = [{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": ONE_PIXEL_PNG}}, {"type": "text", "text": prompt}]
        payload = {"model": case.model_id, "messages": [{"role": "user", "content": content}], "max_tokens": max(1024, output_budget), "stream": stream}
        if case.p_id in {"P-04", "P-05"}:
            payload.update({"messages": [{"role": "user", "content": f"Call echo_contract with value {case.marker}."}], "tools": [{"name": "echo_contract", "description": "Echo a value", "input_schema": {"type": "object", "properties": {"value": {"type": "string"}}, "required": ["value"], "additionalProperties": False}}], "tool_choice": {"type": "tool", "name": "echo_contract"}})
        elif case.p_id == "P-06" and case.reasoning_level:
            payload["effort"] = case.reasoning_level
        elif case.p_id == "P-11":
            payload["messages"] = [{"role": "user", "content": f"Search the web for the official Anthropic homepage, cite it, then include marker {case.marker}."}]
            payload["tools"] = [{"type": "web_search_20250305", "name": "web_search", "max_uses": 1}]
            payload["tool_choice"] = {"type": "any"}
        if cache:
            payload["system"] = [{"type": "text", "text": "CACHE_PREFIX_" + ("0123456789abcdef" * 256), "cache_control": {"type": "ephemeral"}}]
    else:
        parts: list[dict[str, Any]] = [{"text": prompt}]
        if case.p_id == "P-08":
            parts.insert(0, {"inlineData": {"mimeType": "image/png", "data": ONE_PIXEL_PNG}})
        payload = {"contents": [{"role": "user", "parts": parts}], "generationConfig": {"maxOutputTokens": output_budget}}
        if case.p_id in {"P-04", "P-05"}:
            payload["contents"] = [{"role": "user", "parts": [{"text": f"Call echo_contract with value {case.marker}."}]}]
            payload["tools"] = [{"functionDeclarations": [{"name": "echo_contract", "description": "Echo a value", "parameters": {"type": "object", "properties": {"value": {"type": "string"}}, "required": ["value"]}}]}]
        elif case.p_id == "P-10":
            payload["contents"] = [{"role": "user", "parts": [{"text": f'Return JSON with marker exactly "{case.marker}".'}]}]
            payload["generationConfig"].update({"responseMimeType": "application/json", "responseSchema": {"type": "OBJECT", "properties": {"marker": {"type": "STRING"}}, "required": ["marker"]}})
        elif case.p_id == "P-11":
            payload["contents"] = [{"role": "user", "parts": [{"text": f"Search the web for the official Google AI homepage, cite it, then include marker {case.marker}."}]}]
            payload["tools"] = [{"googleSearch": {}}]
    if invalid:
        if case.protocol == "responses":
            payload["input"] = 42
        elif case.protocol in {"chat_completions", "messages"}:
            payload["messages"] = {"invalid": True}
        else:
            payload["contents"] = {"invalid": True}
    return payload


class URLTransport:
    def post(self, url: str, headers: dict[str, str], payload: dict[str, Any], timeout: int) -> HTTPResult:
        request = urllib.request.Request(
            url, data=json.dumps(payload, separators=(",", ":")).encode(), headers=headers, method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                return HTTPResult(response.status, dict(response.headers.items()), response.read(), None)
        except urllib.error.HTTPError as error:
            return HTTPResult(error.code, dict(error.headers.items()), error.read(), None)
        except (urllib.error.URLError, TimeoutError, OSError) as error:
            return HTTPResult(0, {}, b"", {"type": type(error).__name__, "message": str(error)[:400]}, isinstance(error, TimeoutError))


def decode_json(body: bytes) -> Any:
    try:
        return json.loads(body)
    except (json.JSONDecodeError, UnicodeDecodeError):
        return None


def sse_events(body: bytes) -> tuple[list[Any], int]:
    events: list[Any] = []
    errors = 0
    for raw in body.decode("utf-8", errors="replace").splitlines():
        line = raw.strip()
        if not line or line.startswith(("event:", "id:", ":")):
            continue
        if line == "data: [DONE]" or line == "[DONE]":
            events.append("[DONE]")
            continue
        if line.startswith("data:"):
            line = line[5:].strip()
        try:
            events.append(json.loads(line))
        except json.JSONDecodeError:
            errors += 1
    if not events:
        decoded = decode_json(body)
        if isinstance(decoded, list):
            events.extend(decoded)
        elif isinstance(decoded, dict):
            events.append(decoded)
    return events, errors


def extract_text(protocol: str, decoded: Any) -> str:
    if not isinstance(decoded, dict):
        return ""
    if protocol == "responses":
        if isinstance(decoded.get("output_text"), str):
            return decoded["output_text"]
        chunks = []
        for item in decoded.get("output") or []:
            for content in item.get("content") or [] if isinstance(item, dict) else []:
                if isinstance(content, dict) and isinstance(content.get("text"), str):
                    chunks.append(content["text"])
        return "".join(chunks)
    if protocol == "chat_completions":
        choices = decoded.get("choices") or []
        if choices and isinstance(choices[0], dict):
            message = choices[0].get("message") or {}
            return message.get("content") if isinstance(message.get("content"), str) else ""
    if protocol == "messages":
        return "".join(item.get("text", "") for item in decoded.get("content") or [] if isinstance(item, dict))
    candidates = decoded.get("candidates") or []
    if candidates and isinstance(candidates[0], dict):
        return "".join(item.get("text", "") for item in candidates[0].get("content", {}).get("parts", []) if isinstance(item, dict))
    return ""


def extract_usage(protocol: str, decoded: Any) -> dict[str, Any] | None:
    if not isinstance(decoded, dict):
        return None
    raw = decoded.get("usage") if protocol != "generate_content" else decoded.get("usageMetadata")
    if not isinstance(raw, dict):
        return None
    inp = raw.get("input_tokens", raw.get("prompt_tokens", raw.get("promptTokenCount")))
    out = raw.get("output_tokens", raw.get("completion_tokens", raw.get("candidatesTokenCount")))
    cached = raw.get("cached_input_tokens", raw.get("cache_read_input_tokens", raw.get("cachedContentTokenCount", 0)))
    total = raw.get("total_tokens", raw.get("totalTokenCount"))
    result = {"input_tokens": inp, "output_tokens": out, "cached_input_tokens": cached, "total_tokens": total}
    for key in ("input_tokens", "output_tokens", "cached_input_tokens"):
        if isinstance(result[key], bool) or not isinstance(result[key], (int, float)) or result[key] < 0:
            return None
    if result["total_tokens"] is None:
        result["total_tokens"] = result["input_tokens"] + result["output_tokens"]
    return result


def server_web_search_observed(protocol: str, decoded: Any) -> bool:
    if not isinstance(decoded, dict):
        return False
    if protocol == "responses":
        return any(
            isinstance(item, dict) and item.get("type") in {"web_search_call", "web_search_result"}
            for item in decoded.get("output") or []
        )
    if protocol == "messages":
        return any(
            isinstance(item, dict)
            and (
                item.get("type") == "web_search_tool_result"
                or (item.get("type") == "server_tool_use" and item.get("name") == "web_search")
            )
            for item in decoded.get("content") or []
        )
    if protocol == "generate_content":
        candidates = decoded.get("candidates") or []
        return any(
            isinstance(candidate, dict)
            and isinstance(candidate.get("groundingMetadata"), dict)
            and bool(
                candidate["groundingMetadata"].get("groundingChunks")
                or candidate["groundingMetadata"].get("webSearchQueries")
            )
            for candidate in candidates
        )
    # Chat Completions has no canonical server-side Web Search tool contract.
    return False


def extract_tool_call(protocol: str, decoded: Any) -> tuple[dict[str, Any] | None, Any]:
    if not isinstance(decoded, dict):
        return None, None
    raw: Any = None
    if protocol == "responses":
        raw = next((item for item in decoded.get("output") or [] if isinstance(item, dict) and item.get("type") == "function_call"), None)
        if raw:
            return {
                "name": raw.get("name"), "arguments": raw.get("arguments"),
                "call_id": raw.get("call_id") or raw.get("id"),
            }, raw
    elif protocol == "chat_completions":
        choices = decoded.get("choices") or []
        message = choices[0].get("message", {}) if choices and isinstance(choices[0], dict) else {}
        calls = message.get("tool_calls") or [] if isinstance(message, dict) else []
        raw = calls[0] if calls else None
        function = raw.get("function", {}) if isinstance(raw, dict) else {}
        if raw:
            return {"name": function.get("name"), "arguments": function.get("arguments"), "call_id": raw.get("id")}, message
    elif protocol == "messages":
        raw = next((item for item in decoded.get("content") or [] if isinstance(item, dict) and item.get("type") == "tool_use"), None)
        if raw:
            return {"name": raw.get("name"), "arguments": raw.get("input"), "call_id": raw.get("id")}, decoded.get("content")
    else:
        candidates = decoded.get("candidates") or []
        content = candidates[0].get("content", {}) if candidates and isinstance(candidates[0], dict) else {}
        parts = content.get("parts") or [] if isinstance(content, dict) else []
        raw = next((part.get("functionCall") for part in parts if isinstance(part, dict) and isinstance(part.get("functionCall"), dict)), None)
        if raw:
            return {"name": raw.get("name"), "arguments": raw.get("args"), "call_id": raw.get("id") or "gemini-call"}, content
    return None, None


def continuation_payload(case: LiveCase, first: dict[str, Any], tool: dict[str, Any], raw: Any) -> dict[str, Any]:
    followup = f"The tool returned {case.marker}. Reply with exactly {case.marker}."
    if case.protocol == "responses":
        prior_output = [item for item in first.get("output") or [] if isinstance(item, dict)]
        return {
            "model": case.model_id,
            # HTTP Responses rejects previous_response_id because that shortcut
            # belongs to Responses WebSocket v2. Replay the prior output and
            # append the correlated tool result instead.
            "input": prior_output + [
                {"type": "function_call_output", "call_id": tool["call_id"], "output": case.marker},
                {"role": "user", "content": followup},
            ],
            "max_output_tokens": 96,
        }
    if case.protocol == "chat_completions":
        return {
            "model": case.model_id,
            "messages": [
                {"role": "user", "content": f"Call echo_contract with value {case.marker}."},
                raw,
                {"role": "tool", "tool_call_id": tool["call_id"], "content": case.marker},
                {"role": "user", "content": followup},
            ],
            "max_tokens": 96,
        }
    if case.protocol == "messages":
        return {
            "model": case.model_id,
            "messages": [
                {"role": "user", "content": f"Call echo_contract with value {case.marker}."},
                {"role": "assistant", "content": raw},
                {"role": "user", "content": [{"type": "tool_result", "tool_use_id": tool["call_id"], "content": case.marker}, {"type": "text", "text": followup}]},
            ],
            "max_tokens": 1024,
        }
    return {
        "contents": [
            {"role": "user", "parts": [{"text": f"Call echo_contract with value {case.marker}."}]},
            {"role": "model", "parts": raw.get("parts", []) if isinstance(raw, dict) else []},
            {"role": "user", "parts": [{"functionResponse": {"name": tool["name"], "response": {"value": case.marker}}}, {"text": followup}]},
        ],
        "generationConfig": {"maxOutputTokens": 96},
    }


def response_shape(protocol: str, result: HTTPResult, decoded: Any, *, streaming: bool = False) -> dict[str, Any]:
    events, parse_errors = sse_events(result.body) if streaming else ([], 0)
    content_type = next((value for key, value in result.headers.items() if key.lower() == "content-type"), None)
    text = extract_text(protocol, decoded)
    terminal = False
    if streaming:
        kinds = []
        for event in events:
            if isinstance(event, str):
                kinds.append(event)
            elif isinstance(event, dict):
                kinds.append(event.get("type") or event.get("finish_reason") or event.get("finishReason"))
        terminal = any(value in {"response.completed", "message_stop", "[DONE]", "STOP", "MAX_TOKENS", "stop", "length", "tool_calls"} for value in kinds)
        if not text:
            rendered = json.dumps(events, ensure_ascii=False)
            text = rendered if rendered else ""
    elif isinstance(decoded, dict):
        if protocol == "responses":
            terminal = decoded.get("status") in {None, "completed"} and bool(decoded.get("output") or decoded.get("output_text"))
        elif protocol == "chat_completions":
            terminal = bool(decoded.get("choices"))
        elif protocol == "messages":
            terminal = bool(decoded.get("content"))
        else:
            terminal = bool(decoded.get("candidates"))
    error = result.error
    if result.status >= 400 and isinstance(decoded, dict):
        candidate = decoded.get("error", decoded)
        if isinstance(candidate, dict):
            error = {key: candidate.get(key) for key in ("type", "code", "status", "message") if candidate.get(key) is not None}
    return {
        "http_status": result.status,
        "content_type": content_type,
        "request_id": next((value for key, value in result.headers.items() if key.lower() in {"x-request-id", "request-id"}), None),
        "body_sha256": digest_bytes(result.body),
        "body_bytes": len(result.body),
        "complete": terminal,
        "text_present": bool(text),
        "marker_present": False,
        "event_count": len(events),
        "sse_parse_errors": parse_errors,
        "sse_framing": bool(
            streaming
            and (
                isinstance(content_type, str) and "text/event-stream" in content_type.casefold()
                or any(line.startswith((b"data:", b"event:")) for line in result.body.splitlines())
            )
        ),
        "error": error,
        "timed_out": result.timed_out,
    }


def safe_usage_rows(value: Any, started_at: str, case: LiveCase, user_agent: str) -> list[dict[str, Any]]:
    while isinstance(value, dict) and isinstance(value.get("data"), dict):
        value = value["data"]
    rows = []
    if isinstance(value, dict):
        value = value.get("items", value.get("list", []))
    if not isinstance(value, list):
        return []
    try:
        started = datetime.fromisoformat(started_at.replace("Z", "+00:00"))
    except ValueError:
        return []
    allowed = (
        "id", "request_id", "api_key_id", "group_id", "account_id", "model", "requested_model",
        "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
        "input_cost", "output_cost", "cache_creation_cost", "cache_read_cost", "total_cost", "actual_cost",
        "rate_multiplier", "account_rate_multiplier", "request_type", "stream", "duration_ms", "first_token_ms",
        "reasoning_effort", "inbound_endpoint", "upstream_endpoint", "created_at", "user_agent",
    )
    for row in value:
        if not isinstance(row, dict) or row.get("api_key_id") != OWNED_KEY_ID or row.get("group_id") != case.group_id:
            continue
        if row.get("model") != case.model_id and row.get("requested_model") != case.model_id:
            continue
        created_raw = row.get("created_at")
        try:
            created = datetime.fromisoformat(str(created_raw).replace("Z", "+00:00"))
        except ValueError:
            continue
        if created < started or row.get("user_agent") != user_agent:
            continue
        rows.append({key: row.get(key) for key in allowed if key in row})
    return rows


class ProductionController:
    def __init__(self) -> None:
        if os.environ.get(SECRET_NAME):
            raise HarnessError(f"{SECRET_NAME} must come from Agent Switch, not the process environment")
        if not GROUP_PROBE_PATH.is_file():
            raise HarnessError(f"owned group controller is missing: {GROUP_PROBE_PATH}")
        spec = importlib.util.spec_from_file_location("provider_live_group_controller", GROUP_PROBE_PATH)
        if spec is None or spec.loader is None:
            raise HarnessError("cannot load owned group controller")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        self.module = module

    def open(self) -> str:
        last_error: Exception | None = None
        for _ in range(5):
            try:
                current = self.module.owned_key()
                if current.get("id") != OWNED_KEY_ID or current.get("user_id") != OWNED_USER_ID:
                    raise HarnessError("owned key identity mismatch")
                if current.get("group_id") != RESTORE_GROUP_ID:
                    raise HarnessError(f"owned key must start in restore group {RESTORE_GROUP_ID}")
                return current["key"]
            except Exception as error:
                last_error = error
                time.sleep(1)
        raise HarnessError("owned key readback failed after retries") from last_error

    def switch(self, group_id: int) -> None:
        last_error: Exception | None = None
        for _ in range(5):
            try:
                current = self.module.switch_group(group_id)
                if current.get("group_id") != group_id:
                    raise HarnessError(f"group switch readback failed for {group_id}")
                return
            except Exception as error:
                last_error = error
                time.sleep(1)
        raise HarnessError(f"group switch failed after retries for {group_id}") from last_error

    def restore(self) -> None:
        self.switch(RESTORE_GROUP_ID)

    def usage(self, case: LiveCase, started_at: str, user_agent: str) -> list[dict[str, Any]]:
        day = started_at[:10]
        path = (
            f"/admin/usage?page=1&page_size=200&api_key_id={OWNED_KEY_ID}"
            f"&group_id={case.group_id}&model={urllib.parse.quote(case.model_id)}"
            f"&start_date={day}&timezone=UTC"
        )
        for _ in range(5):
            try:
                raw = self.module.admin_cli_json(["api", "GET", path])
            except Exception:
                # Usage attribution is a secondary readback. A transient admin
                # API failure blocks this receipt, but must not abort the batch
                # or discard already-written immutable case receipts.
                time.sleep(1)
                continue
            rows = safe_usage_rows(raw, started_at, case, user_agent)
            if rows:
                return rows
            time.sleep(1)
        return []


def receipt_path(output_dir: Path, case: LiveCase) -> Path:
    slug = re.sub(r"[^A-Za-z0-9._-]+", "_", case.case_key)[:160]
    return output_dir / "cases" / f"{slug}-{digest_bytes(case.case_key.encode())[:12]}.json"


def blocked_classification(shape: dict[str, Any]) -> str:
    if shape.get("timed_out") or shape.get("http_status") == 0:
        return "timeout"
    if shape.get("http_status") == 503:
        return "upstream_overloaded"
    if shape.get("http_status") == 200 and not shape.get("complete"):
        return "http_200_empty_or_incomplete_terminal"
    return "contract_failed"


def offline_runner() -> Any:
    spec = importlib.util.spec_from_file_location("provider_contract_offline_for_live", OFFLINE_RUNNER_PATH)
    if spec is None or spec.loader is None:
        raise HarnessError("offline provider verifier is unavailable")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def minimal_manifest(case: LiveCase) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "model": {
            "id": case.model_id, "upstream_id": case.model_id,
            "display_name": case.model_id, "platform": "openai",
            "context_window": case.context_window, "max_output_tokens": case.max_output_tokens,
        },
        "capabilities": {
            "protocols": [case.protocol], "streaming": True, "tools": True,
            "image_input": True, "cache_read": True,
            "reasoning_efforts": [case.reasoning_level] if case.reasoning_level else [],
        },
        "pricing": {"input_per_mtok_usd": 0, "cached_input_per_mtok_usd": 0, "output_per_mtok_usd": 0, "long_context": None},
        "production": {"group_name": case.group_name},
        "verification": {"expected_text": case.marker},
    }


def verifier_observation(case: LiveCase, shape: dict[str, Any], usage: dict[str, Any] | None, details: dict[str, Any]) -> tuple[str, str | None]:
    if case.p_id == "P-02":
        ok = shape.get("http_status") == 200 and shape.get("sse_framing") is True and shape.get("event_count", 0) > 0 and shape.get("sse_parse_errors") == 0
        return ("passed", None) if ok else ("failed", "stream is not valid SSE")
    runner = offline_runner()
    capability = OFFLINE_CAPABILITIES.get(case.p_id)
    if case.p_id == "P-06":
        capability = f"reasoning.{case.reasoning_level}"
    if not capability:
        return "failed", "case is not mapped to offline verifier"
    observation: dict[str, Any] = {
        "evidence": {"source": "live_probe", "id": "live-" + digest_bytes(case.case_key.encode())[:20]},
        "observed_at": details["observed_at"],
    }
    if case.p_id == "P-01":
        observation.update(status_code=shape["http_status"], text=case.marker if shape.get("marker_present") else "")
    elif case.p_id == "P-03":
        observation.update(status_code=shape["http_status"], events=details.get("events", []), complete=shape.get("complete"), text=case.marker if shape.get("marker_present") else "")
    elif case.p_id == "P-04":
        observation["tool_call"] = details.get("tool_call")
    elif case.p_id == "P-05":
        observation.update(correlated=details.get("correlated") is True, final_text=case.marker if shape.get("marker_present") else "")
    elif case.p_id == "P-06":
        observation.update(effort=case.reasoning_level, reasoning=details.get("reasoning") or {"transport_accepted": shape.get("complete") is True})
    elif case.p_id == "P-07":
        observation["usage"] = usage
    elif case.p_id == "P-08":
        observation.update(accepted=shape.get("http_status") == 200 and shape.get("complete"), final_text=case.marker if shape.get("marker_present") else "")
    elif case.p_id == "P-10":
        observation.update(schema_valid=details.get("schema_valid") is True, output=details.get("structured_output"))
    elif case.p_id == "P-11":
        observation.update(citations=details.get("citations", []), final_text=case.marker if shape.get("text_present") else "")
    elif case.p_id == "P-12":
        observation["usage"] = usage
    elif case.p_id == "P-15":
        error = shape.get("error") or {}
        observation.update(status_code=shape.get("http_status"), error={"code": str(error.get("code") or error.get("type") or error.get("status") or "provider_error"), "message": str(error.get("message") or "provider rejected invalid request")})
    try:
        view = runner._manifest_view(minimal_manifest(case))
        runner._validate_case(runner.ContractCase(case.case_key, case.protocol, capability, {}), observation, view)
    except Exception as exc:  # ContractError is private to the imported module
        return "failed", str(exc)
    return "passed", None


def run_http_case(case: LiveCase, key: str, transport: Any, timeout: int, user_agent: str) -> tuple[dict[str, Any], dict[str, Any], dict[str, Any] | None]:
    streaming = case.p_id in {"P-02", "P-03"}
    invalid = case.p_id == "P-15"
    payload = request_payload(case, stream=streaming, invalid=invalid, cache=case.p_id == "P-07")
    url = endpoint(case.base_url, case.protocol, case.model_id, stream=streaming)
    result = transport.post(url, headers_for(case.protocol, key, user_agent), payload, timeout)
    decoded = decode_json(result.body) if not streaming else None
    shape = response_shape(case.protocol, result, decoded, streaming=streaming)
    details: dict[str, Any] = {"observed_at": utc_now(), "request_payload_sha256": digest_json(payload)}
    if streaming:
        events, _ = sse_events(result.body)
        details["events"] = events
        rendered = json.dumps(events, ensure_ascii=False)
        shape["marker_present"] = case.marker in rendered
        shape["text_present"] = bool(rendered)
    else:
        text = extract_text(case.protocol, decoded)
        shape["marker_present"] = case.marker in text
        shape["text_present"] = bool(text)
    usage = extract_usage(case.protocol, decoded)
    if case.p_id in {"P-04", "P-05"}:
        tool, raw_tool = extract_tool_call(case.protocol, decoded)
        details["tool_call"] = tool
        if case.p_id == "P-05" and tool and isinstance(decoded, dict):
            second_payload = continuation_payload(case, decoded, tool, raw_tool)
            second = transport.post(
                endpoint(case.base_url, case.protocol, case.model_id),
                headers_for(case.protocol, key, user_agent), second_payload, timeout,
            )
            second_decoded = decode_json(second.body)
            shape = response_shape(case.protocol, second, second_decoded)
            final_text = extract_text(case.protocol, second_decoded)
            shape["marker_present"] = case.marker in final_text
            shape["text_present"] = bool(final_text)
            details["correlated"] = shape["http_status"] == 200 and shape["complete"] and shape["marker_present"]
            details["request_payload_sha256"] = digest_json(second_payload)
            details["attempts"] = 2
            usage = extract_usage(case.protocol, second_decoded)
    if case.p_id == "P-06" and isinstance(decoded, dict):
        details["reasoning"] = decoded.get("reasoning") or decoded.get("thinking")
    if case.p_id == "P-10" and shape["text_present"]:
        text = extract_text(case.protocol, decoded)
        try:
            output = json.loads(text)
        except json.JSONDecodeError:
            output = None
        details["structured_output"] = output
        details["schema_valid"] = isinstance(output, dict) and output.get("marker") == case.marker
    if case.p_id == "P-11" and isinstance(decoded, dict):
        rendered = json.dumps(decoded, ensure_ascii=False)
        details["citations"] = sorted(set(re.findall(r"https?://[^\s\"']+", rendered)))
        details["server_tool_observed"] = server_web_search_observed(case.protocol, decoded)
        shape["server_tool_observed"] = details["server_tool_observed"]
    if case.p_id == "P-07" and shape.get("http_status") == 200:
        # Some upstream caches publish the newly written prefix asynchronously.
        # A short bounded delay avoids a false negative from an immediate read.
        time.sleep(3)
        second = transport.post(url, headers_for(case.protocol, key, user_agent), request_payload(case, cache=True), timeout)
        second_decoded = decode_json(second.body)
        shape = response_shape(case.protocol, second, second_decoded)
        text = extract_text(case.protocol, second_decoded)
        shape["marker_present"] = case.marker in text
        shape["text_present"] = bool(text)
        usage = extract_usage(case.protocol, second_decoded)
        details["request_payload_sha256"] = digest_json(request_payload(case, cache=True))
        details["attempts"] = 2
    return shape, details, usage


def execute_case(case: LiveCase, key: str, transport: Any, controller: Any, timeout: int, run_id: str) -> dict[str, Any]:
    started_at = utc_now()
    user_agent = f"laoshirenai-provider-contract-live/1.0 run/{run_id} case/{digest_bytes(case.case_key.encode())[:12]}"
    shape, details, usage = run_http_case(case, key, transport, timeout, user_agent)
    verification, reason = verifier_observation(case, shape, usage, details)
    if case.p_id != "P-15" and not (
        shape.get("http_status") == 200 and shape.get("complete") is True
    ):
        verification = "failed"
        reason = reason or "provider request did not return a complete HTTP 200 terminal result"
    if case.p_id == "P-11" and details.get("server_tool_observed") is not True:
        verification = "failed"
        reason = "web search response lacks a protocol-native server tool or grounding record"
    if verification == "passed":
        result = "pass"
        classification = "verified"
    else:
        result = "blocked"
        classification = blocked_classification(shape)
    usage_rows = []
    if shape.get("http_status") == 200:
        usage_rows = controller.usage(case, started_at, user_agent)
    usage_attribution = {
        "status": "verified" if usage_rows else ("not_applicable" if case.p_id == "P-15" else "blocked"),
        "row_count": len(usage_rows),
        "rows": usage_rows,
    }
    if result == "pass" and case.p_id in {"P-01", "P-04", "P-05", "P-06", "P-07", "P-08", "P-10", "P-11", "P-12"} and not usage_rows:
        result = "blocked"
        classification = "usage_attribution_missing"
    body = {
        "schema_version": 1,
        "kind": "provider_contract_live_case",
        "case_fingerprint": digest_json(asdict(case)),
        "case": asdict(case),
        "result": result,
        "classification": classification,
        "reason": reason,
        "observed_at": started_at,
        "finished_at": utc_now(),
        "request": {"url": endpoint(case.base_url, case.protocol, case.model_id, stream=case.p_id in {"P-02", "P-03"}), "payload_sha256": details["request_payload_sha256"], "user_agent_sha256": digest_bytes(user_agent.encode())},
        "response": shape,
        "usage": usage,
        "usage_attribution": usage_attribution,
        "billing_attribution": {
            "status": usage_attribution["status"],
            "total_cost": sum(float(row.get("total_cost") or 0) for row in usage_rows),
            "actual_cost": sum(float(row.get("actual_cost") or 0) for row in usage_rows),
            "usage_row_ids": [str(row.get("id")) for row in usage_rows if row.get("id") is not None],
        },
        "offline_verifier": {"status": verification, "reason": reason},
        "owned_identity": {"user_id": OWNED_USER_ID, "key_id": OWNED_KEY_ID, "group_id": case.group_id},
        "secret_free": True,
    }
    return with_artifact_sha(body)


def run_cases(
    cases: list[LiveCase], output_dir: Path, *, timeout: int,
    controller_factory: Callable[[], Any] = ProductionController,
    transport_factory: Callable[[], Any] = URLTransport,
) -> dict[str, Any]:
    output_dir.mkdir(parents=True, exist_ok=True)
    run_id = digest_json([asdict(case) for case in cases])[:16]
    state_path = output_dir / "run-state.json"
    state: dict[str, Any] = {
        "schema_version": 1, "kind": "provider_contract_live_state", "run_id": run_id,
        "network_execution": "explicit_live", "started_at": utc_now(), "restored_group_id": None,
        "cases": {}, "secret_free": True,
    }
    controller = controller_factory()
    transport = transport_factory()
    key = ""
    current_group = None
    try:
        key = controller.open()
        if not isinstance(key, str) or not key.startswith("sk-"):
            raise HarnessError("owned key secret is unavailable")
        for case in cases:
            fingerprint = digest_json(asdict(case))
            path = receipt_path(output_dir, case)
            prior = verify_receipt(path, fingerprint)
            if prior and prior.get("result") == "pass":
                state["cases"][case.case_key] = {"status": "resumed", "receipt": str(path), "artifact_sha256": prior["artifact_sha256"]}
                atomic_json(state_path, state)
                continue
            if current_group != case.group_id:
                controller.switch(case.group_id)
                current_group = case.group_id
            receipt = execute_case(case, key, transport, controller, timeout, run_id)
            atomic_json(path, receipt)
            state["cases"][case.case_key] = {"status": receipt["result"], "receipt": str(path), "artifact_sha256": receipt["artifact_sha256"]}
            atomic_json(state_path, state)
    finally:
        key = ""
        controller.restore()
        state["restored_group_id"] = RESTORE_GROUP_ID
        state["finished_at"] = utc_now()
        atomic_json(state_path, state)
    counts: dict[str, int] = {}
    for row in state["cases"].values():
        counts[row["status"]] = counts.get(row["status"], 0) + 1
    summary = with_artifact_sha({
        "schema_version": 1, "kind": "provider_contract_live_run",
        "run_id": run_id, "network_execution": "explicit_live",
        "case_count": len(cases), "status_counts": counts,
        "restored_group_id": state["restored_group_id"],
        "state": str(state_path), "secret_free": True,
    })
    atomic_json(output_dir / "summary.json", summary)
    return summary


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    root.add_argument("command", choices=("plan", "run"))
    root.add_argument("--contracts", type=Path, default=ROOT / "model-doc-contracts")
    root.add_argument("--pricing-inventory", type=Path, default=ROOT / "artifacts" / "public-model-pricing-20260831.json")
    root.add_argument("--queue", type=Path)
    root.add_argument("--model", help="comma-separated exact model IDs")
    root.add_argument("--protocol", help="comma-separated exact protocols")
    root.add_argument("--group-id", help="comma-separated numeric group IDs")
    root.add_argument("--case", help="comma-separated P case IDs")
    root.add_argument("--include-satisfied", action="store_true")
    root.add_argument("--include-candidate-protocols", action="store_true", help="explicitly probe blocked protocol candidates selected by --protocol")
    root.add_argument("--reasoning-level", help="comma-separated exact model reasoning levels for P-06")
    root.add_argument("--output-dir", type=Path, required=True)
    root.add_argument("--timeout", type=int, default=90)
    root.add_argument("--execute", action="store_true")
    root.add_argument("--acknowledge-paid-probes", action="store_true")
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        protocols = parse_csv(args.protocol)
        if protocols - PROTOCOLS:
            raise HarnessError("unsupported protocols: " + ", ".join(sorted(protocols - PROTOCOLS)))
        groups = parse_csv(args.group_id)
        try:
            group_ids = {int(value) for value in groups}
        except ValueError as exc:
            raise HarnessError("--group-id must contain integers") from exc
        cases = build_cases(
            args.contracts, args.pricing_inventory,
            queue_path=args.queue,
            models=parse_csv(args.model), protocols=protocols,
            group_ids=group_ids, p_ids=parse_csv(args.case),
            include_satisfied=args.include_satisfied,
            include_candidate_protocols=args.include_candidate_protocols,
            reasoning_levels=parse_csv(args.reasoning_level),
        )
        if not cases:
            raise HarnessError("filters selected no live provider cases")
        if args.command == "plan":
            report = plan_report(cases, args.output_dir)
            atomic_json(args.output_dir / "plan.json", report)
            print(json.dumps({"case_count": report["case_count"], "paid_request_upper_bound": report["paid_request_upper_bound"], "plan": str(args.output_dir / "plan.json"), "network_execution": "disabled"}, ensure_ascii=False))
            return 0
        if not args.execute or not args.acknowledge_paid_probes:
            raise HarnessError("live run requires both --execute and --acknowledge-paid-probes")
        summary = run_cases(cases, args.output_dir, timeout=args.timeout)
        print(json.dumps({"case_count": summary["case_count"], "status_counts": summary["status_counts"], "restored_group_id": summary["restored_group_id"], "summary": str(args.output_dir / "summary.json")}, ensure_ascii=False))
        return 0 if summary["status_counts"].get("blocked", 0) == 0 else 2
    except (OSError, json.JSONDecodeError, HarnessError) as exc:
        error = {"schema_version": 1, "kind": "provider_contract_live_error", "network_execution": "disabled" if args.command == "plan" or not args.execute else "explicit_live", "error": str(exc), "secret_free": True}
        atomic_json(args.output_dir / "error.json", error)
        print(json.dumps(error, ensure_ascii=False))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

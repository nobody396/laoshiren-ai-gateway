#!/usr/bin/env python3
"""Create, validate, and plan a secret-free model release manifest."""

from __future__ import annotations

import argparse
from datetime import date, datetime
import json
import math
from pathlib import Path
import re
import sys
from typing import Any
from urllib.parse import urlparse

PLATFORMS = {"openai", "anthropic", "grok", "gemini"}
PROTOCOLS = {"responses", "chat_completions", "messages", "generate_content"}
PROTOCOL_SUPPORT = {"supported", "unsupported", "untested"}
PROTOCOL_RECOMMENDATIONS = {"preferred", "allowed", "discouraged", "not_applicable"}
FEATURE_RESULTS = {"pass", "fail", "untested", "not_applicable"}
REASONING_SUPPORT = {"supported", "unsupported", "untested"}
PUBLIC_STATUSES = {"draft", "public", "hidden", "deprecated"}
EVIDENCE_KINDS = {
    "official_docs",
    "provider_api",
    "live_probe",
    "billing_reconciliation",
    "release_artifact",
}
EVIDENCE_RESULTS = {"pass", "fail", "pending"}
REQUIRED_PROTOCOL_FEATURES = {
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
V2_SECTIONS = {
    "lifecycle",
    "protocol_matrix",
    "protocol_feature_matrix",
    "reasoning_matrix",
    "price_matrix",
    "evidence",
}
FORBIDDEN_KEY = re.compile(
    r"(api[_-]?key|access[_-]?token|refresh[_-]?token|password|credential|secret_value|authorization|cookie|client[_-]?secret|private[_-]?key)",
    re.I,
)
SECRET_VALUE = re.compile(r"(?:sk-|Bearer\s+)[A-Za-z0-9_-]{8,}", re.I)
MODEL_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]*$")
EVIDENCE_ID = re.compile(r"^[a-z0-9][a-z0-9._-]*$")
SHA256 = re.compile(r"^[a-fA-F0-9]{64}$")
CODEX_REQUIRED_FIELDS = {
    "slug",
    "display_name",
    "base_instructions",
    "supports_reasoning_summaries",
    "visibility",
    "context_window",
    "max_context_window",
    "auto_compact_token_limit",
}


def fail(message: str) -> None:
    raise ValueError(message)


def walk(value: Any, path: str = "$") -> None:
    if isinstance(value, dict):
        for key, item in value.items():
            if FORBIDDEN_KEY.search(str(key)):
                fail(f"forbidden credential-shaped field: {path}.{key}")
            walk(item, f"{path}.{key}")
    elif isinstance(value, list):
        for index, item in enumerate(value):
            walk(item, f"{path}[{index}]")
    elif isinstance(value, str) and SECRET_VALUE.search(value):
        fail(f"secret-shaped value at {path}")


def require_number(value: Any, path: str, *, allow_zero: bool = False) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        fail(f"{path} must be numeric")
    number = float(value)
    if not math.isfinite(number):
        fail(f"{path} must be finite")
    if number < 0 or (number == 0 and not allow_zero):
        fail(f"{path} must be {'non-negative' if allow_zero else 'positive'}")
    return number


def require_string_array(value: Any, path: str, *, allow_empty: bool = True) -> list[str]:
    if not isinstance(value, list) or not all(isinstance(item, str) and item.strip() for item in value):
        fail(f"{path} must be a string array")
    if not allow_empty and not value:
        fail(f"{path} must not be empty")
    return value


def require_date(value: Any, path: str, *, nullable: bool = True) -> None:
    if value is None and nullable:
        return
    if not isinstance(value, str):
        fail(f"{path} must be an ISO date")
    try:
        date.fromisoformat(value)
    except ValueError:
        fail(f"{path} must be an ISO date")


def require_datetime(value: Any, path: str) -> None:
    if not isinstance(value, str) or "T" not in value:
        fail(f"{path} must be an ISO-8601 timestamp")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        fail(f"{path} must be an ISO-8601 timestamp")
    if parsed.tzinfo is None:
        fail(f"{path} must include a timezone")


def validate_long_context(value: Any, path: str) -> None:
    if value is None:
        return
    if not isinstance(value, dict):
        fail(f"{path} must be an object")
    require_number(value.get("input_threshold"), f"{path}.input_threshold")
    require_number(value.get("input_multiplier"), f"{path}.input_multiplier")
    require_number(value.get("output_multiplier"), f"{path}.output_multiplier")


def validate_legacy_sections(data: dict[str, Any]) -> None:
    model = data.get("model")
    if not isinstance(model, dict):
        fail("model must be an object")
    for field in ("id", "upstream_id", "display_name"):
        if not isinstance(model.get(field), str) or not model[field].strip():
            fail(f"model.{field} is required")
    if not MODEL_ID.fullmatch(model["id"]) or not MODEL_ID.fullmatch(model["upstream_id"]):
        fail("model.id and model.upstream_id must be installer-safe identifiers")
    if model.get("platform") not in PLATFORMS:
        fail("model.platform must be openai, anthropic, grok, or gemini")
    require_number(model.get("context_window"), "model.context_window")
    require_number(model.get("max_output_tokens"), "model.max_output_tokens")
    codex_entry = model.get("codex_catalog_entry")
    if codex_entry is not None:
        if model.get("platform") != "openai" or not isinstance(codex_entry, dict):
            fail("model.codex_catalog_entry is only valid for OpenAI models")
        if codex_entry.get("slug") != model["id"]:
            fail("model.codex_catalog_entry.slug must match model.id")
        missing = sorted(CODEX_REQUIRED_FIELDS - codex_entry.keys())
        if missing:
            fail("model.codex_catalog_entry is missing: " + ", ".join(missing))
        if codex_entry.get("context_window") != model["context_window"]:
            fail("model.codex_catalog_entry.context_window must match model.context_window")

    capabilities = data.get("capabilities")
    if not isinstance(capabilities, dict) or capabilities.get("protocol") not in PROTOCOLS:
        fail("capabilities.protocol is invalid")
    require_string_array(capabilities.get("reasoning_efforts", []), "capabilities.reasoning_efforts")

    pricing = data.get("pricing")
    if not isinstance(pricing, dict):
        fail("pricing must be an object")
    for field in ("input_per_mtok_usd", "output_per_mtok_usd"):
        require_number(pricing.get(field), f"pricing.{field}")
    if pricing.get("cached_input_per_mtok_usd") is not None:
        require_number(pricing["cached_input_per_mtok_usd"], "pricing.cached_input_per_mtok_usd", allow_zero=True)
    evidence_url = pricing.get("evidence_url")
    parsed = urlparse(evidence_url if isinstance(evidence_url, str) else "")
    if parsed.scheme != "https" or not parsed.netloc:
        fail("pricing.evidence_url must be an https URL")
    validate_long_context(pricing.get("long_context"), "pricing.long_context")

    production = data.get("production")
    if not isinstance(production, dict):
        fail("production must be an object")
    if not isinstance(production.get("group_name"), str) or not production["group_name"].strip():
        fail("production.group_name is required")
    if production.get("group_description", "") != "":
        fail("production.group_description must stay empty unless handled by explicit owner exception")
    aliases = require_string_array(production.get("aliases", []), "production.aliases")
    for field in ("legacy_group_names", "managed_predecessor_ids"):
        require_string_array(production.get(field, []), f"production.{field}")
    if not all(MODEL_ID.fullmatch(value) for value in production.get("managed_predecessor_ids", [])):
        fail("production.managed_predecessor_ids must contain installer-safe identifiers")
    if any(alias in {"latest", "grok-latest"} for alias in aliases):
        fail("generic latest aliases require a separate explicit rollout decision")
    if model.get("platform") == "openai" and production.get("client_default") and codex_entry is None:
        fail("OpenAI client defaults require model.codex_catalog_entry")

    verification = data.get("verification")
    if not isinstance(verification, dict) or not isinstance(verification.get("expected_text"), str):
        fail("verification.expected_text is required")


def validate_evidence(data: dict[str, Any]) -> set[str]:
    evidence = data.get("evidence")
    if not isinstance(evidence, list):
        fail("evidence must be an array")
    ids: set[str] = set()
    for index, row in enumerate(evidence):
        path = f"evidence[{index}]"
        if not isinstance(row, dict):
            fail(f"{path} must be an object")
        evidence_id = row.get("id")
        if not isinstance(evidence_id, str) or not EVIDENCE_ID.fullmatch(evidence_id):
            fail(f"{path}.id must be a lowercase evidence identifier")
        if evidence_id in ids:
            fail(f"duplicate evidence id: {evidence_id}")
        ids.add(evidence_id)
        if row.get("kind") not in EVIDENCE_KINDS:
            fail(f"{path}.kind is invalid")
        require_datetime(row.get("observed_at"), f"{path}.observed_at")
        if not isinstance(row.get("subject_version"), str) or not row["subject_version"].strip():
            fail(f"{path}.subject_version is required")
        if row.get("result") not in EVIDENCE_RESULTS:
            fail(f"{path}.result is invalid")
        url = row.get("url")
        digest = row.get("artifact_sha256")
        if url is not None:
            parsed = urlparse(url if isinstance(url, str) else "")
            if parsed.scheme != "https" or not parsed.netloc:
                fail(f"{path}.url must be null or an https URL")
        if digest is not None and (not isinstance(digest, str) or not SHA256.fullmatch(digest)):
            fail(f"{path}.artifact_sha256 must be null or a SHA-256 hex digest")
        if url is None and digest is None:
            fail(f"{path} requires url or artifact_sha256")
    return ids


def validate_evidence_ids(value: Any, path: str, known_ids: set[str]) -> list[str]:
    ids = require_string_array(value, path)
    unknown = sorted(set(ids) - known_ids)
    if unknown:
        fail(f"{path} references unknown evidence ids: {', '.join(unknown)}")
    return ids


def validate_protocol_matrix(data: dict[str, Any], evidence_ids: set[str]) -> tuple[set[str], str | None]:
    rows = data.get("protocol_matrix")
    if not isinstance(rows, list) or not rows:
        fail("protocol_matrix must be a non-empty array")
    protocols: set[str] = set()
    preferred: str | None = None
    supported: set[str] = set()
    for index, row in enumerate(rows):
        path = f"protocol_matrix[{index}]"
        if not isinstance(row, dict):
            fail(f"{path} must be an object")
        protocol = row.get("protocol")
        if protocol not in PROTOCOLS:
            fail(f"{path}.protocol is invalid")
        if protocol in protocols:
            fail(f"duplicate protocol_matrix protocol: {protocol}")
        protocols.add(protocol)
        support = row.get("support")
        recommendation = row.get("recommendation")
        if support not in PROTOCOL_SUPPORT:
            fail(f"{path}.support is invalid")
        if recommendation not in PROTOCOL_RECOMMENDATIONS:
            fail(f"{path}.recommendation is invalid")
        if not isinstance(row.get("recommendation_reason"), str) or not row["recommendation_reason"].strip():
            fail(f"{path}.recommendation_reason is required")
        if recommendation == "preferred":
            if support != "supported":
                fail(f"{path}: preferred protocol must be supported")
            if preferred is not None:
                fail("protocol_matrix must have at most one preferred protocol")
            preferred = protocol
        if support != "supported" and recommendation in {"preferred", "allowed"}:
            fail(f"{path}: {recommendation} protocol must be supported")
        if support == "supported":
            supported.add(protocol)
        validate_evidence_ids(row.get("evidence_ids", []), f"{path}.evidence_ids", evidence_ids)
    return supported, preferred


def validate_protocol_features(
    data: dict[str, Any], declared: set[str], supported: set[str], evidence_ids: set[str]
) -> dict[str, dict[str, str]]:
    rows = data.get("protocol_feature_matrix")
    if not isinstance(rows, list):
        fail("protocol_feature_matrix must be an array")
    by_protocol: dict[str, dict[str, str]] = {}
    for index, row in enumerate(rows):
        path = f"protocol_feature_matrix[{index}]"
        if not isinstance(row, dict):
            fail(f"{path} must be an object")
        protocol = row.get("protocol")
        if protocol not in PROTOCOLS:
            fail(f"{path}.protocol is invalid")
        if protocol not in declared:
            fail(f"{path}.protocol must reference protocol_matrix")
        if protocol in by_protocol:
            fail(f"duplicate protocol_feature_matrix protocol: {protocol}")
        features = row.get("features")
        if not isinstance(features, dict):
            fail(f"{path}.features must be an object")
        missing = sorted(REQUIRED_PROTOCOL_FEATURES - features.keys())
        if missing:
            fail(f"{path}.features is missing: {', '.join(missing)}")
        for feature, result in features.items():
            if result not in FEATURE_RESULTS:
                fail(f"{path}.features.{feature} is invalid")
        by_protocol[protocol] = features
        validate_evidence_ids(row.get("evidence_ids", []), f"{path}.evidence_ids", evidence_ids)
    missing_rows = sorted(supported - by_protocol.keys())
    if missing_rows:
        fail("protocol_feature_matrix is missing supported protocols: " + ", ".join(missing_rows))
    return by_protocol


def validate_reasoning_matrix(
    data: dict[str, Any], supported: set[str], evidence_ids: set[str]
) -> dict[str, set[str]]:
    rows = data.get("reasoning_matrix")
    if not isinstance(rows, list):
        fail("reasoning_matrix must be an array")
    seen: set[tuple[str, str]] = set()
    supported_efforts: dict[str, set[str]] = {}
    for index, row in enumerate(rows):
        path = f"reasoning_matrix[{index}]"
        if not isinstance(row, dict):
            fail(f"{path} must be an object")
        protocol = row.get("protocol")
        effort = row.get("effort")
        if protocol not in supported:
            fail(f"{path}.protocol must reference a supported protocol")
        if not isinstance(effort, str) or not effort.strip():
            fail(f"{path}.effort is required")
        key = (protocol, effort)
        if key in seen:
            fail(f"duplicate reasoning_matrix row: {protocol}/{effort}")
        seen.add(key)
        support = row.get("support")
        if support not in REASONING_SUPPORT:
            fail(f"{path}.support is invalid")
        wire_value = row.get("wire_value")
        if wire_value is not None and (not isinstance(wire_value, str) or not wire_value.strip()):
            fail(f"{path}.wire_value must be null or a non-empty string")
        if support == "supported" and wire_value is None:
            fail(f"{path}.wire_value is required for a supported effort")
        if support == "supported":
            supported_efforts.setdefault(protocol, set()).add(effort)
        validate_evidence_ids(row.get("evidence_ids", []), f"{path}.evidence_ids", evidence_ids)
    return supported_efforts


def validate_price_matrix(data: dict[str, Any], evidence_ids: set[str]) -> None:
    rows = data.get("price_matrix")
    if not isinstance(rows, list) or not rows:
        fail("price_matrix must be a non-empty array")
    scopes: set[str] = set()
    default_row: dict[str, Any] | None = None
    for index, row in enumerate(rows):
        path = f"price_matrix[{index}]"
        if not isinstance(row, dict):
            fail(f"{path} must be an object")
        scope = row.get("scope")
        if not isinstance(scope, str) or not scope.strip():
            fail(f"{path}.scope is required")
        if scope in scopes:
            fail(f"duplicate price_matrix scope: {scope}")
        scopes.add(scope)
        if scope == "default":
            default_row = row
        if not isinstance(row.get("currency"), str) or not re.fullmatch(r"[A-Z]{3}", row["currency"]):
            fail(f"{path}.currency must be a three-letter uppercase code")
        if row.get("unit") != "per_mtok":
            fail(f"{path}.unit must be per_mtok")
        require_number(row.get("input"), f"{path}.input")
        require_number(row.get("output"), f"{path}.output")
        for field in ("cached_input", "cache_write"):
            if row.get(field) is not None:
                require_number(row[field], f"{path}.{field}", allow_zero=True)
        validate_long_context(row.get("long_context"), f"{path}.long_context")
        require_date(row.get("effective_from"), f"{path}.effective_from")
        validate_evidence_ids(row.get("evidence_ids", []), f"{path}.evidence_ids", evidence_ids)
    if default_row is None:
        fail("price_matrix must contain scope=default")
    if default_row.get("currency") != "USD":
        fail("price_matrix default currency must be USD for the legacy pricing projection")

    legacy = data["pricing"]
    projections = {
        "input": legacy["input_per_mtok_usd"],
        "output": legacy["output_per_mtok_usd"],
        "cached_input": legacy.get("cached_input_per_mtok_usd"),
    }
    for field, legacy_value in projections.items():
        if default_row.get(field) != legacy_value:
            fail(f"price_matrix default {field} must match legacy pricing projection")
    if default_row.get("long_context") != legacy.get("long_context"):
        fail("price_matrix default long_context must match legacy pricing projection")


def validate_lifecycle(data: dict[str, Any]) -> None:
    lifecycle = data.get("lifecycle")
    if not isinstance(lifecycle, dict):
        fail("lifecycle must be an object")
    status = lifecycle.get("public_status")
    if status not in PUBLIC_STATUSES:
        fail("lifecycle.public_status is invalid")
    require_date(lifecycle.get("introduced_at"), "lifecycle.introduced_at")
    require_date(lifecycle.get("deprecated_at"), "lifecycle.deprecated_at")
    replacement = lifecycle.get("replacement_model_id")
    if replacement is not None and (not isinstance(replacement, str) or not MODEL_ID.fullmatch(replacement)):
        fail("lifecycle.replacement_model_id must be null or an installer-safe identifier")
    if status == "deprecated" and lifecycle.get("deprecated_at") is None:
        fail("deprecated lifecycle requires lifecycle.deprecated_at")


def validate_v2(data: dict[str, Any]) -> None:
    missing = sorted(V2_SECTIONS - data.keys())
    if missing:
        fail("schema_version 2 is missing: " + ", ".join(missing))
    evidence_ids = validate_evidence(data)
    supported, preferred = validate_protocol_matrix(data, evidence_ids)
    declared = {row["protocol"] for row in data["protocol_matrix"]}
    features = validate_protocol_features(data, declared, supported, evidence_ids)
    supported_efforts = validate_reasoning_matrix(data, supported, evidence_ids)
    for protocol, results in features.items():
        if results["reasoning"] == "pass" and not supported_efforts.get(protocol):
            fail(f"reasoning_matrix requires a supported effort for {protocol}")
        if results["reasoning"] in {"fail", "not_applicable"} and supported_efforts.get(protocol):
            fail(f"reasoning_matrix contradicts protocol_feature_matrix for {protocol}")
    validate_price_matrix(data, evidence_ids)
    validate_lifecycle(data)
    if preferred is not None and data["capabilities"]["protocol"] != preferred:
        fail("capabilities.protocol must match the preferred protocol compatibility projection")


def validate(data: dict[str, Any]) -> None:
    walk(data)
    if data.get("schema_version") not in {1, 2}:
        fail("schema_version must be 1 or 2")
    validate_legacy_sections(data)
    if data["schema_version"] == 2:
        validate_v2(data)


def feature_template() -> dict[str, str]:
    return {name: "untested" for name in sorted(REQUIRED_PROTOCOL_FEATURES)}


def template(args: argparse.Namespace) -> dict[str, Any]:
    protocol = {
        "openai": "responses",
        "anthropic": "messages",
        "grok": "responses",
        "gemini": "generate_content",
    }[args.platform]
    pricing = {
        "input_per_mtok_usd": 1,
        "cached_input_per_mtok_usd": None,
        "output_per_mtok_usd": 1,
        "evidence_url": "https://example.invalid/replace-me",
        "long_context": None,
    }
    return {
        "schema_version": 2,
        "model": {
            "id": args.model_id,
            "upstream_id": args.model_id,
            "display_name": args.display_name,
            "platform": args.platform,
            "context_window": 1,
            "max_output_tokens": 1,
            "codex_catalog_entry": None,
        },
        "capabilities": {
            "protocol": protocol,
            "streaming": True,
            "tools": False,
            "image_input": False,
            "cache_read": False,
            "reasoning_efforts": [],
        },
        "pricing": pricing,
        "lifecycle": {
            "public_status": "draft",
            "introduced_at": None,
            "deprecated_at": None,
            "replacement_model_id": None,
        },
        "protocol_matrix": [
            {
                "protocol": protocol,
                "support": "untested",
                "recommendation": "not_applicable",
                "recommendation_reason": "Complete after Provider Contract testing.",
                "evidence_ids": [],
            }
        ],
        "protocol_feature_matrix": [
            {"protocol": protocol, "features": feature_template(), "evidence_ids": []}
        ],
        "reasoning_matrix": [],
        "price_matrix": [
            {
                "scope": "default",
                "currency": "USD",
                "unit": "per_mtok",
                "input": pricing["input_per_mtok_usd"],
                "output": pricing["output_per_mtok_usd"],
                "cached_input": pricing["cached_input_per_mtok_usd"],
                "cache_write": None,
                "long_context": pricing["long_context"],
                "effective_from": None,
                "evidence_ids": ["price-source"],
            }
        ],
        "evidence": [
            {
                "id": "price-source",
                "kind": "official_docs",
                "url": pricing["evidence_url"],
                "observed_at": "1970-01-01T00:00:00Z",
                "subject_version": "replace-me",
                "result": "pending",
                "artifact_sha256": None,
            }
        ],
        "production": {
            "account_id": None,
            "group_id": None,
            "group_name": "",
            "legacy_group_names": [],
            "group_description": "",
            "client_default": False,
            "managed_predecessor_ids": [],
            "aliases": [],
            "replace_model_id": None,
        },
        "verification": {
            "expected_text": "OK",
            "owned_e2e": True,
            "require_usage_attribution": True,
            "require_accounting_reconciliation": True,
        },
    }


def migration_gaps(data: dict[str, Any]) -> list[str]:
    if data.get("schema_version") == 1:
        return [f"missing {section} (upgrade manifest to schema_version 2)" for section in sorted(V2_SECTIONS)]
    return []


def contract_gaps(data: dict[str, Any]) -> list[str]:
    gaps = migration_gaps(data)
    if data.get("schema_version") != 2:
        return gaps
    preferred = [row for row in data["protocol_matrix"] if row["recommendation"] == "preferred"]
    if len(preferred) != 1:
        gaps.append("protocol_matrix requires exactly one supported preferred protocol before public release")
    for row in data["protocol_matrix"]:
        if row["support"] == "untested":
            gaps.append(f"protocol_matrix:{row['protocol']} is untested")
        if row["support"] != "untested" and not row["evidence_ids"]:
            gaps.append(f"protocol_matrix:{row['protocol']} lacks evidence_ids")
    for row in data["protocol_feature_matrix"]:
        for feature, result in row["features"].items():
            if result == "untested":
                gaps.append(f"protocol_feature_matrix:{row['protocol']}/{feature} is untested")
        if any(result in {"pass", "fail"} for result in row["features"].values()) and not row["evidence_ids"]:
            gaps.append(f"protocol_feature_matrix:{row['protocol']} lacks evidence_ids")
    for row in data["reasoning_matrix"]:
        if row["support"] == "untested":
            gaps.append(f"reasoning_matrix:{row['protocol']}/{row['effort']} is untested")
        if row["support"] != "untested" and not row["evidence_ids"]:
            gaps.append(f"reasoning_matrix:{row['protocol']}/{row['effort']} lacks evidence_ids")
    evidence_by_id = {row["id"]: row for row in data["evidence"]}
    for row in data["price_matrix"]:
        if not row["evidence_ids"]:
            gaps.append(f"price_matrix:{row['scope']} lacks evidence_ids")
        price_sources = [evidence_by_id[evidence_id] for evidence_id in row["evidence_ids"]]
        if not any(source["kind"] in {"official_docs", "provider_api"} for source in price_sources):
            gaps.append(f"price_matrix:{row['scope']} lacks official_docs or provider_api evidence")
    for row in data["evidence"]:
        if row["result"] == "pending":
            gaps.append(f"evidence:{row['id']} is pending")
        if row["url"] and urlparse(row["url"]).hostname == "example.invalid":
            gaps.append(f"evidence:{row['id']} still uses placeholder URL")
        if row["subject_version"] == "replace-me":
            gaps.append(f"evidence:{row['id']} still uses placeholder subject_version")
    lifecycle = data["lifecycle"]
    if lifecycle["public_status"] != "public":
        gaps.append(f"lifecycle.public_status is {lifecycle['public_status']}, not public")
    if lifecycle["public_status"] == "public" and lifecycle["introduced_at"] is None:
        gaps.append("public lifecycle requires lifecycle.introduced_at")
    return gaps


def command_init(args: argparse.Namespace) -> None:
    output = Path(args.output).expanduser().resolve()
    if output.exists() and not args.force:
        fail(f"output already exists: {output}")
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(template(args), ensure_ascii=False, indent=2) + "\n")
    print(output)


def load(path: str) -> dict[str, Any]:
    value = json.loads(Path(path).read_text())
    if not isinstance(value, dict):
        fail("manifest root must be an object")
    return value


def command_validate(args: argparse.Namespace) -> None:
    data = load(args.manifest)
    validate(data)
    gaps = contract_gaps(data)
    print(json.dumps({"valid": True, "complete": not gaps, "schema_version": data["schema_version"], "model_id": data["model"]["id"], "contract_gaps": gaps}, ensure_ascii=False))


def command_plan(args: argparse.Namespace) -> None:
    data = load(args.manifest)
    validate(data)
    model = data["model"]
    production = data["production"]
    gaps = contract_gaps(data)
    plan = {
        "model_id": model["id"],
        "schema_version": data["schema_version"],
        "classification": "catalog-only only if all model-contract conditions pass",
        "public_release_ready": not gaps,
        "migration_gaps": migration_gaps(data),
        "contract_gaps": gaps,
        "provider_contract_runner": {
            "input": "one secret-free schema v2 manifest plus runtime credentials supplied out of band",
            "output": "case results and redacted evidence metadata keyed by evidence id",
            "required_before_catalog_apply": True,
        },
        "phases": [
            "official specification and price evidence",
            "Provider Contract protocol, feature, reasoning, resilience, usage, and billing cases",
            "manifest-driven catalog generation plus bounded adapter tests",
            "PR, CI, merge, immutable deploy",
            f"account mapping dry-run for account {production.get('account_id')}",
            f"complete group update/readback for group {production.get('group_id')}",
            "owned public-gateway visibility, response, usage, accounting, and balance verification",
            "production log and changelog assessment",
        ],
    }
    print(json.dumps(plan, ensure_ascii=False, indent=2))


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    sub = root.add_subparsers(dest="command", required=True)
    init = sub.add_parser("init")
    init.add_argument("--platform", choices=sorted(PLATFORMS), required=True)
    init.add_argument("--model-id", required=True)
    init.add_argument("--display-name", required=True)
    init.add_argument("--output", required=True)
    init.add_argument("--force", action="store_true")
    init.set_defaults(func=command_init)
    validate_cmd = sub.add_parser("validate")
    validate_cmd.add_argument("manifest")
    validate_cmd.set_defaults(func=command_validate)
    plan = sub.add_parser("plan")
    plan.add_argument("manifest")
    plan.set_defaults(func=command_plan)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        args.func(args)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

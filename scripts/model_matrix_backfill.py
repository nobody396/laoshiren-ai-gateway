#!/usr/bin/env python3
"""Conservatively migrate legacy model contracts into the nine-matrix shape.

The migrator is intentionally offline and fail-closed.  It copies only facts
already present in a model contract or the checked-in client matrix.  An exact
OS, observation date, protocol feature, group readback, or price that is not in
the source remains a ``blocked`` ``migration_gap`` cell instead of being
guessed.

``plan`` never writes contracts. ``apply`` atomically writes changed contracts
and retains the first pre-migration copy in a hidden backup directory.
"""

from __future__ import annotations

import argparse
from copy import deepcopy
from datetime import date
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import sys
import tempfile
from typing import Any, Iterable
from urllib.parse import unquote, urlparse


ROOT = Path(__file__).resolve().parents[1]
MATRIX_SCRIPT = ROOT / "scripts" / "model_doc_matrix.py"
EXPECTED_MATRICES = (
    "public_model",
    "model_protocol",
    "model_reasoning",
    "client_protocol",
    "client_reasoning",
    "group_access",
    "client_config_os",
    "test_evidence",
    "model_price",
)
BASE_PROTOCOL_CHECKS = (
    "minimal_text",
    "streaming_terminal",
    "tool_call",
    "tool_result_continuation",
    "usage",
    "invalid_request",
)
PROTOCOL_FEATURES = (
    "web_search",
    "reasoning",
    "prompt_cache",
    "image_input",
    "error_passthrough",
    "stream_disconnect",
    "timeout",
    "retry",
    "billing",
)
FINAL_STATUSES = {"verified", "unsupported"}
ISO_DATE_RE = re.compile(r"(?<!\d)(\d{4}-\d{2}-\d{2})(?!\d)")


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n"


def sha256_json(value: Any) -> str:
    payload = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def atomic_write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def load_matrix_module() -> Any:
    spec = importlib.util.spec_from_file_location("model_doc_matrix_for_backfill", MATRIX_SCRIPT)
    if spec is None or spec.loader is None:
        raise ValueError(f"cannot load matrix auditor: {MATRIX_SCRIPT}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def validate_nine_matrix_schema(path: Path) -> dict[str, Any]:
    schema = load_json(path)
    matrices = (
        schema.get("properties", {})
        .get("matrices", {})
        .get("required")
    )
    if tuple(matrices or ()) != EXPECTED_MATRICES:
        raise ValueError(
            "matrix schema must declare exactly the canonical nine matrices in order"
        )
    return schema


def local_inventory_path(inventory_json: Path | None, pricing_url: str | None) -> Path:
    if inventory_json is not None:
        return inventory_json
    assert pricing_url is not None
    parsed = urlparse(pricing_url)
    if parsed.scheme in {"http", "https"}:
        raise ValueError(
            "network pricing URLs are disabled; export the response to JSON and use --inventory-json"
        )
    if parsed.scheme == "file":
        if parsed.netloc not in {"", "localhost"}:
            raise ValueError("only local file:// pricing URLs are allowed")
        return Path(unquote(parsed.path))
    if parsed.scheme:
        raise ValueError(f"unsupported pricing URL scheme: {parsed.scheme}")
    return Path(pricing_url)


def contracts_from_directory(directory: Path, client_matrix_path: Path) -> list[tuple[Path, dict[str, Any]]]:
    result: list[tuple[Path, dict[str, Any]]] = []
    for path in sorted(directory.glob("*.json")):
        if path.resolve() == client_matrix_path.resolve() or path.name in {"matrix-schema.json", "import-provenance.json"}:
            continue
        raw = load_json(path)
        model_id = raw.get("model", {}).get("id") if isinstance(raw, dict) else None
        if isinstance(model_id, str) and model_id:
            result.append((path, raw))
    return result


def client_protocols(client: dict[str, Any]) -> set[str]:
    legacy = client.get("protocols")
    if isinstance(legacy, list):
        return {value for value in legacy if isinstance(value, str)}
    return {
        row["protocol"]
        for row in client.get("client_protocol", {}).get("protocols", [])
        if isinstance(row, dict)
        and row.get("support") == "supported"
        and isinstance(row.get("protocol"), str)
    }


def client_version_key(client: dict[str, Any]) -> str | None:
    value = client.get("version")
    if isinstance(value, str):
        return value
    value = client.get("client_config_os", {}).get("release", {}).get("version_key")
    return value if isinstance(value, str) else None


def client_display_version(client: dict[str, Any]) -> str | None:
    release = client.get("client_config_os", {}).get("release", {})
    value = release.get("display") if isinstance(release, dict) else None
    return value if isinstance(value, str) else None


def expected_oses(client: dict[str, Any]) -> list[str]:
    # verification_os is itself an explicit client-matrix fact. It determines
    # which cells are required, but never turns an OS-less legacy result green.
    rows = client.get("verification_os")
    if isinstance(rows, list) and rows:
        return sorted({value for value in rows if isinstance(value, str)})
    rows = client.get("client_config_os", {}).get("os_support", [])
    return sorted({
        row["os"] for row in rows
        if isinstance(row, dict)
        and row.get("support") != "unsupported"
        and isinstance(row.get("os"), str)
    })


def exact_version_from_legacy(legacy: Any, client: dict[str, Any]) -> tuple[str | None, bool]:
    if not isinstance(legacy, str) or not legacy.strip():
        return client_version_key(client), False
    raw = legacy.strip()
    if any(marker in raw for marker in ("+", "*", ">", "<", "~", "^")):
        return f"legacy:{raw}", False
    version_key = client_version_key(client)
    display = client_display_version(client)
    if raw == version_key or (display is not None and raw == display):
        return version_key, True
    release = client.get("client_config_os", {}).get("release", {})
    components = release.get("components", []) if isinstance(release, dict) else []
    required = [
        str(row.get("version")) for row in components
        if isinstance(row, dict) and row.get("version") is not None
    ]
    if required and all(version in raw for version in required):
        return version_key, True
    return f"legacy:{raw}", False


def literal_date(*values: Any) -> str | None:
    for value in values:
        if not isinstance(value, str):
            continue
        match = ISO_DATE_RE.search(value)
        if match:
            try:
                return date.fromisoformat(match.group(1)).isoformat()
            except ValueError:
                continue
    return None


def direct_cell(status: str, evidence: str, **fields: Any) -> dict[str, Any]:
    return {"status": status, "evidence": evidence, **fields}


def gap_cell(reason: str, **fields: Any) -> dict[str, Any]:
    return {
        "status": "blocked",
        "evidence": f"migration_gap: {reason}",
        **fields,
    }


def add_if_missing(
    container: dict[str, Any], key: str, value: Any, events: list[dict[str, Any]],
    *, model_id: str, path: str, source: str,
) -> Any:
    if key not in container:
        container[key] = value
        status = value.get("status") if isinstance(value, dict) else None
        events.append({
            "model_id": model_id,
            "path": f"{path}.{key}" if path else key,
            "status": status or "structural",
            "source": source,
        })
    return container[key]


def add_or_upgrade_migration_gap(
    container: dict[str, Any], key: str, value: Any, events: list[dict[str, Any]],
    *, model_id: str, path: str, source: str,
) -> Any:
    existing = container.get(key)
    is_gap = (
        isinstance(existing, dict)
        and existing.get("status") == "blocked"
        and isinstance(existing.get("evidence"), str)
        and existing["evidence"].startswith("migration_gap:")
    )
    if key not in container or is_gap:
        container[key] = deepcopy(value)
        events.append({
            "model_id": model_id,
            "path": f"{path}.{key}",
            "status": value.get("status", "structural") if isinstance(value, dict) else "structural",
            "source": source,
        })
    return container[key]


def evidence_says(evidence: str, check: str) -> bool:
    text = evidence.casefold()
    patterns: dict[str, tuple[str, ...]] = {
        "minimal_text": ("minimal text", "non-stream text", "ordinary text", "production text", "text and streaming"),
        "streaming_terminal": ("sse terminal", "terminal streaming", "streaming terminal", "terminal completion", "streamed messages", "streaming calls completed"),
        "tool_call": ("tool call", "function call", "function calling", "agent loop", "agent-loop", "tool-result loop", "tool-result agent"),
        "tool_result_continuation": ("tool-result continuation", "tool result continuation", "tool-result loop", "agent loop", "agent-loop", "tool-result agent"),
        "usage": ("usage", "billing attribution"),
        "invalid_request": ("invalid_request", "invalid-request", "malformed", "error shape"),
        "web_search": ("web_search", "web search"),
        "reasoning": ("reasoning", "adaptive-thinking", "thinking"),
        "prompt_cache": ("prompt cache", "cache hit", "cached input"),
        "image_input": ("image input", "image returned successfully"),
        "error_passthrough": ("structured http 400", "invalid_request", "invalid-request", "error shape"),
        "stream_disconnect": ("disconnect classification", "interrupted stream classification"),
        "timeout": ("timeout classification", "timed out and classified"),
        "retry": ("retry succeeded", "retry policy passed"),
        "billing": ("actual billing", "persisted billing", "billing attribution"),
    }
    return any(pattern in text for pattern in patterns[check])


def verified_protocol_rows(contract: dict[str, Any]) -> dict[str, dict[str, Any]]:
    return {
        row["name"]: row
        for row in contract.get("protocols", [])
        if isinstance(row, dict)
        and row.get("status") == "verified"
        and isinstance(row.get("name"), str)
        and isinstance(row.get("evidence"), str)
    }


def legacy_client_rows(contract: dict[str, Any]) -> dict[tuple[str, str], dict[str, Any]]:
    rows: dict[tuple[str, str], dict[str, Any]] = {}
    for row in contract.get("clients", []):
        if not isinstance(row, dict):
            continue
        name, protocol = row.get("name"), row.get("protocol")
        if isinstance(name, str) and isinstance(protocol, str):
            rows[(name, protocol)] = row
    return rows


def coverage_rows(contract: dict[str, Any]) -> dict[str, dict[str, Any]]:
    return {
        row["name"]: row
        for row in contract.get("client_coverage", [])
        if isinstance(row, dict) and isinstance(row.get("name"), str)
    }


def compatible_clients(
    verified_protocol_names: Iterable[str], client_matrix: dict[str, Any]
) -> dict[str, dict[str, Any]]:
    protocols = set(verified_protocol_names)
    return {
        client["name"]: client
        for client in client_matrix.get("clients", [])
        if isinstance(client, dict)
        and isinstance(client.get("name"), str)
        and protocols.intersection(client_protocols(client))
    }


def migrate_contract(
    original: dict[str, Any], client_matrix: dict[str, Any],
    inventory_model: dict[str, Any] | None = None, *, currency: str | None = None,
    unit: str | None = None, as_of: date | None = None,
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    contract = deepcopy(original)
    model = contract.get("model") if isinstance(contract.get("model"), dict) else {}
    model_id = model.get("id") if isinstance(model.get("id"), str) else "<unknown>"
    events: list[dict[str, Any]] = []
    matrix = contract.setdefault("test_matrix", {})
    if not isinstance(matrix, dict):
        # Refuse to destroy malformed source data.
        return contract, events

    limits = add_if_missing(matrix, "limits", {}, events, model_id=model_id,
                            path="test_matrix", source="legacy:model")
    if isinstance(limits, dict):
        official = contract.get("verification", {}).get("official_spec_url")
        limit_source = contract.get("verification", {}).get("limits_source")
        for field in ("context_window", "max_output_tokens"):
            if field in model and isinstance(official, str) and limit_source in {"official", "live"}:
                value = direct_cell(
                    "verified",
                    f"legacy contract {limit_source} limit: {official}",
                )
                source = f"legacy:model.{field}+verification"
            else:
                value = gap_cell(f"{field} lacks direct official/live source evidence")
                source = "migration_gap"
            add_if_missing(limits, field, value, events, model_id=model_id,
                           path="test_matrix.limits", source=source)

    modalities = add_if_missing(matrix, "modalities", {}, events, model_id=model_id,
                                path="test_matrix", source="legacy:verification.modalities")
    declared_modalities = contract.get("verification", {}).get("modalities", {})
    if isinstance(modalities, dict):
        for name in ("text", "image", "video"):
            status = declared_modalities.get(name) if isinstance(declared_modalities, dict) else None
            if status in FINAL_STATUSES:
                evidence = contract.get("verification", {}).get("official_spec_url")
                value = direct_cell(status, f"legacy modality declaration: {evidence or model_id}")
                source = "legacy:verification.modalities"
            else:
                value = gap_cell(f"{name} modality has no direct legacy result")
                source = "migration_gap"
            add_if_missing(modalities, name, value, events, model_id=model_id,
                           path="test_matrix.modalities", source=source)

    verified_protocols = verified_protocol_rows(contract)
    protocol_matrix = add_if_missing(matrix, "protocols", {}, events, model_id=model_id,
                                     path="test_matrix", source="legacy:protocols")
    if isinstance(protocol_matrix, dict):
        for protocol, row in sorted(verified_protocols.items()):
            checks = add_if_missing(protocol_matrix, protocol, {}, events, model_id=model_id,
                                    path="test_matrix.protocols", source="legacy:protocols")
            if not isinstance(checks, dict):
                continue
            evidence = row["evidence"]
            for check in BASE_PROTOCOL_CHECKS:
                if evidence_says(evidence, check):
                    value = direct_cell("verified", evidence)
                    source = f"legacy:protocols.{protocol}.evidence"
                else:
                    value = gap_cell(f"{protocol}.{check} is not explicit in legacy evidence")
                    source = "migration_gap"
                add_if_missing(checks, check, value, events, model_id=model_id,
                               path=f"test_matrix.protocols.{protocol}", source=source)

    features = add_if_missing(matrix, "protocol_features", {}, events, model_id=model_id,
                              path="test_matrix", source="legacy:protocols")
    if isinstance(features, dict):
        for protocol, row in sorted(verified_protocols.items()):
            feature_rows = add_if_missing(features, protocol, {}, events, model_id=model_id,
                                          path="test_matrix.protocol_features", source="legacy:protocols")
            if not isinstance(feature_rows, dict):
                continue
            evidence = row["evidence"]
            for feature in PROTOCOL_FEATURES:
                if evidence_says(evidence, feature):
                    value = direct_cell("verified", evidence)
                    source = f"legacy:protocols.{protocol}.evidence"
                else:
                    value = gap_cell(
                        f"{protocol}.{feature} has no direct protocol-feature result"
                    )
                    source = "migration_gap"
                add_if_missing(feature_rows, feature, value, events, model_id=model_id,
                               path=f"test_matrix.protocol_features.{protocol}", source=source)

    reasoning = add_if_missing(matrix, "reasoning", {}, events, model_id=model_id,
                               path="test_matrix", source="legacy:reasoning")
    if isinstance(reasoning, dict):
        model_results = add_if_missing(reasoning, "model_levels", {}, events, model_id=model_id,
                                       path="test_matrix.reasoning", source="legacy:reasoning.model_levels")
        legacy_reasoning = contract.get("reasoning") if isinstance(contract.get("reasoning"), dict) else {}
        levels = legacy_reasoning.get("model_levels") if isinstance(legacy_reasoning, dict) else None
        if isinstance(model_results, dict):
            if isinstance(levels, list):
                for level in levels:
                    if not isinstance(level, str) or not level:
                        continue
                    add_if_missing(
                        model_results,
                        level,
                        direct_cell("verified", "legacy explicit reasoning.model_levels declaration"),
                        events,
                        model_id=model_id,
                        path="test_matrix.reasoning.model_levels",
                        source="legacy:reasoning.model_levels",
                    )

        per_client = add_if_missing(reasoning, "clients", {}, events, model_id=model_id,
                                    path="test_matrix.reasoning", source="legacy:reasoning+client-matrix")
        explicit_levels = {
            row.get("client"): row.get("levels")
            for row in legacy_reasoning.get("client_levels", [])
            if isinstance(row, dict) and isinstance(row.get("client"), str)
            and isinstance(row.get("levels"), list)
        } if isinstance(legacy_reasoning, dict) else {}
        explicit_mappings = legacy_reasoning.get("client_mappings", []) if isinstance(legacy_reasoning, dict) else []
        if isinstance(per_client, dict):
            for name, client in sorted(compatible_clients(verified_protocols, client_matrix).items()):
                client_rows = add_if_missing(per_client, name, {}, events, model_id=model_id,
                                             path="test_matrix.reasoning.clients", source="client-matrix")
                if not isinstance(client_rows, dict):
                    continue
                control = client.get("client_reasoning", {}).get("control_kind")
                supported = sorted(set(verified_protocols).intersection(client_protocols(client)))
                for protocol in supported:
                    if control == "model_defined":
                        value = {
                            "status": "not_exposed",
                            "client_levels": [],
                            "mappings": [],
                            "evidence": "client-matrix explicitly declares model_defined reasoning controls",
                        }
                        source = "client-matrix:client_reasoning.control_kind"
                    elif name in explicit_levels:
                        mappings = [
                            deepcopy(mapping) for mapping in explicit_mappings
                            if isinstance(mapping, dict)
                            and mapping.get("client") == name
                            and mapping.get("protocol") == protocol
                        ]
                        value = {
                            "status": "verified",
                            "client_levels": deepcopy(explicit_levels[name]),
                            "mappings": mappings,
                            "evidence": "legacy explicit reasoning.client_levels and protocol-keyed mappings",
                        }
                        source = "legacy:reasoning.client_levels"
                    else:
                        client_values = client.get("client_reasoning", {}).get("level_control", {}).get("values", [])
                        value = gap_cell(
                            f"{name}/{protocol} fixed reasoning lacks model-specific mapping evidence",
                            client_levels=deepcopy(client_values) if isinstance(client_values, list) else [],
                            mappings=[],
                        )
                        source = "migration_gap"
                    add_if_missing(client_rows, protocol, value, events, model_id=model_id,
                                   path=f"test_matrix.reasoning.clients.{name}", source=source)

    compatible = compatible_clients(verified_protocols, client_matrix)
    coverage_list = contract.setdefault("client_coverage", [])
    if isinstance(coverage_list, list):
        existing_coverage = {
            row.get("name") for row in coverage_list
            if isinstance(row, dict) and isinstance(row.get("name"), str)
        }
        for name in sorted(compatible):
            if name in existing_coverage:
                continue
            coverage_list.append({
                "name": name,
                "status": "blocked",
                "protocols": [],
                "evidence": "migration_gap: protocol-compatible client lacks an exact model/version/OS Agent receipt",
            })
            existing_coverage.add(name)
            events.append({
                "model_id": model_id,
                "path": f"client_coverage.{name}",
                "source": "migration_gap",
                "status": "blocked",
            })

    clients_matrix = add_if_missing(matrix, "clients", {}, events, model_id=model_id,
                                    path="test_matrix", source="legacy:clients")
    legacy_clients = legacy_client_rows(contract)
    legacy_coverage = coverage_rows(contract)
    if isinstance(clients_matrix, dict):
        for name, client in sorted(compatible.items()):
            by_protocol = add_if_missing(clients_matrix, name, {}, events, model_id=model_id,
                                         path="test_matrix.clients", source="legacy:clients")
            if not isinstance(by_protocol, dict):
                continue
            for protocol in sorted(set(verified_protocols).intersection(client_protocols(client))):
                os_rows = add_if_missing(by_protocol, protocol, {}, events, model_id=model_id,
                                         path=f"test_matrix.clients.{name}", source="legacy:clients")
                if not isinstance(os_rows, dict):
                    continue
                legacy = legacy_clients.get((name, protocol))
                coverage = legacy_coverage.get(name)
                evidence = None
                version = None
                version_exact = False
                source_status = None
                if legacy is not None:
                    evidence = legacy.get("evidence")
                    version, version_exact = exact_version_from_legacy(legacy.get("version"), client)
                    source_status = legacy.get("status")
                elif coverage is not None and protocol in (coverage.get("protocols") or []):
                    evidence = coverage.get("evidence")
                    version = client_version_key(client)
                    source_status = coverage.get("status")
                for os_name in expected_oses(client):
                    reason = f"legacy client result does not state exact OS ({os_name} required)"
                    if legacy is None and coverage is None:
                        reason = f"no legacy client result for {name}/{protocol}/{os_name}"
                    elif not version_exact and legacy is not None:
                        reason += "; exact client version is also missing"
                    cell = gap_cell(
                        reason,
                        model_id=model_id,
                        protocol=protocol,
                        client_version=version or client_version_key(client),
                        os=os_name,
                    )
                    if isinstance(evidence, str) and evidence:
                        cell["evidence"] += f"; legacy_evidence: {evidence}"
                    observed = literal_date(evidence)
                    if observed is not None:
                        cell["verified_at"] = observed
                    if source_status in {"unsupported", "blocked"}:
                        cell["legacy_status"] = source_status
                    add_if_missing(os_rows, os_name, cell, events, model_id=model_id,
                                   path=f"test_matrix.clients.{name}.{protocol}", source="migration_gap")

    groups = add_if_missing(matrix, "group_access", {}, events, model_id=model_id,
                            path="test_matrix", source="legacy:access")
    access = contract.get("access") if isinstance(contract.get("access"), dict) else {}
    recommended = contract.get("recommended_protocol")
    recommended_reason = contract.get("recommended_protocol_reason")
    if isinstance(groups, dict):
        for group in access.get("groups", []) if isinstance(access, dict) else []:
            if not isinstance(group, dict) or not isinstance(group.get("name"), str):
                continue
            name = group["name"]
            cell = gap_cell(
                f"{name} lacks dated group-key route/model readback evidence",
                model_id=model_id,
                base_url=access.get("base_url"),
                multiplier=group.get("multiplier"),
                protocols=sorted(verified_protocols),
                recommended_protocol=recommended,
                recommended_protocol_reason=(
                    recommended_reason if isinstance(recommended_reason, str) and recommended_reason
                    else "migration_gap: recommended_protocol_reason is missing"
                ),
            )
            inventory_group = (inventory_model or {}).get("groups", {}).get(name)
            if isinstance(inventory_group, dict):
                cell.update({
                    "group_id": inventory_group.get("group_id"),
                    "platform": inventory_group.get("platform"),
                    "subscription_type": inventory_group.get("subscription_type"),
                })
            add_if_missing(groups, name, cell, events, model_id=model_id,
                           path="test_matrix.group_access", source="legacy:access+migration_gap")

    pricing = add_if_missing(matrix, "pricing", {}, events, model_id=model_id,
                             path="test_matrix", source="migration_gap")
    if isinstance(pricing, dict):
        for group in access.get("groups", []) if isinstance(access, dict) else []:
            if not isinstance(group, dict) or not isinstance(group.get("name"), str):
                continue
            name = group["name"]
            inventory_group = (inventory_model or {}).get("groups", {}).get(name)
            price = inventory_group.get("price") if isinstance(inventory_group, dict) else None
            if isinstance(price, dict) and as_of is not None:
                value = direct_cell("verified", f"{as_of.isoformat()} exported public pricing inventory readback")
                value.update({
                    "verified_at": as_of.isoformat(),
                    "currency": currency,
                    "unit": unit,
                    **price,
                })
                source = "inventory:public_pricing"
            else:
                value = gap_cell(f"{name} has no direct price receipt in the legacy contract")
                source = "migration_gap"
            add_or_upgrade_migration_gap(
                pricing, name, value, events, model_id=model_id,
                path="test_matrix.pricing", source=source,
            )

    return contract, events


def walk_status_cells(value: Any, path: str = "") -> Iterable[tuple[str, dict[str, Any]]]:
    if isinstance(value, dict):
        if isinstance(value.get("status"), str) and isinstance(value.get("evidence"), str):
            yield path, value
        for key in sorted(value):
            child_path = f"{path}.{key}" if path else key
            yield from walk_status_cells(value[key], child_path)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from walk_status_cells(child, f"{path}[{index}]")


def status_summary(contracts: Iterable[dict[str, Any]]) -> dict[str, Any]:
    counts: dict[str, int] = {}
    migration_gaps = 0
    with_matrix = 0
    total = 0
    for contract in contracts:
        total += 1
        matrix = contract.get("test_matrix")
        if isinstance(matrix, dict):
            with_matrix += 1
            for _, cell in walk_status_cells(matrix):
                status = cell["status"]
                counts[status] = counts.get(status, 0) + 1
                if cell["evidence"].startswith("migration_gap:"):
                    migration_gaps += 1
    return {
        "contracts": total,
        "with_test_matrix": with_matrix,
        "status_cells": {key: counts[key] for key in sorted(counts)},
        "migration_gap_cells": migration_gaps,
    }


def audit_snapshot(
    contracts: Iterable[dict[str, Any]], client_matrix: dict[str, Any],
    inventory: dict[str, Any], matrix_module: Any, as_of: date,
) -> dict[str, Any]:
    by_id = {
        contract.get("model", {}).get("id"): contract
        for contract in contracts
        if isinstance(contract.get("model", {}).get("id"), str)
    }
    normalized = matrix_module.normalize_inventory(inventory)
    sections = {name: [] for name in EXPECTED_MATRICES}
    client_sections = matrix_module.audit_client_matrix(client_matrix)
    for name in EXPECTED_MATRICES:
        sections[name].extend(client_sections.get(name, []))
    public = set(normalized["models"])
    missing = sorted(public - set(by_id))
    if missing:
        sections["public_model"].append("missing model contracts: " + ", ".join(missing))
    for model_id in sorted(public.intersection(by_id)):
        result = matrix_module.audit_contract_sections(
            by_id[model_id],
            client_matrix,
            normalized["models"].get(model_id),
            currency=normalized.get("currency"),
            unit=normalized.get("unit"),
            as_of=as_of,
        )
        for name in EXPECTED_MATRICES:
            sections[name].extend(result.get(name, []))
    failures = [failure for name in EXPECTED_MATRICES for failure in sections[name]]
    return {
        "audit_failures": len(failures),
        "public_models": len(public),
        "missing_contracts": missing,
        "by_matrix": {
            name: {
                "remaining": len(sections[name]),
                "failures": sections[name],
            }
            for name in EXPECTED_MATRICES
        },
    }


def all_source_dates(*values: Any) -> Iterable[date]:
    for value in values:
        if isinstance(value, dict):
            for key, child in value.items():
                if key in {"verified_at", "observed_at", "updated_at"} and isinstance(child, str):
                    try:
                        yield date.fromisoformat(child[:10])
                    except ValueError:
                        pass
                yield from all_source_dates(child)
        elif isinstance(value, list):
            for child in value:
                yield from all_source_dates(child)


def build_report(
    before_contracts: list[dict[str, Any]], after_contracts: list[dict[str, Any]],
    client_matrix: dict[str, Any], inventory: dict[str, Any], schema: dict[str, Any],
    events: list[dict[str, Any]], matrix_module: Any, as_of: date, command: str,
) -> dict[str, Any]:
    before = status_summary(before_contracts)
    after = status_summary(after_contracts)
    before_audit = audit_snapshot(before_contracts, client_matrix, inventory, matrix_module, as_of)
    after_audit = audit_snapshot(after_contracts, client_matrix, inventory, matrix_module, as_of)
    before.update({
        "audit_failures": before_audit["audit_failures"],
        "public_models": before_audit["public_models"],
        "missing_contracts": before_audit["missing_contracts"],
    })
    after.update({
        "audit_failures": after_audit["audit_failures"],
        "public_models": after_audit["public_models"],
        "missing_contracts": after_audit["missing_contracts"],
    })
    event_counts: dict[str, int] = {}
    for event in events:
        event_counts[event["status"]] = event_counts.get(event["status"], 0) + 1
    changed_models = sorted({event["model_id"] for event in events})
    return {
        "kind": "model_matrix_backfill",
        "schema_version": 1,
        "command": command,
        "network_execution": "disabled",
        "as_of": as_of.isoformat(),
        "input_fingerprints": {
            "matrix_schema": sha256_json(schema),
            "client_matrix": sha256_json(client_matrix),
            "inventory": sha256_json(inventory),
        },
        "before": before,
        "after": after,
        "backfilled": {
            "contracts_changed": len(changed_models),
            "cells_added": len(events),
            "status_counts": {key: event_counts[key] for key in sorted(event_counts)},
            "models": changed_models,
            "events": sorted(events, key=lambda item: (item["model_id"], item["path"])),
        },
        "remaining": {
            "audit_failures": after_audit["audit_failures"],
            "migration_gap_cells": after["migration_gap_cells"],
            "missing_contracts": after_audit["missing_contracts"],
            "by_matrix": after_audit["by_matrix"],
        },
        "complete": after_audit["audit_failures"] == 0,
    }


def backup_and_write(path: Path, contract: dict[str, Any], contracts_dir: Path) -> None:
    backup_dir = contracts_dir / ".model-matrix-backfill-backups"
    backup = backup_dir / f"{path.name}.bak"
    if not backup.exists():
        backup_dir.mkdir(parents=True, exist_ok=True)
        fd, temporary = tempfile.mkstemp(prefix=f".{backup.name}.", suffix=".tmp", dir=backup_dir)
        os.close(fd)
        try:
            shutil.copy2(path, temporary)
            os.replace(temporary, backup)
        except BaseException:
            try:
                os.unlink(temporary)
            except FileNotFoundError:
                pass
            raise
    atomic_write(path, canonical_json(contract))


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("plan", "apply"))
    parser.add_argument("--contracts", type=Path, required=True)
    parser.add_argument("--client-matrix", type=Path, required=True)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--inventory-json", type=Path)
    source.add_argument(
        "--pricing-url",
        help="offline compatibility option: only file:// or a local path is accepted",
    )
    parser.add_argument("--report", type=Path)
    parser.add_argument("--as-of", help="deterministic ISO date; defaults to the latest input date")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        schema_path = args.contracts / "matrix-schema.json"
        schema = validate_nine_matrix_schema(schema_path)
        client_matrix = load_json(args.client_matrix)
        inventory_path = local_inventory_path(args.inventory_json, args.pricing_url)
        inventory = load_json(inventory_path)
        contract_pairs = contracts_from_directory(args.contracts, args.client_matrix)
        before_contracts = [raw for _, raw in contract_pairs]
        if args.as_of:
            as_of = date.fromisoformat(args.as_of)
        else:
            dates = list(all_source_dates(client_matrix, inventory, before_contracts))
            as_of = max(dates) if dates else date(1970, 1, 1)
        matrix_module = load_matrix_module()
        normalized_inventory = matrix_module.normalize_inventory(inventory)
        migrated: list[tuple[Path, dict[str, Any]]] = []
        events: list[dict[str, Any]] = []
        for path, contract in contract_pairs:
            model_id = contract.get("model", {}).get("id")
            inventory_model = normalized_inventory.get("models", {}).get(model_id)
            after, contract_events = migrate_contract(
                contract,
                client_matrix,
                inventory_model,
                currency=normalized_inventory.get("currency"),
                unit=normalized_inventory.get("unit"),
                as_of=as_of,
            )
            migrated.append((path, after))
            events.extend(contract_events)
        after_contracts = [raw for _, raw in migrated]
        report = build_report(
            before_contracts,
            after_contracts,
            client_matrix,
            inventory,
            schema,
            events,
            matrix_module,
            as_of,
            args.command,
        )
        if args.command == "apply":
            before_by_path = {path: raw for path, raw in contract_pairs}
            for path, contract in migrated:
                if contract != before_by_path[path]:
                    backup_and_write(path, contract, args.contracts)
        rendered = canonical_json(report)
        if args.report:
            atomic_write(args.report, rendered)
        sys.stdout.write(rendered)
        return 0
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        failure = {
            "kind": "model_matrix_backfill",
            "schema_version": 1,
            "complete": False,
            "error": str(exc),
        }
        rendered = canonical_json(failure)
        if getattr(args, "report", None):
            try:
                atomic_write(args.report, rendered)
            except OSError:
                pass
        sys.stdout.write(rendered)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

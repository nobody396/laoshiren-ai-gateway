#!/usr/bin/env python3
"""Audit the nine source-of-truth matrices for public model releases.

``plan`` prints migration and stale-evidence gaps without blocking work in
progress. ``audit`` turns the same gaps into a release failure.
"""

from __future__ import annotations

import argparse
from datetime import date, datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sys
import urllib.request
from zoneinfo import ZoneInfo
from typing import Any


MATRIX_NAMES = (
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
REQUIRED_LIMITS = {"context_window", "max_output_tokens"}
REQUIRED_MODALITIES = {"text", "image", "video"}
REQUIRED_PROTOCOL_CHECKS = {
    "minimal_text", "streaming_terminal", "tool_call",
    "tool_result_continuation", "usage", "invalid_request",
}
# A verified protocol may legitimately have any feature marked unsupported.
REQUIRED_PROTOCOL_FEATURES = {
    "web_search", "reasoning", "image_input", "billing",
}
PRICE_FIELDS = {"input_price", "output_price", "cache_write_price", "cache_read_price"}
OPTIONAL_PRICE_FIELDS = {"long_context", "context_intervals", "time_pricing"}
FINAL_STATUSES = {"verified", "unsupported"}
SPEC_TERMINAL_STATUSES = FINAL_STATUSES | {"not_published", "not_applicable", "not_exposed"}
PRICE_TERMINAL_STATUSES = {
    "verified", "not_published", "not_exposed", "not_applicable", "disabled",
}
BEIJING_TZ = ZoneInfo("Asia/Shanghai")


def beijing_today(now: datetime | None = None) -> date:
    current = now or datetime.now(timezone.utc)
    if current.tzinfo is None:
        current = current.replace(tzinfo=timezone.utc)
    return current.astimezone(BEIJING_TZ).date()
REASONING_STATUSES = {"verified", "unsupported", "not_exposed"}
DEFAULT_MAX_EVIDENCE_AGE_DAYS = 180
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
ROOT = Path(__file__).resolve().parents[1]


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def _unwrap_inventory(raw: Any) -> Any:
    while (
        isinstance(raw, dict) and "data" in raw
        and "groups" not in raw and "models" not in raw
    ):
        raw = raw["data"]
    return raw


def normalize_inventory(raw: Any) -> dict[str, Any]:
    """Keep public identity, group access, and customer price together."""
    raw = _unwrap_inventory(raw)
    result: dict[str, Any] = {
        "currency": raw.get("currency") if isinstance(raw, dict) else None,
        "unit": raw.get("unit") if isinstance(raw, dict) else None,
        "updated_at": raw.get("updated_at") if isinstance(raw, dict) else None,
        "models": {},
    }
    models: dict[str, Any] = result["models"]
    if isinstance(raw, dict) and isinstance(raw.get("groups"), list):
        for group in raw["groups"]:
            if not isinstance(group, dict) or not isinstance(group.get("name"), str):
                continue
            group_name = group["name"]
            for row in group.get("models") or []:
                if not isinstance(row, dict) or row.get("disabled"):
                    continue
                model_id = row.get("model", row.get("id"))
                if not isinstance(model_id, str) or not model_id.strip():
                    continue
                entry = models.setdefault(model_id, {"groups": {}})
                entry["groups"][group_name] = {
                    "group_id": group.get("group_id"),
                    "platform": group.get("platform"),
                    "rate_multiplier": group.get("rate_multiplier"),
                    "subscription_type": group.get("subscription_type"),
                    "price": {
                        key: row.get(key)
                        for key in sorted(PRICE_FIELDS | OPTIONAL_PRICE_FIELDS)
                        if key in row or key in PRICE_FIELDS
                    },
                }
        return result
    if isinstance(raw, dict) and isinstance(raw.get("models"), list):
        raw = raw["models"]
    if isinstance(raw, list):
        for row in raw:
            model_id = row.get("id", row.get("model")) if isinstance(row, dict) else row
            if not isinstance(model_id, str):
                continue
            entry = {"groups": {}}
            if isinstance(row, dict) and isinstance(row.get("pricing"), dict):
                entry["catalog_pricing"] = row["pricing"]
            models[model_id] = entry
        return result
    raise ValueError("unsupported inventory shape")


def load_inventory_catalog(path: Path | None, url: str | None) -> dict[str, Any]:
    if path:
        raw = load_json(path)
    elif url:
        with urllib.request.urlopen(url, timeout=60) as response:
            raw = json.loads(response.read())
    else:
        raise ValueError("inventory source is required")
    return normalize_inventory(raw)


def load_inventory(path: Path | None, url: str | None) -> list[str]:
    """Backward-compatible model-id loader."""
    return sorted(load_inventory_catalog(path, url)["models"])


def _new_sections() -> dict[str, list[str]]:
    return {name: [] for name in MATRIX_NAMES}


def _add(sections: dict[str, list[str]], matrix: str, message: str) -> None:
    sections[matrix].append(message)


def _nonempty(value: Any) -> bool:
    return isinstance(value, str) and bool(value.strip())


def _parse_date(value: Any) -> date | None:
    if not isinstance(value, str):
        return None
    try:
        return date.fromisoformat(value[:10])
    except ValueError:
        return None


def client_protocols(client: dict[str, Any]) -> list[str]:
    """Return supported native protocols from client-matrix v1 or v2."""
    legacy = client.get("protocols")
    if isinstance(legacy, list):
        return [value for value in legacy if isinstance(value, str)]
    rows = client.get("client_protocol", {}).get("protocols", [])
    return [
        row["protocol"] for row in rows
        if isinstance(row, dict) and row.get("support") == "supported"
        and row.get("evidence", {}).get("status") == "verified"
        and isinstance(row.get("protocol"), str)
    ]


def client_version_key(client: dict[str, Any]) -> str | None:
    value = client.get("version")
    if isinstance(value, str):
        return value
    value = client.get("client_config_os", {}).get("release", {}).get("version_key")
    return value if isinstance(value, str) else None


def client_os_names(client: dict[str, Any]) -> list[str]:
    verification_os = client.get("verification_os")
    if isinstance(verification_os, list):
        return [name for name in verification_os if isinstance(name, str)]
    files = client.get("files")
    if isinstance(files, dict):
        return [name for name in files if isinstance(name, str)]
    rows = client.get("client_config_os", {}).get("os_support", [])
    return [
        row["os"] for row in rows
        if isinstance(row, dict) and row.get("support") != "unsupported"
        and row.get("evidence", {}).get("status") == "verified"
        and isinstance(row.get("os"), str)
    ]


def client_reasoning_contract(client: dict[str, Any]) -> dict[str, Any]:
    legacy = client.get("reasoning")
    if isinstance(legacy, dict):
        return legacy
    current = client.get("client_reasoning")
    if not isinstance(current, dict):
        return {}
    return {
        "mode": current.get("control_kind"),
        "levels": current.get("level_control", {}).get("values", []),
        "notes": current.get("notes"),
    }


def _contract_view(value: Any) -> Any:
    if isinstance(value, dict):
        return {key: _contract_view(child) for key, child in value.items() if key != "evidence"}
    if isinstance(value, list):
        return [_contract_view(child) for child in value]
    return value


def _fingerprint(value: Any) -> str:
    payload = json.dumps(_contract_view(value), ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def client_dimension_fingerprints(client: dict[str, Any]) -> dict[str, str]:
    return {
        name: _fingerprint(client.get(name))
        for name in ("client_protocol", "client_reasoning", "client_config_os")
    }


def _client_matrix_evidence_gap(value: Any, path: str, failures: list[str], allow_unverified: bool = False) -> None:
    if not isinstance(value, dict):
        failures.append(f"{path}: evidence is missing")
        return
    status = value.get("status")
    if status not in FINAL_STATUSES and not (allow_unverified and status == "unverified"):
        failures.append(f"{path}: evidence is non-terminal ({status or 'missing'})")
    if _parse_date(value.get("observed_at")) is None:
        failures.append(f"{path}: observed_at must be an ISO date")
    if not _nonempty(value.get("source_ref")):
        failures.append(f"{path}: source_ref is required")
    digest = value.get("artifact_sha256")
    if status in FINAL_STATUSES and (not isinstance(digest, str) or not SHA256_RE.fullmatch(digest)):
        failures.append(f"{path}: terminal evidence requires artifact_sha256")


def evidence_entry(
    value: Any,
    path: str,
    failures: list[str],
    *,
    statuses: set[str] = FINAL_STATUSES,
    require_date: bool = False,
    as_of: date | None = None,
    max_age_days: int = DEFAULT_MAX_EVIDENCE_AGE_DAYS,
) -> str | None:
    if not isinstance(value, dict):
        failures.append(f"{path}: missing evidence object")
        return None
    status = value.get("status")
    if status not in statuses:
        failures.append(f"{path}: status must be one of {', '.join(sorted(statuses))}")
    if not _nonempty(value.get("evidence")):
        failures.append(f"{path}: evidence is required")
    if require_date:
        observed = _parse_date(value.get("verified_at"))
        if observed is None:
            failures.append(f"{path}: verified_at must be an ISO date")
        else:
            today = as_of or beijing_today()
            age = (today - observed).days
            if age < 0:
                failures.append(f"{path}: verified_at is in the future")
            elif age > max_age_days:
                failures.append(f"{path}: stale evidence ({age} days old; maximum {max_age_days})")
    return status if isinstance(status, str) else None


def audit_client_matrix(client_matrix: dict[str, Any]) -> dict[str, list[str]]:
    sections = _new_sections()
    clients = client_matrix.get("clients")
    if not isinstance(clients, list) or not clients:
        _add(sections, "client_protocol", "client-matrix: clients must be a non-empty array")
        return sections
    seen: set[str] = set()
    for client in clients:
        if not isinstance(client, dict) or not _nonempty(client.get("name")):
            _add(sections, "client_protocol", "client-matrix: client name is required")
            continue
        name = client["name"]
        if name in seen:
            _add(sections, "client_protocol", f"client-matrix/{name}: duplicate client")
        seen.add(name)
        if not _nonempty(client_version_key(client)):
            _add(sections, "client_protocol", f"client-matrix/{name}: version is required")
        protocols = client_protocols(client)
        protocol_matrix = client.get("client_protocol")
        protocol_rows = protocol_matrix.get("protocols") if isinstance(protocol_matrix, dict) else None
        all_protocols_explicitly_closed = bool(protocol_rows) and all(
            isinstance(row, dict) and row.get("support") in {"unsupported", "unverified"}
            for row in protocol_rows
        )
        if not isinstance(protocols, list) or not all(_nonempty(p) for p in protocols):
            _add(sections, "client_protocol", f"client-matrix/{name}: supported protocols are invalid")
        elif not protocols and not all_protocols_explicitly_closed:
            _add(sections, "client_protocol", f"client-matrix/{name}: protocols must contain a supported row or every row must be explicitly unsupported or unverified")
        elif len(protocols) != len(set(protocols)):
            _add(sections, "client_protocol", f"client-matrix/{name}: protocols contain duplicates")

        if isinstance(protocol_matrix, dict):
            rows = protocol_matrix.get("protocols")
            by_protocol = {
                row.get("protocol"): row for row in rows or []
                if isinstance(row, dict) and isinstance(row.get("protocol"), str)
            }
            declared_protocols = set(client_matrix.get("protocol_ids") or [])
            if declared_protocols and set(by_protocol) != declared_protocols:
                _add(sections, "client_protocol", f"client-matrix/{name}: protocol rows must enumerate protocol_ids exactly")
            required_features = set(client_matrix.get("client_transport_feature_ids") or REQUIRED_PROTOCOL_CHECKS)
            for protocol, row in by_protocol.items():
                row_support = row.get("support")
                if row_support not in {"supported", "unsupported", "unverified"}:
                    _add(sections, "client_protocol", f"client-matrix/{name}/{protocol}: invalid support state")
                features = row.get("client_transport_features")
                if not isinstance(features, dict) or set(features) != required_features:
                    _add(sections, "client_protocol", f"client-matrix/{name}/{protocol}: transport feature matrix is incomplete")
                elif row_support == "supported":
                    for feature, status in features.items():
                        if status not in FINAL_STATUSES:
                            _add(sections, "client_protocol", f"client-matrix/{name}/{protocol}/{feature}: feature is non-terminal ({status})")
                _client_matrix_evidence_gap(row.get("evidence"), f"client-matrix/{name}/{protocol}", sections["client_protocol"], allow_unverified=(row_support == "unverified"))

        reasoning = client_reasoning_contract(client)
        if not reasoning:
            _add(sections, "client_reasoning", f"client-matrix/{name}: reasoning matrix is missing")
        else:
            mode, levels = reasoning.get("mode"), reasoning.get("levels")
            if mode not in {"fixed", "model_defined", "none"}:
                _add(sections, "client_reasoning", f"client-matrix/{name}: invalid reasoning mode")
            if not isinstance(levels, list):
                _add(sections, "client_reasoning", f"client-matrix/{name}: reasoning levels must be an array")
            elif mode == "fixed" and not levels:
                _add(sections, "client_reasoning", f"client-matrix/{name}: fixed reasoning levels are empty")
            elif mode == "none" and levels:
                _add(sections, "client_reasoning", f"client-matrix/{name}: none reasoning mode must not expose levels")
            if not _nonempty(reasoning.get("notes")):
                _add(sections, "client_reasoning", f"client-matrix/{name}: reasoning notes are required")
        if isinstance(client.get("client_reasoning"), dict):
            _client_matrix_evidence_gap(
                client["client_reasoning"].get("evidence"),
                f"client-matrix/{name}/reasoning", sections["client_reasoning"],
            )

        config_v2 = client.get("client_config_os")
        if isinstance(config_v2, dict):
            release = config_v2.get("release")
            _client_matrix_evidence_gap(
                release.get("evidence") if isinstance(release, dict) else None,
                f"client-matrix/{name}/release", sections["client_config_os"],
            )
            os_rows = config_v2.get("os_support")
            if not isinstance(os_rows, list) or not os_rows:
                _add(sections, "client_config_os", f"client-matrix/{name}: os_support is required")
            else:
                os_ids = {row.get("os") for row in os_rows if isinstance(row, dict)}
                missing_oses = sorted({"macos", "linux", "windows"} - os_ids)
                if missing_oses:
                    _add(sections, "client_config_os", f"client-matrix/{name}: missing OS contracts: {', '.join(missing_oses)}")
            for row in os_rows or []:
                if not isinstance(row, dict) or not _nonempty(row.get("os")):
                    _add(sections, "client_config_os", f"client-matrix/{name}: OS id is required")
                    continue
                os_name = row["os"]
                files = row.get("config_files")
                if row.get("support") != "unsupported" and (
                    not isinstance(files, list) or not files
                    or not all(isinstance(item, dict) and _nonempty(item.get("path")) for item in files)
                ):
                    _add(sections, "client_config_os", f"client-matrix/{name}/{os_name}: config files are required")
                _client_matrix_evidence_gap(row.get("evidence"), f"client-matrix/{name}/{os_name}", sections["client_config_os"])
            endpoint = config_v2.get("endpoint")
            mutation = config_v2.get("mutation")
            verification_commands = config_v2.get("verification_commands")
            if not isinstance(endpoint, dict) or not _nonempty(endpoint.get("base_url_rule")) or not _nonempty(endpoint.get("credential_location")):
                _add(sections, "client_config_os", f"client-matrix/{name}: endpoint contract is incomplete")
            if not isinstance(mutation, dict) or not _nonempty(mutation.get("merge_strategy")):
                _add(sections, "client_config_os", f"client-matrix/{name}: mutation contract is incomplete")
            if not _nonempty(config_v2.get("verification_contract")):
                _add(sections, "client_config_os", f"client-matrix/{name}: verification_contract is missing")
            if not isinstance(verification_commands, list) or not verification_commands:
                _add(sections, "client_config_os", f"client-matrix/{name}: verification_commands are missing")
            elif any(
                "<" in command or ">" in command
                for row in verification_commands if isinstance(row, dict)
                for command in row.get("commands", []) if isinstance(command, str)
            ):
                _add(sections, "client_config_os", f"client-matrix/{name}: verification_commands contain placeholders")
            continue

        files = client.get("files")
        for os_name in ("unix", "windows"):
            paths = files.get(os_name) if isinstance(files, dict) else None
            if not isinstance(paths, list) or not paths or not all(_nonempty(path) for path in paths):
                _add(sections, "client_config_os", f"client-matrix/{name}/{os_name}: config files are required")
        config = client.get("config_contract")
        if not isinstance(config, dict):
            _add(sections, "client_config_os", f"client-matrix/{name}: config_contract is missing")
        else:
            for field in ("base_url_rule", "credential", "merge_strategy", "verification"):
                if not _nonempty(config.get(field)):
                    _add(sections, "client_config_os", f"client-matrix/{name}: config_contract.{field} is required")
    return sections


def _inventory_groups(inventory_model: dict[str, Any] | None) -> dict[str, Any]:
    groups = inventory_model.get("groups") if isinstance(inventory_model, dict) else None
    return groups if isinstance(groups, dict) else {}


def audit_contract_sections(
    contract: dict[str, Any], client_matrix: dict[str, Any],
    inventory_model: dict[str, Any] | None = None, *,
    currency: str | None = None, unit: str | None = None,
    as_of: date | None = None,
    max_age_days: int = DEFAULT_MAX_EVIDENCE_AGE_DAYS,
    canonical_price_rows: list[dict[str, Any]] | None = None,
) -> dict[str, list[str]]:
    sections = _new_sections()
    model = contract.get("model")
    model_id = model.get("id") if isinstance(model, dict) else None
    if not _nonempty(model_id):
        model_id = "<unknown>"
        _add(sections, "public_model", f"{model_id}: model.id is required")

    protocol_rows = contract.get("protocols")
    if not isinstance(protocol_rows, list) or not protocol_rows:
        _add(sections, "model_protocol", f"{model_id}: model protocol matrix is missing")
        protocol_rows = []
    protocol_by_name = {
        item.get("name"): item for item in protocol_rows
        if isinstance(item, dict) and _nonempty(item.get("name"))
    }
    verified_protocols = {name for name, item in protocol_by_name.items() if item.get("status") == "verified"}
    for name, item in protocol_by_name.items():
        evidence_entry(item, f"{model_id}/protocols/{name}", sections["model_protocol"])
    recommended = contract.get("recommended_protocol")
    if recommended not in verified_protocols:
        _add(sections, "model_protocol", f"{model_id}: recommended_protocol must reference a verified protocol")
    if not _nonempty(contract.get("recommended_protocol_reason")):
        _add(sections, "model_protocol", f"{model_id}: recommended_protocol_reason is required")

    expected_clients = {
        client["name"]: client for client in client_matrix.get("clients", [])
        if isinstance(client, dict) and _nonempty(client.get("name"))
        and verified_protocols.intersection(client_protocols(client))
        and client_os_names(client)
    }
    coverage = {
        item.get("name"): item for item in contract.get("client_coverage", [])
        if isinstance(item, dict) and _nonempty(item.get("name"))
    }
    test_matrix = contract.get("test_matrix")
    if not isinstance(test_matrix, dict):
        for matrix in ("model_protocol", "model_reasoning", "group_access", "test_evidence", "model_price"):
            _add(sections, matrix, f"{model_id}: test_matrix is missing")
        return sections

    limits = test_matrix.get("limits")
    if not isinstance(limits, dict):
        _add(sections, "public_model", f"{model_id}: test_matrix.limits is missing")
    else:
        for name in REQUIRED_LIMITS:
            evidence_entry(
                limits.get(name), f"{model_id}/limits/{name}", sections["public_model"],
                statuses=SPEC_TERMINAL_STATUSES,
            )
    modalities = test_matrix.get("modalities")
    if not isinstance(modalities, dict):
        _add(sections, "public_model", f"{model_id}: test_matrix.modalities is missing")
    else:
        for name in REQUIRED_MODALITIES:
            status = evidence_entry(modalities.get(name), f"{model_id}/modalities/{name}", sections["public_model"])
            expected = contract.get("verification", {}).get("modalities", {}).get(name)
            if status and expected and status != expected:
                _add(sections, "public_model", f"{model_id}/modalities/{name}: conflicts with contract")

    protocol_tests = test_matrix.get("protocols")
    feature_tests = test_matrix.get("protocol_features")
    if not isinstance(protocol_tests, dict):
        _add(sections, "model_protocol", f"{model_id}: test_matrix.protocols is missing")
        protocol_tests = {}
    if not isinstance(feature_tests, dict):
        _add(sections, "model_protocol", f"{model_id}: test_matrix.protocol_features is missing")
        feature_tests = {}
    for protocol in verified_protocols:
        checks = protocol_tests.get(protocol)
        if not isinstance(checks, dict):
            _add(sections, "model_protocol", f"{model_id}/{protocol}: protocol checks missing")
        else:
            for check in REQUIRED_PROTOCOL_CHECKS:
                # Protocol availability and individual capabilities are
                # separate facts. A text-capable protocol may have a terminal
                # negative tool/stream check; explicit unsupported is complete
                # evidence and must not invalidate the whole protocol row.
                evidence_entry(checks.get(check), f"{model_id}/{protocol}/{check}", sections["model_protocol"])
        features = feature_tests.get(protocol)
        if not isinstance(features, dict):
            _add(sections, "model_protocol", f"{model_id}/{protocol}: protocol feature matrix missing")
        else:
            for feature in REQUIRED_PROTOCOL_FEATURES:
                evidence_entry(
                    features.get(feature),
                    f"{model_id}/{protocol}/features/{feature}",
                    sections["model_protocol"],
                    statuses=SPEC_TERMINAL_STATUSES,
                )

    reasoning = test_matrix.get("reasoning")
    model_levels = contract.get("reasoning", {}).get("model_levels", [])
    if not isinstance(reasoning, dict):
        _add(sections, "model_reasoning", f"{model_id}: test_matrix.reasoning is missing")
        reasoning = {}
    tested_levels = reasoning.get("model_levels")
    if not isinstance(tested_levels, dict):
        _add(sections, "model_reasoning", f"{model_id}: reasoning.model_levels evidence is missing")
    else:
        for level in model_levels:
            if evidence_entry(tested_levels.get(level), f"{model_id}/reasoning/model/{level}", sections["model_reasoning"]) != "verified":
                _add(sections, "model_reasoning", f"{model_id}/reasoning/model/{level}: level is not verified")

    client_reasoning = reasoning.get("clients")
    if not isinstance(client_reasoning, dict):
        _add(sections, "client_reasoning", f"{model_id}: reasoning.clients is missing")
        client_reasoning = {}
    for name, client in expected_clients.items():
        per_client = client_reasoning.get(name)
        if not isinstance(per_client, dict):
            _add(sections, "client_reasoning", f"{model_id}/{name}: reasoning coverage missing")
            continue
        for protocol in verified_protocols.intersection(client_protocols(client)):
            result = per_client.get(protocol)
            if not isinstance(result, dict):
                message = "legacy reasoning entry must be keyed by protocol" if "status" in per_client else "reasoning coverage missing"
                _add(sections, "client_reasoning", f"{model_id}/{name}/{protocol}: {message}")
                continue
            status = evidence_entry(
                result, f"{model_id}/reasoning/clients/{name}/{protocol}", sections["client_reasoning"],
                statuses=REASONING_STATUSES,
            )
            levels = result.get("client_levels")
            if not isinstance(levels, list):
                _add(sections, "client_reasoning", f"{model_id}/{name}/{protocol}: client_levels must be an array")
                continue
            fixed = client_reasoning_contract(client)
            if fixed.get("mode") == "fixed" and levels != fixed.get("levels"):
                _add(sections, "client_reasoning", f"{model_id}/{name}/{protocol}: fixed client reasoning levels drifted")
            unsupported = set(levels) - set(model_levels)
            mappings = result.get("mappings")
            if status == "verified" and unsupported:
                if not isinstance(mappings, list) or not mappings:
                    _add(sections, "client_reasoning", f"{model_id}/{name}/{protocol}: differing reasoning levels require mappings")
                    continue
                mapping_by_source: dict[str, Any] = {}
                for mapping in mappings:
                    if not isinstance(mapping, dict):
                        continue
                    if mapping.get("client") != name or mapping.get("protocol") != protocol:
                        _add(sections, "client_reasoning", f"{model_id}/{name}/{protocol}: every reasoning mapping must name the exact client and protocol")
                    mapping_by_source[mapping.get("from")] = mapping.get("to")
                missing = sorted(unsupported - mapping_by_source.keys())
                if missing:
                    _add(sections, "client_reasoning", f"{model_id}/{name}/{protocol}: unmapped client reasoning levels: {', '.join(missing)}")
                invalid = sorted({target for target in mapping_by_source.values() if target not in set(model_levels)})
                if invalid:
                    _add(sections, "client_reasoning", f"{model_id}/{name}/{protocol}: mapping targets unsupported model levels: {', '.join(invalid)}")

    # Exact model x protocol x client x version x OS cells.
    client_tests = test_matrix.get("clients")
    if not isinstance(client_tests, dict):
        _add(sections, "test_evidence", f"{model_id}: test_matrix.clients is missing")
        client_tests = {}
    missing_coverage = sorted(set(expected_clients) - set(coverage))
    if missing_coverage:
        _add(sections, "test_evidence", f"{model_id}: missing client coverage: {', '.join(missing_coverage)}")
    for name, client in expected_clients.items():
        result = client_tests.get(name)
        if not isinstance(result, dict):
            _add(sections, "test_evidence", f"{model_id}/{name}: client protocol matrix is missing")
            continue
        verified_for_coverage: set[str] = set()
        for protocol in verified_protocols.intersection(client_protocols(client)):
            os_cells = result.get(protocol)
            if not isinstance(os_cells, dict):
                _add(sections, "test_evidence", f"{model_id}/clients/{name}/{protocol}: OS evidence matrix is missing")
                continue
            all_verified = True
            expected_oses = set(client_os_names(client)) or {"unix", "windows"}
            for os_name in sorted(expected_oses):
                cell = os_cells.get(os_name)
                status = evidence_entry(
                    cell, f"{model_id}/clients/{name}/{protocol}/{os_name}", sections["test_evidence"],
                    require_date=True, as_of=as_of, max_age_days=max_age_days,
                )
                version_key = client_version_key(client)
                if not isinstance(cell, dict) or cell.get("client_version") != version_key:
                    _add(sections, "test_evidence", f"{model_id}/clients/{name}/{protocol}/{os_name}: client_version must exactly match {version_key}")
                if isinstance(cell, dict):
                    if cell.get("model_id") != model_id:
                        _add(sections, "test_evidence", f"{model_id}/clients/{name}/{protocol}/{os_name}: model_id mismatch")
                    if cell.get("protocol") != protocol:
                        _add(sections, "test_evidence", f"{model_id}/clients/{name}/{protocol}/{os_name}: protocol mismatch")
                    if cell.get("os") != os_name:
                        _add(sections, "test_evidence", f"{model_id}/clients/{name}/{protocol}/{os_name}: os mismatch")
                all_verified = all_verified and status == "verified"
            if all_verified:
                verified_for_coverage.add(protocol)
        coverage_item = coverage.get(name) or {}
        if set(coverage_item.get("protocols") or []) != verified_for_coverage:
            _add(sections, "test_evidence", f"{model_id}/{name}: client_coverage protocols must equal fully verified OS cells")
        expected_status = "verified" if verified_for_coverage else "unsupported"
        if coverage_item.get("status") not in FINAL_STATUSES:
            _add(sections, "test_evidence", f"{model_id}/{name}: client status is not final")
        elif coverage_item.get("status") != expected_status:
            _add(sections, "test_evidence", f"{model_id}/{name}: client_coverage status conflicts with exact evidence cells")
    for item in contract.get("clients", []):
        if not isinstance(item, dict):
            continue
        name, protocol = item.get("name"), item.get("protocol")
        per_name = client_tests.get(name)
        cell_map = per_name.get(protocol) if isinstance(per_name, dict) else None
        client = expected_clients.get(name)
        required_oses = client_os_names(client) if isinstance(client, dict) else []
        required_cells = [cell_map.get(os_name) for os_name in required_oses] if isinstance(cell_map, dict) else []
        if not required_cells or any(
            not isinstance(cell, dict) or cell.get("status") != "verified" for cell in required_cells
        ):
            _add(sections, "test_evidence", f"{model_id}/{name}: public client protocol must reference fully verified OS cells")

    public_groups = _inventory_groups(inventory_model)
    access_groups = {
        item.get("name"): item for item in contract.get("access", {}).get("groups", [])
        if isinstance(item, dict) and _nonempty(item.get("name"))
    }
    if public_groups and set(access_groups) != set(public_groups):
        missing, extra = sorted(set(public_groups) - set(access_groups)), sorted(set(access_groups) - set(public_groups))
        if missing:
            _add(sections, "group_access", f"{model_id}: missing public groups: {', '.join(missing)}")
        if extra:
            _add(sections, "group_access", f"{model_id}: groups absent from public inventory: {', '.join(extra)}")
    group_tests = test_matrix.get("group_access")
    if not isinstance(group_tests, dict):
        _add(sections, "group_access", f"{model_id}: test_matrix.group_access is missing")
        group_tests = {}
    for group_name, access in access_groups.items():
        cell = group_tests.get(group_name)
        status = evidence_entry(
            cell, f"{model_id}/group_access/{group_name}", sections["group_access"],
            require_date=True, as_of=as_of, max_age_days=max_age_days,
        )
        if status not in {"verified", "unsupported"}:
            _add(sections, "group_access", f"{model_id}/group_access/{group_name}: public access must be terminal")
        if isinstance(cell, dict):
            if cell.get("model_id") != model_id:
                _add(sections, "group_access", f"{model_id}/group_access/{group_name}: model_id mismatch")
            if cell.get("base_url") != contract.get("access", {}).get("base_url"):
                _add(sections, "group_access", f"{model_id}/group_access/{group_name}: base_url mismatch")
            cell_protocols = set(cell.get("protocols") or [])
            if not cell_protocols or not cell_protocols.issubset(verified_protocols):
                _add(sections, "group_access", f"{model_id}/group_access/{group_name}: protocols must be verified model protocols")
            if cell.get("recommended_protocol") not in cell_protocols:
                _add(sections, "group_access", f"{model_id}/group_access/{group_name}: recommended protocol is not group-accessible")
            if not _nonempty(cell.get("recommended_protocol_reason")):
                _add(sections, "group_access", f"{model_id}/group_access/{group_name}: recommended protocol reason is required")
            expected_group = public_groups.get(group_name)
            expected_multiplier = expected_group.get("rate_multiplier") if isinstance(expected_group, dict) else access.get("multiplier")
            if cell.get("multiplier") != expected_multiplier or access.get("multiplier") != expected_multiplier:
                _add(sections, "group_access", f"{model_id}/group_access/{group_name}: multiplier conflicts with public inventory")

    scoped_prices = {
        (str(row.get("model_id")), str(row.get("group_name"))): row
        for row in (canonical_price_rows or [])
        if isinstance(row, dict) and row.get("price_scope") == "group_customer"
    }
    if canonical_price_rows is not None:
        for group_name in access_groups:
            row = scoped_prices.get((model_id, group_name))
            if not isinstance(row, dict):
                _add(sections, "model_price", f"{model_id}/pricing/{group_name}: canonical group_customer row is missing")
                continue
            if row.get("status") not in PRICE_TERMINAL_STATUSES:
                _add(sections, "model_price", f"{model_id}/pricing/{group_name}: canonical price status is non-terminal ({row.get('status')})")
            component_states = set((row.get("component_status") or {}).values())
            if component_states.intersection({"unknown", "blocked"}):
                _add(sections, "model_price", f"{model_id}/pricing/{group_name}: canonical price components remain non-terminal")
        return sections

    # Backward-compatible migration path when no canonical scoped M9 artifact
    # is supplied. These legacy contract cells are not allowed to override a
    # canonical provider/gateway/customer matrix.
    price_tests = test_matrix.get("pricing")
    if not isinstance(price_tests, dict):
        _add(sections, "model_price", f"{model_id}: test_matrix.pricing is missing")
        price_tests = {}
    for group_name in access_groups:
        cell = price_tests.get(group_name)
        status = evidence_entry(
            cell, f"{model_id}/pricing/{group_name}", sections["model_price"],
            require_date=True, as_of=as_of, max_age_days=max_age_days,
        )
        if status != "verified":
            _add(sections, "model_price", f"{model_id}/pricing/{group_name}: public price must be verified")
        if not isinstance(cell, dict):
            continue
        if currency is not None and cell.get("currency") != currency:
            _add(sections, "model_price", f"{model_id}/pricing/{group_name}: currency conflicts with public inventory")
        if unit is not None and cell.get("unit") != unit:
            _add(sections, "model_price", f"{model_id}/pricing/{group_name}: unit conflicts with public inventory")
        expected_group = public_groups.get(group_name)
        expected_price = expected_group.get("price") if isinstance(expected_group, dict) else None
        if isinstance(expected_price, dict):
            for field in PRICE_FIELDS | OPTIONAL_PRICE_FIELDS:
                if field in expected_price and cell.get(field) != expected_price.get(field):
                    _add(sections, "model_price", f"{model_id}/pricing/{group_name}/{field}: conflicts with public inventory")
        else:
            for field in PRICE_FIELDS:
                if field not in cell:
                    _add(sections, "model_price", f"{model_id}/pricing/{group_name}/{field}: price field is required")
    return sections


def audit_contract(
    contract: dict[str, Any], client_matrix: dict[str, Any],
    inventory_model: dict[str, Any] | None = None, **kwargs: Any,
) -> list[str]:
    sections = audit_contract_sections(contract, client_matrix, inventory_model, **kwargs)
    return [failure for name in MATRIX_NAMES for failure in sections[name]]


def _merge_sections(target: dict[str, list[str]], source: dict[str, list[str]]) -> None:
    for name in MATRIX_NAMES:
        target[name].extend(source.get(name, []))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("audit", "plan"))
    parser.add_argument("--contracts", type=Path, required=True)
    parser.add_argument("--client-matrix", type=Path, required=True)
    parser.add_argument("--max-evidence-age-days", type=int, default=DEFAULT_MAX_EVIDENCE_AGE_DAYS)
    parser.add_argument("--as-of", help="ISO date used for deterministic stale-evidence checks")
    parser.add_argument(
        "--model-price-matrix", type=Path,
        help="canonical scoped M9 artifact; defaults to the newest artifacts/model-price-plan-current-*.json",
    )
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--inventory-json", type=Path)
    source.add_argument("--pricing-url")
    args = parser.parse_args()
    try:
        as_of = date.fromisoformat(args.as_of) if args.as_of else beijing_today()
        inventory = load_inventory_catalog(args.inventory_json, args.pricing_url)
        client_matrix = load_json(args.client_matrix)
        price_matrix_path = args.model_price_matrix
        if price_matrix_path is None:
            candidates = sorted((ROOT / "artifacts").glob("model-price-plan-current-*.json"))
            price_matrix_path = candidates[-1] if candidates else None
        canonical_price_rows: list[dict[str, Any]] | None = None
        canonical_price_issues: list[dict[str, Any]] = []
        if price_matrix_path is not None:
            price_matrix = load_json(price_matrix_path)
            if price_matrix.get("kind") != "model_price_matrix" or price_matrix.get("schema_version") != 2:
                raise ValueError("model price matrix must be a schema_version 2 model_price_matrix artifact")
            canonical_price_rows = price_matrix.get("rows")
            if not isinstance(canonical_price_rows, list):
                raise ValueError("model price matrix rows must be an array")
            canonical_price_issues = [
                issue for issue in price_matrix.get("issues", [])
                if isinstance(issue, dict) and issue.get("type") not in {
                    "not_published", "not_exposed", "not_applicable"
                }
            ]
        contracts: dict[str, dict[str, Any]] = {}
        for path in sorted(args.contracts.glob("*.json")):
            if path.resolve() == args.client_matrix.resolve():
                continue
            raw = load_json(path)
            model_id = raw.get("model", {}).get("id")
            if isinstance(model_id, str):
                contracts[model_id] = raw

        sections = _new_sections()
        _merge_sections(sections, audit_client_matrix(client_matrix))
        for issue in canonical_price_issues:
            _add(
                sections, "model_price",
                "canonical M9: %s/%s/%s: %s" % (
                    issue.get("model_id"), issue.get("group_id"), issue.get("column"), issue.get("message")
                ),
            )
        public_models = set(inventory["models"])
        missing_models = sorted(public_models - set(contracts))
        if missing_models:
            _add(sections, "public_model", "missing model contracts: " + ", ".join(missing_models))
        for model_id in sorted(public_models.intersection(contracts)):
            _merge_sections(sections, audit_contract_sections(
                contracts[model_id], client_matrix, inventory["models"].get(model_id),
                currency=inventory.get("currency"), unit=inventory.get("unit"),
                as_of=as_of, max_age_days=args.max_evidence_age_days,
                canonical_price_rows=canonical_price_rows,
            ))
        failures = [failure for name in MATRIX_NAMES for failure in sections[name]]
        report = {
            "inventory_models": len(public_models), "contracts": len(contracts),
            "complete": not failures,
            "matrices": {name: {"complete": not sections[name], "failures": sections[name]} for name in MATRIX_NAMES},
            "stale_evidence": [failure for failure in failures if "stale evidence" in failure],
            "failures": failures,
        }
        print(json.dumps(report, ensure_ascii=False, indent=2))
        return 2 if args.command == "audit" and failures else 0
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

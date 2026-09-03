#!/usr/bin/env python3
"""Build one deterministic, duplicate-free execution queue from matrix gaps."""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import re
from collections import Counter
from pathlib import Path
from typing import Any


NON_CLI_CLIENT_IDS = {
    "cursor-desktop", "traecode", "doubao-work", "workbuddy", "vscode-local-agent",
}
NON_CLI_CLIENT_ALIASES = NON_CLI_CLIENT_IDS | {"cursor", "cursor-desktop", "trae", "traecode", "doubao-work"}


def load(path: Path) -> Any:
    return json.loads(path.read_text())


def slug(value: Any) -> str:
    return re.sub(r"[^a-z0-9]+", "-", str(value or "").casefold()).strip("-")


def split_failure(value: str) -> tuple[str, str]:
    path, separator, reason = value.partition(": ")
    return path, reason if separator else "matrix audit gap"


def actionable_path(matrix: str, path: str) -> tuple[str | None, str]:
    parts = path.split("/")
    if matrix == "model_price" and len(parts) >= 3 and parts[1] == "pricing":
        return "/".join(parts[:3]), "price_reconciliation"
    if matrix == "test_evidence":
        if len(parts) >= 5 and parts[1] == "clients":
            return "/".join(parts[:5]), "real_client_loop"
        return None, "derived_client_coverage"
    return path, {
        "public_model": "official_spec",
        "model_protocol": "provider_contract",
        "model_reasoning": "model_reasoning",
        "client_protocol": "client_protocol",
        "client_reasoning": "client_reasoning",
        "group_access": "group_access",
        "client_config_os": "client_config_os",
        "model_price": "price_reconciliation",
    }.get(matrix, matrix)


def target_from_path(matrix: str, path: str) -> dict[str, str]:
    parts = path.split("/")
    target: dict[str, str] = {"audit_path": path}
    if parts:
        target["model_id"] = parts[0]
    if matrix == "client_config_os" and len(parts) >= 3 and parts[0] == "client-matrix":
        target.pop("model_id", None)
        target.update({"client_id": parts[1], "os": parts[2]})
    elif matrix == "client_protocol" and len(parts) >= 3 and parts[0] == "client-matrix":
        target.pop("model_id", None)
        target.update({"client_id": parts[1], "protocol": parts[2]})
    elif matrix == "model_protocol" and len(parts) > 1:
        target["protocol"] = parts[1]
        if "features" in parts:
            index = parts.index("features")
            if len(parts) > index + 1:
                target["test_case_id"] = parts[index + 1]
        elif len(parts) > 2:
            target["test_case_id"] = parts[2]
    elif matrix == "group_access" and len(parts) > 2:
        target["group_name"] = parts[2]
    elif matrix == "client_reasoning" and "clients" in parts:
        index = parts.index("clients")
        if len(parts) > index + 2:
            target["client_id"] = parts[index + 1]
            target["protocol"] = parts[index + 2]
    elif matrix == "test_evidence" and len(parts) >= 5:
        target.update({"client_id": parts[2], "protocol": parts[3], "os": parts[4]})
    elif matrix == "model_price" and len(parts) >= 3:
        target["group_name"] = parts[2]
    return target


def canonical_client_lookup(client_matrix: dict[str, Any]) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for client in client_matrix.get("clients", []):
        if not isinstance(client, dict):
            continue
        for value in (client.get("id"), client.get("name")):
            if isinstance(value, str) and value:
                result[slug(value)] = client
    return result


def enrich_semantic_targets(cases: dict[str, dict[str, Any]], client_matrix: dict[str, Any]) -> None:
    clients = canonical_client_lookup(client_matrix)
    for row in cases.values():
        target = row.get("target")
        if not isinstance(target, dict) or not isinstance(target.get("client_id"), str):
            continue
        client = clients.get(slug(target["client_id"]))
        if not client:
            continue
        target["client_id"] = client.get("name") or client.get("id")
        version = client.get("client_config_os", {}).get("release", {}).get("version_key")
        if target.get("client_version") is None and isinstance(version, str):
            target["client_version"] = version
        if row["matrix"] == "client_config_os" and target.get("os") and row["case_type"] == "client_config_os":
            row["case_type"] = "config_qa"
        if row["matrix"] == "client_protocol" and target.get("protocol") and row["case_type"] == "client_protocol":
            row["case_type"] = "client_protocol_runtime"


def semantic_case_key(row: dict[str, Any]) -> str:
    target = row.get("target", {})
    case_type = row.get("case_type")
    client = slug(target.get("client_id"))
    model = target.get("model_id")
    protocol = target.get("protocol")
    version = target.get("client_version")
    os_id = target.get("os")
    feature = target.get("test_case_id")
    if case_type in {"provider_contract", "provider_tool_contract"}:
        value = ("provider_contract", model, protocol, feature)
    elif case_type == "model_reasoning":
        value = ("model_reasoning", model, target.get("reasoning_level"))
    elif case_type == "group_access":
        value = ("group_access", model, target.get("group_id") or slug(target.get("group_name")))
    elif case_type == "real_client_loop":
        value = ("real_client_loop", model, client, version, protocol, os_id)
    elif case_type == "client_protocol_runtime":
        value = ("client_protocol_runtime", client, version, protocol)
    elif case_type == "config_qa":
        value = ("config_qa", client, version, os_id)
    elif case_type == "client_reasoning":
        value = ("client_reasoning", model, client, version, protocol)
    elif case_type == "official_spec":
        value = ("official_spec", model, feature)
    elif case_type == "price_reconciliation":
        value = ("price_reconciliation", model, target.get("group_id") or slug(target.get("group_name")))
    elif case_type == "client_contract_research":
        value = ("client_contract_research", client, version)
    else:
        value = (case_type, tuple(sorted((key, str(value)) for key, value in target.items() if key != "audit_path")))
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def semantic_dedupe(cases: dict[str, dict[str, Any]]) -> tuple[dict[str, dict[str, Any]], dict[str, int]]:
    priority = {
        "client_matrix_runtime_gate": 0,
        "legacy_terminal_status_gate": 1,
        "execution_scope": 2,
        "matrix_audit": 3,
    }
    before_rows = list(cases.values())
    before_batches = len({execution_batch_key(row) for row in before_rows})
    grouped: dict[str, list[dict[str, Any]]] = {}
    for row in before_rows:
        grouped.setdefault(semantic_case_key(row), []).append(row)
    output: dict[str, dict[str, Any]] = {}
    for semantic_key, rows in grouped.items():
        rows.sort(key=lambda row: (priority.get(row.get("source"), 9), row["case_key"]))
        primary = rows[0]
        primary["semantic_key"] = semantic_key
        primary["merged_case_keys"] = sorted({row["case_key"] for row in rows})
        primary["merged_sources"] = sorted({str(row.get("source")) for row in rows})
        primary["merged_audit_paths"] = sorted({
            str(row.get("target", {}).get("audit_path")) for row in rows
            if row.get("target", {}).get("audit_path")
        })
        reasons = []
        for row in rows:
            for reason in row.get("reasons", []):
                if reason not in reasons:
                    reasons.append(reason)
            for key, value in row.get("target", {}).items():
                if key not in primary["target"] or primary["target"][key] is None:
                    primary["target"][key] = value
        primary["reasons"] = reasons
        output[primary["case_key"]] = primary
    after_rows = list(output.values())
    after_batches = len({execution_batch_key(row) for row in after_rows})
    return output, {
        "case_count_before": len(before_rows),
        "case_count_after": len(after_rows),
        "duplicates_removed": len(before_rows) - len(after_rows),
        "batch_count_before": before_batches,
        "batch_count_after": after_batches,
        "batch_work_items_before": len(before_rows),
        "batch_work_items_after": len(after_rows),
    }


def apply_shortest_path_policy(
    cases: dict[str, dict[str, Any]], client_matrix: dict[str, Any],
) -> tuple[dict[str, dict[str, Any]], dict[str, int]]:
    aliases: dict[str, str] = {}
    for client in client_matrix.get("clients", []):
        if not isinstance(client, dict) or not isinstance(client.get("id"), str):
            continue
        for alias in (slug(client["id"]), slug(client.get("name"))):
            if alias:
                aliases[alias] = client["id"]
    if "vscode-local-agent" in {client.get("id") for client in client_matrix.get("clients", []) if isinstance(client, dict)}:
        aliases["vscode"] = "vscode-local-agent"
    by_id = {client.get("id"): client for client in client_matrix.get("clients", []) if isinstance(client, dict)}
    output: dict[str, dict[str, Any]] = {}
    removed_gui = removed_cartesian = 0
    for key, row in cases.items():
        target_client = slug(row.get("target", {}).get("client_id"))
        client_id = aliases.get(target_client) if target_client else None
        if client_id in NON_CLI_CLIENT_IDS or target_client in NON_CLI_CLIENT_ALIASES:
            removed_gui += 1
            continue
        if row.get("case_type") == "real_client_loop" and row.get("source") == "legacy_terminal_status_gate":
            removed_cartesian += 1
            continue
        if row.get("case_type") == "client_contract_research" and client_id in by_id:
            if by_id[client_id].get("client_config_os", {}).get("release", {}).get("evidence", {}).get("status") == "verified":
                row["status"] = "satisfied"
        output[key] = row
    return output, {
        "removed_non_cli_gui_cases": removed_gui,
        "removed_redundant_cartesian_client_loops": removed_cartesian,
    }


def evidence_matches(target: dict[str, str], evidence: dict[str, Any]) -> bool:
    observed = evidence.get("target")
    if not isinstance(observed, dict):
        return False
    for field in ("model_id", "protocol", "os"):
        if field in target and target[field] != observed.get(field):
            return False
    if "client_id" in target:
        if slug(target["client_id"]) != slug(observed.get("client_id")):
            return False
    return any(field in target for field in ("model_id", "client_id", "protocol", "os"))


def receipt_matches(row: dict[str, Any], receipt: dict[str, Any]) -> bool:
    target = row["target"]
    observed = receipt.get("target")
    if not isinstance(observed, dict):
        return False
    case_type = row["case_type"]
    allowed_evidence_types = {
        "provider_contract": {"live_protocol_probe", "regression_test"},
        "provider_tool_contract": {"live_protocol_probe", "regression_test"},
        "model_reasoning": {"live_protocol_probe", "regression_test"},
        "group_access": {"live_protocol_probe", "matrix_intersection"},
        "real_client_loop": {"real_client_loop"},
        "client_protocol_runtime": {"real_client_loop", "regression_test"},
        "config_qa": {"config_qa"},
        "client_reasoning": {"real_client_loop", "regression_test"},
        "official_spec": {"official_spec"},
    }
    if receipt.get("evidence_type") not in allowed_evidence_types.get(case_type, set()):
        return False
    required: tuple[str, ...]
    if case_type in {"provider_contract", "provider_tool_contract"}:
        required = ("model_id", "protocol", "test_case_id")
    elif case_type == "model_reasoning":
        required = ("model_id", "reasoning_level")
    elif case_type == "group_access":
        required = ("model_id", "group_id")
    elif case_type == "real_client_loop":
        required = ("model_id", "client_id", "client_version", "protocol", "os")
    elif case_type == "client_protocol_runtime":
        required = ("client_id", "client_version", "protocol")
    elif case_type == "config_qa":
        required = ("client_id", "client_version", "os")
    elif case_type == "client_reasoning":
        required = ("model_id", "client_id", "protocol")
    elif case_type == "official_spec":
        required = ("model_id", "test_case_id")
    else:
        return False
    if any(target.get(field) is None for field in required):
        return False
    for field in required:
        observed_field = "feature" if field == "test_case_id" else field
        expected = target[field]
        actual = observed.get(observed_field)
        if field == "client_id":
            if slug(expected) != slug(actual):
                return False
        elif field == "client_version":
            if not exact_version_equivalent(expected, actual):
                return False
        elif actual != expected:
            return False
    if case_type == "group_access" and observed.get("feature") not in set(row.get("required_features", [])):
        return False
    if case_type == "real_client_loop" and observed.get("feature") != "agent_loop":
        return False
    return True


def exact_version_equivalent(expected: Any, actual: Any) -> bool:
    if expected == actual:
        return True
    if not isinstance(expected, str) or not isinstance(actual, str):
        return False
    typed_prefixes = {"cli", "app", "desktop", "web"}
    for typed, display in ((expected, actual), (actual, expected)):
        prefix, separator, suffix = typed.partition(":")
        if separator and prefix in typed_prefixes and suffix == display:
            return True
    return False


def latest_receipts_by_feature(receipts: list[dict[str, Any]]) -> list[dict[str, Any]]:
    grouped: dict[str, list[dict[str, Any]]] = {}
    for receipt in receipts:
        feature = str(receipt.get("target", {}).get("feature") or "__default__")
        grouped.setdefault(feature, []).append(receipt)

    def stamp(receipt: dict[str, Any]) -> float:
        raw = receipt.get("observed_at")
        if not isinstance(raw, str):
            return float("-inf")
        try:
            parsed = datetime.fromisoformat(raw.replace("Z", "+00:00"))
            if parsed.tzinfo is None:
                parsed = parsed.replace(tzinfo=timezone.utc)
            return parsed.astimezone(timezone.utc).timestamp()
        except ValueError:
            return float("-inf")

    latest: list[dict[str, Any]] = []
    for rows in grouped.values():
        maximum = max(stamp(row) for row in rows)
        latest.extend(row for row in rows if stamp(row) == maximum)
    return latest


def receipt_artifact_valid(receipt: dict[str, Any]) -> bool:
    uri = receipt.get("artifact_uri")
    expected = receipt.get("artifact_sha256")
    if not isinstance(uri, str) or not isinstance(expected, str):
        return False
    path = Path(uri)
    if not path.is_file():
        return False
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    return digest == expected


def execution_batch_key(row: dict[str, Any]) -> str:
    target = row["target"]
    case_type = row["case_type"]
    model = target.get("model_id", "none")
    client = slug(target.get("client_id", "none"))
    protocol = target.get("protocol", "none")
    os_id = target.get("os", "none")
    version = target.get("client_version", "none")
    if case_type == "official_spec":
        return f"official-spec:{model}"
    if case_type in {"provider_contract", "provider_tool_contract"}:
        return f"provider-contract:{model}:{protocol}"
    if case_type == "model_reasoning":
        return f"model-reasoning:{model}"
    if case_type == "group_access":
        group_id = target.get("group_id", "unknown")
        return f"group-access:{group_id}:{slug(target.get('group_name'))}"
    if case_type == "price_reconciliation":
        return "price-reconciliation:public-inventory"
    if case_type == "real_client_loop":
        return f"real-client-loop:{model}:{client}:{version}:{protocol}:{os_id}"
    if case_type == "client_protocol_runtime":
        return f"client-protocol:{client}:{version}:{protocol}"
    if case_type == "config_qa":
        return f"config-qa:{client}:{version}:{os_id}"
    if case_type == "client_reasoning":
        return f"client-reasoning:{model}:{client}:{protocol}"
    if case_type == "client_contract_research":
        return f"client-research:{client}"
    return f"{case_type}:{row['case_key']}"


def build_queue(
    gaps: dict[str, Any],
    client_matrix: dict[str, Any],
    scope: dict[str, Any],
    evidence_inventory: dict[str, Any],
    contracts: list[dict[str, Any]] | None = None,
    terminal_evidence: dict[str, Any] | None = None,
    pricing_inventory: dict[str, Any] | None = None,
) -> dict[str, Any]:
    cases: dict[str, dict[str, Any]] = {}
    derived: list[dict[str, str]] = []
    actionable_gap_records: list[dict[str, str]] = []
    raw_failures = 0

    for matrix, section in gaps.get("matrices", {}).items():
        for failure in section.get("failures", []):
            raw_failures += 1
            path, reason = split_failure(failure)
            normalized, case_type = actionable_path(matrix, path)
            if normalized is None:
                derived.append({"matrix": matrix, "audit_path": path, "reason": reason})
                continue
            key = f"{matrix}:{normalized}"
            actionable_gap_records.append({
                "matrix": matrix, "audit_path": path, "reason": reason, "case_key": key,
            })
            row = cases.setdefault(key, {
                "case_key": key,
                "matrix": matrix,
                "case_type": case_type,
                "target": target_from_path(matrix, normalized),
                "reasons": [],
                "source": "matrix_audit",
                "status": "planned",
            })
            if reason not in row["reasons"]:
                row["reasons"].append(reason)

    for client in client_matrix.get("clients", []):
        client_id = client.get("id")
        display_name = client.get("name") or client_id
        version = client.get("client_config_os", {}).get("release", {}).get("version_key")
        for protocol in client.get("client_protocol", {}).get("protocols", []):
            if protocol.get("support") != "supported":
                continue
            protocol_id = protocol.get("protocol")
            key = f"client_protocol:{client_id}:{version}:{protocol_id}"
            cases.setdefault(key, {
                "case_key": key,
                "matrix": "client_protocol",
                "case_type": "client_protocol_runtime",
                "target": {"client_id": display_name, "client_version": version, "protocol": protocol_id},
                "reasons": ["supported protocol still requires exact runtime feature evidence"],
                "source": "client_matrix_runtime_gate",
                "status": "planned",
            })
        for os_row in client.get("client_config_os", {}).get("os_support", []):
            os_id = os_row.get("os")
            key = f"client_config_os:{client_id}:{version}:{os_id}"
            cases.setdefault(key, {
                "case_key": key,
                "matrix": "client_config_os",
                "case_type": "config_qa",
                "target": {"client_id": display_name, "client_version": version, "os": os_id},
                "reasons": ["documented config path requires a real OS receipt"],
                "source": "client_matrix_runtime_gate",
                "status": "planned",
                "required_features": ["safe_write", "backup_recovery", "idempotency"],
            })

    for client in scope.get("clients", []):
        if client.get("scope_status") == "existing_contract":
            continue
        client_id = client.get("client_id")
        key = f"client_identity:{client_id}"
        cases.setdefault(key, {
            "case_key": key,
            "matrix": "client_config_os",
            "case_type": "client_contract_research",
            "target": {"client_id": client_id, "client_version": client.get("version_key")},
            "reasons": ["new client identity/version/protocol/config contract is not yet canonical"],
            "source": "execution_scope",
            "status": "planned",
        })

    # Existing prose "verified" and "unsupported" cells remain queued until
    # an immutable terminal receipt proves them. This prevents a green legacy
    # status from escaping the new M8 evidence contract merely because it no
    # longer appears in the ordinary gap report.
    for contract in contracts or []:
        model_id = contract.get("model", {}).get("id")
        if not isinstance(model_id, str):
            continue
        matrix = contract.get("test_matrix", {})
        for section_name, case_type in (("limits", "official_spec"), ("modalities", "official_spec")):
            for feature, cell in matrix.get(section_name, {}).items():
                if not isinstance(cell, dict) or cell.get("status") not in {"verified", "unsupported", "blocked"}:
                    continue
                key = f"public_model:{model_id}/{section_name}/{feature}"
                cases.setdefault(key, {
                    "case_key": key, "matrix": "public_model", "case_type": case_type,
                    "target": {"model_id": model_id, "test_case_id": f"{section_name}.{feature}"},
                    "reasons": ["legacy terminal status requires an immutable M8 receipt"],
                    "source": "legacy_terminal_status_gate", "status": "planned",
                })
        for protocol, checks in matrix.get("protocols", {}).items():
            for check, cell in checks.items():
                if not isinstance(cell, dict) or cell.get("status") not in {"verified", "unsupported", "blocked"}:
                    continue
                key = f"model_protocol:{model_id}/{protocol}/{check}"
                cases.setdefault(key, {
                    "case_key": key, "matrix": "model_protocol", "case_type": "provider_contract",
                    "target": {"model_id": model_id, "protocol": protocol, "test_case_id": check},
                    "reasons": ["legacy terminal status requires an immutable M8 receipt"],
                    "source": "legacy_terminal_status_gate", "status": "planned",
                })
        for protocol, features in matrix.get("protocol_features", {}).items():
            for feature, cell in features.items():
                if not isinstance(cell, dict) or cell.get("status") not in {"verified", "unsupported", "blocked"}:
                    continue
                key = f"model_protocol:{model_id}/{protocol}/features/{feature}"
                cases.setdefault(key, {
                    "case_key": key, "matrix": "model_protocol", "case_type": "provider_contract",
                    "target": {"model_id": model_id, "protocol": protocol, "test_case_id": feature},
                    "reasons": ["legacy terminal status requires an immutable M8 receipt"],
                    "source": "legacy_terminal_status_gate", "status": "planned",
                })
        for protocol, tools in matrix.get("tools", {}).items():
            for tool, cell in tools.items():
                if not isinstance(cell, dict) or cell.get("status") not in {"verified", "unsupported", "blocked"}:
                    continue
                key = f"model_protocol:{model_id}/{protocol}/tools/{tool}"
                cases.setdefault(key, {
                    "case_key": key, "matrix": "model_protocol", "case_type": "provider_tool_contract",
                    "target": {"model_id": model_id, "protocol": protocol, "test_case_id": f"tool.{tool}"},
                    "reasons": ["legacy terminal status requires an immutable M8 receipt"],
                    "source": "legacy_terminal_status_gate", "status": "planned",
                })
        for level, cell in matrix.get("reasoning", {}).get("model_levels", {}).items():
            if not isinstance(cell, dict) or cell.get("status") not in {"verified", "unsupported", "blocked"}:
                continue
            key = f"model_reasoning:{model_id}/reasoning/model/{level}"
            cases.setdefault(key, {
                "case_key": key, "matrix": "model_reasoning", "case_type": "model_reasoning",
                "target": {"model_id": model_id, "reasoning_level": level},
                "reasons": ["legacy terminal status requires an immutable M8 receipt"],
                "source": "legacy_terminal_status_gate", "status": "planned",
            })
        for group_name, cell in matrix.get("group_access", {}).items():
            if not isinstance(cell, dict) or cell.get("status") not in {"verified", "unsupported", "blocked"}:
                continue
            key = f"group_access:{model_id}/group_access/{group_name}"
            cases.setdefault(key, {
                "case_key": key, "matrix": "group_access", "case_type": "group_access",
                "target": {"model_id": model_id, "group_name": group_name},
                "reasons": ["legacy terminal status requires an immutable M8 receipt"],
                "source": "legacy_terminal_status_gate", "status": "planned",
            })
        for client_name, protocols in matrix.get("clients", {}).items():
            if not isinstance(protocols, dict):
                continue
            for protocol, os_rows in protocols.items():
                if not isinstance(os_rows, dict):
                    continue
                for os_id, cell in os_rows.items():
                    if not isinstance(cell, dict) or cell.get("status") not in {"verified", "unsupported"}:
                        continue
                    key = f"test_evidence:{model_id}/clients/{client_name}/{protocol}/{os_id}"
                    cases.setdefault(key, {
                        "case_key": key, "matrix": "test_evidence", "case_type": "real_client_loop",
                        "target": {"model_id": model_id, "client_id": client_name, "client_version": cell.get("client_version"), "protocol": protocol, "os": os_id},
                        "reasons": ["legacy terminal status requires an immutable M8 receipt"],
                        "source": "legacy_terminal_status_gate", "status": "planned",
                    })

    raw_pricing = pricing_inventory or {}
    if isinstance(raw_pricing, dict) and isinstance(raw_pricing.get("data"), dict):
        raw_pricing = raw_pricing["data"]
    group_ids = {
        group.get("name"): str(group.get("group_id"))
        for group in raw_pricing.get("groups", []) if isinstance(group, dict)
        and isinstance(group.get("name"), str) and group.get("group_id") is not None
    } if isinstance(raw_pricing, dict) else {}
    for row in cases.values():
        if row["case_type"] == "group_access":
            group_id = group_ids.get(row["target"].get("group_name"))
            if group_id is not None:
                row["target"]["group_id"] = group_id
            row["required_features"] = ["model_discovery", "route_call"]

    enrich_semantic_targets(cases, client_matrix)
    cases, dedupe = semantic_dedupe(cases)
    cases, shortest_path = apply_shortest_path_policy(cases, client_matrix)

    evidence_rows = evidence_inventory.get("evidence", [])
    receipts = [
        item for item in (terminal_evidence or {}).get("rows", [])
        if item.get("secret_free") is True and receipt_artifact_valid(item)
    ]
    for row in cases.values():
        matches = [item for item in evidence_rows if evidence_matches(row["target"], item)]
        row["candidate_evidence_ids"] = sorted({item["evidence_id"] for item in matches})
        legacy_terminal_ids = {
            item["evidence_id"] for item in matches
            if item.get("proof", {}).get("reusable_as_terminal_evidence") is True
        }
        matched_receipts = [item for item in receipts if receipt_matches(row, item)]
        row["terminal_evidence_ids"] = sorted(legacy_terminal_ids | {item["evidence_id"] for item in matched_receipts})
        # Client protocol support is existential across models: one exact
        # client/version/protocol loop proves the transport, while a failure on
        # another model remains model-specific and must not revoke that client
        # capability. Model-specific cases continue to use latest-result wins.
        if row["case_type"] == "client_protocol_runtime" and any(
            item.get("result") == "pass" for item in matched_receipts
        ):
            latest_receipts = [item for item in matched_receipts if item.get("result") == "pass"]
        else:
            latest_receipts = latest_receipts_by_feature(matched_receipts)
        row["latest_terminal_evidence_ids"] = sorted({item["evidence_id"] for item in latest_receipts})
        terminal_results = sorted({item.get("result") for item in latest_receipts})
        row["terminal_results"] = terminal_results
        if "fail" in terminal_results:
            row["status"] = "terminal_fail_review"
        elif "unsupported" in terminal_results:
            row["status"] = "satisfied"
        elif row.get("required_features"):
            passed_features = {item.get("target", {}).get("feature") for item in matched_receipts if item.get("result") == "pass"}
            if set(row["required_features"]).issubset(passed_features):
                row["status"] = "satisfied"
            elif matched_receipts:
                row["status"] = "partial_evidence_review"
        elif "pass" in terminal_results or "unsupported" in terminal_results or legacy_terminal_ids:
            row["status"] = "satisfied" if ({"pass", "unsupported"} & set(terminal_results)) else "evidence_reuse_review"
        row["batch_key"] = execution_batch_key(row)

    queue = sorted(cases.values(), key=lambda row: row["case_key"])
    merged_case_keys = {
        case_key for row in queue for case_key in row.get("merged_case_keys", [row["case_key"]])
    }
    unmapped_actionable = [
        record for record in actionable_gap_records if record["case_key"] not in merged_case_keys
    ]
    semantic_keys = [row["semantic_key"] for row in queue]
    config_targets = [
        (
            slug(row["target"].get("client_id")),
            row["target"].get("client_version"),
            row["target"].get("os"),
        )
        for row in queue if row["case_type"] == "config_qa"
    ]
    coverage_audit = {
        "raw_failure_count": raw_failures,
        "derived_failure_count": len(derived),
        "actionable_failure_count": len(actionable_gap_records),
        "actionable_case_key_count": len({record["case_key"] for record in actionable_gap_records}),
        "covered_actionable_failure_count": len(actionable_gap_records) - len(unmapped_actionable),
        "unmapped_actionable_failures": unmapped_actionable,
        "semantic_target_count": len(semantic_keys),
        "unique_semantic_target_count": len(set(semantic_keys)),
        "semantic_duplicates_remaining": len(semantic_keys) - len(set(semantic_keys)),
        "config_qa_target_count": len(config_targets),
        "unique_config_qa_target_count": len(set(config_targets)),
        "complete": not unmapped_actionable and len(semantic_keys) == len(set(semantic_keys)) and len(config_targets) == len(set(config_targets)),
    }
    batches: dict[str, list[str]] = {}
    for row in queue:
        if row["status"] == "satisfied":
            continue
        batches.setdefault(row["batch_key"], []).append(row["case_key"])
    digest_payload = json.dumps(queue, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
    return {
        "schema_version": 1,
        "kind": "model_client_unique_test_queue",
        "complete": False,
        "network_execution": "disabled",
        "raw_audit_failures": raw_failures,
        "derived_failures": derived,
        "derived_failure_count": len(derived),
        "queue_count": len(queue),
        "execution_batch_count": len(batches),
        "semantic_dedupe": dedupe,
        "shortest_path_policy": shortest_path,
        "coverage_audit": coverage_audit,
        "counts_by_matrix": dict(sorted(Counter(row["matrix"] for row in queue).items())),
        "counts_by_case_type": dict(sorted(Counter(row["case_type"] for row in queue).items())),
        "terminal_reuse_count": sum(bool(row["terminal_evidence_ids"]) for row in queue),
        "satisfied_case_count": sum(row["status"] == "satisfied" for row in queue),
        "terminal_fail_case_count": sum(row["status"] == "terminal_fail_review" for row in queue),
        "partial_evidence_case_count": sum(row["status"] == "partial_evidence_review" for row in queue),
        "candidate_evidence_links": sum(len(row["candidate_evidence_ids"]) for row in queue),
        "queue_sha256": hashlib.sha256(digest_payload).hexdigest(),
        "execution_batches": [
            {"batch_key": key, "case_count": len(case_keys), "case_keys": case_keys}
            for key, case_keys in sorted(batches.items())
        ],
        "queue": queue,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gaps", type=Path, required=True)
    parser.add_argument("--client-matrix", type=Path, required=True)
    parser.add_argument("--scope", type=Path, required=True)
    parser.add_argument("--evidence-inventory", type=Path, required=True)
    parser.add_argument("--contracts", type=Path, required=True)
    parser.add_argument("--terminal-evidence", type=Path, required=True)
    parser.add_argument("--pricing-inventory", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    contract_rows = [
        load(path) for path in sorted(args.contracts.glob("*.json"))
        if path.name not in {"client-matrix.json", "matrix-schema.json"}
    ]
    report = build_queue(
        load(args.gaps), load(args.client_matrix), load(args.scope), load(args.evidence_inventory),
        contract_rows, load(args.terminal_evidence), load(args.pricing_inventory),
    )
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({key: report[key] for key in (
        "raw_audit_failures", "derived_failure_count", "queue_count", "execution_batch_count", "counts_by_matrix",
        "terminal_reuse_count", "satisfied_case_count", "terminal_fail_case_count", "partial_evidence_case_count", "candidate_evidence_links", "queue_sha256",
        "semantic_dedupe",
        "coverage_audit",
    )}, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

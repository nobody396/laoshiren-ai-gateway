#!/usr/bin/env python3
"""Project exact immutable M8 receipts into model contracts.

For group access, discovery and callability remain separate.  A group/model
cell is verified only when discovery passed and every protocol declared by the
cell has both a passing ``route_call`` and ``minimal_text`` receipt.  Per-
protocol pass/fail history is retained under ``protocol_evidence``; an HTTP 200
empty-terminal failure therefore remains visible and blocks that protocol.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import tempfile
from typing import Any


SHA256_RE = re.compile(r"^[a-f0-9]{64}$")
BASE_PROTOCOL_FEATURES = (
    "minimal_text", "streaming_terminal", "tool_call",
    "tool_result_continuation", "usage", "invalid_request",
)
SECRET_PATTERNS = (
    re.compile(r"-----BEGIN [^-]*PRIVATE KEY-----", re.I),
    re.compile(r"\bBearer\s+[A-Za-z0-9._~+/-]{8,}={0,2}", re.I),
    re.compile(r"\b(?:sk|rk|pk)-[A-Za-z0-9_-]{8,}\b", re.I),
    re.compile(
        r'''(?ix)
        ["']?(?:api[_-]?key|access[_-]?token|refresh[_-]?token|auth[_-]?token|
        password|client[_-]?secret)["']?\s*[:=]\s*["']?(?!null\b|none\b|\[redacted\])
        [A-Za-z0-9._~+/-]{8,}
        '''
    ),
)


def load(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text())
    if not isinstance(value, dict):
        raise ValueError(f"{path} must contain an object")
    return value


def unwrap(value: dict[str, Any]) -> dict[str, Any]:
    return value.get("data") if isinstance(value.get("data"), dict) else value


def group_map(pricing: dict[str, Any]) -> dict[str, str]:
    pricing = unwrap(pricing)
    return {
        item["name"]: str(item["group_id"])
        for item in pricing.get("groups", [])
        if isinstance(item, dict) and isinstance(item.get("name"), str) and item.get("group_id") is not None
    }


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def stable_evidence_id(kind: str, target: dict[str, Any], artifact_sha256: str) -> str:
    payload = json.dumps(
        {"kind": kind, "target": target, "artifact": artifact_sha256},
        sort_keys=True, separators=(",", ":"),
    ).encode()
    return f"evidence-{hashlib.sha256(payload).hexdigest()[:24]}"


def assert_secret_free(value: str | bytes, *, label: str) -> None:
    text = value.decode("utf-8", errors="replace") if isinstance(value, bytes) else value
    if any(pattern.search(text) for pattern in SECRET_PATTERNS):
        raise ValueError(f"secret-shaped value found in {label}")


def receipt_rows(
    index: dict[str, Any], *, artifact_root: Path | None = None,
    verify_artifacts: bool = False,
) -> list[dict[str, Any]]:
    rows = [
        item for item in index.get("rows", [])
        if isinstance(item, dict) and item.get("secret_free") is True
    ]
    seen: set[str] = set()
    for item in rows:
        evidence_id = item.get("evidence_id")
        kind, target, digest = item.get("evidence_type"), item.get("target"), item.get("artifact_sha256")
        if not isinstance(evidence_id, str) or evidence_id in seen:
            raise ValueError("M8 evidence IDs must be present and unique")
        seen.add(evidence_id)
        if not isinstance(kind, str) or not isinstance(target, dict):
            raise ValueError(f"{evidence_id}: evidence_type and target are required")
        if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
            raise ValueError(f"{evidence_id}: invalid artifact_sha256")
        # Evidence producers may use either the canonical target/artifact hash
        # below or another stable content hash.  Uniqueness plus merge conflict
        # detection keeps either form immutable; artifact bytes are verified
        # independently below.  Receipts created by this toolchain use
        # stable_evidence_id(kind, target, digest).
        assert_secret_free(json.dumps(item, ensure_ascii=False), label=evidence_id)
        if verify_artifacts:
            uri = item.get("artifact_uri")
            if not isinstance(uri, str) or not uri:
                raise ValueError(f"{evidence_id}: artifact_uri is required")
            path = Path(uri)
            if not path.is_absolute():
                path = (artifact_root or Path.cwd()) / path
            if not path.is_file():
                raise ValueError(f"{evidence_id}: artifact is missing: {path}")
            raw = path.read_bytes()
            assert_secret_free(raw, label=str(path))
            if file_sha256(path) != digest:
                raise ValueError(f"{evidence_id}: artifact SHA-256 mismatch")
    for item in rows:
        refs = item.get("evidence_refs", [])
        if not isinstance(refs, list) or any(not isinstance(value, str) for value in refs):
            raise ValueError(f"{item['evidence_id']}: evidence_refs must be an array of IDs")
        missing = sorted(set(refs) - seen)
        if missing:
            raise ValueError(f"{item['evidence_id']}: unresolved evidence_refs: {', '.join(missing)}")
        if item["evidence_id"] in refs:
            raise ValueError(f"{item['evidence_id']}: evidence receipt cannot reference itself")
    return rows


def matching(rows: list[dict[str, Any]], **target: str) -> list[dict[str, Any]]:
    output = []
    for row in rows:
        observed = row.get("target")
        if not isinstance(observed, dict):
            continue
        if all(observed.get(key) == value for key, value in target.items()):
            output.append(row)
    return output


def evidence_ids(rows: list[dict[str, Any]]) -> list[str]:
    return sorted({row["evidence_id"] for row in rows if isinstance(row.get("evidence_id"), str)})


def latest_feature_state(rows: list[dict[str, Any]], feature: str) -> dict[str, Any]:
    candidates = [
        row for row in rows
        if row.get("target", {}).get("feature") == feature
        and row.get("result") in {"pass", "fail", "blocked", "unsupported"}
    ]
    if not candidates:
        return {
            "status": "blocked", "observed_at": None,
            "evidence_ids": [], "pass_evidence_ids": [], "failure_evidence_ids": [],
        }
    def observed_key(row: dict[str, Any]) -> tuple[float, str]:
        raw = str(row.get("observed_at", ""))
        try:
            parsed = datetime.fromisoformat(raw.replace("Z", "+00:00"))
            if parsed.tzinfo is None:
                parsed = parsed.replace(tzinfo=timezone.utc)
            stamp = parsed.astimezone(timezone.utc).timestamp()
        except ValueError:
            stamp = float("-inf")
        return stamp, str(row.get("evidence_id", ""))

    candidates.sort(key=observed_key)
    latest_stamp = observed_key(candidates[-1])[0]
    latest = [row for row in candidates if observed_key(row)[0] == latest_stamp]
    latest_at = str(candidates[-1].get("observed_at", ""))
    latest_results = {row.get("result") for row in latest}
    if latest_results == {"pass"}:
        status = "verified"
    elif latest_results == {"unsupported"}:
        status = "unsupported"
    else:
        status = "blocked"
    return {
        "status": status,
        "observed_at": latest_at or None,
        "evidence_ids": evidence_ids(candidates),
        "pass_evidence_ids": evidence_ids([row for row in candidates if row.get("result") == "pass"]),
        "failure_evidence_ids": evidence_ids([row for row in candidates if row.get("result") in {"fail", "blocked", "unsupported"}]),
    }


def project_group_cell(
    cell: dict[str, Any], receipts: list[dict[str, Any]], protocols: list[str]
) -> tuple[bool, str, list[str]]:
    before = json.dumps(cell, ensure_ascii=False, sort_keys=True)
    discovery = latest_feature_state(receipts, "model_discovery")
    protocol_evidence: dict[str, Any] = {}
    for protocol in sorted(set(protocols)):
        exact = [
            row for row in receipts
            if row.get("target", {}).get("protocol") == protocol
        ]
        route = latest_feature_state(exact, "route_call")
        minimal = latest_feature_state(exact, "minimal_text")
        status = "blocked"
        if discovery["status"] == route["status"] == minimal["status"] == "verified":
            status = "verified"
        elif "unsupported" in {discovery["status"], route["status"], minimal["status"]}:
            # A visible model with a terminally unsupported route/minimal call
            # is not usable in this group. Discovery alone can never keep the
            # group in a publishable or ambiguous state.
            status = "unsupported"
        ids = sorted(set(discovery["evidence_ids"] + route["evidence_ids"] + minimal["evidence_ids"]))
        protocol_evidence[protocol] = {
            "status": status,
            "required_features": ["model_discovery", "route_call", "minimal_text"],
            "evidence_ids": ids,
            "pass_evidence_ids": sorted(set(
                discovery["pass_evidence_ids"] + route["pass_evidence_ids"] + minimal["pass_evidence_ids"]
            )),
            "failure_evidence_ids": sorted(set(
                discovery["failure_evidence_ids"] + route["failure_evidence_ids"] + minimal["failure_evidence_ids"]
            )),
            "observed_at": max(
                (value for value in (discovery["observed_at"], route["observed_at"], minimal["observed_at"]) if value),
                default=None,
            ),
        }
    status = "blocked"
    if protocol_evidence and all(row["status"] == "verified" for row in protocol_evidence.values()):
        status = "verified"
    elif protocol_evidence and all(row["status"] == "unsupported" for row in protocol_evidence.values()):
        status = "unsupported"
    ids = sorted({
        evidence_id
        for row in protocol_evidence.values()
        for evidence_id in row["evidence_ids"]
    })
    cell["status"] = status
    cell["protocol_evidence"] = protocol_evidence
    cell["evidence_ids"] = ids
    cell["evidence"] = (
        "M8 group receipts by protocol: "
        + ", ".join(f"{protocol}={row['status']}" for protocol, row in protocol_evidence.items())
    )
    observed = max(
        (row["observed_at"] for row in protocol_evidence.values() if row.get("observed_at")),
        default=None,
    )
    if observed:
        cell["verified_at"] = observed[:10]
    after = json.dumps(cell, ensure_ascii=False, sort_keys=True)
    return before != after, status, ids


def exact_client_version_matches(
    receipt_version: Any, cell_version: Any, canonical: dict[str, Any]
) -> bool:
    if not isinstance(receipt_version, str) or not isinstance(cell_version, str):
        return False
    if receipt_version == cell_version:
        return True
    release = canonical.get("client_config_os", {}).get("release", {})
    # Some raw client receipts record the displayed CLI version while the
    # matrix uses its typed version key (for example 2.1.251 vs cli:2.1.251).
    # This is an exact canonical equivalence, not a range or fuzzy match.
    return (
        isinstance(release, dict)
        and receipt_version == release.get("display")
        and cell_version == release.get("version_key")
    )


def project_client_cell(cell: dict[str, Any], receipts: list[dict[str, Any]]) -> tuple[bool, str]:
    before = json.dumps(cell, ensure_ascii=False, sort_keys=True)
    state = latest_feature_state(receipts, "agent_loop")
    cell["status"] = state["status"]
    linked = sorted({
        evidence_id
        for receipt in receipts
        for evidence_id in [receipt.get("evidence_id"), *(receipt.get("evidence_refs") or [])]
        if isinstance(evidence_id, str)
    })
    cell["evidence_ids"] = linked
    cell["pass_evidence_ids"] = state["pass_evidence_ids"]
    cell["failure_evidence_ids"] = state["failure_evidence_ids"]
    cell["evidence"] = "M8 exact or matrix-intersection receipts: " + ", ".join(linked)
    if state["observed_at"]:
        cell["verified_at"] = state["observed_at"][:10]
    after = json.dumps(cell, ensure_ascii=False, sort_keys=True)
    return before != after, state["status"]


def update_cell(cell: dict[str, Any], receipts: list[dict[str, Any]], *, required_features: set[str] | None = None) -> tuple[bool, str | None]:
    if not receipts:
        return False, None
    before = json.dumps(cell, ensure_ascii=False, sort_keys=True)
    features = required_features or {
        row.get("target", {}).get("feature")
        for row in receipts if isinstance(row.get("target", {}).get("feature"), str)
    }
    states = [latest_feature_state(receipts, feature) for feature in sorted(features)]
    if states and all(state["status"] == "verified" for state in states):
        target_status = "verified"
    elif states and all(state["status"] == "unsupported" for state in states):
        target_status = "unsupported"
    else:
        target_status = "blocked"
    # Current exact evidence wins. A later negative receipt must not leave a
    # legacy verified cell green; the failure remains blocked (never silently
    # converted to unsupported) until a newer pass or an explicit terminal
    # negative contract is recorded.
    if cell.get("status") in {"not_published", "not_exposed", "not_applicable"}:
        target_status = cell["status"]
    cell["status"] = target_status
    cell["evidence_ids"] = evidence_ids(receipts)
    cell["pass_evidence_ids"] = evidence_ids([row for row in receipts if row.get("result") == "pass"])
    cell["failure_evidence_ids"] = evidence_ids([row for row in receipts if row.get("result") in {"fail", "blocked", "unsupported"}])
    cell["evidence"] = "M8 receipts: " + ", ".join(cell["evidence_ids"])
    observed = max((state.get("observed_at") for state in states if state.get("observed_at")), default=None)
    if observed:
        cell["verified_at"] = observed[:10]
    after = json.dumps(cell, ensure_ascii=False, sort_keys=True)
    return before != after, target_status


def project_contract(
    contract: dict[str, Any], rows: list[dict[str, Any]], groups: dict[str, str],
    clients: dict[str, dict[str, Any]] | None = None,
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    result = json.loads(json.dumps(contract))
    model_id = result.get("model", {}).get("id")
    events = []
    matrix = result.get("test_matrix", {})
    protocol_matrix = matrix.setdefault("protocols", {}) if isinstance(matrix, dict) else {}
    declared_protocol_names = {
        row.get("name")
        for row in result.get("protocols", [])
        if isinstance(row, dict) and isinstance(row.get("name"), str)
    }
    receipt_protocols = {
        row.get("target", {}).get("protocol")
        for row in rows
        if row.get("target", {}).get("model_id") == model_id
        and isinstance(row.get("target", {}).get("protocol"), str)
    }
    for protocol in sorted(declared_protocol_names.intersection(receipt_protocols)):
        checks = protocol_matrix.setdefault(protocol, {})
        if isinstance(checks, dict):
            for feature in BASE_PROTOCOL_FEATURES:
                checks.setdefault(feature, {
                    "status": "blocked",
                    "evidence": "No exact terminal M8 receipt has been projected for this protocol check.",
                })
    for group_name, cell in matrix.get("group_access", {}).items():
        group_id = groups.get(group_name)
        if not group_id or not isinstance(cell, dict):
            continue
        receipts = matching(rows, model_id=model_id, group_id=group_id)
        receipts = [
            row for row in receipts
            if row.get("target", {}).get("feature") in {"model_discovery", "route_call", "minimal_text"}
        ]
        if not receipts:
            continue
        declared_protocols = [
            protocol for protocol in cell.get("protocols", [])
            if isinstance(protocol, str)
        ]
        changed, status, ids = project_group_cell(cell, receipts, declared_protocols)
        if changed:
            events.append({
                "model_id": model_id, "matrix": "group_access", "path": group_name,
                "status": status, "evidence_ids": ids,
                "protocol_status": {
                    protocol: value["status"]
                    for protocol, value in cell.get("protocol_evidence", {}).items()
                },
            })
    for protocol, checks in matrix.get("protocols", {}).items():
        for feature, cell in checks.items():
            if not isinstance(cell, dict):
                continue
            receipts = matching(rows, model_id=model_id, protocol=protocol, feature=feature)
            changed, status = update_cell(cell, receipts)
            if changed:
                events.append({"model_id": model_id, "matrix": "model_protocol", "path": f"{protocol}/{feature}", "status": status, "evidence_ids": evidence_ids(receipts)})
    for protocol, checks in matrix.get("protocol_features", {}).items():
        for feature, cell in checks.items():
            if not isinstance(cell, dict):
                continue
            receipts = matching(rows, model_id=model_id, protocol=protocol, feature=feature)
            changed, status = update_cell(cell, receipts)
            if changed:
                events.append({"model_id": model_id, "matrix": "model_protocol", "path": f"{protocol}/features/{feature}", "status": status, "evidence_ids": evidence_ids(receipts)})
    for protocol, checks in matrix.get("tools", {}).items():
        for feature, cell in checks.items():
            if not isinstance(cell, dict):
                continue
            receipts = matching(rows, model_id=model_id, protocol=protocol, feature=feature)
            changed, status = update_cell(cell, receipts)
            if changed:
                events.append({"model_id": model_id, "matrix": "model_protocol", "path": f"{protocol}/tools/{feature}", "status": status, "evidence_ids": evidence_ids(receipts)})
    reasoning_levels = matrix.get("reasoning", {}).get("model_levels", {})
    if isinstance(reasoning_levels, dict):
        for level, cell in reasoning_levels.items():
            if not isinstance(cell, dict):
                continue
            receipts = [
                row for row in rows
                if row.get("target", {}).get("model_id") == model_id
                and row.get("target", {}).get("feature") == "reasoning_level"
                and row.get("target", {}).get("level") == level
                and row.get("result") in {"pass", "unsupported"}
            ]
            changed, status = update_cell(cell, receipts)
            if changed:
                events.append({"model_id": model_id, "matrix": "model_reasoning", "path": f"reasoning/model/{level}", "status": status, "evidence_ids": evidence_ids(receipts)})
    protocol_rows = {
        row.get("name"): row
        for row in result.get("protocols", [])
        if isinstance(row, dict) and isinstance(row.get("name"), str)
    }
    for protocol, checks in matrix.get("protocols", {}).items():
        row = protocol_rows.get(protocol)
        if not isinstance(row, dict) or not isinstance(checks, dict) or row.get("status") != "blocked":
            continue
        statuses = {name: cell.get("status") for name, cell in checks.items() if isinstance(cell, dict)}
        if statuses.get("minimal_text") == "verified" and statuses and all(
            status in {"verified", "unsupported"} for status in statuses.values()
        ):
            before = json.dumps(row, ensure_ascii=False, sort_keys=True)
            ids = sorted({
                evidence_id
                for cell in checks.values() if isinstance(cell, dict)
                for evidence_id in cell.get("evidence_ids", [])
                if isinstance(evidence_id, str)
            })
            row.update({
                "status": "verified",
                "evidence": "M8 base protocol checks are terminal; unsupported sub-capabilities remain explicit: "
                    + ", ".join(f"{name}={status}" for name, status in sorted(statuses.items())),
                "evidence_ids": ids,
            })
            if before != json.dumps(row, ensure_ascii=False, sort_keys=True):
                events.append({"model_id": model_id, "matrix": "model_protocol", "path": f"protocols/{protocol}", "status": "verified", "evidence_ids": ids})
    canonical_clients = clients or {}
    for client_name, protocol_rows in matrix.get("clients", {}).items():
        canonical = canonical_clients.get(client_name)
        if not canonical or not isinstance(protocol_rows, dict):
            continue
        supported_protocols = [
            row.get("protocol") for row in canonical.get("client_protocol", {}).get("protocols", [])
            if row.get("support") == "supported"
        ]
        if not set(protocol_rows).intersection(supported_protocols):
            coverage = result.setdefault("client_coverage", [])
            coverage_row = next((item for item in coverage if item.get("name") == client_name), None)
            if coverage_row is None:
                coverage_row = {"name": client_name}
                coverage.append(coverage_row)
            before_coverage = json.dumps(coverage_row, ensure_ascii=False, sort_keys=True)
            coverage_row.update({
                "status": "unsupported", "protocols": [], "evidence_ids": [],
                "evidence": "No supported canonical client protocol remains for this model.",
            })
            result["clients"] = [item for item in result.setdefault("clients", []) if item.get("name") != client_name]
            if before_coverage != json.dumps(coverage_row, ensure_ascii=False, sort_keys=True):
                events.append({"model_id": model_id, "matrix": "test_evidence", "path": f"{client_name}/coverage", "status": "unsupported", "evidence_ids": []})
            continue
        matched_client_receipts: list[dict[str, Any]] = []
        for protocol, os_rows in protocol_rows.items():
            if not isinstance(os_rows, dict):
                continue
            for os_id, cell in os_rows.items():
                if not isinstance(cell, dict):
                    continue
                receipts = [
                    row for row in rows
                    if row.get("evidence_type") in {"real_client_loop", "matrix_intersection"}
                    and row.get("target", {}).get("model_id") == model_id
                    and str(row.get("target", {}).get("client_id", "")).casefold() == str(canonical.get("id", "")).casefold()
                    and exact_client_version_matches(
                        row.get("target", {}).get("client_version"),
                        cell.get("client_version"), canonical,
                    )
                    and row.get("target", {}).get("protocol") == protocol
                    and row.get("target", {}).get("os") == os_id
                ]
                matched_client_receipts.extend(receipts)
                changed, status = project_client_cell(cell, receipts) if receipts else (False, None)
                if changed:
                    events.append({"model_id": model_id, "matrix": "test_evidence", "path": f"{client_name}/{protocol}/{os_id}", "status": status, "evidence_ids": evidence_ids(receipts)})

        # No exact terminal receipt for this client/model means this projector
        # has no authority to rewrite a manually prepared blocked coverage row
        # or its reasoning metadata.
        if not matched_client_receipts:
            continue

        verification_os = canonical.get("verification_os") or []
        verified_protocols = []
        client_receipts = []
        for protocol in supported_protocols:
            cells = protocol_rows.get(protocol, {})
            required_cells = [cells.get(os_id) for os_id in verification_os]
            if required_cells and all(
                isinstance(cell, dict) and cell.get("status") == "verified" and bool(cell.get("evidence_ids"))
                for cell in required_cells
            ):
                verified_protocols.append(protocol)
                for cell in required_cells:
                    client_receipts.extend(cell.get("evidence_ids") or [])
        coverage = result.setdefault("client_coverage", [])
        coverage_row = next((item for item in coverage if item.get("name") == client_name), None)
        if coverage_row is None:
            coverage_row = {"name": client_name, "status": "blocked", "protocols": [], "evidence": "No exact terminal receipt yet."}
            coverage.append(coverage_row)
        before_coverage = json.dumps(coverage_row, ensure_ascii=False, sort_keys=True)
        coverage_row["status"] = "verified" if verified_protocols else "unsupported"
        coverage_row["protocols"] = verified_protocols
        coverage_row["evidence_ids"] = sorted(set(client_receipts))
        coverage_row["evidence"] = "M8 receipts: " + ", ".join(coverage_row["evidence_ids"]) if client_receipts else "No exact terminal receipt yet."
        if before_coverage != json.dumps(coverage_row, ensure_ascii=False, sort_keys=True):
            events.append({"model_id": model_id, "matrix": "test_evidence", "path": f"{client_name}/coverage", "status": coverage_row["status"], "evidence_ids": coverage_row["evidence_ids"]})
        if verified_protocols:
            public_clients = result.setdefault("clients", [])
            public_row = next((item for item in public_clients if item.get("name") == client_name), None)
            if public_row is None:
                public_row = {"name": client_name, "recommended": False}
                public_clients.append(public_row)
            public_row.update({
                "version": canonical.get("client_config_os", {}).get("release", {}).get("display"),
                "protocol": verified_protocols[0], "status": "verified",
                "evidence_ids": sorted(set(client_receipts)),
                "evidence": "M8 receipts: " + ", ".join(sorted(set(client_receipts))),
            })
        else:
            result["clients"] = [item for item in result.setdefault("clients", []) if item.get("name") != client_name]
    if not result.get("clients"):
        result.setdefault("verification", {})["gateway_e2e"] = False
    return result, events


def atomic_write(path: Path, value: dict[str, Any], backup_dir: Path) -> None:
    backup_dir.mkdir(parents=True, exist_ok=True)
    backup = backup_dir / f"{path.name}.bak"
    if not backup.exists():
        shutil.copy2(path, backup)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as handle:
            json.dump(value, handle, ensure_ascii=False, indent=2)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("plan", "apply"))
    parser.add_argument("--contracts", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--pricing-inventory", type=Path, required=True)
    parser.add_argument("--client-matrix", type=Path, required=True)
    parser.add_argument(
        "--artifact-root", type=Path, default=Path.cwd(),
        help="root used to resolve relative artifact_uri values for SHA/secret verification",
    )
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    rows = receipt_rows(
        load(args.evidence), artifact_root=args.artifact_root, verify_artifacts=True,
    )
    groups = group_map(load(args.pricing_inventory))
    client_rows = load(args.client_matrix).get("clients", [])
    clients = {item.get("name"): item for item in client_rows if isinstance(item, dict) and isinstance(item.get("name"), str)}
    changes = []
    projected = []
    for path in sorted(args.contracts.glob("*.json")):
        if path.name in {"client-matrix.json", "matrix-schema.json"}:
            continue
        original = load(path)
        after, events = project_contract(original, rows, groups, clients)
        if after != original:
            changes.append({"path": str(path), "events": events})
        projected.append((path, original, after))
    group_events = [
        event for item in changes for event in item["events"]
        if event.get("matrix") == "group_access"
    ]
    report = {
        "schema_version": 1, "kind": "model_evidence_projection", "command": args.command,
        "production_write": False, "contract_write": args.command == "apply",
        "contracts_changed": len(changes), "events": sum(len(item["events"]) for item in changes),
        "group_access": {
            "changed": len(group_events),
            "verified": sum(event.get("status") == "verified" for event in group_events),
            "blocked": sum(event.get("status") == "blocked" for event in group_events),
        },
        "evidence_index_sha256": file_sha256(args.evidence),
        "changes": changes,
    }
    assert_secret_free(json.dumps(report, ensure_ascii=False), label="projection report")
    if args.command == "apply":
        backup_dir = args.contracts / ".model-evidence-apply-backups"
        for path, original, after in projected:
            if after != original:
                atomic_write(path, after, backup_dir)
    if args.report:
        args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({key: report[key] for key in ("command", "contracts_changed", "events", "group_access")}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

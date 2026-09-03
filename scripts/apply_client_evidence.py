#!/usr/bin/env python3
"""Project exact M8 receipts into canonical client capability matrices.

Only immutable, secret-free real-client, config-QA, or regression receipts may
upgrade M4 client_protocol, M5 client_reasoning, or M7 client_config_os.  The
projector never runs a client, contacts a provider, or reads a credential.
"""

from __future__ import annotations

import argparse
from copy import deepcopy
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import tempfile
from typing import Any, Iterable


SHA256_RE = re.compile(r"^[a-f0-9]{64}$")
M4_EVIDENCE_TYPES = {"real_client_loop", "real_client_protocol", "regression_test"}
M5_EVIDENCE_TYPES = {"real_client_loop", "regression_test"}
M7_EVIDENCE_TYPES = {"config_qa", "regression_test"}
CONFIG_FEATURES = {"config_safe_write", "config_backup_restore", "config_idempotency"}
REASONING_FEATURES = {"reasoning_transport", "reasoning_level"}
PRESENCE_FEATURES = {"agent_loop", "protocol_presence"}
NEGATIVE_RESULTS = {"fail", "blocked", "expired", "unsupported"}
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
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"{path} must contain an object")
    return value


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def assert_secret_free(value: str | bytes, *, label: str) -> None:
    text = value.decode("utf-8", errors="replace") if isinstance(value, bytes) else value
    if any(pattern.search(text) for pattern in SECRET_PATTERNS):
        raise ValueError(f"secret-shaped value found in {label}")


def parse_observed(value: Any, path: str) -> datetime:
    if not isinstance(value, str) or not value:
        raise ValueError(f"{path}: observed_at is required")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ValueError(f"{path}: observed_at must be ISO-8601") from exc
    if parsed.tzinfo is None:
        raise ValueError(f"{path}: observed_at must include a timezone")
    return parsed.astimezone(timezone.utc)


def receipt_rows(
    evidence: dict[str, Any], *, artifact_root: Path, verify_artifacts: bool,
) -> list[dict[str, Any]]:
    if evidence.get("kind") != "test_evidence_matrix":
        raise ValueError("evidence.kind must be test_evidence_matrix")
    raw_rows = evidence.get("rows")
    if not isinstance(raw_rows, list):
        raise ValueError("evidence.rows must be an array")
    rows: list[dict[str, Any]] = []
    ids: set[str] = set()
    for index, raw in enumerate(raw_rows):
        path = f"evidence.rows[{index}]"
        if not isinstance(raw, dict):
            raise ValueError(f"{path} must be an object")
        if raw.get("secret_free") is not True:
            raise ValueError(f"{path}: secret_free must be true")
        evidence_id = raw.get("evidence_id")
        if not isinstance(evidence_id, str) or not evidence_id or evidence_id in ids:
            raise ValueError(f"{path}: evidence_id must be unique")
        ids.add(evidence_id)
        if raw.get("evidence_type") not in {
            "official_spec", "live_protocol_probe", "real_client_loop", "real_client_protocol",
            "regression_test", "config_qa", "billing_reconciliation", "matrix_intersection",
        }:
            raise ValueError(f"{evidence_id}: invalid evidence_type")
        target = raw.get("target")
        if not isinstance(target, dict):
            raise ValueError(f"{evidence_id}: target is required")
        if raw.get("result") not in {"pass", "fail", "blocked", "expired", "unsupported"}:
            raise ValueError(f"{evidence_id}: invalid result")
        evidence_type = raw.get("evidence_type")
        client_receipt = (
            evidence_type in {"real_client_loop", "real_client_protocol", "config_qa"}
            or (evidence_type == "regression_test" and target.get("client_id") is not None)
        )
        if client_receipt:
            for field in ("client_id", "client_version", "os", "feature"):
                if not isinstance(target.get(field), str) or not target[field]:
                    raise ValueError(f"{evidence_id}: exact client receipt requires target.{field}")
            feature = target.get("feature")
            if evidence_type in {"real_client_loop", "real_client_protocol"} and not isinstance(target.get("protocol"), str):
                raise ValueError(f"{evidence_id}: real client receipt requires target.protocol")
            if evidence_type == "regression_test" and feature not in CONFIG_FEATURES:
                if not isinstance(target.get("protocol"), str):
                    raise ValueError(f"{evidence_id}: client regression requires target.protocol")
        parse_observed(raw.get("observed_at"), evidence_id)
        digest = raw.get("artifact_sha256")
        if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
            raise ValueError(f"{evidence_id}: artifact_sha256 must be lowercase SHA-256")
        uri = raw.get("artifact_uri")
        if not isinstance(uri, str) or not uri:
            raise ValueError(f"{evidence_id}: artifact_uri is required")
        refs = raw.get("evidence_refs", [])
        if not isinstance(refs, list) or any(not isinstance(ref, str) for ref in refs):
            raise ValueError(f"{evidence_id}: evidence_refs must be an array of IDs")
        versions = raw.get("versions", {})
        if not isinstance(versions, dict):
            raise ValueError(f"{evidence_id}: versions must be an object")
        if target.get("client_version") is not None and versions.get("client") is not None:
            if target["client_version"] != versions["client"]:
                raise ValueError(f"{evidence_id}: target and versions client disagree")
        if target.get("os") is not None and versions.get("os") is not None:
            if target["os"] != versions["os"]:
                raise ValueError(f"{evidence_id}: target and versions OS disagree")
        assert_secret_free(json.dumps(raw, ensure_ascii=False), label=evidence_id)
        if verify_artifacts:
            artifact = Path(uri)
            if not artifact.is_absolute():
                artifact = artifact_root / artifact
            if not artifact.is_file():
                raise ValueError(f"{evidence_id}: artifact is missing: {artifact}")
            payload = artifact.read_bytes()
            assert_secret_free(payload, label=str(artifact))
            if file_sha256(artifact) != digest:
                raise ValueError(f"{evidence_id}: artifact SHA-256 mismatch")
        rows.append(raw)
    for row in rows:
        refs = set(row.get("evidence_refs", []))
        missing = sorted(refs - ids)
        if missing:
            raise ValueError(f"{row['evidence_id']}: unresolved evidence_refs: {', '.join(missing)}")
        if row["evidence_id"] in refs:
            raise ValueError(f"{row['evidence_id']}: receipt cannot reference itself")
    return rows


def protocol_rows(client: dict[str, Any]) -> list[dict[str, Any]]:
    rows = client.get("client_protocol", {}).get("protocols", [])
    return [row for row in rows if isinstance(row, dict)]


def release_versions(client: dict[str, Any]) -> set[str]:
    release = client.get("client_config_os", {}).get("release", {})
    values = {release.get("version_key"), release.get("display")}
    return {value for value in values if isinstance(value, str) and value}


def os_ids(client: dict[str, Any]) -> set[str]:
    return {
        row.get("os") for row in client.get("client_config_os", {}).get("os_support", [])
        if isinstance(row, dict) and isinstance(row.get("os"), str)
    }


def exact_receipts(
    rows: Iterable[dict[str, Any]], client: dict[str, Any], *,
    evidence_types: set[str], protocol: str | None = None, os_id: str | None = None,
    features: set[str] | None = None,
) -> list[dict[str, Any]]:
    client_id = client.get("id")
    versions = release_versions(client)
    allowed_oses = os_ids(client)
    output = []
    for row in rows:
        target = row.get("target", {})
        if row.get("evidence_type") not in evidence_types:
            continue
        if target.get("client_id") != client_id:
            continue
        if target.get("client_version") not in versions:
            continue
        if target.get("os") not in allowed_oses:
            continue
        if protocol is not None and target.get("protocol") != protocol:
            continue
        if os_id is not None and target.get("os") != os_id:
            continue
        if features is not None and target.get("feature") not in features:
            continue
        output.append(row)
    return output


def latest_by_os(rows: Iterable[dict[str, Any]]) -> list[dict[str, Any]]:
    grouped: dict[str, list[dict[str, Any]]] = {}
    for row in rows:
        grouped.setdefault(str(row.get("target", {}).get("os")), []).append(row)
    output = []
    for os_id, candidates in sorted(grouped.items()):
        candidates.sort(key=lambda row: (
            parse_observed(row.get("observed_at"), row["evidence_id"]), row["evidence_id"],
        ))
        latest_time = parse_observed(candidates[-1].get("observed_at"), candidates[-1]["evidence_id"])
        tied = [row for row in candidates if parse_observed(row.get("observed_at"), row["evidence_id"]) == latest_time]
        outcomes = {(row.get("result"), row.get("artifact_sha256")) for row in tied}
        results = {row.get("result") for row in tied}
        if len(results) > 1:
            raise ValueError(f"conflicting latest receipts for OS {os_id}: " + ", ".join(row["evidence_id"] for row in tied))
        # Multiple same-result artifacts are independent corroboration. Pick a
        # stable representative for the single-evidence canonical field.
        output.append(sorted(tied, key=lambda row: (row["evidence_id"], sorted(outcomes)))[-1])
    return output


def latest_by_model_os(rows: Iterable[dict[str, Any]]) -> list[dict[str, Any]]:
    """Keep the current result for each exact model and OS.

    M4 describes whether the client implements a protocol/feature at all. A
    newer failure on another model is model compatibility evidence, not proof
    that the client's previously observed protocol implementation vanished.
    """
    grouped: dict[tuple[str, str], list[dict[str, Any]]] = {}
    for row in rows:
        target = row.get("target", {})
        grouped.setdefault((str(target.get("model_id") or ""), str(target.get("os") or "")), []).append(row)
    output: list[dict[str, Any]] = []
    for (model_id, os_id), candidates in sorted(grouped.items()):
        candidates.sort(key=lambda row: (
            parse_observed(row.get("observed_at"), row["evidence_id"]), row["evidence_id"],
        ))
        latest_time = parse_observed(candidates[-1].get("observed_at"), candidates[-1]["evidence_id"])
        tied = [row for row in candidates if parse_observed(row.get("observed_at"), row["evidence_id"]) == latest_time]
        results = {row.get("result") for row in tied}
        if len(results) > 1:
            raise ValueError(
                f"conflicting latest receipts for model {model_id or '[client]'} OS {os_id}: "
                + ", ".join(row["evidence_id"] for row in tied)
            )
        output.append(sorted(tied, key=lambda row: row["evidence_id"])[-1])
    return output


def evidence_object(row: dict[str, Any], status: str) -> dict[str, Any]:
    return {
        "status": status,
        "observed_at": str(row["observed_at"])[:10],
        "source_ref": f"{row['artifact_uri']}#{row['evidence_id']}",
        "artifact_sha256": row["artifact_sha256"],
    }


def latest_pass(rows: Iterable[dict[str, Any]]) -> dict[str, Any] | None:
    latest = latest_by_model_os(rows)
    passes = [row for row in latest if row.get("result") == "pass"]
    if not passes:
        return None
    return max(passes, key=lambda row: (
        parse_observed(row.get("observed_at"), row["evidence_id"]), row["evidence_id"],
    ))


def project_m4(client: dict[str, Any], rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    events = []
    canonical_features = {
        feature
        for row in protocol_rows(client)
        for feature in (row.get("client_transport_features") or {})
    }
    for protocol_row in protocol_rows(client):
        protocol = protocol_row.get("protocol")
        if not isinstance(protocol, str):
            continue
        exact = exact_receipts(
            rows, client, evidence_types=M4_EVIDENCE_TYPES, protocol=protocol,
            features=canonical_features | PRESENCE_FEATURES,
        )
        if not exact:
            continue
        before = json.dumps(protocol_row, ensure_ascii=False, sort_keys=True)
        feature_map = protocol_row.get("client_transport_features")
        if not isinstance(feature_map, dict):
            raise ValueError(f"{client.get('id')}/{protocol}: client_transport_features is required")
        latest_feature_rows: list[dict[str, Any]] = []
        for feature in sorted(canonical_features):
            candidates = [row for row in exact if row.get("target", {}).get("feature") == feature]
            latest = latest_by_model_os(candidates)
            latest_feature_rows.extend(latest)
            if any(row.get("result") == "pass" for row in latest):
                feature_map[feature] = "verified"
            elif latest:
                feature_map[feature] = "unverified"
        explicit_presence = latest_by_model_os([
            row for row in exact if row.get("target", {}).get("feature") in PRESENCE_FEATURES
        ])
        presence = explicit_presence + latest_feature_rows
        global_unsupported = [
            row for row in presence
            if row.get("result") == "unsupported" and row.get("target", {}).get("model_id") is None
        ]
        passing = [row for row in presence if row.get("result") == "pass"]
        if global_unsupported:
            winner = max(global_unsupported, key=lambda row: (
                parse_observed(row.get("observed_at"), row["evidence_id"]), row["evidence_id"],
            ))
            protocol_row["support"] = "unsupported"
            for feature in feature_map:
                feature_map[feature] = "not_applicable"
            protocol_row["evidence"] = evidence_object(winner, "unsupported")
        elif passing:
            winner = max(passing, key=lambda row: (
                parse_observed(row.get("observed_at"), row["evidence_id"]), row["evidence_id"],
            ))
            protocol_row["support"] = "supported"
            protocol_row["evidence"] = evidence_object(winner, "verified")
        elif presence:
            winner = max(presence, key=lambda row: (
                parse_observed(row.get("observed_at"), row["evidence_id"]), row["evidence_id"],
            ))
            protocol_row["evidence"] = evidence_object(winner, "blocked")
        if before != json.dumps(protocol_row, ensure_ascii=False, sort_keys=True):
            events.append({
                "matrix": "client_protocol", "client_id": client.get("id"),
                "protocol": protocol, "status": protocol_row.get("evidence", {}).get("status"),
            })
    return events


def project_m5(client: dict[str, Any], rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    reasoning = client.get("client_reasoning")
    if not isinstance(reasoning, dict):
        return []
    values = set(reasoning.get("level_control", {}).get("values", []))
    supported_protocols = {
        row.get("protocol") for row in protocol_rows(client) if row.get("support") == "supported"
    }
    candidates = exact_receipts(
        rows, client, evidence_types=M5_EVIDENCE_TYPES, features=REASONING_FEATURES,
    )
    candidates = [
        row for row in candidates
        if row.get("target", {}).get("protocol") in supported_protocols
        and isinstance(row.get("target", {}).get("reasoning_level"), str)
        and row["target"]["reasoning_level"] in values
    ]
    if not candidates:
        return []
    before = json.dumps(reasoning, ensure_ascii=False, sort_keys=True)
    passing = latest_pass(candidates)
    if passing:
        winner = passing
        reasoning["evidence"] = evidence_object(winner, "verified")
    else:
        latest = latest_by_os(candidates)
        winner = max(latest, key=lambda row: (
            parse_observed(row.get("observed_at"), row["evidence_id"]), row["evidence_id"],
        ))
        reasoning["evidence"] = evidence_object(winner, "blocked")
    if before == json.dumps(reasoning, ensure_ascii=False, sort_keys=True):
        return []
    return [{
        "matrix": "client_reasoning", "client_id": client.get("id"),
        "protocol": winner.get("target", {}).get("protocol"),
        "reasoning_level": winner.get("target", {}).get("reasoning_level"),
        "status": reasoning.get("evidence", {}).get("status"),
    }]


def project_m7(client: dict[str, Any], rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    events = []
    config = client.get("client_config_os", {})
    for os_row in config.get("os_support", []):
        if not isinstance(os_row, dict) or not isinstance(os_row.get("os"), str):
            continue
        os_id = os_row["os"]
        exact = exact_receipts(
            rows, client, evidence_types=M7_EVIDENCE_TYPES, os_id=os_id,
            features=CONFIG_FEATURES,
        )
        by_feature = {
            feature: latest_by_os([
                row for row in exact if row.get("target", {}).get("feature") == feature
            ])
            for feature in CONFIG_FEATURES
        }
        if not any(by_feature.values()):
            continue
        before = json.dumps(os_row, ensure_ascii=False, sort_keys=True)
        complete = all(by_feature[feature] for feature in CONFIG_FEATURES)
        passes = complete and all(
            any(row.get("result") == "pass" for row in by_feature[feature])
            for feature in CONFIG_FEATURES
        )
        all_latest = [row for values in by_feature.values() for row in values]
        winner = max(all_latest, key=lambda row: (
            parse_observed(row.get("observed_at"), row["evidence_id"]), row["evidence_id"],
        ))
        if passes:
            os_row["support"] = "verified"
            os_row["evidence"] = evidence_object(winner, "verified")
        elif complete or any(row.get("result") in NEGATIVE_RESULTS for row in all_latest):
            if os_row.get("support") == "verified":
                os_row["support"] = "documented"
            os_row["evidence"] = evidence_object(winner, "blocked")
        if before != json.dumps(os_row, ensure_ascii=False, sort_keys=True):
            events.append({
                "matrix": "client_config_os", "client_id": client.get("id"),
                "os": os_id, "status": os_row.get("evidence", {}).get("status"),
            })
    return events


def project(matrix: dict[str, Any], rows: list[dict[str, Any]]) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    result = deepcopy(matrix)
    clients = result.get("clients")
    if not isinstance(clients, list):
        raise ValueError("client matrix clients must be an array")
    events = []
    for client in clients:
        if not isinstance(client, dict) or not isinstance(client.get("id"), str):
            raise ValueError("every client must have an id")
        events.extend(project_m4(client, rows))
        events.extend(project_m5(client, rows))
        events.extend(project_m7(client, rows))
    events.sort(key=lambda event: json.dumps(event, ensure_ascii=False, sort_keys=True))
    return result, events


def atomic_write_json(path: Path, value: dict[str, Any], backup_dir: Path | None = None) -> None:
    payload = json.dumps(value, ensure_ascii=False, indent=2) + "\n"
    if backup_dir is not None:
        backup_dir.mkdir(parents=True, exist_ok=True)
        original_hash = file_sha256(path)
        backup = backup_dir / f"{path.stem}.{original_hash}.json"
        if not backup.exists():
            shutil.copy2(path, backup)
        if file_sha256(backup) != original_hash:
            raise ValueError(f"content-addressed backup verification failed: {backup}")
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def projected_json_sha256(value: dict[str, Any]) -> str:
    payload = (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def parse_args(argv: Iterable[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("plan", "apply"))
    parser.add_argument("--client-matrix", type=Path, required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--artifact-root", type=Path, default=Path.cwd())
    parser.add_argument("--backup-dir", type=Path)
    parser.add_argument("--report", type=Path)
    return parser.parse_args(argv)


def main(argv: Iterable[str] | None = None) -> int:
    args = parse_args(argv)
    matrix = load(args.client_matrix)
    evidence = load(args.evidence)
    # Both plan and apply are evidence gates. A plan that cannot resolve and
    # verify its artifact bytes is not safe input for a later apply.
    verify_artifacts = True
    rows = receipt_rows(evidence, artifact_root=args.artifact_root, verify_artifacts=verify_artifacts)
    projected, events = project(matrix, rows)
    changed = projected != matrix
    report = {
        "schema_version": 1,
        "kind": "client_evidence_projection",
        "command": args.command,
        "production_write": False,
        "client_matrix_write": args.command == "apply" and changed,
        "changed": changed,
        "events": len(events),
        "matrix_event_counts": {
            name: sum(event.get("matrix") == name for event in events)
            for name in ("client_protocol", "client_reasoning", "client_config_os")
        },
        "evidence_index_sha256": file_sha256(args.evidence),
        "client_matrix_before_sha256": file_sha256(args.client_matrix),
        "events_detail": events,
    }
    assert_secret_free(json.dumps(report, ensure_ascii=False), label="projection report")
    if args.command == "apply" and changed:
        atomic_write_json(
            args.client_matrix, projected,
            args.backup_dir or args.client_matrix.parent / ".client-evidence-backups",
        )
        report["client_matrix_after_sha256"] = file_sha256(args.client_matrix)
    else:
        report["client_matrix_after_sha256"] = projected_json_sha256(projected)
    if args.report:
        atomic_write_json(args.report, report)
    print(json.dumps({
        "command": report["command"], "changed": changed, "events": len(events),
        "matrix_event_counts": report["matrix_event_counts"],
    }, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

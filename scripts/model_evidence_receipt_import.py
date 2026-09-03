#!/usr/bin/env python3
"""Import secret-free live artifacts into the canonical M8 evidence index.

Group artifacts are consumed offline.  ``/v1/models`` observations are emitted
only as ``model_discovery`` evidence.  A group route is a pass only when the
artifact records HTTP 200, a complete/non-empty terminal result, and
``passed=true``.  In particular, HTTP 200 with ``completed=false`` is a real
failure receipt and can never upgrade group access.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timedelta
import hashlib
import json
import os
from pathlib import Path
import re
import tempfile
from typing import Any


SHA256_RE = re.compile(r"^[a-f0-9]{64}$")
KNOWN_PROTOCOL_ENDPOINTS = {
    "/responses", "/v1/responses", "/v1/chat/completions", "/v1/messages", "/v1beta/models",
}
PROTOCOL_INBOUND_ENDPOINTS = {
    "responses": {"/responses", "/v1/responses"},
    "chat_completions": {"/v1/chat/completions"},
    "messages": {"/v1/messages"},
    "generate_content": {"/v1beta/models"},
}
PROVIDER_FEATURES = {
    "P-01": "minimal_text",
    "P-02": "streaming_sse",
    "P-03": "streaming_terminal",
    "P-04": "tool_call",
    "P-05": "tool_result_continuation",
    "P-06": "reasoning",
    "P-07": "prompt_cache",
    "P-08": "image_input",
    "P-10": "structured_output",
    "P-11": "web_search",
    "P-12": "usage",
    "P-15": "error_passthrough",
}
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


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def canonical_sha256(value: dict[str, Any]) -> str:
    payload = json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def immediately_after(value: Any) -> Any:
    if not isinstance(value, str):
        return value
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return value
    return (parsed + timedelta(microseconds=1)).isoformat().replace("+00:00", "Z")


def assert_secret_free(value: str | bytes, *, label: str) -> None:
    text = value.decode("utf-8", errors="replace") if isinstance(value, bytes) else value
    if any(pattern.search(text) for pattern in SECRET_PATTERNS):
        raise ValueError(f"secret-shaped value found in {label}")


def validate_group_artifact(artifact: Path, source: dict[str, Any]) -> None:
    raw = artifact.read_bytes()
    assert_secret_free(raw, label=str(artifact))
    if source.get("kind") != "owned_group_model_discovery" or source.get("secret_free") is not True:
        raise ValueError("artifact is not a secret-free owned group probe receipt")
    if source.get("dry_run") is not False or source.get("production_write") is not True:
        raise ValueError("group evidence must come from an executed owned probe, not a dry-run plan")
    embedded = source.get("artifact_sha256")
    if not isinstance(embedded, str) or not SHA256_RE.fullmatch(embedded):
        raise ValueError("group artifact is missing its immutable artifact_sha256")
    body = dict(source)
    body.pop("artifact_sha256", None)
    if canonical_sha256(body) != embedded:
        raise ValueError("group artifact canonical SHA-256 does not match its contents")
    identity = source.get("identity")
    if not isinstance(identity, dict) or not isinstance(identity.get("key_id"), int):
        raise ValueError("group artifact must identify the owned key id")
    if not isinstance(identity.get("user_id"), int):
        raise ValueError("group artifact must identify the owned user id")


def valid_group_result(item: dict[str, Any]) -> tuple[str, str]:
    group = item.get("group")
    key = item.get("key_readback")
    if not isinstance(group, dict) or group.get("id") is None:
        raise ValueError("group result is missing group.id")
    group_id = str(group["id"])
    if (
        not isinstance(key, dict)
        or str(key.get("group_id")) != group_id
        or key.get("status") != "active"
    ):
        raise ValueError(f"group {group_id} lacks an exact active key readback")
    observed_at = item.get("observed_at")
    if not isinstance(observed_at, str) or not observed_at:
        raise ValueError(f"group {group_id} result is missing observed_at")
    return group_id, observed_at


def evidence_id(kind: str, target: dict[str, Any], artifact_sha256: str) -> str:
    payload = json.dumps({"kind": kind, "target": target, "artifact": artifact_sha256}, sort_keys=True, separators=(",", ":")).encode()
    return f"evidence-{hashlib.sha256(payload).hexdigest()[:24]}"


def row(kind: str, target: dict[str, Any], result: str, observed_at: str, artifact: Path, summary: str) -> dict[str, Any]:
    digest = sha256(artifact)
    value = {
        "evidence_id": evidence_id(kind, target, digest),
        "evidence_type": kind,
        "target": target,
        "result": result,
        "observed_at": observed_at,
        "artifact_uri": str(artifact),
        "artifact_sha256": digest,
        "versions": {},
        "summary": summary,
        "secret_free": True,
    }
    rendered = json.dumps(value, ensure_ascii=False)
    assert_secret_free(rendered, label="generated evidence receipt")
    return value


def group_discovery(artifact: Path) -> list[dict[str, Any]]:
    source = load(artifact)
    validate_group_artifact(artifact, source)
    output = []
    for item in source.get("results", []):
        if not isinstance(item, dict):
            continue
        group_id, observed_at = valid_group_result(item)
        group = item.get("group", {})
        discovery = item.get("discovery", {})
        if not isinstance(discovery, dict):
            continue
        # Only listed model IDs receive a discovery receipt. Missing IDs remain
        # missing/blocked in the contract projection; discovery is never route
        # capability proof.
        passed = discovery.get("http_status") == 200 and discovery.get("error") is None
        for model_id in discovery.get("model_ids", []):
            if not isinstance(model_id, str) or not model_id:
                continue
            target = {"model_id": model_id, "group_id": group_id, "feature": "model_discovery"}
            output.append(row(
                "live_protocol_probe", target, "pass" if passed else "fail", observed_at, artifact,
                f"Owned key /v1/models readback for group {group_id} returned model {model_id}; discovery only, not capability proof.",
            ))
    return output


def owned_e2e(artifact: Path, protocol: str) -> list[dict[str, Any]]:
    source = load(artifact)
    if "identity" not in source or "results" not in source:
        raise ValueError("artifact is not an owned E2E report")
    output = []
    group_id = str(source["identity"].get("group_id"))
    observed_at = source.get("finished_at_utc") or source.get("started_at_utc")
    for item in source.get("results", []):
        model_id = item.get("model")
        passed = item.get("passed") is True
        result = "pass" if passed else "fail"
        features = ["minimal_text", "streaming_terminal", "route_call"]
        if passed and source.get("attribution", {}).get("usage_rows"):
            features.append("usage")
        for feature in features:
            target = {"model_id": model_id, "group_id": group_id, "protocol": protocol, "feature": feature}
            output.append(row(
                "live_protocol_probe", target, result, observed_at, artifact,
                f"Owned public-gateway {protocol} probe HTTP {item.get('http_status')}; completed={bool(item.get('completed'))}; feature={feature}.",
            ))
    return output


def group_smoke(artifact: Path) -> list[dict[str, Any]]:
    source = load(artifact)
    validate_group_artifact(artifact, source)
    # A smoke artifact also contains an exact /v1/models readback. Import it as
    # discovery-only evidence so a new owned-key batch can close both halves in
    # one immutable artifact without conflating their meanings.
    output = group_discovery(artifact)
    for item in source.get("results", []):
        if not isinstance(item, dict):
            continue
        group_id, observed_at = valid_group_result(item)
        for probe in item.get("smoke_probes", []):
            if not isinstance(probe, dict):
                continue
            model_id, protocol = probe.get("model_id"), probe.get("protocol")
            if not isinstance(model_id, str) or not isinstance(protocol, str):
                continue
            shape = probe.get("response_shape")
            shape_nonempty = True
            if isinstance(shape, dict):
                shape_nonempty = any(
                    isinstance(shape.get(field), int) and shape[field] > 0
                    for field in ("output_count", "choices_count", "candidates_count", "content_count")
                )
            # Do not trust HTTP status alone or a producer's pass flag alone.
            # completed=true is the persisted proof that a non-empty terminal
            # result was observed for the minimal call.
            passed = (
                probe.get("passed") is True
                and probe.get("http_status") == 200
                and probe.get("completed") is True
                and probe.get("error") is None
                and shape_nonempty
                and isinstance(probe.get("body_sha256"), str)
                and bool(SHA256_RE.fullmatch(probe["body_sha256"]))
            )
            for feature in ("route_call", "minimal_text"):
                target = {
                    "model_id": model_id, "group_id": group_id,
                    "protocol": protocol, "feature": feature,
                }
                output.append(row(
                    "live_protocol_probe", target, "pass" if passed else "fail", observed_at, artifact,
                    f"Owned group {group_id} minimal {protocol} route HTTP {probe.get('http_status')}; completed={probe.get('completed') is True}; feature={feature}.",
                ))
    return output


def client_loop(artifact: Path) -> list[dict[str, Any]]:
    source = load(artifact)
    if source.get("kind") != "real_client_loop" or not source.get("secret_free"):
        raise ValueError("artifact is not a secret-free real client loop")
    assert_secret_free(artifact.read_bytes(), label=str(artifact))
    target = {
        "model_id": source.get("model_id"), "protocol": source.get("protocol"),
        "client_id": source.get("client_id"), "client_version": source.get("client_version"),
        "os": source.get("os"), "architecture": source.get("architecture"), "feature": "agent_loop",
    }
    attribution_rows: list[dict[str, Any]] = []
    attribution_ref = source.get("protocol_attribution")
    attribution_verified = True
    if isinstance(attribution_ref, dict):
        uri, expected_digest = attribution_ref.get("artifact_uri"), attribution_ref.get("artifact_sha256")
        if not isinstance(uri, str) or not isinstance(expected_digest, str):
            raise ValueError("client protocol_attribution requires artifact_uri and artifact_sha256")
        attribution_uri_path = Path(uri)
        attribution_path = attribution_uri_path
        if not attribution_path.is_absolute():
            attribution_path = Path.cwd() / attribution_path
        if not attribution_path.is_file() or sha256(attribution_path) != expected_digest:
            raise ValueError("client protocol attribution artifact SHA-256 mismatch")
        attribution = load(attribution_path)
        assert_secret_free(attribution_path.read_bytes(), label=str(attribution_path))
        embedded = attribution.get("artifact_sha256")
        body = dict(attribution)
        body.pop("artifact_sha256", None)
        if embedded != canonical_sha256(body):
            raise ValueError("client protocol attribution embedded SHA-256 mismatch")
        usage_rows = attribution.get("rows")
        expected_inbound_endpoints = PROTOCOL_INBOUND_ENDPOINTS.get(source.get("protocol"), set())
        declared_inbound = attribution_ref.get("inbound_endpoint")
        declared_upstream = attribution_ref.get("upstream_endpoint")
        attribution_verified = (
            attribution.get("kind") == "client_usage_attribution"
            and attribution.get("secret_free") is True
            and attribution.get("client_id") == source.get("client_id")
            and attribution.get("client_version") == source.get("client_version")
            and attribution.get("group_id") == source.get("group_id")
            and isinstance(usage_rows, list)
            and len(usage_rows) == attribution_ref.get("usage_rows")
            and bool(usage_rows)
            and declared_inbound in expected_inbound_endpoints
            and declared_upstream in KNOWN_PROTOCOL_ENDPOINTS
            and all(
                isinstance(item, dict)
                and item.get("model") == source.get("model_id")
                and item.get("requested_model", item.get("model")) == source.get("model_id")
                and item.get("inbound_endpoint") == declared_inbound
                and item.get("upstream_endpoint") == declared_upstream
                and isinstance(item.get("input_tokens"), int)
                and isinstance(item.get("output_tokens"), int)
                for item in usage_rows
            )
            and attribution_ref.get("verified") is True
        )
        attribution_target = {
            "model_id": source.get("model_id"), "group_id": str(source.get("group_id")),
            "protocol": source.get("protocol"), "client_id": source.get("client_id"),
            "client_version": source.get("client_version"), "os": source.get("os"),
            "architecture": source.get("architecture"), "feature": "usage_attribution",
            "inbound_endpoint": declared_inbound, "upstream_endpoint": declared_upstream,
        }
        attribution_row = row(
            "billing_reconciliation", attribution_target,
            "pass" if attribution_verified else "fail",
            attribution.get("observed_at"), attribution_uri_path,
            f"Owned client usage attribution rows={len(usage_rows) if isinstance(usage_rows, list) else 0}; inbound={declared_inbound}; upstream={declared_upstream}.",
        )
        attribution_rows.append(attribution_row)

    passed = (
        source.get("passed") is True
        and source.get("exit_code") == 0
        and source.get("timed_out") is False
        and source.get("tool_use_observed") is True
        and source.get("tool_result_observed") is True
        and source.get("file_marker_verified") is True
        and source.get("final_marker_verified") is True
        and isinstance(source.get("stdout_bytes"), int)
        and source["stdout_bytes"] > 0
        and isinstance(source.get("stdout_sha256"), str)
        and bool(SHA256_RE.fullmatch(source["stdout_sha256"]))
        and attribution_verified
    )
    result = "pass" if passed else "fail"
    item = row(
        "real_client_loop", target, result, source.get("observed_at"), artifact,
        f"Real {source.get('client_id')} loop exit={source.get('exit_code')}; protocol={source.get('protocol')}; tool and final markers verified={passed}.",
    )
    item["versions"] = {"client": str(source.get("client_version")), "os": str(source.get("os"))}
    if attribution_rows:
        item["evidence_refs"] = [value["evidence_id"] for value in attribution_rows]
    return [item, *attribution_rows]


def provider_live(artifact: Path) -> list[dict[str, Any]]:
    source = load(artifact)
    assert_secret_free(artifact.read_bytes(), label=str(artifact))
    if source.get("kind") != "provider_contract_live_case" or source.get("secret_free") is not True:
        raise ValueError("artifact is not a secret-free live provider receipt")
    embedded = source.get("artifact_sha256")
    body = dict(source)
    body.pop("artifact_sha256", None)
    if not isinstance(embedded, str) or not SHA256_RE.fullmatch(embedded) or canonical_sha256(body) != embedded:
        raise ValueError("provider artifact canonical SHA-256 does not match its contents")
    case = source.get("case")
    if not isinstance(case, dict):
        raise ValueError("provider artifact is missing its exact case")
    p_id = case.get("p_id")
    feature = PROVIDER_FEATURES.get(p_id)
    if feature is None:
        raise ValueError(f"unsupported provider contract case: {p_id}")
    model_id, protocol, group_id = case.get("model_id"), case.get("protocol"), case.get("group_id")
    if not isinstance(model_id, str) or not isinstance(protocol, str) or not isinstance(group_id, int):
        raise ValueError("provider artifact lacks model/protocol/group identity")
    passed = (
        source.get("result") == "pass"
        and source.get("classification") == "verified"
        and source.get("offline_verifier", {}).get("status") == "passed"
    )
    target = {
        "model_id": model_id,
        "group_id": str(group_id),
        "protocol": protocol,
        "feature": feature,
        "p_id": p_id,
    }
    rows = [row(
        "live_protocol_probe", target, "pass" if passed else "fail",
        source.get("observed_at"), artifact,
        f"Owned live provider case {p_id} for group {group_id}; HTTP {source.get('response', {}).get('http_status')}; classification={source.get('classification')}.",
    )]
    response = source.get("response") if isinstance(source.get("response"), dict) else {}
    reason = str(source.get("reason") or "")
    deterministic_negative = (
        not passed
        and response.get("http_status") == 200
        and (
            (p_id == "P-03" and "terminal" in reason)
            or (p_id == "P-04" and "tool_call.name" in reason)
            or (p_id == "P-05" and "tool result must be correlated" in reason)
            or (p_id == "P-12" and reason.startswith("usage."))
        )
    ) or (
        not passed
        and response.get("http_status") == 400
        and p_id in {"P-08", "P-11"}
    )
    if deterministic_negative:
        unsupported_target = dict(target)
        unsupported_target["component"] = "terminal_negative_capability"
        rows.append(row(
            "live_protocol_probe", unsupported_target, "unsupported",
            source.get("finished_at") or source.get("observed_at"), artifact,
            f"Exact valid capability probe produced a deterministic terminal negative result: HTTP {response.get('http_status')}; {reason}.",
        ))
    if p_id == "P-12":
        billing = source.get("billing_attribution")
        billing_passed = (
            isinstance(billing, dict)
            and billing.get("status") == "verified"
            and isinstance(billing.get("usage_row_ids"), list)
            and bool(billing["usage_row_ids"])
            and isinstance(billing.get("actual_cost"), (int, float))
            and isinstance(billing.get("total_cost"), (int, float))
        )
        billing_target = dict(target)
        billing_target["feature"] = "billing"
        billing_target["component"] = "actual_cost_attribution"
        rows.append(row(
            "billing_reconciliation", billing_target,
            "pass" if billing_passed else "fail",
            source.get("finished_at") or source.get("observed_at"), artifact,
            f"Owned provider billing attribution rows={len(billing.get('usage_row_ids', [])) if isinstance(billing, dict) else 0}; actual_cost recorded={billing_passed}.",
        ))
    if p_id == "P-05":
        tool_target = dict(target)
        tool_target["feature"] = "tool_call"
        tool_target["component"] = "continuation_prerequisite"
        rows.append(row(
            "live_protocol_probe", tool_target,
            "pass" if passed else "fail",
            source.get("finished_at") or source.get("observed_at"), artifact,
            f"Tool-result continuation requires and observed the preceding tool call; continuation classification={source.get('classification')}.",
        ))
        if deterministic_negative:
            terminal_tool_target = dict(target)
            terminal_tool_target["feature"] = "tool_call"
            terminal_tool_target["component"] = "terminal_negative_tool_prerequisite_v2"
            rows.append(row(
                "live_protocol_probe", terminal_tool_target, "unsupported",
                immediately_after(source.get("finished_at") or source.get("observed_at")), artifact,
                "A forced tool-result continuation could not observe a usable preceding tool call for this exact model/protocol route.",
            ))
    if p_id == "P-15":
        invalid_target = dict(target)
        invalid_target["feature"] = "invalid_request"
        rows.append(row(
            "live_protocol_probe", invalid_target,
            "pass" if passed else "fail",
            source.get("observed_at"), artifact,
            f"Owned invalid-request case returned HTTP {source.get('response', {}).get('http_status')} with provider error metadata; passthrough classification={source.get('classification')}.",
        ))
    if p_id == "P-06" and isinstance(case.get("reasoning_level"), str):
        level_target = dict(target)
        level_target["feature"] = "reasoning_level"
        level_target["level"] = case["reasoning_level"]
        rows.append(row(
            "live_protocol_probe", level_target,
            "pass" if passed else "fail",
            source.get("observed_at"), artifact,
            f"Exact reasoning level {case['reasoning_level']} transport case; HTTP {source.get('response', {}).get('http_status')}; classification={source.get('classification')}.",
        ))
    return rows


def atomic_write(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as handle:
            json.dump(value, handle, ensure_ascii=False, indent=2)
            handle.write("\n")
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def merge(index: Path, rows: list[dict[str, Any]]) -> dict[str, Any]:
    current = load(index) if index.exists() else {"schema_version": 1, "kind": "test_evidence_matrix", "rows": []}
    before = json.dumps(current, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    by_id = {item["evidence_id"]: item for item in current.get("rows", [])}
    for item in rows:
        existing = by_id.get(item["evidence_id"])
        if existing is not None and existing != item:
            raise ValueError(f"conflicting evidence id: {item['evidence_id']}")
        by_id[item["evidence_id"]] = item
    current["rows"] = sorted(by_id.values(), key=lambda item: item["evidence_id"])
    after = json.dumps(current, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    if before != after or not index.exists():
        atomic_write(index, current)
    return current


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind", choices=("group-discovery", "group-smoke", "owned-e2e", "client-loop", "provider-live"))
    parser.add_argument("--artifact", type=Path, required=True)
    parser.add_argument("--protocol", choices=("responses", "chat_completions", "messages", "generate_content"))
    parser.add_argument("--index", type=Path, required=True)
    args = parser.parse_args()
    if args.kind == "group-discovery":
        imported = group_discovery(args.artifact)
    elif args.kind == "group-smoke":
        imported = group_smoke(args.artifact)
    elif args.kind == "client-loop":
        imported = client_loop(args.artifact)
    elif args.kind == "provider-live":
        imported = provider_live(args.artifact)
    else:
        if not args.protocol:
            parser.error("owned-e2e requires --protocol")
        imported = owned_e2e(args.artifact, args.protocol)
    current = merge(args.index, imported)
    print(json.dumps({"imported": len(imported), "total": len(current["rows"]), "index": str(args.index)}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

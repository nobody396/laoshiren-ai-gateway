"""Resolve terminal matrix evidence offline; a hash string alone is not proof."""
from __future__ import annotations

from datetime import date, datetime, timezone
import hashlib
import json
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

TERMINAL = {"verified", "unsupported", "not_published", "not_exposed", "not_applicable"}
FEATURE_CASES = {"minimal_text":"P-01", "basic_request":"P-01", "streaming_sse":"P-02",
    "streaming_terminal":"P-03", "terminal_event":"P-03", "tool_call":"P-04", "tool_calls":"P-04",
    "tool_result_continuation":"P-05", "tool_result_round_trip":"P-05", "reasoning":"P-06",
    "prompt_cache":"P-07", "image_input":"P-08", "structured_output":"P-10", "web_search":"P-11",
    "usage":"P-12", "invalid_request":"P-15", "error_passthrough":"P-15"}


def fingerprint(value: Any) -> str:
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode()).hexdigest()


def artifact_gaps(value: Any, root: Path, *, as_of: date, max_age_days: int = 180, path: str = "evidence", subject: dict[str, Any] | None = None) -> list[str]:
    """Require resolvable bytes, digest, date and non-fixture origin for each terminal cell.

    No remote URLs are fetched here: store a redacted official-spec snapshot or
    an actual live receipt. Narrative/URL-only evidence stays a migration gap.
    """
    failures = []
    subject = dict(subject or {})
    for component in path.split("/"):
        if component in {"responses","chat_completions","messages","generate_content"}:
            subject["protocol"] = component
    if isinstance(value, dict):
        model = value.get("model", {})
        access = value.get("access", {})
        if isinstance(model, dict) and model.get("id"):
            subject["model_id"] = model["id"]
        if isinstance(access, dict) and access.get("base_url"):
            subject["base_url"] = access["base_url"]
        for key in ("model_id", "protocol", "group_id", "base_url", "client_version", "os"):
            if key in value:
                subject[key] = value[key]
    if isinstance(value, list):
        for index, item in enumerate(value):
            failures.extend(artifact_gaps(item, root, as_of=as_of, max_age_days=max_age_days, path=f"{path}/{index}", subject=subject))
        return failures
    if not isinstance(value, dict):
        return failures
    if value.get("status") in TERMINAL:
        # Summary rows cite a separate evidence object or evidence_ids; only
        # concrete evidence cells own source_ref/digest. Narrative-only facts fail.
        ref = value.get("source_ref")
        if isinstance(value.get("evidence"), dict):
            pass
        elif "evidence_ids" in value:
            failures.append(f"{path}: unresolved evidence_ids require a materialized receipt reference")
        else:
            if not isinstance(ref, str) or not ref:
                failures.append(f"{path}: terminal evidence requires source_ref")
            else:
                parsed = urlsplit(ref)
                relative = Path(parsed.path)
                target = (root / relative).resolve()
                if parsed.scheme or parsed.netloc or relative.is_absolute() or not target.is_relative_to(root.resolve()):
                    failures.append(f"{path}: source_ref must be an in-root relative artifact")
                elif not target.is_file():
                    failures.append(f"{path}: evidence artifact missing")
                else:
                    data = target.read_bytes()
                    if len(data) > 16 * 1024 * 1024:
                        failures.append(f"{path}: evidence artifact exceeds 16 MiB")
                    elif hashlib.sha256(data).hexdigest() != value.get("artifact_sha256"):
                        failures.append(f"{path}: evidence artifact digest mismatch")
                    else:
                        try:
                            receipt = json.loads(data)
                        except (ValueError, UnicodeError):
                            receipt = None
                        if not isinstance(receipt, dict):
                            failures.append(f"{path}: evidence receipt must be a JSON object")
                        elif (receipt.get("network_execution") == "disabled"
                              or receipt.get("mode") == "offline_fixture"
                              or "fixture" in str(receipt.get("kind", "")).lower()):
                            failures.append(f"{path}: offline fixture is not release evidence")
                        else:
                            kind = receipt.get("kind")
                            target_subject = receipt.get("subject", receipt.get("case", {}))
                            if not isinstance(target_subject, dict):
                                target_subject = {}
                            if kind in {"official_model_spec", "official_client_spec"}:
                                url = urlsplit(str(receipt.get("url", "")))
                                if url.scheme != "https" or not url.netloc or url.username or url.password:
                                    failures.append(f"{path}: official receipt requires public HTTPS source")
                            elif kind == "provider_contract_live_case":
                                if (receipt.get("schema_version") != 2
                                        or receipt.get("network_execution") != "explicit_live"
                                        or receipt.get("offline_verifier", {}).get("status") != "passed"
                                        or receipt.get("result") != "pass" or receipt.get("classification") != "verified"
                                        or receipt.get("response", {}).get("http_status") not in range(200,600)
                                        or not target_subject.get("model_id")):
                                    failures.append(f"{path}: incomplete live provider receipt")
                            elif kind == "owned_client_loop":
                                required = ("local_tool_executed", "tool_result_submitted", "final_answer_received")
                                if (receipt.get("network_execution") != "explicit_live"
                                        or receipt.get("exit_code") != 0
                                        or not all(receipt.get(field) is True for field in required)):
                                    failures.append(f"{path}: incomplete real client tool loop")
                            elif kind == "owned_gateway_e2e":
                                if (receipt.get("network_execution") != "explicit_live"
                                        or receipt.get("usage_row_count") != 1
                                        or receipt.get("accounting_command_count") != 1
                                        or receipt.get("balance_reconciled") is not True
                                        or receipt.get("attribution_verified") is not True):
                                    failures.append(f"{path}: incomplete owned accounting proof")
                            else:
                                failures.append(f"{path}: unsupported evidence kind")
                            feature = path.rsplit("/",1)[-1]
                            if "/test_matrix/clients/" in path and kind != "owned_client_loop":
                                failures.append(f"{path}: client cell requires a real client loop receipt")
                            if feature == "billing" and kind != "owned_gateway_e2e":
                                failures.append(f"{path}: billing requires owned accounting proof, not a protocol/usage receipt")
                            if feature in FEATURE_CASES:
                                if kind != "provider_contract_live_case" or target_subject.get("p_id") != FEATURE_CASES[feature]:
                                    failures.append(f"{path}: artifact does not prove this exact feature case")
                            receipt_time = receipt.get("observed_at")
                            wrapper_time = value.get("observed_at",value.get("verified_at"))
                            if receipt_time != wrapper_time:
                                failures.append(f"{path}: wrapper observation date must match source receipt")
                            for field, expected in subject.items():
                                if kind in {"official_model_spec", "official_client_spec"} and field in {"base_url", "group_id"}:
                                    continue
                                if target_subject.get(field) != expected:
                                    failures.append(f"{path}: artifact subject mismatch for {field}")
            observed = value.get("observed_at", value.get("verified_at"))
            try:
                day = date.fromisoformat(observed[:10]) if isinstance(observed, str) else None
                if day is None or not 0 <= (as_of-day).days <= max_age_days:
                    failures.append(f"{path}: evidence is missing, stale or future-dated")
            except ValueError:
                failures.append(f"{path}: invalid evidence date")
    for key, item in value.items():
        if isinstance(item, (dict, list)):
            failures.extend(artifact_gaps(item, root, as_of=as_of, max_age_days=max_age_days, path=f"{path}/{key}", subject=subject))
    return failures

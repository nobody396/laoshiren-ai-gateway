"""Resolve terminal matrix evidence offline; a hash string alone is not proof."""
from __future__ import annotations

from datetime import date, datetime, timezone
import hashlib
import json
import math
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


def linked_artifact(root: Path, ref: Any, digest: Any) -> dict[str, Any] | None:
    if not isinstance(ref,str) or not ref or urlsplit(ref).scheme or urlsplit(ref).netloc:
        return None
    candidate=Path(ref)
    target=(root/candidate).resolve()
    if candidate.is_absolute() or not target.is_relative_to(root.resolve()) or not target.is_file():
        return None
    raw=target.read_bytes()
    if len(raw)>16*1024*1024 or hashlib.sha256(raw).hexdigest()!=digest:
        return None
    try: value=json.loads(raw)
    except (ValueError,UnicodeError): return None
    return value if isinstance(value,dict) else None


def provider_invoice_link_matches(receipt: dict[str, Any], row: Any, root: Path) -> bool:
    if not isinstance(row,dict): return False
    invoice=linked_artifact(root,receipt.get("invoice_ref"),receipt.get("invoice_sha256"))
    probe=linked_artifact(root,row.get("probe_ref"),row.get("probe_artifact_sha256"))
    if not invoice or not probe: return False
    model=receipt.get("subject",{}).get("model_id")
    if (invoice.get("kind")!="provider_invoice_snapshot" or invoice.get("model_id")!=model
            or invoice.get("token_id")!=receipt.get("source_token_id")
            or probe.get("kind")!="provider_contract_live_case" or probe.get("network_execution")!="explicit_live"
            or probe.get("scope")!="direct_upstream_only" or probe.get("case",{}).get("model_id")!=model
            or probe.get("response",{}).get("http_status")!=200): return False
    usage=probe.get("usage") or {}
    if any(usage.get(k)!=row.get(k) for k in ("input_tokens","output_tokens","cached_input_tokens")): return False
    try:
        start=datetime.fromisoformat(probe["observed_at"].replace("Z","+00:00"))
        end=datetime.fromisoformat(probe["finished_at"].replace("Z","+00:00"))
        if start.tzinfo is None or end.tzinfo is None or not 0 <= (end-start).total_seconds() <= 600: return False
        matches=[r for r in invoice.get("rows",[]) if isinstance(r,dict)
                 and r.get("model_name")==model and r.get("token_id")==receipt.get("source_token_id")
                 and r.get("prompt_tokens")==row.get("input_tokens") and r.get("completion_tokens")==row.get("output_tokens")
                 and start.timestamp()-5 <= r.get("created_at",0) <= end.timestamp()+5]
        if len(matches)!=1: return False
        matched=matches[0];metadata=matched.get("billing_metadata",{})
        return (matched.get("request_id")==row.get("provider_request_id") and matched.get("quota")==row.get("quota")
                and row.get("probe_response_request_id")==probe["response"].get("request_id")
                and metadata.get("model_ratio")==receipt.get("input_per_mtok")/2
                and metadata.get("completion_ratio")==receipt.get("output_per_mtok")/receipt.get("input_per_mtok")
                and metadata.get("group_ratio")==receipt.get("group_multiplier"))
    except (KeyError,TypeError,ValueError,ZeroDivisionError): return False


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
                                negative = value.get("status") == "unsupported"
                                result_ok = (
                                    receipt.get("result") in {"fail","blocked"}
                                    and receipt.get("offline_verifier",{}).get("status") == "failed"
                                    if negative else
                                    receipt.get("result") == "pass" and receipt.get("classification") == "verified"
                                    and receipt.get("offline_verifier",{}).get("status") == "passed"
                                )
                                if (receipt.get("schema_version") != 2
                                        or receipt.get("network_execution") != "explicit_live"
                                        or not result_ok
                                        or receipt.get("response", {}).get("http_status") not in range(200,600)
                                        or not target_subject.get("model_id")):
                                    failures.append(f"{path}: incomplete live provider receipt")
                            elif kind == "provider_billing_reconciliation":
                                rows = receipt.get("rows")
                                numeric = lambda x: isinstance(x,(int,float)) and not isinstance(x,bool) and math.isfinite(x) and x >= 0
                                tariff = [receipt.get(k) for k in ("input_per_mtok","output_per_mtok","cache_read_per_mtok","group_multiplier","quota_per_charge_unit","rounding_tolerance")]
                                if (not isinstance(rows,list) or len(rows)<3 or not all(numeric(x) for x in tariff)
                                        or tariff[4] <= 0 or tariff[5] > 1 / tariff[4]):
                                    failures.append(f"{path}: incomplete provider billing reconciliation")
                                else:
                                    seen = set()
                                    for row in rows:
                                        if not isinstance(row,dict) or not provider_invoice_link_matches(receipt,row,root):
                                            failures.append(f"{path}: provider invoice/probe linkage missing or ambiguous")
                                            continue
                                        rid = row.get("provider_request_id")
                                        values = [row.get(k) for k in ("input_tokens","output_tokens","cached_input_tokens","quota","actual_charge")]
                                        if not rid or rid in seen or not all(numeric(x) for x in values) or values[2]>values[0] or row.get("match_count") != 1:
                                            failures.append(f"{path}: invalid or ambiguous provider invoice row")
                                            continue
                                        seen.add(rid)
                                        inp,out,cached,quota,actual=values
                                        expected=((inp-cached)*tariff[0]+out*tariff[1]+cached*tariff[2])/1000000*tariff[3]
                                        if abs(actual-quota/tariff[4])>1e-12 or abs(actual-expected)>tariff[5]+1e-12:
                                            failures.append(f"{path}: provider invoice cost mismatch")
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

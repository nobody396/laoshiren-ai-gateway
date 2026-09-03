#!/usr/bin/env python3
"""Harvest secret-free *candidate* evidence from existing local records.

This command is deliberately offline and read-only with respect to model
contracts.  It inventories claims already present in model contracts, optional
``log.md`` files, and explicitly supplied rollout summaries.  The inventory is
useful for migration planning, but never upgrades a prose summary into exact
terminal proof.

Examples::

    python3 scripts/harvest_model_evidence.py plan --contracts model-doc-contracts
    python3 scripts/harvest_model_evidence.py write --contracts model-doc-contracts \
      --log log.md --rollout-summary /tmp/rollout-summary.md \
      --output /tmp/model-evidence-inventory.json
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import tempfile
from typing import Any, Iterable


SCHEMA_VERSION = 1
KIND = "historical_model_evidence_inventory"
MAX_SOURCE_BYTES = 8 * 1024 * 1024
MAX_TEXT_CHUNKS = 5_000

PROTOCOL_PATTERNS = {
    "responses": re.compile(r"(?i)(?:\bResponses(?:\s+API)?\b|/v1/responses\b)"),
    "chat_completions": re.compile(
        r"(?i)(?:\bChat[ _-]?Completions?\b|/v1/chat/completions\b|\bchat_completions\b)"
    ),
    "messages": re.compile(r"(?i)(?:\bAnthropic\s+Messages\b|/v1/messages\b|\bmessages\s+protocol\b)"),
    "generate_content": re.compile(
        r"(?i)(?:\bGenerateContent\b|\bgenerate_content\b|\bGemini\s+Generate\s*Content\b)"
    ),
}
OS_PATTERNS = {
    "macos": re.compile(r"(?i)(?<![A-Za-z])(?:macOS|Mac\s+OS)(?![A-Za-z])"),
    "windows": re.compile(r"(?i)(?<![A-Za-z])Windows(?![A-Za-z])"),
    "linux": re.compile(r"(?i)(?<![A-Za-z])Linux(?![A-Za-z])"),
}
DATE_RE = re.compile(r"(?<!\d)(20\d{2}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01]))(?!\d)")
VERSION_RE = re.compile(r"(?i)\b(?:app\s*[:=]\s*|cli\s*[:=]\s*|v(?:ersion)?\s*)?(\d+\.\d+(?:\.\d+)?(?:[-+][0-9A-Za-z.-]+)?)\b")

# These patterns redact values, not harmless words such as max_output_tokens or
# token usage.  The redacted text is the only text ever copied to the inventory.
SECRET_PATTERNS = (
    re.compile(r"-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----", re.I | re.S),
    re.compile(r"(?i)(\bAuthorization\s*[:=]\s*)(?:Bearer|Basic)?\s*[^\s,;\]}]+"),
    re.compile(r"(?i)(\bBearer\s+)[A-Za-z0-9._~+/-]{8,}={0,2}"),
    re.compile(
        r"(?i)((?:[A-Z][A-Z0-9_]*(?:API_?KEY|TOKEN|SECRET|PASSWORD)|api[_-]?key|access[_-]?token|refresh[_-]?token|auth[_-]?token|token)"
        r"[\"']?\s*[:=]\s*[\"']?)[^\s,;\"'\]}]+"
    ),
    re.compile(r"\b(?:sk|rk|pk)-[A-Za-z0-9_-]{8,}\b"),
    re.compile(r"\bgh[opusr]_[A-Za-z0-9]{20,}\b"),
    re.compile(r"\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b"),
)


class HarvestError(ValueError):
    """Input cannot be harvested safely or unambiguously."""


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def digest_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sanitize_text(value: str) -> tuple[str, int]:
    """Return text safe to persist and the number of credential redactions."""

    result = value.replace("\x00", "")
    redactions = 0
    for pattern in SECRET_PATTERNS:
        if pattern.groups:
            result, count = pattern.subn(lambda match: match.group(1) + "[REDACTED]", result)
        else:
            result, count = pattern.subn("[REDACTED]", result)
        redactions += count
    # Bounded, single-line summaries keep inventories reviewable and avoid
    # copying an entire terminal transcript into a derived artifact.
    result = re.sub(r"\s+", " ", result).strip()
    if len(result) > 1_000:
        result = result[:997].rstrip() + "..."
    return result, redactions


def _read_source(path: Path) -> bytes:
    try:
        size = path.stat().st_size
    except OSError as exc:
        raise HarvestError(f"cannot stat source {path}: {exc}") from exc
    if size > MAX_SOURCE_BYTES:
        raise HarvestError(f"source is larger than {MAX_SOURCE_BYTES} bytes: {path}")
    try:
        return path.read_bytes()
    except OSError as exc:
        raise HarvestError(f"cannot read source {path}: {exc}") from exc


def _source(path: Path, source_type: str, raw: bytes, redactions: int) -> dict[str, Any]:
    sha256 = digest_bytes(raw)
    safe_uri, uri_redactions = sanitize_text(path.resolve().as_uri())
    return {
        "artifact_id": f"artifact-{sha256[:20]}",
        "artifact_uri": safe_uri,
        "sha256": sha256,
        "source_type": source_type,
        "summary_only": source_type == "rollout_summary",
        "redactions": redactions + uri_redactions,
    }


def _first_date(text: str) -> str | None:
    match = DATE_RE.search(text)
    return match.group(1) if match else None


def _explicit_os(text: str) -> str | None:
    found = [name for name, pattern in OS_PATTERNS.items() if pattern.search(text)]
    return found[0] if len(found) == 1 else None


def _normal_os(value: Any) -> str | None:
    if not isinstance(value, str):
        return None
    key = value.strip().lower().replace(" ", "")
    return {"macos": "macos", "macosx": "macos", "windows": "windows", "linux": "linux"}.get(key)


def _record(
    *,
    source: dict[str, Any],
    summary: str,
    target: dict[str, str],
    claimed_status: str | None = None,
    observed_at: str | None = None,
    redactions: int = 0,
    feature: str | None = None,
) -> dict[str, Any] | None:
    safe_summary, extra_redactions = sanitize_text(summary)
    if not safe_summary:
        return None
    target = {key: value for key, value in target.items() if isinstance(value, str) and value.strip()}
    if not target:
        return None
    proof_reason = (
        "rollout summaries are navigation aids, not immutable terminal receipts"
        if source["source_type"] == "rollout_summary"
        else "the harvester preserves an existing claim but does not verify its underlying terminal artifact"
    )
    body: dict[str, Any] = {
        "artifact_id": source["artifact_id"],
        "artifact_uri": source["artifact_uri"],
        "artifact_sha256": source["sha256"],
        "source_type": source["source_type"],
        "target": target,
        "summary": safe_summary,
        "proof": {
            "classification": "summary_only" if source["source_type"] == "rollout_summary" else "candidate_claim",
            "exact_terminal_proof": False,
            "reusable_as_terminal_evidence": False,
            "reason": proof_reason,
        },
        "secret_free": True,
        "redactions": redactions + extra_redactions,
    }
    if claimed_status:
        body["claimed_status"] = claimed_status
    if observed_at:
        body["observed_at"] = observed_at
    if feature:
        body["feature"] = feature
    fingerprint = digest_bytes(canonical_json(body))
    return {"evidence_id": f"harvest-{fingerprint[:24]}", **body}


def _date_from_cell(cell: dict[str, Any], summary: str) -> str | None:
    # A date in the exact evidence string is strongest.  A date explicitly
    # attached to the same structured cell is also exact.  Contract-wide dates
    # are intentionally not propagated into client/protocol cells.
    return _first_date(summary) or next(
        (
            value[:10]
            for key in ("verified_at", "observed_at")
            if isinstance((value := cell.get(key)), str) and DATE_RE.fullmatch(value[:10])
        ),
        None,
    )


def _contract_records(path: Path, raw: bytes) -> tuple[dict[str, Any] | None, list[dict[str, Any]]]:
    try:
        contract = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise HarvestError(f"invalid JSON contract {path}: {exc}") from exc
    if not isinstance(contract, dict):
        raise HarvestError(f"contract root must be an object: {path}")
    model_id = contract.get("model", {}).get("id") if isinstance(contract.get("model"), dict) else None
    if not isinstance(model_id, str) or not model_id.strip():
        return None, []

    # Count credential redactions over the full source without ever persisting
    # the sanitized full document.
    _, source_redactions = sanitize_text(raw.decode("utf-8", errors="replace"))
    source = _source(path, "model_contract", raw, source_redactions)
    records: list[dict[str, Any]] = []

    for protocol in contract.get("protocols", []) if isinstance(contract.get("protocols"), list) else []:
        if not isinstance(protocol, dict):
            continue
        name, summary = protocol.get("name"), protocol.get("evidence")
        if isinstance(name, str) and isinstance(summary, str):
            item = _record(
                source=source,
                summary=summary,
                target={"model_id": model_id, "protocol": name},
                claimed_status=protocol.get("status") if isinstance(protocol.get("status"), str) else None,
                observed_at=_date_from_cell(protocol, summary),
            )
            if item:
                records.append(item)

    for client in contract.get("clients", []) if isinstance(contract.get("clients"), list) else []:
        if not isinstance(client, dict):
            continue
        name, version = client.get("name"), client.get("version")
        protocol, summary = client.get("protocol"), client.get("evidence")
        if not all(isinstance(value, str) and value.strip() for value in (name, version, protocol, summary)):
            continue
        target = {
            "model_id": model_id,
            "protocol": protocol,
            "client_id": name,
            "client_version": version,
        }
        explicit_os = _explicit_os(summary)
        if explicit_os:
            target["os"] = explicit_os
        item = _record(
            source=source,
            summary=summary,
            target=target,
            claimed_status=client.get("status") if isinstance(client.get("status"), str) else None,
            observed_at=_date_from_cell(client, summary),
        )
        if item:
            records.append(item)

    matrix = contract.get("test_matrix")
    if isinstance(matrix, dict):
        protocols = matrix.get("protocols")
        if isinstance(protocols, dict):
            for protocol_name, checks in protocols.items():
                if not isinstance(protocol_name, str) or not isinstance(checks, dict):
                    continue
                for feature, cell in checks.items():
                    if not isinstance(feature, str) or not isinstance(cell, dict) or not isinstance(cell.get("evidence"), str):
                        continue
                    summary = cell["evidence"]
                    item = _record(
                        source=source,
                        summary=summary,
                        target={"model_id": model_id, "protocol": protocol_name},
                        claimed_status=cell.get("status") if isinstance(cell.get("status"), str) else None,
                        observed_at=_date_from_cell(cell, summary),
                        feature=feature,
                    )
                    if item:
                        records.append(item)

        clients = matrix.get("clients")
        if isinstance(clients, dict):
            for client_name, protocol_cells in clients.items():
                if not isinstance(client_name, str) or not isinstance(protocol_cells, dict):
                    continue
                for protocol_name, os_cells in protocol_cells.items():
                    if not isinstance(protocol_name, str) or not isinstance(os_cells, dict):
                        continue
                    for os_name, cell in os_cells.items():
                        if not isinstance(cell, dict) or not isinstance(cell.get("evidence"), str):
                            continue
                        cell_model = cell.get("model_id")
                        cell_protocol = cell.get("protocol")
                        version = cell.get("client_version")
                        explicit_os = _normal_os(cell.get("os")) or _normal_os(os_name)
                        # Structured values are included only when explicitly
                        # present and consistent with the surrounding path.
                        target = {
                            "model_id": cell_model if isinstance(cell_model, str) else model_id,
                            "protocol": cell_protocol if isinstance(cell_protocol, str) else protocol_name,
                            "client_id": client_name,
                        }
                        if isinstance(version, str) and version.strip():
                            target["client_version"] = version
                        if explicit_os:
                            target["os"] = explicit_os
                        summary = cell["evidence"]
                        item = _record(
                            source=source,
                            summary=summary,
                            target=target,
                            claimed_status=cell.get("status") if isinstance(cell.get("status"), str) else None,
                            observed_at=_date_from_cell(cell, summary),
                        )
                        if item:
                            records.append(item)

    return source, records


def _chunks_from_markdown(text: str) -> Iterable[str]:
    buffer: list[str] = []
    for line in text.splitlines():
        stripped = line.strip()
        starts_item = bool(re.match(r"^(?:[-*+]\s+|\d+[.)]\s+|#{1,6}\s+)", stripped))
        if (not stripped or starts_item) and buffer:
            yield " ".join(buffer)
            buffer = []
        if stripped:
            buffer.append(stripped)
        if starts_item and buffer:
            yield " ".join(buffer)
            buffer = []
    if buffer:
        yield " ".join(buffer)


def _strings_from_json(value: Any) -> Iterable[str]:
    if isinstance(value, dict):
        # Prefer semantic prose fields.  Recursion still supports the common
        # rollout JSONL envelope without serializing arbitrary binary payloads.
        for key, child in value.items():
            if key in {"text", "summary", "message", "content", "payload"}:
                yield from _strings_from_json(child)
            elif isinstance(child, (dict, list)):
                yield from _strings_from_json(child)
    elif isinstance(value, list):
        for child in value:
            yield from _strings_from_json(child)
    elif isinstance(value, str):
        yield value


def _text_chunks(path: Path, raw: bytes) -> list[str]:
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise HarvestError(f"source is not UTF-8 text: {path}") from exc
    chunks: list[str] = []
    if path.suffix.lower() == ".jsonl":
        for line_no, line in enumerate(text.splitlines(), 1):
            if not line.strip():
                continue
            try:
                value = json.loads(line)
            except json.JSONDecodeError as exc:
                raise HarvestError(f"invalid JSONL at {path}:{line_no}: {exc}") from exc
            chunks.extend(_strings_from_json(value))
            if len(chunks) > MAX_TEXT_CHUNKS:
                raise HarvestError(f"too many text chunks in source: {path}")
    else:
        chunks.extend(_chunks_from_markdown(text))
    return chunks[:MAX_TEXT_CHUNKS]


def _known_entities(contracts: list[dict[str, Any]]) -> tuple[list[str], dict[str, set[str]]]:
    models: set[str] = set()
    clients: dict[str, set[str]] = {}
    for contract in contracts:
        model = contract.get("model")
        if isinstance(model, dict) and isinstance(model.get("id"), str):
            models.add(model["id"])
        for client in contract.get("clients", []) if isinstance(contract.get("clients"), list) else []:
            if not isinstance(client, dict) or not isinstance(client.get("name"), str):
                continue
            versions = clients.setdefault(client["name"], set())
            if isinstance(client.get("version"), str):
                versions.add(client["version"])
    return sorted(models, key=len, reverse=True), clients


def _text_records(
    path: Path,
    raw: bytes,
    source_type: str,
    known_models: list[str],
    known_clients: dict[str, set[str]],
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    chunks = _text_chunks(path, raw)
    sanitized: list[tuple[str, int]] = [sanitize_text(chunk) for chunk in chunks]
    source_redactions = sum(count for _, count in sanitized)
    source = _source(path, source_type, raw, source_redactions)
    records: list[dict[str, Any]] = []

    for text, redactions in sanitized:
        if not text:
            continue
        models = [model for model in known_models if re.search(rf"(?<![A-Za-z0-9_.-]){re.escape(model)}(?![A-Za-z0-9_.-])", text, re.I)]
        protocols = [name for name, pattern in PROTOCOL_PATTERNS.items() if pattern.search(text)]
        clients = [name for name in known_clients if re.search(rf"(?<![A-Za-z0-9]){re.escape(name)}(?![A-Za-z0-9])", text, re.I)]

        # Only form an exact target when each mentioned dimension is singular.
        # Ambiguous multi-model/multi-protocol summaries remain navigation text
        # and do not create false Cartesian evidence.
        if len(models) != 1:
            continue
        target: dict[str, str] = {"model_id": models[0]}
        if len(protocols) == 1:
            target["protocol"] = protocols[0]
        if len(clients) == 1:
            client = clients[0]
            target["client_id"] = client
            # Prefer an exact known version literally present in the same chunk.
            exact_versions = [version for version in known_clients[client] if version and version in text]
            if len(exact_versions) == 1:
                target["client_version"] = exact_versions[0]
            elif not known_clients[client]:
                # For a previously unseen version, accept only one explicit
                # numeric version in the same client-specific chunk.
                versions = VERSION_RE.findall(text)
                if len(versions) == 1:
                    target["client_version"] = versions[0]
        explicit_os = _explicit_os(text)
        if explicit_os:
            target["os"] = explicit_os
        item = _record(
            source=source,
            summary=text,
            target=target,
            observed_at=_first_date(text),
            redactions=redactions,
        )
        if item:
            records.append(item)
    return source, records


def _load_contract_documents(paths: list[Path]) -> list[dict[str, Any]]:
    documents: list[dict[str, Any]] = []
    for path in paths:
        raw = _read_source(path)
        try:
            value = json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise HarvestError(f"invalid JSON contract {path}: {exc}") from exc
        if isinstance(value, dict) and isinstance(value.get("model"), dict) and isinstance(value["model"].get("id"), str):
            documents.append(value)
    return documents


def build_inventory(contracts_dir: Path, logs: list[Path], rollout_summaries: list[Path]) -> dict[str, Any]:
    if not contracts_dir.is_dir():
        raise HarvestError(f"contracts directory does not exist: {contracts_dir}")
    contract_paths: list[Path] = []
    for path in sorted(contracts_dir.glob("*.json")):
        try:
            value = json.loads(_read_source(path).decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise HarvestError(f"invalid JSON contract {path}: {exc}") from exc
        if isinstance(value, dict) and isinstance(value.get("model"), dict) and isinstance(value["model"].get("id"), str):
            contract_paths.append(path)
    contract_documents = _load_contract_documents(contract_paths)
    known_models, known_clients = _known_entities(contract_documents)

    sources: list[dict[str, Any]] = []
    records: list[dict[str, Any]] = []
    for path in contract_paths:
        source, harvested = _contract_records(path, _read_source(path))
        if source:
            sources.append(source)
            records.extend(harvested)
    for source_type, paths in (("log_md", logs), ("rollout_summary", rollout_summaries)):
        for path in sorted({item.resolve() for item in paths}):
            source, harvested = _text_records(path, _read_source(path), source_type, known_models, known_clients)
            sources.append(source)
            records.extend(harvested)

    # Stable de-duplication is important because the same prose is often copied
    # from a top-level client claim into a test_matrix cell.
    by_id = {record["evidence_id"]: record for record in records}
    records = [by_id[key] for key in sorted(by_id)]
    sources = sorted(sources, key=lambda item: (item["source_type"], item["artifact_uri"]))
    body = {
        "schema_version": SCHEMA_VERSION,
        "kind": KIND,
        "secret_free": True,
        "network_execution": "disabled",
        "terminal_proof_policy": "summary claims are candidates only; exact terminal receipts remain required",
        "sources": sources,
        "evidence": records,
        "summary": {
            "sources": len(sources),
            "model_contracts": sum(source["source_type"] == "model_contract" for source in sources),
            "log_files": sum(source["source_type"] == "log_md" for source in sources),
            "rollout_summaries": sum(source["source_type"] == "rollout_summary" for source in sources),
            "candidate_records": len(records),
            "records_with_date": sum("observed_at" in record for record in records),
            "records_with_os": sum("os" in record["target"] for record in records),
            "records_with_client_version": sum("client_version" in record["target"] for record in records),
            "redactions": sum(source["redactions"] for source in sources),
            "terminal_records": 0,
        },
    }
    sha256 = digest_bytes(canonical_json(body))
    return {
        **body,
        "artifact_id": f"evidence-inventory-{sha256[:20]}",
        "sha256": sha256,
    }


def _write_atomic(path: Path, inventory: dict[str, Any]) -> bool:
    payload = json.dumps(inventory, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    if path.exists() and path.read_text(encoding="utf-8") == payload:
        return False
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    return True


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("plan", "write"))
    parser.add_argument("--contracts", type=Path, default=Path("model-doc-contracts"))
    parser.add_argument("--log", action="append", type=Path, default=[], help="optional log.md path; repeatable")
    parser.add_argument(
        "--rollout-summary", action="append", type=Path, default=[],
        help="explicit rollout summary (.md or .jsonl) path; repeatable",
    )
    parser.add_argument("--output", type=Path, help="required by write")
    args = parser.parse_args(argv)
    try:
        if args.command == "write" and args.output is None:
            raise HarvestError("write requires --output")
        inventory = build_inventory(args.contracts, args.log, args.rollout_summary)
        if args.command == "plan":
            print(json.dumps(inventory, ensure_ascii=False, indent=2, sort_keys=True))
        else:
            assert args.output is not None
            changed = _write_atomic(args.output, inventory)
            print(json.dumps({
                "artifact_id": inventory["artifact_id"],
                "sha256": inventory["sha256"],
                "output": sanitize_text(str(args.output))[0],
                "changed": changed,
                "summary": inventory["summary"],
            }, ensure_ascii=False, indent=2, sort_keys=True))
        return 0
    except (HarvestError, OSError, UnicodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

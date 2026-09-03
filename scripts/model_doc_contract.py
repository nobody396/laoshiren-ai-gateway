#!/usr/bin/env python3
"""Initialize, validate, and summarize a secret-free model documentation contract."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
from typing import Any
from urllib.parse import urlparse

PROTOCOLS = {"responses", "chat_completions", "messages", "generate_content", "images"}
MODALITIES = {"text", "image", "video"}
PUBLIC_STATUSES = {"verified", "unsupported"}
PROTOCOL_STATUSES = {"verified", "blocked", "unsupported"}
CLIENT_COVERAGE_STATUSES = {"verified", "blocked", "unsupported"}
SCRIPT_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CLIENT_MATRIX_CANDIDATES = (
    SCRIPT_ROOT / "references" / "client-matrix.json",
    SCRIPT_ROOT / "model-doc-contracts" / "client-matrix.json",
)
SECRET_KEY = re.compile(r"(api[_-]?key|access[_-]?token|refresh[_-]?token|password|credential|secret)", re.I)
SECRET_VALUE = re.compile(r"(?:sk-|Bearer\s+)[A-Za-z0-9_-]{8,}", re.I)
MODEL_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]*$")


def fail(message: str) -> None:
    raise ValueError(message)


def walk(value: Any, path: str = "$") -> None:
    if isinstance(value, dict):
        for key, item in value.items():
            if SECRET_KEY.search(str(key)):
                fail(f"forbidden secret-shaped field: {path}.{key}")
            walk(item, f"{path}.{key}")
    elif isinstance(value, list):
        for index, item in enumerate(value):
            walk(item, f"{path}[{index}]")
    elif isinstance(value, str) and SECRET_VALUE.search(value):
        fail(f"secret-shaped value at {path}")


def require_dict(value: Any, path: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        fail(f"{path} must be an object")
    return value


def require_list(value: Any, path: str) -> list[Any]:
    if not isinstance(value, list):
        fail(f"{path} must be an array")
    return value


def require_text(value: Any, path: str) -> str:
    if not isinstance(value, str) or not value.strip():
        fail(f"{path} is required")
    return value.strip()


def require_positive_int(value: Any, path: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
        fail(f"{path} must be a positive integer")
    return value


def load_client_matrix(path: Path | None = None) -> dict[str, Any]:
    matrix_path = path or next(
        (candidate for candidate in DEFAULT_CLIENT_MATRIX_CANDIDATES if candidate.exists()),
        DEFAULT_CLIENT_MATRIX_CANDIDATES[0],
    )
    matrix = json.loads(matrix_path.read_text(encoding="utf-8"))
    if not isinstance(matrix, dict) or not isinstance(matrix.get("clients"), list):
        fail("client matrix must contain a clients array")
    return matrix


def protocol_clients(client_matrix: dict[str, Any]) -> dict[str, set[str]]:
    result = {protocol: set() for protocol in PROTOCOLS}
    for index, raw in enumerate(client_matrix["clients"]):
        client = require_dict(raw, f"client_matrix.clients[{index}]")
        name = require_text(client.get("name"), f"client_matrix.clients[{index}].name")
        protocols = client.get("protocols")
        if not isinstance(protocols, list):
            rows = client.get("client_protocol", {}).get("protocols", [])
            protocols = [
                row.get("protocol")
                for row in rows
                if isinstance(row, dict) and row.get("support") == "supported"
            ]
        protocols = require_list(protocols, f"client_matrix.clients[{index}].protocols")
        for protocol in protocols:
            if protocol not in PROTOCOLS - {"images"}:
                fail(f"client_matrix.clients[{index}] has invalid protocol: {protocol}")
            result[protocol].add(name)
    return result


def validate(data: dict[str, Any], client_matrix: dict[str, Any] | None = None) -> dict[str, Any]:
    walk(data)
    client_matrix = client_matrix or load_client_matrix()
    protocol_client_names = protocol_clients(client_matrix)
    if data.get("schema_version") != 1:
        fail("schema_version must be 1")

    model = require_dict(data.get("model"), "model")
    model_id = require_text(model.get("id"), "model.id")
    if not MODEL_ID.fullmatch(model_id):
        fail("model.id is not safe")
    for field in ("display_name", "family"):
        require_text(model.get(field), f"model.{field}")
    require_positive_int(model.get("context_window"), "model.context_window")
    max_output_tokens = model.get("max_output_tokens")
    if max_output_tokens is None:
        limit_cell = data.get("test_matrix", {}).get("limits", {}).get("max_output_tokens", {})
        if not isinstance(limit_cell, dict) or limit_cell.get("status") not in {"blocked", "not_published"}:
            fail("model.max_output_tokens may be null only with blocked or not_published max-output evidence")
    else:
        require_positive_int(max_output_tokens, "model.max_output_tokens")

    inputs = require_list(model.get("input_modalities"), "model.input_modalities")
    outputs = require_list(model.get("output_modalities"), "model.output_modalities")
    if not inputs or set(inputs) - MODALITIES:
        fail("model.input_modalities must be a non-empty subset of text/image/video")
    if not outputs or any(not isinstance(item, str) or not item.strip() for item in outputs):
        fail("model.output_modalities must contain non-empty strings")

    access = require_dict(data.get("access"), "access")
    base_url = require_text(access.get("base_url"), "access.base_url")
    parsed = urlparse(base_url)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        fail("access.base_url must be an absolute HTTP(S) URL")
    groups = require_list(access.get("groups"), "access.groups")
    if not groups:
        fail("access.groups must not be empty")
    for index, raw in enumerate(groups):
        group = require_dict(raw, f"access.groups[{index}]")
        require_text(group.get("name"), f"access.groups[{index}].name")
        multiplier = group.get("multiplier")
        if isinstance(multiplier, bool) or not isinstance(multiplier, (int, float)) or multiplier <= 0:
            fail(f"access.groups[{index}].multiplier must be positive")

    protocols = require_list(data.get("protocols"), "protocols")
    verified_protocols: set[str] = set()
    if not protocols:
        fail("protocols must not be empty")
    for index, raw in enumerate(protocols):
        protocol = require_dict(raw, f"protocols[{index}]")
        name = require_text(protocol.get("name"), f"protocols[{index}].name")
        if name not in PROTOCOLS:
            fail(f"protocols[{index}].name is invalid")
        status = require_text(protocol.get("status"), f"protocols[{index}].status")
        if status not in PROTOCOL_STATUSES:
            fail(f"protocols[{index}].status must be verified, blocked, or unsupported")
        require_text(protocol.get("evidence"), f"protocols[{index}].evidence")
        if status == "verified":
            verified_protocols.add(name)
    if not verified_protocols:
        fail("at least one protocol must be verified")
    recommended_protocol = require_text(data.get("recommended_protocol"), "recommended_protocol")
    if recommended_protocol not in verified_protocols:
        fail("recommended_protocol must reference a verified protocol")

    clients = require_list(data.get("clients"), "clients")
    draft_gateway = data.get("verification", {}).get("gateway_e2e") is False
    if not clients and not draft_gateway:
        fail("clients must not be empty after gateway E2E")
    for index, raw in enumerate(clients):
        client = require_dict(raw, f"clients[{index}]")
        require_text(client.get("name"), f"clients[{index}].name")
        require_text(client.get("version"), f"clients[{index}].version")
        protocol = require_text(client.get("protocol"), f"clients[{index}].protocol")
        if protocol not in verified_protocols:
            fail(f"clients[{index}].protocol must reference a verified protocol")
        if client.get("status") != "verified":
            fail(f"clients[{index}].status must be verified before public rendering")
        require_text(client.get("evidence"), f"clients[{index}].evidence")
        if not isinstance(client.get("recommended"), bool):
            fail(f"clients[{index}].recommended must be boolean")

    coverage = require_list(data.get("client_coverage"), "client_coverage")
    coverage_by_name: dict[str, dict[str, Any]] = {}
    for index, raw in enumerate(coverage):
        item = require_dict(raw, f"client_coverage[{index}]")
        name = require_text(item.get("name"), f"client_coverage[{index}].name")
        if name in coverage_by_name:
            fail(f"client_coverage contains duplicate client: {name}")
        status = require_text(item.get("status"), f"client_coverage[{index}].status")
        if status not in CLIENT_COVERAGE_STATUSES:
            fail(f"client_coverage[{index}].status is invalid")
        item_protocols = require_list(item.get("protocols"), f"client_coverage[{index}].protocols")
        if (status == "verified" and not item_protocols) or any(protocol not in verified_protocols for protocol in item_protocols):
            fail(f"client_coverage[{index}].protocols must reference verified protocols")
        for protocol in item_protocols:
            if name not in protocol_client_names[protocol]:
                fail(f"client_coverage[{index}] lists incompatible client/protocol: {name}/{protocol}")
        require_text(item.get("evidence"), f"client_coverage[{index}].evidence")
        coverage_by_name[name] = item

    required_clients = set().union(*(protocol_client_names[protocol] for protocol in verified_protocols))
    missing_clients = sorted(required_clients - coverage_by_name.keys())
    if missing_clients:
        fail("client_coverage is missing protocol-compatible clients: " + ", ".join(missing_clients))
    verified_client_names = {client["name"] for client in clients}
    coverage_verified_names = {name for name, item in coverage_by_name.items() if item["status"] == "verified"}
    if verified_client_names != coverage_verified_names:
        fail("clients and verified client_coverage entries must match")

    verification = require_dict(data.get("verification"), "verification")
    official_url = require_text(verification.get("official_spec_url"), "verification.official_spec_url")
    parsed_official = urlparse(official_url)
    if parsed_official.scheme != "https" or not parsed_official.netloc:
        fail("verification.official_spec_url must be an absolute HTTPS URL")
    require_text(verification.get("verified_at"), "verification.verified_at")
    if verification.get("limits_source") not in {"official", "live"}:
        fail("verification.limits_source must be official or live")
    if not isinstance(verification.get("gateway_e2e"), bool):
        fail("verification.gateway_e2e must be boolean")
    modality_results = require_dict(verification.get("modalities"), "verification.modalities")
    if set(modality_results) != MODALITIES:
        fail("verification.modalities must explicitly cover text, image, and video")
    for modality, status in modality_results.items():
        if status not in PUBLIC_STATUSES:
            fail(f"verification.modalities.{modality} must be verified or unsupported")
        supported = modality in inputs
        if supported != (status == "verified"):
            fail(f"model.input_modalities conflicts with verification.modalities.{modality}")

    reasoning = data.get("reasoning")
    if reasoning is not None:
        reasoning = require_dict(reasoning, "reasoning")
        levels = require_list(reasoning.get("model_levels"), "reasoning.model_levels")
        if not all(isinstance(item, str) and item.strip() for item in levels):
            fail("reasoning.model_levels must contain non-empty strings")
        client_levels = require_list(reasoning.get("client_levels", []), "reasoning.client_levels")
        for index, raw in enumerate(client_levels):
            item = require_dict(raw, f"reasoning.client_levels[{index}]")
            require_text(item.get("client"), f"reasoning.client_levels[{index}].client")
            values = require_list(item.get("levels"), f"reasoning.client_levels[{index}].levels")
            if not values or not all(isinstance(value, str) and value.strip() for value in values):
                fail(f"reasoning.client_levels[{index}].levels must contain non-empty strings")
        mappings = require_list(reasoning.get("client_mappings"), "reasoning.client_mappings")
        for index, raw in enumerate(mappings):
            mapping = require_dict(raw, f"reasoning.client_mappings[{index}]")
            for field in ("client", "from", "to"):
                require_text(mapping.get(field), f"reasoning.client_mappings[{index}].{field}")

    return data


def is_publishable(data: dict[str, Any]) -> bool:
    """A public card requires gateway proof and a closed client matrix."""

    return (
        (
            isinstance(data.get("model", {}).get("max_output_tokens"), int)
            or (
                data.get("model", {}).get("max_output_tokens") is None
                and data.get("test_matrix", {}).get("limits", {}).get("max_output_tokens", {}).get("status") == "not_published"
            )
        )
        and bool(data["verification"]["gateway_e2e"])
        and all(item.get("status") != "blocked" for item in data.get("protocols", []))
        and all(
        item["status"] != "blocked" for item in data["client_coverage"]
        )
    )


def template(model_id: str) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "model": {
            "id": model_id,
            "display_name": model_id,
            "family": "TODO",
            "context_window": 1,
            "max_output_tokens": 1,
            "input_modalities": ["text"],
            "output_modalities": ["text"],
        },
        "access": {
            "base_url": "https://api.laoshirenai.com/v1",
            "groups": [{"name": "TODO", "multiplier": 1}],
        },
        "protocols": [{
            "name": "chat_completions",
            "status": "verified",
            "evidence": "TODO",
        }],
        "recommended_protocol": "chat_completions",
        "clients": [{
            "name": "TODO",
            "version": "TODO",
            "protocol": "chat_completions",
            "status": "verified",
            "recommended": False,
            "evidence": "TODO",
        }],
        "client_coverage": [],
        "reasoning": {"model_levels": [], "client_levels": [], "client_mappings": []},
        "test_matrix": {
            "limits": {},
            "modalities": {},
            "protocols": {},
            "clients": {},
            "reasoning": {"model_levels": {}, "clients": {}},
        },
        "verification": {
            "official_spec_url": "https://example.invalid/TODO",
            "verified_at": "YYYY-MM-DD",
            "limits_source": "official",
            "gateway_e2e": False,
            "modalities": {
                "text": "verified",
                "image": "unsupported",
                "video": "unsupported",
            },
        },
    }


def load(path: Path) -> dict[str, Any]:
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        fail("contract root must be an object")
    return data


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    init = sub.add_parser("init")
    init.add_argument("--model-id", required=True)
    init.add_argument("--output", type=Path, required=True)
    init.add_argument("--force", action="store_true")

    validate_cmd = sub.add_parser("validate")
    validate_cmd.add_argument("contract", type=Path)
    validate_cmd.add_argument("--client-matrix", type=Path)

    plan_cmd = sub.add_parser("plan")
    plan_cmd.add_argument("contract", type=Path)
    plan_cmd.add_argument("--client-matrix", type=Path)

    args = parser.parse_args()
    try:
        if args.command == "init":
            if not MODEL_ID.fullmatch(args.model_id):
                fail("invalid model id")
            if args.output.exists() and not args.force:
                fail(f"output already exists: {args.output}")
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(json.dumps(template(args.model_id), ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            print(args.output)
            return 0

        data = validate(load(args.contract), load_client_matrix(args.client_matrix))
        if args.command == "validate":
            print(json.dumps({"valid": True, "model": data["model"]["id"]}, ensure_ascii=False))
            return 0

        print(json.dumps({
            "model": data["model"]["id"],
            "verified_protocols": [item["name"] for item in data["protocols"] if item["status"] == "verified"],
            "verified_clients": [item["name"] for item in data["clients"]],
            "card_order": [
                "identity",
                "limits-group-protocol",
                "modalities-output",
                "base-url",
                "verified-clients",
                "reasoning-caveats",
            ],
            "publishable": is_publishable(data),
        }, ensure_ascii=False, indent=2))
        return 0
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {exc}")
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

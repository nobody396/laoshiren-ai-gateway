#!/usr/bin/env python3
"""Generate the frontend model-documentation contract catalog."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
CONTRACT_DIR = ROOT / "model-doc-contracts"
CLIENT_MATRIX = CONTRACT_DIR / "client-matrix.json"
OUTPUT = ROOT / "frontend" / "src" / "generated" / "modelDocContracts.ts"
ADMIN_OUTPUT = ROOT / "backend" / "internal" / "adminmatrix" / "model_client_matrix.json"
EVIDENCE_FILE = CONTRACT_DIR / "evidence" / "test-evidence.json"
sys.path.insert(0, str(Path(__file__).resolve().parent))
from model_doc_contract import is_publishable, validate, walk  # noqa: E402
from model_doc_matrix import MATRIX_NAMES, audit_contract_sections  # noqa: E402


def _blocked_cell_count(value: Any) -> int:
    """Count evidence cells rather than the field failures they can produce."""

    if isinstance(value, dict):
        return int(value.get("status") == "blocked") + sum(
            _blocked_cell_count(child) for child in value.values()
        )
    if isinstance(value, list):
        return sum(_blocked_cell_count(child) for child in value)
    return 0


def _evidence_fields(value: Any) -> dict[str, Any]:
    if not isinstance(value, dict):
        return {"status": "blocked", "evidence": "matrix cell is missing"}
    return dict(value)


def _project_compatibility(contract: dict[str, Any]) -> dict[str, Any]:
    """Flatten the contract matrices into rows that Vue can consume directly."""

    matrix = contract.get("test_matrix")
    if not isinstance(matrix, dict):
        matrix = {}

    protocol_checks: list[dict[str, Any]] = []
    for protocol, checks in sorted((matrix.get("protocols") or {}).items()):
        if not isinstance(checks, dict):
            continue
        for check, cell in sorted(checks.items()):
            protocol_checks.append({
                "protocol": protocol,
                "check": check,
                **_evidence_fields(cell),
            })

    protocol_features: list[dict[str, Any]] = []
    for protocol, features in sorted((matrix.get("protocol_features") or {}).items()):
        if not isinstance(features, dict):
            continue
        for feature, cell in sorted(features.items()):
            protocol_features.append({
                "protocol": protocol,
                "feature": feature,
                **_evidence_fields(cell),
            })

    exact_clients: list[dict[str, Any]] = []
    for client, protocols in sorted((matrix.get("clients") or {}).items()):
        if not isinstance(protocols, dict):
            continue
        for protocol, os_cells in sorted(protocols.items()):
            if not isinstance(os_cells, dict):
                continue
            for os_name, cell in sorted(os_cells.items()):
                exact_clients.append({
                    "client": client,
                    "protocol": protocol,
                    "os": os_name,
                    **_evidence_fields(cell),
                })

    reasoning_matrix = matrix.get("reasoning")
    if not isinstance(reasoning_matrix, dict):
        reasoning_matrix = {}
    model_level_cells = reasoning_matrix.get("model_levels")
    if not isinstance(model_level_cells, dict):
        model_level_cells = {}
    declared_levels = contract.get("reasoning", {}).get("model_levels", [])
    ordered_levels = list(dict.fromkeys([
        *[level for level in declared_levels if isinstance(level, str)],
        *sorted(level for level in model_level_cells if isinstance(level, str)),
    ]))
    model_levels = [
        {"level": level, **_evidence_fields(model_level_cells.get(level))}
        for level in ordered_levels
    ]

    reasoning_clients: list[dict[str, Any]] = []
    for client, protocols in sorted((reasoning_matrix.get("clients") or {}).items()):
        if not isinstance(protocols, dict):
            continue
        for protocol, cell in sorted(protocols.items()):
            reasoning_clients.append({
                "client": client,
                "protocol": protocol,
                **_evidence_fields(cell),
            })

    group_access = matrix.get("group_access")
    if not isinstance(group_access, dict):
        group_access = {}
    pricing = matrix.get("pricing")
    if not isinstance(pricing, dict):
        pricing = {}
    group_order = [
        group.get("name") for group in contract.get("access", {}).get("groups", [])
        if isinstance(group, dict) and isinstance(group.get("name"), str)
    ]
    all_access_groups = list(dict.fromkeys([*group_order, *sorted(group_access)]))
    all_price_groups = list(dict.fromkeys([*group_order, *sorted(pricing)]))

    return {
        "protocol_checks": protocol_checks,
        "protocol_features": protocol_features,
        "tools": [dict(tool) for tool in contract.get("tools", []) if isinstance(tool, dict)],
        "exact_clients": exact_clients,
        "reasoning": {
            "model_levels": model_levels,
            "clients": reasoning_clients,
            "declared_mappings": contract.get("reasoning", {}).get("client_mappings", []),
        },
        "access": [
            {"group": group, **_evidence_fields(group_access.get(group))}
            for group in all_access_groups
        ],
        "pricing": [
            {"group": group, **_evidence_fields(pricing.get(group))}
            for group in all_price_groups
        ],
    }


def project_contract(
    contract: dict[str, Any], client_matrix: dict[str, Any],
    validation_errors: list[str] | None = None,
) -> dict[str, Any]:
    sections = audit_contract_sections(contract, client_matrix)
    audit_failures = sum(len(sections[name]) for name in MATRIX_NAMES)
    validation_errors = validation_errors or []
    publishable = not validation_errors and is_publishable(contract) and audit_failures == 0
    projected = dict(contract)
    projected["publication"] = {
        "status": "publishable" if publishable else "draft",
        "publishable": publishable,
        "missing_evidence": {
            "blocked_cells": 0 if audit_failures == 0 else _blocked_cell_count(contract.get("test_matrix")),
            "audit_failures": audit_failures,
            "validation_errors": validation_errors,
            "by_matrix": [
                {"matrix": name, "count": len(sections[name])}
                for name in MATRIX_NAMES if sections[name]
            ],
        },
    }
    projected["compatibility"] = _project_compatibility(contract)
    return projected


def _public_cell(value: dict[str, Any], *identity_fields: str) -> dict[str, Any]:
    """Keep only display facts; evidence artifacts stay in the admin payload."""

    fields = ("status", "verified_at", *identity_fields)
    return {field: value[field] for field in fields if field in value}


def project_public_contract(contract: dict[str, Any]) -> dict[str, Any]:
    """Build the small, secret-free public card projection.

    The full evidence/test/client Cartesian product is emitted only into the
    backend-admin payload. This projection intentionally cannot reconstruct
    the complete administrator matrix.
    """

    public = {
        key: contract[key]
        for key in (
            "schema_version", "model", "access", "protocols", "protocol_candidates",
            "tools", "recommended_protocol", "recommended_protocol_reason",
            "reasoning", "verification", "publication",
        )
        if key in contract
    }
    public["clients"] = [
        {
            field: client[field]
            for field in (
                "name", "version", "protocol", "status", "recommended",
                "evidence", "evidence_ids",
            )
            if field in client
        }
        for client in contract.get("clients", [])
        if isinstance(client, dict) and client.get("status") == "verified"
    ]
    public["client_coverage"] = [
        {
            field: client[field]
            for field in ("name", "protocols", "status", "evidence", "evidence_ids")
            if field in client
        }
        for client in contract.get("client_coverage", [])
        if isinstance(client, dict)
    ]
    compatibility = contract.get("compatibility") or {}
    public["compatibility"] = {
        "protocol_checks": [
            _public_cell(cell, "protocol", "check")
            for cell in compatibility.get("protocol_checks", [])
        ],
        "protocol_features": [
            _public_cell(cell, "protocol", "feature")
            for cell in compatibility.get("protocol_features", [])
        ],
        "tools": [
            _public_cell(cell, "protocol", "name")
            for cell in compatibility.get("tools", [])
        ],
        # Public cards only need positive exact-client facts. Blocked and
        # unsupported Cartesian cells remain admin-only.
        "exact_clients": [
            _public_cell(cell, "client", "client_version", "model_id", "protocol", "os")
            for cell in compatibility.get("exact_clients", [])
            if cell.get("status") == "verified"
        ],
        "reasoning": {
            "model_levels": [
                _public_cell(cell, "level")
                for cell in compatibility.get("reasoning", {}).get("model_levels", [])
            ],
            "clients": [
                _public_cell(cell, "client", "protocol", "client_levels", "mappings")
                for cell in compatibility.get("reasoning", {}).get("clients", [])
                if cell.get("status") in {"verified", "not_exposed"}
            ],
            "declared_mappings": compatibility.get("reasoning", {}).get("declared_mappings", []),
        },
        "access": [
            _public_cell(
                cell, "group", "model_id", "base_url", "multiplier", "protocols",
                "recommended_protocol", "recommended_protocol_reason",
            )
            for cell in compatibility.get("access", [])
        ],
        "pricing": [
            _public_cell(
                cell, "group", "currency", "unit", "input_price", "output_price",
                "cache_write_price", "cache_read_price", "long_context",
                "context_intervals", "time_pricing", "source_url",
            )
            for cell in compatibility.get("pricing", [])
        ],
    }
    return public


def render_admin_payload(contracts: list[dict[str, Any]]) -> str:
    client_matrix = json.loads(CLIENT_MATRIX.read_text(encoding="utf-8"))
    evidence_rows = json.loads(EVIDENCE_FILE.read_text(encoding="utf-8")).get("rows", [])
    evidence_index = {
        row["evidence_id"]: {
            field: row.get(field)
            for field in (
                "evidence_type", "result", "observed_at", "artifact_uri",
                "artifact_sha256", "summary", "target",
            )
        }
        for row in evidence_rows
        if isinstance(row, dict) and row.get("evidence_id") and row.get("secret_free") is True
    }
    payload = {
        "schema_version": 1,
        "counts": {
            "models": len(contracts),
            "clients": len(client_matrix.get("clients", [])),
            "intersections": len(contracts) * len(client_matrix.get("clients", [])),
        },
        "contracts": contracts,
        "client_matrix": client_matrix,
        "evidence_index": evidence_index,
    }
    return json.dumps(payload, ensure_ascii=False, indent=2) + "\n"


def load_contracts() -> list[dict[str, Any]]:
    contracts: list[dict[str, Any]] = []
    seen: set[str] = set()
    client_matrix = json.loads(CLIENT_MATRIX.read_text(encoding="utf-8"))
    for path in sorted(CONTRACT_DIR.glob("*.json")):
        if path == CLIENT_MATRIX or path.name == "matrix-schema.json":
            continue
        raw = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(raw, dict):
            raise ValueError(f"{path.name}: root must be an object")
        # Always reject secret-shaped content. Semantic validation failures are
        # retained as draft metadata so an incomplete contract remains visible
        # in the admin matrix instead of disappearing from generation.
        walk(raw)
        validation_errors: list[str] = []
        try:
            contract = validate(raw, client_matrix)
        except ValueError as exc:
            contract = raw
            validation_errors.append(str(exc))
        model_id = contract.get("model", {}).get("id")
        if not isinstance(model_id, str) or not model_id:
            raise ValueError(f"{path.name}: model.id is required")
        if path.stem != model_id:
            raise ValueError(f"{path.name}: filename must match model.id {model_id}")
        if model_id in seen:
            raise ValueError(f"duplicate model contract: {model_id}")
        seen.add(model_id)
        # Draft contracts are intentionally generated too. Public docs may
        # hide them, while the admin matrix needs every known contract and its
        # exact missing-evidence state.
        contracts.append(project_contract(contract, client_matrix, validation_errors))
    return contracts


def render(contracts: list[dict[str, Any]]) -> str:
    encoded = json.dumps([project_public_contract(contract) for contract in contracts], ensure_ascii=False, indent=2)
    return '''// Code generated by scripts/model_doc_catalog.py; DO NOT EDIT.

export type ModelDocProtocolName = 'responses' | 'chat_completions' | 'messages' | 'generate_content' | 'images'
export type ModelDocVerificationStatus = 'verified' | 'unsupported'
export type ModelDocProtocolStatus = ModelDocVerificationStatus | 'blocked'
export type ModelDocCellStatus = 'unknown' | 'planned' | 'verified' | 'unsupported' | 'blocked' | 'stale' | 'not_exposed' | 'not_published' | 'not_applicable'
export type ModelDocMatrixName = 'public_model' | 'model_protocol' | 'model_reasoning' | 'client_protocol' | 'client_reasoning' | 'group_access' | 'client_config_os' | 'test_evidence' | 'model_price'

export interface ModelDocEvidenceCell {
  status: ModelDocCellStatus
  evidence?: string
  evidence_ids?: string[]
  pass_evidence_ids?: string[]
  failure_evidence_ids?: string[]
  verified_at?: string
}

export interface ModelDocProtocolEvidenceSummary {
  status: ModelDocCellStatus
  evidence_ids: string[]
  pass_evidence_ids?: string[]
  failure_evidence_ids?: string[]
  observed_at?: string
  required_features?: string[]
}

export interface ModelDocPricingRow extends ModelDocEvidenceCell {
  group: string
  currency?: string
  unit?: string
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_read_price?: number | null
  long_context?: { input_threshold: number; input_multiplier: number; output_multiplier: number }
  context_intervals?: Array<{
    min_tokens: number
    max_tokens: number
    input_price: number
    output_price: number
    cache_write_price: number | null
    cache_read_price: number | null
  }>
  time_pricing?: {
    timezone: string
    periods: Array<{ start_time: string; end_time: string; multiplier: number }>
  }
  source_url?: string
}

export interface ModelDocCompatibility {
  protocol_checks: Array<ModelDocEvidenceCell & { protocol: ModelDocProtocolName; check: string }>
  protocol_features: Array<ModelDocEvidenceCell & { protocol: ModelDocProtocolName; feature: string }>
  tools: Array<ModelDocEvidenceCell & { name: string; protocol: ModelDocProtocolName }>
  exact_clients: Array<ModelDocEvidenceCell & {
    client: string
    client_version?: string
    model_id?: string
    protocol: ModelDocProtocolName
    os: string
    legacy_status?: ModelDocCellStatus
  }>
  reasoning: {
    model_levels: Array<ModelDocEvidenceCell & { level: string }>
    clients: Array<ModelDocEvidenceCell & {
      client: string
      protocol: ModelDocProtocolName
      client_levels?: string[]
      mappings?: Array<{ client: string; protocol?: ModelDocProtocolName; from: string; to: string }>
    }>
    declared_mappings: Array<{ client: string; protocol?: ModelDocProtocolName; from: string; to: string }>
  }
  access: Array<ModelDocEvidenceCell & {
    group: string
    model_id?: string
    base_url?: string
    multiplier?: number
    protocols?: ModelDocProtocolName[]
    protocol_evidence?: Partial<Record<ModelDocProtocolName, ModelDocProtocolEvidenceSummary>>
    recommended_protocol?: ModelDocProtocolName
    recommended_protocol_reason?: string
  }>
  pricing: ModelDocPricingRow[]
}

export interface ModelDocContract {
  schema_version: 1
  model: {
    id: string
    display_name: string
    family: string
    context_window: number
    max_output_tokens: number | null
    max_output_tokens_public?: boolean
    max_output_tokens_note?: string
    input_modalities: string[]
    output_modalities: string[]
  }
  access: {
    base_url: string
    groups: Array<{ name: string; multiplier: number }>
  }
  protocols: Array<{
    name: ModelDocProtocolName
    status: ModelDocProtocolStatus
    evidence?: string
    evidence_ids?: string[]
  }>
  protocol_candidates?: Array<{
    name: ModelDocProtocolName
    status: ModelDocProtocolStatus
    evidence?: string
    evidence_ids?: string[]
  }>
  tools?: Array<ModelDocEvidenceCell & { name: string; protocol: ModelDocProtocolName }>
  recommended_protocol: ModelDocProtocolName
  recommended_protocol_reason?: string
  clients: Array<{
    name: string
    version: string
    protocol: ModelDocProtocolName
    status: 'verified'
    recommended: boolean
    evidence?: string
    evidence_ids?: string[]
  }>
  client_coverage: Array<{
    name: string
    protocols: ModelDocProtocolName[]
    status: 'verified' | 'blocked' | 'unsupported'
    evidence?: string
    evidence_ids?: string[]
  }>
  reasoning?: {
    model_levels: string[]
    default_level?: string
    client_levels?: Array<{ client: string; levels: string[] }>
    client_mappings: Array<{ client: string; protocol?: ModelDocProtocolName; from: string; to: string }>
  }
  test_matrix?: Record<string, unknown>
  publication: {
    status: 'publishable' | 'draft'
    publishable: boolean
    missing_evidence: {
      blocked_cells: number
      audit_failures: number
      validation_errors: string[]
      by_matrix: Array<{ matrix: ModelDocMatrixName; count: number }>
    }
  }
  compatibility: ModelDocCompatibility
  verification: {
    official_spec_url: string
    verified_at: string
    limits_source: 'official' | 'live'
    gateway_e2e: boolean
    gateway_e2e_scope?: string
    modalities: Record<'text' | 'image' | 'video', ModelDocVerificationStatus>
  }
}

export const modelDocContracts: readonly ModelDocContract[] = ''' + encoded + '''

export const modelDocContractById: Readonly<Record<string, ModelDocContract>> =
  Object.fromEntries(modelDocContracts.map((contract) => [contract.model.id, contract]))
'''


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("apply", "check"))
    args = parser.parse_args()
    try:
        content = render(load_contracts())
        admin_content = render_admin_payload(load_contracts())
        if args.command == "apply":
            OUTPUT.parent.mkdir(parents=True, exist_ok=True)
            ADMIN_OUTPUT.parent.mkdir(parents=True, exist_ok=True)
            changed: list[str] = []
            if not OUTPUT.exists() or OUTPUT.read_text(encoding="utf-8") != content:
                OUTPUT.write_text(content, encoding="utf-8")
                changed.append(str(OUTPUT.relative_to(ROOT)))
            if not ADMIN_OUTPUT.exists() or ADMIN_OUTPUT.read_text(encoding="utf-8") != admin_content:
                ADMIN_OUTPUT.write_text(admin_content, encoding="utf-8")
                changed.append(str(ADMIN_OUTPUT.relative_to(ROOT)))
            print(json.dumps({"changed": changed}, ensure_ascii=False))
            return 0
        if (not OUTPUT.exists() or OUTPUT.read_text(encoding="utf-8") != content
                or not ADMIN_OUTPUT.exists() or ADMIN_OUTPUT.read_text(encoding="utf-8") != admin_content):
            raise ValueError("generated model documentation catalog is stale")
        print(json.dumps({"valid": True, "contracts": len(load_contracts())}, ensure_ascii=False))
        return 0
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

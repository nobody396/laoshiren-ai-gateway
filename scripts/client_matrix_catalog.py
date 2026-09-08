#!/usr/bin/env python3
"""Generate the frontend client projection from canonical client-matrix v2."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "model-doc-contracts" / "client-matrix.json"
OUTPUT = ROOT / "frontend" / "src" / "generated" / "clientMatrix.ts"
GO_OUTPUT = ROOT / "backend" / "internal" / "service" / "client_setup_contract_generated.go"
CONTRACTS = ROOT / "model-doc-contracts"
CODEX_CLIENT_CATALOG = ROOT / "frontend" / "public" / "auto-config" / "codex-model-catalog.json"


def load_reasoning_profiles() -> list[dict]:
    profiles = []
    for path in sorted(CONTRACTS.glob("*.json")):
        if path.name in {"client-matrix.json", "matrix-schema.json", "import-provenance.json"}:
            continue
        contract = json.loads(path.read_text())
        model_id = str(contract.get("model", {}).get("id", "")).strip()
        reasoning = contract.get("reasoning") or {}
        if not model_id:
            continue
        profiles.append({
            "model_id": model_id,
            "model_levels": reasoning.get("model_levels") or [],
            "client_levels": reasoning.get("client_levels") or [],
            "client_mappings": reasoning.get("client_mappings") or [],
        })
    return profiles


def unique_paths(client: dict, os_names: set[str]) -> list[str]:
    paths: list[str] = []
    for item in client["client_config_os"]["os_support"]:
        if item["os"] not in os_names:
            continue
        for entry in item.get("config_files") or []:
            path = entry.get("path")
            if isinstance(path, str) and path not in paths:
                paths.append(path)
    return paths


def project_client(client: dict) -> dict:
    protocol_rows = client["client_protocol"]["protocols"]
    reasoning = client["client_reasoning"]
    config = client["client_config_os"]
    endpoint = config["endpoint"]
    return {
        "id": client["id"],
        "name": client["name"],
        "version": config["release"]["display"],
        "version_key": config["release"]["version_key"],
        "slug": client["slug"],
        "icon": client["icon"],
        "protocols": [row["protocol"] for row in protocol_rows if row["support"] == "supported"],
        "protocol_details": [
            {
                "protocol": row["protocol"],
                "support": row["support"],
                "evidence_status": (row.get("evidence") or {}).get("status"),
            }
            for row in protocol_rows
        ],
        "reasoning": {
            "mode": reasoning["control_kind"],
            "levels": reasoning["level_control"].get("values") or [],
            "modes": [mode["id"] for mode in reasoning.get("modes") or []],
            "unsupported_strategy": reasoning.get("fallback", {}).get("strategy"),
            "notes": reasoning.get("notes", ""),
        },
        "files": {
            "unix": unique_paths(client, {"macos", "linux"}),
            "windows": unique_paths(client, {"windows"}),
        },
        "one_click_status": client["one_click_status"],
        "verification_os": client.get("verification_os") or [],
        "config_contract": {
            "base_url_rule": endpoint["base_url_rule"],
            "credential": endpoint["credential_location"],
            "model_discovery": config["model_discovery"],
            "model_slots": config.get("model_slots") or [],
            "owned_fields": config.get("owned_fields") or [],
            "merge_strategy": config["mutation"]["merge_strategy"],
            "verification": config["verification_contract"],
            "verification_commands": [
                {
                    "operating_systems": row.get("operating_systems") or [],
                    "commands": row.get("commands") or [],
                    "status": row.get("status", "documented"),
                }
                for row in config.get("verification_commands") or []
            ],
        },
    }


def project_setup_contract(client: dict) -> dict:
    """Project the fail-closed setup fields used by the Go ticket service."""
    one_click_status = client["one_click_status"]
    os_contracts = {}
    for item in client["client_config_os"]["os_support"]:
        evidence_status = (item.get("evidence") or {}).get("status")
        os_contracts[item["os"]] = {
            "support": item["support"],
            "ready": (
                one_click_status == "ready"
                and item["support"] == "verified"
                and evidence_status == "verified"
            ),
        }
    return {
        "client_id": client["id"],
        "version_key": client["client_config_os"]["release"]["version_key"],
        "one_click_status": one_click_status,
        "protocols": sorted(
            row["protocol"]
            for row in client["client_protocol"]["protocols"]
            if row["support"] == "supported"
        ),
        "os": os_contracts,
    }


def load_model_protocols() -> dict[str, list[str]]:
    result: dict[str, list[str]] = {}
    for path in sorted(CONTRACTS.glob("*.json")):
        if path.name in {"client-matrix.json", "matrix-schema.json", "import-provenance.json"}:
            continue
        contract = json.loads(path.read_text())
        model_id = str(contract.get("model", {}).get("id", "")).strip()
        if not model_id:
            continue
        result[model_id] = sorted({
            str(row.get("name", "")).strip()
            for row in contract.get("protocols") or []
            if row.get("status") == "verified" and str(row.get("name", "")).strip()
        })
    # Newly released models are first recorded as release contracts before the
    # long-form model document is generated. Setup must still fail closed, but
    # it should consume their explicit supported protocol rows instead of
    # silently treating a fully released model as unknown.
    for path in sorted((CONTRACTS / "releases").glob("*.release.json")):
        contract = json.loads(path.read_text())
        model_id = str(contract.get("model", {}).get("id", "")).strip()
        if not model_id:
            continue
        protocols = {
            str(row.get("protocol", "")).strip()
            for row in contract.get("protocol_matrix") or []
            if row.get("support") == "supported" and str(row.get("protocol", "")).strip()
        }
        result[model_id] = sorted(set(result.get(model_id, [])) | protocols)
    return result


def go_quote(value: str) -> str:
    return json.dumps(value, ensure_ascii=False)


def load_codex_setup_models() -> list[str]:
    catalog = json.loads(CODEX_CLIENT_CATALOG.read_text())
    rows = catalog.get("models") if isinstance(catalog, dict) else None
    if not isinstance(rows, list):
        raise ValueError("Codex client catalog must contain a models array")
    result = {
        str(row.get("slug", "")).strip()
        for row in rows
        if isinstance(row, dict) and str(row.get("slug", "")).strip()
    }
    return sorted(result)


def render_go(data: dict) -> str:
    contracts = sorted(
        (project_setup_contract(client) for client in data["clients"]),
        key=lambda item: item["client_id"],
    )
    lines = [
        "// Code generated by scripts/client_matrix_catalog.py; DO NOT EDIT.",
        "",
        "package service",
        "",
        "type generatedClientSetupContract struct {",
        "\tVersionKey     string",
        "\tOneClickStatus string",
        "\tProtocols      map[string]bool",
        "\tOSReady        map[string]bool",
        "}",
        "",
        "var generatedClientSetupContracts = map[string]generatedClientSetupContract{",
    ]
    for contract in contracts:
        protocols = ", ".join(f"{go_quote(protocol)}: true" for protocol in contract["protocols"])
        os_ready = ", ".join(
            f"{go_quote(os_name)}: {str(bool(cell['ready'])).lower()}"
            for os_name, cell in sorted(contract["os"].items())
        )
        lines.extend([
            f"\t{go_quote(contract['client_id'])}: {{",
            f"\t\tVersionKey: {go_quote(contract['version_key'])},",
            f"\t\tOneClickStatus: {go_quote(contract['one_click_status'])},",
            f"\t\tProtocols: map[string]bool{{{protocols}}},",
            f"\t\tOSReady: map[string]bool{{{os_ready}}},",
            "\t},",
        ])
    lines.extend(["}", "", "var generatedClientSetupModelProtocols = map[string]map[string]bool{"])
    for model_id, protocols in sorted(load_model_protocols().items()):
        values = ", ".join(f"{go_quote(protocol)}: true" for protocol in protocols)
        lines.append(f"\t{go_quote(model_id)}: {{{values}}},")
    lines.extend(["}", "", "var generatedCodexSetupModels = map[string]bool{"])
    for model_id in load_codex_setup_models():
        lines.append(f"\t{go_quote(model_id)}: true,")
    lines.extend(["}", ""])
    raw = "\n".join(lines)
    return subprocess.run(
        ["gofmt"], input=raw, text=True, check=True, capture_output=True,
    ).stdout


def render(data: dict) -> str:
    if data.get("schema_version") != 2:
        raise ValueError("client matrix schema_version must be 2")
    projection = [project_client(client) for client in data["clients"]]
    encoded = json.dumps(projection, ensure_ascii=False, indent=2)
    reasoning = json.dumps(load_reasoning_profiles(), ensure_ascii=False, indent=2)
    return f'''// Code generated by scripts/client_matrix_catalog.py; DO NOT EDIT.

export type ClientMatrixProtocol = 'responses' | 'chat_completions' | 'messages' | 'generate_content'
export interface ClientMatrixEntry {{
  id: string
  name: string
  version: string
  version_key: string
  slug: string
  icon: string
  protocols: ClientMatrixProtocol[]
  protocol_details: Array<{{ protocol: ClientMatrixProtocol; support: 'supported' | 'unsupported' | 'unverified'; evidence_status?: string }}>
  reasoning: {{ mode: 'fixed' | 'model_defined' | 'provider_native' | 'none'; levels: string[]; modes?: string[]; unsupported_strategy?: 'floor' | 'reject' | 'explicit_mapping' | 'unknown' | 'not_applicable'; notes?: string }}
  files: {{ unix: string[]; windows: string[] }}
  one_click_status: 'prototype' | 'ready' | 'disabled'
  verification_os: string[]
  config_contract: {{ base_url_rule: string; credential: string; model_discovery: string; model_slots: string[]; owned_fields: string[]; merge_strategy: string; verification: string; verification_commands: Array<{{ operating_systems: string[]; commands: string[]; status: string }}> }}
}}
export const clientMatrix: readonly ClientMatrixEntry[] = {encoded}
export const clientMatrixBySlug: Readonly<Record<string, ClientMatrixEntry>> = Object.fromEntries(clientMatrix.map(client => [client.slug, client]))
export interface ModelReasoningProfile {{
  model_id: string
  model_levels: string[]
  client_levels: Array<{{ client: string; levels: string[] }}>
  client_mappings: Array<{{ client: string; protocol?: string; from: string; to: string }}>
}}
export const modelReasoningProfiles: readonly ModelReasoningProfile[] = {reasoning}
export const modelReasoningProfileById: Readonly<Record<string, ModelReasoningProfile>> = Object.fromEntries(modelReasoningProfiles.map(profile => [profile.model_id, profile]))
'''


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=("apply", "check"))
    args = parser.parse_args()
    data = json.loads(SOURCE.read_text())
    content = render(data)
    go_content = render_go(data)
    if args.command == "apply":
        OUTPUT.parent.mkdir(parents=True, exist_ok=True)
        changed = not OUTPUT.exists() or OUTPUT.read_text() != content
        go_changed = not GO_OUTPUT.exists() or GO_OUTPUT.read_text() != go_content
        if changed:
            OUTPUT.write_text(content)
        if go_changed:
            GO_OUTPUT.parent.mkdir(parents=True, exist_ok=True)
            GO_OUTPUT.write_text(go_content)
        print(json.dumps({"changed": [
            str(OUTPUT.relative_to(ROOT)) if changed else None,
            str(GO_OUTPUT.relative_to(ROOT)) if go_changed else None,
        ]}, ensure_ascii=False))
        return
    if (not OUTPUT.exists() or OUTPUT.read_text() != content
            or not GO_OUTPUT.exists() or GO_OUTPUT.read_text() != go_content):
        raise SystemExit("generated client matrix is stale")
    print(json.dumps({"valid": True, "clients": len(data["clients"]), "schema_version": 2}, ensure_ascii=False))


if __name__ == "__main__":
    main()

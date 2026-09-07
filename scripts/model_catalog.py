#!/usr/bin/env python3
"""Generate and verify model metadata shared by backend and frontend."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
from pathlib import Path
import re
import sys
from typing import Any
from urllib.parse import urlparse


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CATALOG = ROOT / "model-catalog" / "catalog.json"
MODEL_CONTRACT_DIR = ROOT / "model-doc-contracts"
GO_OUTPUT = ROOT / "backend" / "internal" / "service" / "model_catalog_generated.go"
TS_OUTPUT = ROOT / "frontend" / "src" / "generated" / "modelCatalog.ts"
CODEX_TS_OUTPUT = ROOT / "frontend" / "src" / "generated" / "codexClientCatalog.ts"
INSTALLER_INTEGRITY_OUTPUT = ROOT / "frontend" / "src" / "generated" / "installerIntegrity.ts"
CODEX_CLIENT_BASE = ROOT / "model-catalog" / "codex-client-base.json"
CODEX_CLIENT_OUTPUT = ROOT / "frontend" / "public" / "auto-config" / "codex-model-catalog.json"
POWERSHELL_INSTALLER = ROOT / "frontend" / "public" / "auto-config" / "install.ps1"
SHELL_INSTALLER = ROOT / "frontend" / "public" / "auto-config" / "install.sh"
POWERSHELL_BLOCK_BEGIN = "# BEGIN GENERATED MODEL CATALOG"
POWERSHELL_BLOCK_END = "# END GENERATED MODEL CATALOG"
SHELL_BLOCK_BEGIN = "# BEGIN GENERATED MODEL CATALOG"
SHELL_BLOCK_END = "# END GENERATED MODEL CATALOG"
VERSION_REFERENCE_PATHS = (
    ROOT / "frontend" / "public" / "auto-config" / "install.ps1",
    ROOT / "frontend" / "src" / "docs" / "content" / "auto-config-tool.md",
    ROOT / "frontend" / "src" / "docs" / "content" / "codex-quickstart.md",
    ROOT / "frontend" / "src" / "views" / "user" / "ResourcesView.vue",
)
PLATFORMS = {"openai", "anthropic", "grok", "gemini"}
PRICE_COMPONENT_STATUSES = {
    "verified", "unknown", "blocked", "not_applicable", "not_published", "not_exposed",
}
PRESET_COLORS = {
    "openai": "bg-amber-100 text-amber-700 hover:bg-amber-200 dark:bg-amber-900/30 dark:text-amber-400",
    "anthropic": "bg-indigo-100 text-indigo-700 hover:bg-indigo-200 dark:bg-indigo-900/30 dark:text-indigo-400",
    "grok": "bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800/50 dark:text-slate-300",
    "gemini": "bg-blue-100 text-blue-700 hover:bg-blue-200 dark:bg-blue-900/30 dark:text-blue-400",
}
SECRET_KEY = re.compile(r"(api[_-]?key|access[_-]?token|refresh[_-]?token|password|credential|secret)", re.I)
SECRET_VALUE = re.compile(r"(?:sk-|Bearer\s+)[A-Za-z0-9_-]{8,}", re.I)
MODEL_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]*$")
SEMVER = re.compile(r"^(\d+)\.(\d+)\.(\d+)$")
INSTALLER_VERSION_REFERENCE = re.compile(r"(auto-config/install\.(?:ps1|sh)\?v=)(\d+\.\d+\.\d+)")
CODEX_REQUIRED_FIELDS = {
    "slug",
    "display_name",
    "base_instructions",
    "supports_reasoning_summaries",
    "visibility",
    "context_window",
    "max_context_window",
    "auto_compact_token_limit",
}


def fail(message: str) -> None:
    raise ValueError(message)


def walk_secret_free(value: Any, path: str = "$") -> None:
    if isinstance(value, dict):
        for key, item in value.items():
            if SECRET_KEY.search(str(key)):
                fail(f"credential-shaped catalog field: {path}.{key}")
            walk_secret_free(item, f"{path}.{key}")
    elif isinstance(value, list):
        for index, item in enumerate(value):
            walk_secret_free(item, f"{path}[{index}]")
    elif isinstance(value, str) and SECRET_VALUE.search(value):
        fail(f"secret-shaped catalog value: {path}")


def number(value: Any, path: str, *, allow_zero: bool = False) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        fail(f"{path} must be numeric")
    out = float(value)
    if not math.isfinite(out):
        fail(f"{path} must be finite")
    if out < 0 or (out == 0 and not allow_zero):
        fail(f"{path} must be positive")
    return out


def load_catalog(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        fail("catalog root must be an object")
    validate_catalog(value)
    return value


def catalog_row_from_manifest(manifest: dict[str, Any], previous: dict[str, Any] | None = None) -> dict[str, Any]:
    walk_secret_free(manifest)
    model = manifest.get("model")
    pricing = manifest.get("pricing")
    production = manifest.get("production")
    if not all(isinstance(section, dict) for section in (model, pricing, production)):
        fail("manifest requires model, pricing, and production objects")
    platform = model.get("platform")
    if platform not in PLATFORMS:
        fail("manifest model.platform is invalid")
    long_context = pricing.get("long_context")
    if long_context is not None and not isinstance(long_context, dict):
        fail("manifest pricing.long_context must be an object or null")
    preferred_group = str(production.get("group_name") or "").strip()
    if not preferred_group:
        fail("manifest production.group_name is required")
    previous = previous or {}
    legacy_names = production.get("legacy_group_names", previous.get("public_group", {}).get("legacy_names", []))
    if not isinstance(legacy_names, list):
        fail("manifest production.legacy_group_names must be a string array")
    previous_client_config = previous.get("client_config", {})
    managed_predecessors = production.get(
        "managed_predecessor_ids",
        previous_client_config.get("managed_predecessor_ids", []),
    )
    if not isinstance(managed_predecessors, list):
        fail("manifest production.managed_predecessor_ids must be a string array")
    managed_predecessor_models = production.get(
        "managed_predecessor_models",
        previous_client_config.get("managed_predecessor_models", []),
    )
    if not isinstance(managed_predecessor_models, list):
        fail("manifest production.managed_predecessor_models must be an array")
    codex_catalog_entry = model.get(
        "codex_catalog_entry",
        previous_client_config.get("codex_catalog_entry"),
    )
    return {
        "id": model.get("id"),
        "upstream_id": model.get("upstream_id"),
        "display_name": model.get("display_name"),
        "platform": platform,
        "context_window": model.get("context_window"),
        "max_output_tokens": model.get("max_output_tokens"),
        "client_default": bool(production.get("client_default")),
        "public_price_visibility": production.get(
            "public_price_visibility", previous.get("public_price_visibility", "public")
        ),
        "public_group": {
            "preferred_name": preferred_group.removesuffix(" 分组"),
            "legacy_names": legacy_names,
        },
        "client_config": {
            "managed_predecessor_ids": managed_predecessors,
            "managed_predecessor_models": managed_predecessor_models,
            "codex_catalog_entry": codex_catalog_entry,
        },
        "pricing": {
            "input_per_mtok_usd": pricing.get("input_per_mtok_usd"),
            "cached_input_per_mtok_usd": pricing.get("cached_input_per_mtok_usd", 0),
            "cache_write_5m_per_mtok_usd": pricing.get(
                "cache_write_5m_per_mtok_usd",
                previous.get("pricing", {}).get("cache_write_5m_per_mtok_usd"),
            ),
            "cache_write_1h_per_mtok_usd": pricing.get(
                "cache_write_1h_per_mtok_usd",
                previous.get("pricing", {}).get("cache_write_1h_per_mtok_usd"),
            ),
            "output_per_mtok_usd": pricing.get("output_per_mtok_usd"),
            "component_status": {
                "input": "verified",
                "output": "verified",
                "cached_input": (
                    "verified" if pricing.get("cached_input_per_mtok_usd", 0) is not None else "unknown"
                ),
                "cache_write": (
                    "verified"
                    if pricing.get("cache_write_5m_per_mtok_usd") is not None
                    else "not_applicable"
                ),
                "long_context": "verified" if long_context else "not_applicable",
            },
            "long_context_input_threshold": long_context.get("input_threshold") if long_context else None,
            "long_context_input_multiplier": long_context.get("input_multiplier") if long_context else None,
            "long_context_output_multiplier": long_context.get("output_multiplier") if long_context else None,
            "evidence_url": pricing.get("evidence_url"),
        },
        "provider_pricing": pricing.get(
            "provider_pricing", previous.get("provider_pricing")
        ),
        "preset": {
            "from": model.get("id"),
            "to": model.get("id"),
            "color": PRESET_COLORS[platform],
        },
    }


def merge_manifest(catalog: dict[str, Any], manifest_path: Path) -> dict[str, Any]:
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if not isinstance(manifest, dict):
        fail("manifest root must be an object")
    model_section = manifest.get("model")
    model_id = model_section.get("id") if isinstance(model_section, dict) else None
    previous = next((row for row in catalog["models"] if row.get("id") == model_id), None)
    row = catalog_row_from_manifest(manifest, previous)
    merged = json.loads(json.dumps(catalog))
    merged["models"] = [candidate for candidate in merged["models"] if candidate.get("id") != row["id"]]
    if row["client_default"]:
        for candidate in merged["models"]:
            if candidate.get("platform") == row["platform"]:
                candidate["client_default"] = False
    merged["models"].append(row)
    merged["models"].sort(key=lambda candidate: (candidate["platform"], candidate["id"]))
    previous_codex_entry = (
        previous.get("client_config", {}).get("codex_catalog_entry")
        if isinstance(previous, dict)
        else None
    )
    next_codex_entry = row.get("client_config", {}).get("codex_catalog_entry")
    if (
        installer_model_values(catalog) != installer_model_values(merged)
        or previous_codex_entry != next_codex_entry
    ):
        merged["client_auto_config_version"] = bump_patch(catalog["client_auto_config_version"])
    validate_catalog(merged)
    return merged


def validate_catalog(catalog: dict[str, Any]) -> None:
    walk_secret_free(catalog)
    if catalog.get("schema_version") != 1:
        fail("schema_version must be 1")
    version = catalog.get("client_auto_config_version")
    if not isinstance(version, str) or not SEMVER.fullmatch(version):
        fail("client_auto_config_version must be a semantic version")
    pricing_contract = catalog.get("pricing_contract")
    if pricing_contract is not None:
        if not isinstance(pricing_contract, dict):
            fail("pricing_contract must be an object")
        if pricing_contract.get("default_scope") != "provider_public":
            fail("pricing_contract.default_scope must be provider_public")
        if pricing_contract.get("null_semantics") != "unknown_unless_component_status_is_not_applicable":
            fail("pricing_contract.null_semantics is invalid")
        if set(pricing_contract.get("component_statuses", [])) != PRICE_COMPONENT_STATUSES:
            fail("pricing_contract.component_statuses drifted")
    price_gaps = catalog.get("price_gaps", [])
    if not isinstance(price_gaps, list):
        fail("price_gaps must be an array")
    for index, gap in enumerate(price_gaps):
        path = f"price_gaps[{index}]"
        if not isinstance(gap, dict):
            fail(f"{path} must be an object")
        if not isinstance(gap.get("model_id"), str) or not MODEL_ID.fullmatch(gap["model_id"]):
            fail(f"{path}.model_id must be a model identifier")
        if gap.get("price_scope") != "provider_public":
            fail(f"{path}.price_scope must be provider_public")
        if gap.get("status") not in {"unknown", "blocked", "not_published"}:
            fail(f"{path}.status must be unknown, blocked, or not_published")
        if not isinstance(gap.get("reason"), str) or not gap["reason"].strip():
            fail(f"{path}.reason is required")
        evidence_url = gap.get("evidence_url")
        if evidence_url is not None:
            parsed = urlparse(evidence_url if isinstance(evidence_url, str) else "")
            if parsed.scheme != "https" or not parsed.netloc:
                fail(f"{path}.evidence_url must be an https URL")
    models = catalog.get("models")
    if not isinstance(models, list) or not models:
        fail("models must be a non-empty array")
    ids: set[str] = set()
    default_platforms: set[str] = set()
    for index, model in enumerate(models):
        path = f"models[{index}]"
        if not isinstance(model, dict):
            fail(f"{path} must be an object")
        for field in ("id", "upstream_id", "display_name"):
            if not isinstance(model.get(field), str) or not model[field].strip():
                fail(f"{path}.{field} is required")
        if not MODEL_ID.fullmatch(model["id"]) or not MODEL_ID.fullmatch(model["upstream_id"]):
            fail(f"{path}.id and upstream_id must be installer-safe model identifiers")
        model_id = model["id"].strip().lower()
        if model_id in ids:
            fail(f"duplicate model id: {model_id}")
        ids.add(model_id)
        platform = model.get("platform")
        if platform not in PLATFORMS:
            fail(f"{path}.platform is invalid")
        number(model.get("context_window"), f"{path}.context_window")
        number(model.get("max_output_tokens"), f"{path}.max_output_tokens")
        if model.get("client_default"):
            if platform in default_platforms:
                fail(f"multiple client defaults for platform {platform}")
            default_platforms.add(platform)
        if model.get("public_price_visibility", "public") not in {"public", "hidden"}:
            fail(f"{path}.public_price_visibility must be public or hidden")
        group = model.get("public_group")
        if not isinstance(group, dict) or not isinstance(group.get("preferred_name"), str) or not group["preferred_name"].strip():
            fail(f"{path}.public_group.preferred_name is required")
        legacy = group.get("legacy_names", [])
        if not isinstance(legacy, list) or not all(isinstance(item, str) and item.strip() for item in legacy):
            fail(f"{path}.public_group.legacy_names must be a string array")
        client_config = model.get("client_config", {})
        if not isinstance(client_config, dict):
            fail(f"{path}.client_config must be an object")
        predecessors = client_config.get("managed_predecessor_ids", [])
        if not isinstance(predecessors, list) or not all(
            isinstance(item, str) and MODEL_ID.fullmatch(item) for item in predecessors
        ):
            fail(f"{path}.client_config.managed_predecessor_ids must be model identifiers")
        predecessor_models = client_config.get("managed_predecessor_models", [])
        if not isinstance(predecessor_models, list):
            fail(f"{path}.client_config.managed_predecessor_models must be an array")
        predecessor_model_ids: set[str] = set()
        for predecessor_index, predecessor in enumerate(predecessor_models):
            predecessor_path = f"{path}.client_config.managed_predecessor_models[{predecessor_index}]"
            if not isinstance(predecessor, dict):
                fail(f"{predecessor_path} must be an object")
            if not isinstance(predecessor.get("id"), str) or not MODEL_ID.fullmatch(predecessor["id"]):
                fail(f"{predecessor_path}.id must be a model identifier")
            if not isinstance(predecessor.get("display_name"), str) or not predecessor["display_name"].strip():
                fail(f"{predecessor_path}.display_name is required")
            number(predecessor.get("context_window"), f"{predecessor_path}.context_window")
            predecessor_model_ids.add(predecessor["id"])
        if platform == "grok" and predecessor_model_ids != set(predecessors):
            fail(f"{path}.client_config.managed_predecessor_models must describe every Grok predecessor")
        codex_entry = client_config.get("codex_catalog_entry")
        if codex_entry is not None:
            if platform != "openai" or not isinstance(codex_entry, dict):
                fail(f"{path}.client_config.codex_catalog_entry is only valid for OpenAI models")
            if codex_entry.get("slug") != model["id"]:
                fail(f"{path}.client_config.codex_catalog_entry.slug must match the model id")
            missing_codex_fields = sorted(CODEX_REQUIRED_FIELDS - codex_entry.keys())
            if missing_codex_fields:
                fail(f"{path}.client_config.codex_catalog_entry is missing: {', '.join(missing_codex_fields)}")
            if codex_entry.get("context_window") != model["context_window"]:
                fail(f"{path}.client_config.codex_catalog_entry.context_window must match the model")
        if platform == "openai" and model.get("client_default") and codex_entry is None:
            fail(f"{path}.client_config.codex_catalog_entry is required for an OpenAI client default")
        pricing = model.get("pricing")
        if not isinstance(pricing, dict):
            fail(f"{path}.pricing is required")
        for field in ("input_per_mtok_usd", "output_per_mtok_usd"):
            number(pricing.get(field), f"{path}.pricing.{field}")
        cached_input = pricing.get("cached_input_per_mtok_usd")
        if cached_input is not None:
            number(cached_input, f"{path}.pricing.cached_input_per_mtok_usd", allow_zero=True)
        component_status = pricing.get("component_status", {})
        if not isinstance(component_status, dict):
            fail(f"{path}.pricing.component_status must be an object")
        if set(component_status.values()) - PRICE_COMPONENT_STATUSES:
            fail(f"{path}.pricing.component_status contains invalid states")
        if cached_input is None and component_status.get("cached_input", "unknown") not in {"unknown", "not_applicable"}:
            fail(f"{path}.pricing.component_status.cached_input cannot be verified when price is null")
        cache_write_5m = pricing.get("cache_write_5m_per_mtok_usd")
        cache_write_1h = pricing.get("cache_write_1h_per_mtok_usd")
        if cache_write_5m is not None:
            number(cache_write_5m, f"{path}.pricing.cache_write_5m_per_mtok_usd")
        if cache_write_1h is not None:
            number(cache_write_1h, f"{path}.pricing.cache_write_1h_per_mtok_usd")
            if cache_write_5m is None:
                fail(f"{path}.pricing.cache_write_1h_per_mtok_usd requires the 5m price")
            if float(cache_write_1h) <= float(cache_write_5m):
                fail(f"{path}.pricing.cache_write_1h_per_mtok_usd must exceed the 5m price")
        threshold = pricing.get("long_context_input_threshold")
        if threshold is not None:
            number(threshold, f"{path}.pricing.long_context_input_threshold")
            number(pricing.get("long_context_input_multiplier"), f"{path}.pricing.long_context_input_multiplier")
            number(pricing.get("long_context_output_multiplier"), f"{path}.pricing.long_context_output_multiplier")
        evidence = pricing.get("evidence_url")
        parsed = urlparse(evidence if isinstance(evidence, str) else "")
        if parsed.scheme != "https" or not parsed.netloc:
            fail(f"{path}.pricing.evidence_url must be an https URL")
        provider_pricing = model.get("provider_pricing")
        if provider_pricing is not None:
            if not isinstance(provider_pricing, dict):
                fail(f"{path}.provider_pricing must be an object")
            if provider_pricing.get("price_scope") != "provider_public":
                fail(f"{path}.provider_pricing.price_scope must be provider_public")
            for field in ("input_per_mtok_usd", "output_per_mtok_usd"):
                number(provider_pricing.get(field), f"{path}.provider_pricing.{field}")
            provider_cache = provider_pricing.get("cached_input_per_mtok_usd")
            if provider_cache is not None:
                number(provider_cache, f"{path}.provider_pricing.cached_input_per_mtok_usd", allow_zero=True)
            provider_states = provider_pricing.get("component_status", {})
            if not isinstance(provider_states, dict) or set(provider_states.values()) - PRICE_COMPONENT_STATUSES:
                fail(f"{path}.provider_pricing.component_status is invalid")
            if provider_cache is None and provider_states.get("cached_input", "unknown") not in {"unknown", "not_applicable", "not_published"}:
                fail(f"{path}.provider_pricing.component_status.cached_input cannot be verified when price is null")
            provider_threshold = provider_pricing.get("long_context_input_threshold")
            if provider_threshold is not None:
                number(provider_threshold, f"{path}.provider_pricing.long_context_input_threshold")
                number(provider_pricing.get("long_context_input_multiplier"), f"{path}.provider_pricing.long_context_input_multiplier")
                number(provider_pricing.get("long_context_output_multiplier"), f"{path}.provider_pricing.long_context_output_multiplier")
                cached_multiplier = provider_pricing.get("long_context_cached_input_multiplier")
                if cached_multiplier is not None:
                    number(cached_multiplier, f"{path}.provider_pricing.long_context_cached_input_multiplier")
            provider_evidence = provider_pricing.get("evidence_url")
            parsed = urlparse(provider_evidence if isinstance(provider_evidence, str) else "")
            if parsed.scheme != "https" or not parsed.netloc:
                fail(f"{path}.provider_pricing.evidence_url must be an https URL")
        preset = model.get("preset")
        if not isinstance(preset, dict) or not all(isinstance(preset.get(field), str) and preset[field].strip() for field in ("from", "to", "color")):
            fail(f"{path}.preset is incomplete")


def bump_patch(version: str) -> str:
    match = SEMVER.fullmatch(version)
    if not match:
        fail("client_auto_config_version must be a semantic version")
    major, minor, patch = (int(part) for part in match.groups())
    return f"{major}.{minor}.{patch + 1}"


def installer_contract(model: dict[str, Any] | None) -> dict[str, Any] | None:
    if not model:
        return None
    return {
        "id": model.get("id"),
        "display_name": model.get("display_name"),
        "context_window": model.get("context_window"),
        "managed_predecessor_ids": model.get("client_config", {}).get("managed_predecessor_ids", []),
        "managed_predecessor_models": model.get("client_config", {}).get("managed_predecessor_models", []),
    }


def default_model(catalog: dict[str, Any], platform: str) -> dict[str, Any] | None:
    return next(
        (model for model in catalog["models"] if model["platform"] == platform and model.get("client_default")),
        None,
    )


def installer_model_values(catalog: dict[str, Any]) -> dict[str, dict[str, Any]]:
    fallbacks = {
        "openai": {
            "id": "gpt-5.6-sol",
            "display_name": "GPT-5.6 Sol",
            "context_window": 272000,
            "client_config": {"codex_catalog_entry": {"auto_compact_token_limit": 258000}},
        },
        "anthropic": {"id": "claude-opus-5", "display_name": "Claude Opus 5", "context_window": 200000},
        "grok": {"id": "grok-4.6", "display_name": "Grok 4.6", "context_window": 500000},
        "gemini": {"id": "gemini-3.1-pro", "display_name": "Gemini 3.1 Pro", "context_window": 256000},
    }
    values: dict[str, dict[str, Any]] = {}
    for platform in sorted(PLATFORMS):
        chosen = default_model(catalog, platform) or fallbacks[platform]
        managed_ids = {
            model["id"]
            for model in catalog["models"]
            if model["platform"] == platform
        }
        for model in catalog["models"]:
            if model["platform"] == platform:
                managed_ids.update(model.get("client_config", {}).get("managed_predecessor_ids", []))
        values[platform] = {
            "id": chosen["id"],
            "display_name": chosen["display_name"],
            "context_window": int(chosen["context_window"]),
            "managed_ids": sorted(managed_ids),
        }
        if platform == "grok":
            managed_models = {
                chosen["id"]: {
                    "id": chosen["id"],
                    "display_name": chosen["display_name"],
                    "context_window": int(chosen["context_window"]),
                }
            }
            for model in catalog["models"]:
                if model["platform"] != platform:
                    continue
                managed_models[model["id"]] = {
                    "id": model["id"],
                    "display_name": model["display_name"],
                    "context_window": int(model["context_window"]),
                }
                for predecessor in model.get("client_config", {}).get("managed_predecessor_models", []):
                    managed_models[predecessor["id"]] = {
                        "id": predecessor["id"],
                        "display_name": predecessor["display_name"],
                        "context_window": int(predecessor["context_window"]),
                    }
            values[platform]["managed_models"] = [managed_models[model_id] for model_id in sorted(managed_ids)]
        if platform == "openai":
            values[platform]["auto_compact_token_limit"] = int(
                chosen.get("client_config", {})
                .get("codex_catalog_entry", {})
                .get("auto_compact_token_limit", 258000)
            )
    return values


def powershell_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def shell_quote(value: str) -> str:
    return "'" + value.replace("'", "'\"'\"'") + "'"


def render_powershell_block(catalog: dict[str, Any]) -> str:
    values = installer_model_values(catalog)
    reasoning_json = json.dumps(model_reasoning_levels(), ensure_ascii=False, separators=(",", ":"))
    grok_sections = []
    for model_id in values["grok"]["managed_ids"]:
        grok_sections.extend((f"model.{model_id}", f'model."{model_id}"'))
    section_literal = ", ".join(powershell_quote(section) for section in grok_sections)
    model_profiles = ", ".join(
        "@{ Id = %s; DisplayName = %s; ContextWindow = %d }" % (
            powershell_quote(model["id"]),
            powershell_quote(model["display_name"]),
            model["context_window"],
        )
        for model in values["grok"]["managed_models"]
    )
    return "\n".join((
        POWERSHELL_BLOCK_BEGIN,
        f"$ScriptVersion = {powershell_quote(catalog['client_auto_config_version'])}",
        f"$CatalogOpenAIDefaultModel = {powershell_quote(values['openai']['id'])}",
        f"$CatalogOpenAIContextWindow = {values['openai']['context_window']}",
        f"$CatalogOpenAIAutoCompactTokenLimit = {values['openai']['auto_compact_token_limit']}",
        f"$CatalogAnthropicDefaultModel = {powershell_quote(values['anthropic']['id'])}",
        f"$CatalogGrokDefaultModel = {powershell_quote(values['grok']['id'])}",
        f"$CatalogGrokDefaultDisplayName = {powershell_quote(values['grok']['display_name'])}",
        f"$CatalogGrokDefaultContextWindow = {values['grok']['context_window']}",
        f"$CatalogGrokManagedModels = @({model_profiles})",
        f"$CatalogGrokManagedModelSections = @({section_literal})",
        f"$CatalogGeminiDefaultModel = {powershell_quote(values['gemini']['id'])}",
        "$CatalogGeminiManagedModels = @(%s)" % ", ".join(
            powershell_quote(model_id) for model_id in values["gemini"]["managed_ids"]
        ),
        f"$CatalogModelReasoningJson = {powershell_quote(reasoning_json)}",
        POWERSHELL_BLOCK_END,
    ))


def render_shell_block(catalog: dict[str, Any]) -> str:
    values = installer_model_values(catalog)
    managed_json = json.dumps(values["grok"]["managed_models"], ensure_ascii=False, separators=(",", ":"))
    reasoning_json = json.dumps(model_reasoning_levels(), ensure_ascii=False, separators=(",", ":"))
    return "\n".join((
        SHELL_BLOCK_BEGIN,
        f"SCRIPT_VERSION={shell_quote(catalog['client_auto_config_version'])}",
        f"CATALOG_OPENAI_DEFAULT_MODEL={shell_quote(values['openai']['id'])}",
        f"CATALOG_OPENAI_CONTEXT_WINDOW={values['openai']['context_window']}",
        f"CATALOG_OPENAI_AUTO_COMPACT_TOKEN_LIMIT={values['openai']['auto_compact_token_limit']}",
        f"CATALOG_ANTHROPIC_DEFAULT_MODEL={shell_quote(values['anthropic']['id'])}",
        f"CATALOG_GROK_DEFAULT_MODEL={shell_quote(values['grok']['id'])}",
        f"CATALOG_GROK_DEFAULT_DISPLAY_NAME={shell_quote(values['grok']['display_name'])}",
        f"CATALOG_GROK_DEFAULT_CONTEXT_WINDOW={values['grok']['context_window']}",
        f"CATALOG_GROK_MANAGED_MODELS_JSON={shell_quote(managed_json)}",
        f"CATALOG_GEMINI_DEFAULT_MODEL={shell_quote(values['gemini']['id'])}",
        "CATALOG_GEMINI_MANAGED_MODELS=%s" % shell_quote(" ".join(values["gemini"]["managed_ids"])),
        f"CATALOG_MODEL_REASONING_JSON={shell_quote(reasoning_json)}",
        SHELL_BLOCK_END,
    ))


def model_reasoning_levels() -> dict[str, list[str]]:
    result: dict[str, list[str]] = {}
    for path in sorted(MODEL_CONTRACT_DIR.glob("*.json")):
        if path.name in {"client-matrix.json", "matrix-schema.json", "import-provenance.json"}:
            continue
        contract = json.loads(path.read_text(encoding="utf-8"))
        model_id = str(contract.get("model", {}).get("id", "")).strip()
        if model_id:
            result[model_id] = [
                str(level) for level in contract.get("reasoning", {}).get("model_levels", [])
                if isinstance(level, str)
            ]
    return result


def go_float(value: Any) -> str:
    return format(float(value), ".15g")


def gofmt_align_map_rows(rows: list[str]) -> list[str]:
    """Pad map keys so the generated Go is gofmt-clean (tabular alignment)."""
    if not rows:
        return rows
    prefixes: list[str] = []
    rests: list[str] = []
    for row in rows:
        sep = row.index(": ")
        prefixes.append(row[:sep + 1])
        rests.append(row[sep + 2:])
    width = max(len(prefix) for prefix in prefixes)
    return [prefix.ljust(width) + " " + rest for prefix, rest in zip(prefixes, rests)]


def render_go(catalog: dict[str, Any]) -> str:
    models = sorted(catalog["models"], key=lambda row: row["id"])
    price_rows: list[str] = []
    display_rows: list[str] = []
    default_rows: list[str] = []
    group_rows: list[str] = []
    for model in models:
        pricing = model["pricing"]
        threshold = pricing.get("long_context_input_threshold") or 0
        input_multiplier = pricing.get("long_context_input_multiplier") or 0
        output_multiplier = pricing.get("long_context_output_multiplier") or 0
        cache_write_5m = pricing.get("cache_write_5m_per_mtok_usd") or 0
        cache_write_1h = pricing.get("cache_write_1h_per_mtok_usd") or 0
        cache_write_fields = ""
        if cache_write_5m:
            cache_write_fields += (
                f"CacheCreationPricePerToken: {go_float(cache_write_5m)}e-6, "
                f"CacheCreation5mPrice: {go_float(cache_write_5m)}e-6, "
            )
        if cache_write_1h:
            cache_write_fields += (
                f"CacheCreation1hPrice: {go_float(cache_write_1h)}e-6, "
                "SupportsCacheBreakdown: true, "
            )
        display_cache_write = (
            f", cacheWrite: {go_float(cache_write_5m)}" if cache_write_5m else ""
        )
        price_rows.append(
            f'\t"{model["id"]}": {{InputPricePerToken: {go_float(pricing["input_per_mtok_usd"])}e-6, '
            f'OutputPricePerToken: {go_float(pricing["output_per_mtok_usd"])}e-6, '
            f'{cache_write_fields}'
            f'CacheReadPricePerToken: {go_float(pricing.get("cached_input_per_mtok_usd") or 0)}e-6, '
            f'LongContextInputThreshold: {int(threshold)}, LongContextInputMultiplier: {go_float(input_multiplier)}, '
            f'LongContextOutputMultiplier: {go_float(output_multiplier)}}},'
        )
        display_rows.append(
            f'\t"{model["id"]}": {{input: {go_float(pricing["input_per_mtok_usd"])}, '
            f'output: {go_float(pricing["output_per_mtok_usd"])}, '
            f'{display_cache_write.lstrip(", ") + ", " if display_cache_write else ""}'
            f'cacheRead: {go_float(pricing.get("cached_input_per_mtok_usd") or 0)}}},'
        )
        if model.get("client_default"):
            default_rows.append(f'\t"{model["platform"]}": "{model["id"]}",')
            legacy = ", ".join(json.dumps(item, ensure_ascii=False) for item in model["public_group"].get("legacy_names", []))
            group_rows.append(
                f'\t"{model["platform"]}": {{Preferred: {json.dumps(model["public_group"]["preferred_name"], ensure_ascii=False)}, Legacy: []string{{{legacy}}}}},'
            )
    return """// Code generated by scripts/model_catalog.py; DO NOT EDIT.\n\npackage service\n\nimport \"strings\"\n\ntype generatedCatalogGroupPolicy struct {\n\tPreferred string\n\tLegacy    []string\n}\n\nvar generatedCatalogBillingPrices = map[string]*ModelPricing{\n%s\n}\n\nvar generatedCatalogDisplayPrices = map[string]manualOfficialPrice{\n%s\n}\n\nvar generatedCatalogClientDefaults = map[string]string{\n%s\n}\n\nvar generatedCatalogGroupPolicies = map[string]generatedCatalogGroupPolicy{\n%s\n}\n\nfunc generatedCatalogBillingPrice(model string) *ModelPricing {\n\treturn generatedCatalogBillingPrices[strings.ToLower(strings.TrimSpace(model))]\n}\n\nfunc generatedCatalogClientDefault(platform string) string {\n\treturn generatedCatalogClientDefaults[strings.ToLower(strings.TrimSpace(platform))]\n}\n\nfunc generatedCatalogGroupPolicyFor(platform string) generatedCatalogGroupPolicy {\n\treturn generatedCatalogGroupPolicies[strings.ToLower(strings.TrimSpace(platform))]\n}\n""" % ("\n".join(gofmt_align_map_rows(price_rows)), "\n".join(gofmt_align_map_rows(display_rows)), "\n".join(gofmt_align_map_rows(default_rows)), "\n".join(gofmt_align_map_rows(group_rows)))



def codex_client_catalog(catalog: dict[str, Any]) -> dict[str, Any]:
    base = json.loads(CODEX_CLIENT_BASE.read_text(encoding="utf-8"))
    if not isinstance(base, dict) or not isinstance(base.get("models"), list):
        fail("model-catalog/codex-client-base.json must contain a models array")
    replacements = {
        model["id"]: model["client_config"]["codex_catalog_entry"]
        for model in catalog["models"]
        if model["platform"] == "openai" and model.get("client_config", {}).get("codex_catalog_entry")
    }
    contract_by_model: dict[str, dict[str, Any]] = {}
    for path in sorted(MODEL_CONTRACT_DIR.glob("*.json")):
        if path.name in {"client-matrix.json", "matrix-schema.json", "import-provenance.json"}:
            continue
        contract = json.loads(path.read_text(encoding="utf-8"))
        model_id = str(contract.get("model", {}).get("id", "")).strip()
        if model_id:
            contract_by_model[model_id] = contract

    template = replacements.get("gpt-5.6-sol") or next(
        (entry for entry in base["models"] if entry.get("slug") == "gpt-5.6-sol"),
        base["models"][0],
    )
    codex_efforts = {"none", "minimal", "low", "medium", "high", "xhigh", "max"}
    catalog_by_model = {model["id"]: model for model in catalog["models"]}
    for model_id, contract in contract_by_model.items():
        if model_id in replacements or not any(
            row.get("name") == "responses" and row.get("status") == "verified"
            for row in contract.get("protocols", [])
        ):
            continue
        catalog_model = catalog_by_model.get(model_id)
        contract_model = contract.get("model", {})
        context_window = (catalog_model or {}).get("context_window") or contract_model.get("context_window")
        if not isinstance(context_window, int) or context_window <= 0:
            continue
        display_name = (catalog_model or {}).get("display_name") or contract_model.get("display_name") or model_id
        entry = json.loads(json.dumps(template))
        entry["slug"] = model_id
        entry["display_name"] = display_name
        entry["description"] = f"{display_name} via the 老实人AI Responses gateway."
        entry["context_window"] = context_window
        entry["max_context_window"] = context_window
        entry["auto_compact_token_limit"] = int(context_window * 0.95)
        entry["input_modalities"] = contract.get("model", {}).get("input_modalities") or ["text"]
        levels = [
            level for level in contract.get("reasoning", {}).get("model_levels", [])
            if level in codex_efforts
        ]
        entry["supported_reasoning_levels"] = [
            {"effort": level, "description": f"{level} reasoning"}
            for level in levels
        ]
        entry["default_reasoning_level"] = "high" if "high" in levels else (levels[0] if levels else None)
        entry["additional_speed_tiers"] = []
        entry["service_tiers"] = []
        entry["priority"] = 999
        replacements[model_id] = entry
    merged_models: list[dict[str, Any]] = []
    seen: set[str] = set()
    default = default_model(catalog, "openai")
    if default and default["id"] in replacements:
        merged_models.append(replacements[default["id"]])
        seen.add(default["id"])
    for entry in base["models"]:
        if not isinstance(entry, dict) or not isinstance(entry.get("slug"), str):
            fail("model-catalog/codex-client-base.json contains an invalid model entry")
        slug = entry["slug"]
        if slug in seen:
            continue
        merged_models.append(replacements.get(slug, entry))
        seen.add(slug)
    for slug in sorted(replacements):
        if slug not in seen:
            merged_models.append(replacements[slug])
    base["models"] = merged_models
    return base


def render_codex_client_catalog(catalog: dict[str, Any]) -> str:
    return json.dumps(codex_client_catalog(catalog), ensure_ascii=False, indent=2) + "\n"


def render_ts(catalog: dict[str, Any]) -> str:
    models = []
    for model in sorted(catalog["models"], key=lambda row: (row["platform"], row["id"])):
        models.append({
            "id": model["id"],
            "upstreamId": model["upstream_id"],
            "displayName": model["display_name"],
            "platform": model["platform"],
            "contextWindow": model["context_window"],
            "maxOutputTokens": model["max_output_tokens"],
            "clientDefault": bool(model.get("client_default")),
            "preferredGroupName": model["public_group"]["preferred_name"],
            "legacyGroupNames": model["public_group"].get("legacy_names", []),
            "preset": model["preset"],
        })
    encoded = json.dumps(models, ensure_ascii=False, indent=2)
    client_defaults = {
        platform: values["id"]
        for platform, values in installer_model_values(catalog).items()
    }
    client_defaults_encoded = json.dumps(client_defaults, ensure_ascii=False, indent=2)
    return f"""// Code generated by scripts/model_catalog.py; DO NOT EDIT.\n\nexport interface CatalogModel {{\n  id: string\n  upstreamId: string\n  displayName: string\n  platform: 'openai' | 'anthropic' | 'grok' | 'gemini'\n  contextWindow: number\n  maxOutputTokens: number\n  clientDefault: boolean\n  preferredGroupName: string\n  legacyGroupNames: readonly string[]\n  preset: {{ from: string; to: string; color: string }}\n}}\n\nexport const clientAutoConfigVersion = {json.dumps(catalog['client_auto_config_version'])}\n\nexport const clientAutoConfigDefaults = {client_defaults_encoded} as const\n\nexport const modelCatalog: readonly CatalogModel[] = {encoded}\n\nexport type CatalogPlatform = CatalogModel['platform']\n\nexport const catalogModelsForPlatform = (platform: string): string[] =>\n  modelCatalog.filter((model) => model.platform === platform).map((model) => model.id)\n\nexport const catalogPresetMappingsForPlatform = (platform: string) =>\n  modelCatalog\n    .filter((model) => model.platform === platform)\n    .map((model) => ({{\n      label: model.displayName,\n      from: model.preset.from,\n      to: model.preset.to,\n      color: model.preset.color\n    }}))\n\nexport const optionalCatalogClientDefaultForPlatform = (platform: string) =>\n  modelCatalog.find((candidate) => candidate.platform === platform && candidate.clientDefault)\n\nexport const catalogClientDefaultForPlatform = (platform: string) => {{\n  const model = optionalCatalogClientDefaultForPlatform(platform)\n  if (!model) throw new Error(`missing catalog client default for ${{platform}}`)\n  return model\n}}\n"""


def render_codex_ts(catalog: dict[str, Any]) -> str:
    models = []
    for entry in codex_client_catalog(catalog)["models"]:
        slug = entry.get("slug")
        display_name = entry.get("display_name")
        context_window = entry.get("context_window")
        if not isinstance(slug, str) or not isinstance(display_name, str) or not isinstance(context_window, int):
            fail("Codex client catalog entries require slug, display_name, and integer context_window")
        reasoning_levels = [
            row["effort"]
            for row in entry.get("supported_reasoning_levels", [])
            if isinstance(row, dict) and isinstance(row.get("effort"), str)
        ]
        default_reasoning_level = entry.get("default_reasoning_level")
        models.append({
            "model": slug,
            "displayName": display_name,
            "contextWindow": context_window,
            "reasoningLevels": reasoning_levels,
            "defaultReasoningLevel": default_reasoning_level if isinstance(default_reasoning_level, str) else None,
        })
    encoded = json.dumps(models, ensure_ascii=False, indent=2)
    return f'''// Code generated by scripts/model_catalog.py; DO NOT EDIT.

export interface CodexClientModel {{
  model: string
  displayName: string
  contextWindow: number
  reasoningLevels: readonly string[]
  defaultReasoningLevel: string | null
}}

export const codexClientModels: readonly CodexClientModel[] = {encoded}
'''


def outputs(catalog: dict[str, Any]) -> dict[Path, str]:
    return {
        GO_OUTPUT: render_go(catalog),
        TS_OUTPUT: render_ts(catalog),
        CODEX_TS_OUTPUT: render_codex_ts(catalog),
        CODEX_CLIENT_OUTPUT: render_codex_client_catalog(catalog),
    }


def render_installer_integrity() -> str:
    shell_sha = hashlib.sha256(SHELL_INSTALLER.read_bytes()).hexdigest()
    powershell_sha = hashlib.sha256(POWERSHELL_INSTALLER.read_bytes()).hexdigest()
    return (
        "// Code generated by scripts/model_catalog.py; DO NOT EDIT.\n\n"
        f"export const shellInstallerSha256 = {json.dumps(shell_sha)}\n"
        f"export const powershellInstallerSha256 = {json.dumps(powershell_sha)}\n"
    )


def replace_generated_block(text: str, begin: str, end: str, replacement: str) -> str:
    start = text.find(begin)
    finish = text.find(end)
    if start < 0 or finish < start:
        fail(f"missing generated block markers: {begin}")
    finish += len(end)
    return text[:start] + replacement + text[finish:]


def installer_blocks(catalog: dict[str, Any]) -> dict[Path, tuple[str, str, str]]:
    return {
        POWERSHELL_INSTALLER: (
            POWERSHELL_BLOCK_BEGIN,
            POWERSHELL_BLOCK_END,
            render_powershell_block(catalog),
        ),
        SHELL_INSTALLER: (
            SHELL_BLOCK_BEGIN,
            SHELL_BLOCK_END,
            render_shell_block(catalog),
        ),
    }


def replace_installer_version_references(text: str, version: str) -> str:
    return INSTALLER_VERSION_REFERENCE.sub(lambda match: match.group(1) + version, text)


def apply(catalog: dict[str, Any], catalog_path: Path) -> None:
    changed: list[str] = []
    for path, content in outputs(catalog).items():
        path.parent.mkdir(parents=True, exist_ok=True)
        if path.exists() and path.read_text(encoding="utf-8") == content:
            continue
        path.write_text(content, encoding="utf-8")
        changed.append(str(path.relative_to(ROOT)))
    blocks = installer_blocks(catalog)
    for path, (begin, end, block) in blocks.items():
        current = path.read_text(encoding="utf-8")
        content = replace_generated_block(current, begin, end, block)
        content = replace_installer_version_references(content, catalog["client_auto_config_version"])
        if content != current:
            path.write_text(content, encoding="utf-8")
            changed.append(str(path.relative_to(ROOT)))
    for path in VERSION_REFERENCE_PATHS:
        if path in blocks:
            continue
        current = path.read_text(encoding="utf-8")
        content = replace_installer_version_references(current, catalog["client_auto_config_version"])
        if content != current:
            path.write_text(content, encoding="utf-8")
            changed.append(str(path.relative_to(ROOT)))
    integrity = render_installer_integrity()
    if not INSTALLER_INTEGRITY_OUTPUT.exists() or INSTALLER_INTEGRITY_OUTPUT.read_text(encoding="utf-8") != integrity:
        INSTALLER_INTEGRITY_OUTPUT.write_text(integrity, encoding="utf-8")
        changed.append(str(INSTALLER_INTEGRITY_OUTPUT.relative_to(ROOT)))
    print(json.dumps({"changed": changed, "catalog": str(catalog_path)}, ensure_ascii=False))


def check(catalog: dict[str, Any]) -> None:
    stale = [str(path.relative_to(ROOT)) for path, content in outputs(catalog).items() if not path.exists() or path.read_text(encoding="utf-8") != content]
    for path, (begin, end, block) in installer_blocks(catalog).items():
        current = path.read_text(encoding="utf-8")
        if replace_generated_block(current, begin, end, block) != current:
            stale.append(str(path.relative_to(ROOT)))
    expected_version = catalog["client_auto_config_version"]
    for path in VERSION_REFERENCE_PATHS:
        current = path.read_text(encoding="utf-8")
        versions = {match.group(2) for match in INSTALLER_VERSION_REFERENCE.finditer(current)}
        if versions and versions != {expected_version}:
            stale.append(str(path.relative_to(ROOT)))
    if not INSTALLER_INTEGRITY_OUTPUT.exists() or INSTALLER_INTEGRITY_OUTPUT.read_text(encoding="utf-8") != render_installer_integrity():
        stale.append(str(INSTALLER_INTEGRITY_OUTPUT.relative_to(ROOT)))
    if stale:
        fail("generated model catalog is stale: " + ", ".join(dict.fromkeys(stale)))
    print(json.dumps({"valid": True, "generated_files": len(outputs(catalog)) + len(installer_blocks(catalog)), "models": len(catalog["models"])}, ensure_ascii=False))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("apply", "check"))
    parser.add_argument("--catalog", type=Path, default=DEFAULT_CATALOG)
    parser.add_argument("--manifest", type=Path, help="merge one validated release manifest before applying")
    args = parser.parse_args()
    try:
        catalog_path = args.catalog.resolve()
        catalog = load_catalog(catalog_path)
        if args.manifest:
            if args.command != "apply":
                fail("--manifest is only valid with apply")
            # The normal apply path cannot publish a draft/v1 declaration or
            # a fixture masquerading as production evidence.
            import model_release
            from model_evidence_integrity import artifact_gaps
            from datetime import datetime
            from zoneinfo import ZoneInfo
            manifest = json.loads(args.manifest.read_text())
            model_release.validate(manifest)
            gaps = model_release.contract_gaps(manifest)
            if gaps:
                fail("release manifest is not ready: " + "; ".join(gaps))
            for evidence in manifest.get("evidence", []):
                if evidence.get("kind") not in {"live_probe", "billing_reconciliation"}:
                    continue
                cell = {"status":"verified" if evidence.get("result") == "pass" else "unsupported", "source_ref":evidence.get("artifact_path"),
                        "artifact_sha256":evidence.get("artifact_sha256"),
                        "observed_at":evidence.get("observed_at"),
                        "model_id":manifest["model"]["id"]}
                errors = artifact_gaps(cell, args.manifest.resolve().parent,
                                      as_of=datetime.now(ZoneInfo("Asia/Shanghai")).date())
                if errors:
                    fail("unresolved live release evidence: " + "; ".join(errors))
            catalog = merge_manifest(catalog, args.manifest.resolve())
            catalog_path.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        apply(catalog, catalog_path) if args.command == "apply" else check(catalog)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

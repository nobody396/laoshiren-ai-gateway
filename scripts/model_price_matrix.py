#!/usr/bin/env python3
"""Plan or audit the public model x group price matrix.

The command is deliberately read-only.  It accepts a model catalog, one public
pricing inventory (a JSON file or URL), and an optional *exported* database
fixture.  It never opens a database connection and never applies price changes.

Database fixtures may either use the public inventory ``groups`` shape or a
flat ``rows`` array.  Flat rows must contain ``model``/``model_id`` and
``group_id``; price fields may be top-level or inside ``prices``.  The declared
unit controls token-price normalization (per-token, per-1k, or per-1m).
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import json
import math
from pathlib import Path
import sys
from typing import Any, Iterable
import urllib.request


SCHEMA_VERSION = 2
KIND = "model_price_matrix"
CANONICAL_UNIT = "per_1m_tokens"
PRICE_SCOPES = ("provider_public", "gateway_base", "group_customer")
COMPONENT_STATES = {
    "verified", "unknown", "blocked", "not_applicable", "not_published", "not_exposed",
}
PRICE_COLUMNS = ("input", "output", "cache_read", "cache_write")
LONG_CONTEXT_COLUMNS = (
    "long_context_threshold",
    "long_context_input_multiplier",
    "long_context_cached_input_multiplier",
    "long_context_output_multiplier",
)
IMAGE_COLUMNS = (
    "image_mode",
    "image_price_per_image",
    "image_text_input",
    "image_text_cache_read",
    "image_input",
    "image_cache_read",
    "image_output",
)
MATRIX_COLUMNS = (
    "model_id",
    "price_scope",
    "group_id",
    "group_name",
    "platform",
    "rate_multiplier",
    "currency",
    "unit",
    *PRICE_COLUMNS,
    *LONG_CONTEXT_COLUMNS,
    *IMAGE_COLUMNS,
    "component_status",
    "component_evidence",
    "public_price_visibility",
    "derivation",
    "disabled",
    "status",
)

UNIT_FACTORS = {
    "per_1m_tokens": 1.0,
    "per_million_tokens": 1.0,
    "per_mtok": 1.0,
    "per_1000000_tokens": 1.0,
    "per_1k_tokens": 1000.0,
    "per_thousand_tokens": 1000.0,
    "per_token": 1_000_000.0,
}


class MatrixInputError(ValueError):
    """An input is unsafe or ambiguous, so audit must fail closed."""


@dataclass(frozen=True)
class Source:
    label: str
    value: str


def _load_json_file(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def _load_json_url(url: str) -> Any:
    if not url.startswith(("http://", "https://")):
        raise MatrixInputError("pricing URL must use http or https")
    request = urllib.request.Request(url, headers={"Accept": "application/json"})
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.loads(response.read().decode("utf-8"))


def _object(value: Any, path: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise MatrixInputError(f"{path} must be an object")
    return value


def _array(value: Any, path: str) -> list[Any]:
    if not isinstance(value, list):
        raise MatrixInputError(f"{path} must be an array")
    return value


def _text(value: Any, path: str, *, optional: bool = False) -> str | None:
    if optional and value is None:
        return None
    if not isinstance(value, str) or not value.strip():
        raise MatrixInputError(f"{path} must be a non-empty string")
    return value.strip()


def _number(value: Any, path: str, *, optional: bool = True, positive: bool = False) -> float | None:
    if value is None and optional:
        return None
    if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(float(value)):
        raise MatrixInputError(f"{path} must be a finite number")
    result = float(value)
    if result < 0 or (positive and result <= 0):
        qualifier = "positive" if positive else "non-negative"
        raise MatrixInputError(f"{path} must be {qualifier}")
    return result


def _group_id(value: Any, path: str) -> int | str:
    if isinstance(value, bool) or not isinstance(value, (int, str)):
        raise MatrixInputError(f"{path} must be an integer or string")
    if isinstance(value, str) and not value.strip():
        raise MatrixInputError(f"{path} must not be empty")
    return value


def _unit(value: Any, path: str) -> tuple[str, float]:
    unit = _text(value, path)
    assert unit is not None
    key = unit.lower().replace("-", "_")
    if key not in UNIT_FACTORS:
        raise MatrixInputError(f"{path}: unsupported price unit {unit!r}")
    return CANONICAL_UNIT, UNIT_FACTORS[key]


def _scaled(value: Any, factor: float, path: str) -> float | None:
    number = _number(value, path)
    if number is None:
        return None
    return number * factor


def _empty_row() -> dict[str, Any]:
    return {column: None for column in (*PRICE_COLUMNS, *LONG_CONTEXT_COLUMNS, *IMAGE_COLUMNS)}


def load_catalog(
    raw: Any, source: Source,
) -> tuple[dict[str, dict[str, Any]], list[dict[str, Any]], dict[str, dict[str, Any]]]:
    root = _object(raw, "catalog")
    models = _array(root.get("models"), "catalog.models")
    result: dict[str, dict[str, Any]] = {}
    issues: list[dict[str, Any]] = []
    for index, raw_model in enumerate(models):
        path = f"catalog.models[{index}]"
        model = _object(raw_model, path)
        model_id = _text(model.get("id"), f"{path}.id")
        assert model_id is not None
        if model_id in result:
            issues.append(issue("conflict", model_id, None, "model_id", model_id, model_id,
                                "duplicate model id in catalog", ["catalog", "catalog"]))
            continue
        pricing = _object(model.get("pricing"), f"{path}.pricing")
        provider_pricing = (
            _object(model.get("provider_pricing"), f"{path}.provider_pricing")
            if model.get("provider_pricing") is not None else pricing
        )
        public_group = _object(model.get("public_group"), f"{path}.public_group")
        scope = provider_pricing.get("price_scope", "provider_public")
        if scope != "provider_public":
            raise MatrixInputError(f"{path}.pricing.price_scope must be provider_public")
        visibility = model.get("public_price_visibility", "public")
        if visibility not in {"public", "hidden"}:
            raise MatrixInputError(f"{path}.public_price_visibility must be public or hidden")
        declared_states = provider_pricing.get("component_status", {})
        if not isinstance(declared_states, dict):
            raise MatrixInputError(f"{path}.pricing.component_status must be an object")
        entry = {
            "model_id": model_id,
            "platform": _text(model.get("platform"), f"{path}.platform"),
            "preferred_group_name": _text(public_group.get("preferred_name"), f"{path}.public_group.preferred_name"),
            "currency": "USD",
            "unit": CANONICAL_UNIT,
            "price_scope": scope,
            "public_price_visibility": visibility,
            "input": _number(provider_pricing.get("input_per_mtok_usd"), f"{path}.provider_pricing.input_per_mtok_usd", optional=False),
            "output": _number(provider_pricing.get("output_per_mtok_usd"), f"{path}.provider_pricing.output_per_mtok_usd", optional=False),
            "cache_read": _number(provider_pricing.get("cached_input_per_mtok_usd"), f"{path}.provider_pricing.cached_input_per_mtok_usd"),
            # Forward-compatible: the current catalog does not yet expose this.
            "cache_write": _number(provider_pricing.get("cache_write_per_mtok_usd"), f"{path}.provider_pricing.cache_write_per_mtok_usd"),
            "long_context_threshold": _number(provider_pricing.get("long_context_input_threshold"), f"{path}.provider_pricing.long_context_input_threshold"),
            "long_context_input_multiplier": _number(provider_pricing.get("long_context_input_multiplier"), f"{path}.provider_pricing.long_context_input_multiplier"),
            "long_context_cached_input_multiplier": _number(provider_pricing.get("long_context_cached_input_multiplier"), f"{path}.provider_pricing.long_context_cached_input_multiplier"),
            "long_context_output_multiplier": _number(provider_pricing.get("long_context_output_multiplier"), f"{path}.provider_pricing.long_context_output_multiplier"),
            "evidence_url": _text(provider_pricing.get("evidence_url"), f"{path}.provider_pricing.evidence_url", optional=True),
            "source": source.value,
        }
        entry["component_status"] = {
            "input": declared_states.get("input", "verified"),
            "output": declared_states.get("output", "verified"),
            "cached_input": declared_states.get(
                "cached_input", declared_states.get(
                    "cache_read", "verified" if entry["cache_read"] is not None else "unknown"
                )
            ),
            "cache_write": declared_states.get("cache_write", "not_applicable"),
            "long_context": declared_states.get(
                "long_context",
                "verified" if entry["long_context_threshold"] is not None else "not_applicable",
            ),
        }
        invalid_states = sorted(set(entry["component_status"].values()) - COMPONENT_STATES)
        if invalid_states:
            raise MatrixInputError(f"{path}.pricing.component_status contains invalid states: {', '.join(invalid_states)}")
        threshold_parts = [entry[column] for column in (
            "long_context_threshold", "long_context_input_multiplier", "long_context_output_multiplier"
        )]
        if any(value is not None for value in threshold_parts) and not all(value is not None for value in threshold_parts):
            issues.append(issue("conflict", model_id, None, "long_context", threshold_parts, None,
                                "catalog long-context threshold and both multipliers must be all set or all null",
                                ["catalog", "catalog"]))
        result[model_id] = entry
    gap_rows = root.get("price_gaps", [])
    if not isinstance(gap_rows, list):
        raise MatrixInputError("catalog.price_gaps must be an array")
    gaps: dict[str, dict[str, Any]] = {}
    for index, raw_gap in enumerate(gap_rows):
        path = f"catalog.price_gaps[{index}]"
        gap = _object(raw_gap, path)
        model_id = _text(gap.get("model_id"), f"{path}.model_id")
        assert model_id is not None
        if gap.get("price_scope") != "provider_public":
            raise MatrixInputError(f"{path}.price_scope must be provider_public")
        if gap.get("status") not in {"unknown", "blocked", "not_published"}:
            raise MatrixInputError(f"{path}.status must be unknown, blocked, or not_published")
        reason = _text(gap.get("reason"), f"{path}.reason")
        gaps[model_id] = {
            "model_id": model_id,
            "price_scope": "provider_public",
            "status": gap["status"],
            "reason": reason,
            "evidence_url": gap.get("evidence_url"),
            "source": source.value,
        }
    return result, issues, gaps


def _token_prices(raw: dict[str, Any], factor: float, path: str) -> dict[str, float | None]:
    nested = raw.get("prices")
    prices = _object(nested, f"{path}.prices") if nested is not None else raw
    aliases = {
        "input": ("input", "input_price"),
        "output": ("output", "output_price"),
        "cache_read": ("cache_read", "cache_read_price", "cached_input_price"),
        "cache_write": ("cache_write", "cache_write_price"),
    }
    result: dict[str, float | None] = {}
    for canonical, names in aliases.items():
        value = next((prices[name] for name in names if name in prices), None)
        result[canonical] = _scaled(value, factor, f"{path}.{canonical}")
    return result


def _long_context(raw: dict[str, Any], path: str) -> dict[str, float | None]:
    nested = raw.get("long_context")
    data = _object(nested, f"{path}.long_context") if nested is not None else raw
    threshold = data.get("input_threshold", data.get("long_context_threshold"))
    input_multiplier = data.get("input_multiplier", data.get("long_context_input_multiplier"))
    output_multiplier = data.get("output_multiplier", data.get("long_context_output_multiplier"))
    cached_input_multiplier = data.get(
        "cached_input_multiplier", data.get("long_context_cached_input_multiplier")
    )
    values = {
        "long_context_threshold": _number(threshold, f"{path}.long_context_threshold"),
        "long_context_input_multiplier": _number(input_multiplier, f"{path}.long_context_input_multiplier"),
        "long_context_cached_input_multiplier": _number(cached_input_multiplier, f"{path}.long_context_cached_input_multiplier"),
        "long_context_output_multiplier": _number(output_multiplier, f"{path}.long_context_output_multiplier"),
    }
    present = [values[name] is not None for name in (
        "long_context_threshold", "long_context_input_multiplier", "long_context_output_multiplier"
    )]
    if any(present) and not all(present):
        raise MatrixInputError(f"{path}.long_context must contain threshold and both multipliers")
    return values


def _image_prices(raw: dict[str, Any], factor: float, path: str) -> dict[str, Any]:
    nested = raw.get("image_generation", raw.get("image"))
    data = _object(nested, f"{path}.image") if nested is not None else raw
    mode = data.get("mode", data.get("image_mode"))
    if mode is not None and mode not in {"fixed_per_image", "token"}:
        raise MatrixInputError(f"{path}.image mode must be fixed_per_image or token")
    aliases = {
        "image_price_per_image": ("price_per_image", "image_price_per_image"),
        "image_text_input": ("text_input_price", "image_text_input"),
        "image_text_cache_read": ("text_cached_input_price", "image_text_cache_read"),
        "image_input": ("image_input_price", "image_input"),
        "image_cache_read": ("image_cached_input_price", "image_cache_read"),
        "image_output": ("image_output_price", "image_output"),
    }
    result: dict[str, Any] = {"image_mode": mode}
    for canonical, names in aliases.items():
        value = next((data[name] for name in names if name in data), None)
        # A per-image price is not a token unit and must not be rescaled.
        scale = 1.0 if canonical == "image_price_per_image" else factor
        result[canonical] = _scaled(value, scale, f"{path}.{canonical}")
    if mode == "fixed_per_image" and result["image_price_per_image"] is None:
        raise MatrixInputError(f"{path}.image.price_per_image is required for fixed_per_image")
    if mode == "token" and not any(result[column] is not None for column in IMAGE_COLUMNS[2:]):
        raise MatrixInputError(f"{path}.image token mode requires at least one token price")
    return result


def normalize_group_inventory(raw: Any, source: Source) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    root = _object(raw, source.label)
    # Some HTTP wrappers return the catalog under data.
    if isinstance(root.get("data"), dict) and "groups" in root["data"]:
        root = _object(root["data"], f"{source.label}.data")
    currency = _text(root.get("currency"), f"{source.label}.currency")
    canonical_unit, factor = _unit(root.get("unit"), f"{source.label}.unit")
    groups = _array(root.get("groups"), f"{source.label}.groups")
    rows: list[dict[str, Any]] = []
    for group_index, raw_group in enumerate(groups):
        path = f"{source.label}.groups[{group_index}]"
        group = _object(raw_group, path)
        group_id = _group_id(group.get("group_id"), f"{path}.group_id")
        group_name = _text(group.get("name"), f"{path}.name")
        platform = _text(group.get("platform"), f"{path}.platform")
        multiplier = _number(group.get("rate_multiplier"), f"{path}.rate_multiplier", optional=False, positive=True)
        assert group_name is not None and platform is not None and multiplier is not None
        models = group.get("models")
        if models is None:
            models = []
        for model_index, raw_model in enumerate(_array(models, f"{path}.models")):
            model_path = f"{path}.models[{model_index}]"
            model = _object(raw_model, model_path)
            model_id = _text(model.get("model", model.get("model_id")), f"{model_path}.model")
            assert model_id is not None
            row = _empty_row()
            row.update({
                "model_id": model_id,
                "group_id": group_id,
                "group_name": group_name,
                "platform": platform,
                "rate_multiplier": multiplier,
                "currency": currency,
                "unit": canonical_unit,
                "disabled": bool(model.get("disabled", False)),
                "source": source.value,
            })
            row.update(_token_prices(model, factor, model_path))
            row.update(_long_context(model, model_path))
            rows.append(row)
        image = group.get("image_generation")
        if image is not None:
            row = _empty_row()
            row.update({
                "model_id": "gpt-image-2",
                "group_id": group_id,
                "group_name": group_name,
                "platform": platform,
                "rate_multiplier": multiplier,
                "currency": currency,
                "unit": canonical_unit,
                "disabled": False,
                "source": source.value,
            })
            row.update(_image_prices(group, factor, path))
            rows.append(row)
    return rows, {"currency": currency, "unit": canonical_unit, "source": source.value}


def normalize_flat_inventory(raw: Any, source: Source) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    if isinstance(raw, list):
        root: dict[str, Any] = {"rows": raw, "currency": "CNY", "unit": CANONICAL_UNIT}
    else:
        root = _object(raw, source.label)
    currency = _text(root.get("currency"), f"{source.label}.currency")
    canonical_unit, root_factor = _unit(root.get("unit"), f"{source.label}.unit")
    rows: list[dict[str, Any]] = []
    for index, raw_row in enumerate(_array(root.get("rows"), f"{source.label}.rows")):
        path = f"{source.label}.rows[{index}]"
        item = _object(raw_row, path)
        row_currency = _text(item.get("currency", currency), f"{path}.currency")
        row_unit, factor = _unit(item.get("unit", root.get("unit")), f"{path}.unit")
        model_id = _text(item.get("model", item.get("model_id")), f"{path}.model")
        group_id = _group_id(item.get("group_id"), f"{path}.group_id")
        group_name = _text(item.get("group_name", item.get("name")), f"{path}.group_name")
        platform = _text(item.get("platform"), f"{path}.platform", optional=True)
        multiplier = _number(item.get("rate_multiplier"), f"{path}.rate_multiplier", positive=True)
        assert model_id is not None and group_name is not None
        row = _empty_row()
        row.update({
            "model_id": model_id,
            "group_id": group_id,
            "group_name": group_name,
            "platform": platform,
            "rate_multiplier": multiplier,
            "currency": row_currency,
            "unit": row_unit,
            "disabled": bool(item.get("disabled", False)),
            "source": source.value,
        })
        row.update(_token_prices(item, factor, path))
        row.update(_long_context(item, path))
        if item.get("image") is not None or item.get("image_generation") is not None or item.get("image_mode") is not None:
            row.update(_image_prices(item, factor, path))
        rows.append(row)
    # root_factor is deliberately evaluated even when rows override the unit.
    assert root_factor > 0
    return rows, {"currency": currency, "unit": canonical_unit, "source": source.value}


def normalize_inventory(raw: Any, source: Source) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    if isinstance(raw, dict):
        candidate = raw.get("data") if isinstance(raw.get("data"), dict) else raw
        if isinstance(candidate, dict) and "groups" in candidate:
            return normalize_group_inventory(raw, source)
    if isinstance(raw, list) or (isinstance(raw, dict) and "rows" in raw):
        return normalize_flat_inventory(raw, source)
    raise MatrixInputError(f"{source.label} must contain groups or rows")


def issue(kind: str, model_id: str | None, group_id: Any, column: str | None,
          expected: Any, actual: Any, message: str, sources: list[str]) -> dict[str, Any]:
    return {
        "type": kind,
        "model_id": model_id,
        "group_id": group_id,
        "column": column,
        "expected": expected,
        "actual": actual,
        "sources": sources,
        "message": message,
    }


def _row_key(row: dict[str, Any]) -> tuple[str, str]:
    return str(row["model_id"]).lower(), str(row["group_id"])


def _equal(expected: Any, actual: Any, tolerance: float) -> bool:
    if expected is None or actual is None:
        return expected is actual
    if isinstance(expected, (int, float)) and isinstance(actual, (int, float)):
        return math.isclose(float(expected), float(actual), rel_tol=tolerance, abs_tol=tolerance)
    return expected == actual


def _row_status(component_status: dict[str, str]) -> str:
    states = set(component_status.values())
    if "blocked" in states:
        return "blocked"
    if "unknown" in states:
        return "unknown"
    if "not_published" in states:
        return "not_published"
    if "not_exposed" in states:
        return "not_exposed"
    if states == {"not_applicable"}:
        return "not_applicable"
    return "verified"


def _long_context_status(provider: dict[str, Any] | None, row: dict[str, Any]) -> str:
    public_values = [row.get(column) for column in (
        "long_context_threshold", "long_context_input_multiplier", "long_context_output_multiplier"
    )]
    if all(value is not None for value in public_values):
        return "verified"
    if provider and provider["component_status"].get("long_context") == "verified":
        # The current public pricing API intentionally exposes long-context
        # tiers only for OpenAI models. Provider rules for Gemini/Grok remain
        # valid but are terminally not_exposed on this customer surface.
        return "unknown" if row.get("platform") == "openai" else "not_exposed"
    return "not_applicable"


def _public_component_status(provider: dict[str, Any] | None, row: dict[str, Any]) -> dict[str, str]:
    return {
        "input": "verified" if row.get("input") is not None else "blocked",
        "output": "verified" if row.get("output") is not None else "blocked",
        # Numeric zero is a real free price. Null is not zero and remains
        # unknown when the provider has a cache claim.
        "cached_input": (
            "verified" if row.get("cache_read") is not None
            else "unknown" if provider and provider["component_status"].get("cached_input") != "not_applicable"
            else "not_applicable"
        ),
        "cache_write": "verified" if row.get("cache_write") is not None else "not_applicable",
        "long_context": _long_context_status(provider, row),
    }


def _scoped_row(
    source_row: dict[str, Any], *, scope: str, values: dict[str, Any],
    component_status: dict[str, str], visibility: str, derivation: str | None,
    evidence: dict[str, Any], disabled: bool = False,
) -> dict[str, Any]:
    row = _empty_row()
    row.update({
        "model_id": source_row.get("model_id"),
        "price_scope": scope,
        "group_id": source_row.get("group_id"),
        "group_name": source_row.get("group_name"),
        "platform": source_row.get("platform"),
        "rate_multiplier": source_row.get("rate_multiplier"),
        "currency": source_row.get("currency"),
        "unit": source_row.get("unit", CANONICAL_UNIT),
        "component_status": component_status,
        "component_evidence": {},
        "public_price_visibility": visibility,
        "derivation": derivation,
        "disabled": disabled,
        "status": "disabled" if disabled else _row_status(component_status),
        "evidence": evidence,
        "drift": [],
    })
    row.update(values)
    return row


def build_report(
    catalog: dict[str, dict[str, Any]],
    public_rows: list[dict[str, Any]],
    public_meta: dict[str, Any],
    db_rows: list[dict[str, Any]] | None,
    db_meta: dict[str, Any] | None,
    catalog_issues: list[dict[str, Any]],
    price_gaps: dict[str, dict[str, Any]],
    *,
    mode: str,
    catalog_source: Source,
    public_source: Source,
    db_source: Source | None,
    tolerance: float,
) -> dict[str, Any]:
    """Join price scopes without pretending that they are the same price.

    ``provider_public`` is the official/provider price book from the catalog.
    ``group_customer`` is the customer-visible runtime price. ``gateway_base``
    is an explicit algebraic readback (customer / group multiplier), never an
    inferred provider claim. Cross-scope differences are therefore facts, not
    conflicts.
    """
    issues = list(catalog_issues)
    matrix_rows: list[dict[str, Any]] = []
    public_by_key: dict[tuple[str, str], dict[str, Any]] = {}
    active_models: set[str] = set()
    disabled_models: set[str] = set()

    for raw_row in public_rows:
        row = dict(raw_row)
        key = _row_key(row)
        if key in public_by_key:
            issues.append(issue("conflict", row["model_id"], row["group_id"], "row", None, None,
                                "duplicate public model/group row", ["public", "public"]))
        else:
            public_by_key[key] = row
        if row["disabled"]:
            disabled_models.add(str(row["model_id"]))
        else:
            active_models.add(str(row["model_id"]))

    db_by_key: dict[tuple[str, str], dict[str, Any]] = {}
    if db_rows is not None:
        for row in db_rows:
            key = _row_key(row)
            if key in db_by_key:
                issues.append(issue("conflict", row["model_id"], row["group_id"], "row", None, None,
                                    "duplicate database fixture model/group row", ["database", "database"]))
            else:
                db_by_key[key] = row

    # Provider-public facts are one row per catalog model, independent of
    # whether the gateway currently exposes that model.
    for model_id, entry in sorted(catalog.items()):
        source_row = {
            "model_id": model_id,
            "group_id": None,
            "group_name": entry.get("preferred_group_name"),
            "platform": entry.get("platform"),
            "rate_multiplier": None,
            "currency": entry.get("currency"),
            "unit": entry.get("unit"),
        }
        values = {column: entry.get(column) for column in (*PRICE_COLUMNS, *LONG_CONTEXT_COLUMNS)}
        provider_row = _scoped_row(
            source_row,
            scope="provider_public",
            values=values,
            component_status=dict(entry["component_status"]),
            visibility=entry["public_price_visibility"],
            derivation=None,
            evidence={"catalog": {"source": catalog_source.value, "url": entry.get("evidence_url")},
                      "public": None, "database": None},
        )
        matrix_rows.append(provider_row)
        for component, state in entry["component_status"].items():
            if state in {"unknown", "blocked"}:
                problem = issue(state, model_id, None, component, "known provider-public price", None,
                                f"provider_public {component} is {state}", ["catalog"])
                issues.append(problem)
                provider_row["drift"].append({key: value for key, value in problem.items()
                                               if key in {"type", "column", "expected", "actual", "message"}})

    # A public model can be priced before its full model-catalog contract is
    # publishable. Preserve the observed customer facts and emit one blocked
    # provider-public row; never invent context/output limits to force a row.
    public_model_ids = {
        str(row["model_id"]) for row in public_rows
        if not row["disabled"] and row.get("image_mode") is None
    }
    for model_id in sorted(public_model_ids - set(catalog)):
        gap = price_gaps.get(model_id)
        status = gap.get("status") if gap else "blocked"
        reason = gap.get("reason") if gap else "provider_public price is absent from the catalog"
        component_status = {component: status for component in ("input", "output", "cached_input", "long_context")}
        component_status["cache_write"] = "not_applicable"
        blocked_row = _scoped_row(
            {"model_id": model_id, "group_id": None, "group_name": None, "platform": None,
             "rate_multiplier": None, "currency": "USD", "unit": CANONICAL_UNIT},
            scope="provider_public", values={}, component_status=component_status,
            visibility="public", derivation=None,
            evidence={"catalog": {"source": catalog_source.value, "gap_reason": reason,
                                    "url": gap.get("evidence_url") if gap else None},
                      "public": None, "database": None},
        )
        blocked_row["component_evidence"] = {
            component: reason for component, state in component_status.items()
            if state == status
        }
        matrix_rows.append(blocked_row)
        if status in {"unknown", "blocked"}:
            problem = issue(status, model_id, None, "provider_public", "known provider-public price", None,
                            reason, ["catalog"])
            issues.append(problem)
            blocked_row["drift"].append({key: value for key, value in problem.items()
                                         if key in {"type", "column", "expected", "actual", "message"}})

    for key in sorted(public_by_key, key=lambda item: (item[0], item[1])):
        public = public_by_key[key]
        model_id = str(public["model_id"])
        is_image = public.get("image_mode") is not None
        catalog_entry = catalog.get(model_id)
        row_issues: list[dict[str, Any]] = []

        database = db_by_key.get(key) if db_rows is not None else None
        if db_rows is not None and database is None and not public["disabled"]:
            row_issues.append(issue("missing_database", model_id, public["group_id"], "row", "present", "missing",
                                    "active public row is absent from database export fixture",
                                    ["public", "database"]))
        elif database is not None:
            for column in ("group_name", "platform", "rate_multiplier", "currency", "unit", "disabled"):
                expected = database.get(column)
                actual = public.get(column)
                # Platform and multiplier are optional in flat DB exports.  If
                # exported, however, they become audit claims and must agree.
                if expected is None and column in {"platform", "rate_multiplier"}:
                    continue
                if not _equal(expected, actual, tolerance):
                    row_issues.append(issue("conflict", model_id, public["group_id"], column,
                                            expected, actual,
                                            f"public {column} conflicts with database export",
                                            ["database", "public"]))
            for column in (*PRICE_COLUMNS, *LONG_CONTEXT_COLUMNS, *IMAGE_COLUMNS):
                expected = database.get(column)
                actual = public.get(column)
                if not _equal(expected, actual, tolerance):
                    row_issues.append(issue("conflict", model_id, public["group_id"], column, expected, actual,
                                            f"public {column} conflicts with database export",
                                            ["database", "public"]))

        public_evidence = {
            "catalog": ({
                "source": catalog_source.value,
                "url": catalog_entry.get("evidence_url"),
            } if catalog_entry else None),
            "public": {"source": public_source.value},
            "database": ({"source": db_source.value} if database is not None and db_source else None),
        }
        component_status = (
            {component: "not_applicable" for component in ("input", "output", "cached_input", "cache_write", "long_context")}
            if is_image else _public_component_status(catalog_entry, public)
        )
        customer_values = {column: public.get(column) for column in (*PRICE_COLUMNS, *LONG_CONTEXT_COLUMNS, *IMAGE_COLUMNS)}
        customer_row = _scoped_row(
            public, scope="group_customer", values=customer_values,
            component_status=component_status,
            visibility="public", derivation=None, evidence=public_evidence,
            disabled=bool(public["disabled"]),
        )
        if component_status["long_context"] == "not_exposed":
            customer_row["component_evidence"]["long_context"] = (
                "backend/internal/service/model_pricing_service.go publicLongContextPricing "
                "deliberately emits runtime long_context only for OpenAI tier models"
            )
        customer_row["drift"] = [
            {key: value for key, value in item.items() if key in {"type", "column", "expected", "actual", "message"}}
            for item in row_issues
        ]
        matrix_rows.append(customer_row)

        # Missing long-context runtime pricing is an honest unknown when the
        # official/provider book has a long-context tier. It is not evidence
        # that the official tier is wrong.
        if not public["disabled"] and component_status["long_context"] == "unknown":
            problem = issue("unknown", model_id, public["group_id"], "long_context", "runtime tier", None,
                            "group_customer long-context pricing is not exposed by the public inventory",
                            ["catalog", "public"])
            row_issues.append(problem)
            issues.append(problem)
            customer_row["drift"].append({key: value for key, value in problem.items()
                                           if key in {"type", "column", "expected", "actual", "message"}})

        issues.extend(item for item in row_issues if item not in issues)
        if row_issues and customer_row["status"] == "verified":
            customer_row["status"] = "blocked"

        if not is_image:
            multiplier = public.get("rate_multiplier")
            base_values = {
                column: (round(float(public[column]) / float(multiplier), 8)
                         if public.get(column) is not None and multiplier else None)
                for column in PRICE_COLUMNS
            }
            base_values.update({column: public.get(column) for column in LONG_CONTEXT_COLUMNS})
            gateway_row = _scoped_row(
                public, scope="gateway_base", values=base_values,
                component_status=dict(component_status), visibility="public",
                derivation="group_customer / rate_multiplier",
                evidence={"catalog": None, "public": {"source": public_source.value}, "database": None},
                disabled=bool(public["disabled"]),
            )
            if component_status["long_context"] == "not_exposed":
                gateway_row["component_evidence"]["long_context"] = customer_row["component_evidence"]["long_context"]
            matrix_rows.append(gateway_row)

    # Only catalog rows explicitly intended for the public price surface need
    # a public runtime row. Hidden catalog-only models are valid provider facts.
    for model_id, catalog_entry in sorted(catalog.items()):
        if model_id in active_models:
            continue
        if model_id in disabled_models:
            continue
        if catalog_entry.get("public_price_visibility") == "hidden":
            continue
        missing = issue("missing_public", model_id, None, "model_id", model_id, None,
                        "catalog model has no active public pricing row", ["catalog", "public"])
        issues.append(missing)
        row = _empty_row()
        row.update({
            "model_id": model_id,
            "group_id": None,
            "group_name": catalog_entry.get("preferred_group_name"),
            "platform": catalog_entry.get("platform"),
            "rate_multiplier": None,
            "currency": public_meta.get("currency"),
            "unit": public_meta.get("unit"),
            "disabled": False,
            "price_scope": "group_customer",
            "component_status": {component: "blocked" for component in ("input", "output", "cached_input", "cache_write", "long_context")},
            "public_price_visibility": "public",
            "derivation": None,
            "status": "blocked",
            "evidence": {
                "catalog": {"source": catalog_source.value, "url": catalog_entry.get("evidence_url")},
                "public": None,
                "database": None,
            },
            "drift": [{key: value for key, value in missing.items()
                       if key in {"type", "column", "expected", "actual", "message"}}],
        })
        matrix_rows.append(row)

    if db_rows is not None:
        for key, database in sorted(db_by_key.items(), key=lambda item: item[0]):
            if key in public_by_key or database["disabled"]:
                continue
            missing = issue("missing_public", database["model_id"], database["group_id"], "row", "present", "missing",
                            "database export row is absent from public inventory", ["database", "public"])
            issues.append(missing)

    issue_order = {"input_error": 0, "conflict": 1, "blocked": 2, "unknown": 3,
                   "missing_public": 4, "missing_database": 5}
    issues.sort(key=lambda item: (
        issue_order.get(str(item["type"]), 99),
        str(item.get("model_id") or ""), str(item.get("group_id") or ""), str(item.get("column") or ""),
    ))
    matrix_rows.sort(key=lambda row: (
        str(row["model_id"]).lower(), PRICE_SCOPES.index(row["price_scope"]), str(row.get("group_id") or "")
    ))
    status_counts: dict[str, int] = {}
    for row in matrix_rows:
        status_counts[row["status"]] = status_counts.get(row["status"], 0) + 1
    drift_columns = sorted({str(item["column"]) for item in issues if item.get("column")})
    return {
        "schema_version": SCHEMA_VERSION,
        "kind": KIND,
        "mode": mode,
        "complete": not issues,
        "columns": list(MATRIX_COLUMNS),
        "sources": {
            "catalog": catalog_source.value,
            "public": public_source.value,
            "database_export": db_source.value if db_source else None,
        },
        "normalization": {
            "currency": public_meta.get("currency"),
            "unit": CANONICAL_UNIT,
            "catalog_currency": "USD",
            "catalog_to_public_basis": "not_compared_across_price_scopes",
            "gateway_base_derivation": "group_customer / rate_multiplier",
            "database": db_meta,
        },
        "summary": {
            "catalog_models": len(catalog),
            "public_rows": len(public_rows),
            "database_rows": len(db_rows) if db_rows is not None else None,
            "matrix_rows": len(matrix_rows),
            "issue_count": len(issues),
            "status_counts": dict(sorted(status_counts.items())),
            "drift_columns": drift_columns,
        },
        "rows": matrix_rows,
        "issues": issues,
    }


def input_error_report(mode: str, message: str, sources: dict[str, str | None]) -> dict[str, Any]:
    problem = issue("input_error", None, None, None, None, None, message, ["input"])
    return {
        "schema_version": SCHEMA_VERSION,
        "kind": KIND,
        "mode": mode,
        "complete": False,
        "columns": list(MATRIX_COLUMNS),
        "sources": sources,
        "normalization": {"unit": CANONICAL_UNIT},
        "summary": {
            "catalog_models": 0,
            "public_rows": 0,
            "database_rows": None,
            "matrix_rows": 0,
            "issue_count": 1,
            "status_counts": {},
            "drift_columns": [],
        },
        "rows": [],
        "issues": [problem],
    }


def _emit(report: dict[str, Any], output: Path | None) -> None:
    rendered = json.dumps(report, ensure_ascii=False, indent=2, sort_keys=False) + "\n"
    if output:
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(rendered, encoding="utf-8")
    sys.stdout.write(rendered)


def parse_args(argv: Iterable[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("plan", "audit"))
    parser.add_argument("--catalog", type=Path, default=Path("model-catalog/catalog.json"))
    public = parser.add_mutually_exclusive_group(required=True)
    public.add_argument("--inventory-json", type=Path)
    public.add_argument("--pricing-url")
    parser.add_argument("--db-export", type=Path,
                        help="optional JSON fixture exported from the database; never a DSN")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--tolerance", type=float, default=0.0001)
    args = parser.parse_args(argv)
    if not math.isfinite(args.tolerance) or args.tolerance < 0:
        parser.error("--tolerance must be a non-negative finite number")
    return args


def main(argv: Iterable[str] | None = None) -> int:
    args = parse_args(argv)
    catalog_source = Source("catalog", str(args.catalog))
    public_source = Source(
        "public",
        str(args.inventory_json) if args.inventory_json else str(args.pricing_url),
    )
    db_source = Source("database", str(args.db_export)) if args.db_export else None
    sources = {
        "catalog": catalog_source.value,
        "public": public_source.value,
        "database_export": db_source.value if db_source else None,
    }
    try:
        catalog_raw = _load_json_file(args.catalog)
        public_raw = (_load_json_file(args.inventory_json) if args.inventory_json
                      else _load_json_url(args.pricing_url))
        db_raw = _load_json_file(args.db_export) if args.db_export else None
        catalog, catalog_issues, price_gaps = load_catalog(catalog_raw, catalog_source)
        public_rows, public_meta = normalize_inventory(public_raw, public_source)
        if not catalog and not public_rows:
            raise MatrixInputError("catalog and public inventory are both empty; no price was verified")
        if db_raw is not None and db_source is not None:
            db_rows, db_meta = normalize_inventory(db_raw, db_source)
        else:
            db_rows, db_meta = None, None
        report = build_report(
            catalog, public_rows, public_meta, db_rows, db_meta, catalog_issues, price_gaps,
            mode=args.command,
            catalog_source=catalog_source,
            public_source=public_source,
            db_source=db_source,
            tolerance=args.tolerance,
        )
        _emit(report, args.output)
        return 2 if args.command == "audit" and not report["complete"] else 0
    except (OSError, UnicodeError, json.JSONDecodeError, MatrixInputError) as exc:
        report = input_error_report(args.command, str(exc), sources)
        _emit(report, args.output)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

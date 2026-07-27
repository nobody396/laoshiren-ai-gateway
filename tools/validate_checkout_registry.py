#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from datetime import datetime
from pathlib import Path


def validate(registry_path: Path, checkout_path: Path, action: str, now: datetime | None = None) -> dict:
    registry = json.loads(registry_path.read_text(encoding="utf-8"))
    checkout = checkout_path.resolve()
    entry = next((item for item in registry["checkouts"] if Path(item["path"]).resolve() == checkout), None)
    if entry is None:
        raise ValueError(f"unregistered checkout: {checkout}")
    if action not in entry["allowed_actions"]:
        raise ValueError(f"checkout kind {entry['kind']} forbids action {action}")
    if action == "release" and entry["kind"] not in {"canonical", "release-only"}:
        raise ValueError("release requires canonical or release-only checkout")
    if entry.get("ttl"):
        deadline = datetime.fromisoformat(entry["ttl"])
        current = now or datetime.now(deadline.tzinfo)
        if current > deadline:
            raise ValueError(f"checkout registration expired: {deadline.isoformat()}")
    return entry


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser()
    parser.add_argument("--registry", type=Path, default=root / "docs/ops/checkouts.json")
    parser.add_argument("--path", type=Path, default=root)
    parser.add_argument("--action", choices=["develop", "test", "report", "release"], required=True)
    args = parser.parse_args()
    try:
        entry = validate(args.registry, args.path, args.action)
    except (ValueError, KeyError, json.JSONDecodeError) as exc:
        print(f"checkout validation failed: {exc}")
        return 2
    print(f"checkout validation passed: kind={entry['kind']} action={args.action}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

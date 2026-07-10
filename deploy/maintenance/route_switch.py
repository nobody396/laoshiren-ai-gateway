#!/usr/bin/env python3
"""Atomically switch only Traefik backend URLs without rendering route secrets."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import tempfile


SERVICE_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$")


class RouteSwitchError(ValueError):
    """Raised when an atomic route switch cannot be proven safe."""


def validate_service(value: str) -> str:
    if not SERVICE_RE.fullmatch(value):
        raise RouteSwitchError("service name contains unsupported characters")
    return value


def validate_port(value: int) -> int:
    if value < 1 or value > 65535:
        raise RouteSwitchError("port must be between 1 and 65535")
    return value


def ensure_regular_file(path: Path, *, label: str) -> os.stat_result:
    try:
        metadata = path.lstat()
    except FileNotFoundError as exc:
        raise RouteSwitchError(f"{label} does not exist") from exc
    if stat.S_ISLNK(metadata.st_mode) or not stat.S_ISREG(metadata.st_mode):
        raise RouteSwitchError(f"{label} must be a regular non-symlink file")
    return metadata


def read_text(path: Path, *, label: str) -> str:
    ensure_regular_file(path, label=label)
    try:
        return path.read_text(encoding="utf-8")
    except UnicodeDecodeError as exc:
        raise RouteSwitchError(f"{label} must be valid UTF-8") from exc


def secure_create(path: Path, content: str) -> None:
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    descriptor = os.open(path, flags, 0o600)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", closefd=False) as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
    finally:
        os.close(descriptor)


def atomic_replace(path: Path, content: str) -> None:
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=path.parent
    )
    temporary = Path(temporary_name)
    try:
        os.fchmod(descriptor, 0o600)
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        directory_descriptor = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory_descriptor)
        finally:
            os.close(directory_descriptor)
    finally:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def content_hash(content: str) -> str:
    return hashlib.sha256(content.encode("utf-8")).hexdigest()


def backend_url(service: str, port: int) -> str:
    return f"http://{validate_service(service)}:{validate_port(port)}"


def switch_route(
    config_path: Path,
    backup_path: Path,
    from_service: str,
    to_service: str,
    *,
    port: int,
    expected_count: int,
) -> dict[str, object]:
    if config_path.parent.resolve() != backup_path.parent.resolve():
        raise RouteSwitchError("backup must stay beside the protected route config")
    if expected_count < 1:
        raise RouteSwitchError("expected count must be positive")

    source = backend_url(from_service, port)
    target = backend_url(to_service, port)
    if source == target:
        raise RouteSwitchError("source and target service must differ")

    current = read_text(config_path, label="route config")
    source_count = current.count(source)
    target_count = current.count(target)

    if source_count == 0 and target_count == expected_count:
        ensure_regular_file(backup_path, label="route backup")
        return {
            "status": "already_switched",
            "replacements": target_count,
            "sha256": content_hash(current),
        }
    if source_count != expected_count or target_count != 0:
        raise RouteSwitchError(
            "route backend occurrence count does not match the expected topology"
        )
    reused_backup = False
    if backup_path.exists() or backup_path.is_symlink():
        original = read_text(backup_path, label="route backup")
        if original != current:
            raise RouteSwitchError("existing route backup does not match the current app route")
        reused_backup = True
    else:
        secure_create(backup_path, current)
    updated = current.replace(source, target)
    atomic_replace(config_path, updated)
    return {
        "status": "switched_reusing_backup" if reused_backup else "switched",
        "replacements": expected_count,
        "sha256": content_hash(updated),
    }


def restore_route(
    config_path: Path,
    backup_path: Path,
    current_service: str,
    restore_service: str,
    *,
    port: int,
    expected_count: int,
) -> dict[str, object]:
    if config_path.parent.resolve() != backup_path.parent.resolve():
        raise RouteSwitchError("backup must stay beside the protected route config")
    if expected_count < 1:
        raise RouteSwitchError("expected count must be positive")

    current = read_text(config_path, label="route config")
    current_target = backend_url(current_service, port)
    original = read_text(backup_path, label="route backup")
    restore_target = backend_url(restore_service, port)
    if (
        original.count(restore_target) != expected_count
        or original.count(current_target) != 0
    ):
        raise RouteSwitchError("route backup does not match the expected restore topology")

    if current == original:
        return {
            "status": "already_restored",
            "replacements": expected_count,
            "sha256": content_hash(original),
        }
    if (
        current.count(current_target) != expected_count
        or current.count(restore_target) != 0
    ):
        raise RouteSwitchError("current route is not fully pointed at the expected service")

    atomic_replace(config_path, original)
    return {
        "status": "restored",
        "replacements": expected_count,
        "sha256": content_hash(original),
    }


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    switch = subparsers.add_parser("switch")
    switch.add_argument("--config", type=Path, required=True)
    switch.add_argument("--backup", type=Path, required=True)
    switch.add_argument("--from-service", required=True)
    switch.add_argument("--to-service", required=True)
    switch.add_argument("--port", type=int, default=8080)
    switch.add_argument("--expected-count", type=int, default=3)

    restore = subparsers.add_parser("restore")
    restore.add_argument("--config", type=Path, required=True)
    restore.add_argument("--backup", type=Path, required=True)
    restore.add_argument("--current-service", required=True)
    restore.add_argument("--restore-service", required=True)
    restore.add_argument("--port", type=int, default=8080)
    restore.add_argument("--expected-count", type=int, default=3)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.command == "switch":
            result = switch_route(
                args.config,
                args.backup,
                args.from_service,
                args.to_service,
                port=args.port,
                expected_count=args.expected_count,
            )
        else:
            result = restore_route(
                args.config,
                args.backup,
                args.current_service,
                args.restore_service,
                port=args.port,
                expected_count=args.expected_count,
            )
    except (OSError, RouteSwitchError) as exc:
        print(f"route switch refused: {exc}", file=sys.stderr)
        return 2

    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

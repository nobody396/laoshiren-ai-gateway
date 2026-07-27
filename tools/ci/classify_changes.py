#!/usr/bin/env python3
"""Classify changed paths into the smallest safe CI matrix.

Unknown and release-pipeline changes fail open to the full test matrix. The
classifier is an optimization only; it must never be a way to avoid a gate.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path


FULL_MATRIX_PREFIXES = (
    ".github/",
    "tools/ci/",
    "tools/release/",
)
FULL_MATRIX_FILES = {
    "Dockerfile",
    "Makefile",
    "go.work",
    "go.work.sum",
}
BACKEND_PREFIXES = ("backend/",)
FRONTEND_PREFIXES = ("frontend/",)
DOCKER_PREFIXES = ("backend/", "frontend/", "deploy/")
IGNORED_PREFIXES = ("docs/",)
IGNORED_FILES = {
    "AGENTS.md",
    "README.md",
    "LICENSE",
}


def classify(paths: list[str]) -> dict[str, bool]:
    normalized = sorted({path.strip().lstrip("./") for path in paths if path.strip()})
    result = {
        "backend": False,
        "integration": False,
        "frontend": False,
        "docker": False,
        "full": False,
    }
    if not normalized:
        return {key: True for key in result}

    for path in normalized:
        if path in FULL_MATRIX_FILES or path.startswith(FULL_MATRIX_PREFIXES):
            return {key: True for key in result}
        if path.startswith(BACKEND_PREFIXES):
            result["backend"] = True
            result["integration"] = True
        if path.startswith(FRONTEND_PREFIXES):
            result["frontend"] = True
        if path.startswith(DOCKER_PREFIXES):
            result["docker"] = True

        known = (
            path.startswith(BACKEND_PREFIXES)
            or path.startswith(FRONTEND_PREFIXES)
            or path.startswith("deploy/")
            or path.startswith(IGNORED_PREFIXES)
            or path in IGNORED_FILES
        )
        if not known:
            return {key: True for key in result}

    result["full"] = all(
        result[key] for key in ("backend", "integration", "frontend", "docker")
    )
    return result


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--paths-file", type=Path, required=True)
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args()

    paths = args.paths_file.read_text(encoding="utf-8").splitlines()
    result = classify(paths)
    print(json.dumps({"paths": paths, **result}, ensure_ascii=False, sort_keys=True))
    if args.github_output:
        with args.github_output.open("a", encoding="utf-8") as handle:
            for key, value in result.items():
                handle.write(f"{key}={'true' if value else 'false'}\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

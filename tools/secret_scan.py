#!/usr/bin/env python3
"""High-confidence repository secret scan. Findings never print matched values."""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RULES = {
    "private-key": re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),
    "provider-token": re.compile(r"(?:sk-(?:proj-)?|gh[oprsu]_)[A-Za-z0-9_-]{20,}"),
    "aws-access-key": re.compile(r"(?:AKIA|ASIA)[A-Z0-9]{16}"),
}
SKIP_PARTS = {".git", "node_modules", "dist", "coverage", "vendor"}


def repository_files() -> list[Path]:
    result = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
    )
    return [ROOT / item.decode() for item in result.stdout.split(b"\0") if item]


def scan(paths: list[Path]) -> list[tuple[Path, int, str]]:
    findings: list[tuple[Path, int, str]] = []
    for path in paths:
        if not path.is_file() or any(part in SKIP_PARTS for part in path.parts):
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        for line_number, line in enumerate(text.splitlines(), 1):
            for rule, pattern in RULES.items():
                match = pattern.search(line)
                if match and rule == "provider-token":
                    normalized = match.group(0).lower()
                    if path.name.endswith("_test.go") or any(marker in normalized for marker in ("test", "fixture", "example", "your", "xxxx")):
                        continue
                if match:
                    findings.append((path, line_number, rule))
    return findings


def main(argv: list[str]) -> int:
    paths = [Path(value).resolve() for value in argv] if argv else repository_files()
    findings = scan(paths)
    for path, line, rule in findings:
        try:
            display = path.relative_to(ROOT)
        except ValueError:
            display = Path("<external-fixture>")
        print(f"{display}:{line}: {rule}: [REDACTED]", file=sys.stderr)
    if findings:
        print(f"secret scan failed: {len(findings)} redacted finding(s)", file=sys.stderr)
        return 1
    print(f"secret scan passed: {len(paths)} file(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

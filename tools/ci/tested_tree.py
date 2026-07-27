#!/usr/bin/env python3
"""Record and resolve a successful PR tree attestation.

PR CI records the exact Git tree tested by all required jobs. Main CI may use
the fast path only when GitHub proves that the associated successful PR run
uploaded an attestation whose tested commit has the same tree as main.
Any missing, malformed, expired, or mismatched evidence returns a safe fallback
decision instead of skipping tests.
"""

from __future__ import annotations

import argparse
import io
import json
import os
from pathlib import Path
import subprocess
import urllib.error
import urllib.parse
import urllib.request
import zipfile


def git_value(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()


def record_attestation(
    *,
    repo: Path,
    repository: str,
    pr_number: int,
    head_sha: str,
    base_sha: str,
    run_id: int,
) -> dict:
    tested_commit = git_value(repo, "rev-parse", "HEAD")
    tested_tree = git_value(repo, "rev-parse", "HEAD^{tree}")
    return {
        "version": 1,
        "repository": repository,
        "pr_number": pr_number,
        "head_sha": head_sha,
        "base_sha": base_sha,
        "workflow_run_id": run_id,
        "tested_commit": tested_commit,
        "tested_tree": tested_tree,
    }


class GitHubAPI:
    def __init__(self, repository: str, token: str) -> None:
        self.repository = repository
        self.token = token
        self.root = f"https://api.github.com/repos/{repository}"

    def request_json(self, path: str) -> object:
        request = urllib.request.Request(
            self.root + path,
            headers={
                "Accept": "application/vnd.github+json",
                "Authorization": f"Bearer {self.token}",
                "X-GitHub-Api-Version": "2022-11-28",
                "User-Agent": "laoshirenai-tested-tree",
            },
        )
        with urllib.request.urlopen(request, timeout=20) as response:
            return json.load(response)

    def request_bytes(self, url: str) -> bytes:
        request = urllib.request.Request(
            url,
            headers={
                "Accept": "application/vnd.github+json",
                "Authorization": f"Bearer {self.token}",
                "X-GitHub-Api-Version": "2022-11-28",
                "User-Agent": "laoshirenai-tested-tree",
            },
        )
        with urllib.request.urlopen(request, timeout=30) as response:
            return response.read()


def fallback(reason: str) -> dict:
    return {"fast_path": False, "reason": reason}


def resolve_attestation(
    *,
    repo: Path,
    repository: str,
    commit_sha: str,
    api: GitHubAPI,
) -> dict:
    main_tree = git_value(repo, "rev-parse", f"{commit_sha}^{{tree}}")
    pulls = api.request_json(f"/commits/{commit_sha}/pulls")
    if not isinstance(pulls, list):
        return fallback("associated_pull_response_invalid")
    associated = [
        pull
        for pull in pulls
        if isinstance(pull, dict)
        and pull.get("merged_at")
        and pull.get("merge_commit_sha") == commit_sha
    ]
    if len(associated) != 1:
        return fallback("associated_merged_pull_not_unique")
    pull = associated[0]
    pr_number = int(pull["number"])
    head_sha = str(pull["head"]["sha"])

    query = urllib.parse.urlencode(
        {
            "event": "pull_request",
            "head_sha": head_sha,
            "status": "success",
            "per_page": "30",
        }
    )
    runs_response = api.request_json(f"/actions/workflows/ci.yml/runs?{query}")
    if not isinstance(runs_response, dict):
        return fallback("workflow_runs_response_invalid")
    workflow_runs = runs_response.get("workflow_runs")
    if not isinstance(workflow_runs, list):
        return fallback("workflow_runs_payload_invalid")
    candidates = []
    for run in workflow_runs:
        if not isinstance(run, dict) or run.get("conclusion") != "success":
            continue
        pull_requests = run.get("pull_requests")
        if not isinstance(pull_requests, list):
            continue
        numbers = {
            int(item["number"])
            for item in pull_requests
            if isinstance(item, dict) and "number" in item
        }
        if pr_number in numbers:
            candidates.append(run)
    if not candidates:
        return fallback("successful_pr_run_not_found")
    run = max(candidates, key=lambda item: int(item["id"]))
    run_id = int(run["id"])

    artifacts_response = api.request_json(f"/actions/runs/{run_id}/artifacts")
    if not isinstance(artifacts_response, dict):
        return fallback("artifacts_response_invalid")
    artifact_name = f"tested-tree-{head_sha}"
    artifacts_payload = artifacts_response.get("artifacts")
    if not isinstance(artifacts_payload, list):
        return fallback("artifacts_payload_invalid")
    artifacts = [
        item
        for item in artifacts_payload
        if isinstance(item, dict)
        and item.get("name") == artifact_name
        and not item.get("expired")
    ]
    if len(artifacts) != 1:
        return fallback("tested_tree_artifact_not_unique")

    archive = api.request_bytes(str(artifacts[0]["archive_download_url"]))
    with zipfile.ZipFile(io.BytesIO(archive)) as bundle:
        names = [name for name in bundle.namelist() if name.endswith("tested-tree.json")]
        if len(names) != 1:
            return fallback("tested_tree_payload_not_unique")
        payload = json.loads(bundle.read(names[0]))
    if not isinstance(payload, dict):
        return fallback("tested_tree_payload_invalid")

    expected = {
        "version": 1,
        "repository": repository,
        "pr_number": pr_number,
        "head_sha": head_sha,
        "workflow_run_id": run_id,
    }
    for key, value in expected.items():
        if payload.get(key) != value:
            return fallback(f"tested_tree_{key}_mismatch")

    tested_commit = str(payload.get("tested_commit", ""))
    tested_tree = str(payload.get("tested_tree", ""))
    if len(tested_commit) != 40 or len(tested_tree) != 40:
        return fallback("tested_tree_hash_invalid")
    tested_object = api.request_json(f"/git/commits/{tested_commit}")
    if not isinstance(tested_object, dict):
        return fallback("tested_commit_response_invalid")
    github_tree = str((tested_object.get("tree") or {}).get("sha", ""))
    if github_tree != tested_tree:
        return fallback("tested_commit_tree_mismatch")
    if tested_tree != main_tree:
        return fallback("main_tree_mismatch")
    return {
        "fast_path": True,
        "reason": "tested_tree_matches_main",
        "pr_number": pr_number,
        "pr_run_id": run_id,
        "tested_commit": tested_commit,
        "tested_tree": tested_tree,
        "main_commit": commit_sha,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    record = subparsers.add_parser("record")
    record.add_argument("--repo", type=Path, required=True)
    record.add_argument("--repository", required=True)
    record.add_argument("--pr-number", type=int, required=True)
    record.add_argument("--head-sha", required=True)
    record.add_argument("--base-sha", required=True)
    record.add_argument("--run-id", type=int, required=True)
    record.add_argument("--output", type=Path, required=True)

    resolve = subparsers.add_parser("resolve")
    resolve.add_argument("--repo", type=Path, required=True)
    resolve.add_argument("--repository", required=True)
    resolve.add_argument("--commit-sha", required=True)
    resolve.add_argument("--output", type=Path, required=True)

    args = parser.parse_args()
    if args.command == "record":
        result = record_attestation(
            repo=args.repo,
            repository=args.repository,
            pr_number=args.pr_number,
            head_sha=args.head_sha,
            base_sha=args.base_sha,
            run_id=args.run_id,
        )
    else:
        token = os.environ.get("GITHUB_TOKEN", "").strip()
        if not token:
            result = fallback("github_token_missing")
        else:
            try:
                result = resolve_attestation(
                    repo=args.repo,
                    repository=args.repository,
                    commit_sha=args.commit_sha,
                    api=GitHubAPI(args.repository, token),
                )
            except (
                OSError,
                AttributeError,
                TypeError,
                ValueError,
                KeyError,
                json.JSONDecodeError,
                urllib.error.URLError,
                zipfile.BadZipFile,
                subprocess.SubprocessError,
            ) as exc:
                result = fallback(f"resolution_error:{type(exc).__name__}")
    args.output.write_text(
        json.dumps(result, ensure_ascii=False, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

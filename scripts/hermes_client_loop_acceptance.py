#!/usr/bin/env python3
"""Run one authorized Hermes 0.20.0 Responses tool loop.

This harness is inert unless both live switches are present. The owned key is
passed only in the Hermes child environment, raw client output is never stored,
and the owned key is restored to group 6 in ``finally``.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile
import time
from typing import Any
import uuid


ROOT = Path(__file__).resolve().parents[1]
GROUP_PROBE = Path(
    "/Users/fujunhao/AgentWorkspace/skill-hub/own/laoshirenai-skills/skills/"
    "laoshirenai-account-ops/scripts/group_matrix_probe.py"
)
OWNED_KEY_ID = 128
OWNED_USER_ID = 2
RESTORE_GROUP_ID = 6
CLIENT_VERSION = "0.20.0"
BASE_URL = "https://api.laoshirenai.com/v1"


def now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def canonical_sha(value: dict[str, Any]) -> str:
    payload = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode()).hexdigest()


def file_sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def artifact_uri(path: Path) -> str:
    resolved = path.resolve()
    try:
        return str(resolved.relative_to(ROOT))
    except ValueError:
        return str(resolved)


def atomic_receipt(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    safe = dict(value)
    safe["artifact_sha256"] = canonical_sha(safe)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(safe, handle, ensure_ascii=False, indent=2)
            handle.write("\n")
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def load_controller() -> Any:
    spec = importlib.util.spec_from_file_location("hermes_loop_group_controller", GROUP_PROBE)
    if spec is None or spec.loader is None:
        raise RuntimeError("owned group controller is unavailable")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def unwrap_rows(value: Any) -> list[dict[str, Any]]:
    while isinstance(value, dict) and isinstance(value.get("data"), dict):
        value = value["data"]
    if isinstance(value, dict):
        value = value.get("items", value.get("list", []))
    return [item for item in value if isinstance(item, dict)] if isinstance(value, list) else []


def switch_group(controller: Any, group_id: int) -> dict[str, Any]:
    last_error: Exception | None = None
    for _ in range(5):
        try:
            current = controller.switch_group(group_id)
            if current.get("group_id") != group_id:
                raise RuntimeError("group switch readback mismatch")
            return current
        except Exception as error:  # pragma: no cover - retry branch is unit-tested through failure
            last_error = error
            time.sleep(1)
    raise RuntimeError(f"group switch failed for {group_id}") from last_error


SAFE_USAGE_FIELDS = (
    "id", "created_at", "user_id", "api_key_id", "group_id", "account_id",
    "requested_model", "model", "inbound_endpoint", "upstream_endpoint",
    "input_tokens", "output_tokens", "cache_read_input_tokens", "stream",
    "reasoning_effort", "user_agent", "request_id", "actual_cost",
)


def query_usage(controller: Any, *, model_id: str, group_id: int) -> tuple[str, list[dict[str, Any]]]:
    path = (
        f"/admin/usage?page=1&page_size=200&api_key_id={OWNED_KEY_ID}"
        f"&group_id={group_id}&model={model_id}&sort_by=id&sort_order=desc&exact_total=true"
    )
    return path, unwrap_rows(controller.admin_cli_json(["api", "GET", path]))


def accepted_usage_rows(
    candidates: list[dict[str, Any]], *, baseline_ids: set[Any], model_id: str, group_id: int,
) -> list[dict[str, Any]]:
    accepted = []
    for row in candidates:
        if row.get("id") in baseline_ids:
            continue
        if row.get("api_key_id") != OWNED_KEY_ID or row.get("group_id") != group_id:
            continue
        if row.get("model") != model_id and row.get("requested_model") != model_id:
            continue
        if row.get("inbound_endpoint") != "/v1/responses" or row.get("upstream_endpoint") != "/v1/responses":
            continue
        agent = str(row.get("user_agent", ""))
        if agent and not (agent.startswith("OpenAI/Python ") and "2.24.0" in agent):
            continue
        if not isinstance(row.get("input_tokens"), int) or not isinstance(row.get("output_tokens"), int):
            continue
        accepted.append({key: row.get(key) for key in SAFE_USAGE_FIELDS if key in row})
    return sorted(accepted, key=lambda item: str(item.get("created_at", "")))


def poll_usage(
    controller: Any, *, baseline_ids: set[Any], model_id: str, group_id: int,
    attempts: int = 15, delay: float = 2,
) -> tuple[str, list[dict[str, Any]]]:
    path = ""
    for _ in range(attempts):
        path, candidates = query_usage(controller, model_id=model_id, group_id=group_id)
        rows = accepted_usage_rows(
            candidates, baseline_ids=baseline_ids, model_id=model_id, group_id=group_id,
        )
        if rows:
            return path, rows
        time.sleep(delay)
    return path, []


def hermes_command(model_id: str, prompt: str) -> list[str]:
    return [
        "hermes", "chat", "-Q", "-q", prompt, "--provider", "openai-api",
        "--model", model_id, "--toolsets", "terminal", "--ignore-user-config",
        "--ignore-rules", "--max-turns", "10", "--yolo",
    ]


def child_environment(home: Path, key: str) -> dict[str, str]:
    env = os.environ.copy()
    env.update({
        "HOME": str(home),
        "OPENAI_API_KEY": key,
        "OPENAI_BASE_URL": BASE_URL,
        "NO_COLOR": "1",
    })
    return env


def execute_child(command: list[str], *, cwd: Path, env: dict[str, str], timeout: int) -> tuple[subprocess.CompletedProcess[bytes], bool, int]:
    started = time.monotonic()
    try:
        completed = subprocess.run(
            command, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            timeout=timeout, check=False,
        )
        timed_out = False
    except subprocess.TimeoutExpired as error:
        completed = subprocess.CompletedProcess(command, 124, error.stdout or b"", error.stderr or b"")
        timed_out = True
    return completed, timed_out, int((time.monotonic() - started) * 1000)


def live_gate(args: argparse.Namespace) -> None:
    if not args.execute or not args.acknowledge_paid_probes:
        raise SystemExit("live run requires --execute and --acknowledge-paid-probes")


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    result.add_argument("--model", required=True)
    result.add_argument("--group-id", type=int, required=True)
    result.add_argument("--output", type=Path, required=True)
    result.add_argument("--timeout", type=int, default=300)
    result.add_argument("--execute", action="store_true")
    result.add_argument("--acknowledge-paid-probes", action="store_true")
    return result


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    live_gate(args)  # Must run before controller/secret access.
    controller = load_controller()
    started_at = now()
    run_id = uuid.uuid4().hex[:12]
    input_marker = f"HERMES_MATRIX_INPUT_{run_id}"
    tool_marker = f"HERMES_MATRIX_TOOL_{run_id}"
    final_marker = f"HERMES_MATRIX_AGENT_OK_{run_id}"
    completed = subprocess.CompletedProcess([], 1, b"", b"")
    timed_out = False
    duration_ms = 0
    file_verified = final_verified = usage_verified = False
    usage_path = ""
    usage: list[dict[str, Any]] = []
    target_readback: dict[str, Any] | None = None
    restored_readback: dict[str, Any] | None = None
    run_error: str | None = None
    key = ""
    work_root: tempfile.TemporaryDirectory[str] | None = None
    safety_passed = False
    try:
        owned = controller.owned_key()
        if (
            owned.get("id") != OWNED_KEY_ID or owned.get("user_id") != OWNED_USER_ID
            or owned.get("group_id") != RESTORE_GROUP_ID or not owned.get("key")
        ):
            raise RuntimeError("owned key safety gate failed")
        safety_passed = True
        key = owned["key"]
        _, baseline = query_usage(controller, model_id=args.model, group_id=args.group_id)
        baseline_ids = {row.get("id") for row in baseline}
        target_readback = switch_group(controller, args.group_id)

        work_root = tempfile.TemporaryDirectory(prefix="hermes-matrix-acceptance-")
        root = Path(work_root.name)
        home, workspace = root / "home", root / "workspace"
        home.mkdir(); workspace.mkdir()
        (workspace / "fixture.txt").write_text(input_marker + "\n", encoding="utf-8")
        prompt = (
            "Use terminal tools to read fixture.txt. Then use a terminal tool to write exactly "
            f"{tool_marker} followed by a newline to result.txt and read result.txt back. "
            f"After the tool result, reply exactly {final_marker}."
        )
        command = hermes_command(args.model, prompt)
        env = child_environment(home, key)
        completed, timed_out, duration_ms = execute_child(
            command, cwd=workspace, env=env, timeout=args.timeout,
        )
        env["OPENAI_API_KEY"] = ""
        key = ""
        file_verified = (
            (workspace / "result.txt").is_file()
            and (workspace / "result.txt").read_text(encoding="utf-8").strip() == tool_marker
        )
        stdout_text = completed.stdout.decode("utf-8", errors="replace")
        final_verified = stdout_text.strip().endswith(final_marker)
        usage_path, usage = poll_usage(
            controller, baseline_ids=baseline_ids, model_id=args.model, group_id=args.group_id,
        )
        usage_verified = len(usage) >= 2
    except Exception as error:
        run_error = type(error).__name__
    finally:
        key = ""
        if safety_passed:
            try:
                restored_readback = switch_group(controller, RESTORE_GROUP_ID)
            except Exception as error:  # Receipt still records a failed restoration.
                restored_readback = {"group_id": None, "error_type": type(error).__name__}
        if work_root is not None:
            work_root.cleanup()

    stdout = completed.stdout if isinstance(completed.stdout, bytes) else b""
    stderr = completed.stderr if isinstance(completed.stderr, bytes) else b""
    restored = isinstance(restored_readback, dict) and restored_readback.get("group_id") == RESTORE_GROUP_ID
    tool_use_observed = file_verified
    tool_result_observed = file_verified and final_verified
    attribution_path = args.output.with_name(f"{args.output.stem}-usage-attribution{args.output.suffix}")
    attribution = {
        "schema_version": 1,
        "kind": "client_usage_attribution",
        "client_id": "hermes-agent",
        "client_version": CLIENT_VERSION,
        "user_id": OWNED_USER_ID,
        "api_key_id": OWNED_KEY_ID,
        "group_id": args.group_id,
        "observed_at": now(),
        "query": usage_path,
        "rows": usage,
        "secret_free": True,
    }
    atomic_receipt(attribution_path, attribution)
    passed = all((
        completed.returncode == 0, not timed_out, run_error is None,
        tool_use_observed, tool_result_observed, file_verified, final_verified,
        usage_verified, restored,
    ))
    receipt = {
        "schema_version": 1,
        "kind": "real_client_loop",
        "secret_free": True,
        "client_id": "hermes-agent",
        "client_version": CLIENT_VERSION,
        "os": "macos",
        "architecture": platform.machine(),
        "model_id": args.model,
        "protocol": "responses",
        "base_url": BASE_URL,
        "group_id": args.group_id,
        "owned_identity": {"user_id": OWNED_USER_ID, "key_id": OWNED_KEY_ID},
        "observed_at": started_at,
        "finished_at": now(),
        "duration_ms": duration_ms,
        "command_contract": {
            "executable": "hermes",
            "provider": "openai-api",
            "toolsets": "terminal",
            "max_turns": 10,
            "isolated_home": True,
        },
        "target_group_readback": (
            {"group_id": target_readback.get("group_id")} if isinstance(target_readback, dict) else None
        ),
        "exit_code": completed.returncode,
        "timed_out": timed_out,
        "run_error_type": run_error,
        "file_marker_verified": file_verified,
        "final_marker_verified": final_verified,
        "tool_use_observed": tool_use_observed,
        "tool_result_observed": tool_result_observed,
        "tool_result_continuation_verified": file_verified and final_verified,
        "stdout_bytes": len(stdout),
        "stdout_sha256": hashlib.sha256(stdout).hexdigest(),
        "stderr_bytes": len(stderr),
        "stderr_sha256": hashlib.sha256(stderr).hexdigest(),
        "protocol_attribution": {
            "artifact_uri": artifact_uri(attribution_path),
            "artifact_sha256": file_sha(attribution_path),
            "usage_rows": len(usage),
            "inbound_endpoint": "/v1/responses",
            "upstream_endpoint": "/v1/responses",
            "verified": usage_verified,
        },
        "restoration": {
            "group_id": restored_readback.get("group_id") if isinstance(restored_readback, dict) else None,
            "verified": restored,
        },
        "restored_group_id": restored_readback.get("group_id") if isinstance(restored_readback, dict) else None,
        "passed": passed,
    }
    atomic_receipt(args.output, receipt)
    print(json.dumps({
        "output": str(args.output), "passed": passed, "exit_code": completed.returncode,
        "file_marker": file_verified, "final_marker": final_verified,
        "usage_rows": len(usage), "restored_group_id": receipt["restoration"]["group_id"],
    }, ensure_ascii=False))
    return 0 if passed else 2


if __name__ == "__main__":
    raise SystemExit(main())

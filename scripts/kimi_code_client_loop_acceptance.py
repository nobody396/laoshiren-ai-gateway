#!/usr/bin/env python3
"""Run exact Kimi Code 0.38.0 file-tool loops with an owned matrix key.

This harness is live-disabled by default.  A live run requires both explicit
switches, isolates ``HOME``/``KIMI_CODE_HOME``, passes the API key only through
``KIMI_MODEL_API_KEY``, verifies usage attribution, writes secret-free atomic
receipts, and restores the owned key to group 6 in ``finally``.

The Kimi CLI configuration tree is scanned before and after every run.  If the
client resolves the environment value into a plaintext file, the case is
refused with a credential-policy blocker and the temporary tree is destroyed.
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
import re
import shutil
import stat
import subprocess
import tempfile
import time
from typing import Any, Callable, Iterable, Sequence


ROOT = Path(__file__).resolve().parents[1]
GROUP_PROBE = Path(
    "/Users/fujunhao/AgentWorkspace/skill-hub/own/laoshirenai-skills/skills/"
    "laoshirenai-account-ops/scripts/group_matrix_probe.py"
)
OWNED_KEY_ID = 128
OWNED_USER_ID = 2
OWNED_KEY_NAME = "一键安装 · Codex"
RESTORE_GROUP_ID = 6
CLIENT_VERSION = "0.38.0"
BASE_URL = "https://api.laoshirenai.com"
PROTOCOLS = ("responses", "chat_completions", "messages", "generate_content")
PROTOCOL_CONFIG = {
    "responses": {
        "provider_type": "openai_responses",
        "base_url": f"{BASE_URL}/v1",
        "endpoint": "/v1/responses",
    },
    "chat_completions": {
        "provider_type": "openai",
        "base_url": f"{BASE_URL}/v1",
        "endpoint": "/v1/chat/completions",
    },
    "messages": {
        "provider_type": "anthropic",
        "base_url": BASE_URL,
        "endpoint": "/v1/messages",
    },
    "generate_content": {
        "provider_type": "google-genai",
        "base_url": BASE_URL,
        "endpoint": "/v1beta/models",
    },
}
POLICY_BLOCKER = (
    "Kimi Code persisted the environment-only API key in plaintext under its "
    "temporary configuration tree. The live case is refused by Agent Switch "
    "credential policy; the temporary tree was destroyed."
)
STREAM_POLICY_BLOCKER = (
    "Kimi Code echoed the environment-only API key into process output. The "
    "live case is refused by Agent Switch credential policy; stored hashes use "
    "redacted bytes and no raw output is retained."
)
SAFE_ID = re.compile(r"[^A-Za-z0-9._-]+")
MODEL_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]*$")
CREDENTIAL_ENV = re.compile(
    r"(?:API[_-]?KEY|TOKEN|SECRET|PASSWORD|AUTHORIZATION|COOKIE|CREDENTIAL)", re.I,
)
SECRET_PATTERNS = (
    re.compile(r"\bBearer\s+[A-Za-z0-9._~+/-]{8,}", re.I),
    re.compile(r"\b(?:sk|rk|pk)-[A-Za-z0-9_-]{8,}\b", re.I),
    re.compile(r"AIza[A-Za-z0-9_-]{12,}"),
)


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def canonical_sha(value: dict[str, Any]) -> str:
    payload = json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":"),
    ).encode()
    return hashlib.sha256(payload).hexdigest()


def assert_secret_free(value: Any, label: str) -> None:
    rendered = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False)
    if any(pattern.search(rendered) for pattern in SECRET_PATTERNS):
        raise RuntimeError(f"secret-shaped value found in {label}")


def atomic_json(path: Path, value: dict[str, Any]) -> None:
    """Write one canonical receipt atomically with mode 0600."""

    path.parent.mkdir(parents=True, exist_ok=True)
    payload = dict(value)
    payload.pop("artifact_sha256", None)
    assert_secret_free(payload, str(path))
    payload["artifact_sha256"] = canonical_sha(payload)
    fd, temporary = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=path.parent,
    )
    try:
        os.fchmod(fd, stat.S_IRUSR | stat.S_IWUSR)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(payload, handle, ensure_ascii=False, indent=2)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)
        directory_fd = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def load_controller() -> Any:
    spec = importlib.util.spec_from_file_location(
        "kimi_loop_group_controller", GROUP_PROBE,
    )
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
        except Exception as error:  # pragma: no cover - retry branch is deterministic in tests
            last_error = error
            time.sleep(1)
    raise RuntimeError(f"group switch failed for {group_id}") from last_error


def parse_target(raw: str) -> tuple[str, int, str]:
    model_and_group, separator, protocol = raw.rpartition(":")
    model_id, separator2, group_raw = model_and_group.rpartition(":")
    if (
        not separator or not separator2 or not MODEL_ID.fullmatch(model_id)
        or not group_raw.isdigit() or int(group_raw) <= 0
        or protocol not in PROTOCOLS
    ):
        raise ValueError(
            f"invalid target {raw!r}; expected MODEL_ID:GROUP_ID:"
            "responses|chat_completions|messages|generate_content"
        )
    return model_id, int(group_raw), protocol


def executable_version(
    runner: Callable[..., subprocess.CompletedProcess[bytes]] = subprocess.run,
) -> str:
    completed = runner(
        ["kimi", "--version"], stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        timeout=30, check=False,
    )
    output = completed.stdout.decode("utf-8", errors="replace").strip()
    if completed.returncode != 0 or output != CLIENT_VERSION:
        raise RuntimeError(
            f"Kimi Code version gate requires {CLIENT_VERSION}; "
            f"observed exit={completed.returncode} version={output or '<empty>'}"
        )
    return output


def write_ephemeral_config(home: Path) -> Path:
    config_dir = home / ".kimi-code"
    config_dir.mkdir(parents=True, mode=0o700)
    os.chmod(config_dir, 0o700)
    config = config_dir / "config.toml"
    config.write_text(
        "# Ephemeral acceptance configuration.\n"
        "# Model, protocol, Base URL, and credential are environment-only.\n",
        encoding="utf-8",
    )
    os.chmod(config, 0o600)
    return config


def runtime_environment(
    *, home: Path, key: str, model_id: str, protocol: str,
    context_size: int, max_output_tokens: int,
    source: dict[str, str] | None = None,
) -> dict[str, str]:
    if protocol not in PROTOCOL_CONFIG:
        raise ValueError(f"unsupported protocol: {protocol}")
    config = PROTOCOL_CONFIG[protocol]
    source = source or os.environ
    env = {
        name: value for name, value in source.items()
        if not CREDENTIAL_ENV.search(name)
    }
    env.update({
        "HOME": str(home),
        "KIMI_CODE_HOME": str(home / ".kimi-code"),
        "KIMI_MODEL_NAME": model_id,
        "KIMI_MODEL_API_KEY": key,
        "KIMI_MODEL_PROVIDER_TYPE": config["provider_type"],
        "KIMI_MODEL_BASE_URL": config["base_url"],
        "KIMI_MODEL_MAX_CONTEXT_SIZE": str(context_size),
        "KIMI_MODEL_MAX_OUTPUT_SIZE": str(max_output_tokens),
        "KIMI_MODEL_CAPABILITIES": "thinking,tool_use",
        "NO_COLOR": "1",
    })
    return env


def secret_file_matches(root: Path, secret: str) -> list[dict[str, Any]]:
    """Return metadata only for files containing the exact secret value."""

    if not secret:
        raise ValueError("secret must not be empty")
    needle = secret.encode()
    matches: list[dict[str, Any]] = []
    if not root.exists():
        return matches
    for path in root.rglob("*"):
        if not path.is_file() or path.is_symlink():
            continue
        try:
            data = path.read_bytes()
        except OSError:
            continue
        if needle not in data:
            continue
        matches.append({
            "path": str(path.relative_to(root)),
            "mode": oct(stat.S_IMODE(path.stat().st_mode)),
            "bytes": len(data),
        })
    return matches


def walk_objects(value: Any) -> Iterable[dict[str, Any]]:
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from walk_objects(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk_objects(child)


def parse_events(raw: bytes, *, file_marker: str, final_marker: str) -> dict[str, Any]:
    events: list[dict[str, Any]] = []
    counts: dict[str, int] = {}
    tool_use = tool_result = False
    non_user_rendered: list[str] = []
    assistant_fragments: list[str] = []
    tool_names: set[str] = set()
    for line in raw.decode("utf-8", errors="replace").splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(event, dict):
            continue
        events.append(event)
        event_type = str(event.get("type") or event.get("event") or "unknown").lower()
        counts[event_type] = counts.get(event_type, 0) + 1
        for item in walk_objects(event):
            role = str(item.get("role") or "").lower()
            item_type = str(item.get("type") or item.get("kind") or "").lower()
            step_type = str(item.get("step_type") or "").lower()
            state = str(item.get("state") or item.get("status") or "").lower()
            if role != "user":
                non_user_rendered.append(json.dumps(item, ensure_ascii=False))
            candidate = item.get("tool_name") or item.get("name")
            if isinstance(candidate, str):
                tool_names.add(candidate)
            if (
                "tool_call" in item_type or "tool_use" in item_type
                or "function_call" in item_type or step_type == "tool"
            ):
                tool_use = True
            if (
                "tool_result" in item_type or "tool_response" in item_type
                or "function_response" in item_type
                or (step_type == "tool" and state in {"done", "completed", "success"})
            ):
                tool_result = True
            for key in ("toolCall", "tool_call", "functionCall", "function_call"):
                if isinstance(item.get(key), dict):
                    tool_use = True
            for key in ("toolResult", "tool_result", "functionResponse", "function_response"):
                if item.get(key) is not None:
                    tool_result = True
            if role == "assistant" or item_type in {"assistant", "final", "result"}:
                for key in ("text", "content", "result", "response"):
                    value = item.get(key)
                    if isinstance(value, str):
                        assistant_fragments.append(value)
    non_user_text = "\n".join(non_user_rendered)
    assistant_text = "\n".join(assistant_fragments)
    return {
        "event_count": len(events),
        "event_types": counts,
        "tool_names": sorted(tool_names),
        "tool_use_observed": tool_use,
        "tool_result_observed": tool_result,
        "file_marker_observed_after_tool": file_marker in non_user_text,
        "final_marker_verified": final_marker in assistant_text,
        "assistant_text_sha256": hashlib.sha256(assistant_text.encode()).hexdigest(),
        "assistant_text_bytes": len(assistant_text.encode()),
    }


def safe_usage_row(row: dict[str, Any]) -> dict[str, Any]:
    fields = (
        "id", "created_at", "user_id", "api_key_id", "group_id", "account_id",
        "requested_model", "model", "inbound_endpoint", "upstream_endpoint",
        "input_tokens", "output_tokens", "cache_read_input_tokens",
        "cache_creation_input_tokens", "reasoning_effort", "duration_ms",
        "first_token_ms", "stream", "request_id", "user_agent", "total_cost",
        "actual_cost", "billing_type",
    )
    return {field: row.get(field) for field in fields if field in row}


def usage_snapshot(controller: Any, *, model_id: str, group_id: int) -> tuple[str, list[dict[str, Any]]]:
    path = (
        f"/admin/usage?page=1&page_size=200&api_key_id={OWNED_KEY_ID}"
        f"&group_id={group_id}&model={model_id}&sort_by=id&sort_order=desc"
        "&exact_total=true"
    )
    return path, unwrap_rows(controller.admin_cli_json(["api", "GET", path]))


def attributed_usage(
    controller: Any, *, model_id: str, group_id: int, protocol: str,
    before_ids: set[Any], attempts: int = 12, sleep: Callable[[float], None] = time.sleep,
) -> tuple[str, list[dict[str, Any]], bool]:
    expected = PROTOCOL_CONFIG[protocol]["endpoint"]
    path = ""
    selected: list[dict[str, Any]] = []
    for _ in range(attempts):
        path, rows = usage_snapshot(controller, model_id=model_id, group_id=group_id)
        selected = [
            safe_usage_row(row) for row in rows
            if row.get("id") not in before_ids
            and row.get("api_key_id") == OWNED_KEY_ID
            and row.get("group_id") == group_id
            and (row.get("model") == model_id or row.get("requested_model") == model_id)
            and row.get("inbound_endpoint") == expected
            and row.get("upstream_endpoint") == expected
            and "kimi" in str(row.get("user_agent") or "").lower()
        ]
        if len(selected) >= 2:
            break
        sleep(1)
    verified = len(selected) >= 2 and all(
        isinstance(row.get("input_tokens"), int)
        and isinstance(row.get("output_tokens"), int)
        for row in selected
    )
    return path, sorted(selected, key=lambda row: str(row.get("created_at") or "")), verified


def run_case(
    controller: Any, key: str, *, model_id: str, group_id: int, protocol: str,
    timeout: int, context_size: int, max_output_tokens: int,
    runner: Callable[..., subprocess.CompletedProcess[bytes]] = subprocess.run,
) -> tuple[dict[str, Any], bool]:
    switched = switch_group(controller, group_id)
    discovery = controller.discover(BASE_URL, key, 30)
    if model_id not in discovery.get("model_ids", []):
        raise RuntimeError(f"{model_id} missing after group {group_id} readback")
    _, before = usage_snapshot(controller, model_id=model_id, group_id=group_id)
    before_ids = {row.get("id") for row in before}
    started_at = utc_now()
    token = hashlib.sha256(f"{model_id}:{protocol}:{started_at}".encode()).hexdigest()[:12]
    file_marker = f"KIMI_MATRIX_INPUT_{token}"
    shell_marker = f"KIMI_MATRIX_FILE_{token}"
    final_marker = f"KIMI_MATRIX_AGENT_OK_{token}"
    temporary_root = Path(tempfile.mkdtemp(prefix="kimi-code-matrix-loop-"))
    home, workspace, empty_skills = (
        temporary_root / "home", temporary_root / "workspace", temporary_root / "empty-skills",
    )
    home.mkdir(); workspace.mkdir(); empty_skills.mkdir()
    config = write_ephemeral_config(home)
    (workspace / "fixture.txt").write_text(file_marker + "\n", encoding="utf-8")
    initial_matches = secret_file_matches(temporary_root, key)
    if initial_matches:
        shutil.rmtree(temporary_root, ignore_errors=True)
        raise RuntimeError("credential policy preflight failed before client launch")
    env = runtime_environment(
        home=home, key=key, model_id=model_id, protocol=protocol,
        context_size=context_size, max_output_tokens=max_output_tokens,
    )
    prompt = (
        "Use a file-reading tool to read fixture.txt. Then use a shell tool to write exactly "
        f"{shell_marker} followed by a newline to result.txt. Read result.txt with a file tool. "
        f"After the tool results, reply exactly {final_marker} and nothing else."
    )
    command = [
        "kimi", "-m", model_id, "-p", prompt,
        "--output-format", "stream-json", "--skills-dir", str(empty_skills),
    ]
    begin = time.monotonic()
    timed_out = False
    try:
        try:
            completed = runner(
                command, cwd=workspace, env=env, stdout=subprocess.PIPE,
                stderr=subprocess.PIPE, timeout=timeout, check=False,
            )
        except subprocess.TimeoutExpired as error:
            completed = subprocess.CompletedProcess(
                command, 124, error.stdout or b"", error.stderr or b"",
            )
            timed_out = True
        stdout = completed.stdout if isinstance(completed.stdout, bytes) else str(completed.stdout or "").encode()
        stderr = completed.stderr if isinstance(completed.stderr, bytes) else str(completed.stderr or "").encode()
        secret_bytes = key.encode()
        stream_secret_detected = secret_bytes in stdout or secret_bytes in stderr
        safe_stdout = stdout.replace(secret_bytes, b"[REDACTED]")
        safe_stderr = stderr.replace(secret_bytes, b"[REDACTED]")
        parsed = parse_events(stdout, file_marker=file_marker, final_marker=final_marker)
        shell_file_verified = (
            (workspace / "result.txt").is_file()
            and (workspace / "result.txt").read_text(encoding="utf-8").strip() == shell_marker
        )
        persisted = secret_file_matches(temporary_root, key)
        policy_blocker = (
            POLICY_BLOCKER if persisted else
            STREAM_POLICY_BLOCKER if stream_secret_detected else None
        )
        usage_path, usage, attribution_verified = attributed_usage(
            controller, model_id=model_id, group_id=group_id, protocol=protocol,
            before_ids=before_ids,
        )
        passed = all((
            completed.returncode == 0, not timed_out, not policy_blocker,
            parsed["tool_use_observed"], parsed["tool_result_observed"],
            parsed["file_marker_observed_after_tool"], shell_file_verified,
            parsed["final_marker_verified"], attribution_verified,
        ))
        receipt = {
            "schema_version": 1,
            "kind": "real_client_loop",
            "secret_free": True,
            "network_execution": "explicit_live",
            "client_id": "kimi-code",
            "client_version": f"cli:{CLIENT_VERSION}",
            "os": "macos",
            "architecture": platform.machine(),
            "model_id": model_id,
            "protocol": protocol,
            "base_url": PROTOCOL_CONFIG[protocol]["base_url"],
            "group_id": group_id,
            "owned_key": {
                "user_id": OWNED_USER_ID,
                "key_id": OWNED_KEY_ID,
                "name": OWNED_KEY_NAME,
            },
            "observed_at": started_at,
            "finished_at": utc_now(),
            "duration_ms": int((time.monotonic() - begin) * 1000),
            "command_contract": {
                "executable": "kimi",
                "version": CLIENT_VERSION,
                "provider_type": PROTOCOL_CONFIG[protocol]["provider_type"],
                "credential": "KIMI_MODEL_API_KEY environment only",
                "temporary_home": True,
                "automatic_permissions": True,
                "output_format": "stream-json",
            },
            "group_readback": {
                key: switched.get(key) for key in ("id", "user_id", "status", "group_id")
                if key in switched
            },
            "model_discovery": {
                key: discovery.get(key) for key in ("http_status", "model_count", "model_ids")
                if key in discovery
            },
            "exit_code": completed.returncode,
            "timed_out": timed_out,
            **parsed,
            "shell_file_verified": shell_file_verified,
            "stdout_sha256": hashlib.sha256(safe_stdout).hexdigest(),
            "stdout_bytes": len(stdout),
            "stderr_sha256": hashlib.sha256(safe_stderr).hexdigest(),
            "stderr_bytes": len(stderr),
            "credential_policy": {
                "key_transport": "environment_only",
                "config_path": str(config.relative_to(temporary_root)),
                "plaintext_persistence_detected": bool(persisted),
                "plaintext_stream_echo_detected": stream_secret_detected,
                "detected_files": persisted,
                "blocker": policy_blocker,
            },
            "usage_attribution": {
                "path": usage_path,
                "expected_endpoint": PROTOCOL_CONFIG[protocol]["endpoint"],
                "rows": usage,
                "row_count": len(usage),
                "verified": attribution_verified,
            },
            "restored_group_id": None,
            "passed": passed,
        }
        return receipt, bool(policy_blocker)
    finally:
        env["KIMI_MODEL_API_KEY"] = ""
        shutil.rmtree(temporary_root, ignore_errors=True)


def receipt_path(output_dir: Path, model_id: str, group_id: int, protocol: str) -> Path:
    safe_model = SAFE_ID.sub("-", model_id).strip("-")
    return output_dir / f"kimi-code-loop-{safe_model}-{protocol}-group{group_id}-20260901.json"


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--target", action="append", required=True,
        help="MODEL_ID:GROUP_ID:PROTOCOL; repeatable",
    )
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--timeout", type=int, default=240)
    parser.add_argument("--context-size", type=int, default=262144)
    parser.add_argument("--max-output-tokens", type=int, default=131072)
    parser.add_argument("--execute", action="store_true")
    parser.add_argument("--acknowledge-paid-probes", action="store_true")
    args = parser.parse_args(argv)
    if not args.execute or not args.acknowledge_paid_probes:
        raise SystemExit("live run requires --execute and --acknowledge-paid-probes")
    if args.timeout <= 0 or args.context_size <= 0 or args.max_output_tokens <= 0:
        raise SystemExit("timeout and token limits must be positive")
    try:
        targets = [parse_target(raw) for raw in args.target]
    except ValueError as error:
        raise SystemExit(str(error)) from error
    if len(targets) != len(set(targets)):
        raise SystemExit("duplicate --target values are not allowed")

    executable_version()
    controller = load_controller()
    current = controller.owned_key()
    if (
        current.get("id") != OWNED_KEY_ID
        or current.get("user_id") != OWNED_USER_ID
        or current.get("group_id") != RESTORE_GROUP_ID
        or not current.get("key")
    ):
        raise RuntimeError("owned key safety gate failed")
    key = str(current["key"])
    pending: list[tuple[Path, dict[str, Any]]] = []
    all_passed = True
    policy_blocked = False
    restored: dict[str, Any] | None = None
    restoration_error: str | None = None
    try:
        for model_id, group_id, protocol in targets:
            path = receipt_path(args.output_dir, model_id, group_id, protocol)
            try:
                receipt, case_policy_blocked = run_case(
                    controller, key, model_id=model_id, group_id=group_id,
                    protocol=protocol, timeout=args.timeout,
                    context_size=args.context_size,
                    max_output_tokens=args.max_output_tokens,
                )
            except Exception as error:
                safe_error = f"{type(error).__name__}: {error}".replace(
                    str(current.get("key") or ""), "[REDACTED]",
                )
                receipt = {
                    "schema_version": 1,
                    "kind": "real_client_loop",
                    "secret_free": True,
                    "network_execution": "explicit_live",
                    "client_id": "kimi-code",
                    "client_version": f"cli:{CLIENT_VERSION}",
                    "model_id": model_id,
                    "protocol": protocol,
                    "group_id": group_id,
                    "observed_at": utc_now(),
                    "error": safe_error,
                    "restored_group_id": None,
                    "passed": False,
                }
                case_policy_blocked = POLICY_BLOCKER in str(error)
            pending.append((path, receipt))
            all_passed = bool(receipt.get("passed")) and all_passed
            policy_blocked = policy_blocked or case_policy_blocked
            if case_policy_blocked:
                break
    finally:
        key = ""
        current["key"] = ""
        try:
            restored = switch_group(controller, RESTORE_GROUP_ID)
        except Exception as error:
            restoration_error = f"{type(error).__name__}: {error}"

    restored_group = restored.get("group_id") if isinstance(restored, dict) else None
    if restored_group != RESTORE_GROUP_ID:
        all_passed = False
    for path, receipt in pending:
        receipt["restored_group_id"] = restored_group
        receipt["restoration_error"] = restoration_error
        if restoration_error:
            receipt["passed"] = False
        atomic_json(path, receipt)
        print(json.dumps({
            "model_id": receipt.get("model_id"),
            "protocol": receipt.get("protocol"),
            "group_id": receipt.get("group_id"),
            "passed": receipt.get("passed"),
            "policy_blocked": bool(
                receipt.get("credential_policy", {}).get("blocker")
                if isinstance(receipt.get("credential_policy"), dict) else False
            ),
            "artifact": str(path),
        }, ensure_ascii=False))
    if policy_blocked:
        return 3
    return 0 if all_passed else 2


if __name__ == "__main__":
    raise SystemExit(main())

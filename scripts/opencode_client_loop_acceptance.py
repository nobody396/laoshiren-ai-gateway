#!/usr/bin/env python3
"""Run exact OpenCode 1.18.15 tool loops with the owned matrix key.

This command is live-only and requires both ``--execute`` and
``--acknowledge-paid-probes``.  The owned key is retrieved through the shared
Agent Switch controller, passed only in the child process environment, and is
never written to OpenCode configuration or any receipt.  Every target uses an
isolated temporary HOME/XDG config and the owned key is restored to group 6 in
``finally``.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tempfile
import time
from typing import Any
import urllib.parse

sys.path.insert(0, str(Path(__file__).resolve().parent))

from claude_code_client_loop_acceptance import (  # noqa: E402
    OWNED_KEY_ID,
    OWNED_KEY_NAME,
    OWNED_USER_ID,
    RESTORE_GROUP_ID,
    file_sha,
    load_controller,
    rows,
    switch_group,
)


ROOT = Path(__file__).resolve().parents[1]
CLIENT_VERSION = "1.18.15"
CLIENT_VERSION_KEY = f"cli:{CLIENT_VERSION}"
KEY_ENV = "LAOSHIRENAI_OPENCODE_KEY"
PROTOCOLS = {"responses", "chat_completions", "messages", "generate_content"}
PROVIDER_PACKAGES = {
    "responses": "@ai-sdk/openai",
    "chat_completions": "@ai-sdk/openai-compatible",
    "messages": "@ai-sdk/anthropic",
    "generate_content": "@ai-sdk/google",
}
BASE_URLS = {
    "responses": "https://api.laoshirenai.com/v1",
    "chat_completions": "https://api.laoshirenai.com/v1",
    "messages": "https://api.laoshirenai.com/v1",
    "generate_content": "https://api.laoshirenai.com/v1beta",
}
ENDPOINTS = {
    "responses": "/v1/responses",
    "chat_completions": "/v1/chat/completions",
    "messages": "/v1/messages",
    "generate_content": "/v1beta/models",
}
KNOWN_UPSTREAM_ENDPOINTS = set(ENDPOINTS.values())
SECRET_PATTERNS = (
    re.compile(r"\bBearer\s+[A-Za-z0-9._~+/-]{8,}", re.I),
    re.compile(r"\b(?:sk|rk|pk)-[A-Za-z0-9_-]{8,}\b", re.I),
    re.compile(r"AIza[A-Za-z0-9_-]{12,}"),
)
SAFE_MODEL = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]*$")
CREDENTIAL_ENV = re.compile(r"(?:API[_-]?KEY|TOKEN|SECRET|PASSWORD|AUTHORIZATION|COOKIE)", re.I)


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def canonical_sha(value: dict[str, Any]) -> str:
    payload = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(payload).hexdigest()


def assert_secret_free(value: Any, label: str) -> None:
    rendered = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False)
    if any(pattern.search(rendered) for pattern in SECRET_PATTERNS):
        raise RuntimeError(f"secret-shaped value found in {label}")


def atomic_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    body = dict(value)
    body.pop("artifact_sha256", None)
    assert_secret_free(body, str(path))
    body["artifact_sha256"] = canonical_sha(body)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(body, handle, ensure_ascii=False, indent=2, sort_keys=True)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def provider_id(protocol: str) -> str:
    return "laoshirenai-" + protocol.replace("_", "-")


def build_config(model_id: str, protocol: str) -> dict[str, Any]:
    if protocol not in PROTOCOLS:
        raise ValueError(f"unsupported protocol: {protocol}")
    return {
        "$schema": "https://opencode.ai/config.json",
        "provider": {
            provider_id(protocol): {
                "npm": PROVIDER_PACKAGES[protocol],
                "name": f"LaoshirenAI {protocol}",
                "options": {
                    "baseURL": BASE_URLS[protocol],
                    # OpenCode resolves this placeholder from the child env.
                    # The literal key must never appear on disk.
                    "apiKey": "{env:" + KEY_ENV + "}",
                },
                "models": {
                    model_id: {
                        "name": model_id,
                    }
                },
            }
        },
    }


def write_config(home: Path, model_id: str, protocol: str) -> Path:
    config = build_config(model_id, protocol)
    path = home / ".config" / "opencode" / "opencode.json"
    path.parent.mkdir(parents=True, exist_ok=True)
    rendered = json.dumps(config, ensure_ascii=False, indent=2) + "\n"
    assert_secret_free(rendered, "temporary OpenCode config")
    path.write_text(rendered, encoding="utf-8")
    return path


def child_env(home: Path, key: str, source: dict[str, str] | None = None) -> dict[str, str]:
    source = source or os.environ
    env = {
        name: value for name, value in source.items()
        if not CREDENTIAL_ENV.search(name)
    }
    env.update({
        "HOME": str(home),
        "XDG_CONFIG_HOME": str(home / ".config"),
        KEY_ENV: key,
        "NO_COLOR": "1",
    })
    return env


def decode_json_stream(raw: bytes) -> list[Any]:
    text = raw.decode("utf-8", errors="replace")
    decoder = json.JSONDecoder()
    offset = 0
    output = []
    while offset < len(text):
        while offset < len(text) and text[offset].isspace():
            offset += 1
        if offset >= len(text):
            break
        try:
            value, end = decoder.raw_decode(text, offset)
        except json.JSONDecodeError:
            next_line = text.find("\n", offset)
            offset = len(text) if next_line < 0 else next_line + 1
            continue
        output.append(value)
        offset = end
    return output


def walk(value: Any):
    yield value
    if isinstance(value, dict):
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def parse_events(raw: bytes, file_marker: str, final_marker: str) -> dict[str, Any]:
    events = decode_json_stream(raw)
    event_counts: dict[str, int] = {}
    tool_names: set[str] = set()
    successful_tools: set[str] = set()
    assistant_fragments: list[str] = []
    tool_use = tool_result = False
    file_marker_seen = False
    for event in events:
        if not isinstance(event, dict):
            continue
        event_type = str(event.get("type") or "unknown")
        event_counts[event_type] = event_counts.get(event_type, 0) + 1
        part = event.get("part")
        if event_type == "text" and isinstance(part, dict) and isinstance(part.get("text"), str):
            assistant_fragments.append(part["text"])
        for item in walk(event):
            if isinstance(item, str):
                file_marker_seen = file_marker_seen or file_marker in item
                continue
            if not isinstance(item, dict):
                continue
            tool = item.get("tool") or item.get("tool_name")
            state = item.get("state")
            if isinstance(tool, str):
                tool_names.add(tool)
                tool_use = True
                if isinstance(state, dict) and state.get("status") == "completed":
                    successful_tools.add(tool)
                    tool_result = True
            if item.get("type") == "tool_result" or item.get("event") == "tool_result":
                tool_result = True
    assistant_text = "\n".join(assistant_fragments)
    read_success = any(name.casefold() in {"read", "read_file", "readfile", "view_file"} for name in successful_tools)
    shell_success = any(name.casefold() in {"bash", "shell", "run_command", "run_shell_command", "execute_command"} for name in successful_tools)
    return {
        "event_count": len(events),
        "event_types": dict(sorted(event_counts.items())),
        "tool_names": sorted(tool_names),
        "successful_tools": sorted(successful_tools),
        "tool_use_observed": tool_use,
        "tool_result_observed": tool_result,
        "read_tool_verified": read_success,
        "shell_tool_verified": shell_success,
        "file_marker_verified": file_marker_seen and read_success,
        "final_marker_verified": final_marker in assistant_text,
        "assistant_text_sha256": hashlib.sha256(assistant_text.encode()).hexdigest(),
        "assistant_text_bytes": len(assistant_text.encode()),
    }


def safe_usage(row: dict[str, Any]) -> dict[str, Any]:
    allowed = (
        "id", "created_at", "api_key_id", "group_id", "account_id",
        "requested_model", "model", "request_type", "inbound_endpoint",
        "upstream_endpoint", "user_agent", "stream", "input_tokens",
        "output_tokens", "cache_creation_tokens", "cache_read_tokens",
        "reasoning_effort", "total_cost", "actual_cost",
    )
    return {key: row.get(key) for key in allowed if key in row}


def usage_rows(
    controller: Any, *, model_id: str, group_id: int, protocol: str,
    started_at: str,
) -> tuple[list[dict[str, Any]], str | None]:
    started = datetime.fromisoformat(started_at.replace("Z", "+00:00"))
    endpoint = ENDPOINTS[protocol]
    path = (
        f"/admin/usage?page=1&page_size=200&api_key_id={OWNED_KEY_ID}"
        f"&group_id={group_id}&model={urllib.parse.quote(model_id, safe='._:-')}"
        f"&start_date={started_at[:10]}&timezone=UTC"
    )
    for _ in range(10):
        try:
            candidates = rows(controller.admin_cli_json(["api", "GET", path]))
        except Exception:
            candidates = []
        result = []
        for item in candidates:
            try:
                created = datetime.fromisoformat(str(item.get("created_at", "")).replace("Z", "+00:00"))
            except ValueError:
                continue
            if created < started:
                continue
            if item.get("api_key_id") != OWNED_KEY_ID or item.get("group_id") != group_id:
                continue
            if item.get("model") != model_id and item.get("requested_model") != model_id:
                continue
            if item.get("inbound_endpoint") != endpoint or item.get("upstream_endpoint") not in KNOWN_UPSTREAM_ENDPOINTS:
                continue
            if "opencode" not in str(item.get("user_agent", "")).casefold():
                continue
            result.append(safe_usage(item))
        if result:
            upstreams = {str(item.get("upstream_endpoint")) for item in result}
            if len(upstreams) == 1:
                return sorted(result, key=lambda item: str(item.get("created_at", ""))), upstreams.pop()
        time.sleep(1)
    return [], None


def version_ok(raw: bytes) -> bool:
    text = raw.decode("utf-8", errors="replace").strip()
    return bool(re.search(r"(?:^|\D)1\.18\.15(?:\D|$)", text))


def command_for(model_id: str, protocol: str, workspace: Path, prompt: str) -> list[str]:
    return [
        "opencode", "run", "--auto", "--pure", "--format", "json",
        "--model", f"{provider_id(protocol)}/{model_id}",
        "--dir", str(workspace), prompt,
    ]


def artifact_uri(path: Path) -> str:
    resolved = path.resolve()
    try:
        return str(resolved.relative_to(ROOT))
    except ValueError:
        return str(resolved)


def run_one(
    controller: Any,
    key: str,
    *,
    model_id: str,
    group_id: int,
    protocol: str,
    output_dir: Path,
    timeout: int,
) -> tuple[bool, Path]:
    switch_group(controller, group_id)
    started_at = utc_now()
    seed = hashlib.sha256(f"{model_id}:{protocol}".encode()).hexdigest()[:12]
    file_marker = f"OPENCODE_FILE_{seed}"
    shell_marker = f"OPENCODE_SHELL_{seed}"
    final_marker = f"OPENCODE_AGENT_DONE_{seed}"
    with tempfile.TemporaryDirectory(prefix="opencode-matrix-loop-") as temporary:
        root = Path(temporary)
        home = root / "home"
        workspace = root / "workspace"
        home.mkdir()
        workspace.mkdir()
        config_path = write_config(home, model_id, protocol)
        (workspace / "proof.txt").write_text(file_marker + "\n", encoding="utf-8")
        prompt = (
            "Complete every step using local tools; never guess file contents. "
            "Use the Read tool to read proof.txt. Then use Bash or Shell to write exactly "
            f"{shell_marker} followed by a newline to result.txt. Use Read again to verify "
            f"result.txt. After all tool results reply exactly {final_marker}."
        )
        env = child_env(home, key)
        version_env = child_env(home, "")
        version_env.pop(KEY_ENV, None)
        version = subprocess.run(
            ["opencode", "--version"], cwd=workspace, env=version_env,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30, check=False,
        )
        command = command_for(model_id, protocol, workspace, prompt)
        begin = time.monotonic()
        try:
            completed = subprocess.run(
                command, cwd=workspace, env=env, stdout=subprocess.PIPE,
                stderr=subprocess.PIPE, timeout=timeout, check=False,
            )
            timed_out = False
        except subprocess.TimeoutExpired as error:
            completed = subprocess.CompletedProcess(
                command, 124, error.stdout or b"", error.stderr or b"",
            )
            timed_out = True
        duration_ms = int((time.monotonic() - begin) * 1000)
        # Erase the only in-memory child environment copy as soon as the child
        # exits. The temporary config contains only {env:...}, never the key.
        env[KEY_ENV] = ""
        parsed = parse_events(completed.stdout, file_marker, final_marker)
        shell_file_verified = (
            (workspace / "result.txt").is_file()
            and (workspace / "result.txt").read_text(encoding="utf-8").strip() == shell_marker
        )
        config_sha256 = file_sha(config_path)
        config_secret_free = key not in config_path.read_text(encoding="utf-8")

    attribution_rows, upstream_endpoint = usage_rows(
        controller, model_id=model_id, group_id=group_id,
        protocol=protocol, started_at=started_at,
    )
    output_dir.mkdir(parents=True, exist_ok=True)
    stem = f"opencode-{protocol}-{model_id}-group{group_id}-20260901"
    attribution_path = output_dir / f"{stem}-usage.json"
    atomic_json(attribution_path, {
        "schema_version": 1,
        "kind": "client_usage_attribution",
        "client_id": "opencode",
        "client_version": CLIENT_VERSION_KEY,
        "user_id": OWNED_USER_ID,
        "api_key_id": OWNED_KEY_ID,
        "group_id": group_id,
        "model_id": model_id,
        "protocol": protocol,
        "observed_at": utc_now(),
        "rows": attribution_rows,
        "secret_free": True,
    })
    passed = all((
        version.returncode == 0,
        version_ok(version.stdout),
        completed.returncode == 0,
        not timed_out,
        parsed["tool_use_observed"],
        parsed["tool_result_observed"],
        parsed["read_tool_verified"],
        parsed["shell_tool_verified"],
        parsed["file_marker_verified"],
        parsed["final_marker_verified"],
        shell_file_verified,
        config_secret_free,
        bool(attribution_rows),
        upstream_endpoint in KNOWN_UPSTREAM_ENDPOINTS,
    ))
    artifact_path = output_dir / f"{stem}-loop.json"
    atomic_json(artifact_path, {
        "schema_version": 1,
        "kind": "real_client_loop",
        "secret_free": True,
        "client_id": "opencode",
        "client_version": CLIENT_VERSION_KEY,
        "os": "macos",
        "architecture": platform.machine(),
        "model_id": model_id,
        "protocol": protocol,
        "base_url": BASE_URLS[protocol],
        "group_id": group_id,
        "owned_key": {
            "user_id": OWNED_USER_ID,
            "key_id": OWNED_KEY_ID,
            "name": OWNED_KEY_NAME,
        },
        "observed_at": started_at,
        "finished_at": utc_now(),
        "duration_ms": duration_ms,
        "command_contract": {
            "client_version": CLIENT_VERSION,
            "provider_id": provider_id(protocol),
            "provider_package": PROVIDER_PACKAGES[protocol],
            "model": model_id,
            "output_format": "json",
            "auto": True,
            "pure": True,
        },
        "temporary_config": {
            "path": "~/.config/opencode/opencode.json",
            "sha256": config_sha256,
            "api_key_source": f"child env {KEY_ENV}",
            "literal_key_written": not config_secret_free,
            "removed_with_temporary_home": True,
        },
        "exit_code": completed.returncode,
        "timed_out": timed_out,
        **parsed,
        "shell_file_verified": shell_file_verified,
        "stdout_sha256": hashlib.sha256(completed.stdout).hexdigest(),
        "stderr_sha256": hashlib.sha256(completed.stderr).hexdigest(),
        "stdout_bytes": len(completed.stdout),
        "stderr_bytes": len(completed.stderr),
        "restored_group_id": None,
        "passed": passed,
        "protocol_attribution": {
            "artifact_uri": artifact_uri(attribution_path),
            "artifact_sha256": file_sha(attribution_path),
            "usage_rows": len(attribution_rows),
            "inbound_endpoint": ENDPOINTS[protocol],
            "upstream_endpoint": upstream_endpoint,
            "verified": bool(attribution_rows),
        },
    })
    print(json.dumps({
        "model_id": model_id,
        "group_id": group_id,
        "protocol": protocol,
        "passed": passed,
        "artifact": str(artifact_path),
    }, ensure_ascii=False))
    return passed, artifact_path


def parse_target(raw: str) -> tuple[str, int, str]:
    parts = raw.rsplit(":", 2)
    if len(parts) != 3:
        raise argparse.ArgumentTypeError("target must be MODEL_ID:GROUP_ID:PROTOCOL")
    model_id, group_raw, protocol = parts
    if not SAFE_MODEL.fullmatch(model_id) or not group_raw.isdigit() or int(group_raw) <= 0:
        raise argparse.ArgumentTypeError("target model/group is invalid")
    if protocol not in PROTOCOLS:
        raise argparse.ArgumentTypeError("target protocol is invalid")
    return model_id, int(group_raw), protocol


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--target", action="append", type=parse_target, required=True,
        help="MODEL_ID:GROUP_ID:PROTOCOL; repeatable",
    )
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--timeout", type=int, default=360)
    parser.add_argument("--execute", action="store_true")
    parser.add_argument("--acknowledge-paid-probes", action="store_true")
    args = parser.parse_args(argv)
    if not args.execute or not args.acknowledge_paid_probes:
        raise SystemExit("live run requires --execute and --acknowledge-paid-probes")

    controller = load_controller()
    key = ""
    all_passed = True
    artifacts: list[Path] = []
    restored = False
    try:
        current = controller.owned_key()
        if (
            current.get("id") != OWNED_KEY_ID
            or current.get("user_id") != OWNED_USER_ID
            or current.get("group_id") != RESTORE_GROUP_ID
        ):
            raise RuntimeError("owned key safety gate failed")
        key = current["key"]
        for model_id, group_id, protocol in args.target:
            passed, artifact = run_one(
                controller, key, model_id=model_id, group_id=group_id,
                protocol=protocol, output_dir=args.output_dir,
                timeout=args.timeout,
            )
            artifacts.append(artifact)
            all_passed = passed and all_passed
    finally:
        key = ""
        switch_group(controller, RESTORE_GROUP_ID)
        restored = True

    if restored:
        for artifact_path in artifacts:
            artifact = json.loads(artifact_path.read_text(encoding="utf-8"))
            artifact.pop("artifact_sha256", None)
            artifact["restored_group_id"] = RESTORE_GROUP_ID
            atomic_json(artifact_path, artifact)
    return 0 if all_passed else 2


if __name__ == "__main__":
    raise SystemExit(main())

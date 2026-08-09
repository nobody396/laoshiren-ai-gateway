#!/usr/bin/env python3
"""End-to-end acceptance test for the feedback co-creation workflow.

The script never prints credentials. It uses a real authenticated user for the
user-side flow and either that admin JWT or an Admin API Key for admin actions.
Every successful run intentionally leaves one auditable feedback ticket and one
fixed 5-unit reward on the selected owned test identity.
"""

from __future__ import annotations

import argparse
import base64
from decimal import Decimal
import json
import os
import secrets
import sys
import time
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, build_opener


class E2EError(RuntimeError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise E2EError(message)


class API:
    def __init__(self, base_url: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.opener = build_opener()

    def request(
        self,
        method: str,
        path: str,
        *,
        token: str = "",
        admin_key: str = "",
        payload: Any | None = None,
        body: bytes | None = None,
        content_type: str = "application/json",
        expected: tuple[int, ...] = (200,),
    ) -> tuple[int, dict[str, Any]]:
        headers = {"Accept": "application/json"}
        if token:
            headers["Authorization"] = "Bearer " + token
        if admin_key:
            headers["x-api-key"] = admin_key
        if payload is not None:
            body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        if body is not None:
            headers["Content-Type"] = content_type
        req = Request(self.base_url + path, data=body, headers=headers, method=method)
        try:
            with self.opener.open(req, timeout=30) as response:
                status = response.status
                raw = response.read()
        except HTTPError as exc:
            status = exc.code
            raw = exc.read()
        except URLError as exc:
            raise E2EError(f"request failed: {method} {path}: {exc.reason}") from exc
        try:
            decoded = json.loads(raw) if raw else {}
        except json.JSONDecodeError as exc:
            raise E2EError(f"non-JSON response: {method} {path} HTTP {status}") from exc
        if status not in expected:
            message = decoded.get("message", "unexpected response") if isinstance(decoded, dict) else "unexpected response"
            raise E2EError(f"{method} {path} returned HTTP {status}: {message}")
        require(isinstance(decoded, dict), f"invalid envelope: {method} {path}")
        return status, decoded

    def request_bytes(self, path: str, *, expected: tuple[int, ...] = (200,)) -> tuple[int, bytes, str]:
        req = Request(self.base_url + path, headers={"Accept": "image/*"}, method="GET")
        try:
            with self.opener.open(req, timeout=30) as response:
                status = response.status
                raw = response.read()
                content_type = response.headers.get_content_type()
        except HTTPError as exc:
            status = exc.code
            raw = exc.read()
            content_type = exc.headers.get_content_type()
        except URLError as exc:
            raise E2EError(f"request failed: GET {path}: {exc.reason}") from exc
        if status not in expected:
            raise E2EError(f"GET {path} returned HTTP {status}")
        return status, raw, content_type


def data(envelope: dict[str, Any]) -> Any:
    require(envelope.get("code") == 0, f"API error envelope: {envelope.get('message', 'unknown')}")
    return envelope.get("data")


def auth_headers(admin_key: str, user_token: str) -> dict[str, str]:
    return {"admin_key": admin_key} if admin_key else {"token": user_token}


def login(api: API, email: str, password: str) -> tuple[str, dict[str, Any]]:
    _, envelope = api.request(
        "POST",
        "/api/v1/auth/login",
        payload={"email": email, "password": password, "turnstile_token": ""},
    )
    result = data(envelope)
    require(isinstance(result, dict), "login did not return an object")
    require(not result.get("requires_2fa"), "test identity requires interactive 2FA")
    token = str(result.get("access_token", "")).strip()
    require(len(token) > 20, "login did not return an access token")
    user = result.get("user")
    require(isinstance(user, dict), "login did not return the user")
    return token, user


def make_png_multipart() -> tuple[bytes, str]:
    # A valid transparent 1x1 PNG.
    png = base64.b64decode(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
    )
    boundary = "----feedback-e2e-" + secrets.token_hex(12)
    body = (
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="file"; filename="feedback-e2e.png"\r\n'
        "Content-Type: image/png\r\n\r\n"
    ).encode() + png + f"\r\n--{boundary}--\r\n".encode()
    return body, "multipart/form-data; boundary=" + boundary


def run(args: argparse.Namespace) -> dict[str, Any]:
    email = os.environ.get(args.user_email_env, "").strip()
    password = os.environ.get(args.user_password_env, "")
    admin_key = os.environ.get(args.admin_api_key_env, "").strip()
    require(email != "" and password != "", "owned test user credentials are unavailable")

    api = API(args.base_url)
    _, health = api.request("GET", "/health")
    require(bool(health), "health response is empty")
    user_token, login_user = login(api, email, password)
    user_id = int(login_user["id"])
    require(user_id > 0, "invalid owned test user id")
    if args.expected_user_id:
        require(user_id == args.expected_user_id, "refusing unexpected test identity")
    if not admin_key:
        require(login_user.get("role") == "admin", "admin API key is required for a non-admin test user")

    _, before_envelope = api.request("GET", "/api/v1/user/profile", token=user_token)
    before = data(before_envelope)
    before_balance = Decimal(str(before["balance"]))

    image_urls: list[str] = []
    upload_body, upload_type = make_png_multipart()
    upload_status, upload_envelope = api.request(
        "POST",
        "/api/v1/feedbacks/upload-image",
        token=user_token,
        body=upload_body,
        content_type=upload_type,
        expected=(201, 503),
    )
    upload_available = upload_status == 201
    if upload_available:
        upload_result = data(upload_envelope)
        require(isinstance(upload_result, dict) and str(upload_result.get("url", "")).startswith("/api/v1/feedback-images/"), "upload did not return a private signed URL")
        image_url = str(upload_result["url"])
        _, image_body, image_content_type = api.request_bytes(image_url)
        require(image_body.startswith(b"\x89PNG\r\n\x1a\n"), "private screenshot readback is not the uploaded PNG")
        require(image_content_type == "image/png", "private screenshot readback has the wrong content type")
        image_urls.append(image_url)
    else:
        require(args.allow_upload_unavailable, "private screenshot storage is unavailable")

    nonce = time.strftime("%Y%m%d-%H%M%S") + "-" + secrets.token_hex(3)
    request_id = "feedback-e2e-" + nonce
    title = "反馈共创生产链路验收 " + nonce
    _, created_envelope = api.request(
        "POST",
        "/api/v1/feedbacks",
        token=user_token,
        payload={
            "category": "bug",
            "title": title,
            "content": "这是自有测试账号发起的反馈共创端到端验收工单。",
            "images": image_urls,
            "contact": "owned-e2e",
            "request_id": request_id,
        },
        expected=(201,),
    )
    created = data(created_envelope)
    feedback_id = int(created["id"])
    require(created.get("request_id") == request_id, "request id was not persisted")
    require(created.get("triage_status") == "unreviewed", "new feedback was not queued")

    _, user_detail_envelope = api.request("GET", f"/api/v1/feedbacks/{feedback_id}", token=user_token)
    user_detail = data(user_detail_envelope)
    require(user_detail.get("triage_summary", "") == "", "user response leaked triage summary")
    require(user_detail.get("repair_recommendation", "") == "", "user response leaked repair recommendation")
    require([event["event_type"] for event in user_detail["events"]] == ["submitted"], "submitted event is missing")

    admin_auth = auth_headers(admin_key, user_token)
    _, queue_envelope = api.request(
        "GET", "/api/v1/admin/feedbacks/agent-queue?page=1&page_size=100", **admin_auth
    )
    queue = data(queue_envelope)
    require(any(int(item["id"]) == feedback_id for item in queue["items"]), "ticket is absent from agent queue")

    triage_payload = {
        "triage_status": "confirmed",
        "triage_priority": "P1",
        "triage_summary": "端到端验收已复现并确认。",
        "triage_confidence": 1.0,
        "repair_difficulty": "low",
        "repair_recommendation": "执行反馈工作流验收。",
    }
    api.request(
        "POST",
        f"/api/v1/admin/feedbacks/{feedback_id}/agent-analysis",
        payload=triage_payload,
        **admin_auth,
    )
    # Same analysis is idempotent; a different second analysis must conflict.
    api.request(
        "POST",
        f"/api/v1/admin/feedbacks/{feedback_id}/agent-analysis",
        payload=triage_payload,
        **admin_auth,
    )
    api.request(
        "POST",
        f"/api/v1/admin/feedbacks/{feedback_id}/agent-analysis",
        payload={**triage_payload, "triage_priority": "P2"},
        expected=(409,),
        **admin_auth,
    )

    batch_id = "FB-E2E-" + nonce
    _, accept_envelope = api.request(
        "POST",
        "/api/v1/admin/feedbacks/accept",
        payload={"ids": [feedback_id], "batch_id": batch_id},
        **admin_auth,
    )
    accept = data(accept_envelope)
    first_result = accept["results"][0]
    require(not first_result.get("error"), "accept failed")
    require(Decimal(str(first_result["reward"]["amount"])) == Decimal("5"), "reward is not fixed at 5")
    require(first_result.get("already_accepted") is False, "first accept was treated as duplicate")

    _, repeat_envelope = api.request(
        "POST",
        "/api/v1/admin/feedbacks/accept",
        payload={"ids": [feedback_id], "batch_id": batch_id},
        **admin_auth,
    )
    repeat = data(repeat_envelope)["results"][0]
    require(repeat.get("already_accepted") is True and not repeat.get("error"), "accept is not idempotent")

    _, after_envelope = api.request("GET", "/api/v1/user/profile", token=user_token)
    after_balance = Decimal(str(data(after_envelope)["balance"]))
    require(after_balance - before_balance == Decimal("5"), "balance did not increase exactly once by 5")

    _, rewards_envelope = api.request(
        "GET",
        f"/api/v1/admin/feedbacks/rewards?page=1&page_size=100&batch_id={batch_id}",
        **admin_auth,
    )
    rewards = data(rewards_envelope)
    matching_rewards = [item for item in rewards["items"] if int(item["feedback_id"]) == feedback_id]
    require(len(matching_rewards) == 1, "reward ledger is missing or duplicated")
    require(Decimal(str(matching_rewards[0]["amount"])) == Decimal("5"), "reward ledger amount is wrong")

    api.request("POST", f"/api/v1/admin/feedbacks/{feedback_id}/mark-fixing", **admin_auth)
    api.request(
        "POST",
        f"/api/v1/admin/feedbacks/{feedback_id}/complete",
        payload={"resolved_version": "feedback-e2e-1", "notify_in_app": True},
        **admin_auth,
    )
    # Completion and in-app notification are idempotent for the same version.
    api.request(
        "POST",
        f"/api/v1/admin/feedbacks/{feedback_id}/complete",
        payload={"resolved_version": "feedback-e2e-1", "notify_in_app": True},
        **admin_auth,
    )

    _, notifications_envelope = api.request("GET", "/api/v1/notifications?limit=100", token=user_token)
    notifications = data(notifications_envelope)
    ticket_notifications = [item for item in notifications["items"] if int(item.get("feedback_id") or 0) == feedback_id]
    require(sorted(item["type"] for item in ticket_notifications) == ["feedback_fixed", "feedback_reward"], "expected reward and fixed notifications exactly once")

    api.request(
        "POST",
        f"/api/v1/feedbacks/{feedback_id}/verification",
        token=user_token,
        payload={"resolved": False, "note": "第一次验收模拟仍未解决。"},
    )
    api.request("POST", f"/api/v1/admin/feedbacks/{feedback_id}/mark-fixing", **admin_auth)
    api.request(
        "POST",
        f"/api/v1/admin/feedbacks/{feedback_id}/complete",
        payload={"resolved_version": "feedback-e2e-2", "notify_in_app": True},
        **admin_auth,
    )
    api.request(
        "POST",
        f"/api/v1/feedbacks/{feedback_id}/verification",
        token=user_token,
        payload={"resolved": True, "note": "第二次验收确认问题已解决。"},
    )

    _, final_user_envelope = api.request("GET", f"/api/v1/feedbacks/{feedback_id}", token=user_token)
    final_user = data(final_user_envelope)
    require(final_user.get("fix_status") == "verified" and final_user.get("status") == "closed", "ticket did not close after user verification")
    event_types = [event["event_type"] for event in final_user["events"]]
    for required_event in ("submitted", "triaged", "accepted", "reward_granted", "fix_started", "fixed", "notified", "reopened", "verified"):
        require(required_event in event_types, f"timeline is missing {required_event}")
    require(all("ledger_id" not in event.get("metadata", {}) for event in final_user["events"]), "user timeline leaked ledger details")
    require(final_user["reward"].get("account_change_record_id") in (None, 0), "user response leaked account ledger id")

    # Read-one and read-all must both be ownership-safe and effective.
    api.request("POST", f"/api/v1/notifications/{ticket_notifications[0]['id']}/read", token=user_token)
    api.request("POST", "/api/v1/notifications/read-all", token=user_token)
    _, readback_envelope = api.request("GET", "/api/v1/notifications?limit=100", token=user_token)
    require(data(readback_envelope)["unread_count"] == 0, "notifications were not marked read")

    return {
        "status": "passed",
        "base_url": args.base_url,
        "owned_user_id": user_id,
        "feedback_id": feedback_id,
        "batch_id": batch_id,
        "reward_amount": "5.00",
        "balance_delta": str(after_balance - before_balance),
        "screenshot_upload": "passed" if upload_available else "unavailable-allowed",
        "final_fix_status": final_user["fix_status"],
        "timeline_event_count": len(final_user["events"]),
        "email_sent": False,
    }


def parser() -> argparse.ArgumentParser:
    value = argparse.ArgumentParser()
    value.add_argument("--base-url", default="http://127.0.0.1:8080")
    value.add_argument("--user-email-env", default="ADMIN_EMAIL")
    value.add_argument("--user-password-env", default="ADMIN_PASSWORD")
    value.add_argument("--admin-api-key-env", default="SUB2API_ADMIN_API_KEY")
    value.add_argument("--expected-user-id", type=int, default=0)
    value.add_argument("--allow-upload-unavailable", action="store_true")
    return value


def main() -> int:
    try:
        result = run(parser().parse_args())
    except (E2EError, KeyError, TypeError, ValueError) as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False))
        return 1
    print(json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())

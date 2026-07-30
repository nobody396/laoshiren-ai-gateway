#!/usr/bin/env python3
"""Isolated Stage acceptance check for partner self-consumption commission.

The check uses the isolated Stage partner created by the base acceptance flow,
enables the per-partner policy through the admin API, redeems a real ¥3 paid
balance code, and creates an API key. A small Go helper then invokes the same
billing repository used by the gateway. The final phase verifies the
partner-facing wallet and commission APIs.

This script is intentionally Stage-only. It never prints passwords, access
tokens, API keys, or unredeemed card codes.
"""
from __future__ import annotations

import json
import os
import sys
import uuid
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path


BASE = os.environ.get("AFFILIATE_STAGING_URL", "http://127.0.0.1:18080").rstrip("/") + "/api/v1"
ADMIN_EMAIL = os.environ.get("AFFILIATE_STAGING_ADMIN_EMAIL", "ops-admin@partner.local")
ADMIN_PASS = os.environ["AFFILIATE_STAGING_ADMIN_PASSWORD"]
DEMO_PASS = "Demo123456"
CONTEXT_PATH = Path(os.environ["AFFILIATE_SELF_E2E_CONTEXT"])


def unwrap(parsed):
    if isinstance(parsed, dict) and "code" in parsed:
        if parsed.get("code") not in (0, "0"):
            raise RuntimeError(
                "api code=%s message=%s" % (parsed.get("code"), parsed.get("message"))
            )
        return parsed.get("data")
    if isinstance(parsed, dict) and "data" in parsed and len(parsed) <= 3:
        return parsed.get("data")
    return parsed


def req(path: str, *, method="GET", payload=None, token=None):
    headers = {"Accept": "application/json"}
    body = None
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(BASE + path, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            raw = response.read()
            return unwrap(json.loads(raw.decode("utf-8"))) if raw else None
    except urllib.error.HTTPError as exc:
        response_body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(
            "%s %s HTTP %s: %s" % (method, path, exc.code, response_body[:800])
        ) from exc


def login(email: str, password: str):
    result = req("/auth/login", method="POST", payload={"email": email, "password": password})
    token = result.get("access_token")
    if not token:
        raise RuntimeError("login returned no access token")
    return token


def prepare():
    admin_token = login(ADMIN_EMAIL, ADMIN_PASS)
    suffix = uuid.uuid4().hex[:12]
    email = "partner-upgrade@partner.local"
    partner_token = login(email, DEMO_PASS)
    me = req("/auth/me", token=partner_token)
    user_id = int(me["id"])
    if me.get("role") != "agent":
        raise RuntimeError("run the base Stage E2E before the self-commission check")
    balance_before = float(me.get("balance", 0))

    risk_items = req("/admin/agents/affiliate-risk?limit=500", token=admin_token)["items"]
    principal = next((item for item in risk_items if int(item["agent_id"]) == user_id), None)
    if not principal:
        raise RuntimeError("partner principal was not created")
    if (
        principal.get("agent_status") != "active"
        or principal.get("risk_status") != "clear"
        or principal.get("has_upstream")
        or not principal.get("self_commission_eligible")
    ):
        raise RuntimeError("disposable partner is not eligible: %s" % principal)

    if principal.get("self_commission_enabled"):
        policy = principal
    else:
        policy = req(
            "/admin/agents/%d/self-commission-policy" % user_id,
            method="PUT",
            token=admin_token,
            payload={
                "enabled": True,
                "expected_revision": int(principal["self_commission_revision"]),
                "reason": "隔离 Stage 本人消费返佣端到端验收",
            },
        )
    policy_enabled = policy.get("enabled", policy.get("self_commission_enabled"))
    policy_rate = policy.get("rate_bps", policy.get("self_commission_rate_bps", 0))
    policy_eligible = policy.get("eligible", policy.get("self_commission_eligible"))
    if not policy_enabled or int(policy_rate) != 1000 or not policy_eligible:
        raise RuntimeError("unexpected enabled policy: %s" % policy)

    generated = req(
        "/admin/redeem-codes/generate",
        method="POST",
        token=admin_token,
        payload={
            "count": 1,
            "type": "balance",
            "value": 3,
            "batch_name": "isolated Stage self-commission acceptance",
            "purpose": "sale_recharge",
            "sales_status": "sold",
            "sales_channel": "stage-e2e",
            "external_order_no": "SELF-E2E-" + suffix.upper(),
            "internal_notes": "disposable Stage fixture; already paid for test",
        },
    )
    if not isinstance(generated, list) or len(generated) != 1:
        raise RuntimeError("expected one generated paid card")

    wallet_before = req("/agent/affiliate/wallet", token=partner_token)
    records_before = req(
        "/agent/commissions?"
        + urllib.parse.urlencode(
            {"page": 1, "page_size": 100, "type": "self_consumption_commission"}
        ),
        token=partner_token,
    )
    records_before_items = records_before.get("items") or []
    redeemed = req(
        "/redeem",
        method="POST",
        token=partner_token,
        payload={"code": generated[0]["code"]},
    )
    if redeemed.get("type") != "balance" or abs(float(redeemed.get("value", -1)) - 3.0) > 1e-9:
        raise RuntimeError("unexpected ¥3 paid card redemption response")
    me_after_redeem = req("/auth/me", token=partner_token)
    if abs(float(me_after_redeem.get("balance", -1)) - (balance_before + 3.0)) > 1e-9:
        raise RuntimeError("¥3 paid card did not credit the isolated partner")

    api_key = req(
        "/keys",
        method="POST",
        token=partner_token,
        payload={"name": "本人返佣 Stage 验收密钥"},
    )
    api_key_id = int(api_key["id"])

    CONTEXT_PATH.write_text(
        json.dumps(
            {
                "user_id": user_id,
                "api_key_id": api_key_id,
                "partner_token": partner_token,
                "email": email,
                "balance_before_micros": int(round(balance_before * 1_000_000)),
                "available_cash_before": int(wallet_before["available_cash_micros"]),
                "lifetime_earned_before": int(wallet_before["lifetime_earned_micros"]),
                "commission_amount_before": sum(
                    float(item.get("amount", 0)) for item in records_before_items
                ),
                "commission_source_before": sum(
                    float(item.get("source_amount", 0)) for item in records_before_items
                ),
            }
        ),
        encoding="utf-8",
    )
    CONTEXT_PATH.chmod(0o600)
    print("OK self policy + future paid purchase - isolated partner enabled, ¥3 credited")


def verify():
    context = json.loads(CONTEXT_PATH.read_text(encoding="utf-8"))
    token = context["partner_token"]
    wallet = req("/agent/affiliate/wallet", token=token)
    if int(wallet.get("available_cash_micros", -1)) != context["available_cash_before"] + 300_000:
        raise RuntimeError("expected available cash to increase by exactly ¥0.30: %s" % wallet)
    if int(wallet.get("lifetime_earned_micros", -1)) != context["lifetime_earned_before"] + 300_000:
        raise RuntimeError("expected lifetime earnings to increase by exactly ¥0.30: %s" % wallet)

    query = urllib.parse.urlencode(
        {"page": 1, "page_size": 100, "type": "self_consumption_commission"}
    )
    records = req("/agent/commissions?" + query, token=token)
    items = records.get("items") or []
    if not items:
        raise RuntimeError("expected a partner self-consumption commission record")
    if any(item.get("type") != "self_consumption_commission" for item in items):
        raise RuntimeError("unexpected commission type in self-consumption filter")
    commission_amount = sum(float(item.get("amount", 0)) for item in items)
    commission_source = sum(float(item.get("source_amount", 0)) for item in items)
    if (
        abs(commission_amount - context["commission_amount_before"] - 0.3) > 1e-9
        or abs(commission_source - context["commission_source_before"] - 3.0) > 1e-9
    ):
        raise RuntimeError("daily self-consumption commission aggregate did not increase correctly")

    print(
        "OK real billing settlement + partner UI APIs - ¥3 consumed, ¥0.30 cash commission, daily record updated"
    )
    print("SUMMARY self-consumption commission Stage E2E passed")


if __name__ == "__main__":
    if len(sys.argv) != 2 or sys.argv[1] not in {"prepare", "verify"}:
        raise SystemExit("usage: affiliate-self-commission-e2e-check.py prepare|verify")
    prepare() if sys.argv[1] == "prepare" else verify()

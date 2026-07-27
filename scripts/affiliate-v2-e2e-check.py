#!/usr/bin/env python3
"""Affiliate V2 staging end-to-end API checks.

The script expects AFFILIATE_STAGING_ADMIN_PASSWORD in the environment.
It never prints the password. It uses demo users seeded by affiliate-v2-demo-data.sh.
"""
from __future__ import annotations

import json
import os
import uuid
import urllib.error
import urllib.request
from pathlib import Path

BASE = os.environ.get('AFFILIATE_STAGING_URL', 'http://127.0.0.1:18080').rstrip('/') + '/api/v1'
DEMO_PASS = 'Demo123456'
ADMIN_EMAIL = os.environ.get('AFFILIATE_STAGING_ADMIN_EMAIL', 'affiliate-staging@local.invalid')
ADMIN_PASS = os.environ['AFFILIATE_STAGING_ADMIN_PASSWORD']
ROOT = Path(__file__).resolve().parents[1]

results: list[tuple[str, str]] = []


def unwrap(parsed):
    if isinstance(parsed, dict) and 'code' in parsed:
        if parsed.get('code') not in (0, '0'):
            raise RuntimeError('api code=%s message=%s' % (parsed.get('code'), parsed.get('message')))
        return parsed.get('data')
    if isinstance(parsed, dict) and 'data' in parsed and len(parsed) <= 3:
        return parsed.get('data')
    return parsed


def req(path: str, *, method='GET', payload=None, token=None, headers=None, raw=None, expect_bytes=False):
    request_headers = {'Accept': 'application/json'}
    if headers:
        request_headers.update(headers)
    data = raw
    if payload is not None:
        data = json.dumps(payload).encode('utf-8')
        request_headers['Content-Type'] = 'application/json'
    if token:
        request_headers['Authorization'] = 'Bearer ' + token
    request = urllib.request.Request(BASE + path, data=data, headers=request_headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read()
            if expect_bytes:
                return body
            if not body:
                return None
            return unwrap(json.loads(body.decode('utf-8')))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode('utf-8', errors='replace')
        raise RuntimeError('%s %s HTTP %s: %s' % (method, path, exc.code, body[:800])) from exc


def login(email: str, password: str = DEMO_PASS):
    data = req('/auth/login', method='POST', payload={'email': email, 'password': password})
    token = data.get('access_token')
    if not token:
        raise RuntimeError('login failed for ' + email)
    return token, data.get('user', {})


def ok(name: str, detail=''):
    results.append((name, detail))
    print('OK ' + name + ((' - ' + detail) if detail else ''))


def multipart_png_body(file_path: Path):
    boundary = '----affiliateE2E' + uuid.uuid4().hex
    file_bytes = file_path.read_bytes()
    body = (
        ('--%s\r\n' % boundary) +
        'Content-Disposition: form-data; name="file"; filename="favicon.png"\r\n' +
        'Content-Type: image/png\r\n\r\n'
    ).encode() + file_bytes + ('\r\n--%s--\r\n' % boundary).encode()
    return body, {'Content-Type': 'multipart/form-data; boundary=' + boundary}


def main():
    candidate_token, candidate_user = login('agent-candidate@demo.local')
    if candidate_user.get('role') == 'agent':
        raise RuntimeError('candidate unexpectedly already agent before activation')
    invite = req('/user/invite-code', token=candidate_token)
    assert invite['invite_code'] == 'CANDIDATE', invite
    qual = req('/user/affiliate/qualification', token=candidate_token)
    assert qual['can_activate'] is True and qual['valid_direct_user_count'] >= 10, qual
    ok('ordinary invite + qualification', 'invite=%s can_activate=%s' % (invite['invite_code'], qual['can_activate']))

    activation = req('/user/affiliate/activate', method='POST', token=candidate_token)
    default_link = activation['default_link']
    assert default_link['is_default'] is True and default_link['customer_rebate_rate_bps'] == 500, default_link
    me_after = req('/auth/me', token=candidate_token)
    assert me_after['role'] == 'agent', me_after
    ok('candidate activation', 'default_ref=%s role=%s' % (default_link['code'], me_after['role']))

    links = req('/agent/affiliate/links', token=candidate_token)['items']
    assert any(item['is_default'] for item in links), links
    created = req('/agent/affiliate/links', method='POST', token=candidate_token, payload={
        'name': 'API E2E 活动链接',
        'channel': 'api-e2e',
        'customer_rebate_rate_bps': 600,
    })
    assert created['customer_rebate_rate_bps'] == 600 and created['agent_commission_rate_bps'] == 400, created
    rated = req('/agent/affiliate/links/%s/rate' % created['id'], method='PUT', token=candidate_token, payload={'customer_rebate_rate_bps': 700})
    assert rated['customer_rebate_rate_bps'] == 700 and rated['agent_commission_rate_bps'] == 300, rated
    disabled = req('/agent/affiliate/links/%s/status' % created['id'], method='PUT', token=candidate_token, payload={'status': 'paused'})
    assert disabled['status'] == 'disabled', disabled
    enabled = req('/agent/affiliate/links/%s/status' % created['id'], method='PUT', token=candidate_token, payload={'status': 'active'})
    assert enabled['status'] == 'active', enabled
    ok('dynamic link lifecycle', 'link=%s rate=7/3 toggled' % created['code'])

    profile = req('/agent/payment-profile', method='PUT', token=candidate_token, payload={
        'alipay_real_name': 'API E2E 测试',
        'alipay_account': 'api-e2e-alipay@example.com',
        'contact_phone': '13800002222',
        'payment_note': 'API E2E 自动测试',
    })
    assert profile['alipay_account'] == 'api-e2e-alipay@example.com', profile
    body, headers = multipart_png_body(ROOT / 'frontend/public/favicon.png')
    profile2 = req('/agent/payment-profile/alipay-qr', method='POST', token=candidate_token, raw=body, headers=headers)
    assert profile2['has_alipay_qr'] is True and profile2['verification_status'] == 'pending_review', profile2
    qr = req('/agent/payment-profile/alipay-qr', token=candidate_token, expect_bytes=True)
    assert len(qr) > 100, len(qr)
    ok('payment profile submit + QR', 'status=%s qr_bytes=%d' % (profile2['verification_status'], len(qr)))

    alpha_token, _ = login('agent-alpha@demo.local')
    wallet = req('/agent/affiliate/wallet', token=alpha_token)
    assert wallet['conversion_multiplier_millis'] == 1200, wallet
    available = int(wallet['available_cash_micros'])
    if available >= 20_000_000:
        conv = req('/agent/affiliate/wallet/convert', method='POST', token=alpha_token, payload={'amount_micros': 10_000_000}, headers={'Idempotency-Key': 'e2e-convert-' + uuid.uuid4().hex})
        assert conv['credit_amount_micros'] == 12_000_000, conv
        ok('wallet conversion', '¥10 cash -> ⚡12')
    else:
        ok('wallet conversion skipped', 'available=%d' % available)
    wallet2 = req('/agent/affiliate/wallet', token=alpha_token)
    if int(wallet2['available_cash_micros']) >= int(wallet2['withdrawal_minimum_micros']):
        wd = req('/agent/affiliate/withdrawals', method='POST', token=alpha_token, payload={'amount_micros': int(wallet2['withdrawal_minimum_micros'])}, headers={'Idempotency-Key': 'e2e-withdraw-' + uuid.uuid4().hex})
        assert wd['status'] == 'processing', wd
        ok('withdrawal request', 'id=%s amount_micros=%s' % (wd['id'], wd['amount_micros']))
    else:
        wd = None
        ok('withdrawal request skipped', 'available=%s' % wallet2['available_cash_micros'])

    admin_token, _ = login(ADMIN_EMAIL, ADMIN_PASS)
    program = req('/admin/agents/affiliate-program', token=admin_token)
    assert program['mode'] == 'live' and program['revision'] >= 1, program
    program_update_payload = dict(program)
    program_update_payload['withdrawal_sla_hours'] = program['withdrawal_sla_hours']
    program2 = req('/admin/agents/affiliate-program', method='PUT', token=admin_token, payload=program_update_payload)
    assert program2['mode'] == 'live', program2
    policy = req('/admin/agents/affiliate-commercial-policy', token=admin_token)
    assert policy['credit_asset_symbol'] == '⚡' and policy['passes_configured_margin_gate'] is True, policy
    assert all(abs(float(group['rate_multiplier']) - 0.5) < 1e-9 for group in policy['group_targets'] if group['name'].startswith('GPT ')), policy['group_targets']
    ok('admin program + commercial policy', 'margin=%s gpt_targets=%d' % (policy['minimum_stress_margin_percent'], len(policy['group_targets'])))

    pending = req('/admin/agents/payment-profiles/pending', token=admin_token)['items']
    if not any(item['agent_id'] == profile2['agent_id'] for item in pending):
        raise RuntimeError('candidate profile not in pending list')
    admin_qr = req('/admin/agents/%s/payment-profile/alipay-qr' % profile2['agent_id'], token=admin_token, expect_bytes=True)
    assert len(admin_qr) == len(qr), (len(admin_qr), len(qr))
    verified = req('/admin/agents/%s/payment-profile/verification' % profile2['agent_id'], method='PUT', token=admin_token, payload={'status': 'verified', 'note': 'API E2E 审核通过'})
    assert verified['verification_status'] == 'verified', verified
    ok('admin payment review + QR preview', 'agent=%s verified' % profile2['agent_id'])

    risk_review = req('/admin/agents/%s/affiliate-risk' % profile2['agent_id'], method='PUT', token=admin_token, payload={'status': 'review', 'reason': 'API E2E 临时暂停确认'})
    assert risk_review['next_risk_status'] == 'review', risk_review
    risk_clear = req('/admin/agents/%s/affiliate-risk' % profile2['agent_id'], method='PUT', token=admin_token, payload={'status': 'clear', 'reason': 'API E2E 恢复合作'})
    assert risk_clear['next_risk_status'] == 'clear', risk_clear
    ok('admin risk status update', 'clear -> review -> clear')

    if wd:
        admin_wds = req('/admin/agents/affiliate-withdrawals', token=admin_token)['items']
        assert any(item['id'] == wd['id'] for item in admin_wds), admin_wds
        admin_wd_qr = req('/admin/agents/affiliate-withdrawals/%s/payment-qr' % wd['id'], token=admin_token, expect_bytes=True)
        assert len(admin_wd_qr) > 100, len(admin_wd_qr)
        req('/admin/agents/affiliate-withdrawals/%s/complete' % wd['id'], method='POST', token=admin_token, payload={'payment_reference': 'E2E-PAID-' + uuid.uuid4().hex[:8]})
        ok('admin withdrawal complete', 'id=%s qr_bytes=%d' % (wd['id'], len(admin_wd_qr)))

    community = req('/admin/agents/affiliate-community', token=admin_token)
    community2 = req('/admin/agents/affiliate-community', method='PUT', token=admin_token, payload={
        'enabled': community['enabled'],
        'title': community['title'],
        'message': community['message'] + ' ',
        'revision': community['revision'],
    })
    assert community2['revision'] >= community['revision'], community2
    body2, headers2 = multipart_png_body(ROOT / 'frontend/public/favicon.png')
    community3 = req('/admin/agents/affiliate-community/qr', method='POST', token=admin_token, raw=body2, headers=headers2)
    assert community3['has_qr_code'] is True, community3
    comm_qr = req('/admin/agents/affiliate-community/qr', token=admin_token, expect_bytes=True)
    assert len(comm_qr) > 100, len(comm_qr)
    ok('admin community settings + QR', 'revision=%s qr_bytes=%d' % (community3['revision'], len(comm_qr)))

    print('\nSUMMARY affiliate e2e passed: %d checks' % len(results))


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Lightweight GEO/SEO technical audit for laoshirenai public pages.

The script intentionally uses only Python stdlib and never reads secrets. It checks
crawlability signals that should be valid before looking at Search Console/GA4:
HTTP status, title, description, canonical, JSON-LD, visible static HTML, llms.txt,
sitemap coverage, and soft-404 behavior.
"""

from __future__ import annotations

import argparse
import datetime as dt
import html
import json
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Iterable

DEFAULT_BASE_URL = "https://laoshirenai.com"
USER_AGENT = "laoshirenai-seo-geo-audit/1.0 (+https://laoshirenai.com)"


@dataclass
class PageAudit:
    url: str
    status: int | None
    title: str
    description: str
    canonical: str
    robots: str
    ld_json_count: int
    visible_chars: int
    h1_count: int
    error: str = ""


def fetch(url: str, timeout: int = 10) -> tuple[int | None, str, dict[str, str], str]:
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8", "replace")
            headers = {k.lower(): v for k, v in response.headers.items()}
            return response.status, body, headers, ""
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", "replace")
        headers = {k.lower(): v for k, v in exc.headers.items()}
        return exc.code, body, headers, ""
    except Exception as exc:  # noqa: BLE001 - diagnostic script
        return None, "", {}, f"{type(exc).__name__}: {exc}"


def tag_content(pattern: str, source: str) -> str:
    match = re.search(pattern, source, re.I | re.S)
    return html.unescape(match.group(1).strip()) if match else ""


def visible_text_length(source: str) -> int:
    body_match = re.search(r"<body[^>]*>(.*?)</body>", source, re.I | re.S)
    body = body_match.group(1) if body_match else source
    body = re.sub(r"<(script|style|svg)[\s\S]*?</\1>", " ", body, flags=re.I)
    body = re.sub(r"<[^>]+>", " ", body)
    body = html.unescape(body)
    body = re.sub(r"\s+", "", body)
    return len(body)


def audit_page(url: str, timeout: int = 10) -> PageAudit:
    status, body, _headers, error = fetch(url, timeout=timeout)
    return PageAudit(
        url=url,
        status=status,
        title=tag_content(r"<title[^>]*>(.*?)</title>", body),
        description=tag_content(r"<meta\s+name=[\"']description[\"'][^>]*content=[\"'](.*?)[\"']", body),
        canonical=tag_content(r"<link\s+rel=[\"']canonical[\"'][^>]*href=[\"'](.*?)[\"']", body),
        robots=tag_content(r"<meta\s+name=[\"']robots[\"'][^>]*content=[\"'](.*?)[\"']", body),
        ld_json_count=len(re.findall(r"application/ld\+json", body, re.I)),
        visible_chars=visible_text_length(body),
        h1_count=len(re.findall(r"<h1[\s>]", body, re.I)),
        error=error,
    )


def sitemap_urls(base_url: str, timeout: int = 10) -> list[str]:
    status, body, _headers, error = fetch(urllib.parse.urljoin(base_url, "/sitemap.xml"), timeout=timeout)
    if status != 200 or error:
        raise RuntimeError(f"failed to fetch sitemap: status={status} error={error}")
    return re.findall(r"<loc>(.*?)</loc>", body)


def audit_pages(urls: list[str], timeout: int, workers: int) -> list[PageAudit]:
    if workers <= 1:
        return [audit_page(url, timeout=timeout) for url in urls]
    with ThreadPoolExecutor(max_workers=workers) as executor:
        return list(executor.map(lambda url: audit_page(url, timeout=timeout), urls))


def summarize(pages: list[PageAudit], soft_404: PageAudit, llms_status: int | None) -> str:
    checked_at = dt.datetime.now(dt.timezone.utc).astimezone().isoformat(timespec="seconds")
    failures = [p for p in pages if p.status != 200 or p.error]
    empty_body = [p for p in pages if p.visible_chars < 100]
    missing_title = [p for p in pages if not p.title]
    missing_description = [p for p in pages if not p.description]
    missing_canonical = [p for p in pages if not p.canonical]
    missing_schema = [p for p in pages if p.ld_json_count == 0]
    canonical_mismatch = [p for p in pages if p.canonical and p.canonical.rstrip("/") != p.url.rstrip("/")]

    lines = [
        "# 老实人AI GEO/SEO 技术体检",
        "",
        f"生成时间：{checked_at}",
        "",
        "## 汇总",
        "",
        f"- sitemap 页面数：{len(pages)}",
        f"- HTTP 非 200 / 抓取失败：{len(failures)}",
        f"- 原始 HTML 正文过短（<100 字符）：{len(empty_body)}",
        f"- 缺 title：{len(missing_title)}",
        f"- 缺 description：{len(missing_description)}",
        f"- 缺 canonical：{len(missing_canonical)}",
        f"- canonical 不匹配：{len(canonical_mismatch)}",
        f"- 缺 JSON-LD：{len(missing_schema)}",
        f"- llms.txt 状态：{llms_status}",
        f"- soft-404 测试状态：{soft_404.status} / robots={soft_404.robots or '-'}",
        "",
        "## P0 页面",
        "",
    ]

    p0_paths = [
        "/docs/claude-code-china-guide",
        "/docs/codex-china-guide",
        "/docs/codex-no-api-key-guide",
        "/docs/codex-custom-api-guide",
    ]
    by_path = {urllib.parse.urlparse(p.url).path: p for p in pages}
    for path in p0_paths:
        page = by_path.get(path)
        if not page:
            lines.append(f"- {path}: MISSING")
            continue
        lines.append(
            f"- {path}: status={page.status}, visible_chars={page.visible_chars}, "
            f"h1={page.h1_count}, ld_json={page.ld_json_count}, title={page.title}"
        )

    lines.extend(["", "## 问题明细", ""])
    for label, bucket in [
        ("抓取失败", failures),
        ("正文过短", empty_body),
        ("缺 title", missing_title),
        ("缺 description", missing_description),
        ("缺 canonical", missing_canonical),
        ("canonical 不匹配", canonical_mismatch),
        ("缺 JSON-LD", missing_schema),
    ]:
        lines.append(f"### {label}")
        if not bucket:
            lines.append("- 无")
        else:
            for page in bucket[:30]:
                lines.append(f"- {page.url} status={page.status} visible_chars={page.visible_chars} error={page.error}")
        lines.append("")

    lines.extend([
        "## JSON 快照",
        "",
        "```json",
        json.dumps({"pages": [asdict(p) for p in pages], "soft404": asdict(soft_404)}, ensure_ascii=False, indent=2),
        "```",
        "",
    ])
    return "\n".join(lines)


def main(argv: Iterable[str]) -> int:
    parser = argparse.ArgumentParser(description="Audit laoshirenai GEO/SEO crawlability")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL)
    parser.add_argument("--limit", type=int, default=0, help="limit sitemap URLs for quick checks")
    parser.add_argument("--timeout", type=int, default=10, help="per-request timeout in seconds")
    parser.add_argument("--workers", type=int, default=8, help="parallel page fetches")
    parser.add_argument("--out", type=Path)
    args = parser.parse_args(list(argv))

    base_url = args.base_url.rstrip("/")
    urls = sitemap_urls(base_url, timeout=args.timeout)
    if args.limit:
        urls = urls[: args.limit]

    pages = audit_pages(urls, timeout=args.timeout, workers=args.workers)
    soft_404 = audit_page(urllib.parse.urljoin(base_url + "/", "docs/__seo_geo_missing_test__"), timeout=args.timeout)
    llms_status, _llms_body, _llms_headers, _llms_error = fetch(urllib.parse.urljoin(base_url, "/llms.txt"), timeout=args.timeout)
    report = summarize(pages, soft_404, llms_status)

    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(report, encoding="utf-8")
        print(args.out)
    else:
        print(report)

    hard_fail = any(p.status != 200 for p in pages) or any(p.visible_chars < 100 for p in pages)
    hard_fail = hard_fail or soft_404.status == 200 or "noindex" not in soft_404.robots
    return 1 if hard_fail else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

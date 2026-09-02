#!/usr/bin/env python3
"""Render only customer commands backed by an explicit tested adapter."""

from __future__ import annotations

import argparse
import sys


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def render_workbuddy_windows(group_id: int, group_name: str) -> str:
    group = ps_quote(group_name)
    parts = [
        f"$groupId={group_id}",
        f"$groupName={group}",
        "$k=Read-Host \"请粘贴 $groupName API Key\"",
        "if([string]::IsNullOrWhiteSpace($k)){throw 'API Key 不能为空'}",
        "try{$catalog=Invoke-RestMethod -Uri 'https://api.laoshirenai.com/api/v1/public/model-pricing' -Method Get}catch{throw \"读取分组目录失败：$($_.Exception.Message)\"}",
        "$group=@($catalog.data.groups|Where-Object{[int64]$_.group_id -eq $groupId})[0]",
        "if($null -eq $group){throw \"找不到分组：$groupName（ID $groupId）\"}",
        "$groupIds=@($group.models|ForEach-Object{[string]$_.model}|Sort-Object -Unique)",
        "if($groupIds.Count -eq 0){throw \"分组没有公开可用模型：$groupName\"}",
        "try{$keyCatalog=Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{Authorization=\"Bearer $k\"} -Method Get}catch{throw \"读取 Key 可用模型失败：$($_.Exception.Message)\"}",
        "$keyIds=@($keyCatalog.data|ForEach-Object{[string]$_.id}|Sort-Object -Unique)",
        "$ids=@($groupIds|Where-Object{$keyIds -contains $_}|Sort-Object -Unique)",
        "if($ids.Count -ne $groupIds.Count){$missing=@($groupIds|Where-Object{$keyIds -notcontains $_});throw \"该 Key 无权调用分组全部模型，缺少：$($missing -join '、')\"}",
        "$d=Join-Path $HOME '.codebuddy'",
        "$p=Join-Path $d 'models.json'",
        "New-Item -ItemType Directory -Force -Path $d|Out-Null",
        "if(Test-Path $p){try{$c=Get-Content $p -Raw|ConvertFrom-Json}catch{throw \"现有 models.json 格式错误，未修改：$p\"}}else{$c=[pscustomobject]@{models=@()}}",
        "if(-not $c.PSObject.Properties['models']){$c|Add-Member -NotePropertyName models -NotePropertyValue @()}",
        "$prefix=\"老实人AI $groupName\"",
        "$old=@($c.models)",
        "$oldIds=@($old|Where-Object{$_.name -like \"$prefix*\"}|ForEach-Object{[string]$_.id})",
        "$new=@($ids|ForEach-Object{[pscustomobject]@{id=[string]$_;name=\"$prefix（$($_)）\";vendor='OpenAI';apiKey=$k;url='https://api.laoshirenai.com/v1/chat/completions';supportsToolCall=$true;supportsImages=$true;supportsReasoning=$true}})",
        "$c.models=@($old|Where-Object{($oldIds -notcontains $_.id)-and($ids -notcontains $_.id)})+$new",
        "if($c.PSObject.Properties['availableModels'] -and @($c.availableModels).Count -gt 0){$c.availableModels=@($c.availableModels|Where-Object{($oldIds -notcontains $_)-and($ids -notcontains $_)})+$ids}",
        "$json=$c|ConvertTo-Json -Depth 20",
        "$unchanged=(Test-Path $p)-and([IO.File]::ReadAllText($p)-eq $json)",
        "if($unchanged){Write-Host \"已是最新配置，可选模型：$($ids -join '、')\"}else{if(Test-Path $p){Copy-Item $p \"$p.bak-$(Get-Date -Format yyyyMMddHHmmssfff)\"};$tmp=\"$p.tmp\";[IO.File]::WriteAllText($tmp,$json,[Text.UTF8Encoding]::new($false));if(Test-Path $p){[IO.File]::Replace($tmp,$p,$null)}else{[IO.File]::Move($tmp,$p)};Write-Host \"接入完成，可选模型：$($ids -join '、')\"}",
    ]
    return ";".join(parts)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--client", required=True)
    parser.add_argument("--group-id", required=True, type=int)
    parser.add_argument("--group-name", required=True)
    parser.add_argument("--os", required=True, dest="os_name")
    return parser.parse_args()


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    if hasattr(sys.stderr, "reconfigure"):
        sys.stderr.reconfigure(encoding="utf-8")
    args = parse_args()
    key = (args.client.casefold(), args.os_name.casefold())
    if key == ("workbuddy", "windows"):
        print(render_workbuddy_windows(args.group_id, args.group_name))
        return 0
    print(
        f"no tested one-line adapter for {args.client!r} on {args.os_name!r}",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

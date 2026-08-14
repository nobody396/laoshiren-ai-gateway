$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$DefaultProviderId = 'openai-official-pro'
$DefaultProviderName = 'OpenAI Official Pro'

$CcSwitchDb = if ($env:CC_SWITCH_DB) { $env:CC_SWITCH_DB } else { Join-Path $HOME '.cc-switch\cc-switch.db' }
$CodexAuthPath = if ($env:CODEX_AUTH_PATH) { $env:CODEX_AUTH_PATH } else { Join-Path $HOME '.codex\auth.json' }
$CodexConfigPath = if ($env:CODEX_CONFIG_PATH) { $env:CODEX_CONFIG_PATH } else { Join-Path $HOME '.codex\config.toml' }
$ProviderId = if ($env:CCS_OPENAI_PROVIDER_ID) { $env:CCS_OPENAI_PROVIDER_ID } else { $DefaultProviderId }
$ProviderName = if ($env:CCS_OPENAI_PROVIDER_NAME) { $env:CCS_OPENAI_PROVIDER_NAME } else { $DefaultProviderName }

function Write-Info {
  param([string]$Message)
  Write-Host "[INFO] $Message"
}

function Stop-Script {
  param([string]$Message)
  Write-Error "[ERROR] $Message"
  exit 1
}

function Get-PythonRunner {
  if (Get-Command py -ErrorAction SilentlyContinue) {
    return @{ Command = 'py'; Args = @('-3', '-') }
  }
  if (Get-Command python3 -ErrorAction SilentlyContinue) {
    return @{ Command = 'python3'; Args = @('-') }
  }
  if (Get-Command python -ErrorAction SilentlyContinue) {
    return @{ Command = 'python'; Args = @('-') }
  }
  return $null
}

if (-not (Test-Path -LiteralPath $CcSwitchDb)) {
  Stop-Script "未找到 CC Switch 数据库: $CcSwitchDb"
}
if (-not (Test-Path -LiteralPath $CodexAuthPath)) {
  Stop-Script "未找到 Codex 登录文件: $CodexAuthPath。请先在 Codex 中使用 ChatGPT/OpenAI 官方账号登录。"
}

$Runner = Get-PythonRunner
if ($null -eq $Runner) {
  Stop-Script "缺少 Python。请先安装 Python 3，或在 CC Switch 中手动添加 OpenAI Official Provider。"
}

$BackupDir = Join-Path $HOME '.cc-switch\backups'
New-Item -ItemType Directory -Force -Path $BackupDir | Out-Null
$BackupPath = Join-Path $BackupDir ("cc-switch-before-openai-official-{0}.db" -f (Get-Date -Format 'yyyyMMdd-HHmmss'))
Copy-Item -LiteralPath $CcSwitchDb -Destination $BackupPath -Force
Write-Info "已备份 CC Switch 数据库: $BackupPath"

$env:CC_SWITCH_DB = $CcSwitchDb
$env:CODEX_AUTH_PATH = $CodexAuthPath
$env:CODEX_CONFIG_PATH = $CodexConfigPath
$env:PROVIDER_ID = $ProviderId
$env:PROVIDER_NAME = $ProviderName

$PythonScript = @'
import json
import os
import sqlite3
import time
from pathlib import Path

db_path = Path(os.environ["CC_SWITCH_DB"]).expanduser()
auth_path = Path(os.environ["CODEX_AUTH_PATH"]).expanduser()
config_path = Path(os.environ["CODEX_CONFIG_PATH"]).expanduser()
provider_id = os.environ["PROVIDER_ID"].strip() or "openai-official-pro"
provider_name = os.environ["PROVIDER_NAME"].strip() or "OpenAI Official Pro"

def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8") if path.exists() else ""

def read_json(path: Path) -> dict:
    try:
        data = json.loads(read_text(path))
    except Exception as exc:
        raise SystemExit(f"Codex 登录文件不是有效 JSON: {path}: {exc}")
    if not isinstance(data, dict):
        raise SystemExit(f"Codex 登录文件格式不正确: {path}")
    return data

def is_official_auth(auth: dict) -> bool:
    if str(auth.get("auth_mode", "")).lower() == "chatgpt":
        return True
    tokens = auth.get("tokens")
    return isinstance(tokens, dict) and any(tokens.get(k) for k in ("access_token", "refresh_token", "id_token"))

auth = read_json(auth_path)
if not is_official_auth(auth):
    raise SystemExit(
        "当前 ~/.codex/auth.json 看起来不是 ChatGPT/OpenAI 官方登录态。\n"
        "请先运行 codex 并选择 Sign in with ChatGPT，或在 Codex App 中登录官方订阅。"
    )

config_text = read_text(config_path).strip()
now_ms = int(time.time() * 1000)

conn = sqlite3.connect(db_path)
conn.row_factory = sqlite3.Row
try:
    cur = conn.cursor()
    default_row = cur.execute(
        "SELECT settings_config, meta, cost_multiplier, limit_daily_usd, limit_monthly_usd, icon, icon_color "
        "FROM providers WHERE app_type = 'codex' AND id = 'default'"
    ).fetchone()

    settings_config = None
    meta = {"commonConfigEnabled": True}
    cost_multiplier = "1.0"
    limit_daily_usd = None
    limit_monthly_usd = None
    icon = None
    icon_color = None

    if default_row:
        try:
            default_config = json.loads(default_row["settings_config"])
        except Exception:
            default_config = {}
        default_auth = default_config.get("auth") if isinstance(default_config, dict) else None
        if isinstance(default_auth, dict) and is_official_auth(default_auth):
            default_config["auth"] = auth
            if config_text:
                default_config["config"] = config_text
            settings_config = json.dumps(default_config, ensure_ascii=False, separators=(",", ":"))
            try:
                parsed_meta = json.loads(default_row["meta"] or "{}")
                if isinstance(parsed_meta, dict):
                    meta = parsed_meta
            except Exception:
                pass
            cost_multiplier = default_row["cost_multiplier"] or "1.0"
            limit_daily_usd = default_row["limit_daily_usd"]
            limit_monthly_usd = default_row["limit_monthly_usd"]
            icon = default_row["icon"]
            icon_color = default_row["icon_color"]

    if settings_config is None:
        settings_config = json.dumps(
            {
                "auth": auth,
                "config": config_text,
            },
            ensure_ascii=False,
            separators=(",", ":"),
        )

    sort_index_row = cur.execute(
        "SELECT COALESCE(MAX(sort_index), 0) AS max_sort FROM providers WHERE app_type = 'codex'"
    ).fetchone()
    sort_index = int(sort_index_row["max_sort"] or 0) + 1

    notes = (
        "ChatGPT/OpenAI 官方订阅 Provider，由本机 Codex 官方登录态生成。\n\n"
        "注意：CC Switch 卡片显示的是已用百分比；Codex App 菜单显示的是剩余百分比。"
    )

    cur.execute("BEGIN")
    cur.execute("UPDATE providers SET is_current = 0 WHERE app_type = 'codex'")
    cur.execute(
        """
        INSERT INTO providers (
            id, app_type, name, settings_config, website_url, category, created_at, sort_index,
            notes, icon, icon_color, meta, is_current, in_failover_queue, cost_multiplier,
            limit_daily_usd, limit_monthly_usd, provider_type
        ) VALUES (?, 'codex', ?, ?, 'https://chatgpt.com', 'official', ?, ?, ?, ?, ?, ?, 1, 0, ?, ?, ?, NULL)
        ON CONFLICT(id, app_type) DO UPDATE SET
            name = excluded.name,
            settings_config = excluded.settings_config,
            website_url = excluded.website_url,
            category = excluded.category,
            notes = excluded.notes,
            icon = excluded.icon,
            icon_color = excluded.icon_color,
            meta = excluded.meta,
            is_current = 1,
            in_failover_queue = 0,
            cost_multiplier = excluded.cost_multiplier,
            limit_daily_usd = excluded.limit_daily_usd,
            limit_monthly_usd = excluded.limit_monthly_usd
        """,
        (
            provider_id,
            provider_name,
            settings_config,
            now_ms,
            sort_index,
            notes,
            icon,
            icon_color,
            json.dumps(meta, ensure_ascii=False, separators=(",", ":")),
            cost_multiplier,
            limit_daily_usd,
            limit_monthly_usd,
        ),
    )
    conn.commit()
finally:
    conn.close()

settings_path = db_path.parent / "settings.json"
if settings_path.exists():
    try:
        settings = json.loads(settings_path.read_text(encoding="utf-8"))
        if isinstance(settings, dict):
            settings["currentProviderCodex"] = provider_id
            settings_path.write_text(json.dumps(settings, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    except Exception:
        pass

print(f"[INFO] 已保存官方订阅 Provider: {provider_name}")
print("[INFO] 请重启或打开 CC Switch，在 Codex 页面查看并切换。")
'@

$PythonCommand = $Runner['Command']
$PythonArgs = $Runner['Args']
$PythonScript | & $PythonCommand @PythonArgs

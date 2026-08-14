#!/usr/bin/env bash
set -euo pipefail

SCRIPT_VERSION="1.2.2"
MINIMUM_VERSION="3.16.5"
RELEASE_URL="https://github.com/farion1231/cc-switch/releases/latest"
RELEASE_API_URL="https://api.github.com/repos/farion1231/cc-switch/releases/latest"
MIRROR_MANIFEST_URL="https://laoshirenai.com/api/v1/public-downloads/cc-switch/latest.json"
MIRROR_PACKAGE_PREFIX="https://laoshirenai.com/downloads/cc-switch/"
BUNDLE_ID="com.ccswitch.desktop"
OFFICIAL_TEAM_ID="R8UR22V2F9"
LATEST_VERSION=""
LATEST_ASSET_URL=""
LATEST_ASSET_SHA256=""
LATEST_ASSET_SOURCE=""
INSTALLED_APP_PATH=""
TEMP_PATHS=()

info() {
  printf '\033[36m[CC Switch]\033[0m %s\n' "$1"
}

ok() {
  printf '\033[32m[完成]\033[0m %s\n' "$1"
}

problem() {
  printf '\033[33m[需要处理]\033[0m %s\n' "$1"
}

cleanup() {
  local temp_path
  for temp_path in "${TEMP_PATHS[@]:-}"; do
    if [[ -n "$temp_path" && -e "$temp_path" ]]; then
      /bin/rm -rf "$temp_path"
    fi
  done
}
trap cleanup EXIT HUP INT TERM

open_release_page() {
  if [[ "${CCS_DIAGNOSTIC_NO_OPEN:-}" == "1" ]]; then
    printf '下载地址: %s\n' "$RELEASE_URL"
    return
  fi

  /usr/bin/open "$RELEASE_URL" >/dev/null 2>&1 || printf '请打开官方下载页: %s\n' "$RELEASE_URL"
  printf '自动安装没有完成，已打开 CC Switch 官方下载页作为兜底。\n'
}

version_is_older() {
  /usr/bin/awk -v current="$1" -v minimum="$2" '
    BEGIN {
      split(current, a, ".");
      split(minimum, b, ".");
      for (i = 1; i <= 4; i++) {
        av = a[i] + 0;
        bv = b[i] + 0;
        if (av < bv) exit 0;
        if (av > bv) exit 1;
      }
      exit 1;
    }
  '
}

find_cc_switch_app() {
  local candidate

  for candidate in \
    "/Applications/CC Switch.app" \
    "$HOME/Applications/CC Switch.app" \
    "$HOME/Downloads/CC Switch.app" \
    "$HOME/Desktop/CC Switch.app"; do
    if [[ -d "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return
    fi
  done

  if command -v mdfind >/dev/null 2>&1; then
    while IFS= read -r candidate; do
      if [[ "$candidate" == *.app && -d "$candidate" ]]; then
        printf '%s\n' "$candidate"
        return
      fi
    done < <(/usr/bin/mdfind "kMDItemCFBundleIdentifier == '$BUNDLE_ID'" 2>/dev/null)
  fi
}

is_cc_switch_running() {
  [[ "$(/usr/bin/osascript -e 'application id "com.ccswitch.desktop" is running' 2>/dev/null || printf 'false')" == "true" ]]
}

wait_for_cc_switch_exit() {
  if ! is_cc_switch_running; then
    return
  fi

  info "升级前正在安全退出 CC Switch..."
  /usr/bin/osascript -e 'tell application id "com.ccswitch.desktop" to quit' >/dev/null 2>&1 || true
  for _ in $(/usr/bin/jot 30 1 30); do
    if ! is_cc_switch_running; then
      return
    fi
    /bin/sleep 0.5
  done

  problem "CC Switch 仍在后台运行。为了保护本地代理和配置，脚本不会强制结束进程。"
  printf '请在 Dock 或菜单栏找到 CC Switch，右键选择“退出”，然后回到终端按回车。\n'
  IFS= read -r _
  for _ in $(/usr/bin/jot 40 1 40); do
    if ! is_cc_switch_running; then
      return
    fi
    /bin/sleep 0.5
  done

  return 1
}

find_brew() {
  local candidate
  for candidate in "$(command -v brew 2>/dev/null || true)" /opt/homebrew/bin/brew /usr/local/bin/brew; do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return
    fi
  done
}

fetch_latest_macos_mirror() {
  local metadata asset_count index name tag
  metadata="$(/usr/bin/mktemp)"
  TEMP_PATHS+=("$metadata")

  info "正在查询老实人 AI 本站缓存的 CC Switch 最新版..."
  /usr/bin/curl -fL --retry 2 --connect-timeout 15 \
    -H "User-Agent: laoshirenai-cc-switch-diagnostic/$SCRIPT_VERSION" \
    "$MIRROR_MANIFEST_URL" -o "$metadata"

  tag="$(/usr/bin/plutil -extract version raw "$metadata" 2>/dev/null || true)"
  LATEST_VERSION="${tag#v}"
  if [[ ! "$LATEST_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.][0-9]+)?$ ]]; then
    return 1
  fi

  asset_count="$(/usr/bin/plutil -extract assets raw "$metadata" 2>/dev/null || true)"
  if [[ ! "$asset_count" =~ ^[0-9]+$ ]]; then
    return 1
  fi

  for ((index = 0; index < asset_count; index++)); do
    name="$(/usr/bin/plutil -extract "assets.$index.name" raw "$metadata" 2>/dev/null || true)"
    if [[ "$name" != "CC-Switch-v${LATEST_VERSION}-macOS.tar.gz" ]]; then
      continue
    fi

    LATEST_ASSET_URL="$(/usr/bin/plutil -extract "assets.$index.download_url" raw "$metadata" 2>/dev/null || true)"
    LATEST_ASSET_SHA256="$(/usr/bin/plutil -extract "assets.$index.sha256" raw "$metadata" 2>/dev/null || true)"
    break
  done

  if [[ "$LATEST_ASSET_URL" != "$MIRROR_PACKAGE_PREFIX"* ]] || \
     [[ ! "$LATEST_ASSET_SHA256" =~ ^[0-9a-fA-F]{64}$ ]]; then
    return 1
  fi
  LATEST_ASSET_SOURCE="老实人 AI 本站缓存"
}

fetch_latest_macos_official() {
  local metadata asset_count index name digest tag
  metadata="$(/usr/bin/mktemp)"
  TEMP_PATHS+=("$metadata")

  info "本站缓存暂不可用，正在查询 CC Switch 官方 GitHub 作为兜底..."
  /usr/bin/curl -fL --retry 3 --connect-timeout 20 \
    -H 'Accept: application/vnd.github+json' \
    -H "User-Agent: laoshirenai-cc-switch-diagnostic/$SCRIPT_VERSION" \
    -H 'X-GitHub-Api-Version: 2022-11-28' \
    "$RELEASE_API_URL" -o "$metadata"

  tag="$(/usr/bin/plutil -extract tag_name raw "$metadata" 2>/dev/null || true)"
  LATEST_VERSION="${tag#v}"
  if [[ ! "$LATEST_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.][0-9]+)?$ ]]; then
    problem "官方版本信息格式不正确。"
    return 1
  fi

  asset_count="$(/usr/bin/plutil -extract assets raw "$metadata" 2>/dev/null || true)"
  if [[ ! "$asset_count" =~ ^[0-9]+$ ]]; then
    problem "无法读取官方发布资源。"
    return 1
  fi

  for ((index = 0; index < asset_count; index++)); do
    name="$(/usr/bin/plutil -extract "assets.$index.name" raw "$metadata" 2>/dev/null || true)"
    if [[ "$name" != "CC-Switch-v${LATEST_VERSION}-macOS.tar.gz" ]]; then
      continue
    fi

    LATEST_ASSET_URL="$(/usr/bin/plutil -extract "assets.$index.browser_download_url" raw "$metadata" 2>/dev/null || true)"
    digest="$(/usr/bin/plutil -extract "assets.$index.digest" raw "$metadata" 2>/dev/null || true)"
    LATEST_ASSET_SHA256="${digest#sha256:}"
    break
  done

  if [[ ! "$LATEST_ASSET_URL" =~ ^https://github\.com/farion1231/cc-switch/releases/download/ ]] || \
     [[ ! "$LATEST_ASSET_SHA256" =~ ^[0-9a-fA-F]{64}$ ]]; then
    problem "官方发布包缺少可验证的下载地址或 SHA-256 摘要。"
    return 1
  fi
  LATEST_ASSET_SOURCE="CC Switch 官方 GitHub"
}

fetch_latest_macos_release() {
  if fetch_latest_macos_mirror; then
    return
  fi
  problem "本站缓存暂时不可用，将使用官方 GitHub 兜底。"
  fetch_latest_macos_official
}

verify_official_app() {
  local app_path="$1" detected_bundle team_id

  [[ -f "$app_path/Contents/Info.plist" ]] || return 1
  detected_bundle="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app_path/Contents/Info.plist" 2>/dev/null || true)"
  [[ "$detected_bundle" == "$BUNDLE_ID" ]] || return 1

  /usr/bin/codesign --verify --deep --strict "$app_path" >/dev/null 2>&1 || return 1
  team_id="$(/usr/bin/codesign -dv --verbose=4 "$app_path" 2>&1 | /usr/bin/awk -F= '/^TeamIdentifier=/{print $2; exit}')"
  [[ "$team_id" == "$OFFICIAL_TEAM_ID" ]] || return 1
  /usr/sbin/spctl --assess --type execute "$app_path" >/dev/null 2>&1 || return 1
}

install_latest_macos_app() {
  local requested_target="${1:-}" brew_path work_dir archive staged_app actual_sha expected_sha target backup old_app lsregister
  old_app=""

  brew_path="$(find_brew || true)"
  if [[ -n "$brew_path" ]] && "$brew_path" list --cask cc-switch >/dev/null 2>&1; then
    wait_for_cc_switch_exit || {
      problem "CC Switch 仍在运行，已停止升级。"
      return 1
    }
    info "检测到 Homebrew 安装，正在使用官方 cask 自动升级..."
    if "$brew_path" upgrade --cask cc-switch; then
      INSTALLED_APP_PATH="$(find_cc_switch_app || true)"
      if [[ -n "$INSTALLED_APP_PATH" ]] && verify_official_app "$INSTALLED_APP_PATH"; then
        ok "CC Switch 已通过 Homebrew 自动升级。"
        return
      fi
    fi
    problem "Homebrew 自动升级未完成，将改用官方签名安装包。"
  fi

  if [[ -z "$LATEST_VERSION" || -z "$LATEST_ASSET_URL" || -z "$LATEST_ASSET_SHA256" ]]; then
    fetch_latest_macos_release || return 1
  fi
  work_dir="$(/usr/bin/mktemp -d)"
  TEMP_PATHS+=("$work_dir")
  archive="$work_dir/CC-Switch-v${LATEST_VERSION}-macOS.tar.gz"

  info "正在从${LATEST_ASSET_SOURCE}下载 CC Switch $LATEST_VERSION..."
  /usr/bin/curl -fL --retry 3 --connect-timeout 20 \
    -H "User-Agent: laoshirenai-cc-switch-diagnostic/$SCRIPT_VERSION" \
    "$LATEST_ASSET_URL" -o "$archive"

  actual_sha="$(/usr/bin/shasum -a 256 "$archive" | /usr/bin/awk '{print $1}')"
  expected_sha="$(printf '%s' "$LATEST_ASSET_SHA256" | /usr/bin/tr '[:upper:]' '[:lower:]')"
  if [[ "$actual_sha" != "$expected_sha" ]]; then
    problem "安装包 SHA-256 校验失败，已停止安装。"
    return 1
  fi
  ok "${LATEST_ASSET_SOURCE}安装包 SHA-256 校验通过。"

  /usr/bin/tar -xzf "$archive" -C "$work_dir"
  staged_app="$work_dir/CC Switch.app"
  if ! verify_official_app "$staged_app"; then
    problem "官方应用的 Bundle ID、开发者签名或 Apple 公证校验失败。"
    return 1
  fi
  ok "Apple 签名与公证校验通过。"

  wait_for_cc_switch_exit || {
    problem "CC Switch 仍在运行，已停止升级。"
    return 1
  }

  case "$requested_target" in
    "/Applications/CC Switch.app"|"$HOME/Applications/CC Switch.app")
      target="$requested_target"
      ;;
    *)
      target="$HOME/Applications/CC Switch.app"
      old_app="$requested_target"
      ;;
  esac

  /bin/mkdir -p "$(/usr/bin/dirname "$target")"
  backup=""
  if [[ -d "$target" ]]; then
    backup="${target}.diagnostic-backup-$(/bin/date +%s)"
    if ! /bin/mv "$target" "$backup"; then
      if [[ "$target" != "$HOME/Applications/CC Switch.app" ]]; then
        old_app="$target"
        target="$HOME/Applications/CC Switch.app"
        /bin/mkdir -p "$HOME/Applications"
        backup=""
      else
        problem "无法替换 $target。"
        return 1
      fi
    fi
  fi

  if ! /usr/bin/ditto "$staged_app" "$target"; then
    if [[ -n "$backup" && -d "$backup" && ! -d "$target" ]]; then
      /bin/mv "$backup" "$target" || true
    fi
    problem "复制新版 CC Switch 失败。"
    return 1
  fi

  if ! verify_official_app "$target"; then
    /bin/rm -rf "$target"
    if [[ -n "$backup" && -d "$backup" ]]; then
      /bin/mv "$backup" "$target" || true
    fi
    problem "安装后的应用验证失败，已恢复旧版本。"
    return 1
  fi

  if [[ -n "$backup" && -d "$backup" ]]; then
    /bin/rm -rf "$backup"
  fi

  if [[ -n "$old_app" && -d "$old_app" ]]; then
    lsregister="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
    if [[ -x "$lsregister" ]]; then
      "$lsregister" -u "$old_app" >/dev/null 2>&1 || true
    fi
  fi

  INSTALLED_APP_PATH="$target"
  ok "CC Switch 已自动安装到 $INSTALLED_APP_PATH。"
}

if [[ "${CCS_DIAGNOSTIC_LIBRARY_ONLY:-}" == "1" ]]; then
  if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
    return 0
  fi
  exit 0
fi

printf '\nCC Switch 自动诊断修复 v%s\n' "$SCRIPT_VERSION"
printf '它只检查本机 CC Switch 版本和 ccswitch:// 协议，不会读取或上传 API Key。\n'
printf '缺失或版本过旧时，会优先从老实人 AI 本站缓存下载、校验并自动安装；本站不可用才访问官方 GitHub。\n\n'

if [[ "$(/usr/bin/uname -s)" != "Darwin" ]]; then
  printf '这条命令只适用于 macOS。Windows 请使用页面提供的 PowerShell 命令。\n' >&2
  exit 1
fi

info "正在查找已经安装的 CC Switch..."
APP_PATH="$(find_cc_switch_app || true)"
WAS_UPGRADED=0

if [[ -z "$APP_PATH" ]]; then
  problem "没有找到 CC Switch，将自动安装最新版。"
  if install_latest_macos_app ""; then
    APP_PATH="$INSTALLED_APP_PATH"
    WAS_UPGRADED=1
  else
    open_release_page
    printf '\n处理完成后可以直接关闭终端窗口。\n'
    exit 0
  fi
elif [[ ! -f "$APP_PATH/Contents/Info.plist" ]]; then
  problem "找到的应用不完整，将自动重新安装官方最新版。"
  if install_latest_macos_app "$APP_PATH"; then
    APP_PATH="$INSTALLED_APP_PATH"
    WAS_UPGRADED=1
  else
    open_release_page
    exit 0
  fi
else
  DETECTED_BUNDLE_ID="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$APP_PATH/Contents/Info.plist" 2>/dev/null || true)"
  if [[ "$DETECTED_BUNDLE_ID" != "$BUNDLE_ID" ]]; then
    problem "应用身份校验失败，将在当前用户目录安装官方最新版。"
    if install_latest_macos_app ""; then
      APP_PATH="$INSTALLED_APP_PATH"
      WAS_UPGRADED=1
    else
      open_release_page
      exit 0
    fi
  fi
fi

case "$APP_PATH" in
  /Applications/*|"$HOME/Applications/"*)
    ;;
  *)
    TARGET_APP="$HOME/Applications/CC Switch.app"
    info "检测到 CC Switch 还在下载目录，正在放入当前用户的应用程序目录..."
    wait_for_cc_switch_exit || {
      problem "请先退出 CC Switch 后重新执行本命令。"
      exit 0
    }
    /bin/mkdir -p "$HOME/Applications"
    /usr/bin/ditto "$APP_PATH" "$TARGET_APP"
    APP_PATH="$TARGET_APP"
    ok "应用已放到 $APP_PATH"
    ;;
esac

VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP_PATH/Contents/Info.plist" 2>/dev/null || true)"
if [[ -n "$VERSION" ]]; then
  printf '检测到 CC Switch: %s\n' "$APP_PATH"
  printf '当前版本: %s\n' "$VERSION"
  if fetch_latest_macos_release; then
    if version_is_older "$VERSION" "$LATEST_VERSION"; then
      problem "检测到最新版 $LATEST_VERSION，将自动升级。"
      if install_latest_macos_app "$APP_PATH"; then
        APP_PATH="$INSTALLED_APP_PATH"
        VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP_PATH/Contents/Info.plist" 2>/dev/null || true)"
        WAS_UPGRADED=1
      elif version_is_older "$VERSION" "$MINIMUM_VERSION"; then
        open_release_page
        exit 0
      else
        problem "自动升级暂未完成，将继续修复 Deep Link。"
      fi
    fi
  elif version_is_older "$VERSION" "$MINIMUM_VERSION"; then
    problem "版本低于 $MINIMUM_VERSION，但本站缓存与官方源均不可用。"
    open_release_page
    exit 0
  else
    problem "暂时无法检查最新版，将继续修复 Deep Link。"
  fi
else
  problem "无法读取版本号，将先修复 Deep Link。若导入仍失败，可重新运行本命令自动安装最新版。"
fi

info "正在重新注册 ccswitch:// 协议..."
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
if [[ -x "$LSREGISTER" ]]; then
  "$LSREGISTER" -f "$APP_PATH"
fi

if [[ "${CCS_DIAGNOSTIC_NO_LAUNCH:-}" != "1" ]]; then
  /usr/bin/open "$APP_PATH" --args --register-protocol
  if [[ "$WAS_UPGRADED" == "1" ]]; then
    ok "最新版 CC Switch 已重新打开。"
  fi
fi

ok "Deep Link 已重新注册。"
ok "诊断、升级与修复完成。请回到网页，重新点击一键导入。"
printf '\n现在可以直接关闭终端窗口。\n'

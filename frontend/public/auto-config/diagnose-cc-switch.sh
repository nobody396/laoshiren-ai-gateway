#!/usr/bin/env bash
set -euo pipefail

SCRIPT_VERSION="1.0.0"
MINIMUM_VERSION="3.16.5"
RELEASE_URL="https://github.com/farion1231/cc-switch/releases/latest"
BUNDLE_ID="com.ccswitch.desktop"

info() {
  printf '\033[36m[CC Switch]\033[0m %s\n' "$1"
}

ok() {
  printf '\033[32m[完成]\033[0m %s\n' "$1"
}

problem() {
  printf '\033[33m[需要处理]\033[0m %s\n' "$1"
}

open_release_page() {
  if [[ "${CCS_DIAGNOSTIC_NO_OPEN:-}" == "1" ]]; then
    printf '下载地址: %s\n' "$RELEASE_URL"
    return
  fi

  /usr/bin/open "$RELEASE_URL" >/dev/null 2>&1 || printf '请打开下载页: %s\n' "$RELEASE_URL"
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

printf '\nCC Switch 自动诊断修复 v%s\n' "$SCRIPT_VERSION"
printf '它只检查本机 CC Switch 版本和 ccswitch:// 协议，不会读取或上传 API Key。\n\n'

if [[ "$(/usr/bin/uname -s)" != "Darwin" ]]; then
  printf '这条命令只适用于 macOS。Windows 请使用页面提供的 PowerShell 命令。\n' >&2
  exit 1
fi

info "正在查找已经安装的 CC Switch..."
APP_PATH="$(find_cc_switch_app || true)"

if [[ -z "$APP_PATH" ]]; then
  problem "没有找到 CC Switch。"
  printf '已为你打开官方下载页。请下载 macOS.dmg，把 CC Switch 拖进“应用程序”，打开一次后再运行本命令。\n'
  open_release_page
  printf '\n处理完成后可以直接关闭终端窗口。\n'
  exit 0
fi

if [[ ! -f "$APP_PATH/Contents/Info.plist" ]]; then
  problem "找到的应用不完整: $APP_PATH"
  printf '请从官方下载页重新下载 macOS.dmg。\n'
  open_release_page
  exit 0
fi

DETECTED_BUNDLE_ID="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$APP_PATH/Contents/Info.plist" 2>/dev/null || true)"
if [[ "$DETECTED_BUNDLE_ID" != "$BUNDLE_ID" ]]; then
  problem "应用身份校验失败，不会注册这个应用。"
  printf '请从官方下载页重新安装 CC Switch。\n'
  open_release_page
  exit 0
fi

case "$APP_PATH" in
  /Applications/*|"$HOME/Applications/"*)
    ;;
  *)
    TARGET_APP="$HOME/Applications/CC Switch.app"
    info "检测到 CC Switch 还在下载目录，正在复制到当前用户的应用程序目录..."
    /bin/mkdir -p "$HOME/Applications"
    if [[ ! -d "$TARGET_APP" ]]; then
      /usr/bin/ditto "$APP_PATH" "$TARGET_APP"
    fi
    APP_PATH="$TARGET_APP"
    ok "应用已放到 $APP_PATH"
    ;;
esac

VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP_PATH/Contents/Info.plist" 2>/dev/null || true)"
NEEDS_UPGRADE=0
if [[ -n "$VERSION" ]]; then
  printf '检测到 CC Switch: %s\n' "$APP_PATH"
  printf '当前版本: %s\n' "$VERSION"
  if version_is_older "$VERSION" "$MINIMUM_VERSION"; then
    NEEDS_UPGRADE=1
    problem "版本低于 $MINIMUM_VERSION，部分一键导入功能可能无法使用。"
  fi
else
  problem "无法读取版本号，将先修复 Deep Link。若导入仍失败，请升级最新版。"
fi

info "正在重新注册 ccswitch:// 协议..."
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
if [[ -x "$LSREGISTER" ]]; then
  "$LSREGISTER" -f "$APP_PATH"
fi

if ! /usr/bin/open -a "CC Switch" --args --register-protocol; then
  /usr/bin/open "$APP_PATH" --args --register-protocol
fi

ok "Deep Link 已重新注册。"

if [[ "$NEEDS_UPGRADE" == "1" ]]; then
  problem "Deep Link 已修复，但 CC Switch 版本太旧。"
  printf '已打开官方下载页。请下载 macOS.dmg 覆盖安装，然后回到网页重新点击“一键导入”。\n'
  open_release_page
else
  ok "诊断修复完成。请回到网页，重新点击一键导入。"
fi

printf '\n现在可以直接关闭终端窗口。\n'

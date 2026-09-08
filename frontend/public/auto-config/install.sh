#!/usr/bin/env bash

# Generated catalog metadata includes values consumed by the PowerShell twin
# but intentionally retained here for cross-installer fingerprint parity.
# shellcheck disable=SC2034

set -euo pipefail

# BEGIN GENERATED MODEL CATALOG
SCRIPT_VERSION='0.7.21'
CATALOG_OPENAI_DEFAULT_MODEL='gpt-5.6-sol'
CATALOG_OPENAI_CONTEXT_WINDOW=272000
CATALOG_OPENAI_AUTO_COMPACT_TOKEN_LIMIT=258000
CATALOG_ANTHROPIC_DEFAULT_MODEL='claude-opus-5'
CATALOG_GROK_DEFAULT_MODEL='grok-4.6'
CATALOG_GROK_DEFAULT_DISPLAY_NAME='Grok 4.6'
CATALOG_GROK_DEFAULT_CONTEXT_WINDOW=500000
CATALOG_GROK_MANAGED_MODELS_JSON='[{"id":"grok-4.5","display_name":"Grok 4.5","context_window":500000},{"id":"grok-4.6","display_name":"Grok 4.6","context_window":500000}]'
CATALOG_GEMINI_DEFAULT_MODEL='gemini-3.7-flash'
CATALOG_GEMINI_MANAGED_MODELS='gemini-3.1-pro gemini-3.7-flash gemini-3.7-flash-high gemini-3.8-flash'
CATALOG_MODEL_REASONING_JSON='{"claude-fable-5-1":["low","medium","high","xhigh","max"],"claude-fable-5":["low","medium","high","xhigh","max"],"claude-haiku-4-5":[],"claude-opus-4-5":["low","medium","high","max"],"claude-opus-4-6":["low","medium","high","max"],"claude-opus-4-7":["low","medium","high","xhigh","max"],"claude-opus-4-8":["low","medium","high","xhigh","max"],"claude-opus-5":["low","medium","high","xhigh","max"],"claude-sonnet-4-6":["low","medium","high","max"],"claude-sonnet-5":["low","medium","high","xhigh","max"],"deepseek-v4-flash-0731":["low","high","max"],"deepseek-v4-pro-0813":["low","high","max"],"gemini-3.1-pro":["low","medium","high"],"gemini-3.7-flash":["low","medium","high"],"gemini-3.8-flash":["low","medium","high"],"glm-5.2":["none","minimal","low","medium","high","xhigh","max"],"glm-5.3":["low","high","max"],"gpt-5.3-codex-spark":["none"],"gpt-5.4-mini":["none","low","medium","high","xhigh"],"gpt-5.4":["none","low","medium","high","xhigh"],"gpt-5.5":["none","low","medium","high","xhigh"],"gpt-5.6-luna":["none","low","medium","high","xhigh","max"],"gpt-5.6-sol":["none","low","medium","high","xhigh","max"],"gpt-5.6-terra":["none","low","medium","high","xhigh","max"],"gpt-daybreak-blue-latest":[],"grok-4.5":["low","medium","high","xhigh"],"grok-4.6":["low","medium","high","xhigh"],"kimi-k2.7-code":["always_on"],"kimi-k3":["low","high","max"],"minimax-m3":["disabled","adaptive"],"qwen3.6-flash":["none","minimal","low","medium"],"qwen3.6-plus":["none","minimal","low","medium"],"qwen3.7-flash":["none","minimal","low","medium"],"qwen3.7-max":["none","minimal","low","medium","high","xhigh","max"],"qwen3.7-plus":["none","minimal","low","medium","high","xhigh","max"],"qwen3.8-max":["none","minimal","low","medium","high","xhigh","max"]}'
# END GENERATED MODEL CATALOG
DEFAULT_BASE_URL="https://api.laoshirenai.com"
DEFAULT_SETUP_EXCHANGE_URL="https://laoshirenai.com/api/v1/public-setup/exchange"
DEFAULT_CODEX_MANIFEST_URL="https://laoshirenai.com/api/v1/public-downloads/codex/latest.json"
DEFAULT_CODEX_MODEL_CATALOG_URL="https://laoshirenai.com/auto-config/codex-model-catalog.json?v=${SCRIPT_VERSION}"
DEFAULT_GROK_CC_SWITCH_IMPORTER_URL="https://laoshirenai.com/auto-config/import-grok-cc-switch-provider.cjs?v=${SCRIPT_VERSION}"
DEFAULT_TOPUP_URL="https://laoshirenai.com/get-subscription"
DEFAULT_TOOLS="all"
DEFAULT_NODE_INDEX_PRIMARY="https://npmmirror.com/mirrors/node/index.tab"
DEFAULT_NODE_INDEX_FALLBACK="https://nodejs.org/dist/index.tab"
DEFAULT_NODE_DIST_PRIMARY="https://npmmirror.com/mirrors/node"
DEFAULT_NODE_DIST_FALLBACK="https://nodejs.org/dist"
DEFAULT_NPM_REGISTRY="https://registry.npmmirror.com"
FALLBACK_NPM_REGISTRY="https://registry.npmjs.org"
MIN_NODE_MAJOR=20

LAOSHIRENAI_HOME="${HOME}/.laoshirenai"
NODE_INSTALL_ROOT="${LAOSHIRENAI_HOME}/node"
NODE_CURRENT_DIR="${NODE_INSTALL_ROOT}/current"
NPM_PREFIX="${LAOSHIRENAI_HOME}/npm-global"
LOCAL_BIN_DIR="${HOME}/.local/bin"
CLAUDE_SETTINGS_PATH="${HOME}/.claude/settings.json"
CODEX_DIR="${HOME}/.codex"
CODEX_AUTH_PATH="${CODEX_DIR}/auth.json"
CODEX_CONFIG_PATH="${CODEX_DIR}/config.toml"
CODEX_MODEL_CATALOG_PATH="${CODEX_DIR}/laoshirenai-model-catalog.json"
GROK_DIR="${HOME}/.grok"
GROK_CONFIG_PATH="${GROK_DIR}/config.toml"
GROK_BIN_PATH="${GROK_DIR}/bin/grok"
GEMINI_DIR="${HOME}/.gemini"
GEMINI_ENV_PATH="${GEMINI_DIR}/.env"
GEMINI_SETTINGS_PATH="${GEMINI_DIR}/settings.json"
KIMI_DIR="${HOME}/.kimi-code"
KIMI_CONFIG_PATH="${KIMI_DIR}/config.toml"

BASE_URL="${DEFAULT_BASE_URL}"
TOOLS="${DEFAULT_TOOLS}"
CLAUDE_API_KEY="${LAOSHIRENAI_CLAUDE_API_KEY:-}"
CODEX_API_KEY="${LAOSHIRENAI_CODEX_API_KEY:-}"
GROK_API_KEY="${LAOSHIRENAI_GROK_API_KEY:-}"
GEMINI_API_KEY="${LAOSHIRENAI_GEMINI_API_KEY:-}"
KIMI_API_KEY="${LAOSHIRENAI_KIMI_API_KEY:-}"
GROK_CC_SWITCH_COMPAT=0
NODE_VERSION_OVERRIDE="${LAOSHIRENAI_NODE_VERSION:-}"
SKIP_CLIENT_INSTALL=0
FORCE_CLIENT_INSTALL=0
INSTALL_CODEX_APP=0
SETUP_TOKEN="${LAOSHIRENAI_SETUP_TOKEN:-}"
SELECTED_MODEL="${LAOSHIRENAI_MODEL_ID:-}"
SELECTED_PROTOCOL="${LAOSHIRENAI_PROTOCOL:-}"
SELECTED_REASONING="${LAOSHIRENAI_REASONING_EFFORT:-}"
GROK_API_BACKEND="responses"
SETUP_EXCHANGE_URL="${LAOSHIRENAI_SETUP_EXCHANGE_URL:-$DEFAULT_SETUP_EXCHANGE_URL}"
CODEX_MANIFEST_URL="${LAOSHIRENAI_CODEX_MANIFEST_URL:-$DEFAULT_CODEX_MANIFEST_URL}"
CODEX_MODEL_CATALOG_URL="${LAOSHIRENAI_CODEX_MODEL_CATALOG_URL:-$DEFAULT_CODEX_MODEL_CATALOG_URL}"
GROK_CC_SWITCH_IMPORTER_URL="${LAOSHIRENAI_GROK_CC_SWITCH_IMPORTER_URL:-$DEFAULT_GROK_CC_SWITCH_IMPORTER_URL}"
BALANCE_READY=1

# 兼容统一 API Key 环境变量；若未提供专用 Key，则回退复用统一值。
UNIFIED_API_KEY="${LAOSHIRENAI_API_KEY:-}"
if [ -n "$UNIFIED_API_KEY" ]; then
  [ -n "$CLAUDE_API_KEY" ] || CLAUDE_API_KEY="$UNIFIED_API_KEY"
  [ -n "$CODEX_API_KEY" ] || CODEX_API_KEY="$UNIFIED_API_KEY"
  [ -n "$GROK_API_KEY" ] || GROK_API_KEY="$UNIFIED_API_KEY"
  [ -n "$GEMINI_API_KEY" ] || GEMINI_API_KEY="$UNIFIED_API_KEY"
fi

# 支持通过环境变量覆盖基础参数，兼容管道执行或预置 shell 环境。
ENV_BASE_URL="${LAOSHIRENAI_BASE_URL:-}"
ENV_TOOLS="${LAOSHIRENAI_TOOLS:-}"
[ -n "$ENV_BASE_URL" ] && BASE_URL="$ENV_BASE_URL"
[ -n "$ENV_TOOLS" ] && TOOLS="$ENV_TOOLS"
[ "${LAOSHIRENAI_SKIP_CLIENT_INSTALL:-0}" = "1" ] && SKIP_CLIENT_INSTALL=1
[ "${LAOSHIRENAI_FORCE_CLIENT_INSTALL:-0}" = "1" ] && FORCE_CLIENT_INSTALL=1
[ "${LAOSHIRENAI_INSTALL_CODEX_APP:-0}" = "1" ] && INSTALL_CODEX_APP=1
[ "${LAOSHIRENAI_GROK_CC_SWITCH_COMPAT:-0}" = "1" ] && GROK_CC_SWITCH_COMPAT=1

NODE_BIN=""
NPM_BIN=""
PROFILE_FILE=""
PLATFORM_ID=""
NODE_ARCHIVE_NAME=""
USE_PROXYLESS_NPM=0
ACTIVE_NPM_REGISTRY="${DEFAULT_NPM_REGISTRY}"
INSTALL_CLAUDE_CLIENT=0
INSTALL_CODEX_CLIENT=0
INSTALL_GROK_CLIENT=0
INSTALL_GEMINI_CLIENT=0
EXISTING_CLAUDE_COMMAND=""
EXISTING_CODEX_COMMAND=""
EXISTING_GROK_COMMAND=""
EXISTING_GEMINI_COMMAND=""

# 输出信息日志，便于用户识别当前执行步骤。
log_info() {
  printf '[INFO] %s\n' "$1"
}

# 输出警告日志，提醒用户当前不是理想路径但仍可继续。
log_warn() {
  printf '[WARN] %s\n' "$1" >&2
}

# 输出错误日志并退出，避免脚本在异常状态下继续写配置。
log_error() {
  printf '[ERROR] %s\n' "$1" >&2
  exit 1
}

# 校验当前机器是否具备脚本运行所需的基础命令。
require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log_error "缺少必要命令: $1"
  fi
}

# 统一创建目录，避免后续写文件时因目录缺失失败。
ensure_dir() {
  mkdir -p "$1"
}

# 在首次改写配置前创建备份，避免覆盖用户已有文件后无法恢复。
create_backup_if_needed() {
  local target_path="$1"
  local backup_path="${target_path}.bak"

  if [ -f "$target_path" ] && [ ! -f "$backup_path" ]; then
    cp "$target_path" "$backup_path"
    log_info "已创建备份: $backup_path"
  fi
}

# 检测当前 shell 并选择最合适的 profile 文件写入 PATH。
detect_profile_file() {
  local shell_name
  shell_name="$(basename "${SHELL:-}")"

  case "$shell_name" in
    zsh)
      printf '%s/.zshrc' "$HOME"
      ;;
    bash)
      printf '%s/.bashrc' "$HOME"
      ;;
    *)
      printf '%s/.profile' "$HOME"
      ;;
  esac
}

# 将脚本需要的 PATH 导出块追加到 profile，保证新终端也能直接使用命令。
ensure_profile_exports() {
  local marker_begin="# >>> laoshirenai auto config >>>"
  local marker_end="# <<< laoshirenai auto config <<<"

  PROFILE_FILE="$(detect_profile_file)"
  touch "$PROFILE_FILE"

  if ! grep -Fq "$marker_begin" "$PROFILE_FILE"; then
    {
      printf '\n%s\n' "$marker_begin"
      # shellcheck disable=SC2016 # Keep $PATH literal for future shells.
      printf 'export PATH="%s/bin:%s/bin:%s:%s/bin:$PATH"\n' "$NODE_CURRENT_DIR" "$NPM_PREFIX" "$LOCAL_BIN_DIR" "$GROK_DIR"
      printf '%s\n' "$marker_end"
    } >>"$PROFILE_FILE"
    log_info "已写入 PATH 到 ${PROFILE_FILE}"
  elif ! grep -Fq "${GROK_DIR}/bin" "$PROFILE_FILE"; then
    local profile_tmp
    profile_tmp="$(mktemp)"
    awk -v marker="$marker_end" -v grok_bin="${GROK_DIR}/bin" '
      $0 == marker { printf "export PATH=\"%s:$PATH\"\n", grok_bin }
      { print }
    ' "$PROFILE_FILE" >"$profile_tmp"
    mv "$profile_tmp" "$PROFILE_FILE"
    log_info "已把 Grok Build 加入 PATH: ${PROFILE_FILE}"
  fi

  export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:${LOCAL_BIN_DIR}:${GROK_DIR}/bin:${PATH}"
}

# 为 claude 和 codex 生成稳定包装脚本，避免用户切换终端后找不到 node 运行时。
ensure_wrapper_scripts() {
  if ! needs_client_install; then
    return 0
  fi

  ensure_dir "$LOCAL_BIN_DIR"

  if [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ]; then
    cat >"${LOCAL_BIN_DIR}/claude" <<EOF
#!/usr/bin/env bash
export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:\$PATH"
exec "${NPM_PREFIX}/bin/claude" "\$@"
EOF
    chmod +x "${LOCAL_BIN_DIR}/claude"
  fi

  if [ "$INSTALL_CODEX_CLIENT" -eq 1 ]; then
    cat >"${LOCAL_BIN_DIR}/codex" <<EOF
#!/usr/bin/env bash
export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:\$PATH"
exec "${NPM_PREFIX}/bin/codex" "\$@"
EOF
    chmod +x "${LOCAL_BIN_DIR}/codex"
  fi

  if [ "$INSTALL_GEMINI_CLIENT" -eq 1 ]; then
    cat >"${LOCAL_BIN_DIR}/gemini" <<EOF
#!/usr/bin/env bash
export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:\$PATH"
exec "${NPM_PREFIX}/bin/gemini" "\$@"
EOF
    chmod +x "${LOCAL_BIN_DIR}/gemini"
  fi

  if [ "$INSTALL_KIMI_CLIENT" -eq 1 ]; then
    cat >"${LOCAL_BIN_DIR}/kimi" <<EOF
#!/usr/bin/env bash
export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:\$PATH"
exec "${NPM_PREFIX}/bin/kimi" "\$@"
EOF
    chmod +x "${LOCAL_BIN_DIR}/kimi"
  fi
}

# 判断当前代理变量是否指向本地代理，避免用户残留的失效代理把 npm 请求全部带偏。
detect_broken_local_proxy() {
  local proxy_value

  for proxy_value in "${HTTP_PROXY:-}" "${HTTPS_PROXY:-}" "${ALL_PROXY:-}" "${http_proxy:-}" "${https_proxy:-}" "${all_proxy:-}"; do
    case "$proxy_value" in
      *127.0.0.1*|*localhost*|*::1*)
        USE_PROXYLESS_NPM=1
        return 0
        ;;
    esac
  done

  USE_PROXYLESS_NPM=0
}

# 统一执行 npm 命令；当检测到失效本地代理时，主动清理代理环境变量再执行。
run_npm() {
  if [ "$USE_PROXYLESS_NPM" -eq 1 ]; then
    env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u http_proxy -u https_proxy -u all_proxy HOME="$HOME" "$NPM_BIN" "$@"
    return $?
  fi

  HOME="$HOME" "$NPM_BIN" "$@"
}

# 将下载步骤统一为带回退地址的实现，优先国内镜像，失败再走官方源。
download_to_file() {
  local output_path="$1"
  shift
  local url

  for url in "$@"; do
    if curl -fsSL "$url" -o "$output_path"; then
      return 0
    fi
    log_warn "下载失败，尝试下一个地址: $url"
  done

  return 1
}

# 拉取文本资源，主要用于获取 Node 版本索引。
download_text() {
  local url

  for url in "$@"; do
    if curl -fsSL "$url"; then
      return 0
    fi
    log_warn "读取失败，尝试下一个地址: $url"
  done

  return 1
}

# 规范化工具选择参数，限制脚本只处理支持的客户端。
normalize_tools() {
  local normalized_value

  normalized_value="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"

  case "$normalized_value" in
    all|claude|codex|grok|gemini|kimi)
      printf '%s' "$normalized_value"
      ;;
    *)
      log_error "不支持的 --tools 值: $1，可选值为 all / claude / codex / grok / gemini / kimi"
      ;;
  esac
}

# 解析命令行参数，支持完全非交互执行，也支持用户只传一部分参数。
parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --api-key)
        [ $# -ge 2 ] || log_error "--api-key 需要一个值"
        CLAUDE_API_KEY="$2"
        shift 2
        ;;
      --codex-api-key)
        [ $# -ge 2 ] || log_error "--codex-api-key 需要一个值"
        CODEX_API_KEY="$2"
        shift 2
        ;;
      --grok-api-key)
        [ $# -ge 2 ] || log_error "--grok-api-key 需要一个值"
        GROK_API_KEY="$2"
        shift 2
        ;;
      --gemini-api-key)
        [ $# -ge 2 ] || log_error "--gemini-api-key 需要一个值"
        GEMINI_API_KEY="$2"
        shift 2
        ;;
      --kimi-api-key)
        [ $# -ge 2 ] || log_error "--kimi-api-key 需要一个值"
        KIMI_API_KEY="$2"
        shift 2
        ;;
      --base-url)
        [ $# -ge 2 ] || log_error "--base-url 需要一个值"
        BASE_URL="$2"
        shift 2
        ;;
      --tools)
        [ $# -ge 2 ] || log_error "--tools 需要一个值"
        TOOLS="$(normalize_tools "$2")"
        shift 2
        ;;
      --node-version)
        [ $# -ge 2 ] || log_error "--node-version 需要一个值"
        NODE_VERSION_OVERRIDE="$2"
        shift 2
        ;;
      --skip-client-install)
        SKIP_CLIENT_INSTALL=1
        shift
        ;;
      --force-client-install)
        FORCE_CLIENT_INSTALL=1
        shift
        ;;
      --install-codex-app)
        INSTALL_CODEX_APP=1
        shift
        ;;
      --help|-h)
        cat <<'EOF'
老实人 AI 一键安装与自动配置脚本

用法:
  bash install.sh --api-key <Claude_API_Key> [--codex-api-key <Codex_API_Key>] [--grok-api-key <Grok_API_Key>] [--gemini-api-key <Gemini_API_Key>] [--tools all|claude|codex|grok|gemini] [--base-url https://api.laoshirenai.com]

参数:
  --api-key             Claude Code API Key
  --codex-api-key       Codex API Key
  --grok-api-key        Grok Build API Key
  --gemini-api-key      Gemini CLI API Key
  --tools               需要配置的工具，默认 all
  --base-url            API 基础地址，默认 https://api.laoshirenai.com
  --node-version        指定 Node.js 版本，例如 v24.11.0
  --skip-client-install 仅写配置，不安装客户端
  --force-client-install 即使检测到已有客户端，也重新安装所选 CLI
  --install-codex-app    同时安装或更新与当前 Mac 芯片匹配的 Codex App
EOF
        exit 0
        ;;
      *)
        log_error "未知参数: $1"
        ;;
    esac
  done
}

# 在脚本通过管道执行时，从终端读取指定工具的 API Key，避免 stdin 已被脚本内容占用。
prompt_for_named_api_key() {
  local key_label="$1"
  local prompt_text="$2"
  local target_name="$3"
  local option_hint="$4"
  local env_hint="$5"
  local input_value=""

  if [ ! -t 0 ] && [ ! -r /dev/tty ]; then
    log_error "当前不是交互终端，请通过 ${option_hint} 或 ${env_hint} 传入 ${key_label}"
  fi

  printf '%s: ' "$prompt_text" >/dev/tty
  IFS= read -r input_value </dev/tty
  printf '\n' >/dev/tty

  [ -n "$input_value" ] || log_error "${key_label} 不能为空"
  printf -v "$target_name" '%s' "$input_value"
}

# 根据用户选择的工具范围，分别补齐 Claude Code 与 Codex 所需的 API Key。
prompt_for_api_keys() {
  if [ -n "$SETUP_TOKEN" ]; then
    log_info "检测到一次性安装凭证，将自动领取对应客户端的专用配置"
    return 0
  fi
  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]; then
    if [ -z "$CLAUDE_API_KEY" ]; then
      prompt_for_named_api_key "Claude Code API Key" "请输入 Claude Code API Key" "CLAUDE_API_KEY" "--api-key" "LAOSHIRENAI_CLAUDE_API_KEY"
    fi
  fi

  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]; then
    if [ -z "$CODEX_API_KEY" ]; then
      prompt_for_named_api_key "Codex API Key" "请输入 Codex API Key" "CODEX_API_KEY" "--codex-api-key" "LAOSHIRENAI_CODEX_API_KEY"
    fi
  fi

  if [ "$TOOLS" = "grok" ] && [ -z "$GROK_API_KEY" ]; then
    prompt_for_named_api_key "Grok Build API Key" "请输入 Grok Build API Key" "GROK_API_KEY" "--grok-api-key" "LAOSHIRENAI_GROK_API_KEY"
  fi

  if [ "$TOOLS" = "gemini" ] && [ -z "$GEMINI_API_KEY" ]; then
    prompt_for_named_api_key "Gemini CLI API Key" "请输入 Gemini CLI API Key" "GEMINI_API_KEY" "--gemini-api-key" "LAOSHIRENAI_GEMINI_API_KEY"
  fi
  if [ "$TOOLS" = "kimi" ] && [ -z "$KIMI_API_KEY" ]; then
    prompt_for_named_api_key "Kimi Code API Key" "请输入 Kimi Code API Key" "KIMI_API_KEY" "--kimi-api-key" "LAOSHIRENAI_KIMI_API_KEY"
  fi
}

# 返回一个真正可运行的现有 CLI；PATH 残留但无法执行的命令不算已安装。
get_usable_client_command() {
  local command_name="$1"
  local command_path=""

  command_path="$(command -v "$command_name" 2>/dev/null || true)"
  [ -n "$command_path" ] || return 1

  if "$command_path" --version >/dev/null 2>&1; then
    printf '%s' "$command_path"
    return 0
  fi

  log_warn "检测到 ${command_name} 命令，但它当前无法运行，将按缺失客户端处理: ${command_path}"
  return 1
}

# 默认复用可用的现有 CLI，仅在缺失时安装；显式 force/skip 参数仍优先。
resolve_client_install_plan() {
  if [ "$SKIP_CLIENT_INSTALL" -eq 1 ] && [ "$FORCE_CLIENT_INSTALL" -eq 1 ]; then
    log_error "不能同时使用 --skip-client-install 和 --force-client-install"
  fi

  INSTALL_CLAUDE_CLIENT=0
  INSTALL_CODEX_CLIENT=0
  INSTALL_GROK_CLIENT=0
  INSTALL_KIMI_CLIENT=0

  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]; then
    EXISTING_CLAUDE_COMMAND="$(get_usable_client_command claude || true)"
    if [ "$FORCE_CLIENT_INSTALL" -eq 1 ]; then
      INSTALL_CLAUDE_CLIENT=1
      log_info "已要求强制重新安装 Claude Code CLI"
    elif [ -n "$EXISTING_CLAUDE_COMMAND" ]; then
      log_info "检测到现有 Claude Code CLI，跳过重复安装: ${EXISTING_CLAUDE_COMMAND}"
    elif [ "$SKIP_CLIENT_INSTALL" -eq 1 ]; then
      log_warn "未检测到可用的 Claude Code CLI，但已按要求跳过安装"
    else
      INSTALL_CLAUDE_CLIENT=1
    fi
  fi

  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]; then
    EXISTING_CODEX_COMMAND="$(get_usable_client_command codex || true)"
    if [ "$FORCE_CLIENT_INSTALL" -eq 1 ]; then
      INSTALL_CODEX_CLIENT=1
      log_info "已要求强制重新安装 Codex CLI"
    elif [ -n "$EXISTING_CODEX_COMMAND" ]; then
      log_info "检测到现有 Codex CLI，跳过重复安装: ${EXISTING_CODEX_COMMAND}"
    elif [ "$SKIP_CLIENT_INSTALL" -eq 1 ]; then
      log_warn "未检测到可用的 Codex CLI，但已按要求跳过安装"
    else
      INSTALL_CODEX_CLIENT=1
    fi
  fi

  if [ "$TOOLS" = "grok" ]; then
    EXISTING_GROK_COMMAND="$(get_usable_client_command grok || true)"
    if [ "$FORCE_CLIENT_INSTALL" -eq 1 ]; then
      INSTALL_GROK_CLIENT=1
      log_info "已要求强制重新安装 Grok Build"
    elif [ -n "$EXISTING_GROK_COMMAND" ]; then
      log_info "检测到现有 Grok Build，跳过重复安装: ${EXISTING_GROK_COMMAND}"
    elif [ "$SKIP_CLIENT_INSTALL" -eq 1 ]; then
      log_warn "未检测到可用的 Grok Build，但已按要求跳过安装"
    else
      INSTALL_GROK_CLIENT=1
    fi
  fi

  if [ "$TOOLS" = "gemini" ]; then
    EXISTING_GEMINI_COMMAND="$(get_usable_client_command gemini || true)"
    if [ "$FORCE_CLIENT_INSTALL" -eq 1 ]; then
      INSTALL_GEMINI_CLIENT=1
      log_info "已要求强制重新安装 Gemini CLI"
    elif [ -n "$EXISTING_GEMINI_COMMAND" ]; then
      log_info "检测到现有 Gemini CLI，跳过重复安装: ${EXISTING_GEMINI_COMMAND}"
    elif [ "$SKIP_CLIENT_INSTALL" -eq 1 ]; then
      log_warn "未检测到可用的 Gemini CLI，但已按要求跳过安装"
    else
      INSTALL_GEMINI_CLIENT=1
    fi
  fi
  if [ "$TOOLS" = "kimi" ]; then
    EXISTING_KIMI_COMMAND="$(get_usable_client_command kimi || true)"
    if [ "$FORCE_CLIENT_INSTALL" -eq 1 ]; then
      INSTALL_KIMI_CLIENT=1
    elif [ -n "$EXISTING_KIMI_COMMAND" ]; then
      log_info "检测到现有 Kimi Code，跳过重复安装: ${EXISTING_KIMI_COMMAND}"
    elif [ "$SKIP_CLIENT_INSTALL" -eq 1 ]; then
      log_warn "未检测到可用的 Kimi Code，但已按要求跳过安装"
    else
      INSTALL_KIMI_CLIENT=1
    fi
  fi
}

# 从 npm registry 读取最新稳定版本；国内镜像失败时回退官方源。
get_latest_package_version() {
  local package_path="$1"
  local registry response version

  for registry in "$DEFAULT_NPM_REGISTRY" "$FALLBACK_NPM_REGISTRY"; do
    response="$(curl -fsSL "${registry}/${package_path}/latest" 2>/dev/null || true)"
    version="$(printf '%s' "$response" | sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
    if [ -n "$version" ]; then
      printf '%s' "$version"
      return 0
    fi
  done
  return 1
}

get_client_version() {
  "$1" --version 2>/dev/null | sed -n 's/[^0-9]*\([0-9][0-9]*\(\.[0-9][0-9]*\)\{1,3\}\).*/\1/p' | head -n 1
}

# 仅当本机版本确实低于 registry 最新版本时更新，避免重复安装或意外降级。
version_is_older() {
  awk -v current="$1" -v latest="$2" 'BEGIN {
    n = split(current, a, "."); m = split(latest, b, "."); max = n > m ? n : m;
    for (i = 1; i <= max; i++) {
      av = (i <= n ? a[i] + 0 : 0); bv = (i <= m ? b[i] + 0 : 0);
      if (av < bv) exit 0; if (av > bv) exit 1;
    }
    exit 1;
  }'
}

check_client_update() {
  local label="$1" command_path="$2" package_path="$3" install_variable="$4"
  local current_version latest_version

  [ -n "$command_path" ] || return 0
  current_version="$(get_client_version "$command_path" || true)"
  latest_version="$(get_latest_package_version "$package_path" || true)"
  if [ -z "$current_version" ] || [ -z "$latest_version" ]; then
    log_warn "无法比较 ${label} 版本，本次保留现有可用版本并继续配置测试"
    return 0
  fi
  if version_is_older "$current_version" "$latest_version"; then
    printf -v "$install_variable" '%s' 1
    log_info "检测到 ${label} 可更新: ${current_version} -> ${latest_version}"
  else
    log_info "${label} 已是当前版本: ${current_version}"
  fi
}

resolve_client_update_plan() {
  [ "$SKIP_CLIENT_INSTALL" -eq 0 ] || return 0
  [ "$FORCE_CLIENT_INSTALL" -eq 0 ] || return 0
  [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ] || check_client_update "Claude Code CLI" "$EXISTING_CLAUDE_COMMAND" '@anthropic-ai%2Fclaude-code' INSTALL_CLAUDE_CLIENT
  [ "$INSTALL_CODEX_CLIENT" -eq 1 ] || check_client_update "Codex CLI" "$EXISTING_CODEX_COMMAND" '@openai%2Fcodex' INSTALL_CODEX_CLIENT
  [ "$INSTALL_GEMINI_CLIENT" -eq 1 ] || check_client_update "Gemini CLI" "$EXISTING_GEMINI_COMMAND" '@google%2Fgemini-cli' INSTALL_GEMINI_CLIENT
  [ "$INSTALL_KIMI_CLIENT" -eq 1 ] || check_client_update "Kimi Code CLI" "$EXISTING_KIMI_COMMAND" '@moonshot-ai%2Fkimi-code' INSTALL_KIMI_CLIENT
}

needs_client_install() {
  [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ] || [ "$INSTALL_CODEX_CLIENT" -eq 1 ] || [ "$INSTALL_GROK_CLIENT" -eq 1 ] || [ "$INSTALL_GEMINI_CLIENT" -eq 1 ] || [ "$INSTALL_KIMI_CLIENT" -eq 1 ]
}

needs_npm_client_install() {
  [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ] || [ "$INSTALL_CODEX_CLIENT" -eq 1 ] || [ "$INSTALL_GEMINI_CLIENT" -eq 1 ] || [ "$INSTALL_KIMI_CLIENT" -eq 1 ]
}

# 判断系统自带 node 是否可直接复用，避免重复下载安装。
has_usable_system_node() {
  if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    return 1
  fi

  local node_version
  local node_major

  node_version="$(node --version 2>/dev/null || true)"
  node_major="$(printf '%s' "$node_version" | sed 's/^v//' | cut -d. -f1)"

  [ -n "$node_major" ] || return 1
  [ "$node_major" -ge "$MIN_NODE_MAJOR" ] || return 1

  if [ "$GROK_CC_SWITCH_COMPAT" -eq 1 ] && uses_grok; then
    node --no-warnings -e 'require("node:sqlite").DatabaseSync' >/dev/null 2>&1 || return 1
  fi
  return 0
}

# 识别操作系统与架构，并映射到 Node 发布包命名规则。
detect_platform() {
  local os_name
  local arch_name

  os_name="$(uname -s)"
  arch_name="$(uname -m)"

  case "$os_name" in
    Darwin)
      case "$arch_name" in
        arm64|aarch64)
          PLATFORM_ID="darwin-arm64"
          NODE_ARCHIVE_NAME="darwin-arm64"
          ;;
        x86_64)
          PLATFORM_ID="darwin-x64"
          NODE_ARCHIVE_NAME="darwin-x64"
          ;;
        *)
          log_error "暂不支持的 macOS 架构: $arch_name"
          ;;
      esac
      ;;
    Linux)
      case "$arch_name" in
        arm64|aarch64)
          PLATFORM_ID="linux-arm64"
          NODE_ARCHIVE_NAME="linux-arm64"
          ;;
        x86_64|amd64)
          PLATFORM_ID="linux-x64"
          NODE_ARCHIVE_NAME="linux-x64"
          ;;
        *)
          log_error "暂不支持的 Linux 架构: $arch_name"
          ;;
      esac
      ;;
    *)
      log_error "install.sh 仅支持 macOS / Linux，当前系统为: $os_name"
      ;;
  esac
}

# 从 Node 版本索引中选出最新 LTS，避免脚本内硬编码版本逐渐过期。
resolve_node_version() {
  if [ -n "$NODE_VERSION_OVERRIDE" ]; then
    case "$NODE_VERSION_OVERRIDE" in
      v*)
        printf '%s' "$NODE_VERSION_OVERRIDE"
        ;;
      *)
        printf 'v%s' "$NODE_VERSION_OVERRIDE"
        ;;
    esac
    return 0
  fi

  local index_tab
  local version

  index_tab="$(download_text "$DEFAULT_NODE_INDEX_PRIMARY" "$DEFAULT_NODE_INDEX_FALLBACK")" || \
    log_error "无法获取 Node.js 版本索引"

  version="$(printf '%s\n' "$index_tab" | awk -F '\t' 'NR > 1 && $10 != "" && $10 != "-" && $10 != "false" { print $1; exit }')"

  [ -n "$version" ] || log_error "无法解析最新 LTS 版本"
  printf '%s' "$version"
}

# 下载并安装本地 Node 运行时，避免无管理员权限用户无法继续。
install_local_node() {
  local version
  local install_dir
  local archive_name
  local tmp_dir
  local archive_path
  local extracted_dir

  detect_platform
  version="$(resolve_node_version)"
  install_dir="${NODE_INSTALL_ROOT}/${version}/${PLATFORM_ID}"
  archive_name="node-${version}-${NODE_ARCHIVE_NAME}.tar.gz"

  if [ ! -x "${install_dir}/bin/node" ]; then
    tmp_dir="$(mktemp -d)"
    archive_path="${tmp_dir}/${archive_name}"

    log_info "正在下载 Node.js ${version} (${PLATFORM_ID})"
    download_to_file "$archive_path" \
      "${DEFAULT_NODE_DIST_PRIMARY}/${version}/${archive_name}" \
      "${DEFAULT_NODE_DIST_FALLBACK}/${version}/${archive_name}" || \
      log_error "Node.js 下载失败，请检查网络后重试"

    tar -xzf "$archive_path" -C "$tmp_dir"
    extracted_dir="$(find "$tmp_dir" -maxdepth 1 -type d -name "node-${version}-*" | head -n 1)"
    [ -n "$extracted_dir" ] || log_error "Node.js 解压失败"

    ensure_dir "$(dirname "$install_dir")"
    rm -rf "$install_dir"
    mv "$extracted_dir" "$install_dir"
    rm -rf "$tmp_dir"
  fi

  ln -sfn "$install_dir" "$NODE_CURRENT_DIR"
  NODE_BIN="${NODE_CURRENT_DIR}/bin/node"
  NPM_BIN="${NODE_CURRENT_DIR}/bin/npm"
}

# 统一确定本次运行实际使用的 Node/npm 路径。
ensure_node_runtime() {
  if has_usable_system_node; then
    NODE_BIN="$(command -v node)"
    NPM_BIN="$(command -v npm)"
    log_info "检测到可用系统 Node.js: $("$NODE_BIN" --version)"
    return 0
  fi

  log_warn "未检测到可用的 Node.js，开始安装本地运行时"
  install_local_node
  if [ "$GROK_CC_SWITCH_COMPAT" -eq 1 ] && uses_grok; then
    "$NODE_BIN" --no-warnings -e 'require("node:sqlite").DatabaseSync' >/dev/null 2>&1 || \
      log_error "当前 Node.js 不支持 CC Switch 安全导入，请移除 LAOSHIRENAI_NODE_VERSION 覆盖后重试"
  fi
  log_info "本地 Node.js 已就绪: $("$NODE_BIN" --version)"
}

# 用一次性凭证换取当前目标的专用 API Key。响应只在本机内存/临时文件中解析，
# 不会把 Key 打印到终端或写入 shell 历史。
exchange_setup_ticket() {
  [ -n "$SETUP_TOKEN" ] || return 0

  local tmp_dir
  local request_path
  local response_path
  local status_code
  local parsed
  local target
  local received_key
  local received_base_url
  local received_model
  local received_protocol

  tmp_dir="$(mktemp -d)"
  request_path="${tmp_dir}/request.json"
  response_path="${tmp_dir}/response.json"
  SETUP_TICKET="$SETUP_TOKEN" "$NODE_BIN" <<'EOF' >"$request_path"
process.stdout.write(JSON.stringify({ ticket: process.env.SETUP_TICKET }))
EOF

  log_info "正在领取一次性安装配置"
  status_code="$(curl -sS -o "$response_path" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    --data-binary "@${request_path}" \
    "$SETUP_EXCHANGE_URL" || true)"
  rm -f "$request_path"
  if [ "$status_code" != "200" ]; then
    rm -rf "$tmp_dir"
    log_error "一次性配置命令无效、已过期或已使用，请回到 API 密钥页或安装与下载页重新生成"
  fi

  parsed="$(SETUP_RESPONSE_PATH="$response_path" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const body = JSON.parse(fs.readFileSync(process.env.SETUP_RESPONSE_PATH, 'utf8'))
const data = body && body.data
if (!data || !['claude', 'codex', 'grok', 'gemini', 'kimi'].includes(data.target) || !data.api_key || !data.base_url || (data.client_id && (!data.model_id || !data.protocol))) {
  process.exit(2)
}
process.stdout.write([
  Buffer.from(String(data.target)).toString('base64'),
  Buffer.from(String(data.api_key)).toString('base64'),
  Buffer.from(String(data.base_url)).toString('base64'),
  Buffer.from(String(data.model_id || '')).toString('base64'),
  Buffer.from(String(data.protocol || '')).toString('base64')
].join(':'))
EOF
  )" || {
    rm -rf "$tmp_dir"
    log_error "服务器返回的一键安装配置格式无效"
  }
  rm -rf "$tmp_dir"

  IFS=: read -r target received_key received_base_url received_model received_protocol <<<"$parsed"
  target="$(printf '%s' "$target" | base64 -d)"
  received_key="$(printf '%s' "$received_key" | base64 -d)"
  received_base_url="$(printf '%s' "$received_base_url" | base64 -d)"
  received_model="$(printf '%s' "$received_model" | base64 -d)"
  received_protocol="$(printf '%s' "$received_protocol" | base64 -d)"
  [ "$target" = "$TOOLS" ] || log_error "安装凭证与当前工具不匹配，请重新生成"

  BASE_URL="$received_base_url"
  SELECTED_MODEL="$received_model"
  SELECTED_PROTOCOL="$received_protocol"
  if [ "$target" = "claude" ]; then
    CLAUDE_API_KEY="$received_key"
    [ -z "$SELECTED_MODEL" ] || CATALOG_ANTHROPIC_DEFAULT_MODEL="$SELECTED_MODEL"
  elif [ "$target" = "codex" ]; then
    CODEX_API_KEY="$received_key"
  elif [ "$target" = "gemini" ]; then
    GEMINI_API_KEY="$received_key"
    if [ -n "$SELECTED_MODEL" ]; then
      CATALOG_GEMINI_DEFAULT_MODEL="$SELECTED_MODEL"
      CATALOG_GEMINI_MANAGED_MODELS="$SELECTED_MODEL"
    fi
  elif [ "$target" = "kimi" ]; then
    KIMI_API_KEY="$received_key"
  else
    GROK_API_KEY="$received_key"
    if [ -n "$SELECTED_MODEL" ]; then
      CATALOG_GROK_DEFAULT_MODEL="$SELECTED_MODEL"
      CATALOG_GROK_DEFAULT_DISPLAY_NAME="$SELECTED_MODEL"
      GROK_API_BACKEND="$SELECTED_PROTOCOL"
      CATALOG_GROK_MANAGED_MODELS_JSON="$(MODEL_ID="$SELECTED_MODEL" "$NODE_BIN" -e 'process.stdout.write(JSON.stringify([{id:process.env.MODEL_ID,display_name:process.env.MODEL_ID,context_window:null}]))')"
    fi
  fi
  SETUP_TOKEN=""
  unset LAOSHIRENAI_SETUP_TOKEN SETUP_TICKET
  log_info "专用配置领取成功"
}

# 手动路径（无一次性票据）下，页面表单选择通过 LAOSHIRENAI_MODEL_ID 等环境变量
# 传入。这里把它们应用到各客户端真正写入的默认值，与票据路径保持一致；
# 否则手写配置会静默落回内置默认模型与默认协议。
apply_manual_selection() {
  [ -n "$SELECTED_MODEL" ] || return 0
  case "$TOOLS" in
    claude)
      CATALOG_ANTHROPIC_DEFAULT_MODEL="$SELECTED_MODEL"
      ;;
    gemini)
      CATALOG_GEMINI_DEFAULT_MODEL="$SELECTED_MODEL"
      CATALOG_GEMINI_MANAGED_MODELS="$SELECTED_MODEL"
      ;;
    grok)
      CATALOG_GROK_DEFAULT_MODEL="$SELECTED_MODEL"
      CATALOG_GROK_DEFAULT_DISPLAY_NAME="$SELECTED_MODEL"
      CATALOG_GROK_MANAGED_MODELS_JSON="$(MODEL_ID="$SELECTED_MODEL" "$NODE_BIN" -e 'process.stdout.write(JSON.stringify([{id:process.env.MODEL_ID,display_name:process.env.MODEL_ID,context_window:null}]))')"
      if [ -n "$SELECTED_PROTOCOL" ]; then
        case "$SELECTED_PROTOCOL" in
          responses|chat_completions|messages) GROK_API_BACKEND="$SELECTED_PROTOCOL" ;;
          *) log_error "Grok Build 不支持协议: ${SELECTED_PROTOCOL}" ;;
        esac
      fi
      ;;
  esac
}

# 将 npm 切到国内镜像，降低无代理环境下的失败率。
ensure_npm_registry() {
  ACTIVE_NPM_REGISTRY="$1"
  run_npm config set registry "$ACTIVE_NPM_REGISTRY" --location=user >/dev/null 2>&1 || \
    log_warn "设置 npm 镜像失败，后续安装将继续尝试默认配置"
}

# 安装 npm 包时优先走国内镜像，失败后自动回退到官方 registry。
npm_install_with_fallback() {
  local package_name="$1"

  if run_npm install -g --prefix "$NPM_PREFIX" --registry "$ACTIVE_NPM_REGISTRY" "$package_name"; then
    return 0
  fi

  if [ "$ACTIVE_NPM_REGISTRY" != "$FALLBACK_NPM_REGISTRY" ]; then
    log_warn "从 ${ACTIVE_NPM_REGISTRY} 安装失败，切换到官方 registry 重试"
    ensure_npm_registry "$FALLBACK_NPM_REGISTRY"
    run_npm install -g --prefix "$NPM_PREFIX" --registry "$ACTIVE_NPM_REGISTRY" "$package_name" && return 0
  fi

  log_error "安装 ${package_name} 失败，请检查网络后重试"
}

# 安装指定客户端包，全部安装到用户目录，避免污染系统环境。
install_requested_clients() {
  if ! needs_client_install; then
    log_info "所选客户端无需安装，本次仅写入配置并测试 API Key"
    return 0
  fi

  if needs_npm_client_install; then
    ensure_dir "$NPM_PREFIX"
    detect_broken_local_proxy
    if [ "$USE_PROXYLESS_NPM" -eq 1 ]; then
      log_warn "检测到本地代理环境变量，安装客户端时将临时绕过代理"
    fi
    ensure_npm_registry "$DEFAULT_NPM_REGISTRY"
  fi

  if [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ]; then
    log_info "正在安装或更新 Claude Code"
    npm_install_with_fallback "@anthropic-ai/claude-code@latest"
  fi

  if [ "$INSTALL_CODEX_CLIENT" -eq 1 ]; then
    log_info "正在安装或更新 Codex"
    npm_install_with_fallback "@openai/codex@latest"
  fi

  if [ "$INSTALL_GROK_CLIENT" -eq 1 ]; then
    log_info "正在通过 xAI 官方安装器安装或更新 Grok Build"
    curl -fsSL https://x.ai/cli/install.sh | bash || log_error "Grok Build 安装失败，请检查网络后重试"
  fi

  if [ "$INSTALL_GEMINI_CLIENT" -eq 1 ]; then
    log_info "正在安装或更新 Gemini CLI"
    npm_install_with_fallback "@google/gemini-cli@latest"
  fi
  if [ "$INSTALL_KIMI_CLIENT" -eq 1 ]; then
    log_info "正在安装或更新 Kimi Code"
    npm_install_with_fallback "@moonshot-ai/kimi-code@latest"
  fi
}

install_codex_app_if_requested() {
  [ "$INSTALL_CODEX_APP" -eq 1 ] || return 0
  uses_codex || log_error "--install-codex-app 只能与 Codex 一起使用"

  local os_name
  local arch_name
  local target_arch
  local tmp_dir
  local manifest_path
  local asset_record
  local asset_name
  local asset_url
  local expected_sha
  local dmg_path
  local actual_sha
  local mount_dir
  local source_app
  local app_name
  local process_name
  local destination_root
  local destination_app
  local source_version
  local current_version=""
  local current_app=""

  os_name="$(uname -s)"
  if [ "$os_name" != "Darwin" ]; then
    log_warn "Codex App 自动安装目前只在 macOS 使用 install.sh；Windows 请使用 PowerShell 命令"
    return 0
  fi
  arch_name="$(uname -m)"
  case "$arch_name" in
    arm64|aarch64) target_arch="arm64" ;;
    x86_64|amd64) target_arch="x64" ;;
    *) log_error "暂不支持的 macOS 架构: $arch_name" ;;
  esac

  tmp_dir="$(mktemp -d)"
  manifest_path="${tmp_dir}/latest.json"
  curl -fsSL "$CODEX_MANIFEST_URL" -o "$manifest_path" || {
    rm -rf "$tmp_dir"
    log_error "无法读取本站 Codex App 最新版本清单"
  }
  asset_record="$(MANIFEST_PATH="$manifest_path" TARGET_ARCH="$target_arch" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const manifest = JSON.parse(fs.readFileSync(process.env.MANIFEST_PATH, 'utf8'))
const assets = Array.isArray(manifest.assets) ? manifest.assets : []
const asset = assets.find((item) =>
  item.platform === 'macos' &&
  (item.arch === process.env.TARGET_ARCH || item.arch === 'universal') &&
  String(item.name || '').toLowerCase().endsWith('.dmg'))
if (!asset || !asset.download_url || !/^[a-f0-9]{64}$/i.test(String(asset.sha256 || ''))) process.exit(2)
process.stdout.write([
  Buffer.from(String(asset.name)).toString('base64'),
  Buffer.from(String(asset.download_url)).toString('base64'),
  Buffer.from(String(asset.sha256).toLowerCase()).toString('base64')
].join(':'))
EOF
  )" || {
    rm -rf "$tmp_dir"
    log_error "本站缓存中暂时没有适合当前 Mac 芯片的 Codex App"
  }
  IFS=: read -r asset_name asset_url expected_sha <<<"$asset_record"
  asset_name="$(printf '%s' "$asset_name" | base64 -d)"
  asset_url="$(printf '%s' "$asset_url" | base64 -d)"
  expected_sha="$(printf '%s' "$expected_sha" | base64 -d)"
  case "$asset_url" in
    https://laoshirenai.com/downloads/codex/*) ;;
    *) rm -rf "$tmp_dir"; log_error "Codex App 下载地址未通过同站校验" ;;
  esac

  dmg_path="${tmp_dir}/${asset_name}"
  log_info "正在从本站缓存下载 OpenAI 官方 Codex App (${target_arch})"
  curl -fL --retry 3 --retry-delay 2 "$asset_url" -o "$dmg_path" || {
    rm -rf "$tmp_dir"
    log_error "Codex App 下载失败，请检查网络后重试"
  }
  actual_sha="$(shasum -a 256 "$dmg_path" | awk '{print tolower($1)}')"
  if [ "$actual_sha" != "$expected_sha" ]; then
    rm -rf "$tmp_dir"
    log_error "Codex App SHA256 校验失败，已停止安装"
  fi

  mount_dir="${tmp_dir}/mount"
  mkdir -p "$mount_dir"
  hdiutil attach "$dmg_path" -nobrowse -quiet -mountpoint "$mount_dir" || {
    rm -rf "$tmp_dir"
    log_error "Codex App 镜像挂载失败"
  }
  source_app="$(find "$mount_dir" -maxdepth 2 -type d \( -name 'ChatGPT.app' -o -name 'Codex.app' \) -print -quit)"
  if [ -z "$source_app" ] || ! codesign --verify --deep --strict "$source_app" >/dev/null 2>&1; then
    hdiutil detach "$mount_dir" -quiet >/dev/null 2>&1 || true
    rm -rf "$tmp_dir"
    log_error "Codex App 签名校验失败，已停止安装"
  fi

  app_name="$(basename "$source_app")"
  process_name="${app_name%.app}"
  source_version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$source_app/Contents/Info.plist" 2>/dev/null || true)"
  for candidate in "/Applications/${app_name}" "$HOME/Applications/${app_name}"; do
    if [ -d "$candidate" ]; then
      current_app="$candidate"
      current_version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$candidate/Contents/Info.plist" 2>/dev/null || true)"
      break
    fi
  done
  if [ -n "$current_app" ] && [ -n "$source_version" ] && [ "$current_version" = "$source_version" ]; then
    log_info "Codex App 已是最新版本 (${source_version})"
  else
    destination_root="$HOME/Applications"
    destination_app="${destination_root}/${app_name}"
    mkdir -p "$destination_root"
    if pgrep -x "$process_name" >/dev/null 2>&1; then
      log_warn "检测到 Codex App 正在运行，将先安全退出再更新"
      osascript -e "tell application \"${process_name}\" to quit" >/dev/null 2>&1 || true
      sleep 2
    fi
    rm -rf "$destination_app"
    ditto "$source_app" "$destination_app"
    log_info "Codex App 已安装到 ${destination_app}${source_version:+（版本 ${source_version}）}"
  fi

  hdiutil detach "$mount_dir" -quiet >/dev/null 2>&1 || true
  rm -rf "$tmp_dir"
}

# 用 Node 安全合并 Claude Code 的 JSON 配置，尽量保留用户已有字段。
write_claude_config() {
  create_backup_if_needed "$CLAUDE_SETTINGS_PATH"
  ensure_dir "$(dirname "$CLAUDE_SETTINGS_PATH")"

  CONFIG_PATH="$CLAUDE_SETTINGS_PATH" CONFIG_BASE_URL="$BASE_URL" CONFIG_API_KEY="$CLAUDE_API_KEY" CONFIG_MODEL="$CATALOG_ANTHROPIC_DEFAULT_MODEL" CONFIG_REASONING_JSON="$CATALOG_MODEL_REASONING_JSON" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const path = process.env.CONFIG_PATH
const baseUrl = process.env.CONFIG_BASE_URL
const apiKey = process.env.CONFIG_API_KEY
const model = process.env.CONFIG_MODEL
const reasoning = JSON.parse(process.env.CONFIG_REASONING_JSON || '{}')

let config = {}
if (fs.existsSync(path)) {
  try {
    config = JSON.parse(fs.readFileSync(path, 'utf8'))
  } catch (error) {
    throw new Error(`refusing to overwrite malformed Claude settings: ${error.message}`)
  }
}

if (!config || typeof config !== 'object' || Array.isArray(config)) {
  throw new Error('refusing to overwrite non-object Claude settings')
}

if (!config.env || typeof config.env !== 'object' || Array.isArray(config.env)) {
  if (config.env != null) throw new Error('refusing to replace non-object Claude env')
  config.env = {}
}

config.model = model
delete config.effortLevel
if (!config.modelSettings || typeof config.modelSettings !== 'object' || Array.isArray(config.modelSettings)) {
  if (config.modelSettings != null) throw new Error('refusing to replace non-object Claude modelSettings')
  config.modelSettings = {}
}
const levels = Array.isArray(reasoning[model]) ? reasoning[model] : []
const currentModelSettings = config.modelSettings[model]
if (currentModelSettings != null && (typeof currentModelSettings !== 'object' || Array.isArray(currentModelSettings))) {
  throw new Error('refusing to replace non-object Claude modelSettings entry')
}
const modelSettings = currentModelSettings || {}
if (levels.includes('high')) modelSettings.effortLevel = 'high'
else delete modelSettings.effortLevel
if (Object.keys(modelSettings).length) config.modelSettings[model] = modelSettings
else delete config.modelSettings[model]
config.env.ANTHROPIC_BASE_URL = baseUrl
config.env.ANTHROPIC_AUTH_TOKEN = apiKey
config.env.ANTHROPIC_MODEL = model
config.env.ANTHROPIC_DEFAULT_OPUS_MODEL = model
config.env.ANTHROPIC_DEFAULT_SONNET_MODEL = model
config.env.ANTHROPIC_DEFAULT_HAIKU_MODEL = model
config.env.ANTHROPIC_DEFAULT_FABLE_MODEL = model
config.env.CLAUDE_CODE_ATTRIBUTION_HEADER = '0'
config.env.CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY = '1'

const temporaryPath = `${path}.tmp.${process.pid}.${Date.now()}`
try {
  fs.writeFileSync(temporaryPath, `${JSON.stringify(config, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 })
  fs.renameSync(temporaryPath, path)
  try { fs.chmodSync(path, 0o600) } catch {}
} finally {
  try { fs.unlinkSync(temporaryPath) } catch {}
}
EOF
}

# 合并 Codex 的 auth.json，仅覆盖 API Key 字段。
write_codex_auth() {
  create_backup_if_needed "$CODEX_AUTH_PATH"
  ensure_dir "$(dirname "$CODEX_AUTH_PATH")"

  CONFIG_PATH="$CODEX_AUTH_PATH" CONFIG_API_KEY="$CODEX_API_KEY" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const path = process.env.CONFIG_PATH
const apiKey = process.env.CONFIG_API_KEY

let config = {}
if (fs.existsSync(path)) {
  try {
    config = JSON.parse(fs.readFileSync(path, 'utf8'))
  } catch (error) {
    throw new Error(`refusing to overwrite malformed Codex auth: ${error.message}`)
  }
}

if (!config || typeof config !== 'object' || Array.isArray(config)) {
  throw new Error('refusing to overwrite non-object Codex auth')
}

config.OPENAI_API_KEY = apiKey
const temporaryPath = `${path}.tmp.${process.pid}.${Date.now()}`
try {
  fs.writeFileSync(temporaryPath, `${JSON.stringify(config, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 })
  fs.renameSync(temporaryPath, path)
  try { fs.chmodSync(path, 0o600) } catch {}
} finally {
  try { fs.unlinkSync(temporaryPath) } catch {}
}
EOF
}

# 把全局模板裁剪成当前 API Key 所属分组真正开放的模型。未知但已授权的
# 分组专属别名（例如 Daybreak）继承 Sol 的客户端能力模板；目录内容仍以
# /v1/models 为准，不能因为全局模板缺少别名而把它丢掉。
filter_codex_model_catalog() {
  local source_path="$1"
  local authorized_path="$2"

  SOURCE_PATH="$source_path" AUTHORIZED_PATH="$authorized_path" PREFERRED_MODEL="$SELECTED_MODEL" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const sourcePath = process.env.SOURCE_PATH
const authorizedPath = process.env.AUTHORIZED_PATH
const source = JSON.parse(fs.readFileSync(sourcePath, 'utf8'))
const response = JSON.parse(fs.readFileSync(authorizedPath, 'utf8'))
const models = Array.isArray(source.models) ? source.models : []
const required = ['slug', 'base_instructions', 'supports_reasoning_summaries', 'context_window', 'visibility']
const authorized = []
const preferredModel = (process.env.PREFERRED_MODEL || '').trim()
const seen = new Set()

for (const row of Array.isArray(response.data) ? response.data : []) {
  const id = typeof row?.id === 'string' ? row.id.trim() : ''
  if (!id || id === 'codex-auto-review' || seen.has(id)) continue
  seen.add(id)
  authorized.push(id)
}

if (
  models.length === 0 ||
  models.some((model) => required.some((field) => !(field in model))) ||
  authorized.length === 0
) {
  throw new Error('invalid Codex model catalog or empty group model list')
}
if (preferredModel && !authorized.includes(preferredModel)) {
  throw new Error('ticket model is not present in the current key model list')
}

const byID = new Map(models.map((model) => [model.slug, model]))
const missing = authorized.filter((id) => !byID.has(id))
if (missing.length) {
  throw new Error(`Codex catalog does not cover every authorized model: ${missing.join(', ')}`)
}
const filtered = authorized
  .flatMap((id) => byID.has(id) ? [{ ...byID.get(id) }] : [])
  .map((model, index) => ({ ...model, priority: index + 1 }))

if (filtered.length === 0) throw new Error('the current key has no Codex-compatible Responses model')
if (preferredModel && !filtered.some(model => model.slug === preferredModel)) {
  throw new Error('ticket model is not present in the Codex Responses catalog')
}

if (filtered.some((model) => required.some((field) => !(field in model)))) {
  throw new Error('filtered Codex model catalog is incomplete')
}

fs.writeFileSync(sourcePath, `${JSON.stringify({ models: filtered }, null, 2)}\n`, 'utf8')
process.stdout.write(preferredModel || (filtered.some(model => model.slug === 'gpt-5.6-sol') ? 'gpt-5.6-sol' : filtered[0].slug))
EOF
}

# 写入当前分组受支持的模型目录，阻止 Codex 回退到官方缓存后展示网关不支持的模型。
write_codex_model_catalog() {
  create_backup_if_needed "$CODEX_MODEL_CATALOG_PATH"
  ensure_dir "$CODEX_DIR"

  local source_path="${CODEX_MODEL_CATALOG_PATH}.download"
  local authorized_path="${CODEX_MODEL_CATALOG_PATH}.authorized"
  local api_base_url
  local status_code
  local selected_default
  download_to_file "$source_path" "$CODEX_MODEL_CATALOG_URL" || \
    log_error "Codex 模型目录下载失败，请稍后重试"

  api_base_url="$(normalize_openai_v1_base_url "$BASE_URL")"
  status_code="$(curl -sS -o "$authorized_path" -w '%{http_code}' \
    -H "Authorization: Bearer ${CODEX_API_KEY}" \
    "${api_base_url}/models" || true)"
  if [ "$status_code" != "200" ]; then
    rm -f "$source_path" "$authorized_path"
    log_error "Codex 分组模型读取失败: ${api_base_url}/models 返回 HTTP ${status_code}"
  fi

  if ! selected_default="$(filter_codex_model_catalog "$source_path" "$authorized_path")"; then
    rm -f "$source_path" "$authorized_path"
    log_error "Codex 分组模型目录生成失败"
  fi
  CATALOG_OPENAI_DEFAULT_MODEL="$selected_default"
  mv -f "$source_path" "$CODEX_MODEL_CATALOG_PATH"
  rm -f "$authorized_path"
}

# 只更新 Codex 的本站 owned fields 和独立 Provider block，保留 MCP、其他 Provider、
# 权限、通知和用户自定义设置；损坏或不完整的表头直接拒绝写入。
write_codex_config() {
  create_backup_if_needed "$CODEX_CONFIG_PATH"
  ensure_dir "$(dirname "$CODEX_CONFIG_PATH")"

  CONFIG_PATH="$CODEX_CONFIG_PATH" MODEL_CATALOG_PATH="$CODEX_MODEL_CATALOG_PATH" CONFIG_BASE_URL="$BASE_URL" CONFIG_MODEL="$CATALOG_OPENAI_DEFAULT_MODEL" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const path = process.env.CONFIG_PATH
const catalog = JSON.parse(fs.readFileSync(process.env.MODEL_CATALOG_PATH, 'utf8'))
const model = catalog.models.find(row => row.slug === process.env.CONFIG_MODEL)
if (!model) throw new Error('selected Codex model is missing from the authorized catalog')
const levels = (model.supported_reasoning_levels || []).map(row => row.effort)
const effort = levels.includes('high') ? 'high' : (levels[0] || '')
const ownedRoot = new Set(['model_provider','model','review_model','model_reasoning_effort','model_catalog_json','disable_response_storage','network_access','preferred_auth_method','model_context_window','model_auto_compact_token_limit'])
const managedSection = 'model_providers.laoshirenai_responses'
const text = fs.existsSync(path) ? fs.readFileSync(path, 'utf8') : ''
if (text.includes('\u0000')) throw new Error('refusing to rewrite malformed TOML containing NUL')
const kept = []
let section = ''
let dropping = false
for (const line of text.split(/\r?\n/)) {
  const trimmed = line.trim()
  if (trimmed.startsWith('[')) {
    const header = trimmed.match(/^\[([^\]]+)\](?:\s*#.*)?$/)
    if (!header) throw new Error(`refusing to rewrite malformed TOML header: ${trimmed}`)
    section = header[1]
    dropping = section === managedSection
  }
  if (dropping) continue
  const key = section === '' ? trimmed.match(/^([A-Za-z0-9_.-]+)\s*=/)?.[1] : undefined
  if (key && ownedRoot.has(key)) continue
  if (trimmed === '# BEGIN LAOSHIRENAI CODEX PROVIDER' || trimmed === '# END LAOSHIRENAI CODEX PROVIDER') continue
  kept.push(line)
}
while (kept.length && !kept[kept.length - 1].trim()) kept.pop()
while (kept.length && !kept[0].trim()) kept.shift()
const root = [
  `model_provider = ${JSON.stringify('laoshirenai_responses')}`,
  `model = ${JSON.stringify(model.slug)}`,
  `review_model = ${JSON.stringify(model.slug)}`,
  ...(effort ? [`model_reasoning_effort = ${JSON.stringify(effort)}`] : []),
  'model_catalog_json = "laoshirenai-model-catalog.json"',
  'disable_response_storage = true',
  'network_access = "enabled"',
  'preferred_auth_method = "apikey"',
  `model_context_window = ${Number(model.context_window)}`,
  `model_auto_compact_token_limit = ${Number(model.auto_compact_token_limit)}`,
  '',
]
const provider = [
  '# BEGIN LAOSHIRENAI CODEX PROVIDER',
  `[${managedSection}]`,
  'name = "老实人AI Responses"',
  `base_url = ${JSON.stringify(process.env.CONFIG_BASE_URL)}`,
  'wire_api = "responses"',
  'requires_openai_auth = true',
  '# END LAOSHIRENAI CODEX PROVIDER',
  '',
]
const output = [...root, ...kept, ...(kept.length ? [''] : []), ...provider].join('\n')
const temporaryPath = `${path}.tmp.${process.pid}.${Date.now()}`
try {
  fs.writeFileSync(temporaryPath, output, { encoding: 'utf8', mode: 0o600 })
  fs.renameSync(temporaryPath, path)
  try { fs.chmodSync(path, 0o600) } catch {}
} finally {
  try { fs.unlinkSync(temporaryPath) } catch {}
}
EOF
}

# 在保留其他 Grok 设置的前提下，确定性更新 Grok 月卡模型与默认模型。
write_grok_config() {
  create_backup_if_needed "$GROK_CONFIG_PATH"
  ensure_dir "$GROK_DIR"

  CONFIG_PATH="$GROK_CONFIG_PATH" CONFIG_BASE_URL="$(normalize_openai_v1_base_url "$BASE_URL")" CONFIG_API_KEY="$GROK_API_KEY" CONFIG_MODEL="$CATALOG_GROK_DEFAULT_MODEL" CONFIG_PROTOCOL="$GROK_API_BACKEND" CONFIG_MANAGED_MODELS="$CATALOG_GROK_MANAGED_MODELS_JSON" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const path = process.env.CONFIG_PATH
const baseUrl = process.env.CONFIG_BASE_URL
const apiKey = process.env.CONFIG_API_KEY
const model = process.env.CONFIG_MODEL
const protocol = process.env.CONFIG_PROTOCOL || 'responses'
const managedModels = JSON.parse(process.env.CONFIG_MANAGED_MODELS || '[]')
const managedModelIds = managedModels.map((profile) => profile.id)
const managedSections = new Set(managedModelIds.flatMap((id) => [`model.${id}`, `model."${id}"`]))
let text = fs.existsSync(path) ? fs.readFileSync(path, 'utf8') : ''
let lines = text.split(/\r?\n/)

// Remove the model block previously managed by this installer.
const kept = []
let droppingModel = false
for (const line of lines) {
  const trimmed = line.trim()
  const header = trimmed.match(/^\[([^\]]+)\]$/)
  if (trimmed.startsWith('[') && !header) throw new Error(`refusing to rewrite malformed Grok TOML header: ${trimmed}`)
  if (header) droppingModel = managedSections.has(header[1])
  if (!droppingModel && line.trim() !== '# Managed by laoshirenai one-click setup') kept.push(line)
}

lines = kept

// Set the default model without duplicating an existing [models] table.
let modelsHeader = lines.findIndex((line) => line.trim() === '[models]')
if (modelsHeader < 0) {
  while (lines.length && !lines[lines.length - 1].trim()) lines.pop()
  lines.push('', '[models]', `default = ${JSON.stringify(model)}`)
} else {
  let end = lines.length
  for (let i = modelsHeader + 1; i < lines.length; i++) {
    if (/^\s*\[/.test(lines[i])) { end = i; break }
  }
  let replaced = false
  for (let i = modelsHeader + 1; i < end; i++) {
    if (/^\s*default\s*=/.test(lines[i])) {
      lines[i] = `default = ${JSON.stringify(model)}`
      replaced = true
      break
    }
  }
  if (!replaced) lines.splice(modelsHeader + 1, 0, `default = ${JSON.stringify(model)}`)
}

while (lines.length && !lines[lines.length - 1].trim()) lines.pop()
lines.push('', '# Managed by laoshirenai one-click setup')
for (const profile of managedModels) {
  const block = [
    `[model.${JSON.stringify(profile.id)}]`,
    `model = ${JSON.stringify(profile.id)}`,
    `base_url = ${JSON.stringify(baseUrl)}`,
    `name = ${JSON.stringify(profile.display_name)}`,
    `description = ${JSON.stringify(profile.display_name)}`,
    `api_key = ${JSON.stringify(apiKey)}`,
    `api_backend = ${JSON.stringify(protocol)}`,
  ]
  if (Number(profile.context_window) > 0) block.push(`context_window = ${Number(profile.context_window)}`)
  block.push('')
  lines.push(...block)
}

const temporaryPath = `${path}.tmp.${process.pid}.${Date.now()}`
try {
  fs.writeFileSync(temporaryPath, lines.join('\n'), { encoding: 'utf8', mode: 0o600 })
  try { fs.chmodSync(temporaryPath, 0o600) } catch {}
  fs.renameSync(temporaryPath, path)
  try { fs.chmodSync(path, 0o600) } catch {}
} finally {
  try { fs.unlinkSync(temporaryPath) } catch {}
}
EOF
}

# Read the selected Key's complete model list immediately before writing Grok
# Build. Refuse partial imports when the ticket default disappears.
discover_grok_models() {
  local response_path
  local api_base_url
  local status_code
  local parsed
  response_path="$(mktemp)"
  api_base_url="$(normalize_openai_v1_base_url "$BASE_URL")"
  status_code="$(curl -sS -o "$response_path" -w '%{http_code}' \
    -H "Authorization: Bearer ${GROK_API_KEY}" \
    "${api_base_url}/models" || true)"
  [ "$status_code" = "200" ] || { rm -f "$response_path"; log_error "Grok Build 分组模型读取失败: HTTP ${status_code}"; }
  parsed="$(MODELS_PATH="$response_path" PREFERRED_MODEL="$SELECTED_MODEL" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const body = JSON.parse(fs.readFileSync(process.env.MODELS_PATH, 'utf8'))
const preferred = (process.env.PREFERRED_MODEL || '').trim()
const ids = []
const seen = new Set()
for (const row of Array.isArray(body.data) ? body.data : []) {
  const id = typeof row?.id === 'string' ? row.id.trim() : ''
  if (!id || id === 'codex-auto-review' || seen.has(id) || !/^[A-Za-z0-9][A-Za-z0-9._:-]*$/.test(id)) continue
  seen.add(id)
  ids.push(id)
}
if (!ids.length) throw new Error('empty model list')
if (preferred && !seen.has(preferred)) throw new Error('ticket model is no longer available')
const selected = preferred || (seen.has('gpt-5.6-sol') ? 'gpt-5.6-sol' : ids[0])
process.stdout.write(`${selected}\n${JSON.stringify(ids.map(id => ({id, display_name:id, context_window:null})))}`)
EOF
  )" || { rm -f "$response_path"; log_error "Grok Build 分组模型目录无效"; }
  rm -f "$response_path"
  CATALOG_GROK_DEFAULT_MODEL="${parsed%%$'\n'*}"
  CATALOG_GROK_DEFAULT_DISPLAY_NAME="$CATALOG_GROK_DEFAULT_MODEL"
  CATALOG_GROK_MANAGED_MODELS_JSON="${parsed#*$'\n'}"
}

write_kimi_config() {
  local response_path
  local api_base_url
  local status_code
  response_path="$(mktemp)"
  api_base_url="$(normalize_openai_v1_base_url "$BASE_URL")"
  status_code="$(curl -sS -o "$response_path" -w '%{http_code}' \
    -H "Authorization: Bearer ${KIMI_API_KEY}" "${api_base_url}/models" || true)"
  [ "$status_code" = "200" ] || { rm -f "$response_path"; log_error "Kimi Code 分组模型读取失败: HTTP ${status_code}"; }

  create_backup_if_needed "$KIMI_CONFIG_PATH"
  ensure_dir "$KIMI_DIR"
  CONFIG_PATH="$KIMI_CONFIG_PATH" MODELS_PATH="$response_path" CONFIG_BASE_URL="$api_base_url" CONFIG_API_KEY="$KIMI_API_KEY" PREFERRED_MODEL="$SELECTED_MODEL" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const path = process.env.CONFIG_PATH
const response = JSON.parse(fs.readFileSync(process.env.MODELS_PATH, 'utf8'))
const preferred = (process.env.PREFERRED_MODEL || '').trim()
const ids = []
const seen = new Set()
for (const row of Array.isArray(response.data) ? response.data : []) {
  const id = typeof row?.id === 'string' ? row.id.trim() : ''
  if (!id || id === 'codex-auto-review' || seen.has(id) || !/^[A-Za-z0-9][A-Za-z0-9._:-]*$/.test(id)) continue
  seen.add(id)
  ids.push(id)
}
if (!ids.length) throw new Error('empty Kimi Code model list')
if (preferred && !seen.has(preferred)) throw new Error('ticket model is no longer available')
const selected = preferred || (seen.has('kimi-k3') ? 'kimi-k3' : ids[0])
const text = fs.existsSync(path) ? fs.readFileSync(path, 'utf8') : ''
if (text.includes('\u0000')) throw new Error('refusing malformed Kimi Code TOML')
const kept = []
let section = ''
let managed = false
for (const line of text.split(/\r?\n/)) {
  const trimmed = line.trim()
  if (trimmed === '# BEGIN LSRAI KIMI CODE') { managed = true; continue }
  if (trimmed === '# END LSRAI KIMI CODE') { managed = false; continue }
  if (managed) continue
  if (trimmed.startsWith('[')) {
    const header = trimmed.match(/^\[([^\]]+)\](?:\s*#.*)?$/)
    if (!header) throw new Error(`refusing malformed TOML header: ${trimmed}`)
    section = header[1]
  }
  if (section === '' && /^default_model\s*=/.test(trimmed)) continue
  kept.push(line)
}
while (kept.length && !kept[0].trim()) kept.shift()
while (kept.length && !kept[kept.length - 1].trim()) kept.pop()
const block = [
  `default_model = ${JSON.stringify(`lsrai/${selected}`)}`,
  '',
  '# BEGIN LSRAI KIMI CODE',
  '[providers.lsrai]',
  'type = "openai"',
  `base_url = ${JSON.stringify(process.env.CONFIG_BASE_URL)}`,
  `api_key = ${JSON.stringify(process.env.CONFIG_API_KEY)}`,
  '',
]
for (const id of ids) {
  block.push(`[models.${JSON.stringify(`lsrai/${id}`)}]`)
  block.push('provider = "lsrai"')
  block.push(`model = ${JSON.stringify(id)}`)
  block.push('max_context_size = 262144')
  block.push('')
}
block.push('# END LSRAI KIMI CODE', '')
const output = [...block, ...(kept.length ? kept.concat('') : [])].join('\n')
const temporaryPath = `${path}.tmp.${process.pid}.${Date.now()}`
try {
  fs.writeFileSync(temporaryPath, output, {encoding:'utf8', mode:0o600})
  fs.renameSync(temporaryPath, path)
  try { fs.chmodSync(path, 0o600) } catch {}
} finally { try { fs.unlinkSync(temporaryPath) } catch {} }
EOF
  rm -f "$response_path"
}

# 合并写入 Gemini CLI 的 ~/.gemini/.env 与 settings.json：.env 只更新本站管理的
# 四个键并保留其他行，settings.json 只更新鉴权方式、默认模型和本站管理的
# thinkingConfig 覆盖项，其余字段与 overrides 原样保留。
write_gemini_config() {
  create_backup_if_needed "$GEMINI_ENV_PATH"
  create_backup_if_needed "$GEMINI_SETTINGS_PATH"
  ensure_dir "$GEMINI_DIR"

  ENV_PATH="$GEMINI_ENV_PATH" CONFIG_BASE_URL="$BASE_URL" CONFIG_API_KEY="$GEMINI_API_KEY" CONFIG_MODEL="$CATALOG_GEMINI_DEFAULT_MODEL" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const path = process.env.ENV_PATH
const managed = {
  GEMINI_API_KEY: process.env.CONFIG_API_KEY,
  GOOGLE_GEMINI_BASE_URL: process.env.CONFIG_BASE_URL,
  GOOGLE_GENAI_USE_VERTEXAI: 'false',
  GEMINI_MODEL: process.env.CONFIG_MODEL
}

let text = fs.existsSync(path) ? fs.readFileSync(path, 'utf8') : ''
let lines = text.length ? text.split(/\r?\n/) : []
if (lines.length && lines[lines.length - 1] === '') lines.pop()

const seen = new Set()
lines = lines.map((line) => {
  const match = line.match(/^([A-Za-z_][A-Za-z0-9_]*)=/)
  if (!match || !(match[1] in managed)) return line
  seen.add(match[1])
  return `${match[1]}=${managed[match[1]]}`
})
for (const [key, value] of Object.entries(managed)) {
  if (!seen.has(key)) lines.push(`${key}=${value}`)
}

const temporaryPath = `${path}.tmp.${process.pid}.${Date.now()}`
try {
  fs.writeFileSync(temporaryPath, `${lines.join('\n')}\n`, { encoding: 'utf8', mode: 0o600 })
  try { fs.chmodSync(temporaryPath, 0o600) } catch {}
  fs.renameSync(temporaryPath, path)
  try { fs.chmodSync(path, 0o600) } catch {}
} finally {
  try { fs.unlinkSync(temporaryPath) } catch {}
}
EOF

  CONFIG_PATH="$GEMINI_SETTINGS_PATH" CONFIG_MODEL="$CATALOG_GEMINI_DEFAULT_MODEL" CONFIG_MANAGED_MODELS="$CATALOG_GEMINI_MANAGED_MODELS" CONFIG_REASONING="$SELECTED_REASONING" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const path = process.env.CONFIG_PATH
const model = process.env.CONFIG_MODEL
const managedModels = (process.env.CONFIG_MANAGED_MODELS || '').split(/\s+/).filter(Boolean)
const managedSet = new Set(managedModels)
const requestedReasoning = String(process.env.CONFIG_REASONING || '').toLowerCase()
const thinkingLevel = ['low', 'medium', 'high'].includes(requestedReasoning) ? requestedReasoning.toUpperCase() : 'HIGH'

let config = {}
if (fs.existsSync(path)) {
  try {
    config = JSON.parse(fs.readFileSync(path, 'utf8'))
  } catch (error) {
    throw new Error(`refusing to overwrite malformed Gemini settings: ${error.message}`)
  }
}

if (!config || typeof config !== 'object' || Array.isArray(config)) {
  throw new Error('refusing to overwrite non-object Gemini settings')
}

if (!config.security || typeof config.security !== 'object' || Array.isArray(config.security)) {
  config.security = {}
}
if (!config.security.auth || typeof config.security.auth !== 'object' || Array.isArray(config.security.auth)) {
  config.security.auth = {}
}
config.security.auth.selectedType = 'gemini-api-key'

if (!config.model || typeof config.model !== 'object' || Array.isArray(config.model)) {
  config.model = {}
}
config.model.name = model

if (!config.modelConfigs || typeof config.modelConfigs !== 'object' || Array.isArray(config.modelConfigs)) {
  config.modelConfigs = {}
}
const overrides = Array.isArray(config.modelConfigs.overrides) ? config.modelConfigs.overrides : []
config.modelConfigs.overrides = overrides.filter(
  (entry) => !entry || typeof entry !== 'object' || !entry.match || !managedSet.has(entry.match.model)
)
for (const modelId of managedModels) {
  config.modelConfigs.overrides.push({
    match: { model: modelId },
    generateContentConfig: { thinkingConfig: { thinkingLevel } }
  })
}

fs.writeFileSync(path, `${JSON.stringify(config, null, 2)}\n`, 'utf8')
EOF
}

stop_cc_switch_for_import() {
  command -v pgrep >/dev/null 2>&1 || return 0
  pgrep -x "cc-switch" >/dev/null 2>&1 || return 0

  if command -v osascript >/dev/null 2>&1; then
    osascript -e 'tell application id "com.ccswitch.desktop" to quit' >/dev/null 2>&1 || true
  fi
  local attempt
  for attempt in $(seq 1 20); do
    pgrep -x "cc-switch" >/dev/null 2>&1 || return 0
    sleep 0.25
  done

  # Tauri may remain in the tray after its last window closes. TERM lets SQLite
  # finish normally; the importer still checkpoints and backs up before writing.
  pkill -TERM -x "cc-switch" >/dev/null 2>&1 || true
  for attempt in $(seq 1 20); do
    pgrep -x "cc-switch" >/dev/null 2>&1 || return 0
    sleep 0.25
  done
  return 1
}

open_cc_switch_if_requested() {
  [ "$GROK_CC_SWITCH_COMPAT" -eq 1 ] && uses_grok || return 0

  if [ "$(uname -s)" != "Darwin" ] || ! command -v open >/dev/null 2>&1 || ! open -Ra "CC Switch" >/dev/null 2>&1; then
    log_warn "Grok Build 已配置好；未找到官方 CC Switch，可稍后安装后重试"
    return 0
  fi

  local tmp_dir importer_path db_path attempt
  tmp_dir="$(mktemp -d)"
  importer_path="${tmp_dir}/import-grok-cc-switch-provider.cjs"
  if [ -n "${LAOSHIRENAI_GROK_CC_SWITCH_IMPORTER_PATH:-}" ]; then
    cp "$LAOSHIRENAI_GROK_CC_SWITCH_IMPORTER_PATH" "$importer_path"
  else
    download_to_file "$importer_path" "$GROK_CC_SWITCH_IMPORTER_URL" || {
      rm -rf "$tmp_dir"
      log_error "CC Switch Provider 导入组件下载失败，请稍后重试"
    }
  fi

  db_path="$("$NODE_BIN" --no-warnings "$importer_path" --print-db-path)"
  if [ ! -f "$db_path" ]; then
    open -gja "CC Switch" >/dev/null 2>&1 || true
    for _ in $(seq 1 40); do
      [ -f "$db_path" ] && break
      sleep 0.25
    done
  fi

  if ! stop_cc_switch_for_import; then
    rm -rf "$tmp_dir"
    log_error "CC Switch 正在运行且无法安全刷新，请退出后重试这一行命令"
  fi

  db_path="$("$NODE_BIN" --no-warnings "$importer_path" --print-db-path)"
  if ! GROK_PROVIDER_DEFAULT_MODEL="$CATALOG_GROK_DEFAULT_MODEL" \
    GROK_PROVIDER_BASE_URL="$(normalize_openai_v1_base_url "$BASE_URL")" \
    GROK_PROVIDER_API_KEY="$GROK_API_KEY" \
    GROK_PROVIDER_MODELS_JSON="$CATALOG_GROK_MANAGED_MODELS_JSON" \
    "$NODE_BIN" --no-warnings "$importer_path" >/dev/null; then
    rm -rf "$tmp_dir"
    log_error "CC Switch Provider 导入失败，本机 Grok Build 配置和原有 Provider 均已保留"
  fi
  rm -rf "$tmp_dir"

  if open -a "CC Switch" >/dev/null 2>&1; then
    log_info "已将 Grok 分组导入官方 CC Switch，并保留其他 Provider"
    return 0
  fi
  log_warn "Grok 分组已导入 CC Switch，但无法自动打开应用"
}

uses_codex() {
  [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]
}

uses_claude() {
  [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]
}

uses_grok() {
  [ "$TOOLS" = "grok" ]
}

uses_gemini() {
  [ "$TOOLS" = "gemini" ]
}

uses_kimi() {
  [ "$TOOLS" = "kimi" ]
}

normalize_openai_v1_base_url() {
  local normalized_url

  normalized_url="${1%/}"
  case "$normalized_url" in
    */v1)
      printf '%s' "$normalized_url"
      ;;
    *)
      printf '%s/v1' "$normalized_url"
      ;;
  esac
}

verify_api_key_readiness() {
  local label="$1"
  local api_key="$2"
  local api_base_url
  local tmp_dir
  local response_path
  local status_code
  local readiness

  api_base_url="$(normalize_openai_v1_base_url "$BASE_URL")"
  tmp_dir="$(mktemp -d)"
  response_path="${tmp_dir}/usage.json"
  log_info "正在检查 ${label} 专用 Key 和账户余额"
  status_code="$(curl -sS -o "$response_path" -w '%{http_code}' \
    -H "Authorization: Bearer ${api_key}" \
    "${api_base_url}/usage" || true)"

  if [ "$status_code" != "200" ]; then
    rm -rf "$tmp_dir"
    log_error "${label} 专用 Key 验证失败: ${api_base_url}/usage 返回 HTTP ${status_code}"
  fi

  readiness="$(USAGE_RESPONSE_PATH="$response_path" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const data = JSON.parse(fs.readFileSync(process.env.USAGE_RESPONSE_PATH, 'utf8'))
if (data.mode === 'quota_limited' && data.status && !['active', 'quota_exhausted'].includes(data.status)) {
  process.stdout.write('invalid')
} else if (typeof data.remaining === 'number' && data.remaining <= 0) {
  process.stdout.write('insufficient')
} else {
  process.stdout.write('ready')
}


EOF
  )" || {
    rm -rf "$tmp_dir"
    log_error "${label} 余额响应解析失败"
  }
  rm -rf "$tmp_dir"

  if [ "$readiness" = "insufficient" ]; then
    BALANCE_READY=0
    log_warn "${label} 已安装并配置完成，但当前余额/套餐额度不足"
  elif [ "$readiness" != "ready" ]; then
    log_error "${label} 专用 Key 当前不可用，请在网站检查 Key 状态"
  fi

  status_code="$(curl -sS -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer ${api_key}" \
    "${api_base_url}/models" || true)"
  if [ "$status_code" != "200" ]; then
    log_error "${label} 连通性测试失败: ${api_base_url}/models 返回 HTTP ${status_code}"
  fi
  log_info "${label} 专用 Key、余额和连通性检查通过"
}


# 对票据中精确选择的模型和协议执行一次最小真实请求；只验证终态和非空文本，
# 不把 /v1/models 可见性误当成模型可调用证明。
verify_selected_model_request() {
  [ -n "$SELECTED_MODEL" ] && [ -n "$SELECTED_PROTOCOL" ] || return 0

  local api_key="" api_base_url root_url endpoint response_path request_path status_code
  case "$TOOLS" in
    claude) api_key="$CLAUDE_API_KEY" ;;
    codex) api_key="$CODEX_API_KEY" ;;
    grok) api_key="$GROK_API_KEY" ;;
    gemini) api_key="$GEMINI_API_KEY" ;;
    kimi) api_key="$KIMI_API_KEY" ;;
    *) return 0 ;;
  esac
  api_base_url="$(normalize_openai_v1_base_url "$BASE_URL")"
  root_url="${api_base_url%/v1}"
  request_path="$(mktemp)"
  response_path="$(mktemp)"
  trap 'rm -f "$request_path" "$response_path"' RETURN

  case "$SELECTED_PROTOCOL" in
    responses)
      endpoint="${api_base_url}/responses"
      MODEL_ID="$SELECTED_MODEL" "$NODE_BIN" -e 'process.stdout.write(JSON.stringify({model:process.env.MODEL_ID,input:"只回复 CONFIG_OK",max_output_tokens:32}))' >"$request_path"
      status_code="$(curl -sS -o "$response_path" -w '%{http_code}' -H 'Content-Type: application/json' -H "Authorization: Bearer ${api_key}" --data-binary "@${request_path}" "$endpoint" || true)"
      ;;
    chat_completions)
      endpoint="${api_base_url}/chat/completions"
      MODEL_ID="$SELECTED_MODEL" "$NODE_BIN" -e 'process.stdout.write(JSON.stringify({model:process.env.MODEL_ID,messages:[{role:"user",content:"只回复 CONFIG_OK"}],max_tokens:32}))' >"$request_path"
      status_code="$(curl -sS -o "$response_path" -w '%{http_code}' -H 'Content-Type: application/json' -H "Authorization: Bearer ${api_key}" --data-binary "@${request_path}" "$endpoint" || true)"
      ;;
    messages)
      endpoint="${api_base_url}/messages"
      MODEL_ID="$SELECTED_MODEL" "$NODE_BIN" -e 'process.stdout.write(JSON.stringify({model:process.env.MODEL_ID,messages:[{role:"user",content:"只回复 CONFIG_OK"}],max_tokens:32}))' >"$request_path"
      status_code="$(curl -sS -o "$response_path" -w '%{http_code}' -H 'Content-Type: application/json' -H 'anthropic-version: 2023-06-01' -H "x-api-key: ${api_key}" --data-binary "@${request_path}" "$endpoint" || true)"
      ;;
    generate_content)
      endpoint="${root_url}/v1beta/models/${SELECTED_MODEL}:generateContent"
      printf '%s' '{"contents":[{"role":"user","parts":[{"text":"只回复 CONFIG_OK"}]}],"generationConfig":{"maxOutputTokens":32}}' >"$request_path"
      status_code="$(curl -sS -o "$response_path" -w '%{http_code}' -H 'Content-Type: application/json' -H "x-goog-api-key: ${api_key}" --data-binary "@${request_path}" "$endpoint" || true)"
      ;;
    *) log_error "票据返回了不支持的协议: ${SELECTED_PROTOCOL}" ;;
  esac
  [ "$status_code" = "200" ] || log_error "${SELECTED_MODEL} 最小验证失败: ${SELECTED_PROTOCOL} 返回 HTTP ${status_code}"

  RESPONSE_PATH="$response_path" PROTOCOL="$SELECTED_PROTOCOL" "$NODE_BIN" <<'EOF' || log_error "最小验证响应缺少完整终态或文本"
const fs = require('node:fs')
const body = JSON.parse(fs.readFileSync(process.env.RESPONSE_PATH, 'utf8'))
const protocol = process.env.PROTOCOL
let ok = false
if (protocol === 'responses') {
  const output = Array.isArray(body.output) ? body.output : []
  const hasText = output.some(item => item?.type === 'message' && (Array.isArray(item?.content) ? item.content : []).some(part => part?.type === 'output_text' && part?.text))
  ok = body.status === 'completed' || Boolean(body.output_text) || hasText
}
if (protocol === 'chat_completions') ok = Boolean(body.choices?.[0]?.message?.content) && Boolean(body.choices?.[0]?.finish_reason)
if (protocol === 'messages') ok = Array.isArray(body.content) && body.content.some(part => part?.type === 'text' && part.text) && Boolean(body.stop_reason)
if (protocol === 'generate_content') ok = Boolean(body.candidates?.[0]?.content?.parts?.some(part => part?.text)) && Boolean(body.candidates?.[0]?.finishReason)
if (!ok) process.exit(2)
EOF
  rm -f "$request_path" "$response_path"
  trap - RETURN
  log_info "${SELECTED_MODEL} · ${SELECTED_PROTOCOL} 最小真实请求通过"
}

verify_claude_api_key() {
  uses_claude || return 0
  verify_api_key_readiness "Claude Code" "$CLAUDE_API_KEY"
}

verify_codex_api_key() {
  uses_codex || return 0
  verify_api_key_readiness "Codex" "$CODEX_API_KEY"
}

verify_grok_api_key() {
  uses_grok || return 0
  verify_api_key_readiness "Grok Build" "$GROK_API_KEY"
}

verify_gemini_api_key() {
  uses_gemini || return 0
  verify_api_key_readiness "Gemini CLI" "$GEMINI_API_KEY"
}

verify_kimi_api_key() {
  uses_kimi || return 0
  verify_api_key_readiness "Kimi Code" "$KIMI_API_KEY"
}

# 根据用户选择写入 Claude Code 配置。
configure_claude() {
  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]; then
    log_info "正在写入 Claude Code 配置"
    write_claude_config
  fi
}

# 根据用户选择写入 Codex 配置。
configure_codex() {
  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]; then
    log_info "正在写入 Codex 配置"
    write_codex_auth
    write_codex_model_catalog
    write_codex_config
  fi
}

configure_grok() {
  if uses_grok; then
    log_info "正在写入 Grok Build 原生模型配置"
    discover_grok_models
    write_grok_config
  fi
}

configure_gemini() {
  if uses_gemini; then
    log_info "正在写入 Gemini CLI 配置"
    write_gemini_config
  fi
}

configure_kimi() {
  if uses_kimi; then
    log_info "正在写入 Kimi Code 配置"
    write_kimi_config
  fi
}

# 使用绝对路径做最小自检；复用已有客户端时也不会误报“安装失败”。
verify_client_commands() {
  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]; then
    if [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ]; then
      "${NPM_PREFIX}/bin/claude" --version >/dev/null 2>&1 || log_error "Claude Code 安装验证失败"
    elif [ -n "$EXISTING_CLAUDE_COMMAND" ]; then
      "$EXISTING_CLAUDE_COMMAND" --version >/dev/null 2>&1 || log_error "现有 Claude Code CLI 验证失败"
    fi
  fi

  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]; then
    if [ "$INSTALL_CODEX_CLIENT" -eq 1 ]; then
      "${NPM_PREFIX}/bin/codex" --version >/dev/null 2>&1 || log_error "Codex 安装验证失败"
    elif [ -n "$EXISTING_CODEX_COMMAND" ]; then
      "$EXISTING_CODEX_COMMAND" --version >/dev/null 2>&1 || log_error "现有 Codex CLI 验证失败"
    fi
  fi

  if uses_grok; then
    local grok_command="$EXISTING_GROK_COMMAND"
    [ "$INSTALL_GROK_CLIENT" -eq 0 ] || grok_command="$GROK_BIN_PATH"
    if [ -z "$grok_command" ] || ! "$grok_command" --version >/dev/null 2>&1; then
      log_error "Grok Build 安装验证失败"
    fi
  fi

  if uses_gemini; then
    if [ "$INSTALL_GEMINI_CLIENT" -eq 1 ]; then
      "${NPM_PREFIX}/bin/gemini" --version >/dev/null 2>&1 || log_error "Gemini CLI 安装验证失败"
    elif [ -n "$EXISTING_GEMINI_COMMAND" ]; then
      "$EXISTING_GEMINI_COMMAND" --version >/dev/null 2>&1 || log_error "现有 Gemini CLI 验证失败"
    fi
  fi
  if uses_kimi; then
    if [ "$INSTALL_KIMI_CLIENT" -eq 1 ]; then
      "${NPM_PREFIX}/bin/kimi" --version >/dev/null 2>&1 || log_error "Kimi Code 安装验证失败"
    elif [ -n "$EXISTING_KIMI_COMMAND" ]; then
      "$EXISTING_KIMI_COMMAND" --version >/dev/null 2>&1 || log_error "现有 Kimi Code 验证失败"
    fi
  fi
}

# 输出最终结果和下一步指引，帮助用户在新终端中直接使用命令。
print_summary() {
  log_info "老实人 AI 自动配置完成"
  printf '\n'
  printf '  - API 地址: %s\n' "$BASE_URL"
  printf '  - 工具范围: %s\n' "$TOOLS"
  printf '  - Claude 配置: %s\n' "$CLAUDE_SETTINGS_PATH"
  printf '  - Codex 鉴权: %s\n' "$CODEX_AUTH_PATH"
  printf '  - Codex 配置: %s\n' "$CODEX_CONFIG_PATH"
  if uses_grok; then
    printf '  - Grok Build 配置: %s\n' "$GROK_CONFIG_PATH"
  fi
  if uses_gemini; then
    printf '  - Gemini CLI 环境配置: %s\n' "$GEMINI_ENV_PATH"
    printf '  - Gemini CLI 设置: %s\n' "$GEMINI_SETTINGS_PATH"
    printf '  - Gemini CLI 默认模型: %s\n' "$CATALOG_GEMINI_DEFAULT_MODEL"
  fi
  if uses_kimi; then
    printf '  - Kimi Code 配置: %s\n' "$KIMI_CONFIG_PATH"
  fi
  if uses_claude; then
    printf '  - Claude Code 专用 Key: 已配置\n'
    if [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ]; then
      printf '  - Claude Code CLI: 本次已安装\n'
    elif [ -n "$EXISTING_CLAUDE_COMMAND" ]; then
      printf '  - Claude Code CLI: 已保留现有安装 (%s)\n' "$EXISTING_CLAUDE_COMMAND"
    fi
  fi
  if uses_codex; then
    printf '  - Codex 专用 Key: 已配置\n'
    if [ "$INSTALL_CODEX_CLIENT" -eq 1 ]; then
      printf '  - Codex CLI: 本次已安装\n'
    elif [ -n "$EXISTING_CODEX_COMMAND" ]; then
      printf '  - Codex CLI: 已保留现有安装 (%s)\n' "$EXISTING_CODEX_COMMAND"
    fi
  fi
  if uses_grok; then
    printf '  - Grok Build 专用 Key: 已配置\n'
    if [ "$INSTALL_GROK_CLIENT" -eq 1 ]; then
      printf '  - Grok Build CLI: 本次已安装\n'
    elif [ -n "$EXISTING_GROK_COMMAND" ]; then
      printf '  - Grok Build CLI: 已保留现有安装 (%s)\n' "$EXISTING_GROK_COMMAND"
    fi
  fi
  if uses_gemini; then
    printf '  - Gemini CLI 专用 Key: 已配置\n'
    if [ "$INSTALL_GEMINI_CLIENT" -eq 1 ]; then
      printf '  - Gemini CLI: 本次已安装\n'
    elif [ -n "$EXISTING_GEMINI_COMMAND" ]; then
      printf '  - Gemini CLI: 已保留现有安装 (%s)\n' "$EXISTING_GEMINI_COMMAND"
    fi
  fi
  if uses_kimi; then
    printf '  - Kimi Code 专用 Key: 已配置\n'
  fi
  if [ -n "$PROFILE_FILE" ]; then
    printf '  - PATH 已写入: %s\n' "$PROFILE_FILE"
  fi
  printf '\n回滚方法（仅显示本次存在的备份）:\n'
  local rollback_path
  for rollback_path in "$CLAUDE_SETTINGS_PATH" "$CODEX_AUTH_PATH" "$CODEX_CONFIG_PATH" "$CODEX_MODEL_CATALOG_PATH" "$GROK_CONFIG_PATH" "$GEMINI_ENV_PATH" "$GEMINI_SETTINGS_PATH" "$KIMI_CONFIG_PATH"; do
    [ -f "${rollback_path}.bak" ] && printf '  cp %q %q\n' "${rollback_path}.bak" "$rollback_path"
  done
  printf '\n'
  if [ "$BALANCE_READY" -eq 1 ]; then
    printf '✅ 余额/套餐额度充足，现在可以直接使用。\n\n'
  else
    printf '⚠️  安装和配置已经完成，但余额/套餐额度不足。\n'
    printf '   请充值或购买套餐后直接打开使用：%s\n\n' "$DEFAULT_TOPUP_URL"
  fi
  printf '建议执行:\n'
  if [ -n "$PROFILE_FILE" ]; then
    printf '  source %s\n' "$PROFILE_FILE"
  fi
  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]; then
    printf '  claude --version\n'
  fi
  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]; then
    printf '  codex --version\n'
  fi
  if uses_grok; then
    printf '  grok --version\n'
    printf '  grok -m %s -p "只回复 OK"\n' "$CATALOG_GROK_DEFAULT_MODEL"
  fi
  if uses_gemini; then
    printf '  gemini --version\n'
  fi
  if uses_kimi; then
    printf '  kimi --version\n'
  fi
}

# 组织整个安装流程，确保步骤顺序稳定且可复用。
main() {
  log_info "老实人 AI 自动配置脚本 v${SCRIPT_VERSION}"
  require_command curl
  require_command tar
  parse_args "$@"
  TOOLS="$(normalize_tools "$TOOLS")"
  prompt_for_api_keys
  # 一次性凭证和配置文件都使用 Node 做严格 JSON 解析，因此先确保运行时可用。
  ensure_node_runtime
  if [ -n "$SETUP_TOKEN" ]; then
    exchange_setup_ticket
  else
    apply_manual_selection
  fi
  resolve_client_install_plan
  resolve_client_update_plan
  if needs_client_install; then
    ensure_profile_exports
  else
    log_info "检测到所选客户端已存在或已要求跳过安装；不修改 PATH"
  fi
  install_requested_clients
  install_codex_app_if_requested
  ensure_wrapper_scripts
  configure_claude
  configure_codex
  configure_grok
  configure_gemini
  configure_kimi
  verify_claude_api_key
  verify_codex_api_key
  verify_grok_api_key
  verify_gemini_api_key
  verify_kimi_api_key
  verify_selected_model_request
  verify_client_commands
  open_cc_switch_if_requested
  print_summary
}

if [ "${LAOSHIRENAI_INSTALLER_SOURCE_ONLY:-0}" != "1" ]; then
  main "$@"
fi

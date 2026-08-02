#!/usr/bin/env bash

set -euo pipefail

SCRIPT_VERSION="0.5.2"
DEFAULT_BASE_URL="https://api.laoshirenai.com"
DEFAULT_SETUP_EXCHANGE_URL="https://laoshirenai.com/api/v1/public-setup/exchange"
DEFAULT_CODEX_MANIFEST_URL="https://laoshirenai.com/api/v1/public-downloads/codex/latest.json"
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

BASE_URL="${DEFAULT_BASE_URL}"
TOOLS="${DEFAULT_TOOLS}"
CLAUDE_API_KEY="${LAOSHIRENAI_CLAUDE_API_KEY:-}"
CODEX_API_KEY="${LAOSHIRENAI_CODEX_API_KEY:-}"
NODE_VERSION_OVERRIDE="${LAOSHIRENAI_NODE_VERSION:-}"
SKIP_CLIENT_INSTALL=0
FORCE_CLIENT_INSTALL=0
INSTALL_CODEX_APP=0
SETUP_TOKEN="${LAOSHIRENAI_SETUP_TOKEN:-}"
SETUP_EXCHANGE_URL="${LAOSHIRENAI_SETUP_EXCHANGE_URL:-$DEFAULT_SETUP_EXCHANGE_URL}"
CODEX_MANIFEST_URL="${LAOSHIRENAI_CODEX_MANIFEST_URL:-$DEFAULT_CODEX_MANIFEST_URL}"
BALANCE_READY=1

# 兼容统一 API Key 环境变量；若未提供专用 Key，则回退复用统一值。
UNIFIED_API_KEY="${LAOSHIRENAI_API_KEY:-}"
if [ -n "$UNIFIED_API_KEY" ]; then
  [ -n "$CLAUDE_API_KEY" ] || CLAUDE_API_KEY="$UNIFIED_API_KEY"
  [ -n "$CODEX_API_KEY" ] || CODEX_API_KEY="$UNIFIED_API_KEY"
fi

# 支持通过环境变量覆盖基础参数，兼容管道执行或预置 shell 环境。
ENV_BASE_URL="${LAOSHIRENAI_BASE_URL:-}"
ENV_TOOLS="${LAOSHIRENAI_TOOLS:-}"
[ -n "$ENV_BASE_URL" ] && BASE_URL="$ENV_BASE_URL"
[ -n "$ENV_TOOLS" ] && TOOLS="$ENV_TOOLS"
[ "${LAOSHIRENAI_SKIP_CLIENT_INSTALL:-0}" = "1" ] && SKIP_CLIENT_INSTALL=1
[ "${LAOSHIRENAI_FORCE_CLIENT_INSTALL:-0}" = "1" ] && FORCE_CLIENT_INSTALL=1
[ "${LAOSHIRENAI_INSTALL_CODEX_APP:-0}" = "1" ] && INSTALL_CODEX_APP=1

NODE_BIN=""
NPM_BIN=""
PROFILE_FILE=""
PLATFORM_ID=""
NODE_ARCHIVE_NAME=""
USE_PROXYLESS_NPM=0
ACTIVE_NPM_REGISTRY="${DEFAULT_NPM_REGISTRY}"
INSTALL_CLAUDE_CLIENT=0
INSTALL_CODEX_CLIENT=0
EXISTING_CLAUDE_COMMAND=""
EXISTING_CODEX_COMMAND=""

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
      printf 'export PATH="%s/bin:%s/bin:%s:$PATH"\n' "$NODE_CURRENT_DIR" "$NPM_PREFIX" "$LOCAL_BIN_DIR"
      printf '%s\n' "$marker_end"
    } >>"$PROFILE_FILE"
    log_info "已写入 PATH 到 ${PROFILE_FILE}"
  fi

  export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:${LOCAL_BIN_DIR}:${PATH}"
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
    all|claude|codex)
      printf '%s' "$normalized_value"
      ;;
    *)
      log_error "不支持的 --tools 值: $1，可选值为 all / claude / codex"
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
  bash install.sh --api-key <Claude_API_Key> [--codex-api-key <Codex_API_Key>] [--tools all|claude|codex] [--base-url https://api.laoshirenai.com]

参数:
  --api-key             Claude Code API Key
  --codex-api-key       Codex API Key
  --tools               需要配置的工具，默认 all
  --base-url            API 基础地址，默认 https://api.laoshirenai.com
  --node-version        指定 Node.js 版本，例如 v24.11.0
  --skip-client-install 仅写配置，不安装 claude/codex 包
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
}

needs_client_install() {
  [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ] || [ "$INSTALL_CODEX_CLIENT" -eq 1 ]
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
  [ "$node_major" -ge "$MIN_NODE_MAJOR" ]
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
if (!data || !['claude', 'codex'].includes(data.target) || !data.api_key || !data.base_url) {
  process.exit(2)
}
process.stdout.write([
  Buffer.from(String(data.target)).toString('base64'),
  Buffer.from(String(data.api_key)).toString('base64'),
  Buffer.from(String(data.base_url)).toString('base64')
].join(':'))
EOF
  )" || {
    rm -rf "$tmp_dir"
    log_error "服务器返回的一键安装配置格式无效"
  }
  rm -rf "$tmp_dir"

  IFS=: read -r target received_key received_base_url <<<"$parsed"
  target="$(printf '%s' "$target" | base64 -d)"
  received_key="$(printf '%s' "$received_key" | base64 -d)"
  received_base_url="$(printf '%s' "$received_base_url" | base64 -d)"
  [ "$target" = "$TOOLS" ] || log_error "安装凭证与当前工具不匹配，请重新生成"

  BASE_URL="$received_base_url"
  if [ "$target" = "claude" ]; then
    CLAUDE_API_KEY="$received_key"
  else
    CODEX_API_KEY="$received_key"
  fi
  SETUP_TOKEN=""
  unset LAOSHIRENAI_SETUP_TOKEN SETUP_TICKET
  log_info "专用配置领取成功"
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

  ensure_dir "$NPM_PREFIX"
  detect_broken_local_proxy
  if [ "$USE_PROXYLESS_NPM" -eq 1 ]; then
    log_warn "检测到本地代理环境变量，安装客户端时将临时绕过代理"
  fi
  ensure_npm_registry "$DEFAULT_NPM_REGISTRY"

  if [ "$INSTALL_CLAUDE_CLIENT" -eq 1 ]; then
    log_info "正在安装 Claude Code"
    npm_install_with_fallback "@anthropic-ai/claude-code@latest"
  fi

  if [ "$INSTALL_CODEX_CLIENT" -eq 1 ]; then
    log_info "正在安装 Codex"
    npm_install_with_fallback "@openai/codex@latest"
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
    https://laoshirenai.com/api/v1/public-downloads/codex/packages/*) ;;
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

  CONFIG_PATH="$CLAUDE_SETTINGS_PATH" CONFIG_BASE_URL="$BASE_URL" CONFIG_API_KEY="$CLAUDE_API_KEY" "$NODE_BIN" <<'EOF'
const fs = require('node:fs')
const path = process.env.CONFIG_PATH
const baseUrl = process.env.CONFIG_BASE_URL
const apiKey = process.env.CONFIG_API_KEY

let config = {}
if (fs.existsSync(path)) {
  try {
    config = JSON.parse(fs.readFileSync(path, 'utf8'))
  } catch (error) {
    config = {}
  }
}

if (!config || typeof config !== 'object' || Array.isArray(config)) {
  config = {}
}

if (!config.env || typeof config.env !== 'object' || Array.isArray(config.env)) {
  config.env = {}
}

config.model = 'claude-opus-5'
config.effortLevel = 'xhigh'
config.env.ANTHROPIC_BASE_URL = baseUrl
config.env.ANTHROPIC_AUTH_TOKEN = apiKey
config.env.CLAUDE_CODE_ATTRIBUTION_HEADER = '0'
config.env.CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY = '1'

fs.writeFileSync(path, `${JSON.stringify(config, null, 2)}\n`, 'utf8')
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
    config = {}
  }
}

if (!config || typeof config !== 'object' || Array.isArray(config)) {
  config = {}
}

config.OPENAI_API_KEY = apiKey
fs.writeFileSync(path, `${JSON.stringify(config, null, 2)}\n`, 'utf8')
EOF
}

# 生成 Codex 的核心 TOML 配置，第一版采用确定性覆盖策略并配合备份保证可回滚。
write_codex_config() {
  create_backup_if_needed "$CODEX_CONFIG_PATH"
  ensure_dir "$(dirname "$CODEX_CONFIG_PATH")"

  cat >"$CODEX_CONFIG_PATH" <<EOF
model_provider = "OpenAI"
model = "gpt-5.6-sol"
review_model = "gpt-5.6-sol"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
preferred_auth_method = "apikey"

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${BASE_URL}"
wire_api = "responses"
requires_openai_auth = true
EOF
}

uses_codex() {
  [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]
}

uses_claude() {
  [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]
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
    return 0
  fi
  if [ "$readiness" != "ready" ]; then
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

verify_claude_api_key() {
  uses_claude || return 0
  verify_api_key_readiness "Claude Code" "$CLAUDE_API_KEY"
}

verify_codex_api_key() {
  uses_codex || return 0
  verify_api_key_readiness "Codex" "$CODEX_API_KEY"
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
    write_codex_config
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
  if [ -n "$PROFILE_FILE" ]; then
    printf '  - PATH 已写入: %s\n' "$PROFILE_FILE"
  fi
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
}

# 组织整个安装流程，确保步骤顺序稳定且可复用。
main() {
  log_info "老实人 AI 自动配置脚本 v${SCRIPT_VERSION}"
  require_command curl
  require_command tar
  parse_args "$@"
  TOOLS="$(normalize_tools "$TOOLS")"
  prompt_for_api_keys
  exchange_setup_ticket
  resolve_client_install_plan
  ensure_node_runtime
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
  verify_claude_api_key
  verify_codex_api_key
  verify_client_commands
  print_summary
}

main "$@"

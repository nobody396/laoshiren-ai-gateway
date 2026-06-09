#!/usr/bin/env bash

set -euo pipefail

SCRIPT_VERSION="0.1.0"
DEFAULT_BASE_URL="https://api.laoshirenai.com"
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

NODE_BIN=""
NPM_BIN=""
PROFILE_FILE=""
PLATFORM_ID=""
NODE_ARCHIVE_NAME=""
USE_PROXYLESS_NPM=0
ACTIVE_NPM_REGISTRY="${DEFAULT_NPM_REGISTRY}"

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
      printf 'export PATH="%s/bin:%s/bin:%s:$PATH"\n' "$NODE_CURRENT_DIR" "$NPM_PREFIX" "$LOCAL_BIN_DIR"
      printf '%s\n' "$marker_end"
    } >>"$PROFILE_FILE"
    log_info "已写入 PATH 到 ${PROFILE_FILE}"
  fi

  export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:${LOCAL_BIN_DIR}:${PATH}"
}

# 为 claude 和 codex 生成稳定包装脚本，避免用户切换终端后找不到 node 运行时。
ensure_wrapper_scripts() {
  ensure_dir "$LOCAL_BIN_DIR"

  cat >"${LOCAL_BIN_DIR}/claude" <<EOF
#!/usr/bin/env bash
export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:\$PATH"
exec "${NPM_PREFIX}/bin/claude" "\$@"
EOF
  chmod +x "${LOCAL_BIN_DIR}/claude"

  cat >"${LOCAL_BIN_DIR}/codex" <<EOF
#!/usr/bin/env bash
export PATH="${NODE_CURRENT_DIR}/bin:${NPM_PREFIX}/bin:\$PATH"
exec "${NPM_PREFIX}/bin/codex" "\$@"
EOF
  chmod +x "${LOCAL_BIN_DIR}/codex"
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
  if [ "$SKIP_CLIENT_INSTALL" -eq 1 ]; then
    log_warn "已跳过客户端安装，仅写入配置文件"
    return 0
  fi

  ensure_dir "$NPM_PREFIX"
  detect_broken_local_proxy
  if [ "$USE_PROXYLESS_NPM" -eq 1 ]; then
    log_warn "检测到本地代理环境变量，安装客户端时将临时绕过代理"
  fi
  ensure_npm_registry "$DEFAULT_NPM_REGISTRY"

  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]; then
    log_info "正在安装 Claude Code"
    npm_install_with_fallback "@anthropic-ai/claude-code@latest"
  fi

  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]; then
    log_info "正在安装 Codex"
    npm_install_with_fallback "@openai/codex@latest"
  fi
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

config.env.ANTHROPIC_BASE_URL = baseUrl
config.env.ANTHROPIC_AUTH_TOKEN = apiKey
config.env.CLAUDE_CODE_ATTRIBUTION_HEADER = '0'
config.env.CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY = '1'
config.env.ANTHROPIC_CUSTOM_MODEL_OPTION = 'claude-fable-5'
config.env.ANTHROPIC_CUSTOM_MODEL_OPTION_NAME = 'Claude Fable 5'
config.env.ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION = 'Claude Fable 5 via 老实人AI gateway'

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
model = "gpt-5.4"
review_model = "gpt-5.4"
model_reasoning_effort = "high"
disable_response_storage = true
network_access = "enabled"

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${BASE_URL}"
wire_api = "responses"
requires_openai_auth = true
EOF
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

# 使用安装后的可执行文件做一次最小自检，证明命令确实可运行。
verify_client_commands() {
  if [ "$SKIP_CLIENT_INSTALL" -eq 1 ]; then
    return 0
  fi

  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]; then
    "${NPM_PREFIX}/bin/claude" --version >/dev/null 2>&1 || log_error "Claude Code 安装验证失败"
  fi

  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]; then
    "${NPM_PREFIX}/bin/codex" --version >/dev/null 2>&1 || log_error "Codex 安装验证失败"
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
  printf '  - PATH 已写入: %s\n' "$PROFILE_FILE"
  printf '\n'
  printf '建议执行:\n'
  printf '  source %s\n' "$PROFILE_FILE"
  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "claude" ]; then
    printf '  claude --version\n'
  fi
  if [ "$TOOLS" = "all" ] || [ "$TOOLS" = "codex" ]; then
    printf '  codex --version\n'
  fi
}

# 组织整个安装流程，确保步骤顺序稳定且可复用。
main() {
  require_command curl
  require_command tar
  parse_args "$@"
  TOOLS="$(normalize_tools "$TOOLS")"
  prompt_for_api_keys
  ensure_node_runtime
  ensure_profile_exports
  install_requested_clients
  ensure_wrapper_scripts
  configure_claude
  configure_codex
  verify_client_commands
  print_summary
}

main "$@"

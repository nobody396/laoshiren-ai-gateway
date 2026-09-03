#!/usr/bin/env bash
set -euo pipefail
ROOT=/tmp/lsr-client-config-qa
NODE="$ROOT/node"
pass=0
run_claude(){ local h; h=$(mktemp -d); HOME="$h" LAOSHIRENAI_INSTALLER_SOURCE_ONLY=1 bash -c '
  source "$1"; NODE_BIN="$2"; BASE_URL=https://api.example.com; CLAUDE_API_KEY=test-key; CATALOG_ANTHROPIC_DEFAULT_MODEL=claude-opus-5
  mkdir -p "$HOME/.claude"; printf "%s\n" "{\"permissions\":{\"allow\":[\"keep\"]},\"env\":{\"KEEP\":\"yes\"}}" > "$HOME/.claude/settings.json"
  cp "$HOME/.claude/settings.json" "$HOME/original"; write_claude_config; first=$(sha256sum "$HOME/.claude/settings.json"|cut -d" " -f1); write_claude_config; second=$(sha256sum "$HOME/.claude/settings.json"|cut -d" " -f1); test "$first" = "$second"; cmp "$HOME/original" "$HOME/.claude/settings.json.bak"; "$NODE_BIN" -e "const d=require(process.env.HOME+\"/.claude/settings.json\");if(d.permissions.allow[0]!==\"keep\"||d.env.KEEP!==\"yes\"||d.model!==\"claude-opus-5\"||d.modelSettings[\"claude-opus-5\"].effortLevel!==\"high\")process.exit(1)"
' _ "$ROOT/install.sh" "$NODE"; rm -rf "$h"; }
run_codex(){ local h; h=$(mktemp -d); HOME="$h" LAOSHIRENAI_INSTALLER_SOURCE_ONLY=1 bash -c '
  source "$1"; NODE_BIN="$2"; BASE_URL=https://api.example.com; CODEX_API_KEY=test-key; CATALOG_OPENAI_DEFAULT_MODEL=qwen3.7-max
  mkdir -p "$HOME/.codex"; cp "$3" "$HOME/.codex/laoshirenai-model-catalog.json"; printf "%s\n" "model = \"old\"" "notify = [\"keep\"]" "" "[mcp_servers.keep]" "command = \"keep\"" > "$HOME/.codex/config.toml"; printf "%s\n" "{\"KEEP\":\"yes\"}" > "$HOME/.codex/auth.json"; cp "$HOME/.codex/config.toml" "$HOME/original"
  write_codex_auth; write_codex_config; first=$(sha256sum "$HOME/.codex/config.toml"|cut -d" " -f1); write_codex_config; second=$(sha256sum "$HOME/.codex/config.toml"|cut -d" " -f1); test "$first" = "$second"; cmp "$HOME/original" "$HOME/.codex/config.toml.bak"; grep -q "\[mcp_servers.keep\]" "$HOME/.codex/config.toml"; grep -q "model = \"qwen3.7-max\"" "$HOME/.codex/config.toml"; "$NODE_BIN" -e "const d=require(process.env.HOME+\"/.codex/auth.json\");if(d.KEEP!==\"yes\"||d.OPENAI_API_KEY!==\"test-key\")process.exit(1)"
' _ "$ROOT/install.sh" "$NODE" "$ROOT/codex-model-catalog.json"; rm -rf "$h"; }
run_grok(){ local h; h=$(mktemp -d); HOME="$h" LAOSHIRENAI_INSTALLER_SOURCE_ONLY=1 bash -c '
  source "$1"; NODE_BIN="$2"; BASE_URL=https://api.example.com; GROK_API_KEY=test-key; CATALOG_GROK_DEFAULT_MODEL=qwen3.7-max; GROK_API_BACKEND=responses; CATALOG_GROK_MANAGED_MODELS_JSON='"'"'[{"id":"qwen3.7-max","display_name":"Qwen 3.7 Max","context_window":1000000}]'"'"'
  mkdir -p "$HOME/.grok"; printf "%s\n" "[preferences]" "theme = \"dark\"" > "$HOME/.grok/config.toml"; cp "$HOME/.grok/config.toml" "$HOME/original"; write_grok_config; first=$(sha256sum "$HOME/.grok/config.toml"|cut -d" " -f1); write_grok_config; second=$(sha256sum "$HOME/.grok/config.toml"|cut -d" " -f1); test "$first" = "$second"; cmp "$HOME/original" "$HOME/.grok/config.toml.bak"; grep -q "theme = \"dark\"" "$HOME/.grok/config.toml"; grep -q "api_backend = \"responses\"" "$HOME/.grok/config.toml"
' _ "$ROOT/install.sh" "$NODE"; rm -rf "$h"; }
run_gemini(){ local h; h=$(mktemp -d); HOME="$h" LAOSHIRENAI_INSTALLER_SOURCE_ONLY=1 bash -c '
  source "$1"; NODE_BIN="$2"; BASE_URL=https://api.example.com; GEMINI_API_KEY=test-key; CATALOG_GEMINI_DEFAULT_MODEL=gemini-3.7-flash; CATALOG_GEMINI_MANAGED_MODELS=gemini-3.7-flash
  mkdir -p "$HOME/.gemini"; printf "%s\n" "KEEP=yes" > "$HOME/.gemini/.env"; printf "%s\n" "{\"theme\":\"dark\"}" > "$HOME/.gemini/settings.json"; cp "$HOME/.gemini/settings.json" "$HOME/original"; write_gemini_config; first=$(sha256sum "$HOME/.gemini/settings.json"|cut -d" " -f1); write_gemini_config; second=$(sha256sum "$HOME/.gemini/settings.json"|cut -d" " -f1); test "$first" = "$second"; cmp "$HOME/original" "$HOME/.gemini/settings.json.bak"; "$NODE_BIN" -e "const d=require(process.env.HOME+\"/.gemini/settings.json\");if(d.theme!==\"dark\"||d.model.name!==\"gemini-3.7-flash\")process.exit(1)"; grep -q "KEEP=yes" "$HOME/.gemini/.env"
' _ "$ROOT/install.sh" "$NODE"; rm -rf "$h"; }
run_claude; echo PASS claude
run_codex; echo PASS codex
run_grok; echo PASS grok
run_gemini; echo PASS gemini

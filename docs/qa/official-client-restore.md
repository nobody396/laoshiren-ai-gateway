# Official client restore — local acceptance

Scope: Codex and Claude Code, not a factory reset or subscription purchase.
The modal reuses `CcsClientIcon`; macOS/Linux share a POSIX command, Windows uses
PowerShell. No API Key or setup ticket appears in the copied command.

## Ownership

- Codex: remove the one-click `laoshirenai_responses` provider or CC Switch
  `custom` provider **only for the exact gateway endpoint**, remove managed
  root fields and the API Key, preserve existing ChatGPT tokens, other providers,
  MCP and history. Do not delete model catalog files. Conflicting overrides stop
  before mutation. TOML formatting/comments are not preserved.
- Claude Code: remove managed fields from user `settings.json` only when the
  endpoint is exactly ours (including `/antigravity`). Preserve unrelated model
  preferences, env entries, hooks, permissions and MCP. Never edit
  `.credentials.json`, Keychain, `.claude.json`, project or organization settings.
  `CLAUDE_CONFIG_DIR` is respected. Other credentials/helpers/forced login or
  terminal provider overrides block the write.
- Both: preflight, private recoverable backup, atomic replacement, exact file
  readback, second run byte-identical. Download hashes are generated from the
  committed standalone scripts. No successful file write claims successful
  official authentication or a paid model request.

Official references: [Claude settings and precedence](https://code.claude.com/docs/en/settings),
[Claude authentication](https://code.claude.com/docs/en/authentication).

## Verified locally

- `RESTORE_TEST_SHELL=pwsh pnpm --dir frontend run restore:test`: 55 passing.
  Executes actual page-generated commands against a local HTTP fixture through
  bash, zsh and macOS PowerShell 7. Success, network failure, bad checksum and
  child failure all tested. Temporary downloads are removed in every case.
- `pnpm --dir frontend run test:run`: 191 files / 979 tests passing.
- Typecheck, production build, frontend boundary and bundle budget pass.
- Modified UI/TS/build files lint clean. CJS fixtures lint clean with only the
  TypeScript `no-var-requires` rule disabled because they are CommonJS.
- Local component preview: `http://127.0.0.1:4187/_restore-qa.html`, Chrome via
  Playwright (Browser plugin unavailable). 1440×1000 and 390×844; light/dark,
  icons loaded, both tool choices, OS switch, exact clipboard readback, close,
  no page/console errors or framework error overlay. No customer API or session.
  Preview files/server removed after QA. Screenshots and experiment results kept
  at `/Users/fujunhao/laoshirenai/local/qa/official-client-restore`.

## Ablation

Only temporary copies were mutated; the shipped implementation was untouched.

| Removed | Regression |
| --- | --- |
| Backup write | Recovery assertion fails (1 test) |
| Exact endpoint guard | Third-party/lookalike endpoints are modified (2 tests fail) |
| Terminal env guard | Token/cloud overrides slip through (2 tests fail) |

Keep these three safeguards. Avoid a generic restore adapter registry, plugin
framework, extra API, setup ticket, duplicate icon map or duplicate download
renderer. Two small client-specific scripts and one download renderer suffice.

## Remaining boundaries

Native Windows PowerShell 5.1/7 and Linux acceptance run in the existing CI
workflows; macOS pwsh alone is **not** native Windows evidence. Real official
login and billable requests were not executed. Nothing was deployed and the
owner's real local client files were not touched. Other clients remain visibly
unsupported: Grok Build, Gemini CLI, OpenCode, Kimi Code, ZCode, WorkBuddy and
CC Switch's OpenClaw/Hermes. Setup support does not prove restore support.

# Text client guides — owner preview

Scope: owner-requested concise text configuration guides for every client in the current 14-client matrix. Keep existing graphical configuration source unchanged and do not publish it. The owner requested a local preview before deciding further work.

- Three steps: obtain a Key/model ID, configure the selected tool, start and verify.
- Show client-specific Base URL, file locations, OS notes and concrete reference syntax, without promising that every model/protocol/OS combination was tested.
- Reuse DocsTerminalCommand for macOS-style windows and clipboard, and DocsToc for document navigation. OS/tool tabs navigate text; no Key inputs, API execution or configuration generators.
- All 14 tools stay represented. Qoder and MiniMax now have native-configuration instructions with explicit account/credential gates, not fabricated file templates. Native third-party settings steps are distinct from our unpublished graphical generator.
- Keep production routes, public feature flags, capability matrices, installer code and the old graphical components byte-identical.
- The local-only Vite entry lives outside the repository at local/customer-guides/drafts/text-docs-preview-20260904. It binds loopback and rejects /api/ requests. Review URL: http://127.0.0.1:8910/text-docs-preview.
- Configuration snippets are reference drafts, not newly completed real-client evidence. Never enter real credentials into the preview or execute snippets against personal client files during UI QA.

## Kimi Code follow-up

- Replaced the Kimi placeholder with the official environment-only recipe: `KIMI_MODEL_API_KEY`, `KIMI_MODEL_PROVIDER_TYPE=openai_responses`, `KIMI_MODEL_BASE_URL` ending in `/v1`, and `KIMI_MODEL_NAME`.
- Current executable 0.40.1, macOS arm64. Isolated fake-upstream check verified authentication, Responses path and unchanged config; dummy credential was absent from output and the temporary file tree.
- Owned user 2 / key 128 / group 6 remained unchanged. Two real runs on `gpt-5.4` read a fixture and returned its exact contents; the final stream includes a `Read` tool call, tool-result message, and assistant final reply. Initial attempt exited without the marker; its cause was not retained, so this is not a reliability or all-model acceptance claim.
- No plaintext credential in captured output or isolated client files; original client configuration was never touched. Windows/Linux snippets are supplied but not claimed as native-runtime verified.
- Shared terminal renders OS-specific blocks; preview stays local and the graphical generator, routes and capability matrix stay unchanged.
- Secret-free receipts and screenshots: `local/customer-guides/drafts/text-docs-preview-20260904/kimi-verification/` and adjacent `kimi-*.png` (workspace-local, not published).

## MiniMax Code and Qoder follow-up

- Checked official registry releases: `@minimax-ai/code@0.3.2`, `@qoder-ai/qodercli@1.1.42`; downloaded to the local audit folder and verified SHA-512 integrity. No global installs, real credentials, login changes, or production writes.
- MiniMax: isolated provider add/list/test succeeded using a dummy key and loopback server. Observed `/v1/responses`, expected model and Bearer authentication. The key value was persisted to `config.yaml`; `--api-key-env` is an import, not a runtime secret reference. New docs use the native `/model` → Add 3rd-party provider → Custom Provider workflow and warn clearly about plaintext storage. `provider add --use` initially failed its test-before-activation gate; no unsafe one-line import is published.
- Qoder: inspected the exact release's Custom URL wizard (URL → model name → display name → openai/anthropic style → api_key), conditional on `allow_byok >= 2`. Current isolated `--list-models` is login-gated. BYOK check sends URL/model/credential to Qoder's authenticated service. Docs give the conditional native route without claiming arbitrary-account availability or a direct credential-only client path. No login gate bypass, no real key sent to Qoder, no invented settings.json storage.
- Neither client is newly production-E2E verified. MiniMax is mock configuration/transport verified on macOS; Qoder is source/help/login-gate verified. Windows/Linux runtime and every model combination remain unverified. Capability matrices and public visibility are unchanged.
- UI: 11 focused tests, typecheck, targeted ESLint passed; desktop/mobile layouts have no page overflow and retain three sections and zero credential inputs. Screenshots and receipts remain under the local preview directory's `remaining-clients/` and adjacent image files.

## Key-scoped one-click setup implementation (owner request)

User acceptance: after creating a Key and choosing its groups, the existing key-row action opens a client icon picker; selecting client and macOS/Linux/Windows yields one command, installs missing clients, skips matching installed clients, configures and tests. The Key is reused, not rebound, and groups/models are not hard-coded.

Implemented local slice:
- `ClientSetupModal` reuses BaseDialog and DocsTerminalCommand; all 14 matrix client identities/icons are shown. Non-ready exact-version/OS adapters are explanation-only, never promoted by this UI.
- Authenticated `GET /resources/setup-plans` derives each plan from fresh Key/payer permissions, ordered group declarations, model protocols and generated client contracts. A new setup-ticket purpose binds the plan fingerprint. Issuance and one-time redemption both re-evaluate authority/catalog; stale plans and expired/disabled/foreign keys fail closed.
- Installer v0.7.16 consumes the metadata plan, verifies OS and exact client version, re-reads authenticated model discovery before writing, installs missing npm clients at the bound version, and does not auto-update an existing plan client. Existing legacy flows remain separate.
- Codex catalog is restricted to all planned IDs (missing metadata fails closed); Grok writes per-model protocols into owned `laoshirenai/` aliases and removes retired owned aliases while preserving other models; fixed-slot tools receive a default and their existing tool-specific configuration. Gemini 0.57.0 plan flow uses gateway auth.
- Automatic checks currently cover installed version, authorization/model discovery and a default-model API reply with terminal text. These are not a real client file-tool continuation loop. Never claim all client runtime flows passed based on these checks.

Not complete / release blockers:
- Only the four existing ready installer targets (Claude Code, Codex, Grok Build, Gemini CLI) are eligible from the generated matrix; the other ten require dedicated installers, safe credential handling and exact-version/native-OS acceptance. Kimi tutorial success does not promote installer status. Qoder login/Custom URL and MiniMax plaintext storage gates remain.
- Need native Windows 5.1/7 and Linux fixture CI for this diff, then exact real-client fixtures and any missing model metadata closure before publishing. No live credentials used in local installer tests. No production deployment.
- macOS/Linux shell tests and Windows fixture tests are isolated, ordinary-user oriented; no personal client config files are modified. The Windows tests exercise Restricted execution policy, safe shim choice, non-ASCII/space paths and idempotency.

Preview: `/client-setup-preview` at the existing loopback Vite server. This is a clearly labelled mock Key/three-group interaction, not production authorization. Its commands are non-executable placeholders; real commands require authenticated backend tickets.

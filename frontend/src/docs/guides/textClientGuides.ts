import { clientMatrix } from '@/generated/clientMatrix'

export type GuideOS = 'macos' | 'windows' | 'linux'
export type GuideMode = 'file' | 'environment' | 'native' | 'pending'
export interface GuideBlock { label: string; content: string; candidate?: boolean; operatingSystems?: GuideOS[] }
export interface TextClientGuide {
  id: string
  scope: string
  referenceVersion: string
  verification?: Partial<Record<GuideOS, string>>
  mode: GuideMode
  navLabel?: string
  notice?: { title: string; body: string }
  baseUrl: string
  protocol: string
  files: Partial<Record<GuideOS, string[]>>
  keyEnv?: string
  environment?: Record<string, string>
  blocks: GuideBlock[]
  instructions: string[]
  notes: string[]
  launch?: string
  sources: Array<{ label: string; url: string }>
}

const root = 'https://api.laoshirenai.com'
const v1 = `${root}/v1`
const json = (value: unknown) => JSON.stringify(value, null, 2)
const filePaths = (unix: string[], windows: string[]) => ({ macos: unix, linux: unix, windows })

// Editorial examples, not a replacement capability matrix. Client identities and
// reference versions remain in generated/clientMatrix. No recipe promotes a
// protocol, OS, model or one-click status. Publication requires separate review.
export const textClientGuides: readonly TextClientGuide[] = [
  {
    id: 'claude-code', referenceVersion: 'cli:2.1.251', scope: 'Claude Code CLI，不是 Claude Desktop', mode: 'file', baseUrl: root, protocol: 'Messages',
    files: filePaths(['~/.claude/settings.json'], ['%USERPROFILE%\\.claude\\settings.json']), keyEnv: 'ANTHROPIC_AUTH_TOKEN',
    blocks: [{ label: 'settings.json · 合并以下字段', content: json({ model: 'YOUR_MODEL_ID', env: { ANTHROPIC_BASE_URL: root, ANTHROPIC_MODEL: 'YOUR_MODEL_ID', ANTHROPIC_DEFAULT_OPUS_MODEL: 'YOUR_MODEL_ID', ANTHROPIC_DEFAULT_SONNET_MODEL: 'YOUR_MODEL_ID', ANTHROPIC_DEFAULT_HAIKU_MODEL: 'YOUR_MODEL_ID' } }) }],
    instructions: ['先备份 settings.json；不存在时创建 .claude 文件夹和该文件。', '将示例中的 YOUR_MODEL_ID 全部替换为同一个可用模型 ID；已有 env 字段时合并，不要新建第二个 env。'],
    notes: ['Key 从当前终端的 ANTHROPIC_AUTH_TOKEN 读取，不放进这份 JSON 示例。', '简版先把主模型和三个模型槽位设成同一个模型，槽位调用也按该模型计费；以后可分别修改。', 'Base URL 不加 /v1，也不填 /v1/messages。不要把 CLI 配置套给桌面版或编辑器扩展。'], launch: 'claude --version\nclaude',
    sources: [{ label: 'Claude Code 官方配置', url: 'https://code.claude.com/docs/en/settings' }],
  },
  {
    id: 'codex', referenceVersion: 'cli:0.151.0', scope: 'Codex CLI · 使用用户级配置', mode: 'file', baseUrl: v1, protocol: 'Responses',
    files: filePaths(['~/.codex/config.toml'], ['%USERPROFILE%\\.codex\\config.toml']),
    blocks: [{ label: 'config.toml · 用户目录，不是项目目录', content: `model_provider = "laoshirenai"\nmodel = "YOUR_MODEL_ID"\n\n[model_providers.laoshirenai]\nname = "老实人AI"\nbase_url = "${v1}"\nwire_api = "responses"\nenv_key = "LAOSHIRENAI_API_KEY"\nrequires_openai_auth = false` }],
    instructions: ['先备份 config.toml。model_provider 和 model 放在文件顶层，不能放到其他 [section] 内。', '已有 [model_providers.laoshirenai] 时修改原段落；不要重复追加同名段落。'],
    notes: ['本例使用环境变量认证，不修改原有 auth.json。', '这里是最终写入文件的地址：带 /v1。wire_api 保持 responses，不改成 chat。', '如果设置过 CODEX_HOME，编辑该目录中的 config.toml；其他模型可能还需要专用模型元数据。'], launch: 'codex --version\ncodex',
    sources: [{ label: 'Codex 官方配置', url: 'https://learn.chatgpt.com/zh-Hans/docs/config-file/config-advanced' }],
  },
  {
    id: 'grok-build', referenceVersion: 'cli:1.0.13', scope: 'Grok Build CLI，不是 Grok 网页聊天', mode: 'file', baseUrl: v1, protocol: 'Responses 示例',
    files: filePaths(['~/.grok/config.toml'], ['%USERPROFILE%\\.grok\\config.toml']),
    blocks: [{ label: 'config.toml', content: `[models]\ndefault = "laoshirenai"\n\n[model.laoshirenai]\nmodel = "YOUR_MODEL_ID"\nbase_url = "${v1}"\nenv_key = "LAOSHIRENAI_API_KEY"\napi_backend = "responses"` }],
    instructions: ['将这两个段落合并到用户级配置，不要覆盖其他模型条目。', 'default 指向本地条目 laoshirenai；model 才是发给接口的模型 ID。'],
    notes: ['设置过 GROK_HOME 时，使用实际目录。不同协议要单独核对，不只改地址。'], launch: 'grok --version\ngrok',
    sources: [{ label: 'Grok Build 官方设置', url: 'https://docs.x.ai/build/settings' }],
  },
  {
    id: 'kimi-code', referenceVersion: 'cli:0.40.1', verification: { macos: 'macOS · gpt-5.4 文件读取实测通过' }, scope: 'Kimi Code CLI · 本次会话接入，不改原配置', mode: 'environment', baseUrl: v1, protocol: 'Responses',
    files: filePaths(['~/.kimi-code/config.toml（本教程不修改）'], ['%USERPROFILE%\\.kimi-code\\config.toml（本教程不修改）']), keyEnv: 'KIMI_MODEL_API_KEY',
    blocks: [
      { label: 'Bash / Zsh · 替换模型 ID 后执行', operatingSystems: ['macos', 'linux'], content: `export KIMI_MODEL_PROVIDER_TYPE='openai_responses'\nexport KIMI_MODEL_BASE_URL='${v1}'\nexport KIMI_MODEL_NAME='YOUR_MODEL_ID'` },
      { label: 'PowerShell · 替换模型 ID 后执行', operatingSystems: ['windows'], content: `$env:KIMI_MODEL_PROVIDER_TYPE = 'openai_responses'\n$env:KIMI_MODEL_BASE_URL = '${v1}'\n$env:KIMI_MODEL_NAME = 'YOUR_MODEL_ID'` },
    ],
    instructions: ['不用编辑文件。在第一步输入 Key 的同一个终端中执行下面三行。', '将 YOUR_MODEL_ID 换成这把 Key 有权调用、支持 Responses 和工具调用的模型 ID；不是模型的中文名称。'],
    notes: ['Key 由第一步放入 KIMI_MODEL_API_KEY；只设置 KIMI_API_KEY 或 OPENAI_API_KEY 不会生效。', 'Base URL 带 /v1，不加 /responses。不要运行 kimi login 来填写本站 Key，那是官方账号登录。', '环境配置优先于 config.toml 的默认模型；启动时不要再加 -m，否则会覆盖本教程的模型选择。', '关闭终端后本次设置失效；下次重新执行这三步。原 config.toml、官方登录和其他供应商配置都保留。', '本次实测为 macOS 的 0.40.1；Windows 与 Linux 提供对应环境变量语法，尚未做原生客户端实测。不是列表中的所有模型都已逐一验收。'],
    launch: 'kimi --version\nkimi',
    sources: [{ label: 'Kimi Code 官方环境变量配置', url: 'https://moonshotai.github.io/kimi-code/en/configuration/env-vars.html' }],
  },
  {
    id: 'opencode', referenceVersion: 'cli:1.18.15', scope: 'OpenCode V1 · 1.18.15 配置结构', mode: 'file', baseUrl: v1, protocol: 'Chat Completions 示例',
    files: filePaths(['~/.config/opencode/opencode.json'], ['%USERPROFILE%\\.config\\opencode\\opencode.json']),
    blocks: [{ label: 'opencode.json · V1 格式', content: json({ $schema: 'https://opencode.ai/config.json', model: 'laoshirenai/YOUR_MODEL_ID', provider: { laoshirenai: { npm: '@ai-sdk/openai-compatible', name: '老实人AI', options: { baseURL: v1, apiKey: '{env:LAOSHIRENAI_API_KEY}' }, models: { YOUR_MODEL_ID: { name: 'YOUR_MODEL_ID' } } } } }) }],
    instructions: ['保留已有配置，把 laoshirenai 合并到 provider 下；默认 model 放在顶层。', '所有 YOUR_MODEL_ID 一起替换。顶层 model 的格式是「供应商 ID/模型 ID」。'],
    notes: ['这是 V1，不能照抄 V2 的 providers / package / settings 字段。', '本例使用 Chat Completions。Responses 的 provider package 不同，不能只改协议名称。', '新增模型要更新 models 条目，本例不会自动导入整份模型列表。'], launch: 'opencode --version\nopencode',
    sources: [{ label: 'OpenCode 1.18.15 官方示例', url: 'https://github.com/anomalyco/opencode/blob/v1.18.15/packages/web/src/content/docs/providers.mdx' }],
  },
  {
    id: 'zcode', referenceVersion: 'app:3.10.1+cli:0.16.5', scope: 'ZCode 桌面版与配套 CLI', mode: 'native', baseUrl: v1, protocol: 'Chat Completions 示例',
    files: filePaths(['~/.zcode/v2/config.json'], ['%USERPROFILE%\\.zcode\\v2\\config.json']), blocks: [],
    instructions: ['打开 ZCode 自带的模型供应商设置，新增自定义供应商。', '本篇先用 Chat Completions 格式，Base URL 填下方带 /v1 的地址；不要只改协议名称而沿用整份配置。', '填入 Key 和精确模型 ID，保存后在新会话选择这个模型。'],
    notes: ['这一版通过工具自己的设置入口操作，不提供未经确认的整份 JSON 覆盖模板。', '.zcode/cli/config.json 是另一份 CLI 配置，不要当成桌面模型配置替换。'],
    sources: [{ label: 'ZCode 官方文档', url: 'https://docs.z.ai/zcode' }],
  },
  {
    id: 'gemini-cli', referenceVersion: 'cli:0.57.0', scope: 'Gemini CLI · 0.57.0 参考配置', mode: 'file', baseUrl: root, protocol: 'Gemini GenerateContent',
    files: filePaths(['~/.gemini/settings.json', '~/.gemini/.env'], ['%USERPROFILE%\\.gemini\\settings.json', '%USERPROFILE%\\.gemini\\.env']), keyEnv: 'GEMINI_API_KEY', environment: { GOOGLE_GEMINI_BASE_URL: root, GOOGLE_GENAI_USE_VERTEXAI: 'false', GOOGLE_GENAI_USE_GCA: 'false' },
    blocks: [
      { label: '.env · 这里只放地址，不放 Key', content: `GOOGLE_GEMINI_BASE_URL=${root}\nGOOGLE_GENAI_USE_VERTEXAI=false\nGOOGLE_GENAI_USE_GCA=false` },
      { label: 'settings.json', content: json({ security: { auth: { selectedType: 'gateway' } }, model: { name: 'YOUR_MODEL_ID' } }) },
    ],
    instructions: ['合并 .env 中这三个非敏感设置，Key 使用当前终端的 GEMINI_API_KEY。', '备份并合并 settings.json，模型 ID 使用这把 Key 可用的 Gemini 模型。'],
    notes: ['0.57.0 源码把自定义网关地址对应的认证类型命名为 gateway，不混用 Google 登录或 Vertex AI。', '不要给根地址追加 /v1beta。其他版本的认证、辅助模型行为可能不同，先核对版本。', '如果报错里出现你没选过的旧模型名，请保留版本和报错联系客服，不要反复修改 Base URL。'], launch: 'gemini --version\ngemini -m YOUR_MODEL_ID',
    sources: [{ label: 'Gemini CLI 0.57.0 认证源码', url: 'https://github.com/google-gemini/gemini-cli/blob/v0.57.0/packages/core/src/core/contentGenerator.ts' }],
  },
  {
    id: 'antigravity', referenceVersion: 'cli:1.1.22', scope: 'Antigravity CLI（agy），不是桌面 IDE', mode: 'file', baseUrl: root, protocol: 'Gemini GenerateContent',
    files: filePaths(['~/.gemini/antigravity-cli/settings.json'], ['%USERPROFILE%\\.gemini\\antigravity-cli\\settings.json']), keyEnv: 'GEMINI_API_KEY', environment: { GOOGLE_GEMINI_BASE_URL: root },
    blocks: [{ label: 'settings.json · CLI 专用目录', content: json({ modelProvider: 'gemini' }) }],
    instructions: ['在 CLI 专用 settings.json 中设置 modelProvider。不要修改 IDE 的内部数据库。', '启动前在同一终端设置 GOOGLE_GEMINI_BASE_URL，然后使用 agy 选择 Key 有权调用的 Gemini 模型。'],
    notes: ['CLI 不自动读取 .env；环境变量必须在启动 agy 的终端生效。', '官方滚动说明和矩阵参考版本有差异，这份配置仍需按实际版本验收；不能作为桌面 IDE 的教程。'], launch: 'agy --version\nagy --model YOUR_MODEL_ID',
    sources: [{ label: 'Antigravity CLI 官方说明', url: 'https://www.antigravity.google/docs/cli/install/' }],
  },
  {
    id: 'hermes-agent', referenceVersion: 'cli:0.20.0', scope: 'Hermes Agent CLI · 0.20.0', mode: 'file', baseUrl: v1, protocol: 'Responses 示例',
    files: filePaths(['~/.hermes/config.yaml'], ['~/.hermes/config.yaml']),
    blocks: [{ label: 'config.yaml · 选择供应商，也选择默认模型', content: `providers:\n  laoshirenai:\n    api: ${v1}\n    key_env: LAOSHIRENAI_API_KEY\n    transport: codex_responses\n    default_model: YOUR_MODEL_ID\n\nmodel:\n  provider: custom:laoshirenai\n  default: YOUR_MODEL_ID` }],
    instructions: ['合并 providers.laoshirenai，不要覆盖其他供应商。', '同时修改顶层 model.provider 和 model.default；只写供应商的 default_model 不会切换主 Agent。'],
    notes: ['HERMES_HOME 可改变配置目录。Windows 要明确实际使用的是原生环境还是 WSL，不把两边文件混用。', '这里只给 Responses 示例，不把官方支持的其他 transport 当成本站全部验证通过。', '本例读取环境变量 Key；从同一个终端启动 Hermes。'], launch: 'hermes --version\nhermes',
    sources: [{ label: 'Hermes 0.20.0 对应源码说明', url: 'https://github.com/NousResearch/hermes-agent/blob/c0106e50e7ecedb3ce34e785d949725dc4e0e457/website/docs/integrations/providers.md' }],
  },
  {
    id: 'qoder', referenceVersion: 'cli:1.1.42', scope: 'Qoder CLI · 使用账号开放的 Custom URL 入口', mode: 'native', navLabel: '需权限', baseUrl: v1, protocol: 'openai · Chat Completions 示例',
    notice: { title: '先登录 Qoder，并确认有 Custom URL 权限', body: '这不是免登录接入。已核对 1.1.42 的配置流程；当前隔离测试停在官方账号登录，尚未完成本站模型调用验收。没有 Custom URL 选项时，请先联系 Qoder 确认账号权限，不要把本站 Key 填给其他官方供应商。' },
    files: filePaths(['~/.qoder/settings.json（不是 BYOK 手改入口）'], ['%USERPROFILE%\\.qoder\\settings.json（不是 BYOK 手改入口）']),
    blocks: [{ label: '终端 · 确认版本并启动', content: 'qoder --version\nqoder' }, { label: '进入 Qoder 后输入 · 不是系统终端命令', content: '/model' }],
    instructions: ['启动 qoder，按提示登录你自己的 Qoder 账号；本站 API Key 不是 Qoder 的登录凭证。', '在 Qoder 内输入 /model → Custom → Add custom model...，在供应商列表选择 Custom URL...。', 'Base URL 填 https://api.laoshirenai.com/v1；Model name 填 YOUR_MODEL_ID，替换成 Key 可用、支持 Chat Completions 和工具调用的模型 ID。', 'Display name 可填「老实人AI」；接口格式选择 openai，再在 API Key 输入框填写专门为 Qoder 创建的 Key。', '等待工具校验成功，再回到 Custom 列表选中刚添加的模型；不要继续使用默认 Auto 模型来验证本站接入。'],
    notes: ['Custom URL 的显示由 Qoder 账号权限控制。找不到入口不代表本站 Base URL 写错，也不要靠修改内部开关绕过限制。', '这里选择 openai，并填带 /v1 的基础地址；不追加 /responses，不把其他工具的协议名称照搬过来。', 'Qoder 会将自定义 URL、模型及 Key 发给它的服务做 BYOK 校验；这不是凭证仅在你电脑与本站之间传递的方案。请使用独立、限额的 Key，停用时撤销。', '不要在 settings.json 中手写 BYOK，也不填写 QODER_PERSONAL_ACCESS_TOKEN 来冒充本站 Key。实际供应商选项以你的账号界面为准。', '这是普通 CLI 教程，不是 Qoder SDK、企业入口或 IDE 的配置说明；Windows ARM64 暂不在官方支持范围。'],
    sources: [{ label: 'Qoder 官方 Custom Models', url: 'https://docs.qoder.com/cli/custom-models' }, { label: 'Qoder 官方版本记录', url: 'https://docs.qoder.com/release-notes/qoder-cli' }, { label: '1.1.42 官方发行包', url: 'https://www.npmjs.com/package/@qoder-ai/qodercli/v/1.1.42' }],
  },
  {
    id: 'minimax-code', referenceVersion: 'cli:0.3.2', scope: 'MiniMax Code CLI（mcode），不是 MiniMax Desktop', mode: 'native', navLabel: '本机存 Key', baseUrl: v1, protocol: 'openai-responses · Responses 示例',
    verification: { macos: '配置与本地模拟接口验证通过 · 非生产调用验收' },
    notice: { title: '这个工具会把 Key 保存到本机', body: '0.3.2 会把第三方 Key 明文写入 config.yaml。即使使用 --api-key-env，也不是只引用环境变量。下方采用工具自己的配置入口；请用独立、限额的 Key，不共享或上传配置文件。本次只用了模拟凭证验证，没有写入真实 Key 或做本站生产调用。' },
    files: filePaths(['~/.minimax/config.yaml'], ['%USERPROFILE%\\.minimax\\config.yaml']),
    blocks: [{ label: '终端 · 确认版本并启动', content: 'mcode --version\nmcode' }, { label: '进入 MiniMax Code 后输入 · 不是系统终端命令', content: '/model' }],
    instructions: ['确认使用 @minimax-ai/code 的 mcode CLI。0.3.2 需要 Node.js 22.19 及以上的 22.x，或 24–26；先在终端启动 mcode。', '在工具内输入 /model，选择 Add 3rd-party provider…，再选择 Custom Provider。不要在 MiniMax 官方登录框填写本站 Key。', '供应商名称填 laoshirenai；接口格式选 openai-responses；Base URL 填 https://api.laoshirenai.com/v1。', '模型 ID 填 YOUR_MODEL_ID，替换成 Key 可用、支持 Responses 和工具调用的模型 ID；API Key 填你为这个工具创建的 Key。', '先测试连接，成功后保存并应用该模型。回到 /model 确认选中的是 laoshirenai 下的模型，再开始新会话。'],
    notes: ['无需手写或覆盖 YAML，工具负责保存供应商和默认模型；设置过 MINIMAX_DATA_DIR / MAVIS_DATA_DIR 时，以实际目录为准。', '带 /v1 的地址配 openai-responses；不要保留默认 anthropic-messages 协议却填写这套配置。', '本次确认了新增供应商、保存、读取和 /v1/responses 模拟请求，不等于真实模型或 Windows/Linux 客户端已完成验收。', '命令行 provider add 的 --use 不能跳过连接测试；简单接入按上面的交互流程操作，不反复追加供应商。', '关掉终端不会删除已经保存的 Key。不再使用时，在工具里移除该供应商，并在本站撤销专用 Key。'],
    sources: [{ label: 'MiniMax Code 0.3.2 官方发行包与说明', url: 'https://www.npmjs.com/package/@minimax-ai/code/v/0.3.2' }],
  },
  {
    id: 'workbuddy', referenceVersion: 'app:5.3.14+cli:2.115.0', scope: 'WorkBuddy 桌面版，不是内置 CodeBuddy CLI', mode: 'native', baseUrl: `${v1}/chat/completions`, protocol: 'Chat Completions · 完整 URL 模式',
    files: { macos: ['~/.workbuddy/models.json'], windows: ['%USERPROFILE%\\.workbuddy\\models.json'] }, blocks: [],
    instructions: ['在 WorkBuddy 自带的「设置 → 模型 → Custom」中新增模型。', '使用完整 URL 模式（开启自定义协议开关），填入下方地址、Key 和精确模型 ID。', '保存后完全退出并重新打开 WorkBuddy，在模型选择器中选中刚添加的模型。'],
    notes: ['开关关闭时用 https://api.laoshirenai.com/v1；开关开启时用完整 /v1/chat/completions。两种地址不要混填。', '桌面配置在 .workbuddy，不是 .codebuddy。本例不要求手动覆盖 models.json。', '当前不提供 Linux 版配置；此开关也不代表能任意切换为 Responses 或 Messages。'],
    sources: [{ label: 'WorkBuddy 官方模型设置', url: 'https://www.codebuddy.ai/docs/zh/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model' }],
  },
  {
    id: 'deepseek-harness', referenceVersion: 'cli:0.1.1-rc.2', scope: 'DeepSeek Harness CLI · Developer Preview', mode: 'file', baseUrl: v1, protocol: 'Chat Completions 示例',
    files: filePaths(['~/.dsh/settings.yaml'], ['%USERPROFILE%\\.dsh\\settings.yaml']),
    blocks: [{ label: 'settings.yaml · rc.2 格式', content: `agent-default-model:\n  provider: laoshirenai-chat\n  model: YOUR_MODEL_ID\n\nllm-pi-ai:\n  providers:\n    laoshirenai-chat:\n      apiKeyEnv: LAOSHIRENAI_API_KEY\n      api: openai-completions\n      baseURL: ${v1}\n      models:\n        - id: YOUR_MODEL_ID` }],
    instructions: ['DSH_HOME 可改变目录；保留已有配置，只合并示例中的段落。', '供应商名称与默认模型中的 provider 一致；两处模型 ID 一起替换。'],
    notes: ['修改默认模型不会强制切换已有会话，请新建会话验证。', '这是开发预览版的字段示例，不代表本站全部模型和系统已通过实际任务验证。'], launch: 'dsh --version\ndsh',
    sources: [{ label: 'DSH rc.2 官方配置', url: 'https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.1.1-rc.2/docs/user/guide/providers.md' }],
  },
  {
    id: 'vscode-local-agent', referenceVersion: 'desktop:1.135.0+builtin-copilot:0.63.0', scope: 'VS Code 第一方 Local Agent / Copilot', mode: 'native', baseUrl: `${v1}/responses`, protocol: 'Responses 示例 · 填完整接口 URL',
    files: { macos: ['~/Library/Application Support/Code/User/chatLanguageModels.json'], linux: ['~/.config/Code/User/chatLanguageModels.json'], windows: ['%APPDATA%\\Code\\User\\chatLanguageModels.json'] }, blocks: [],
    instructions: ['打开命令面板，运行 Chat: Manage Language Models，选择 Add Models → Custom Endpoint。', '选择 Responses，填写完整 /v1/responses 地址和模型 ID。凭证按 VS Code 的提示保存到安全存储。', '使用命令打开实际配置文件；若使用 Profile，路径可能不是默认用户目录。新建 Chat 会话并选择添加的模型。'],
    notes: ['改用 Chat 时完整地址为 /v1/chat/completions；Messages 为 /v1/messages。', '不要把真实 Key 写成普通 JSON 字符串，也不要把这个入口和 Claude Code 扩展、其他插件混为一谈。'],
    sources: [{ label: 'VS Code 官方模型设置', url: 'https://code.visualstudio.com/docs/agent-customization/language-models' }],
  },
]

export const textGuideClients = clientMatrix.map(client => ({
  client,
  guide: textClientGuides.find(guide => guide.id === client.id),
}))

export function keySetupCommand(os: GuideOS, keyEnv?: string, environment: Record<string, string> = {}): string {
  const parts = os === 'windows'
    ? [`$env:LAOSHIRENAI_API_KEY = [System.Net.NetworkCredential]::new('', (Read-Host '粘贴 API Key（输入不显示）' -AsSecureString)).Password`]
    : [`printf '粘贴 API Key（输入不显示）：'`, 'read -rs LAOSHIRENAI_API_KEY', 'export LAOSHIRENAI_API_KEY', `printf '\\n'`]
  if (keyEnv) parts.push(os === 'windows' ? `$env:${keyEnv} = $env:LAOSHIRENAI_API_KEY` : `export ${keyEnv}="$LAOSHIRENAI_API_KEY"`)
  for (const [key, value] of Object.entries(environment)) {
    parts.push(os === 'windows' ? `$env:${key} = '${value}'` : `export ${key}='${value}'`)
  }
  // One physical line prevents a pasted next statement being consumed by read.
  return parts.join('; ')
}

export function modelsCommand(os: GuideOS): string {
  if (os === 'windows') return `(Invoke-RestMethod -Uri '${root}/v1/models' -Headers @{Authorization="Bearer $env:LAOSHIRENAI_API_KEY"}).data.id`
  return `curl --fail --silent --show-error --config - <<EOF\nurl = "${root}/v1/models"\nheader = "Authorization: Bearer $LAOSHIRENAI_API_KEY"\nEOF`
}

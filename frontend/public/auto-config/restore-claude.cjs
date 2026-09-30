"use strict";

// scripts/restore/claude.cjs
var fs = require("node:fs");
var path = require("node:path");
var os = require("node:os");
var modelKeys = [
  "ANTHROPIC_MODEL",
  "ANTHROPIC_DEFAULT_OPUS_MODEL",
  "ANTHROPIC_DEFAULT_SONNET_MODEL",
  "ANTHROPIC_DEFAULT_HAIKU_MODEL",
  "ANTHROPIC_DEFAULT_FABLE_MODEL"
];
var credentialOverrides = [
  "ANTHROPIC_API_KEY",
  "ANTHROPIC_AUTH_TOKEN",
  "ANTHROPIC_BASE_URL",
  "CLAUDE_CODE_USE_BEDROCK",
  "CLAUDE_CODE_USE_VERTEX",
  "CLAUDE_CODE_USE_FOUNDRY",
  "ANTHROPIC_PROFILE",
  "ANTHROPIC_FEDERATION_ACCOUNT",
  "ANTHROPIC_FEDERATION_PROVIDER"
];
var object = (value) => value !== null && typeof value === "object" && !Array.isArray(value);
function restore(home = process.env.CLAUDE_CONFIG_DIR || path.join(os.homedir(), ".claude"), env = process.env) {
  const file = path.join(home, "settings.json");
  if (credentialOverrides.some((key) => env[key])) {
    throw new Error("\u68C0\u6D4B\u5230\u7EC8\u7AEF\u9274\u6743\u6216\u7AEF\u70B9\u73AF\u5883\u53D8\u91CF\uFF0C\u8BF7\u5148\u79FB\u9664\u8986\u76D6\u8BBE\u7F6E\u540E\u91CD\u8BD5\uFF1B\u672A\u4FEE\u6539\u4EFB\u4F55\u6587\u4EF6");
  }
  if (!fs.existsSync(file)) return null;
  if (!fs.lstatSync(file).isFile()) throw new Error("\u914D\u7F6E\u6587\u4EF6\u4E0D\u80FD\u662F\u7B26\u53F7\u94FE\u63A5\u6216\u76EE\u5F55\uFF0C\u8BF7\u624B\u52A8\u68C0\u67E5");
  const before = fs.readFileSync(file, "utf8");
  let config;
  try {
    config = JSON.parse(before.replace(/^\uFEFF/, ""));
  } catch {
    throw new Error("Claude Code \u914D\u7F6E\u89E3\u6790\u5931\u8D25\uFF0C\u672A\u4FEE\u6539\u4EFB\u4F55\u6587\u4EF6\uFF0C\u8BF7\u68C0\u67E5 JSON \u683C\u5F0F");
  }
  if (!object(config) || config.env !== void 0 && !object(config.env) || config.modelSettings !== void 0 && !object(config.modelSettings)) {
    throw new Error("Claude Code \u914D\u7F6E\u3001env \u6216 modelSettings \u4E0D\u662F JSON \u5BF9\u8C61\uFF0C\u672A\u4FEE\u6539\u4EFB\u4F55\u6587\u4EF6");
  }
  const settings = config.env || {};
  const base = settings.ANTHROPIC_BASE_URL;
  if (!base) return null;
  if (![
    "https://api.laoshirenai.com",
    "https://api.laoshirenai.com/",
    "https://api.laoshirenai.com/v1",
    "https://api.laoshirenai.com/v1/",
    "https://api.laoshirenai.com/antigravity",
    "https://api.laoshirenai.com/antigravity/"
  ].includes(base)) {
    throw new Error("\u5F53\u524D\u672A\u4F7F\u7528\u8001\u5B9E\u4EBAAI Claude Code \u7AEF\u70B9\uFF0C\u672A\u4FEE\u6539\u4EFB\u4F55\u6587\u4EF6\uFF1B\u8BF7\u68C0\u67E5 CLAUDE_CONFIG_DIR \u6216\u624B\u52A8\u914D\u7F6E");
  }
  if (config.apiKeyHelper || config.forceLoginMethod || config.forceLoginOrgUUID || config.forceLoginGatewayUrl || credentialOverrides.filter((key) => !["ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN"].includes(key)).some((key) => settings[key])) {
    throw new Error("\u68C0\u6D4B\u5230\u5176\u4ED6\u9274\u6743\u3001\u4E91\u670D\u52A1\u6216\u7EC4\u7EC7\u767B\u5F55\u8BBE\u7F6E\uFF0C\u8BF7\u5148\u624B\u52A8\u68C0\u67E5\uFF1B\u672A\u4FEE\u6539\u4EFB\u4F55\u6587\u4EF6");
  }
  const models = new Set(modelKeys.map((key) => settings[key]).filter((value) => typeof value === "string" && value));
  if (models.has(config.model)) delete config.model;
  for (const model of models) {
    const entry = config.modelSettings?.[model];
    if (entry !== void 0 && !object(entry)) throw new Error("\u6A21\u578B\u8BBE\u7F6E\u4E0D\u662F JSON \u5BF9\u8C61\uFF0C\u672A\u4FEE\u6539\u4EFB\u4F55\u6587\u4EF6");
    if (entry) {
      delete entry.effortLevel;
      if (!Object.keys(entry).length) delete config.modelSettings[model];
    }
  }
  if (config.modelSettings && !Object.keys(config.modelSettings).length) delete config.modelSettings;
  for (const key of [
    "ANTHROPIC_BASE_URL",
    "ANTHROPIC_AUTH_TOKEN",
    ...modelKeys,
    "CLAUDE_CODE_ATTRIBUTION_HEADER",
    "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY",
    "CLAUDE_CODE_EFFORT_LEVEL"
  ]) delete settings[key];
  if (!Object.keys(settings).length) delete config.env;
  const after = JSON.stringify(config, null, 2) + "\n";
  const backup = fs.mkdtempSync(path.join(home, "before-official-"));
  fs.chmodSync(backup, 448);
  fs.writeFileSync(path.join(backup, "settings.json"), before, { flag: "wx", mode: 384 });
  const temp = `${file}.restore-${process.pid}`;
  try {
    fs.writeFileSync(temp, after, { flag: "wx", mode: 384 });
    if (fs.readFileSync(file, "utf8") !== before) throw new Error("concurrent change");
    fs.renameSync(temp, file);
    if (fs.readFileSync(file, "utf8") !== after) throw new Error("readback mismatch");
  } catch {
    throw new Error(`\u8FD8\u539F\u672A\u5B8C\u6210\uFF0C\u539F\u6587\u4EF6\u5907\u4EFD\u4FDD\u7559\u5728 ${backup}`);
  } finally {
    if (fs.existsSync(temp)) fs.unlinkSync(temp);
  }
  return backup;
}
if (require.main === module) {
  try {
    const backup = restore();
    console.log(backup ? `\u5DF2\u79FB\u9664 Claude Code \u7528\u6237\u8BBE\u7F6E\u4E2D\u7684\u672C\u7AD9\u63A5\u5165\u914D\u7F6E\uFF0C\u6587\u4EF6\u56DE\u8BFB\u901A\u8FC7\u3002\u5907\u4EFD\uFF1A${backup}` : "\u6CA1\u6709\u9700\u8981\u8FD8\u539F\u7684\u8001\u5B9E\u4EBAAI\u914D\u7F6E\uFF0C\u672A\u4FEE\u6539\u4EFB\u4F55\u6587\u4EF6\u3002");
    console.log("\u8BF7\u91CD\u542F Claude Code\uFF0C\u5FC5\u8981\u65F6\u8FD0\u884C /login \u767B\u5F55\u81EA\u5DF1\u7684\u5B98\u65B9\u8D26\u53F7\uFF0C\u518D\u7528 /status \u68C0\u67E5\u3002\u5B98\u65B9\u767B\u5F55\u53CA\u8BF7\u6C42\u5C1A\u672A\u9A8C\u8BC1\u3002");
    console.log("\u4F1A\u8BDD\u3001MCP\u3001\u6743\u9650\u3001Hooks \u548C\u5B98\u65B9\u767B\u5F55\u6587\u4EF6\u672A\u4FEE\u6539\u3002\u9879\u76EE/\u7EC4\u7EC7\u8BBE\u7F6E\u3001\u542F\u52A8\u53C2\u6570\u4E0E CC Switch \u4ECD\u53EF\u80FD\u8986\u76D6\u914D\u7F6E\u3002\u8BF7\u52FF\u5206\u4EAB\u5907\u4EFD\u6587\u4EF6\u3002");
  } catch (error) {
    console.error(error.name === "Error" ? error.message : "\u914D\u7F6E\u8FD8\u539F\u5931\u8D25\uFF0C\u8BF7\u68C0\u67E5\u6587\u4EF6\u6743\u9650\uFF1B\u672A\u9A8C\u8BC1\u5B98\u65B9\u767B\u5F55");
    process.exitCode = 1;
  }
}
module.exports = { restore };

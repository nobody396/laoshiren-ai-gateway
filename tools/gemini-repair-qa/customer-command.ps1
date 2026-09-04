@'
const fs = require('node:fs'), path = require('node:path'), os = require('node:os');
const dir = path.join(process.env.GEMINI_CLI_HOME || os.homedir(), '.gemini');
const file = path.join(dir, 'settings.json');
const raw = fs.existsSync(file) ? fs.readFileSync(file, 'utf8') : '{}';
const config = JSON.parse(raw.replace(/^\uFEFF/, ''));
function obj(parent, key) { if (parent[key] === undefined) parent[key] = {}; if (!parent[key] || typeof parent[key] !== 'object' || Array.isArray(parent[key])) throw Error('Invalid object: ' + key); return parent[key]; }
if (!config || typeof config !== 'object' || Array.isArray(config)) throw Error('Invalid settings.json');
obj(config, 'experimental').dynamicModelConfiguration = true;
obj(config, 'model').name = 'gemini-3.8-flash';
const models = obj(config, 'modelConfigs'), definitions = obj(models, 'modelDefinitions'), resolutions = obj(models, 'modelIdResolutions');
for (const model of ['gemini-3.8-flash', 'gemini-3.7-flash', 'gemini-3.1-pro']) {
  definitions[model] = { ...definitions[model], tier: model.endsWith('pro') ? 'pro' : 'flash', family: 'gemini-3', isPreview: false, isVisible: true };
  resolutions[model] = { default: model };
}
for (const model of ['flash-lite', 'gemini-3.1-flash-lite', 'gemini-3.5-flash', 'flash', 'gemini-2.5-flash-lite', 'gemini-2.5-flash', 'gemini-2.5-pro', 'gemini-3-flash-preview', 'gemini-3-pro-preview', 'gemini-3.1-pro-preview']) resolutions[model] = { default: 'gemini-3.8-flash' };
const next = JSON.stringify(config, null, 2) + '\n';
if (next !== raw) {
  fs.mkdirSync(dir, { recursive: true });
  if (fs.existsSync(file)) fs.copyFileSync(file, file + '.bak-' + Date.now(), fs.constants.COPYFILE_EXCL);
  const tmp = file + '.tmp-' + process.pid;
  try { fs.writeFileSync(tmp, next, {flag:'wx',mode:0o600}); fs.renameSync(tmp, file); } finally { if (fs.existsSync(tmp)) fs.unlinkSync(tmp); }
}
console.log('Gemini model repair complete. Restart Gemini CLI. Auxiliary requests use gemini-3.8-flash.');
'@ | node
if ($LASTEXITCODE -ne 0) { throw 'Gemini repair failed; do not start Gemini.' }

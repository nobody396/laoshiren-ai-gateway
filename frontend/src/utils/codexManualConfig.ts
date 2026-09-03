import { buildCodexModelCatalog, resolveCodexModels } from '@/utils/ccSwitchImport'
import type { ClaudeManualPlatform } from '@/utils/claudeCodeManualConfig'

export type CodexManualPlatform = ClaudeManualPlatform
export type CodexReasoningChoice = 'auto' | 'none' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max'

export interface CodexManualConfigInput {
  baseUrl: string
  apiKey: string
  mainModel: string
  reviewModel?: string
  reasoningEffort?: string
  availableModels?: readonly string[]
}

const shellQuote = (value: string): string => `'${value.replace(/'/g, `'"'"'`)}'`
const powerShellQuote = (value: string): string => `'${value.replace(/'/g, "''")}'`

const encodeBase64Utf8 = (value: string): string => {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  bytes.forEach((byte) => { binary += String.fromCharCode(byte) })
  return btoa(binary)
}

export const normalizeCodexBaseUrl = (value: string): string => {
  const normalized = value.trim().replace(/\/+$/, '')
  return normalized.endsWith('/v1') ? normalized : `${normalized}/v1`
}

export const buildCodexOwnedConfig = (input: CodexManualConfigInput) => {
  const mainModel = input.mainModel.trim()
  const reviewModel = input.reviewModel?.trim() || mainModel
  const available = [...new Set([
    ...(input.availableModels ?? []).map(model => model.trim()).filter(Boolean),
    mainModel,
    reviewModel,
  ].filter(Boolean))]
  const models = resolveCodexModels(available)
  const catalog = JSON.parse(buildCodexModelCatalog(models)) as {
    models: Array<Record<string, unknown> & {
      slug: string
      supported_reasoning_levels?: Array<{ effort?: string; description?: string }>
    }>
  }
  const selected = catalog.models.find(model => model.slug === mainModel)
  if (!selected) throw new Error('主模型不在当前 Codex 模型目录中')
  const supported = new Set((selected.supported_reasoning_levels ?? []).map(row => row.effort).filter(Boolean))
  const requested = input.reasoningEffort?.trim() || 'auto'
  const effort = requested !== 'auto' && supported.has(requested) ? requested : ''

  return {
    baseUrl: normalizeCodexBaseUrl(input.baseUrl),
    apiKey: input.apiKey,
    mainModel,
    reviewModel,
    effort,
    catalog,
    contextWindow: Number(selected.context_window),
    autoCompactTokenLimit: Number(selected.auto_compact_token_limit),
  }
}

const unixWriter = String.raw`
import base64,json,os,pathlib,re,shutil

payload=json.loads(base64.b64decode(os.environ["CODEX_CFG_B64"]).decode())
root=pathlib.Path.home()/".codex"
root.mkdir(parents=True,exist_ok=True)
config_path=root/"config.toml"
auth_path=root/"auth.json"
catalog_path=root/"laoshirenai-model-catalog.json"

def backup(path):
    if path.exists(): shutil.copy2(path,str(path)+".bak")

def atomic_text(path,text):
    tmp=path.with_name(path.name+".tmp")
    tmp.write_text(text,encoding="utf-8")
    os.chmod(tmp,0o600)
    tmp.replace(path)

auth=json.loads(auth_path.read_text(encoding="utf-8")) if auth_path.exists() else {}
if not isinstance(auth,dict): raise ValueError("auth.json root must be an object")

text=config_path.read_text(encoding="utf-8") if config_path.exists() else ""
if "\x00" in text: raise ValueError("config.toml contains NUL")
owned={"model_provider","model","review_model","model_reasoning_effort","model_catalog_json","disable_response_storage","network_access","preferred_auth_method","model_context_window","model_auto_compact_token_limit"}
managed="model_providers.laoshirenai_responses"
kept=[]
section=""
dropping=False
for line in text.splitlines():
    stripped=line.strip()
    if stripped.startswith("["):
        match=re.fullmatch(r"\[([^\]]+)\](?:\s*#.*)?",stripped)
        if not match: raise ValueError("refusing malformed TOML header: "+stripped)
        section=match.group(1)
        dropping=section==managed
    if dropping: continue
    key=re.match(r"^([A-Za-z0-9_.-]+)\s*=",stripped).group(1) if section=="" and re.match(r"^([A-Za-z0-9_.-]+)\s*=",stripped) else ""
    if key in owned: continue
    if stripped in {"# BEGIN LAOSHIRENAI CODEX PROVIDER","# END LAOSHIRENAI CODEX PROVIDER"}: continue
    kept.append(line)
while kept and not kept[-1].strip(): kept.pop()
while kept and not kept[0].strip(): kept.pop(0)

quote=lambda value: json.dumps(value,ensure_ascii=False)
head=[
    "model_provider = "+quote("laoshirenai_responses"),
    "model = "+quote(payload["mainModel"]),
    "review_model = "+quote(payload["reviewModel"]),
]
if payload.get("effort"): head.append("model_reasoning_effort = "+quote(payload["effort"]))
head += [
    'model_catalog_json = "laoshirenai-model-catalog.json"',
    "disable_response_storage = true",
    'network_access = "enabled"',
    'preferred_auth_method = "apikey"',
    "model_context_window = "+str(int(payload["contextWindow"])),
    "model_auto_compact_token_limit = "+str(int(payload["autoCompactTokenLimit"])),
    "",
]
provider=[
    "# BEGIN LAOSHIRENAI CODEX PROVIDER",
    "[model_providers.laoshirenai_responses]",
    'name = "老实人AI Responses"',
    "base_url = "+quote(payload["baseUrl"]),
    'wire_api = "responses"',
    "requires_openai_auth = true",
    "# END LAOSHIRENAI CODEX PROVIDER",
    "",
]
output="\n".join(head+kept+([""] if kept else [])+provider)

auth["OPENAI_API_KEY"]=payload["apiKey"]
backup(auth_path)
atomic_text(auth_path,json.dumps(auth,ensure_ascii=False,indent=2)+"\n")
backup(catalog_path)
atomic_text(catalog_path,json.dumps(payload["catalog"],ensure_ascii=False,indent=2)+"\n")
backup(config_path)
atomic_text(config_path,output)
print("Codex config updated:",config_path)
`.trim()

const buildWindowsWriter = (payload: string): string => [
  "$ErrorActionPreference='Stop'",
  `$o=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String(${powerShellQuote(payload)}))|ConvertFrom-Json`,
  "$d=Join-Path $HOME '.codex'",
  'New-Item -ItemType Directory -Force -Path $d|Out-Null',
  "$cp=Join-Path $d 'config.toml'",
  "$ap=Join-Path $d 'auth.json'",
  "$mp=Join-Path $d 'laoshirenai-model-catalog.json'",
  "if(Test-Path -LiteralPath $ap){$a=Get-Content -LiteralPath $ap -Raw|ConvertFrom-Json;if($null -eq $a){throw 'auth.json must contain an object'}}else{$a=[pscustomobject]@{}}",
  "$owned=@('model_provider','model','review_model','model_reasoning_effort','model_catalog_json','disable_response_storage','network_access','preferred_auth_method','model_context_window','model_auto_compact_token_limit')",
  "$managed='model_providers.laoshirenai_responses'",
  '$lines=if(Test-Path -LiteralPath $cp){@(Get-Content -LiteralPath $cp)}else{@()}',
  '$kept=[Collections.Generic.List[string]]::new();$section="";$drop=$false',
  "foreach($line in $lines){$t=$line.Trim();if($t.StartsWith('[')){if($t -notmatch '^\\[([^\\]]+)\\](?:\\s*#.*)?$'){throw ('refusing malformed TOML header: '+$t)};$section=$Matches[1];$drop=$section -eq $managed};if($drop){continue};if($section -eq '' -and $t -match '^([A-Za-z0-9_.-]+)\\s*=' -and $Matches[1] -in $owned){continue};if($t -in @('# BEGIN LAOSHIRENAI CODEX PROVIDER','# END LAOSHIRENAI CODEX PROVIDER')){continue};$kept.Add($line)}",
  "while($kept.Count -gt 0 -and [string]::IsNullOrWhiteSpace($kept[$kept.Count-1])){$kept.RemoveAt($kept.Count-1)}",
  "while($kept.Count -gt 0 -and [string]::IsNullOrWhiteSpace($kept[0])){$kept.RemoveAt(0)}",
  'function Q([string]$v){ConvertTo-Json $v -Compress}',
  '$out=[Collections.Generic.List[string]]::new()',
  "$out.Add('model_provider = '+(Q 'laoshirenai_responses'));$out.Add('model = '+(Q ([string]$o.mainModel)));$out.Add('review_model = '+(Q ([string]$o.reviewModel)))",
  "if($o.effort){$out.Add('model_reasoning_effort = '+(Q ([string]$o.effort)))}",
  "$out.Add('model_catalog_json = \"laoshirenai-model-catalog.json\"');$out.Add('disable_response_storage = true');$out.Add('network_access = \"enabled\"');$out.Add('preferred_auth_method = \"apikey\"');$out.Add('model_context_window = '+[int64]$o.contextWindow);$out.Add('model_auto_compact_token_limit = '+[int64]$o.autoCompactTokenLimit);$out.Add('')",
  'foreach($line in $kept){$out.Add($line)};if($kept.Count){$out.Add(\'\')}',
  "$out.Add('# BEGIN LAOSHIRENAI CODEX PROVIDER');$out.Add('[model_providers.laoshirenai_responses]');$out.Add('name = \"老实人AI Responses\"');$out.Add('base_url = '+(Q ([string]$o.baseUrl)));$out.Add('wire_api = \"responses\"');$out.Add('requires_openai_auth = true');$out.Add('# END LAOSHIRENAI CODEX PROVIDER')",
  "foreach($p in @($cp,$ap,$mp)){if(Test-Path -LiteralPath $p){Copy-Item -LiteralPath $p -Destination ($p+'.bak') -Force}}",
  "$a|Add-Member -NotePropertyName OPENAI_API_KEY -NotePropertyValue ([string]$o.apiKey) -Force",
  "$at=$ap+'.tmp';$a|ConvertTo-Json -Depth 100|Set-Content -LiteralPath $at -Encoding UTF8;Move-Item -LiteralPath $at -Destination $ap -Force",
  "$mt=$mp+'.tmp';$o.catalog|ConvertTo-Json -Depth 100|Set-Content -LiteralPath $mt -Encoding UTF8;Move-Item -LiteralPath $mt -Destination $mp -Force",
  "$ct=$cp+'.tmp';[IO.File]::WriteAllLines($ct,$out,[Text.UTF8Encoding]::new($false));Move-Item -LiteralPath $ct -Destination $cp -Force",
  "Write-Host 'Codex config updated:' $cp",
].join('; ')

export const buildCodexSettingsCommand = (
  input: CodexManualConfigInput,
  platform: CodexManualPlatform,
): string => {
  const payload = encodeBase64Utf8(JSON.stringify(buildCodexOwnedConfig(input)))
  if (platform === 'windows') return buildWindowsWriter(payload)
  const writer = encodeBase64Utf8(unixWriter)
  return `CODEX_CFG_B64=${shellQuote(payload)} python3 -c ${shellQuote(`import base64;exec(base64.b64decode(${JSON.stringify(writer)}).decode())`)}`
}

export const buildCodexVerificationCommand = (platform: CodexManualPlatform): string => {
  const run = 'codex exec --skip-git-repo-check --ephemeral --json "只回复 CODEX_OK"'
  return platform === 'windows'
    ? `codex --version; if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}; ${run}`
    : `codex --version && ${run}`
}

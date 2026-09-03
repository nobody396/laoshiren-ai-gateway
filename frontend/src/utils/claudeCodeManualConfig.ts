export type ClaudeManualPlatform = 'windows' | 'macos' | 'linux'
export type ClaudeEffortChoice = 'auto' | 'low' | 'medium' | 'high' | 'xhigh' | 'max' | 'ultracode'

export interface ClaudeManualConfigInput {
  baseUrl: string
  apiKey: string
  mainModel: string
  effortLevel?: ClaudeEffortChoice
  opusModel?: string
  sonnetModel?: string
  haikuModel?: string
  fableModel?: string
}

const shellQuote = (value: string): string => `'${value.replace(/'/g, `'"'"'`)}'`
const powerShellQuote = (value: string): string => `'${value.replace(/'/g, "''")}'`

export const normalizeClaudeBaseUrl = (value: string): string => {
  const normalized = value.trim().replace(/\/+$/, '')
  return normalized.endsWith('/v1') ? normalized.slice(0, -3) : normalized
}

const modelEndpoint = (baseUrl: string): string => `${normalizeClaudeBaseUrl(baseUrl)}/v1/models`

export const buildClaudeModelListCommand = (
  baseUrl: string,
  apiKey: string,
  platform: ClaudeManualPlatform,
): string => {
  const endpoint = modelEndpoint(baseUrl)
  if (platform === 'windows') {
    return [
      "$ErrorActionPreference='Stop'",
      `try{$r=Invoke-RestMethod -Uri ${powerShellQuote(endpoint)} -Headers @{Authorization=${powerShellQuote(`Bearer ${apiKey}`)}};$r.data.id}`,
      "catch{$m=$_.ErrorDetails.Message;try{$p=$m|ConvertFrom-Json;if($p.code -eq 'API_KEY_QUOTA_EXHAUSTED'){$m='这把 API Key 设置的额度已用完；请在 API 密钥页面重置用量、提高或关闭额度上限'}else{$m=if($p.message){$p.message}elseif($p.error.message){$p.error.message}else{$m}}}catch{};if(-not $m){$m=$_.Exception.Message};Write-Error ('读取模型失败：'+$m);exit 1}",
    ].join('; ')
  }
  const python = [
    'import json,sys',
    'raw=sys.stdin.read()',
    'body,status=raw.rsplit("\\n",1) if "\\n" in raw else (raw,"000")',
    'payload=json.loads(body) if body.strip().startswith(("{","[")) else {}',
    'error=payload.get("error",{}) if isinstance(payload,dict) else {}',
    'message=(payload.get("message") or error.get("message") or "网关未返回错误详情") if isinstance(payload,dict) else "响应格式错误"',
    'code=(payload.get("code") or error.get("code") or "") if isinstance(payload,dict) else ""',
    'message="这把 API Key 设置的额度已用完；请在 API 密钥页面重置用量、提高或关闭额度上限" if code=="API_KEY_QUOTA_EXHAUSTED" else message',
    'status=="200" or sys.exit("读取模型失败：HTTP %s：%s%s"%(status,message,("（%s）"%code) if code else ""))',
    'print("\\n".join(str(x.get("id","")) for x in payload.get("data",[]) if x.get("id")))',
  ].join('; ')
  return `set -o pipefail; curl -sS ${shellQuote(endpoint)} -H ${shellQuote(`Authorization: Bearer ${apiKey}`)} -w ${shellQuote('\\n%{http_code}')} | python3 -c ${shellQuote(python)}`
}

export const buildClaudeOwnedConfig = (input: ClaudeManualConfigInput) => {
  const mainModel = input.mainModel.trim()
  const fallback = (value?: string) => value?.trim() || mainModel
  const config: {
    model: string
    modelSettingsUpdate: {
      model: string
      effortLevel: 'low' | 'medium' | 'high' | 'xhigh' | null
    }
    env: Record<string, string>
  } = {
    model: mainModel,
    modelSettingsUpdate: {
      model: mainModel,
      effortLevel: input.effortLevel && ['low', 'medium', 'high', 'xhigh'].includes(input.effortLevel)
        ? input.effortLevel as 'low' | 'medium' | 'high' | 'xhigh'
        : null,
    },
    env: {
      ANTHROPIC_BASE_URL: normalizeClaudeBaseUrl(input.baseUrl),
      ANTHROPIC_AUTH_TOKEN: input.apiKey,
      ANTHROPIC_MODEL: mainModel,
      ANTHROPIC_DEFAULT_OPUS_MODEL: fallback(input.opusModel),
      ANTHROPIC_DEFAULT_SONNET_MODEL: fallback(input.sonnetModel),
      ANTHROPIC_DEFAULT_HAIKU_MODEL: fallback(input.haikuModel),
      ANTHROPIC_DEFAULT_FABLE_MODEL: fallback(input.fableModel),
      CLAUDE_CODE_ATTRIBUTION_HEADER: '0',
      CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1',
    },
  }
  return config
}

const encodeBase64Utf8 = (value: string): string => {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  bytes.forEach((byte) => { binary += String.fromCharCode(byte) })
  return btoa(binary)
}

const unixMergeScript = [
  'import base64,json,os,pathlib,shutil',
  'p=pathlib.Path.home()/".claude"/"settings.json"',
  'o=json.loads(base64.b64decode(os.environ["CLAUDE_CFG_B64"]).decode())',
  'c=json.loads(p.read_text()) if p.exists() else {}',
  'assert isinstance(c,dict),"settings.json root must be an object"',
  'e=c.get("env",{})',
  'assert isinstance(e,dict),"settings.json env must be an object"',
  'c["model"]=o["model"]',
  'ms=c.get("modelSettings",{})',
  'assert isinstance(ms,dict),"settings.json modelSettings must be an object"',
  'mu=o["modelSettingsUpdate"]',
  'mn=mu["model"]',
  'me=ms.get(mn,{})',
  'assert isinstance(me,dict),"settings.json modelSettings entry must be an object"',
  'me.pop("effortLevel",None) if mu["effortLevel"] is None else me.__setitem__("effortLevel",mu["effortLevel"])',
  'ms.pop(mn,None) if not me else ms.__setitem__(mn,me)',
  'c["modelSettings"]=ms',
  'e.update(o["env"])',
  'c["env"]=e',
  'p.parent.mkdir(parents=True,exist_ok=True)',
  'p.exists() and shutil.copy2(p,str(p)+".bak")',
  't=p.with_name(p.name+".tmp")',
  't.write_text(json.dumps(c,ensure_ascii=False,indent=2)+"\\n")',
  't.replace(p)',
  'print("Claude Code config updated:",p)',
].join(';')

export const buildClaudeSettingsCommand = (
  input: ClaudeManualConfigInput,
  platform: ClaudeManualPlatform,
): string => {
  const payload = encodeBase64Utf8(JSON.stringify(buildClaudeOwnedConfig(input)))
  if (platform !== 'windows') {
    return `CLAUDE_CFG_B64=${shellQuote(payload)} python3 -c ${shellQuote(unixMergeScript)}`
  }

  return [
    "$ErrorActionPreference='Stop'",
    `$o=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String(${powerShellQuote(payload)})) | ConvertFrom-Json`,
    "$p=Join-Path $HOME '.claude\\settings.json'",
    'New-Item -ItemType Directory -Force -Path (Split-Path $p) | Out-Null',
    "if(Test-Path $p){$c=Get-Content -LiteralPath $p -Raw | ConvertFrom-Json; if($null -eq $c){throw 'settings.json must contain an object'}; Copy-Item -LiteralPath $p -Destination ($p+'.bak') -Force}else{$c=[pscustomobject]@{}}",
    'if($null -eq $c.env){$c | Add-Member -NotePropertyName env -NotePropertyValue ([pscustomobject]@{}) -Force}',
    '$c | Add-Member -NotePropertyName model -NotePropertyValue $o.model -Force',
    'if($null -eq $c.modelSettings){$c | Add-Member -NotePropertyName modelSettings -NotePropertyValue ([pscustomobject]@{}) -Force}',
    '$mn=[string]$o.modelSettingsUpdate.model',
    '$me=$c.modelSettings.PSObject.Properties[$mn].Value',
    'if($null -eq $me){$me=[pscustomobject]@{}}',
    'if($null -eq $o.modelSettingsUpdate.effortLevel){$me.PSObject.Properties.Remove(\'effortLevel\')}else{$me | Add-Member -NotePropertyName effortLevel -NotePropertyValue $o.modelSettingsUpdate.effortLevel -Force}',
    'if($me.PSObject.Properties.Count -eq 0){$c.modelSettings.PSObject.Properties.Remove($mn)}else{$c.modelSettings | Add-Member -NotePropertyName $mn -NotePropertyValue $me -Force}',
    'foreach($q in $o.env.PSObject.Properties){$c.env | Add-Member -NotePropertyName $q.Name -NotePropertyValue $q.Value -Force}',
    '$tmp=$p+\'.tmp\'',
    '$c | ConvertTo-Json -Depth 100 | Set-Content -LiteralPath $tmp -Encoding UTF8',
    'Move-Item -LiteralPath $tmp -Destination $p -Force',
    "Write-Host 'Claude Code config updated:' $p",
  ].join('; ')
}

export const buildClaudeVerificationCommand = (effort: ClaudeEffortChoice = 'auto'): string => {
  const effortFlag = effort === 'max' || effort === 'ultracode' ? ` --effort ${effort}` : ''
  return `claude${effortFlag} -p "只回复 CLAUDE_CODE_OK"`
}

export const claudeVerificationCommand = buildClaudeVerificationCommand()

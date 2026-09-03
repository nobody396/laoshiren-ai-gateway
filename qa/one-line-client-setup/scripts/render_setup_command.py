#!/usr/bin/env python3
"""Render only customer commands backed by an explicit tested adapter."""

from __future__ import annotations

import argparse
import base64
import json
import sys


def ps_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def render_workbuddy_windows(group_id: int, group_name: str) -> str:
    group = ps_quote(group_name)
    parts = [
        f"$groupId={group_id}",
        f"$groupName={group}",
        "$k=Read-Host \"请粘贴 $groupName API Key\"",
        "if([string]::IsNullOrWhiteSpace($k)){throw 'API Key 不能为空'}",
        "try{$catalog=Invoke-RestMethod -Uri 'https://api.laoshirenai.com/api/v1/public/model-pricing' -Method Get}catch{throw \"读取分组目录失败：$($_.Exception.Message)\"}",
        "$group=@($catalog.data.groups|Where-Object{[int64]$_.group_id -eq $groupId})[0]",
        "if($null -eq $group){throw \"找不到分组：$groupName（ID $groupId）\"}",
        "$groupIds=@($group.models|ForEach-Object{[string]$_.model}|Sort-Object -Unique)",
        "if($groupIds.Count -eq 0){throw \"分组没有公开可用模型：$groupName\"}",
        "try{$keyCatalog=Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{Authorization=\"Bearer $k\"} -Method Get}catch{throw \"读取 Key 可用模型失败：$($_.Exception.Message)\"}",
        "$keyIds=@($keyCatalog.data|ForEach-Object{[string]$_.id}|Sort-Object -Unique)",
        "$ids=@($groupIds|Where-Object{$keyIds -contains $_}|Sort-Object -Unique)",
        "if($ids.Count -ne $groupIds.Count){$missing=@($groupIds|Where-Object{$keyIds -notcontains $_});throw \"该 Key 无权调用分组全部模型，缺少：$($missing -join '、')\"}",
        "$d=Join-Path $HOME '.workbuddy'",
        "$p=Join-Path $d 'models.json'",
        "New-Item -ItemType Directory -Force -Path $d|Out-Null",
        "if(Test-Path $p){$raw=[IO.File]::ReadAllText($p);if([string]::IsNullOrWhiteSpace($raw)){$root=@();$arrayRoot=$true}else{try{$parsed=$raw|ConvertFrom-Json}catch{throw \"现有 models.json 格式错误，未修改：$p\"};$arrayRoot=$raw.TrimStart().StartsWith('[');if($arrayRoot){$root=@($parsed)}elseif($parsed -is [pscustomobject]){$root=$parsed}else{throw \"现有 models.json 顶层必须是数组或对象，未修改：$p\"}}}else{$root=@();$arrayRoot=$true}",
        "if($arrayRoot){$models=@($root);$envelope=$null}else{$envelope=$root;if($envelope.PSObject.Properties['models']){$models=@($envelope.models)}else{$models=@()}}",
        "$prefix=\"老实人AI $groupName\"",
        "$old=@($models)",
        "$oldIds=@($old|Where-Object{$_.name -like \"$prefix*\" -or ($_.url -eq 'https://api.laoshirenai.com/v1/chat/completions' -and $_.apiKey -eq $k)}|ForEach-Object{[string]$_.id})",
        "$new=@($ids|ForEach-Object{$modelId=[string]$_;$efforts=if($modelId -in @('gpt-5.6-sol','gpt-5.6-terra')){@('low','medium','high','xhigh','max')}else{@('low','medium','high','xhigh')};[pscustomobject]@{id=$modelId;name=$modelId;vendor='OpenAI';apiKey=$k;url='https://api.laoshirenai.com/v1/chat/completions';supportsToolCall=$true;supportsImages=$true;supportsReasoning=$true;onlyReasoning=$false;useCustomProtocol=$false;maxInputTokens=1050000;maxOutputTokens=128000;reasoning=[pscustomobject]@{defaultEffort='medium';supportedEfforts=$efforts;canDisableThinking=$false}}})",
        "$newModels=@($old|Where-Object{($oldIds -notcontains $_.id)-and($ids -notcontains $_.id)})+$new",
        "if($arrayRoot){$output=@($newModels);$json=ConvertTo-Json -InputObject $output -Depth 20}else{if($envelope.PSObject.Properties['models']){$envelope.models=$newModels}else{$envelope|Add-Member -NotePropertyName models -NotePropertyValue $newModels};if($envelope.PSObject.Properties['availableModels'] -and @($envelope.availableModels).Count -gt 0){$envelope.availableModels=@($envelope.availableModels|Where-Object{($oldIds -notcontains $_)-and($ids -notcontains $_)})+$ids};$json=ConvertTo-Json -InputObject $envelope -Depth 20}",
        "$unchanged=(Test-Path $p)-and([IO.File]::ReadAllText($p)-eq $json)",
        "if($unchanged){Write-Host \"已是最新配置，可选模型：$($ids -join '、')；请完全退出并重新打开 WorkBuddy\"}else{$tmp=\"$p.tmp\";[IO.File]::WriteAllText($tmp,$json,[Text.UTF8Encoding]::new($false));if(Test-Path $p){$backup=\"$p.bak-$(Get-Date -Format yyyyMMddHHmmssfff)\";[IO.File]::Replace($tmp,$p,$backup)}else{[IO.File]::Move($tmp,$p)};Write-Host \"接入完成，可选模型：$($ids -join '、')；请完全退出并重新打开 WorkBuddy\"}",
    ]
    return ";".join(parts)


def workbuddy_macos_payload(group_id: int, group_name: str) -> str:
    group_literal = json.dumps(group_name, ensure_ascii=False)
    return f'''import datetime,getpass,json,os,pathlib,shutil,tempfile,urllib.error,urllib.request
GROUP_ID={group_id}
GROUP_NAME={group_literal}
BASE="https://api.laoshirenai.com"
def fetch(url,key=None):
    headers={{"Accept":"application/json","User-Agent":"laoshirenai-one-line-client-setup/1.0"}}
    if key is not None: headers["Authorization"]="Bearer "+key
    try:
        with urllib.request.urlopen(urllib.request.Request(url,headers=headers),timeout=30) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        try: detail=json.loads(error.read().decode("utf-8","replace")).get("message",str(error))
        except Exception: detail=str(error)
        raise SystemExit("接口请求失败："+detail)
    except Exception as error:
        raise SystemExit("网络请求失败："+str(error))
key=getpass.getpass("请粘贴 "+GROUP_NAME+" API Key：").strip()
if not key: raise SystemExit("API Key 不能为空")
catalog=fetch(BASE+"/api/v1/public/model-pricing")
groups=((catalog.get("data") or {{}}).get("groups") or [])
group=next((item for item in groups if int(item.get("group_id",-1))==GROUP_ID),None)
if group is None: raise SystemExit("找不到分组："+GROUP_NAME+"（ID "+str(GROUP_ID)+"）")
group_ids=sorted({{str(item.get("model")) for item in (group.get("models") or []) if item.get("model")}})
if not group_ids: raise SystemExit("分组没有公开可用模型："+GROUP_NAME)
key_catalog=fetch(BASE+"/v1/models",key)
key_ids={{str(item.get("id")) for item in (key_catalog.get("data") or []) if item.get("id")}}
missing=[model for model in group_ids if model not in key_ids]
if missing: raise SystemExit("该 Key 无权调用分组全部模型，缺少："+"、".join(missing))
path=pathlib.Path.home()/".workbuddy"/"models.json"
path.parent.mkdir(parents=True,exist_ok=True)
if path.exists():
    raw=path.read_text(encoding="utf-8-sig")
    if raw.strip():
        try: root=json.loads(raw)
        except Exception: raise SystemExit("现有 models.json 格式错误，未修改："+str(path))
    else: root=[]
else: root=[]
if isinstance(root,list):
    models=root
    envelope=None
elif isinstance(root,dict):
    models=root.get("models",[])
    if not isinstance(models,list): raise SystemExit("现有 models 字段必须是数组，未修改："+str(path))
    envelope=root
else: raise SystemExit("现有 models.json 顶层必须是数组或对象，未修改："+str(path))
prefix="老实人AI "+GROUP_NAME
old_ids={{str(item.get("id")) for item in models if isinstance(item,dict) and (str(item.get("name","")).startswith(prefix) or (item.get("url")==BASE+"/v1/chat/completions" and item.get("apiKey")==key))}}
kept=[item for item in models if not (isinstance(item,dict) and (str(item.get("id")) in set(group_ids) or str(item.get("id")) in old_ids))]
created=[]
for model in group_ids:
    efforts=["low","medium","high","xhigh","max"] if model in {{"gpt-5.6-sol","gpt-5.6-terra"}} else ["low","medium","high","xhigh"]
    created.append({{"id":model,"name":model,"vendor":"OpenAI","apiKey":key,"url":BASE+"/v1/chat/completions","supportsToolCall":True,"supportsImages":True,"supportsReasoning":True,"onlyReasoning":False,"useCustomProtocol":False,"maxInputTokens":1050000,"maxOutputTokens":128000,"reasoning":{{"defaultEffort":"medium","supportedEfforts":efforts,"canDisableThinking":False}}}})
new_models=kept+created
if envelope is None:
    output=new_models
else:
    envelope["models"]=new_models
    output=envelope
if envelope is not None and "availableModels" in envelope:
    visible=envelope["availableModels"]
    if not isinstance(visible,list): raise SystemExit("现有 availableModels 字段必须是数组，未修改："+str(path))
    if visible: envelope["availableModels"]=[item for item in visible if str(item) not in old_ids and str(item) not in set(group_ids)]+group_ids
text=json.dumps(output,ensure_ascii=False,indent=2)+"\\n"
if path.exists() and path.read_text(encoding="utf-8")==text:
    print("已是最新配置，可选模型："+"、".join(group_ids)+"；请完全退出并重新打开 WorkBuddy")
else:
    if path.exists():
        stamp=datetime.datetime.now().strftime("%Y%m%d%H%M%S%f")
        shutil.copy2(path,str(path)+".bak-"+stamp)
    fd,tmp=tempfile.mkstemp(prefix=".models.json.",suffix=".tmp",dir=path.parent)
    try:
        with os.fdopen(fd,"w",encoding="utf-8",newline="\\n") as handle: handle.write(text)
        os.chmod(tmp,0o600)
        os.replace(tmp,path)
    finally:
        if os.path.exists(tmp): os.unlink(tmp)
    print("接入完成，可选模型："+"、".join(group_ids)+"；请完全退出并重新打开 WorkBuddy")
'''


def render_workbuddy_macos(group_id: int, group_name: str) -> str:
    payload = workbuddy_macos_payload(group_id, group_name).encode("utf-8")
    encoded = base64.b64encode(payload).decode("ascii")
    return (
        "python3 -c 'import base64;exec(compile(base64.b64decode(\""
        + encoded
        + "\"),\"<laoshirenai-workbuddy-setup>\",\"exec\"))'"
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--client", required=True)
    parser.add_argument("--group-id", required=True, type=int)
    parser.add_argument("--group-name", required=True)
    parser.add_argument("--os", required=True, dest="os_name")
    return parser.parse_args()


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    if hasattr(sys.stderr, "reconfigure"):
        sys.stderr.reconfigure(encoding="utf-8")
    args = parse_args()
    key = (args.client.casefold(), args.os_name.casefold())
    if key[0] == "workbuddy" and (
        args.group_id != 58 or args.group_name != "GPT 经济线路"
    ):
        print(
            "no tested WorkBuddy group profile for this group; build protocol and model metadata evidence first",
            file=sys.stderr,
        )
        return 2
    if key == ("workbuddy", "windows"):
        print(render_workbuddy_windows(args.group_id, args.group_name))
        return 0
    if key in {("workbuddy", "macos"), ("workbuddy", "mac") }:
        print(render_workbuddy_macos(args.group_id, args.group_name))
        return 0
    print(
        f"no tested one-line adapter for {args.client!r} on {args.os_name!r}",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

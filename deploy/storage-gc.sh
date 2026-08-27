#!/usr/bin/env bash
set -euo pipefail
exec 9>/run/lock/laoshirenai-storage-gc.lock
/usr/bin/flock -n 9 || exit 0

BEFORE_KB=$(/usr/bin/df --output=used / | /usr/bin/tail -1 | /usr/bin/tr -d ' ')

INSTALLER_SUMMARY=$(/usr/bin/python3 - <<'PY'
import json, pathlib, shutil, subprocess, time
root=pathlib.Path('/var/lib/docker/volumes/laoshirenai-app-tazu5m-data/_data/downloads')
reserved={('claude-desktop','macos-static')}
explicit_legacy={('claude-desktop','latest')}
deleted=[]
skipped=[]
now=time.time()

def size_bytes(p):
    return int(subprocess.check_output(['/usr/bin/du','-sb',str(p)]).split()[0])

def manifest(tool_dir):
    data=json.loads((tool_dir/'manifest.json').read_text())
    version=str(data.get('version','')).strip()
    if not version:
        raise RuntimeError(f'empty manifest version: {tool_dir}')
    return data,version

for manifest_path in sorted(root.glob('*/manifest.json')):
    tool_dir=manifest_path.parent
    tool=tool_dir.name
    _,current=manifest(tool_dir)
    for entry in list(tool_dir.iterdir()):
        if not entry.is_dir() or entry.is_symlink():
            continue
        _,current=manifest(tool_dir)  # fail safe if a sync advanced
        if entry.name==current or (tool,entry.name) in reserved:
            continue
        safe=False
        vm=entry/'.manifest.json'
        if vm.is_file():
            try:
                old=json.loads(vm.read_text())
                safe=(old.get('tool')==tool and str(old.get('version','')).strip()==entry.name)
            except Exception:
                safe=False
        if (tool,entry.name) in explicit_legacy:
            safe=True
        # A failed/incomplete sync has no version manifest. Remove it only when
        # it has been untouched for a full day and no active temp file exists.
        if not safe and now-entry.stat().st_mtime>86400 and not any(entry.rglob('*.tmp')):
            safe=True
        if not safe:
            skipped.append(f'{tool}/{entry.name}')
            continue
        b=size_bytes(entry)
        shutil.rmtree(entry)
        deleted.append((f'{tool}/{entry.name}',b))

# Current published packages must remain complete.
for manifest_path in sorted(root.glob('*/manifest.json')):
    tool_dir=manifest_path.parent
    data,current=manifest(tool_dir)
    version_dir=tool_dir/current
    for asset in data.get('assets',[]):
        p=version_dir/pathlib.Path(str(asset.get('name',''))).name
        if not p.is_file():
            raise RuntimeError(f'missing current asset: {tool_dir.name}/{current}/{p.name}')
        expected=int(asset.get('size',0) or 0)
        if expected and p.stat().st_size!=expected:
            raise RuntimeError(f'bad current asset size: {tool_dir.name}/{current}/{p.name}')

print(f'installer_deleted_dirs={len(deleted)} installer_reclaimed_gib={sum(x[1] for x in deleted)/1024**3:.2f} skipped_in_progress={len(skipped)}')
PY
)

DOCKER_SUMMARY='docker_prune=skipped_service_update'
if ! /usr/bin/docker service ls -q | /usr/bin/xargs -r /usr/bin/docker service inspect --format '{{if .UpdateStatus}}{{.UpdateStatus.State}}{{end}}' | /usr/bin/grep -Eq 'updating|rollback'; then
  /usr/bin/docker container prune -f --filter until=24h >/dev/null
  /usr/bin/docker image prune -a -f --filter until=24h >/dev/null
  DOCKER_SUMMARY='docker_prune=completed_unused_older_than_24h'
fi

AFTER_KB=$(/usr/bin/df --output=used / | /usr/bin/tail -1 | /usr/bin/tr -d ' ')
FREED_KB=$((BEFORE_KB-AFTER_KB))
SUMMARY="$INSTALLER_SUMMARY $DOCKER_SUMMARY filesystem_reclaimed_mib=$((FREED_KB/1024))"
/usr/bin/logger -t laoshirenai-storage-gc -- "$SUMMARY"
printf '%s\n' "$SUMMARY"

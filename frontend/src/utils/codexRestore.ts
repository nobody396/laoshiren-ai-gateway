import { codexRestoreSha256 } from '@/generated/codexRestoreIntegrity'

// This is deliberately Codex-only. Restore needs no Key, group, or setup ticket.
export function buildCodexRestoreCommand(windows: boolean): string {
  const url = `https://laoshirenai.com/auto-config/restore-codex.cjs?v=${codexRestoreSha256}`
  if (windows) {
    return [
      '& {',
      "$ErrorActionPreference='Stop'",
      "$n=Join-Path $HOME '.laoshirenai\\node\\current\\node.exe'",
      "if(!(Test-Path -LiteralPath $n -PathType Leaf)){$n=(Get-Command node.exe -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source}",
      '$p=[IO.Path]::GetTempFileName()',
      `try { Invoke-WebRequest -UseBasicParsing -Uri '${url}' -OutFile $p`,
      `if((Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash.ToLowerInvariant() -ne '${codexRestoreSha256}'){throw 'Restore checksum mismatch'}`,
      '& $n $p',
      "if($LASTEXITCODE -ne 0){throw 'Codex restore failed; see message above'}",
      '} finally { Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue }',
      '}'
    ].join('; ').replace('{; ', '{ ')
  }
  return [
    '(set -eu',
    'n="$HOME/.laoshirenai/node/current/bin/node"',
    '[ -x "$n" ] || n="$(command -v node)"',
    'p="$(mktemp)"',
    `trap 'rm -f "$p"' EXIT`,
    `curl -fsSL --connect-timeout 15 --max-time 120 '${url}' -o "$p"`,
    `"$n" -e 'const f=require("fs"),c=require("crypto");if(c.createHash("sha256").update(f.readFileSync(process.argv[1])).digest("hex")!=="${codexRestoreSha256}")throw Error("Restore checksum mismatch")' "$p"`,
    '"$n" "$p")'
  ].join('; ')
}

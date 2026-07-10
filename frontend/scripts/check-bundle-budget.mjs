import { existsSync, readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { gzipSync } from 'node:zlib'

// First optimized production baseline: 168,578 bytes initial gzip. Budgets keep
// roughly 13%/16% headroom rather than silently accepting a new baseline.
export const DEFAULT_INITIAL_GZIP_BUDGET = 190_000
export const DEFAULT_CHUNK_GZIP_BUDGET = 300_000
export const FORBIDDEN_INITIAL_CHUNKS = ['optional-xlsx', 'optional-editor', 'optional-onboarding', 'optional-chart']

export function collectInitialFiles(manifest, entryKey = 'index.html') {
  const entry = manifest[entryKey] ?? Object.values(manifest).find((item) => item.isEntry)
  if (!entry) throw new Error(`entry not found: ${entryKey}`)
  const files = new Set()
  const visited = new Set()
  function visit(item) {
    if (!item || visited.has(item.file)) return
    visited.add(item.file)
    files.add(item.file)
    for (const css of item.css ?? []) files.add(css)
    for (const key of item.imports ?? []) visit(manifest[key])
  }
  visit(entry)
  return [...files]
}

export function checkBundleBudget({ manifest, distDir, initialBudget, chunkBudget }) {
  const initialFiles = collectInitialFiles(manifest)
  const allFiles = [...new Set(Object.values(manifest).flatMap((item) => [item.file, ...(item.css ?? [])]))]
  const gzipSize = (file) => gzipSync(readFileSync(join(distDir, file))).byteLength
  const initialGzip = initialFiles.reduce((total, file) => total + gzipSize(file), 0)
  const chunks = allFiles.map((file) => ({ file, gzip: gzipSize(file) }))
  const failures = []
  if (initialGzip > initialBudget) failures.push(`initial gzip ${initialGzip} > ${initialBudget}`)
  for (const chunk of chunks) {
    if (chunk.gzip > chunkBudget) failures.push(`chunk ${chunk.file} gzip ${chunk.gzip} > ${chunkBudget}`)
  }
  for (const file of initialFiles) {
    if (FORBIDDEN_INITIAL_CHUNKS.some((name) => file.includes(name))) {
      failures.push(`optional chunk preloaded by initial route: ${file}`)
    }
  }
  return { initialFiles, initialGzip, chunks, failures }
}

function parseArg(name, fallback) {
  const index = process.argv.indexOf(name)
  return index >= 0 ? process.argv[index + 1] : fallback
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  const manifestPath = resolve(parseArg('--manifest', '../backend/internal/web/dist/.vite/manifest.json'))
  if (!existsSync(manifestPath)) throw new Error(`manifest missing: ${manifestPath}`)
  const result = checkBundleBudget({
    manifest: JSON.parse(readFileSync(manifestPath, 'utf8')),
    distDir: dirname(dirname(manifestPath)),
    initialBudget: Number(process.env.BUNDLE_INITIAL_GZIP_BUDGET || DEFAULT_INITIAL_GZIP_BUDGET),
    chunkBudget: Number(process.env.BUNDLE_CHUNK_GZIP_BUDGET || DEFAULT_CHUNK_GZIP_BUDGET)
  })
  console.log(JSON.stringify({ initialGzip: result.initialGzip, initialFiles: result.initialFiles, largest: result.chunks.sort((a, b) => b.gzip - a.gzip).slice(0, 5) }))
  if (result.failures.length) {
    for (const failure of result.failures) console.error(failure)
    process.exitCode = 1
  }
}

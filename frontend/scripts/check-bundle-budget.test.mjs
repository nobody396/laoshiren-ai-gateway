import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { checkBundleBudget, collectInitialFiles } from './check-bundle-budget.mjs'

const manifest = {
  'index.html': { file: 'assets/index.js', isEntry: true, imports: ['_shared.js'], css: ['assets/index.css'] },
  '_shared.js': { file: 'assets/shared.js' },
  'lazy.ts': { file: 'assets/optional-xlsx.js', isDynamicEntry: true }
}

test('walks only static imports for the initial route', () => {
  assert.deepEqual(collectInitialFiles(manifest).sort(), ['assets/index.css', 'assets/index.js', 'assets/shared.js'])
})

test('rejects initial and single chunk regressions', () => {
  const root = mkdtempSync(join(tmpdir(), 'bundle-budget-'))
  mkdirSync(join(root, 'assets'))
  for (const file of ['index.js', 'index.css', 'shared.js', 'optional-xlsx.js']) {
    writeFileSync(join(root, 'assets', file), Buffer.from(Array.from({ length: 200 }, (_, i) => i % 251)))
  }
  const result = checkBundleBudget({ manifest, distDir: root, initialBudget: 10, chunkBudget: 10 })
  assert.ok(result.failures.some((failure) => failure.startsWith('initial gzip')))
  assert.ok(result.failures.some((failure) => failure.startsWith('chunk ')))
})

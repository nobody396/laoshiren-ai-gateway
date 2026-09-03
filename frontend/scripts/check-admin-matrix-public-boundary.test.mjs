import assert from 'node:assert/strict'
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { findLeaks } from './check-admin-matrix-public-boundary.mjs'

test('findLeaks detects a complete admin matrix marker and ignores public summaries', async () => {
  const root = await mkdtemp(join(tmpdir(), 'admin-matrix-boundary-'))
  try {
    await mkdir(join(root, 'assets'))
    await writeFile(join(root, 'assets', 'public.js'), 'const counts={models:34,clients:17}')
    assert.deepEqual(await findLeaks(root), [])
    await writeFile(join(root, 'assets', 'leak.js'), 'const client_transport_feature_ids=[]')
    assert.deepEqual(await findLeaks(root), [join(root, 'assets', 'leak.js')])
  } finally {
    await rm(root, { recursive: true, force: true })
  }
})

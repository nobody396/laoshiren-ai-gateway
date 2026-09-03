import { readFile, readdir, stat } from 'node:fs/promises'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

export const ADMIN_MATRIX_MARKERS = [
  'client_transport_feature_ids',
  'evidence_reuse_match',
  'client-matrix-source-20260831',
]

export async function findLeaks(directory, markers = ADMIN_MATRIX_MARKERS) {
  const leaks = []
  async function walk(path) {
    for (const name of await readdir(path)) {
      const child = resolve(path, name)
      const info = await stat(child)
      if (info.isDirectory()) await walk(child)
      else {
        const content = await readFile(child)
        const text = content.toString('utf8')
        if (markers.some(marker => text.includes(marker))) leaks.push(child)
      }
    }
  }
  await walk(directory)
  return leaks
}

export async function checkAdminMatrixPublicBoundary(root = resolve(fileURLToPath(new URL('..', import.meta.url)))) {
  const view = await readFile(resolve(root, 'src/views/admin/ModelClientMatrixView.vue'), 'utf8')
  if (view.includes('clientMatrixAdmin')) throw new Error('admin matrix view imports a static admin matrix')
  const leaks = await findLeaks(resolve(root, '../backend/internal/web/dist'))
  if (leaks.length) throw new Error(`admin matrix markers leaked into public build: ${leaks.join(', ')}`)
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await checkAdminMatrixPublicBoundary()
  console.log('admin matrix public boundary: ok')
}

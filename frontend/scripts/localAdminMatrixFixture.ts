import { readFile } from 'node:fs/promises'
import type { Plugin, ViteDevServer } from 'vite'

export const LOCAL_PREVIEW_ADMIN_TOKEN = 'local-preview-admin-token'

interface LocalAdminMatrixFixtureOptions {
  enabled: boolean
  fixturePath: string
  token?: string
}

/** Serve the backend-generated matrix only from the Vite development server. */
export function localAdminMatrixFixture(options: LocalAdminMatrixFixtureOptions): Plugin {
  const token = options.token ?? LOCAL_PREVIEW_ADMIN_TOKEN
  return {
    name: 'local-admin-model-client-matrix-fixture',
    apply: 'serve',
    configureServer(server: ViteDevServer) {
      if (!options.enabled) return
      server.middlewares.use(async (req, res, next) => {
        const path = String(req.url || '').split('?')[0]
        if (req.method !== 'GET' || path !== '/api/v1/admin/model-client-matrix') {
          next()
          return
        }

        res.setHeader('X-Local-Preview-Fixture', '1')
        res.setHeader('Cache-Control', 'private, no-store')
        res.setHeader('Content-Type', 'application/json; charset=utf-8')
        if (req.headers.authorization !== `Bearer ${token}`) {
          res.statusCode = 401
          res.end(JSON.stringify({ code: 'UNAUTHORIZED', message: 'Local preview administrator session required' }))
          return
        }

        try {
          res.statusCode = 200
          res.end(await readFile(options.fixturePath))
        } catch {
          res.statusCode = 500
          res.end(JSON.stringify({ code: 'FIXTURE_UNAVAILABLE', message: 'Local preview matrix fixture is unavailable' }))
        }
      })
    }
  }
}

import { EventEmitter } from 'node:events'
import { mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { LOCAL_PREVIEW_ADMIN_TOKEN, localAdminMatrixFixture } from '../../scripts/localAdminMatrixFixture'

const temporaryDirectories: string[] = []

afterEach(async () => {
  await Promise.all(temporaryDirectories.splice(0).map(path => rm(path, { recursive: true, force: true })))
})

function captureMiddleware(plugin: ReturnType<typeof localAdminMatrixFixture>) {
  let middleware: any
  const server = { middlewares: { use(handler: any) { middleware = handler } } }
  if (typeof plugin.configureServer === 'function') plugin.configureServer(server as any)
  return middleware
}

async function invoke(middleware: any, options: { path?: string; token?: string } = {}) {
  const req = new EventEmitter() as any
  req.method = 'GET'
  req.url = options.path ?? '/api/v1/admin/model-client-matrix'
  req.headers = options.token ? { authorization: `Bearer ${options.token}` } : {}
  const headers = new Map<string, string>()
  let body = Buffer.alloc(0)
  let nextCalled = false
  const res = {
    statusCode: 0,
    setHeader(name: string, value: string) { headers.set(name, value) },
    end(value?: string | Buffer) { body = Buffer.isBuffer(value) ? value : Buffer.from(value ?? '') },
  }
  await middleware(req, res, () => { nextCalled = true })
  return { status: res.statusCode, headers, body: body.toString(), nextCalled }
}

describe('localAdminMatrixFixture', () => {
  it('is serve-only and does not register when local preview is disabled', () => {
    const plugin = localAdminMatrixFixture({ enabled: false, fixturePath: '/missing' })
    expect(plugin.apply).toBe('serve')
    expect(captureMiddleware(plugin)).toBeUndefined()
  })

  it('requires the local preview admin token and marks fixture responses', async () => {
    const directory = await mkdtemp(join(tmpdir(), 'admin-matrix-fixture-'))
    temporaryDirectories.push(directory)
    const fixturePath = join(directory, 'matrix.json')
    await writeFile(fixturePath, JSON.stringify({ counts: { models: 34, clients: 17, intersections: 578 } }))
    const middleware = captureMiddleware(localAdminMatrixFixture({ enabled: true, fixturePath }))

    const denied = await invoke(middleware)
    expect(denied.status).toBe(401)
    expect(denied.headers.get('X-Local-Preview-Fixture')).toBe('1')
    expect(denied.body).not.toContain('intersections')

    const allowed = await invoke(middleware, { token: LOCAL_PREVIEW_ADMIN_TOKEN })
    expect(allowed.status).toBe(200)
    expect(allowed.headers.get('X-Local-Preview-Fixture')).toBe('1')
    expect(JSON.parse(allowed.body).counts).toEqual({ models: 34, clients: 17, intersections: 578 })
  })

  it('passes unrelated requests to the normal proxy chain', async () => {
    const middleware = captureMiddleware(localAdminMatrixFixture({ enabled: true, fixturePath: '/missing' }))
    const result = await invoke(middleware, { path: '/api/v1/public/model-pricing' })
    expect(result.nextCalled).toBe(true)
    expect(result.headers.has('X-Local-Preview-Fixture')).toBe(false)
  })
})

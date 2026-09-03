import { defineConfig, loadEnv, Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import checker from 'vite-plugin-checker'
import { resolve } from 'path'
import { localAdminMatrixFixture, LOCAL_PREVIEW_ADMIN_TOKEN } from './scripts/localAdminMatrixFixture'

/**
 * Vite 插件：开发模式下 mock 后端 setup 接口，跳过安装向导
 */
function mockSetupStatus(): Plugin {
  return {
    name: 'mock-setup-status',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use('/setup/status', (_req, res) => {
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify({ needs_setup: false, step: '' }))
      })
    }
  }
}

/**
 * 本地文档预览的管理员会话。
 *
 * 仅在显式提供 LOCAL_PREVIEW_ADMIN_PASSWORD 时启用，避免 127.0.0.1 的
 * 登录请求被代理到生产环境。它只覆盖预览所需的登录、当前用户和 RBAC
 * 读取接口，其余 API 仍按原配置代理。
 */
function mockLocalPreviewAdmin(password: string | undefined): Plugin {
  const acceptAnyPreviewPassword = password === '__LOCAL_PREVIEW_ANY__'
  const token = LOCAL_PREVIEW_ADMIN_TOKEN
  const refreshToken = 'local-preview-admin-refresh-token'
  const now = new Date().toISOString()
  const user = {
    id: 1,
    username: 'admin',
    email: 'admin@example.com',
    role: 'admin',
    balance: 0,
    concurrency: 100,
    status: 'active',
    allowed_groups: null,
    created_at: now,
    updated_at: now,
    first_recharged: false,
    run_mode: 'standard'
  }

  const send = (res: any, data: unknown, status = 200) => {
    res.statusCode = status
    res.setHeader('Content-Type', 'application/json; charset=utf-8')
    res.end(JSON.stringify(status < 400
      ? { code: 0, message: 'success', data }
      : { code: 'INVALID_CREDENTIALS', message: 'invalid email or password' }))
  }

  const readJsonBody = (req: any): Promise<Record<string, unknown>> => new Promise((resolveBody) => {
    let raw = ''
    req.on('data', (chunk: Buffer) => { raw += chunk.toString() })
    req.on('end', () => {
      try { resolveBody(JSON.parse(raw || '{}')) } catch { resolveBody({}) }
    })
  })

  const authorized = (req: any) => req.headers.authorization === `Bearer ${token}`

  return {
    name: 'mock-local-preview-admin',
    apply: 'serve',
    configureServer(server) {
      if (!password) return
      server.middlewares.use(async (req, res, next) => {
        const path = String(req.url || '').split('?')[0]

        if (req.method === 'POST' && path === '/api/v1/auth/login') {
          const body = await readJsonBody(req)
          const passwordAccepted = acceptAnyPreviewPassword
            ? typeof body.password === 'string' && body.password.length > 0
            : body.password === password
          if (body.email !== 'admin@example.com' || !passwordAccepted) {
            send(res, null, 401)
            return
          }
          send(res, {
            access_token: token,
            refresh_token: refreshToken,
            expires_in: 86400,
            token_type: 'Bearer',
            user
          })
          return
        }

        if (req.method === 'POST' && path === '/api/v1/auth/refresh') {
          const body = await readJsonBody(req)
          if (body.refresh_token !== refreshToken) {
            send(res, null, 401)
            return
          }
          send(res, {
            access_token: token,
            refresh_token: refreshToken,
            expires_in: 86400,
            token_type: 'Bearer'
          })
          return
        }

        if (req.method === 'GET' && path === '/api/v1/auth/me' && authorized(req)) {
          send(res, user)
          return
        }

        if (req.method === 'GET' && path === '/api/v1/admin/rbac/me/permissions' && authorized(req)) {
          send(res, ['*'])
          return
        }

        if (req.method === 'GET' && path === '/api/v1/admin/rbac/menu' && authorized(req)) {
          send(res, [])
          return
        }

        // App.vue 与后台布局会在登录后自动预加载这些资源。若继续代理到
        // 生产环境，生产后台会把本地预览 Token 判为无效并触发全局登出。
        if (req.method === 'GET' && path === '/api/v1/subscriptions/active' && authorized(req)) {
          send(res, [])
          return
        }

        if (req.method === 'GET' && path === '/api/v1/announcements' && authorized(req)) {
          send(res, [])
          return
        }

        if (req.method === 'GET' && path === '/api/v1/announcements/popup-state' && authorized(req)) {
          send(res, { last_prompted_announcement_id: 0 })
          return
        }

        if (req.method === 'GET' && path === '/api/v1/admin/settings' && authorized(req)) {
          send(res, {
            ops_monitoring_enabled: true,
            ops_realtime_monitoring_enabled: true,
            ops_query_mode_default: 'auto',
            custom_menu_items: []
          })
          return
        }

        if (
          req.method === 'GET'
          && path === '/api/v1/admin/agents/affiliate-operations-summary'
          && authorized(req)
        ) {
          send(res, { actionable_total: 0 })
          return
        }

        if (req.method === 'GET' && path === '/api/v1/changelog/latest' && authorized(req)) {
          send(res, null)
          return
        }

        if (req.method === 'GET' && path === '/api/v1/notifications' && authorized(req)) {
          send(res, { items: [], unread_count: 0 })
          return
        }

        if (
          req.method === 'GET'
          && path === '/api/v1/admin/feedbacks/agent-queue'
          && authorized(req)
        ) {
          send(res, { items: [], total: 0, page: 1, page_size: 5, pages: 0 })
          return
        }

        if (req.method === 'GET' && path === '/api/v1/public/model-pricing' && authorized(req)) {
          send(res, {
            updated_at: now,
            currency: 'CNY',
            unit: '1M tokens',
            groups: []
          })
          return
        }

        next()
      })
    }
  }
}

/**
 * Vite 插件：开发模式下注入公开配置到 index.html
 * 与生产模式的后端注入行为保持一致，消除闪烁
 */
function injectPublicSettings(backendUrl: string): Plugin {
  return {
    name: 'inject-public-settings',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        try {
          const response = await fetch(`${backendUrl}/api/v1/settings/public`, {
            signal: AbortSignal.timeout(2000)
          })
          if (response.ok) {
            const data = await response.json()
            if (data.code === 0 && data.data) {
              const script = `<script>window.__APP_CONFIG__=${JSON.stringify(data.data)};</script>`
              return html.replace('</head>', `${script}\n</head>`)
            }
          }
        } catch (e) {
          console.warn('[vite] 无法获取公开配置，将回退到 API 调用:', (e as Error).message)
        }
        return html
      }
    }
  }
}

export default defineConfig(({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, process.cwd(), '')
  const backendUrl = env.VITE_DEV_PROXY_TARGET || 'http://localhost:8080'
  const devPort = Number(env.VITE_DEV_PORT || 3000)
  const localPreviewAdminPassword = process.env.LOCAL_PREVIEW_ADMIN_PASSWORD
    ?? (process.argv.includes('4178') ? '__LOCAL_PREVIEW_ANY__' : undefined)

  return {
    plugins: [
      vue(),
      checker({
        typescript: true,
        vueTsc: true
      }),
      mockSetupStatus(),
      mockLocalPreviewAdmin(
        localPreviewAdminPassword
      ),
      localAdminMatrixFixture({
        enabled: Boolean(localPreviewAdminPassword),
        fixturePath: resolve(__dirname, '../backend/internal/adminmatrix/model_client_matrix.json')
      }),
      injectPublicSettings(backendUrl)
    ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      // 使用 vue-i18n 运行时版本，避免 CSP unsafe-eval 问题
      'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'
    }
  },
  define: {
    // 启用 vue-i18n JIT 编译，在 CSP 环境下处理消息插值
    // JIT 编译器生成 AST 对象而非 JS 代码，无需 unsafe-eval
    __INTLIFY_JIT_COMPILATION__: true
  },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    manifest: true,
    rollupOptions: {
      output: {
        onlyExplicitManualChunks: true,
        /**
         * 手动分包配置
         * 分离第三方库并按功能合并应用代码，避免循环依赖
         */
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // Vue 核心库
            if (
              id.includes('/vue/') ||
              id.includes('/vue-router/') ||
              id.includes('/pinia/') ||
              id.includes('/@vue/')
            ) {
              return 'vendor-vue'
            }

            // XLSX、编辑器、引导和图表只允许由对应的懒加载功能引用。
            if (id.includes('/xlsx/')) {
              return 'optional-xlsx'
            }
            if (id.includes('/md-editor-v3/')) {
              return 'optional-editor'
            }
            if (id.includes('/marked/') || id.includes('/dompurify/')) {
              return 'vendor-markdown'
            }
            if (id.includes('/driver.js/')) {
              return 'optional-onboarding'
            }

            // UI 工具库（较大，单独分离）
            if (id.includes('/@vueuse/')) {
              return 'vendor-ui'
            }

            // 图表库
            if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
              return 'optional-chart'
            }

            // 国际化
            if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
              return 'vendor-i18n'
            }

            // 其余依赖由 Rollup 根据静态/动态入口关系自动分包，禁止 catch-all vendor。
            return undefined
          }

          // 应用代码：按入口点自动分包，不手动干预
          // 这样可以避免循环依赖，同时保持合理的 chunk 数量
        }
      }
    }
  },
    server: {
      host: '0.0.0.0',
      port: devPort,
      proxy: {
        '/api': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1': {
          target: backendUrl,
          changeOrigin: true
        },
        '/__gateway': {
          target: 'https://api.laoshirenai.com',
          changeOrigin: true,
          rewrite: path => path.replace(/^\/__gateway/, '')
        },
        '/setup': {
          target: backendUrl,
          changeOrigin: true
        }
      }
    }
  }
})

# SECURITY FIX C1: Google API Key Auth Parity

Date: 2026-05-16

## 修复了什么

- Google/Gemini/Antigravity v1beta API key 中间件现在拒绝 query 参数中的 `key` 和 `api_key`，要求使用 `Authorization: Bearer ...`、`x-goog-api-key` 或 `x-api-key` header。
- Google-style API key 路径现在执行 API key 的 IP whitelist/blacklist 限制，并使用与标准 API key 中间件一致的可信客户端 IP 解析逻辑。
- Google-style API key 路径现在拒绝 `expired` 和 `quota_exhausted` 状态。
- Google-style API key 路径现在拒绝由 `ExpiresAt` 和 `Quota`/`QuotaUsed` 字段计算出的 runtime expired 和 runtime quota exhausted。
- 保留了已有用户 active、余额、订阅组限制校验，以及 Google-style error response 格式。

## Changed files

- `backend/internal/server/middleware/api_key_auth_google.go`
- `backend/internal/server/middleware/api_key_auth_google_test.go`
- `SECURITY_FIX_C1_GOOGLE_API_KEY_AUTH_PARITY_2026-05-16.md`

## 新增/更新了哪些测试

- 更新 `TestApiKeyAuthWithSubscriptionGoogle_QueryApiKeyRejected`，明确 `api_key` query 参数必须拒绝。
- 将原 `/v1beta?key=` 兼容放行测试改为 `TestApiKeyAuthWithSubscriptionGoogle_QueryKeyRejectedOnV1Beta`。
- 新增 `TestApiKeyAuthWithSubscriptionGoogle_QueryKeyRejectedOnAntigravityV1Beta`。
- 新增 `TestApiKeyAuthWithSubscriptionGoogle_RejectsExpiredAndQuotaExhaustedStatus`。
- 新增 `TestApiKeyAuthWithSubscriptionGoogle_RejectsRuntimeExpiredAndQuotaExhausted`。
- 新增 `TestApiKeyAuthWithSubscriptionGoogle_IPRestrictionDoesNotTrustSpoofedForwardHeaders`。

## 已运行哪些测试及结果

- `go test -tags=unit ./internal/server/middleware -run 'TestApiKeyAuthWithSubscriptionGoogle_(QueryKeyRejected|Rejects|IPRestriction)' -count=1`
  - 修复前失败，复现了 C1 差异。
- `go test -tags=unit ./internal/server/middleware -run 'TestApiKeyAuthWithSubscriptionGoogle' -count=1`
  - 通过。
- `go test -tags=unit ./internal/server/middleware -run 'Test(APIKeyAuth|ApiKeyAuthWithSubscriptionGoogle)' -count=1`
  - 通过。
- `go build -o /tmp/sub2api-server-check ./cmd/server`
  - 通过。

## 上线前还需要做什么

- 在 staging 环境验证 `/v1beta` 和 `/antigravity/v1beta` 客户端均使用 header 传递 API key。
- 通知仍使用 query 参数 `key` 或 `api_key` 的客户端迁移到 header。
- 确认反向代理可信代理配置正确，使 `gin.Context.ClientIP()` 的可信 IP 解析链符合生产网络拓扑。

## 是否需要数据库迁移

不需要。

## 是否有剩余风险

- C1 范围内未保留 `/v1beta` 或 `/antigravity/v1beta` 的 query `key=` 兼容例外，因此没有继续允许 query-based key usage 的剩余风险。
- 兼容性风险：仍依赖 query 参数传 key 的客户端会收到 Google-style `400 INVALID_ARGUMENT`，需要迁移到 header。

## 下一个待处理问题

C2: admin account credentials are exposed by default in responses.

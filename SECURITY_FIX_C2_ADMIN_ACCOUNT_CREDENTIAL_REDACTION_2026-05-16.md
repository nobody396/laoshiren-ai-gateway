# SECURITY FIX C2: Admin Account Credential Redaction

Date: 2026-05-16

## 修复了什么

修复管理端普通 account 响应默认暴露上游凭据的问题。

`dto.AccountFromServiceShallow` 现在在 HTTP response DTO 边界对 `credentials` 做响应安全复制，移除 token/key/password/secret/authorization 类字段，例如 `access_token`、`refresh_token`、`api_key`、`authorization`、`password`、`secret`、`client_secret`、`aws_secret_access_key`、`aws_session_token` 等。

保留普通 UI 需要的非敏感配置字段，例如 `base_url`、`model_mapping`、`tier_id`、`plan_type`、`expires_at`、`subscription_expires_at`、`intercept_warmup_requests`、`pool_mode`、`pool_mode_retry_count` 等。

DTO 脱敏和 service 更新合并共用 `internal/pkg/accountcredentials` 的敏感 key 判定，避免响应脱敏规则和保存兼容规则分叉。

管理端更新 account credentials 时现在使用合并策略：脱敏响应里缺省的敏感字段会从已有 `service.Account.Credentials` 保留；如果请求显式提交新的敏感字段，则使用新值替换旧值。这样普通编辑 `base_url`、`model_mapping`、`intercept_warmup_requests` 等非敏感字段不会清掉既有上游 `api_key`、OAuth token 或 AWS secret/session token。

修复没有修改持久化 schema，也没有改变运行时读取完整 credentials 的路径。

导出/迁移类接口保持不变：`backend/internal/handler/admin/account_data.go` 的 `accounts/data` 导出仍按现有导出语义返回 credentials。

## Changed Files

- `backend/internal/handler/dto/account_credentials.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/account_credentials_redaction_test.go`
- `backend/internal/handler/admin/account_handler_credentials_redaction_test.go`
- `backend/internal/handler/admin/admin_service_stub_test.go`
- `backend/internal/pkg/accountcredentials/credentials.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/admin_service_credentials_test.go`
- `SECURITY_FIX_C2_ADMIN_ACCOUNT_CREDENTIAL_REDACTION_2026-05-16.md`

## 新增/更新了哪些测试

- 新增 DTO 回归测试：`TestAccountFromServiceRedactsCredentialSecrets`
  - 断言普通 account DTO JSON 不包含 sentinel secret values。
  - 断言非敏感配置字段仍保留。
  - 断言源 `service.Account.Credentials` 未被修改。
- 新增 admin handler 回归测试：`TestAccountHandlerListAndDetailRedactCredentialSecrets`
  - 覆盖普通 admin account list/detail JSON。
  - 断言 sentinel secret values 不出现在响应体。
  - 断言 `base_url` 和 `intercept_warmup_requests` 等配置仍可返回。
- 更新 `stubAdminService.GetAccount`，使 handler detail 测试能返回测试预置账号。
- 新增 service 更新回归测试：`TestUpdateAccountCredentialsPreservesMissingSensitiveKeysAfterRedactedEdit`
  - 断言已有 `api_key` / `access_token` 在脱敏后的非敏感字段更新中仍保留。
  - 断言 `base_url`、`model_mapping`、`intercept_warmup_requests` 可正常更新。
- 新增 service 更新回归测试：`TestUpdateAccountCredentialsExplicitSensitiveKeyReplacesExistingValue`
  - 断言显式提交新的敏感字段时会替换旧值。
  - 断言其他缺省敏感字段仍保留。

## 已运行哪些测试及结果

- `go test -tags=unit ./internal/handler/dto ./internal/handler/admin ./internal/service -run 'Test.*(Account|Credential|Redact|C2)' -count=1`
  - 结果：通过。
- `go test -tags=unit ./internal/handler/admin -run 'Test.*Account' -count=1`
  - 结果：通过。
- `go build -o /tmp/sub2api-server-check ./cmd/server`
  - 结果：通过。

修复前，新增回归测试可复现 C2：DTO 和 admin list 响应均包含 sentinel credential secret；返工新增的 service 更新测试可复现脱敏响应副本覆盖持久化 credentials 导致 secret 丢失。修复后，上述测试均通过。

## 上线前还需要做什么

- 在预发或等价非生产环境确认管理端账号列表、详情、创建、更新、清错、可调度切换、刷新后响应仍能展示必要的非敏感配置字段。
- 重点确认管理端编辑保存路径：普通非敏感字段更新不应清掉既有 secret；本次已在 service 层添加回归测试覆盖。
- 确认任何依赖普通 account JSON 读取完整 secret 的内部脚本已改用明确的导出/迁移接口或服务端内部读取。

## 是否需要数据库迁移

不需要数据库迁移。

## 是否有剩余风险

- 当前修复基于 credentials key 名称过滤。若未来新增凭据字段使用无法识别的命名，仍需要同步加入过滤规则或改为显式 allowlist DTO。
- `accounts/data` 等导出/迁移接口仍可能包含 secrets，这是本次明确排除的导出语义，不属于普通默认响应。
- 用户 API key 列表/detail 的 key 脱敏未处理；这是 C2 handoff 中暂停的 P2 follow-up，本次未做。

## 下一个待处理问题

D2: usage billing, usage_log, and commission are not atomic

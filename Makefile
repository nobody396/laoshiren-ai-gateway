.PHONY: build build-backend build-frontend build-datamanagementd test test-backend test-backend-integration test-frontend test-datamanagementd secret-scan checkout-validate

GIT_SHA ?= $(shell git rev-parse HEAD)

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 编译 datamanagementd（宿主机数据管理进程）
build-datamanagementd:
	@cd datamanagement && go build -o datamanagementd ./cmd/datamanagementd

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-backend-integration:
	@$(MAKE) -C backend test-integration GIT_SHA=$(GIT_SHA)

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck

test-datamanagementd:
	@cd datamanagement && go test ./...

secret-scan:
	@python3 tools/secret_scan.py

checkout-validate:
	@python3 tools/validate_checkout_registry.py --action develop

.PHONY: affiliate-staging-init affiliate-staging-validate affiliate-staging-up affiliate-staging-down affiliate-staging-status

affiliate-staging-init:
	@./scripts/affiliate-v2-staging.sh init-secrets

affiliate-staging-validate:
	@./scripts/affiliate-v2-staging.sh validate

affiliate-staging-up:
	@./scripts/affiliate-v2-staging.sh up

affiliate-staging-down:
	@./scripts/affiliate-v2-staging.sh down

affiliate-staging-status:
	@./scripts/affiliate-v2-staging.sh status

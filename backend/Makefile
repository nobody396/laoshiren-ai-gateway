.PHONY: build generate test test-unit test-integration test-e2e

VERSION ?= $(shell tr -d '\r\n' < ./cmd/server/VERSION)
LDFLAGS ?= -s -w -X main.Version=$(VERSION)
GIT_SHA ?= $(shell git rev-parse HEAD)
INTEGRATION_TIMEOUT ?= 20m
INTEGRATION_PACKAGES := ./internal/repository ./internal/middleware ./internal/server/routes
INTEGRATION_SENTINEL := TestIntegrationHarnessSentinel

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -trimpath -o bin/server ./cmd/server

generate:
	go generate ./ent
	go generate ./cmd/server

test:
	go test ./...
	golangci-lint run ./...

test-unit:
	go test -tags=unit ./...

test-integration:
	@set -eu; \
		echo "integration_test_commit_sha=$(GIT_SHA)"; \
		if [ "$${CI:-}" = "true" ]; then \
			docker info >/dev/null 2>&1 || { echo "CI=true requires a working Docker daemon" >&2; exit 1; }; \
		fi; \
		log_file="$$(mktemp -t sub2api-integration.XXXXXX)"; \
		trap 'rm -f "$$log_file"' EXIT HUP INT TERM; \
		set +e; \
		go test -p=1 -tags=integration -count=1 -timeout=$(INTEGRATION_TIMEOUT) -v $(INTEGRATION_PACKAGES) >"$$log_file" 2>&1; \
		test_status=$$?; \
		set -e; \
		cat "$$log_file"; \
		if [ $$test_status -ne 0 ]; then exit $$test_status; fi; \
		if ! grep -Eq -- '--- PASS: $(INTEGRATION_SENTINEL)( |$$)' "$$log_file"; then \
			echo "required repository integration sentinel did not execute" >&2; \
			exit 1; \
		fi

test-e2e:
	./scripts/e2e-test.sh

test-e2e-local:
	go test -tags=e2e -v -timeout=300s ./internal/integration/...

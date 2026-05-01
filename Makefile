.DEFAULT_GOAL := help
GO ?= go
GH ?= gh
PROJECT := go-rdp-android

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; OFS=""; print ""} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: test
test: ## Run Go tests
	$(GO) test ./... -count=1

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: check
check: vet test ## Run vet and tests

.PHONY: build-go
build-go: ## Build Go mock/server packages
	$(GO) build ./...

.PHONY: run-mock
run-mock: ## Run the desktop mock RDP server on :3390
	$(GO) run ./cmd/mock-server

.PHONY: probe
probe: ## Probe a running mock server with TPKT/X.224/MCS handshake
	$(GO) run ./cmd/probe -addr 127.0.0.1:3390

.PHONY: smoke
smoke: ## Run mock server and probe it locally
	@set -eu; \
	$(GO) run ./cmd/mock-server >mock-server.log 2>&1 & \
	pid=$$!; \
	trap 'kill $$pid 2>/dev/null || true; cat mock-server.log; rm -f mock-server.log' EXIT; \
	sleep 2; \
	$(GO) run ./cmd/probe

.PHONY: coverage
coverage: ## Run Go coverage
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

.PHONY: android-build
android-build: ## Build Android debug APK (requires Android SDK + Gradle)
	cd android && gradle :app:assembleDebug

.PHONY: ci
ci: ci-status ## Show latest GitHub Actions run status

.PHONY: ci-list
ci-list: ## List recent GitHub Actions runs
	$(GH) run list --limit 10

.PHONY: ci-status
ci-status: ## Show latest GitHub Actions run summary
	$(GH) run view --json databaseId,status,conclusion,headSha,displayTitle,createdAt,updatedAt,url

.PHONY: ci-jobs
ci-jobs: ## Show jobs for the latest GitHub Actions run
	$(GH) run view --json jobs --jq '.jobs[] | [.name, .status, (.conclusion // ""), .startedAt, .completedAt] | @tsv'

.PHONY: ci-watch
ci-watch: ## Watch latest GitHub Actions run until completion
	$(GH) run watch

.PHONY: ci-log
ci-log: ## Show failed logs for the latest GitHub Actions run
	$(GH) run view --log-failed

.PHONY: ci-log-all
ci-log-all: ## Show all logs for the latest GitHub Actions run
	$(GH) run view --log

.PHONY: ci-rerun
ci-rerun: ## Rerun the latest failed GitHub Actions jobs
	$(GH) run rerun --failed

.PHONY: clean
clean: ## Clean generated outputs
	$(GO) clean
	rm -f coverage.out coverage.html
	rm -rf bin android/.gradle android/build android/app/build

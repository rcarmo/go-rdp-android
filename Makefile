.DEFAULT_GOAL := help
GO ?= go
GH ?= gh
GOMOBILE ?= gomobile
PROJECT := go-rdp-android
MOBILE_AAR := android/app/libs/mobile.aar
COVERAGE_MIN ?= 75.0

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

.PHONY: run-mock-pattern
run-mock-pattern: ## Run the mock server with animated test-pattern frames
	GO_RDP_ANDROID_TRACE=1 $(GO) run ./cmd/mock-server -test-pattern -width 320 -height 240 -fps 5

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
coverage: ## Run Go coverage and enforce COVERAGE_MIN
	mkdir -p .gotmp
	GOTMPDIR="$(CURDIR)/.gotmp" $(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tee coverage.func.txt
	GOTMPDIR="$(CURDIR)/.gotmp" $(GO) run ./scripts/check-coverage.go coverage.func.txt $(COVERAGE_MIN)
	rm -rf .gotmp

.PHONY: gomobile-init
gomobile-init: ## Install/init gomobile tooling
	$(GO) install golang.org/x/mobile/cmd/gomobile@latest
	$(GOMOBILE) init

.PHONY: gomobile-bind
gomobile-bind: ## Build mobile.aar from the Go mobile package
	mkdir -p android/app/libs
	$(GOMOBILE) bind -target=android -androidapi 29 -o $(MOBILE_AAR) ./mobile

.PHONY: check-aar-api
check-aar-api: ## Verify generated gomobile AAR Java API shape
	mkdir -p .gotmp
	GOTMPDIR="$(CURDIR)/.gotmp" $(GO) run ./scripts/check-aar-api.go $(MOBILE_AAR)
	rm -rf .gotmp

.PHONY: check-aar-artifact
check-aar-artifact: ## Verify generated gomobile AAR contents
	mkdir -p .gotmp
	GOTMPDIR="$(CURDIR)/.gotmp" $(GO) run ./scripts/check-android-artifact.go aar $(MOBILE_AAR)
	rm -rf .gotmp

.PHONY: check-apk-artifact
check-apk-artifact: ## Verify debug APK contents; set REQUIRE_GO_LIBS=1 for Go-backed APK
	mkdir -p .gotmp
	@if [ "$(REQUIRE_GO_LIBS)" = "1" ]; then \
		GOTMPDIR="$(CURDIR)/.gotmp" $(GO) run ./scripts/check-android-artifact.go apk android/app/build/outputs/apk/debug/app-debug.apk --require-go-libs; \
	else \
		GOTMPDIR="$(CURDIR)/.gotmp" $(GO) run ./scripts/check-android-artifact.go apk android/app/build/outputs/apk/debug/app-debug.apk; \
	fi
	rm -rf .gotmp

.PHONY: android-build
android-build: ## Build Android debug APK (requires Android SDK + Gradle)
	cd android && gradle :app:assembleDebug

.PHONY: android-build-go
android-build-go: gomobile-bind android-build ## Build gomobile AAR and Android debug APK

.PHONY: ux-report
ux-report: ## Generate UX PDF report from emulator-artifacts (requires npm ci + Playwright browsers)
	npm run ux:report -- --artifacts emulator-artifacts --out emulator-artifacts/ux-report

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

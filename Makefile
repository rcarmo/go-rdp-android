.DEFAULT_GOAL := help
GO ?= go
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

.PHONY: coverage
coverage: ## Run Go coverage
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

.PHONY: android-build
android-build: ## Build Android debug APK (requires Android SDK + Gradle)
	cd android && gradle :app:assembleDebug

.PHONY: clean
clean: ## Clean generated outputs
	$(GO) clean
	rm -f coverage.out coverage.html
	rm -rf bin android/.gradle android/build android/app/build

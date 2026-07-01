# ----------------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# ----------------------------------------------------------------------------

# Constants
VERSION_FILE=version.txt
VERSION=$(shell cat $(VERSION_FILE))
BINARY_NAME=thunderid
PRODUCT_NAME=ThunderID

export WITHOUT_CONSENT ?= false

# Tools
PROJECT_DIR := $(realpath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))/backend
PROJECT_BIN_DIR := $(PROJECT_DIR)/bin
TOOL_BIN ?= $(PROJECT_BIN_DIR)/tools
GOLANGCI_LINT ?= $(TOOL_BIN)/golangci-lint
MOCKERY ?= $(TOOL_BIN)/mockery
I18N_EXTRACTOR ?= $(TOOL_BIN)/i18n-extractor

# Tools versions
GOLANGCI_LINT_VERSION ?= v1.64.8
MOCKERY_VERSION ?= v3.5.5

$(TOOL_BIN):
	mkdir -p $(TOOL_BIN)

all: prepare clean build_with_coverage build

backend: prepare clean build_with_coverage build_backend

prepare:
	chmod +x build.sh

clean:
	./build.sh clean $(OS) $(ARCH)

build: build_frontend build_backend package_samples

build_backend:
	./build.sh build_backend $(OS) $(ARCH)

build_frontend:
	./build.sh build_frontend

build_docs:
	./build.sh build_docs

package_samples:
	./build.sh package_samples

test:
	./build.sh test $(OS) $(ARCH)

test_unit:
	./build.sh test_unit $(OS) $(ARCH)

test_integration:
	./build.sh test_integration "$(OS)" "$(ARCH)" "$(RUN)" "$(PACKAGE)"

build_with_coverage:
	@echo "================================================================"
	@echo "Building with coverage for unit and integration tests..."
	@echo "================================================================"
	./build.sh test_unit $(OS) $(ARCH)
	ENABLE_COVERAGE=true ./build.sh build_backend $(OS) $(ARCH)
	./build.sh build_frontend
	./build.sh test_integration $(OS) $(ARCH) "$(RUN)" "$(PACKAGE)"
	./build.sh merge_coverage $(OS) $(ARCH)
	@echo "================================================================"

build_with_coverage_only:
	@echo "================================================================"
	@echo "Building with coverage instrumentation (unit tests only)..."
	@echo "================================================================"
	./build.sh test_unit $(OS) $(ARCH)
	ENABLE_COVERAGE=true ./build.sh build_backend $(OS) $(ARCH)
	@echo "================================================================"

run:
	./build.sh run $(OS) $(ARCH)

run_backend:
	./build.sh run_backend $(OS) $(ARCH)

debug_backend:
	./build.sh debug_backend $(OS) $(ARCH)

run_frontend:
	./build.sh run_frontend $(OS) $(ARCH)

run_docs:
	./build.sh run_docs

docker-build:
	docker build -t $(BINARY_NAME):$(VERSION) .

docker-build-latest:
	docker build -t $(BINARY_NAME):latest .

docker-build-multiarch:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(BINARY_NAME):$(VERSION) .

docker-build-multiarch-latest:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(BINARY_NAME):latest .

docker-build-multiarch-push:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest --push .

lint: lint_backend lint_frontend

build_tools:
	./build.sh build_tools

test_tools:
	./build.sh test_tools

lint_tools:
	./build.sh lint_tools

lint_docs:
	@command -v vale >/dev/null 2>&1 || (echo "vale is not installed. See https://vale.sh/docs/vale-cli/installation/ for installation instructions." && exit 1)
	vale docs/

lint_backend: check_i18n golangci-lint
	cd backend && $(GOLANGCI_LINT) run ./...

lint_frontend:
	pnpm install --frozen-lockfile && pnpm build:frontend && pnpm lint

generate_i18n: install-i18n-extractor
	@echo "Extracting i18n messages from backend source code..."
	cd backend && $(I18N_EXTRACTOR) -source ./internal,./pkg/thunderidengine -output ./internal/system/i18n/core/defaults.go
	@echo "i18n defaults generated successfully"

check_i18n: install-i18n-extractor
	@echo "Checking i18n messages..."
	@cd backend && $(I18N_EXTRACTOR) -source ./internal,./pkg/thunderidengine -output ../defaults.check.go > /dev/null
	@diff -u backend/internal/system/i18n/core/defaults.go defaults.check.go > /dev/null || (echo "i18n generated file is out of sync. Please run 'make generate_i18n'" && rm defaults.check.go && exit 1)
	@rm defaults.check.go
	@echo "i18n messages are up to date"

mockery: install-mockery
	cd backend && $(MOCKERY) --config .mockery.public.yml
	cd backend && $(MOCKERY) --config .mockery.private.yml

verify_mocks: mockery
	@if [ -n "$$(git status --porcelain --untracked-files=all -- backend/tests/mocks ':(glob)backend/internal/**/*_mock_test.go')" ]; then \
		echo "Mock files have been regenerated and differ from what is committed."; \
		echo "Please review and commit the changes before pushing:"; \
		git status --porcelain --untracked-files=all -- backend/tests/mocks ':(glob)backend/internal/**/*_mock_test.go'; \
		exit 1; \
	fi
	@echo "All mock files are up to date."

format_check:
	pnpm install --frozen-lockfile
	pnpm format:check

test_frontend:
	pnpm install --frozen-lockfile && pnpm build:packages
	cd frontend/apps/console && pnpm test
	cd frontend/apps/gate && pnpm test

security_audit:
	cd frontend && pnpm audit --audit-level=high
	cd tests/e2e && npm audit --audit-level=high

test_e2e:
	chmod +x tests/e2e/run-e2e.sh
	tests/e2e/run-e2e.sh

pr_checks: verify_mocks lint format_check test_unit test_frontend test_integration build_backend build_frontend package_samples

help:
	@echo "Makefile targets:"
	@echo "  all                           - Clean, build, and test the project."
	@echo "  backend                       - Clean, build, and test only the backend."
	@echo "  clean                         - Remove build artifacts."
	@echo "  build                         - Build $(PRODUCT_NAME) (backend + frontend + samples)."
	@echo "  build_backend                 - Build the backend Go application."
	@echo "  build_frontend                - Build the frontend applications."
	@echo "  build_docs                    - Build the documentation."
	@echo "  package_samples               - Package sample applications."
	@echo "  test_unit                     - Run unit tests."
	@echo "  test_integration              - Run integration tests. Use RUN= for test filter, PACKAGE= for package filter."
	@echo "  build_with_coverage  		   - Build with coverage flags, run unit and integration tests, and generate combined coverage report."
	@echo "  build_with_coverage_only      - Build with coverage instrumentation (unit tests only, no integration tests)."
	@echo "  test                          - Run all tests (unit and integration)."
	@echo "  run                           - Build and run the $(PRODUCT_NAME) server locally."
	@echo "  run_backend                   - Build and run the $(PRODUCT_NAME) backend locally."
	@echo "  debug_backend                 - Build and run the $(PRODUCT_NAME) backend locally in debug mode."
	@echo "  run_frontend                  - Build and run the frontend applications locally."
	@echo "  run_docs                      - Run the documentation development server with live reload."
	@echo "  docker-build                  - Build single-arch Docker image with version tag."
	@echo "  docker-build-latest           - Build single-arch Docker image with latest tag."
	@echo "  docker-build-multiarch        - Build multi-arch Docker image with version tag."
	@echo "  docker-build-multiarch-latest - Build multi-arch Docker image with latest tag."
	@echo "  docker-build-multiarch-push   - Build and push multi-arch images to registry."
	@echo "  build_tools                   - Build all tool binaries (CLI + i18n-extractor + npm tools)."
	@echo "  test_tools                    - Run tests for all tools."
	@echo "  lint_tools                    - Run linting on all tools."
	@echo "  lint                          - Run linting on backend and frontend code."
	@echo "  lint_backend                  - Run golangci-lint on the backend code."
	@echo "  lint_frontend                 - Run ESLint on the frontend code."
	@echo "  lint_docs                     - Run Vale style linting on the documentation (requires vale)."
	@echo "  mockery                       - Generate mocks for unit tests using mockery."
	@echo "  verify_mocks                  - Regenerate and verify mock files are in sync with interfaces."
	@echo "  format_check                  - Check frontend code formatting with Prettier."
	@echo "  test_frontend                 - Run frontend unit tests (console + gate apps)."
	@echo "  security_audit                - Run dependency security audit on frontend and E2E deps (simplified; CI applies additional ignore rules)."
	@echo "  test_e2e                      - Start server, import declarative resources, start sample app, and run Playwright E2E tests."
	@echo "  pr_checks                     - Run all checks that CI performs on pull requests."
	@echo "  generate_i18n                 - Extract i18n messages and generate defaults.go."
	@echo "  help                          - Show this help message."

.PHONY: all prepare clean build build_backend build_frontend build_docs package_samples run
.PHONY: docker-build docker-build-latest docker-build-multiarch
.PHONY: docker-build-multiarch-latest docker-build-multiarch-push
.PHONY: test_unit test_integration build_with_coverage build_with_coverage_only test
.PHONY: help go_install_tool
.PHONY: lint lint_backend lint_frontend lint_docs golangci-lint mockery install-mockery
.PHONY: verify_mocks format_check test_frontend security_audit test_e2e pr_checks
.PHONY: run_backend debug_backend run_frontend run_docs

define go_install_tool
	cd /tmp && \
	GOBIN=$(TOOL_BIN) go install $(2)@$(3)
endef

golangci-lint: $(GOLANGCI_LINT)

$(GOLANGCI_LINT): $(TOOL_BIN)
	$(call go_install_tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

install-mockery: $(MOCKERY)

$(MOCKERY): $(TOOL_BIN)
	$(call go_install_tool,$(MOCKERY),github.com/vektra/mockery/v3,$(MOCKERY_VERSION))

install-i18n-extractor: $(I18N_EXTRACTOR)

$(I18N_EXTRACTOR): $(TOOL_BIN)
	@echo "Running unit tests for i18n-extractor..."
	cd tools/i18n-extractor && go test -v .
	@echo "Building i18n-extractor..."
	cd tools/i18n-extractor && go build -o $(I18N_EXTRACTOR) .

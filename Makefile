MAKEFLAGS    += --always-make --warn-undefined-variables
SHELL        := /usr/bin/env bash
.SHELLFLAGS  := -e -o pipefail -c
.NOTPARALLEL :

PROJECT_ROOT ?= $(shell git rev-parse --show-toplevel)
PROJECT_NAME ?= onyx

format: ## Run the code formatter
	pre-commit run --all-files golangci-lint-fmt

lint: ## Run the linters
	pre-commit run --all-files

test: ## Run all tests (`make test package=<pkg>` for a specific package)
	@set -o pipefail; \
		go tool courtney -t "-json" -v -o .coverage "$${package:-./...}" 2>/dev/null \
		| go tool gotestsum --raw-command --ignore-non-json-output-lines --format testname -- cat
	@sed -i.bak -e '/\.gen\.go:/d' .coverage
	@rm -f .coverage.bak
	@if grep -E '0$$' .coverage; then printf "\\e[31m[ERROR]\\e[0m   Missing test coverage\\n"; exit 1; fi;

gen: ## Run `go generate` on the whole project
	go generate ./...

mocks: ## Generate mocks for all interfaces
	@find . -type d -name mocks -print0 | xargs -0 --no-run-if-empty rm -rf
	@go tool mockery

cov.render: ## Show coverage stats in your browser
	go tool cover -html=.coverage

mod: ## Run `go mod tidy` to ensure your go.sum is up-to-date
	go mod tidy

update: ## Update all project dependencies
	# Update all the dependencies (`./...`), the test dependencies (`-t`),
	# the tools (`tool`), while preventing the toolchain from being updated (toolchain@...)
	go get -u -t toolchain@1.27.3 tool ./...
	go mod tidy

clean: ## Clean project with `go clean`
	go clean

# Imports
include ${PROJECT_ROOT}/.makefiles/Makefile.help
ifneq ("$(wildcard ${PROJECT_ROOT}/Dockerfile)", "")
  include ${PROJECT_ROOT}/.makefiles/Makefile.docker
endif

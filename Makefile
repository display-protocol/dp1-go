# Simple developer tasks for dp1-go (see AGENTS.md for full verification notes).

COVERAGE_THRESHOLD ?= 80

.PHONY: help format import lint test coverage verify validate

help:
	@echo "Targets:"
	@echo "  make format   - go fmt ./..."
	@echo "  make import   - golangci-lint fmt (goimports + local-prefixes; not run by golangci-lint run)"
	@echo "  make lint     - golangci-lint run"
	@echo "  make test     - go test ./... -race -count=1 (no coverage; use coverage for threshold)"
	@echo "  make coverage - scripts/check-coverage.sh (race + merged coverage; threshold: COVERAGE_THRESHOLD=$(COVERAGE_THRESHOLD))"
	@echo "  make verify   - import, lint, coverage (alias: validate)"

format:
	go fmt ./...

import:
	golangci-lint fmt

lint:
	golangci-lint run

test:
	go test ./... -race -count=1

coverage:
	bash scripts/check-coverage.sh "$(COVERAGE_THRESHOLD)"

verify validate: import lint coverage

.DEFAULT_GOAL := help

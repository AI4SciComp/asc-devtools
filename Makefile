# Copyright (C) 2026, Yi Cai.
# SPDX-License-Identifier: Apache-2.0
#
# Naming convention:
#   Targets: lowercase with hyphens, e.g. test-race
#   Variables: uppercase with underscores, e.g. BASE_BRANCH

.DEFAULT_GOAL := help
.ONESHELL:
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

# -----------------------------------------------------------------------------
# Project configuration
# -----------------------------------------------------------------------------
ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BINARY := $(ROOT_DIR)/asc
GO_FILES := $(shell git -C "$(ROOT_DIR)" ls-files --cached --others --exclude-standard -- '*.go')

GO ?= go
GIT ?= git
GH ?= gh
ASC ?= asc

VERSION ?= 0.1.0
REMOTE ?= origin
BASE_BRANCH ?= main
USER_PREFIX ?= $(HOME)/.local

MSG ?=
COMMIT_MSG ?= Updated at $(shell date +"%Y-%m-%d %H:%M:%S")
BRANCH ?=
USER_NAME ?=
USER_EMAIL ?=
DRAFT ?= 1
LIMIT ?= 10
RUN_ID ?=
REPOSITORIES ?=

export REMOTE BASE_BRANCH MSG COMMIT_MSG BRANCH USER_NAME USER_EMAIL
export DRAFT LIMIT RUN_ID REPOSITORIES

# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
.PHONY: help
help:
	@echo "asc-devtools utilities"
	@echo "======================"
	@echo
	@echo "Development:"
	@echo "  make fmt                    - Format tracked Go source"
	@echo "  make tidy                   - Normalize Go module metadata"
	@echo "  make test                   - Run unit and integration tests"
	@echo "  make test-race              - Run tests with the race detector"
	@echo "  make coverage               - Write coverage/coverage.out"
	@echo "  make build                  - Build ./asc"
	@echo "  make check                  - Run the complete local validation suite"
	@echo "  make clean                  - Remove generated local artifacts"
	@echo
	@echo "Installation:"
	@echo "  make install                - Build and install under /usr/local with sudo"
	@echo "  make install-user           - Build and install under $(USER_PREFIX)"
	@echo "  make uninstall              - Hash-guarded removal from /usr/local"
	@echo "  make uninstall-user         - Hash-guarded removal from $(USER_PREFIX)"
	@echo
	@echo "asc:"
	@echo "  make asc-doctor             - Run asc diagnostics"
	@echo "  make asc-list               - List organization repositories"
	@echo "  make asc-status [REPOSITORIES='asc-cpp ...']"
	@echo "  make asc-sync-dry-run [REPOSITORIES='asc-cpp ...']"
	@echo "  make asc-sync [REPOSITORIES='asc-cpp ...']"
	@echo
	@echo "Git and GitHub:"
	@echo "  make git-help               - Show Git shortcut help"
	@echo "  make gh-help                - Show GitHub CLI shortcut help"
	@echo "  make git-save MSG='message' - Stage, commit, and push the current branch"
	@echo "  make gh-pr-create           - Push and open a draft pull request"
	@echo "  make gh-pr-checks           - Show checks for the current pull request"

.PHONY: git-help
git-help:
	@echo "Git shortcuts"
	@echo "============="
	@echo
	@echo "Inspection:"
	@echo "  make git-status"
	@echo "  make git-log"
	@echo "  make git-diff"
	@echo "  make git-info"
	@echo "  make git-whoami"
	@echo
	@echo "Local changes:"
	@echo "  make git-add"
	@echo "  make git-commit [MSG='message']"
	@echo "  make git-ac [MSG='message']"
	@echo "  make git-save [MSG='message']"
	@echo
	@echo "Remote synchronization:"
	@echo "  make git-fetch"
	@echo "  make git-update             - Fetch and show new $(REMOTE)/$(BASE_BRANCH) commits"
	@echo "  make git-pull               - Fast-forward-only pull from the upstream"
	@echo "  make git-push               - Push, creating an upstream when needed"
	@echo "  make git-sync               - Fast-forward-only pull, then push"
	@echo
	@echo "Branches and identity:"
	@echo "  make git-branch-create BRANCH=feature/name"
	@echo "  make git-branch-switch BRANCH=main"
	@echo "  make git-login USER_NAME='name' USER_EMAIL='email'"

.PHONY: gh-help
gh-help:
	@echo "GitHub CLI shortcuts"
	@echo "===================="
	@echo
	@echo "Authentication and repository:"
	@echo "  make gh-auth-login"
	@echo "  make gh-auth-status"
	@echo "  make gh-repo-view"
	@echo
	@echo "Pull requests:"
	@echo "  make gh-pr-status"
	@echo "  make gh-pr-create [DRAFT=0] [BASE_BRANCH=main]"
	@echo "  make gh-pr-view"
	@echo "  make gh-pr-checks"
	@echo "  make gh-pr-checks-watch"
	@echo "  make gh-pr-merge            - Confirm, then squash-merge the current PR"
	@echo
	@echo "Actions:"
	@echo "  make gh-runs [LIMIT=10]"
	@echo "  make gh-run-watch RUN_ID=123456"

# -----------------------------------------------------------------------------
# Development and installation
# -----------------------------------------------------------------------------
.PHONY: fmt
fmt:
	@cd "$(ROOT_DIR)"
	gofmt -w $(GO_FILES)

.PHONY: fmt-check
fmt-check:
	@cd "$(ROOT_DIR)"
	unformatted="$$(gofmt -l $(GO_FILES))"
	if [[ -n "$$unformatted" ]]; then
		echo "ERROR: the following Go files need formatting:" >&2
		printf '%s\n' "$$unformatted" >&2
		exit 1
	fi

.PHONY: tidy
tidy:
	@cd "$(ROOT_DIR)"
	"$(GO)" mod tidy

.PHONY: tidy-check
tidy-check:
	@cd "$(ROOT_DIR)"
	"$(GO)" mod tidy -diff

.PHONY: vet
vet:
	@cd "$(ROOT_DIR)"
	"$(GO)" vet ./...

.PHONY: dependency-check
dependency-check:
	@cd "$(ROOT_DIR)"
	if [[ -e go.sum ]]; then
		echo "ERROR: go.sum exists; asc-devtools must remain standard-library-only." >&2
		exit 1
	fi
	if grep -q '^require' go.mod; then
		echo "ERROR: go.mod contains a dependency requirement." >&2
		exit 1
	fi

.PHONY: test
test:
	@cd "$(ROOT_DIR)"
	"$(GO)" test ./...

.PHONY: test-race
test-race:
	@cd "$(ROOT_DIR)"
	"$(GO)" test -race ./...

.PHONY: coverage
coverage:
	@cd "$(ROOT_DIR)"
	mkdir -p coverage
	"$(GO)" test -coverprofile=coverage/coverage.out ./...
	"$(GO)" tool cover -func=coverage/coverage.out

.PHONY: build
build:
	@cd "$(ROOT_DIR)"
	CGO_ENABLED=0 "$(GO)" build -buildvcs=false -trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" \
		-o "$(BINARY)" ./cmd/asc
	"$(BINARY)" --version

.PHONY: install-test
install-test: build
	@cd "$(ROOT_DIR)"
	./scripts/test_install.sh "$(BINARY)"

.PHONY: check
check:
	@$(MAKE) --no-print-directory fmt-check
	$(MAKE) --no-print-directory tidy-check
	$(MAKE) --no-print-directory dependency-check
	$(MAKE) --no-print-directory vet
	$(MAKE) --no-print-directory test
	$(MAKE) --no-print-directory test-race
	$(MAKE) --no-print-directory install-test
	git -C "$(ROOT_DIR)" diff --check

.PHONY: clean
clean:
	@rm -f -- "$(BINARY)"
	rm -rf -- "$(ROOT_DIR)/coverage"

.PHONY: install
install: build
	@cd "$(ROOT_DIR)"
	sudo ./scripts/install.sh --binary "$(BINARY)"

.PHONY: install-user
install-user: build
	@cd "$(ROOT_DIR)"
	./scripts/install.sh --binary "$(BINARY)" --prefix "$(USER_PREFIX)"

.PHONY: uninstall
uninstall:
	@cd "$(ROOT_DIR)"
	sudo ./scripts/uninstall.sh

.PHONY: uninstall-user
uninstall-user:
	@cd "$(ROOT_DIR)"
	./scripts/uninstall.sh --prefix "$(USER_PREFIX)"

# -----------------------------------------------------------------------------
# asc shortcuts
# -----------------------------------------------------------------------------
.PHONY: asc-doctor
asc-doctor:
	@"$(ASC)" doctor

.PHONY: asc-list
asc-list:
	@"$(ASC)" repo list

.PHONY: asc-status
asc-status:
	@if [[ -n "$$REPOSITORIES" ]]; then
		read -r -a repositories <<< "$$REPOSITORIES"
		"$(ASC)" repo status "$${repositories[@]}"
	else
		"$(ASC)" repo status
	fi

.PHONY: asc-sync-dry-run
asc-sync-dry-run:
	@if [[ -n "$$REPOSITORIES" ]]; then
		read -r -a repositories <<< "$$REPOSITORIES"
		"$(ASC)" repo sync "$${repositories[@]}" --dry-run
	else
		"$(ASC)" repo sync --dry-run
	fi

.PHONY: asc-sync
asc-sync:
	@if [[ -n "$$REPOSITORIES" ]]; then
		read -r -a repositories <<< "$$REPOSITORIES"
		"$(ASC)" repo sync "$${repositories[@]}"
	else
		"$(ASC)" repo sync
	fi

# -----------------------------------------------------------------------------
# Internal Git/GitHub checks
# -----------------------------------------------------------------------------
.PHONY: _check-repository
_check-repository:
	@"$(GIT)" -C "$(ROOT_DIR)" rev-parse --is-inside-work-tree >/dev/null

.PHONY: _check-current-branch
_check-current-branch: _check-repository
	@branch="$$("$(GIT)" -C "$(ROOT_DIR)" branch --show-current)"
	if [[ -z "$$branch" ]]; then
		echo "ERROR: detached HEAD has no current branch." >&2
		exit 1
	fi

.PHONY: _check-feature-branch
_check-feature-branch: _check-current-branch
	@branch="$$("$(GIT)" -C "$(ROOT_DIR)" branch --show-current)"
	if [[ "$$branch" == "$$BASE_BRANCH" ]]; then
		echo "ERROR: create or switch to a feature branch first." >&2
		exit 1
	fi

.PHONY: _check-clean
_check-clean: _check-repository
	@if [[ -n "$$("$(GIT)" -C "$(ROOT_DIR)" status --porcelain)" ]]; then
		echo "ERROR: working tree is dirty; review, commit, or stash it first." >&2
		"$(GIT)" -C "$(ROOT_DIR)" status --short
		exit 1
	fi

.PHONY: _check-upstream
_check-upstream: _check-current-branch
	@if ! "$(GIT)" -C "$(ROOT_DIR)" rev-parse --verify '@{upstream}' >/dev/null 2>&1; then
		echo "ERROR: current branch has no upstream; run 'make git-push' first." >&2
		exit 1
	fi

.PHONY: _check-gh
_check-gh:
	@command -v "$(GH)" >/dev/null 2>&1 || {
		echo "ERROR: GitHub CLI ($(GH)) is not available in PATH." >&2
		exit 1
	}

# -----------------------------------------------------------------------------
# Git shortcuts
# -----------------------------------------------------------------------------
.PHONY: git-status
git-status: _check-repository
	@"$(GIT)" -C "$(ROOT_DIR)" status

.PHONY: git-log
git-log: _check-repository
	@"$(GIT)" -C "$(ROOT_DIR)" log --oneline --graph --decorate --all -10

.PHONY: git-diff
git-diff: _check-repository
	@echo "==> Unstaged changes"
	"$(GIT)" -C "$(ROOT_DIR)" diff
	echo
	echo "==> Staged changes"
	"$(GIT)" -C "$(ROOT_DIR)" diff --cached

.PHONY: git-info
git-info: _check-repository
	@branch="$$("$(GIT)" -C "$(ROOT_DIR)" branch --show-current)"
	upstream="$$("$(GIT)" -C "$(ROOT_DIR)" rev-parse --abbrev-ref '@{upstream}' 2>/dev/null || true)"
	echo "Repository:     $(ROOT_DIR)"
	echo "Current branch: $${branch:-<detached>}"
	echo "Remote URL:     $$("$(GIT)" -C "$(ROOT_DIR)" remote get-url "$$REMOTE")"
	echo "Upstream:       $${upstream:-<not set>}"
	echo "Latest commit:  $$("$(GIT)" -C "$(ROOT_DIR)" log -1 --oneline)"

.PHONY: git-whoami
git-whoami: _check-repository
	@echo "Local repository identity:"
	echo "  Name:  $$("$(GIT)" -C "$(ROOT_DIR)" config user.name 2>/dev/null || echo '<not set>')"
	echo "  Email: $$("$(GIT)" -C "$(ROOT_DIR)" config user.email 2>/dev/null || echo '<not set>')"
	echo
	echo "Global identity:"
	echo "  Name:  $$("$(GIT)" config --global user.name 2>/dev/null || echo '<not set>')"
	echo "  Email: $$("$(GIT)" config --global user.email 2>/dev/null || echo '<not set>')"

.PHONY: git-login
git-login: _check-repository
	@if [[ -z "$$USER_NAME" || -z "$$USER_EMAIL" ]]; then
		echo "ERROR: pass USER_NAME='name' and USER_EMAIL='email'." >&2
		exit 1
	fi
	"$(GIT)" -C "$(ROOT_DIR)" config user.name "$$USER_NAME"
	"$(GIT)" -C "$(ROOT_DIR)" config user.email "$$USER_EMAIL"
	$(MAKE) --no-print-directory git-whoami

.PHONY: git-add
git-add: _check-repository
	@echo "==> Staging all changes"
	"$(GIT)" -C "$(ROOT_DIR)" add -A
	"$(GIT)" -C "$(ROOT_DIR)" status --short

.PHONY: git-commit
git-commit: _check-repository
	@if "$(GIT)" -C "$(ROOT_DIR)" diff --cached --quiet; then
		echo "==> Nothing staged to commit."
	else
		message="$${MSG:-$${COMMIT_MSG}}"
		echo "==> Committing staged changes"
		"$(GIT)" -C "$(ROOT_DIR)" commit -m "$$message"
		"$(GIT)" -C "$(ROOT_DIR)" log -1 --oneline
	fi

.PHONY: git-ac
git-ac:
	@$(MAKE) --no-print-directory git-add
	$(MAKE) --no-print-directory git-commit

.PHONY: git-fetch
git-fetch: _check-repository
	@echo "==> Fetching and pruning $$REMOTE"
	"$(GIT)" -C "$(ROOT_DIR)" fetch --prune "$$REMOTE"

.PHONY: git-update
git-update: git-fetch
	@reference="$$REMOTE/$$BASE_BRANCH"
	if ! "$(GIT)" -C "$(ROOT_DIR)" rev-parse --verify "$$reference" >/dev/null 2>&1; then
		echo "ERROR: remote branch $$reference does not exist." >&2
		exit 1
	fi
	echo "==> Commits in $$reference that are not in HEAD"
	"$(GIT)" -C "$(ROOT_DIR)" log --oneline --graph --decorate "HEAD..$$reference"

.PHONY: git-pull
git-pull: _check-clean _check-upstream
	@echo "==> Pulling the current upstream with fast-forward only"
	"$(GIT)" -C "$(ROOT_DIR)" pull --ff-only

.PHONY: git-push
git-push: _check-current-branch
	@branch="$$("$(GIT)" -C "$(ROOT_DIR)" branch --show-current)"
	if "$(GIT)" -C "$(ROOT_DIR)" rev-parse --verify '@{upstream}' >/dev/null 2>&1; then
		echo "==> Pushing $$branch to its configured upstream"
		"$(GIT)" -C "$(ROOT_DIR)" push
	else
		echo "==> Publishing $$branch to $$REMOTE and setting its upstream"
		"$(GIT)" -C "$(ROOT_DIR)" push -u "$$REMOTE" "$$branch"
	fi

.PHONY: git-sync
git-sync: _check-clean _check-upstream
	@$(MAKE) --no-print-directory git-pull
	$(MAKE) --no-print-directory git-push

.PHONY: git-save
git-save:
	@$(MAKE) --no-print-directory git-add
	$(MAKE) --no-print-directory git-commit
	$(MAKE) --no-print-directory git-push

.PHONY: git-branch-create
git-branch-create: _check-repository
	@if [[ -z "$$BRANCH" ]]; then
		echo "ERROR: pass BRANCH=feature/name." >&2
		exit 1
	fi
	"$(GIT)" check-ref-format --branch "$$BRANCH" >/dev/null
	"$(GIT)" -C "$(ROOT_DIR)" switch -c "$$BRANCH"

.PHONY: git-branch-switch
git-branch-switch: _check-repository
	@if [[ -z "$$BRANCH" ]]; then
		echo "ERROR: pass BRANCH=main." >&2
		exit 1
	fi
	"$(GIT)" check-ref-format --branch "$$BRANCH" >/dev/null
	"$(GIT)" -C "$(ROOT_DIR)" switch "$$BRANCH"

# -----------------------------------------------------------------------------
# GitHub CLI shortcuts
# -----------------------------------------------------------------------------
.PHONY: gh-auth-login
gh-auth-login: _check-gh
	@"$(GH)" auth login --hostname github.com --git-protocol ssh --web

.PHONY: gh-auth-status
gh-auth-status: _check-gh
	@"$(GH)" auth status --hostname github.com

.PHONY: gh-repo-view
gh-repo-view: _check-gh
	@"$(GH)" repo view

.PHONY: gh-pr-status
gh-pr-status: _check-gh
	@"$(GH)" pr status

.PHONY: gh-pr-create
gh-pr-create: _check-gh _check-clean _check-feature-branch
	@branch="$$("$(GIT)" -C "$(ROOT_DIR)" branch --show-current)"
	if "$(GIT)" -C "$(ROOT_DIR)" rev-parse --verify '@{upstream}' >/dev/null 2>&1; then
		"$(GIT)" -C "$(ROOT_DIR)" push
	else
		"$(GIT)" -C "$(ROOT_DIR)" push -u "$$REMOTE" "$$branch"
	fi
	args=(pr create --fill --base "$$BASE_BRANCH")
	if [[ "$$DRAFT" != "0" ]]; then
		args+=(--draft)
	fi
	"$(GH)" "$${args[@]}"

.PHONY: gh-pr-view
gh-pr-view: _check-gh
	@"$(GH)" pr view

.PHONY: gh-pr-checks
gh-pr-checks: _check-gh
	@"$(GH)" pr checks

.PHONY: gh-pr-checks-watch
gh-pr-checks-watch: _check-gh
	@"$(GH)" pr checks --watch

.PHONY: gh-pr-merge
gh-pr-merge: _check-gh
	@"$(GH)" pr view
	if read -r -p "Squash-merge this pull request? [y/N] " confirmation &&
		[[ "$$confirmation" =~ ^[Yy]$$ ]]; then
		"$(GH)" pr merge --squash
	else
		echo "==> Merge cancelled."
	fi

.PHONY: gh-runs
gh-runs: _check-gh
	@if [[ ! "$$LIMIT" =~ ^[1-9][0-9]*$$ ]]; then
		echo "ERROR: LIMIT must be a positive integer." >&2
		exit 1
	fi
	"$(GH)" run list --limit "$$LIMIT"

.PHONY: gh-run-watch
gh-run-watch: _check-gh
	@if [[ ! "$$RUN_ID" =~ ^[1-9][0-9]*$$ ]]; then
		echo "ERROR: pass a numeric RUN_ID." >&2
		exit 1
	fi
	"$(GH)" run watch "$$RUN_ID" --exit-status

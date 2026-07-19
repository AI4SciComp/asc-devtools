.ONESHELL:
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

REMOTE ?= origin
MSG ?=
WORKFLOW := ./scripts/github.sh
export ASC_DEVTOOLS_REMOTE := $(REMOTE)
export ASC_DEVTOOLS_MESSAGE := $(MSG)

.DEFAULT_GOAL := help

.PHONY: help
help:
	@printf '%s\n' \
		'asc-devtools GitHub workflow' \
		'============================' \
		'' \
		'  make github-status                 Fetch and show branch/parity status' \
		'  make github-import                 Import standalone branches into main' \
		'  make github-check                  Verify published branch parity' \
		'  make github-publish [MSG="..."]     Atomically publish main and standalone branches' \
		'  make github-fetch                  Fetch main, go, python, and shell' \
		'  make workflow-test                 Run the offline Git workflow tests' \
		'' \
		'Variables:' \
		'  REMOTE=origin                      Git remote to use' \
		'  MSG="Updated at ..."               Optional one-line commit message'

.PHONY: github-fetch
github-fetch:
	@"$(WORKFLOW)" fetch

.PHONY: github-status
github-status:
	@"$(WORKFLOW)" status

.PHONY: github-import
github-import:
	@"$(WORKFLOW)" import

.PHONY: github-check
github-check:
	@"$(WORKFLOW)" check

.PHONY: github-publish github-save save
github-publish github-save save:
	@"$(WORKFLOW)" publish

.PHONY: workflow-test
workflow-test:
	@./scripts/test_github_workflow.sh

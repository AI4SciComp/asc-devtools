# Contributing

The active implementation requires Go 1.25 or newer and uses only the standard
library. Do not add third-party module requirements or move application logic
into shell.

Keep the canonical v0.1 surface limited to doctor, workspace, repository
list/clone/status/sync/save, top-level configure/build/test, and Bash completion.
Do not add commands that reset, clean, stash, check out, rebase, delete branches,
publish releases, or open pull requests. Commit, push, and force behavior must
remain isolated to the explicit, reviewed `repo save` workflow.

Run:

```bash
gofmt -w ./cmd ./internal
go mod tidy
test ! -e go.sum
! grep -q '^require' go.mod
go vet ./...
go test ./...
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o /tmp/asc ./cmd/asc
/tmp/asc --help
/tmp/asc --version
scripts/test_install.sh /tmp/asc
git diff --check
```

Tests must remain offline and independent of live GitHub, user tokens, SSH keys,
home directories, global Git configuration, locale, and mutable sibling
repositories. Use `httptest`, fake runners, `t.TempDir`, and repository-local
Git identities.

Preserve argument-slice process execution, direct-child containment, token
redaction, bounded HTTP reads and pagination, clone non-overwrite behavior, and
fast-forward-only sync. Preserve save's pre-commit fetch check and require an
explicit `--force` for remote history replacement. Update tests and user
documentation with every behavior change.

Contributions are licensed under Apache-2.0.

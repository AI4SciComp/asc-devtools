# Contributing

The active implementation requires Go 1.25+ and uses only the standard library.
Do not add third-party module requirements or move business logic into shell.

Run all checks:

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o /tmp/asc ./cmd/asc
/tmp/asc --help
/tmp/asc --version
scripts/test_install.sh /tmp/asc
go mod tidy
```

Tests must remain offline and independent of live GitHub, user tokens, SSH keys,
home directories, global Git configuration, and the clock. Use `httptest`, fake
runners, `t.TempDir`, and repository-local Git identities.

Preserve argument-slice process execution, direct-child containment, token
redaction, bounded HTTP reads, and fast-forward-only sync. Do not introduce
commands that discard work, rewrite history, commit, push, or publish.

Update tests and command documentation with behavior changes. Contributions are
licensed under Apache-2.0.

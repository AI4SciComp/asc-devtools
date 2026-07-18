# Installation

## Release binary

A release binary does not require Go:

```bash
install -Dm755 ./asc "${HOME}/.local/bin/asc"
```

Add the user binary directory to PATH once:

```bash
export PATH="${HOME}/.local/bin:${PATH}"
```

## Build on Ubuntu/WSL2

Go 1.25 or newer is required to build:

```bash
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=0.1.0" \
  -o ./dist/asc ./cmd/asc
install -Dm755 ./dist/asc "${HOME}/.local/bin/asc"
```

For development:

```bash
go install ./cmd/asc
```

The local installer can build or copy an already-built binary:

```bash
./scripts/install.sh
./scripts/install.sh --binary ./dist/asc --prefix "${HOME}/.local"
```

It performs no download, sudo operation, telemetry, or shell-configuration edit.
Upgrade by installing a newly built or downloaded binary over the prior asc
binary.

## Completion

```bash
mkdir -p "${HOME}/.local/share/bash-completion/completions"
asc completion bash >"${HOME}/.local/share/bash-completion/completions/asc"
```

For the current shell only:

```bash
source <(asc completion bash)
```

## Verification

```bash
asc --version
asc --help
asc doctor
```

API discovery works publicly without GitHub CLI. Configure a token for private
repositories and separately configure Git SSH or HTTPS credentials for cloning.

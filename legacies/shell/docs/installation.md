# Installation

## Direct checkout use

```bash
./bin/asc --version
./bin/asc --help
```

The executable resolves `lib/asc` relative to the checkout, including when the
executable is reached through a symbolic link.

## User installation

```bash
./install.sh
export PATH="${HOME}/.local/bin:${PATH}"
```

Files are copied to:

```text
~/.local/bin/asc
~/.local/lib/asc/*.sh
~/.local/share/bash-completion/completions/asc
~/.local/share/asc-devtools/install-manifest
```

Use `./install.sh --prefix /absolute/path` for another prefix. Re-running the
installer updates marked asc files. It performs a preflight and refuses any
unrelated existing target before copying.

## Bash completion

With a standard bash-completion installation, the installed completion path is
loaded automatically. Otherwise:

```bash
source <(asc completion bash)
```

Completion reads only local configuration and workspace names; it does not query
GitHub on Tab.

## Uninstallation

```bash
./uninstall.sh
./uninstall.sh --prefix /absolute/path
```

Only the explicitly listed, marked files are removed. Unrelated files are
reported and retained. Empty asc-specific directories are removed with `rmdir`;
the installation prefix itself is never recursively deleted.

## Authentication

```bash
gh auth login
gh auth status
ssh -T git@github.com
asc doctor
```

GitHub CLI uses its stored API token to query `AI4SciComp`. SSH cloning uses the
personal account's SSH key. A successful GitHub SSH greeting commonly returns a
nonzero status because GitHub provides no interactive shell; `asc doctor` checks
the greeting text.


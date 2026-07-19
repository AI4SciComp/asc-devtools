# Contributing

The implementation targets Bash 4.4+ and must remain free of Python, jq,
third-party runtime libraries, and compiled helpers.

Run the complete local checks:

```bash
bash -n bin/asc lib/asc/*.sh completions/asc.bash scripts/*.sh tests/*.sh
shellcheck bin/asc lib/asc/*.sh completions/asc.bash scripts/*.sh tests/*.sh
shfmt -d -i 2 -ci bin/asc lib/asc/*.sh completions/asc.bash scripts/*.sh tests/*.sh
./tests/run_tests.sh
./scripts/package_update.sh --output /tmp/asc-shell-release-test
```

Use arrays for commands, quote expansions, and keep repository input as
validated names. Tests must remain offline and isolated from the user's home,
Git configuration, live organization, credentials, and workspace. Do not add
operations that discard work, rewrite history, automatically commit, or push.
Commit and push are allowed only inside the reviewed `repo save` transaction.
Vendor tests use only synthetic temporary asc-cmake sources and must never
inspect or modify a real sibling checkout.
Self-update tests use only mocked release downloads and temporary managed
prefixes.

Update command documentation and tests with behavioral changes. Contributions
are licensed under Apache-2.0.

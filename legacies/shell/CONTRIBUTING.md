# Contributing

The active implementation is Bash 4.4+ and must remain free of Python, jq,
third-party runtime libraries, and compiled helpers.

Run the complete local checks:

```bash
bash -n bin/asc lib/asc/*.sh completions/asc.bash tests/*.sh install.sh uninstall.sh
shellcheck bin/asc lib/asc/*.sh completions/asc.bash tests/*.sh install.sh uninstall.sh
shfmt -d -i 2 -ci bin/asc lib/asc/*.sh completions/asc.bash tests/*.sh install.sh uninstall.sh
./tests/run_tests.sh
```

Use arrays for commands, quote expansions, keep repository input as validated
names, and maintain strict mode in executable/runtime modules. Completion must
not alter options in the interactive shell.

Tests must remain offline and isolated from the user's home, global Git
configuration, live organization, and real workspace. Add executable mocks or
temporary repositories with repository-local identities.

Do not add operations that discard work, rewrite history, automatically commit,
or push. Update command documentation and tests with behavioral changes.
Contributions are licensed under Apache-2.0.


// Package completion embeds shell completion definitions.
package completion

import _ "embed"

// Bash is the Bash completion definition printed by `asc completion bash`.
//
//go:embed asc.bash
var Bash string

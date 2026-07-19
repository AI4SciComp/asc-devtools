# ruff: noqa: E501
"""Static Bash completion shared with the other implementation branches."""

BASH_COMPLETION = r"""_asc_completion() {
  local current previous commands repo_commands choices workspace
  COMPREPLY=()
  current="${COMP_WORDS[COMP_CWORD]}"
  previous="${COMP_WORDS[COMP_CWORD-1]}"
  commands="doctor workspace repo configure build test completion"
  repo_commands="list clone status sync"
  choices="${commands}"
  if [[ "${COMP_WORDS[1]:-}" == "repo" ]]; then
    choices="${repo_commands}"
    if ((COMP_CWORD > 2)); then
      workspace=$(asc workspace 2>/dev/null) || workspace=""
      if [[ -n "${workspace}" && -d "${workspace}" ]]; then
        choices="$(command find "${workspace}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null)"
      fi
    fi
  elif [[ "${COMP_WORDS[1]:-}" == "configure" || "${COMP_WORDS[1]:-}" == "build" || "${COMP_WORDS[1]:-}" == "test" ]]; then
    choices="dev debug release --preset"
  elif [[ "${COMP_WORDS[1]:-}" == "completion" ]]; then
    choices="bash"
  elif [[ "${previous}" == "--protocol" ]]; then
    choices="ssh https"
  fi
  COMPREPLY=($(compgen -W "${choices}" -- "${current}"))
}
complete -F _asc_completion asc
"""

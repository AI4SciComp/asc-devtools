#!/usr/bin/env bash
# asc-devtools managed file

_asc_completion_restore_errexit=false
_asc_completion_restore_nounset=false
_asc_completion_restore_pipefail=false
if [[ $- != *e* ]]; then _asc_completion_restore_errexit=true; fi
if [[ $- != *u* ]]; then _asc_completion_restore_nounset=true; fi
if ! shopt -qo pipefail; then _asc_completion_restore_pipefail=true; fi
set -o errexit
set -o nounset
set -o pipefail

_asc_completion() {
  local current="${COMP_WORDS[COMP_CWORD]:-}"
  local previous="${COMP_WORDS[COMP_CWORD - 1]:-}"
  local top_command="${COMP_WORDS[1]:-}"
  local repo_command="${COMP_WORDS[2]:-}"
  local workspace=""
  local repository_path
  local -a candidates=()
  COMPREPLY=()

  if ((COMP_CWORD == 1)); then
    candidates=(doctor workspace repo configure build test completion --help --version --config --organization --workspace --repository-prefix --clone-protocol --remote --include-dot-github --no-include-dot-github)
    mapfile -t COMPREPLY < <(compgen -W "${candidates[*]}" -- "${current}")
    return 0
  fi
  if [[ "${top_command}" == "repo" && ${COMP_CWORD} -eq 2 ]]; then
    mapfile -t COMPREPLY < <(compgen -W "list clone status sync" -- "${current}")
    return 0
  fi
  case "${previous}" in
    --clone-protocol)
      mapfile -t COMPREPLY < <(compgen -W "ssh https" -- "${current}")
      return 0
      ;;
    --preset)
      mapfile -t COMPREPLY < <(compgen -W "dev release" -- "${current}")
      return 0
      ;;
  esac
  if [[ "${top_command}" == "completion" ]]; then
    mapfile -t COMPREPLY < <(compgen -W "bash" -- "${current}")
    return 0
  fi

  if [[ "${top_command}" == "configure" || "${top_command}" == "build" ||
    "${top_command}" == "test" ||
    ("${top_command}" == "repo" && "${repo_command}" != "list") ]]; then
    workspace=$(asc workspace 2>/dev/null) || return 0
    [[ -d "${workspace}" ]] || return 0
    shopt -s nullglob
    for repository_path in "${workspace}"/asc-* "${workspace}"/.github; do
      [[ -e "${repository_path}/.git" ]] || continue
      candidates+=("${repository_path##*/}")
    done
    shopt -u nullglob
    mapfile -t COMPREPLY < <(compgen -W "${candidates[*]}" -- "${current}")
  fi
}

complete -F _asc_completion asc

if [[ "${_asc_completion_restore_errexit}" == "true" ]]; then set +o errexit; fi
if [[ "${_asc_completion_restore_nounset}" == "true" ]]; then set +o nounset; fi
if [[ "${_asc_completion_restore_pipefail}" == "true" ]]; then set +o pipefail; fi
unset _asc_completion_restore_errexit
unset _asc_completion_restore_nounset
unset _asc_completion_restore_pipefail

# Bash completion for asc. This function performs no network operations.
_asc_completion() {
  local current="${COMP_WORDS[COMP_CWORD]}"
  local previous="${COMP_WORDS[COMP_CWORD-1]}"
  local command="${COMP_WORDS[1]}"
  local repo_command="${COMP_WORDS[2]}"
  local choices=""
  COMPREPLY=()

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    choices="doctor workspace repo configure build test completion --help --version --config --organization --workspace --no-color"
  elif [[ ${command} == repo && ${COMP_CWORD} -eq 2 ]]; then
    choices="list clone status sync"
  elif [[ ${previous} == --protocol ]]; then
    choices="ssh https"
  elif [[ ${previous} == --preset ]]; then
    choices="dev release"
  elif [[ ${command} == completion ]]; then
    choices="bash"
  elif [[ ${command} == configure || ${command} == build || ${command} == test || (${command} == repo && ${repo_command} != list) ]]; then
    local workspace
    workspace=$(asc workspace 2>/dev/null) || return 0
    [[ -d ${workspace} ]] || return 0
    local repository
    for repository in "${workspace}"/asc-* "${workspace}"/.github; do
      [[ -e ${repository}/.git ]] && choices+=" ${repository##*/}"
    done
  fi
  COMPREPLY=($(compgen -W "${choices}" -- "${current}"))
}
complete -F _asc_completion asc

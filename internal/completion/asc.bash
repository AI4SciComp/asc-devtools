# Bash completion for asc. This function performs no network operations.
_asc_completion() {
  local current="${COMP_WORDS[COMP_CWORD]}"
  local previous="${COMP_WORDS[COMP_CWORD-1]}"
  local command="${COMP_WORDS[1]}"
  local repo_command="${COMP_WORDS[2]}"
  local choices=""
  COMPREPLY=()

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    choices="doctor workspace agent workflow repo cmake configure build test update completion --help --version --config --organization --workspace --no-color"
	elif [[ ${command} == workspace && ${COMP_CWORD} -eq 2 ]]; then
		choices="init validate"
	elif [[ ${command} == agent && ${COMP_CWORD} -eq 2 ]]; then
		choices="init"
	elif [[ ${command} == workflow && ${COMP_CWORD} -eq 2 ]]; then
		choices="validate"
	elif [[ ${command} == repo && ${COMP_CWORD} -eq 2 ]]; then
		choices="list clone status sync save"
	elif [[ ${command} == cmake && ${COMP_CWORD} -eq 2 ]]; then
		choices="configure build test workflow presets vendor"
	elif [[ ${command} == cmake && ${repo_command} == vendor && ${COMP_CWORD} -eq 3 ]]; then
		choices="status plan apply"
	elif [[ ${previous} == --protocol ]]; then
		choices="ssh https"
	elif [[ ${previous} == --preset || ${previous} == --configure-preset || ${previous} == --build-preset || ${previous} == --test-preset ]]; then
		choices="dev release"
  elif [[ ${command} == completion ]]; then
    choices="bash"
	elif [[ ${command} == configure || ${command} == build || ${command} == test || ${command} == cmake || (${command} == repo && ${repo_command} != list) ]]; then
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

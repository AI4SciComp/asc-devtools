# Bash completion for asc.
_asc_completion() {
    local current previous command workspace
    COMPREPLY=()
    current="${COMP_WORDS[COMP_CWORD]}"
    previous="${COMP_WORDS[COMP_CWORD-1]}"
    command="${COMP_WORDS[1]}"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "doctor workspace repo configure build test --help --version --config" -- "${current}"))
        return
    fi
    if [[ ${command} == repo && ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=($(compgen -W "list clone status sync" -- "${current}"))
        return
    fi
    if [[ ${previous} == --clone-protocol ]]; then
        COMPREPLY=($(compgen -W "ssh https" -- "${current}"))
        return
    fi
    if [[ ${previous} == --preset ]]; then
        workspace=$(asc workspace 2>/dev/null) || return
        local repository="${COMP_WORDS[2]}"
        local preset_file="${workspace}/${repository}/CMakePresets.json"
        [[ -r ${preset_file} ]] || return
        COMPREPLY=($(compgen -W "$(sed -n 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "${preset_file}")" -- "${current}"))
        return
    fi
    if [[ ${command} == configure || ${command} == build || ${command} == test || ${command} == repo ]]; then
        workspace=$(asc workspace 2>/dev/null) || return
        [[ -d ${workspace} ]] || return
        local names=""
        local path
        for path in "${workspace}"/asc-* "${workspace}"/.github; do
            [[ -e ${path}/.git ]] && names+=" ${path##*/}"
        done
        COMPREPLY=($(compgen -W "${names}" -- "${current}"))
    fi
}
complete -F _asc_completion asc


#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

install_error() {
  printf 'install.sh: error: %s\n' "$*" >&2
}

install_main() {
  local source_root
  local prefix="${HOME}/.local"
  local target
  local source_file
  local relative
  local -a relative_files=(
    bin/asc
    lib/asc/common.sh
    lib/asc/config.sh
    lib/asc/process.sh
    lib/asc/github.sh
    lib/asc/repository.sh
    lib/asc/doctor.sh
    lib/asc/cmake.sh
  )
  source_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

  while (($# > 0)); do
    case "$1" in
      --prefix)
        if (($# < 2)); then
          install_error "--prefix requires a value"
          return 2
        fi
        prefix="$2"
        shift 2
        ;;
      -h | --help)
        printf 'Usage: ./install.sh [--prefix PATH]\n'
        return 0
        ;;
      *)
        install_error "unknown option: $1"
        return 2
        ;;
    esac
  done
  if [[ "${prefix}" != /* || "${prefix}" == "/" ]]; then
    install_error "prefix must be an absolute path other than /"
    return 1
  fi
  prefix="${prefix%/}"

  local completion_target="${prefix}/share/bash-completion/completions/asc"
  local manifest_target="${prefix}/share/asc-devtools/install-manifest"
  local -a targets=("${completion_target}" "${manifest_target}")
  for relative in "${relative_files[@]}"; do targets+=("${prefix}/${relative}"); done
  for target in "${targets[@]}"; do
    if [[ -e "${target}" ]] && ! grep -Fq '# asc-devtools managed file' "${target}" 2>/dev/null; then
      install_error "refusing to overwrite unrelated file: ${target}"
      return 1
    fi
  done

  mkdir -p -- \
    "${prefix}/bin" "${prefix}/lib/asc" \
    "${prefix}/share/bash-completion/completions" \
    "${prefix}/share/asc-devtools"
  for relative in "${relative_files[@]}"; do
    source_file="${source_root}/${relative}"
    target="${prefix}/${relative}"
    if [[ "${relative}" == "bin/asc" ]]; then
      install -m 0755 -- "${source_file}" "${target}"
    else
      install -m 0644 -- "${source_file}" "${target}"
    fi
    printf 'Installed %s\n' "${target}"
  done
  install -m 0644 -- "${source_root}/completions/asc.bash" "${completion_target}"
  printf 'Installed %s\n' "${completion_target}"
  {
    printf '# asc-devtools managed file\n'
    printf '%s\n' "${prefix}/bin/asc"
    for relative in "${relative_files[@]:1}"; do printf '%s/%s\n' "${prefix}" "${relative}"; done
    printf '%s\n' "${completion_target}"
    printf '%s\n' "${manifest_target}"
  } >"${manifest_target}"
  chmod 0644 -- "${manifest_target}"
  printf 'Installed %s\n' "${manifest_target}"
  printf 'Ensure %s/bin is on PATH.\n' "${prefix}"
}

install_main "$@"

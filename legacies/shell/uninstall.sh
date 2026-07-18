#!/usr/bin/env bash
# asc-devtools managed file

set -o errexit
set -o nounset
set -o pipefail

uninstall_error() {
  printf 'uninstall.sh: error: %s\n' "$*" >&2
}

uninstall_main() {
  local prefix="${HOME}/.local"
  local target
  local failures=0
  local -a targets=()
  while (($# > 0)); do
    case "$1" in
      --prefix)
        if (($# < 2)); then
          uninstall_error "--prefix requires a value"
          return 2
        fi
        prefix="$2"
        shift 2
        ;;
      -h | --help)
        printf 'Usage: ./uninstall.sh [--prefix PATH]\n'
        return 0
        ;;
      *)
        uninstall_error "unknown option: $1"
        return 2
        ;;
    esac
  done
  if [[ "${prefix}" != /* || "${prefix}" == "/" ]]; then
    uninstall_error "prefix must be an absolute path other than /"
    return 1
  fi
  prefix="${prefix%/}"
  targets=(
    "${prefix}/bin/asc"
    "${prefix}/lib/asc/common.sh"
    "${prefix}/lib/asc/config.sh"
    "${prefix}/lib/asc/process.sh"
    "${prefix}/lib/asc/github.sh"
    "${prefix}/lib/asc/repository.sh"
    "${prefix}/lib/asc/doctor.sh"
    "${prefix}/lib/asc/cmake.sh"
    "${prefix}/share/bash-completion/completions/asc"
    "${prefix}/share/asc-devtools/install-manifest"
  )
  for target in "${targets[@]}"; do
    [[ -e "${target}" ]] || continue
    if ! grep -Fq '# asc-devtools managed file' "${target}" 2>/dev/null; then
      printf 'WARN Refusing to remove unrelated file: %s\n' "${target}" >&2
      ((failures += 1))
      continue
    fi
    rm -- "${target}"
    printf 'Removed %s\n' "${target}"
  done
  rmdir -- \
    "${prefix}/lib/asc" \
    "${prefix}/share/asc-devtools" \
    "${prefix}/share/bash-completion/completions" \
    "${prefix}/share/bash-completion" \
    2>/dev/null || true
  ((failures == 0))
}

uninstall_main "$@"

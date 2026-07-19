#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

readonly remote="${ASC_DEVTOOLS_REMOTE:-origin}"
readonly -a implementations=(go python shell)
repository_root=""
temporary_root=""
prepared_commit_result=""
declare -a temporary_worktrees=()

usage() {
  cat <<'EOF'
Usage: ./scripts/github.sh {fetch|status|import|check|publish}

Manage the asc-devtools main/go/python/shell branch relationship with Git.
Set ASC_DEVTOOLS_REMOTE to select a remote and ASC_DEVTOOLS_MESSAGE to override
the default "Updated at YYYY-MM-DD HH:MM:SS" publish message.
EOF
}

fail() {
  printf 'github.sh: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  local worktree
  set +o errexit
  for worktree in "${temporary_worktrees[@]}"; do
    git -C "${repository_root}" worktree remove --force "${worktree}" >/dev/null 2>&1
  done
  if [[ -n "${temporary_root}" &&
    "${temporary_root}" == "${TMPDIR:-/tmp}"/asc-devtools-github.* &&
    -d "${temporary_root}" ]]; then
    rm -rf -- "${temporary_root}"
  fi
}
trap cleanup EXIT

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

initialize() {
  require_command git
  require_command rsync
  require_command tar
  repository_root=$(git rev-parse --show-toplevel 2>/dev/null) ||
    fail "run this command inside the asc-devtools Git worktree"
  cd -- "${repository_root}"
  git remote get-url "${remote}" >/dev/null 2>&1 ||
    fail "Git remote is not configured: ${remote}"
  [[ -d branches/go && -d branches/python && -d branches/shell ]] ||
    fail "main must contain branches/go, branches/python, and branches/shell"
}

remote_ref() {
  printf 'refs/remotes/%s/%s\n' "${remote}" "$1"
}

fetch_all() {
  local branch
  for branch in main "${implementations[@]}"; do
    printf 'fetch: %s/%s\n' "${remote}" "${branch}"
    git fetch --no-tags "${remote}" \
      "+refs/heads/${branch}:refs/remotes/${remote}/${branch}"
  done
}

ensure_main_branch() {
  local branch
  branch=$(git branch --show-current)
  [[ "${branch}" == main ]] || fail "current branch must be main, found: ${branch:-detached HEAD}"
}

ensure_no_git_operation() {
  local marker
  for marker in MERGE_HEAD CHERRY_PICK_HEAD REVERT_HEAD; do
    git rev-parse --verify -q "${marker}" >/dev/null &&
      fail "finish the active Git operation before continuing: ${marker}"
  done
  [[ ! -d "$(git rev-parse --git-path rebase-merge)" &&
    ! -d "$(git rev-parse --git-path rebase-apply)" ]] ||
    fail "finish the active rebase before continuing"
}

ensure_clean_worktree() {
  [[ -z "$(git status --porcelain=v1 --untracked-files=all)" ]] ||
    fail "working tree must be clean"
}

ensure_clean_index() {
  git diff --cached --quiet || fail "index already contains staged changes"
}

ensure_remote_main_is_ancestor() {
  local tracked_main
  tracked_main=$(remote_ref main)
  git merge-base --is-ancestor "${tracked_main}" HEAD ||
    fail "local main is behind or diverged from ${remote}/main"
}

tree_id() {
  if [[ "$1" == *:* ]]; then
    git rev-parse "$1"
  else
    git rev-parse "$1^{tree}"
  fi
}

relation() {
  local local_commit="$1"
  local remote_commit="$2"
  if [[ "${local_commit}" == "${remote_commit}" ]]; then
    printf 'equal'
  elif git merge-base --is-ancestor "${remote_commit}" "${local_commit}"; then
    printf 'ahead'
  elif git merge-base --is-ancestor "${local_commit}" "${remote_commit}"; then
    printf 'behind'
  else
    printf 'diverged'
  fi
}

command_fetch() {
  fetch_all
}

command_status() {
  local implementation remote_main local_main embedded_tree standalone_tree parity
  fetch_all
  local_main=$(git rev-parse HEAD)
  remote_main=$(git rev-parse "$(remote_ref main)")
  printf '\n%-8s %-12s %-12s %s\n' BRANCH LOCAL REMOTE STATE
  printf '%-8s %-12s %-12s %s\n' main "${local_main:0:12}" "${remote_main:0:12}" \
    "$(relation "${local_main}" "${remote_main}")"
  for implementation in "${implementations[@]}"; do
    embedded_tree=$(tree_id "HEAD:branches/${implementation}")
    standalone_tree=$(tree_id "$(remote_ref "${implementation}")")
    parity=mismatch
    [[ "${embedded_tree}" == "${standalone_tree}" ]] && parity=equal
    printf '%-8s %-12s %-12s %s\n' "${implementation}" \
      "${embedded_tree:0:12}" "${standalone_tree:0:12}" "tree-${parity}"
  done
  if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
    printf '\nWorking tree changes:\n'
    git status --short
  fi
}

make_temporary_root() {
  [[ -n "${temporary_root}" ]] ||
    temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/asc-devtools-github.XXXXXXXX")
}

export_tree() {
  local treeish="$1"
  local destination="$2"
  mkdir -p -- "${destination}"
  git archive --format=tar "${treeish}" | tar -xf - -C "${destination}"
}

command_import() {
  local implementation export_directory tracked_main
  ensure_main_branch
  ensure_no_git_operation
  ensure_clean_worktree
  fetch_all
  tracked_main=$(git rev-parse "$(remote_ref main)")
  [[ "$(git rev-parse HEAD)" == "${tracked_main}" ]] ||
    fail "github-import requires local main to equal ${remote}/main"
  make_temporary_root
  for implementation in "${implementations[@]}"; do
    export_directory="${temporary_root}/export-${implementation}"
    export_tree "$(remote_ref "${implementation}")" "${export_directory}"
    rsync --archive --delete --exclude='.git' \
      "${export_directory}/" "branches/${implementation}/"
    printf 'imported: %s -> branches/%s\n' "${remote}/${implementation}" "${implementation}"
  done
  git diff --check
  printf '\nReview these uncommitted changes before publishing:\n'
  git status --short
}

command_check() {
  local implementation embedded_tree standalone_tree failed=0
  ensure_main_branch
  ensure_no_git_operation
  ensure_clean_worktree
  fetch_all
  [[ "$(git rev-parse HEAD)" == "$(git rev-parse "$(remote_ref main)")" ]] || {
    printf 'mismatch: local main does not equal %s/main\n' "${remote}" >&2
    failed=1
  }
  for implementation in "${implementations[@]}"; do
    embedded_tree=$(tree_id "HEAD:branches/${implementation}")
    standalone_tree=$(tree_id "$(remote_ref "${implementation}")")
    if [[ "${embedded_tree}" == "${standalone_tree}" ]]; then
      printf 'equal: branches/%s == %s/%s\n' "${implementation}" "${remote}" "${implementation}"
    else
      printf 'mismatch: branches/%s != %s/%s\n' "${implementation}" "${remote}" "${implementation}" >&2
      failed=1
    fi
  done
  ((failed == 0)) || fail "published branch parity check failed"
}

publish_message() {
  local message="${ASC_DEVTOOLS_MESSAGE:-}"
  if [[ -z "${message}" ]]; then
    printf -v message 'Updated at %(%Y-%m-%d %H:%M:%S)T' -1
  fi
  [[ "${message}" != *$'\n'* && "${message}" != *$'\r'* ]] ||
    fail "commit message must be one line"
  printf '%s\n' "${message}"
}

prepare_standalone_commit() {
  local implementation="$1"
  local message="$2"
  local tracked_ref worktree_directory export_directory embedded_tree prepared_tree
  tracked_ref=$(remote_ref "${implementation}")
  embedded_tree=$(tree_id "HEAD:branches/${implementation}")
  if [[ "${embedded_tree}" == "$(tree_id "${tracked_ref}")" ]]; then
    prepared_commit_result=$(git rev-parse "${tracked_ref}")
    return
  fi
  worktree_directory="${temporary_root}/worktree-${implementation}"
  export_directory="${temporary_root}/publish-${implementation}"
  git worktree add --quiet --detach "${worktree_directory}" "${tracked_ref}"
  temporary_worktrees+=("${worktree_directory}")
  export_tree "HEAD:branches/${implementation}" "${export_directory}"
  rsync --archive --delete --exclude='.git' \
    "${export_directory}/" "${worktree_directory}/"
  git -C "${worktree_directory}" add --all
  git -C "${worktree_directory}" diff --cached --check
  git -C "${worktree_directory}" commit --quiet -m "${message}"
  prepared_tree=$(tree_id "$(git -C "${worktree_directory}" rev-parse HEAD)")
  [[ "${prepared_tree}" == "${embedded_tree}" ]] ||
    fail "prepared ${implementation} tree does not match main"
  prepared_commit_result=$(git -C "${worktree_directory}" rev-parse HEAD)
}

command_publish() {
  local message main_commit remote_main implementation prepared_commit remote_commit
  local -a push_specs=()
  ensure_main_branch
  ensure_no_git_operation
  ensure_clean_index
  fetch_all
  ensure_remote_main_is_ancestor
  message=$(publish_message)
  git add --all
  git diff --cached --check
  if ! git diff --cached --quiet; then
    git commit -m "${message}"
  fi
  main_commit=$(git rev-parse HEAD)
  remote_main=$(git rev-parse "$(remote_ref main)")
  [[ "${main_commit}" == "${remote_main}" ]] ||
    push_specs+=("${main_commit}:refs/heads/main")

  make_temporary_root
  for implementation in "${implementations[@]}"; do
    prepare_standalone_commit "${implementation}" "${message}"
    prepared_commit="${prepared_commit_result}"
    remote_commit=$(git rev-parse "$(remote_ref "${implementation}")")
    [[ "${prepared_commit}" == "${remote_commit}" ]] ||
      push_specs+=("${prepared_commit}:refs/heads/${implementation}")
  done

  if ((${#push_specs[@]} == 0)); then
    printf 'publish: no branch changes\n'
    return
  fi
  printf 'publish: atomic push to %s\n' "${remote}"
  printf '  %s\n' "${push_specs[@]}"
  git push --atomic "${remote}" "${push_specs[@]}"
  fetch_all
  printf 'publish: complete (%s)\n' "${message}"
}

main() {
  local command_name="${1:-}"
  (($# == 1)) || {
    usage >&2
    exit 2
  }
  initialize
  case "${command_name}" in
    fetch) command_fetch ;;
    status) command_status ;;
    import) command_import ;;
    check) command_check ;;
    publish) command_publish ;;
    -h | --help | help) usage ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
}

main "$@"

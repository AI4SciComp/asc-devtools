#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

project_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
workflow="${project_root}/scripts/github.sh"
test_root=$(mktemp -d "${TMPDIR:-/tmp}/asc-devtools-workflow-test.XXXXXXXX")

cleanup() {
  if [[ "${test_root}" == "${TMPDIR:-/tmp}"/asc-devtools-workflow-test.* &&
    -d "${test_root}" ]]; then
    rm -rf -- "${test_root}"
  fi
}
trap cleanup EXIT

fail() {
  printf 'test_github_workflow.sh: %s\n' "$*" >&2
  exit 1
}

remote="${test_root}/remote.git"
seed="${test_root}/seed"
checkout="${test_root}/checkout"

git init --quiet --bare "${remote}"
git init --quiet "${seed}"
git -C "${seed}" config user.name "Workflow Tests"
git -C "${seed}" config user.email "workflow@example.invalid"
git -C "${seed}" remote add origin "${remote}"

mkdir -p "${seed}/branches/go" "${seed}/branches/python" "${seed}/branches/shell"
printf 'root\n' >"${seed}/README.md"
printf 'main-go\n' >"${seed}/branches/go/value.txt"
printf 'main-python\n' >"${seed}/branches/python/value.txt"
printf 'main-shell\n' >"${seed}/branches/shell/value.txt"
git -C "${seed}" add --all
git -C "${seed}" commit --quiet -m main
git -C "${seed}" branch -M main
git -C "${seed}" push --quiet origin main

for implementation in go python shell; do
  git -C "${seed}" switch --quiet --orphan "${implementation}"
  git -C "${seed}" rm --quiet -rf --ignore-unmatch .
  printf 'remote-%s\n' "${implementation}" >"${seed}/value.txt"
  git -C "${seed}" add value.txt
  git -C "${seed}" commit --quiet -m "${implementation}"
  git -C "${seed}" push --quiet origin "${implementation}"
done
git -C "${seed}" switch --quiet main

git clone --quiet --branch main "${remote}" "${checkout}"
git -C "${checkout}" config user.name "Workflow Tests"
git -C "${checkout}" config user.email "workflow@example.invalid"

(
  cd -- "${checkout}"
  ASC_DEVTOOLS_REMOTE=origin "${workflow}" import >/dev/null
)
for implementation in go python shell; do
  [[ "$(<"${checkout}/branches/${implementation}/value.txt")" == "remote-${implementation}" ]] ||
    fail "import did not update ${implementation}"
done

(
  cd -- "${checkout}"
  ASC_DEVTOOLS_REMOTE=origin ASC_DEVTOOLS_MESSAGE="Import standalone branches" \
    "${workflow}" publish >/dev/null
  ASC_DEVTOOLS_REMOTE=origin "${workflow}" check >/dev/null
)

printf 'published-python\n' >"${checkout}/branches/python/value.txt"
printf 'published-root\n' >>"${checkout}/README.md"
before_main=$(git --git-dir="${remote}" rev-parse refs/heads/main)
mkdir -p "${remote}/hooks"
printf '#!/usr/bin/env bash\nexit 1\n' >"${remote}/hooks/pre-receive"
chmod +x "${remote}/hooks/pre-receive"
if (
  cd -- "${checkout}"
  ASC_DEVTOOLS_REMOTE=origin ASC_DEVTOOLS_MESSAGE="Rejected atomic publish" \
    "${workflow}" publish >/dev/null 2>&1
); then
  fail "publish unexpectedly succeeded with rejecting remote"
fi
[[ "$(git --git-dir="${remote}" rev-parse refs/heads/main)" == "${before_main}" ]] ||
  fail "atomic rejection moved main"
[[ "$(git --git-dir="${remote}" show refs/heads/python:value.txt)" == remote-python ]] ||
  fail "atomic rejection moved python"

rm -f -- "${remote}/hooks/pre-receive"
(
  cd -- "${checkout}"
  ASC_DEVTOOLS_REMOTE=origin ASC_DEVTOOLS_MESSAGE="Retry atomic publish" \
    "${workflow}" publish >/dev/null
  ASC_DEVTOOLS_REMOTE=origin "${workflow}" check >/dev/null
)
[[ "$(git --git-dir="${remote}" show refs/heads/python:value.txt)" == published-python ]] ||
  fail "retry did not publish python"
[[ "$(git --git-dir="${remote}" show refs/heads/main:branches/python/value.txt)" == published-python ]] ||
  fail "retry did not publish matching main tree"

printf 'default-message\n' >>"${checkout}/README.md"
(
  cd -- "${checkout}"
  env -u ASC_DEVTOOLS_MESSAGE ASC_DEVTOOLS_REMOTE=origin \
    "${workflow}" publish >/dev/null
  ASC_DEVTOOLS_REMOTE=origin "${workflow}" check >/dev/null
)
default_subject=$(git --git-dir="${remote}" log -1 --format=%s refs/heads/main)
[[ "${default_subject}" =~ ^Updated\ at\ [0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}:[0-9]{2}$ ]] ||
  fail "default publish message has an unexpected format: ${default_subject}"

printf 'GitHub workflow tests passed\n'

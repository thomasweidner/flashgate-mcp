#!/usr/bin/env bash
set -uo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
failures=()
check_count=0
temporary_directory=""

check() {
  local condition="$1"
  local name="$2"
  check_count=$((check_count + 1))
  if [[ "${condition}" != true ]]; then
    failures+=("${name}")
  fi
}

cleanup() {
  if [[ -n "${temporary_directory}" && -d "${temporary_directory}" ]]; then
    rm -rf -- "${temporary_directory}"
  fi
}
trap cleanup EXIT

temporary_directory="$(mktemp -d)"
fake_repository="${temporary_directory}/repo"
fake_scripts="${fake_repository}/scripts"
fake_benchmarks="${fake_repository}/benchmarks"
stub_directory="${temporary_directory}/stub"
mkdir -p -- "${fake_scripts}" "${fake_benchmarks}" "${stub_directory}"
wrapper="${fake_scripts}/benchmark.sh"
cp -- "${script_dir}/benchmark.sh" "${wrapper}"
marker="${temporary_directory}/go-invoked.marker"
output="${temporary_directory}/baseline.linux-amd64.json"
stub="${stub_directory}/go"
printf '#!/usr/bin/env bash\n: > %q\nif [[ "$1" == env && "$2" == GOARCH ]]; then printf "amd64\\n"; exit 0; fi\nexit 97\n' "${marker}" >"${stub}"
chmod 700 "${stub}"

stdout_path="${temporary_directory}/stdout.log"
stderr_path="${temporary_directory}/stderr.log"
PATH="${stub_directory}:${PATH}" bash "${wrapper}" --record-baseline --output "${output}" >"${stdout_path}" 2>"${stderr_path}"
exit_code=$?
stderr_value="$(<"${stderr_path}")"

check "$([[ ${exit_code} -ne 0 ]] && printf true || printf false)" 'record mode exits nonzero'
check "$([[ "${stderr_value}" == *'Authoritative baseline recording is not supported by scripts/benchmark.sh'* ]] && printf true || printf false)" 'record mode emits policy error on stderr'
check "$([[ ! -e "${marker}" ]] && printf true || printf false)" 'record mode does not invoke go'
check "$([[ ! -e "${output}" ]] && printf true || printf false)" 'record mode does not write baseline output'
check "$([[ ! -d "${fake_repository}/build" ]] && printf true || printf false)" 'record mode does not create build directory'
check "$([[ ! -s "${stdout_path}" ]] && printf true || printf false)" 'record mode does not emit success output'

canonical_output="${fake_repository}/benchmarks/../benchmarks/baseline.linux-amd64.json"
: >"${stdout_path}"
: >"${stderr_path}"
PATH="${stub_directory}:${PATH}" bash "${wrapper}" --output "${canonical_output}" >"${stdout_path}" 2>"${stderr_path}"
canonical_exit_code=$?
canonical_stdout="$(<"${stdout_path}")"
canonical_stderr="$(<"${stderr_path}")"
check "$([[ ${canonical_exit_code} -ne 0 ]] && printf true || printf false)" 'non-authoritative mode rejects canonical baseline path'
check "$([[ "${canonical_stderr}" == *'protected baseline directory'* ]] && printf true || printf false)" 'canonical-path rejection is reported'
check "$([[ -z "${canonical_stdout}" ]] && printf true || printf false)" 'canonical-path rejection emits no success output'
check "$([[ ! -e "${marker}" ]] && printf true || printf false)" 'canonical-path rejection does not invoke go'
check "$([[ ! -e "${fake_repository}/benchmarks/baseline.linux-amd64.json" ]] && printf true || printf false)" 'canonical-path rejection does not write output'
check "$([[ ! -d "${fake_repository}/build" ]] && printf true || printf false)" 'canonical-path rejection does not create build directory'

baseline_target="${fake_benchmarks}/baseline.linux-amd64.json"
printf 'baseline' >"${baseline_target}"
baseline_hash="$(sha256sum "${baseline_target}" | cut -d ' ' -f 1)"

rm -f -- "${marker}"
alias_directory="${temporary_directory}/alias-benchmarks"
ln -s -- "${fake_benchmarks}" "${alias_directory}"
alias_output="${alias_directory}/benchmark-current.linux-amd64.json"
: >"${stdout_path}"
: >"${stderr_path}"
PATH="${stub_directory}:${PATH}" bash "${wrapper}" --output "${alias_output}" >"${stdout_path}" 2>"${stderr_path}"
alias_exit_code=$?
alias_stderr="$(<"${stderr_path}")"
check "$([[ ${alias_exit_code} -ne 0 ]] && printf true || printf false)" 'noncanonical baseline-directory alias exits nonzero'
check "$([[ "${alias_stderr}" == *'protected baseline directory'* ]] && printf true || printf false)" 'noncanonical baseline-directory alias emits policy error'
check "$([[ ! -e "${marker}" ]] && printf true || printf false)" 'noncanonical baseline-directory alias does not invoke go'

rm -f -- "${marker}"
linked_output="${temporary_directory}/benchmark-current.linux-amd64.json"
ln -s -- "${baseline_target}" "${linked_output}"
: >"${stdout_path}"
: >"${stderr_path}"
PATH="${stub_directory}:${PATH}" bash "${wrapper}" --output "${linked_output}" >"${stdout_path}" 2>"${stderr_path}"
linked_exit_code=$?
linked_stderr="$(<"${stderr_path}")"
check "$([[ ${linked_exit_code} -ne 0 ]] && printf true || printf false)" 'noncanonical final file symlink exits nonzero'
check "$([[ "${linked_stderr}" == *'final symbolic-link output target'* ]] && printf true || printf false)" 'noncanonical final file symlink emits policy error'
check "$([[ ! -e "${marker}" ]] && printf true || printf false)" 'noncanonical final file symlink does not invoke go'

rm -f -- "${linked_output}" "${marker}"
ln -s -- "${temporary_directory}/missing.json" "${linked_output}"
: >"${stdout_path}"
: >"${stderr_path}"
PATH="${stub_directory}:${PATH}" bash "${wrapper}" --output "${linked_output}" >"${stdout_path}" 2>"${stderr_path}"
broken_exit_code=$?
broken_stderr="$(<"${stderr_path}")"
check "$([[ ${broken_exit_code} -ne 0 ]] && printf true || printf false)" 'broken final file symlink exits nonzero'
check "$([[ "${broken_stderr}" == *'final symbolic-link output target'* ]] && printf true || printf false)" 'broken final file symlink emits policy error'
check "$([[ ! -e "${marker}" ]] && printf true || printf false)" 'broken final file symlink does not invoke go'

rm -f -- "${linked_output}" "${marker}"
mkdir -p -- "${fake_repository}/build"
default_output="${fake_repository}/build/benchmark-current.linux-amd64.json"
ln -s -- "${baseline_target}" "${default_output}"
: >"${stdout_path}"
: >"${stderr_path}"
PATH="${stub_directory}:${PATH}" bash "${wrapper}" >"${stdout_path}" 2>"${stderr_path}"
default_exit_code=$?
default_stderr="$(<"${stderr_path}")"
check "$([[ ${default_exit_code} -ne 0 ]] && printf true || printf false)" 'default output final symlink exits nonzero'
check "$([[ "${default_stderr}" == *'final symbolic-link output target'* ]] && printf true || printf false)" 'default output is checked before build output'

current_baseline_hash="$(sha256sum "${baseline_target}" | cut -d ' ' -f 1)"
check "$([[ "${current_baseline_hash}" == "${baseline_hash}" ]] && printf true || printf false)" 'authoritative Linux baseline remains unchanged'

clock_marker="${temporary_directory}/clock-invoked.marker"
printf '#!/usr/bin/env bash\n: > %q\nexit 99\n' "${clock_marker}" >"${stub_directory}/date"
printf '#!/usr/bin/env bash\nexit 0\n' >"${stub_directory}/git"
chmod 700 "${stub_directory}/date" "${stub_directory}/git"
rm -f -- "${marker}"
diagnostic_output="${temporary_directory}/diagnostic.json"
PATH="${stub_directory}:${PATH}" bash "${wrapper}" --quick --output "${diagnostic_output}" >"${stdout_path}" 2>"${stderr_path}"
diagnostic_exit_code=$?
check "$([[ ${diagnostic_exit_code} -eq 97 && -e "${marker}" ]] && printf true || printf false)" 'regular diagnostic output reaches the Go stub'
check "$([[ ! -e "${clock_marker}" ]] && printf true || printf false)" 'diagnostic wrapper does not consult the wall clock'

if (( ${#failures[@]} == 0 )); then
  printf 'Status       : PASS\nCheckCount   : %d\nFailureCount : 0\nFailures     :\n' "${check_count}"
  exit 0
fi
printf 'Status       : FAIL\nCheckCount   : %d\nFailureCount : %d\nFailures     : %s\n' "${check_count}" "${#failures[@]}" "${failures[*]}"
exit 1

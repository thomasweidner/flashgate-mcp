#!/usr/bin/env bash
set -euo pipefail

quick=false
record_baseline=false
output_path=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --quick)
      quick=true
      ;;
    --record-baseline)
      record_baseline=true
      ;;
    --output)
      shift
      output_path="${1:?--output requires a path}"
      ;;
    *)
      printf 'Unknown argument: %s\n' "$1" >&2
      exit 2
      ;;
  esac
  shift
done

if [[ "${record_baseline}" == true ]]; then
  printf 'Authoritative baseline recording is not supported by scripts/benchmark.sh. Use the documented two-phase prebuilt workflow with a native Linux checkout under /home and the Windows controller workspace outside synchronized storage.\n' >&2
  exit 2
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
build_dir="${repo_root}/build"
server_binary="${build_dir}/flashgate-mcp"
benchmark_binary="${build_dir}/flashgate-benchmark"
budget_path="${repo_root}/benchmarks/budgets.json"
protected_baseline_dir="$(realpath -e -- "${repo_root}/benchmarks")"

assert_non_authoritative_output_path() {
  local candidate_path="$1"
  local parent_path
  local physical_parent

  if [[ -L "${candidate_path}" ]]; then
    printf 'Non-authoritative benchmark runs must not use a final symbolic-link output target.\n' >&2
    return 1
  fi
  if [[ -e "${candidate_path}" && ! -f "${candidate_path}" ]]; then
    printf 'Non-authoritative benchmark output targets must be regular files.\n' >&2
    return 1
  fi

  parent_path="$(dirname -- "${candidate_path}")"
  if [[ -e "${parent_path}" ]]; then
    physical_parent="$(realpath -e -- "${parent_path}")"
    if [[ "${physical_parent}" == "${protected_baseline_dir}" ]]; then
      printf 'Non-authoritative benchmark runs must not write into the protected baseline directory.\n' >&2
      return 1
    fi
  elif [[ "$(basename -- "${candidate_path}")" == baseline.*-*.json ]]; then
    printf 'Non-authoritative benchmark runs must not use a canonical baseline name below an unresolved physical parent path.\n' >&2
    return 1
  fi
}

if [[ -z "${output_path}" ]]; then
  output_path="${build_dir}/benchmark-current.linux-$(go env GOARCH).json"
elif [[ "${output_path}" != /* ]]; then
  output_path="${repo_root}/${output_path}"
fi
output_path="$(realpath -m -s -- "${output_path}")"
assert_non_authoritative_output_path "${output_path}"

working_tree_dirty="$(git status --porcelain --untracked-files=all)"

mkdir -p "${build_dir}"
go build -o "${server_binary}" ./cmd/server
go build -o "${benchmark_binary}" ./cmd/benchmark
commit="$(git rev-parse HEAD)"

arguments=(
  -binary "${server_binary}"
  -output "${output_path}"
  -commit "${commit}"
  -budgets "${budget_path}"
  -protected-baseline-dir "${protected_baseline_dir}"
)
if [[ -n "${working_tree_dirty}" ]]; then
  arguments+=(-working-tree-dirty)
fi
if [[ "${quick}" == true ]]; then
  arguments+=(-quick)
fi

assert_non_authoritative_output_path "${output_path}"
"${benchmark_binary}" "${arguments[@]}"

printf 'Benchmark result: %s\n' "${output_path}"

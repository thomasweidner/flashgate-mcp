#!/usr/bin/env bash
set -Eeuo pipefail

export LC_ALL=C

required_bash_path='/usr/bin/bash'
repository_root=''
bash_path="$required_bash_path"
failures=()
inventory=()
git_path=''
before_status_hash=''
before_script_hash=''
mutation_detected='false'

add_failure() {
  failures+=("$1")
}

usage() {
  printf 'Usage: scripts/test-shell-scripts.sh [--repository-root PATH] [--bash-path /usr/bin/bash]\n'
}

while (($# > 0)); do
  case "$1" in
    --repository-root)
      (($# >= 2)) || { add_failure 'Missing value for --repository-root.'; break; }
      repository_root=$2
      shift 2
      ;;
    --bash-path)
      (($# >= 2)) || { add_failure 'Missing value for --bash-path.'; break; }
      bash_path=$2
      shift 2
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      add_failure "Unknown argument: $1"
      shift
      ;;
  esac
done

if [[ -z "$repository_root" ]]; then
  repository_root=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
fi

hash_repository_status() {
  "$git_path" -C "$repository_root" status --porcelain=v1 -z --untracked-files=all |
    /usr/bin/sha256sum | /usr/bin/cut -d ' ' -f 1
}

hash_inventory() {
  local path full
  for path in "${inventory[@]}"; do
    full="$repository_root/$path"
    if [[ -f "$full" && ! -L "$full" ]]; then
      printf '%s=%s\n' "$path" "$(/usr/bin/sha256sum -- "$full" | /usr/bin/cut -d ' ' -f 1)"
    else
      printf '%s=<missing-or-nonregular>\n' "$path"
    fi
  done | /usr/bin/sha256sum | /usr/bin/cut -d ' ' -f 1
}

if ((${#failures[@]} == 0)); then
  if [[ "$bash_path" != "$required_bash_path" ]]; then
    add_failure "Bash path must be exactly $required_bash_path."
  elif [[ ! -x "$bash_path" || -L "$bash_path" ]]; then
    add_failure "Required native Bash executable is unavailable or is a link: $bash_path"
  fi

  git_path=$(command -v git || true)
  if [[ -z "$git_path" || ! -x "$git_path" ]]; then
    add_failure 'Git executable is unavailable.'
  fi
fi

if ((${#failures[@]} == 0)); then
  if [[ ! -d "$repository_root" || -L "$repository_root" ]]; then
    add_failure "Repository root is unavailable or is a link: $repository_root"
  else
    repository_root=$(cd -P -- "$repository_root" && pwd)
    top_level=$($git_path -C "$repository_root" rev-parse --show-toplevel 2>/dev/null || true)
    if [[ -z "$top_level" || "$top_level" != "$repository_root" ]]; then
      add_failure "Repository root is not the actual Git top-level: $repository_root"
    fi
  fi
fi

if ((${#failures[@]} == 0)); then
  before_status_hash=$(hash_repository_status)
  mapfile -d '' -t tracked_paths < <(
    "$git_path" -C "$repository_root" -c core.quotePath=false ls-files --cached --others --exclude-standard -z
  )
  for path in "${tracked_paths[@]}"; do
    if [[ -z "$path" || "$path" == /* || "$path" == *'\\'* || "$path" == *'//'* || "$path" == *$'\n'* ]]; then
      add_failure "Git returned a non-canonical tracked path: $path"
      continue
    fi
    case "/$path/" in
      *'/../'*|*'/./'*) add_failure "Git returned a traversing tracked path: $path"; continue ;;
    esac

    if [[ "$path" == *.sh ]]; then
      inventory+=("$path")
      continue
    fi
    full_path="$repository_root/$path"
    if [[ -f "$full_path" && ! -L "$full_path" ]]; then
      IFS= read -r first_line < "$full_path" || true
      if [[ "$first_line" =~ ^\#\!.*bash([[:space:]]|$) ]]; then
        inventory+=("$path")
      fi
    fi
  done

  if ((${#inventory[@]} > 0)); then
    mapfile -t inventory < <(printf '%s\n' "${inventory[@]}" | /usr/bin/sort -u)
  fi
  if ((${#inventory[@]} == 0)); then
    add_failure 'Bash inventory is empty.'
  fi
  before_script_hash=$(hash_inventory)
fi

if ((${#failures[@]} == 0)); then
  for path in "${inventory[@]}"; do
    full_path="$repository_root/$path"
    if [[ ! -f "$full_path" || -L "$full_path" ]]; then
      add_failure "$path: tracked Bash entry point is missing, nonregular, or a link."
      continue
    fi
    if ! /usr/bin/iconv -f UTF-8 -t UTF-8 -- "$full_path" >/dev/null 2>&1; then
      add_failure "$path: file is not strict UTF-8."
      continue
    fi
    if LC_ALL=C /usr/bin/grep -q $'\r' -- "$full_path"; then
      add_failure "$path: Bash entry point must use LF line endings."
    fi
    final_byte=$(/usr/bin/tail -c 1 -- "$full_path" | /usr/bin/od -An -t x1 | /usr/bin/tr -d ' \n')
    if [[ "$final_byte" != '0a' ]]; then
      add_failure "$path: Bash entry point must end with a newline."
    fi
    IFS= read -r first_line < "$full_path" || true
    if [[ "$first_line" != '#!/usr/bin/env bash' && "$first_line" != '#!/bin/bash' ]]; then
      add_failure "$path: unsupported or missing Bash shebang."
    fi

    syntax_output=''
    if syntax_output=$(/usr/bin/timeout 30s "$bash_path" -n -- "$full_path" 2>&1); then
      :
    else
      syntax_exit=$?
      add_failure "$path: native Bash syntax validation failed (exit=$syntax_exit): $syntax_output"
    fi
  done
fi

if [[ -n "$git_path" && -d "$repository_root" ]]; then
  after_status_hash=$(hash_repository_status 2>/dev/null || true)
  after_script_hash=$(hash_inventory 2>/dev/null || true)
  if [[ "$before_status_hash" != "$after_status_hash" || "$before_script_hash" != "$after_script_hash" ]]; then
    mutation_detected='true'
    add_failure 'Repository state or tracked Bash bytes changed during validation.'
  fi
fi

status='PASS'
exit_code=0
if ((${#failures[@]} > 0)); then
  status='FAIL'
  exit_code=1
fi
diagnostics='NONE'
if ((${#failures[@]} > 0)); then
  diagnostics=$(IFS=' | '; printf '%s' "${failures[*]}")
fi

printf '%s\n' \
  "Status: $status" \
  "RepositoryRoot: $repository_root" \
  "Shell: $bash_path" \
  "GitPath: $git_path" \
  "BashScriptCount: ${#inventory[@]}" \
  "RepositoryMutationDetected: $mutation_detected" \
  "Diagnostics: $diagnostics" \
  'WarningCount: 0' \
  "FailureCount: ${#failures[@]}"

exit "$exit_code"

#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
project_root="$(cd "${script_dir}/.." && pwd)"
lock_file="${project_root}/release-lock.json"
destination="${1:-${project_root}/.ci/xparse-skills}"

read_lock_string() {
  local field="$1"
  sed -n "s/^[[:space:]]*\"${field}\":[[:space:]]*\"\([^\"]*\)\".*/\1/p" \
    "$lock_file"
}

repository="$(read_lock_string skills_repository)"
commit="$(read_lock_string skills_commit)"

if [ -z "$repository" ] || [[ ! "$commit" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Invalid skills repository or commit in ${lock_file}." >&2
  exit 1
fi
if [ -e "$destination" ]; then
  echo "Destination already exists: ${destination}" >&2
  exit 1
fi

mkdir -p "$(dirname "$destination")"
git clone --filter=blob:none --no-checkout "$repository" "$destination"
git -C "$destination" fetch --depth=1 origin "$commit"
git -C "$destination" checkout --detach "$commit"

actual_commit="$(git -C "$destination" rev-parse HEAD)"
if [ "$actual_commit" != "$commit" ]; then
  echo "Checked out ${actual_commit}, expected ${commit}." >&2
  exit 1
fi

printf '%s\n' "$destination"

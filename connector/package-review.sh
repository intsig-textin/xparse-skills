#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <version> <output-directory>" >&2
  echo "Example: $0 v2.2.1 ~/Downloads" >&2
  exit 2
fi

version="$1"
output_dir="$2"
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid version: expected v<major>.<minor>.<patch>." >&2
  exit 2
fi

script_dir="$(cd "$(dirname "$0")" && pwd)"
project_root="$(cd "${script_dir}/.." && pwd)"
skills_repository="${XPARSE_SKILLS_REPOSITORY:-}"
if [ -z "$skills_repository" ]; then
  for candidate in \
    "${project_root}/../xparse-skills-v2.2.1" \
    "${project_root}/../xparse-skills"; do
    if [ -d "$candidate" ]; then
      skills_repository="$candidate"
      break
    fi
  done
fi
if [ -z "$skills_repository" ]; then
  echo "xparse-skills repository not found; set XPARSE_SKILLS_REPOSITORY." >&2
  exit 1
fi

lock_file="${project_root}/release-lock.json"
locked_skills_commit="$(sed -n \
  's/^[[:space:]]*"skills_commit":[[:space:]]*"\([0-9a-f]*\)".*/\1/p' \
  "$lock_file")"
actual_skills_commit="$(git -C "$skills_repository" rev-parse HEAD)"
if [ "$actual_skills_commit" != "$locked_skills_commit" ]; then
  echo "xparse-skills checkout does not match release-lock.json." >&2
  exit 1
fi

skill_source="${skills_repository}/skills/xparse-parse"
validator="${script_dir}/validate-review-input.py"
python3 "$validator" \
  "${script_dir}/cli.json" \
  "${script_dir}/connector-meta.json" \
  "${script_dir}/marketplace-entry.json" \
  "$version" \
  "$skill_source"

package_name="textin-xparse-${version}"
archive_name="textin-xparse-${version}-cn.zip"
stage_parent="$(mktemp -d)"
package_root="${stage_parent}/${package_name}"
temporary_archive="${stage_parent}/${archive_name}"
final_archive="${output_dir%/}/${archive_name}"

cleanup() {
  find "$stage_parent" -depth -delete 2>/dev/null || true
}
trap cleanup EXIT

if [ -e "$final_archive" ]; then
  echo "Refusing to overwrite existing package: ${final_archive}" >&2
  exit 1
fi

mkdir -p "$package_root" "$output_dir" "${package_root}/skills"
cp "${script_dir}/cli.json" "${package_root}/cli.json"
cp "${script_dir}/connector-meta.json" "${package_root}/connector-meta.json"
cp "${script_dir}/marketplace-entry.json" "${package_root}/marketplace-entry.json"
cp "${script_dir}/icon.png" "${package_root}/icon.png"
cp -R "$skill_source" "${package_root}/skills/xparse-parse"

if find "$package_root" \
  \( -name 'REVIEW.md' -o -name '.DS_Store' -o -name '*.bak' -o -name '.dev-flow' \) \
  -print -quit | grep -q .; then
  echo "Review package contains a forbidden file." >&2
  exit 1
fi
if [ -e "${package_root}/skills/xparse-doc-tools" ]; then
  echo "Review package must contain only xparse-parse." >&2
  exit 1
fi

python3 - "$package_root" <<'PY'
from __future__ import annotations

import hashlib
import sys
from pathlib import Path

root = Path(sys.argv[1])
lines: list[str] = []
for path in sorted(item for item in root.rglob("*") if item.is_file()):
    if path.name == "SHA256SUMS":
        continue
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    lines.append(f"{digest}  {path.relative_to(root).as_posix()}")
(root / "SHA256SUMS").write_text("\n".join(lines) + "\n", encoding="utf-8")
PY

(
  cd "$stage_parent"
  COPYFILE_DISABLE=1 zip -X -q -r "$temporary_archive" "$package_name"
)
mv "$temporary_archive" "$final_archive"
printf 'Created domestic Connector review package: %s\n' "$final_archive"

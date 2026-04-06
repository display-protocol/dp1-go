#!/usr/bin/env bash
# Usage: check-coverage.sh [threshold_percent] [merged_coverprofile_out]
# Second argument, if set, writes the merged atomic coverprofile (e.g. for Codecov) on success.
set -euo pipefail
cd "$(dirname "$0")/.."
threshold="${1:-80}"
outfile="${2:-}"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
echo "mode: atomic" >"$tmp"
while read -r pkg; do
  f=$(mktemp)
  # Match CI: race detector + atomic coverprofile per package (merged below).
  go test "$pkg" -race -count=1 -coverprofile="$f" -covermode=atomic >/dev/null
  tail -n +2 "$f" >>"$tmp"
  rm -f "$f"
done < <(go list ./... | grep -v '/internal/schema$')
pct=$(go tool cover -func="$tmp" | awk '/^total:/{gsub("%","",$3); print $3}')
awk -v p="$pct" -v t="$threshold" 'BEGIN{exit !(p+0 >= t+0)}' || {
  echo "coverage ${pct}% is below ${threshold}%"
  go tool cover -func="$tmp" | tail -20
  exit 1
}
echo "total coverage: ${pct}% (threshold ${threshold}%)"
if [[ -n "${outfile}" ]]; then
  cp "$tmp" "${outfile}"
fi

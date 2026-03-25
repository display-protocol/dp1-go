#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
threshold="${1:-80}"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
echo "mode: atomic" >"$tmp"
while read -r pkg; do
  f=$(mktemp)
  go test "$pkg" -coverprofile="$f" -covermode=atomic >/dev/null
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

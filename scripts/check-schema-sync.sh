#!/usr/bin/env bash
# Usage: check-schema-sync.sh <dp1-checkout> [--write]
#
# Fails when the schemas embedded under internal/schema/ are not byte copies of the DP-1 spec
# repository (display-protocol/dp1) at the commit pinned in internal/schema/dp1-commit.txt.
# --write copies the pinned upstream files over the embedded ones instead of failing; the Go
# types, merge rules and tests that the new schemas imply are still a hand-written port.
#
# Why byte copies and a pin: the copy is hand-maintained, and before this check four spec
# changes landed upstream in four days without reaching it, so documents the spec accepted
# failed here. The pin makes "which spec does this SDK implement" a single line; moving it
# without syncing turns the build red instead of letting the drift ship.
#
# The upstream set is every JSON Schema (an object with "$schema" and "$id") under core/ and
# extensions/ at the pin, examples excluded. A new upstream schema therefore fails the check
# until it is embedded and ported.
set -euo pipefail
cd "$(dirname "$0")/.."

dp1="${1:?usage: check-schema-sync.sh <dp1-checkout> [--write]}"
mode="${2:-check}"
[[ "$mode" == "check" || "$mode" == "--write" ]] || { echo "unknown option: $mode"; exit 2; }
pin=$(tr -d '[:space:]' <internal/schema/dp1-commit.txt)
git -C "$dp1" cat-file -e "${pin}^{commit}" 2>/dev/null || {
  echo "dp1 checkout at $dp1 does not have the pinned commit $pin; fetch it first"
  exit 2
}

# Embedded files with no upstream counterpart, each with the reason it stays. The spec removed
# playlist-group in dp1#41, but dp1-feed-v2, dp1-cli and dp1-validator still import the
# playlistgroup package; dropping it is its own breaking change, not part of a schema sync.
local_only=(core/v1.1.0/playlist-group.json)

# Upstream keeps core schemas under core/<version>/schemas/; this module embeds them one level up.
embedded_path() { sed -E 's#^core/([^/]+)/schemas/#core/\1/#' <<<"$1"; }

status=0
declare -A mapped=()
blob=$(mktemp)
trap 'rm -f "$blob"' EXIT
while read -r up; do
  # A failed read is an error, never "not a schema": skipping it would let a new upstream
  # schema go unchecked, since nothing local exists for the extra-file scan below to flag.
  git -C "$dp1" show "$pin:$up" >"$blob" || { echo "cannot read dp1 $pin:$up"; exit 2; }
  jq -e 'type == "object" and has("$schema") and has("$id")' "$blob" >/dev/null 2>&1 || continue
  local_path="$(embedded_path "$up")"
  mapped["$local_path"]=1
  dst="internal/schema/$local_path"
  if [[ -f "$dst" ]] && cmp -s "$blob" "$dst"; then
    continue
  fi
  if [[ "$mode" == "--write" ]]; then
    mkdir -p "$(dirname "$dst")"
    cp "$blob" "$dst"
    echo "wrote   $dst  <- dp1 $up"
  else
    [[ -f "$dst" ]] && echo "differs $dst  (dp1 $up)" || echo "missing $dst  (dp1 $up)"
    status=1
  fi
done < <(git -C "$dp1" ls-tree -r --name-only "$pin" -- core extensions | grep -E '\.json$' | grep -v '/examples/')

while read -r f; do
  rel="${f#internal/schema/}"
  [[ -n "${mapped[$rel]:-}" ]] && continue
  printf '%s\n' "${local_only[@]}" | grep -qxF "$rel" && continue
  echo "extra   $f  (no schema at this path in dp1 $pin; delete it or list it in local_only)"
  status=1
done < <(find internal/schema/core internal/schema/extensions -name '*.json' | sort)

if [[ $status -ne 0 ]]; then
  echo "embedded schemas are out of sync with dp1 $pin; run: scripts/check-schema-sync.sh <dp1-checkout> --write"
  exit 1
fi
echo "embedded schemas match dp1 $pin"

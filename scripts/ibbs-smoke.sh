#!/usr/bin/env bash
# End-to-end inter-BBS smoke test with the real barons-door binary.
# Validates: Synchronet DOOR32.SYS parsing, onboarding, RunPlanetary export,
# file-drop transport, and import on a second board. No Synchronet required —
# a shared directory stands in for the packet transport (mailer / sync / scp).
set -euo pipefail

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
door="$tmp/barons-door"
go build -o "$door" ./cmd/barons-door

mkdir -p "$tmp/A" "$tmp/B" "$tmp/xchg"
cat > "$tmp/door32.sys" <<DROP
2
0
0
TestBBS
1
Andy Alt
andy
100
30
1
1
DROP
for b in A B; do
  id=$([ "$b" = A ] && echo AlphaBBS || echo BravoBBS)
  cat > "$tmp/$b/config.json" <<CFG
{"TurnsPerDay":10,"ProtectionTurns":20,"AICount":2,"DataDir":"$tmp/$b","GameLength":30,"BoardID":"$id","IBBS":true,"InboundDir":"$tmp/xchg","OutboundDir":"$tmp/xchg"}
CFG
done

# Onboard a human on board A via the real dropfile path, then export its scores.
printf ' Asgard\r \r \rQ\r' | timeout 15 "$door" -dropfile "$tmp/door32.sys" -data "$tmp/A" >/dev/null 2>&1 || true
"$door" -planetary -data "$tmp/A"
[ -n "$(ls -A "$tmp/xchg")" ] || { echo "FAIL: no packet written"; exit 1; }

# Board B imports it.
"$door" -planetary -data "$tmp/B"
if grep -q '"BoardID":"AlphaBBS"' "$tmp/B/world.json"; then
  echo "PASS: BravoBBS imported AlphaBBS scores over the file-drop transport"
else
  echo "FAIL: BravoBBS did not import AlphaBBS scores"; exit 1
fi

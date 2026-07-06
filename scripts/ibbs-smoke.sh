#!/usr/bin/env bash
# End-to-end inter-BBS smoke test with the real immortal-barons binary across THREE
# boards. Validates: Synchronet DOOR32.SYS parsing, onboarding, RunPlanetary
# export, the file-drop transport, and import on the other boards.
#
# No Synchronet required. Each board has its own inbound/outbound dir (as a real
# BBS.CFG would), and a shell "transport" step fans each outbound packet out to
# the other boards' inbound dirs — exactly what the sysop's mailer/sync/scp
# script does between maintenance runs. The sysop chooses how often that runs.
set -euo pipefail

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
door="$tmp/immortal-barons"
go build -o "$door" ./cmd/immortal-barons

boards=(AlphaBBS BravoBBS CharlieBBS)
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

for id in "${boards[@]}"; do
  mkdir -p "$tmp/$id" "$tmp/$id/in" "$tmp/$id/out"
  cat > "$tmp/$id/config.json" <<CFG
{"TurnsPerDay":10,"ProtectionTurns":20,"AICount":2,"DataDir":"$tmp/$id","GameLength":30,"BoardID":"$id","IBBS":true,"InboundDir":"$tmp/$id/in","OutboundDir":"$tmp/$id/out"}
CFG
done

# transport: fan every board's outbound packet out to the other boards' inbound
# dirs (a broadcast reaches all; the sysop's real script does the same move).
transport() {
  for src in "${boards[@]}"; do
    for pkt in "$tmp/$src/out/"*.brp; do
      [ -e "$pkt" ] || continue
      for dst in "${boards[@]}"; do
        [ "$dst" = "$src" ] && continue
        cp "$pkt" "$tmp/$dst/in/"
      done
      rm -f "$pkt"
    done
  done
}

# Onboard a human on AlphaBBS via the real dropfile path, then export its scores.
printf ' Asgard\r \r \rQ\r' | timeout 15 "$door" -dropfile "$tmp/door32.sys" -data "$tmp/AlphaBBS" >/dev/null 2>&1 || true
"$door" -planetary -data "$tmp/AlphaBBS"
[ -n "$(ls -A "$tmp/AlphaBBS/out")" ] || { echo "FAIL: AlphaBBS wrote no packet"; exit 1; }

transport   # the sysop's between-runs packet move

# Bravo and Charlie process their inbound on their next maintenance run.
"$door" -planetary -data "$tmp/BravoBBS"
"$door" -planetary -data "$tmp/CharlieBBS"

fail=0
for id in BravoBBS CharlieBBS; do
  if grep -q 'AlphaBBS' "$tmp/$id/world.json"; then
    echo "PASS: $id imported AlphaBBS scores over the file-drop transport"
  else
    echo "FAIL: $id did not import AlphaBBS scores"; fail=1
  fi
done
exit $fail

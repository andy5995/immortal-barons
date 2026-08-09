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
# Comm type 0 = local; a socket type needs a real handle on line 2.
cat > "$tmp/door32.sys" <<DROP
0
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
{"TurnsPerDay":10,"ProtectionTurns":20,"AICount":2,"DataDir":"$tmp/$id","GameLength":30,"IBBS":true}
CFG
  # This board's own identity and directories: bbs.cfg, not config.json.
  cat > "$tmp/$id/bbs.cfg" <<CFG
BoardID $id
Inbound $tmp/$id/in
Outbound $tmp/$id/out
CFG
  # Which drop file this "BBS" writes. Machine-local, normally set once with
  # -set-dropfile; the door refuses to start without it.
  echo '{"DropfileFormat":"door32"}' > "$tmp/$id/door.json"
  # Build the world from that config. The door onboards a caller into an
  # existing game; it does not create one.
  "$door" -reset-from-config -data "$tmp/$id" >/dev/null
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
printf ' Asgard\r \r \r0\r' | timeout 15 "$door" -dropfile "$tmp/door32.sys" -data "$tmp/AlphaBBS" >/dev/null 2>&1 || true
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

# ---------------------------------------------------------------------------
# Part 2: the same exchange, routed (#106). Alpha and Charlie have no link to
# each other — Bravo hosts them both, and has to pass Alpha's packet on. Here
# each board's outbound directory IS its neighbour's inbound directory, which
# is what a mailer's per-link filebox amounts to, so there is no transport step
# to run: if Charlie learns about Alpha, Bravo relayed it.
# ---------------------------------------------------------------------------
r="$tmp/routed"
for id in "${boards[@]}"; do
  mkdir -p "$r/$id/in"
done
write_roster() {  # the Coordinator's roster, identical on every board
  cat > "$1" <<'NODES'
1
AlphaBBS
1:1/1
Orlando
FL
USA

2 HOST 1 3
BravoBBS
1:1/2
Houston
TX
USA

3
CharlieBBS
1:1/3
Hull
MA
USA
NODES
}
for id in "${boards[@]}"; do
  cat > "$r/$id/config.json" <<CFG
{"TurnsPerDay":10,"ProtectionTurns":20,"DataDir":"$r/$id","GameLength":30,"IBBS":true}
CFG
  cat > "$r/$id/bbs.cfg" <<CFG
BoardID $id
LeagueNumber 42
Inbound $r/$id/in
CFG
  case "$id" in
    # The leaves send everything to their uplink, whoever it is addressed to.
    AlphaBBS|CharlieBBS) echo "Outbound $r/BravoBBS/in" >> "$r/$id/bbs.cfg" ;;
    # The hub has one link per neighbour.
    BravoBBS)
      echo "Outbound $r/BravoBBS/out" >> "$r/$id/bbs.cfg"
      echo "Link 1 $r/AlphaBBS/in" >> "$r/$id/bbs.cfg"
      echo "Link 3 $r/CharlieBBS/in" >> "$r/$id/bbs.cfg"
      ;;
  esac
  echo '{"DropfileFormat":"door32"}' > "$r/$id/door.json"
  "$door" -reset-from-config -data "$r/$id" >/dev/null
  write_roster "$r/$id/ibnodes.dat"
done

printf ' Asgard\r \r \r0\r' | timeout 15 "$door" -dropfile "$tmp/door32.sys" -data "$r/AlphaBBS" >/dev/null 2>&1 || true
"$door" -planetary -data "$r/AlphaBBS"   # writes into Bravo's inbound
"$door" -planetary -data "$r/BravoBBS"   # applies its own copy, relays Charlie's
"$door" -planetary -data "$r/CharlieBBS"

if grep -q 'AlphaBBS' "$r/CharlieBBS/world.json"; then
  echo "PASS: CharlieBBS learned of AlphaBBS through the hub, with no link between them"
else
  echo "FAIL: the hub did not relay AlphaBBS's packet to CharlieBBS"; fail=1
fi

exit $fail

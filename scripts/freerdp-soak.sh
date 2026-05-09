#!/usr/bin/env bash
set -euo pipefail

SOAK_ITERATIONS="${SOAK_ITERATIONS:-30}"
SOAK_DURATION_SEC="${SOAK_DURATION_SEC:-8}"
SOAK_MODE="${SOAK_MODE:-nla}"
SOAK_ADDR="${SOAK_ADDR:-127.0.0.1:3390}"
SOAK_USERNAME="${SOAK_USERNAME:-runner}"
SOAK_PASSWORD="${SOAK_PASSWORD:-secret}"
SOAK_MAX_RSS_DELTA_KB="${SOAK_MAX_RSS_DELTA_KB:-51200}"
SOAK_ITERATION_TIMEOUT_SEC="${SOAK_ITERATION_TIMEOUT_SEC:-45}"
SOAK_TRACE="${SOAK_TRACE:-0}"

ART_DIR="${SOAK_ARTIFACT_DIR:-soak-artifacts}"
mkdir -p "$ART_DIR/attempts"

mkdir -p .gotmp
GOTMPDIR="$PWD/.gotmp" go build -o "$ART_DIR/mock-server" ./cmd/mock-server

if [[ "$SOAK_TRACE" == "1" ]]; then
  GO_RDP_ANDROID_TRACE=1 "$ART_DIR/mock-server" -addr "$SOAK_ADDR" -test-pattern -width 320 -height 240 -fps 5 -username "$SOAK_USERNAME" -password "$SOAK_PASSWORD" >"$ART_DIR/mock-server.log" 2>&1 &
else
  "$ART_DIR/mock-server" -addr "$SOAK_ADDR" -test-pattern -width 320 -height 240 -fps 5 -username "$SOAK_USERNAME" -password "$SOAK_PASSWORD" >"$ART_DIR/mock-server.log" 2>&1 &
fi
SERVER_PID=$!

cleanup() {
  kill "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT

for _ in $(seq 1 80); do
  if grep -q 'listening on' "$ART_DIR/mock-server.log"; then
    break
  fi
  sleep 0.25
done
grep -q 'listening on' "$ART_DIR/mock-server.log"

CSV="$ART_DIR/soak.csv"
echo "iteration,mode,exit_code,rss_kb" > "$CSV"

for i in $(seq 1 "$SOAK_ITERATIONS"); do
  ATT_DIR="$ART_DIR/attempts/$i"
  mkdir -p "$ATT_DIR"

  set +e
  timeout --signal=TERM --kill-after=10 "${SOAK_ITERATION_TIMEOUT_SEC}s" xvfb-run -a bash -c "
    set +e
    xfreerdp /v:${SOAK_ADDR} /sec:${SOAK_MODE} /cert:ignore /u:${SOAK_USERNAME} /p:${SOAK_PASSWORD} /log-level:WARN >'${ATT_DIR}/xfreerdp.log' 2>&1 &
    pid=\$!
    sleep ${SOAK_DURATION_SEC}
    if kill -0 \$pid >/dev/null 2>&1; then
      kill -INT \$pid >/dev/null 2>&1 || true
      for _ in \$(seq 1 20); do
        if ! kill -0 \$pid >/dev/null 2>&1; then
          break
        fi
        sleep 0.25
      done
      if kill -0 \$pid >/dev/null 2>&1; then
        kill -TERM \$pid >/dev/null 2>&1 || true
        sleep 1
      fi
      if kill -0 \$pid >/dev/null 2>&1; then
        kill -KILL \$pid >/dev/null 2>&1 || true
      fi
    fi
    wait \$pid
    echo \$? >'${ATT_DIR}/exit_code.txt'
  "
  XVFB_STATUS=$?
  set -e

  if [[ "$XVFB_STATUS" -eq 124 ]]; then
    echo "iteration ${i} exceeded timeout ${SOAK_ITERATION_TIMEOUT_SEC}s" > "$ATT_DIR/timeout.txt"
    EXIT_CODE="124"
  else
    EXIT_CODE="$(cat "$ATT_DIR/exit_code.txt" 2>/dev/null || echo "$XVFB_STATUS")"
  fi

  RSS_KB="$(awk '/VmRSS:/ {print $2}' /proc/${SERVER_PID}/status 2>/dev/null || echo 0)"

  echo "$i,$SOAK_MODE,$EXIT_CODE,$RSS_KB" >> "$CSV"

done

MIN_RSS="$(awk -F, 'NR==2{min=$4} NR>2 && $4<min{min=$4} END{print min+0}' "$CSV")"
MAX_RSS="$(awk -F, 'NR==2{max=$4} NR>2 && $4>max{max=$4} END{print max+0}' "$CSV")"
DELTA_RSS=$((MAX_RSS - MIN_RSS))

PASS="true"
if (( DELTA_RSS > SOAK_MAX_RSS_DELTA_KB )); then
  PASS="false"
fi

SUMMARY="$ART_DIR/summary.md"
{
  echo "# FreeRDP soak summary"
  echo
  echo "- mode: $SOAK_MODE"
  echo "- iterations: $SOAK_ITERATIONS"
  echo "- duration per iteration (sec): $SOAK_DURATION_SEC"
  echo "- rss min (KB): $MIN_RSS"
  echo "- rss max (KB): $MAX_RSS"
  echo "- rss delta (KB): $DELTA_RSS"
  echo "- rss delta threshold (KB): $SOAK_MAX_RSS_DELTA_KB"
  echo "- iteration timeout (sec): $SOAK_ITERATION_TIMEOUT_SEC"
  echo "- trace enabled: $SOAK_TRACE"
  echo "- pass: $PASS"
} > "$SUMMARY"

if [[ "$PASS" != "true" ]]; then
  echo "RSS delta exceeded threshold: $DELTA_RSS > $SOAK_MAX_RSS_DELTA_KB" >&2
  exit 1
fi

echo "Soak complete: $SUMMARY"

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-$ROOT/encoding-matrix-artifacts}"
XFREERDP="${XFREERDP:-$(command -v xfreerdp3 || command -v xfreerdp || true)}"
XVFB="${XVFB:-$(command -v Xvfb || true)}"
DISPLAY_NUM="${ENCODING_MATRIX_DISPLAY:-:98}"
SERVER_PID=""
XVFB_PID=""

if [[ -z "$XFREERDP" ]]; then
  echo "xfreerdp3/xfreerdp not found; install freerdp3-x11 or equivalent" >&2
  exit 127
fi
if [[ -z "$XVFB" ]]; then
  echo "Xvfb not found; install xvfb" >&2
  exit 127
fi

cleanup() {
  if [[ -n "$SERVER_PID" ]]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  if [[ -n "$XVFB_PID" ]]; then
    kill "$XVFB_PID" 2>/dev/null || true
    wait "$XVFB_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

mkdir -p "$OUT" "$ROOT/.gotmp"
cd "$ROOT"
GOTMPDIR="$ROOT/.gotmp" go build -o "$OUT/mock-server" ./cmd/mock-server

"$XVFB" "$DISPLAY_NUM" -screen 0 1280x800x24 >"$OUT/xvfb.log" 2>&1 &
XVFB_PID=$!
sleep 1

run_case() {
  local label="$1"
  local server_env="$2"
  local server_args="$3"
  local client_args="$4"
  local dir="$OUT/$label"
  mkdir -p "$dir"
  SERVER_PID=""
  echo "== $label =="
  # shellcheck disable=SC2086
  env GO_RDP_ANDROID_TRACE=1 $server_env "$OUT/mock-server" $server_args -width 320 -height 240 -fps 5 -username runner -password secret >"$dir/mock-server.log" 2>&1 &
  SERVER_PID=$!
  for _ in $(seq 1 80); do
    if grep -q 'listening on' "$dir/mock-server.log"; then
      break
    fi
    sleep 0.1
  done
  if ! grep -q 'listening on' "$dir/mock-server.log"; then
    echo "server failed to listen for $label" >&2
    return 1
  fi

  set +e
  # shellcheck disable=SC2086
  DISPLAY="$DISPLAY_NUM" timeout --preserve-status 12s "$XFREERDP" /v:127.0.0.1:3390 /u:runner /p:secret /cert:ignore /log-level:TRACE $client_args >"$dir/xfreerdp.log" 2>&1 &
  local client_pid=$!
  sleep 6
  DISPLAY="$DISPLAY_NUM" xwd -root -silent -out "$dir/xfreerdp-root.xwd" 2>/dev/null
  wait "$client_pid"
  local exit_code=$?
  set -e
  echo "$exit_code" >"$dir/exit-code.txt"
  kill -INT "$SERVER_PID" 2>/dev/null || true
  sleep 0.5
  kill "$SERVER_PID" 2>/dev/null || true
  wait "$SERVER_PID" 2>/dev/null || true
  SERVER_PID=""
  GOTMPDIR="$ROOT/.gotmp" go run ./scripts/summarize-freerdp.go "$dir" >/dev/null
}

run_case bitmap 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1' '-test-pattern' '/sec:nla /bpp:24'
run_case rdpgfx-planar 'GO_RDP_ANDROID_DISABLE_H264=1' '-test-pattern' '/sec:nla /gfx'
printf '\x00\x00\x00\x01\x67\x42\x00\x1f\x00\x00\x00\x01\x68\xce\x06\xe2\x00\x00\x00\x01\x65\x88\x84' >"$OUT/h264-idr.h264"
run_case h264-avc420-forced 'GO_RDP_ANDROID_FORCE_H264=1' "-test-pattern -h264-file $OUT/h264-idr.h264 -h264-fps 5" '/sec:nla /gfx:AVC420'
run_case h264-forced-gfx-fallback 'GO_RDP_ANDROID_FORCE_H264=1' "-test-pattern -h264-file $OUT/h264-idr.h264 -h264-fps 5" '/sec:nla /gfx'

cat >"$OUT/summary.md" <<SUMMARY
# RDP encoding matrix

Generated: $(date -Is)
FreeRDP: $("$XFREERDP" /version 2>/dev/null | head -1)
Server: cmd/mock-server test pattern, NLA credentials runner/secret

| Case | Exit | Active | Bitmap | RDPGFX | H.264 reason | H.264 writes | H.264 bytes |
| --- | ---: | --- | --- | --- | --- | ---: | ---: |
SUMMARY
python3 - "$OUT" >>"$OUT/summary.md" <<'PY'
import json, pathlib, sys
base = pathlib.Path(sys.argv[1])
for label in ["bitmap", "rdpgfx-planar", "h264-avc420-forced", "h264-forced-gfx-fallback"]:
    s = json.load(open(base / label / "summary.json"))
    print(f"| {label} | {s.get('exit_code')} | {s.get('active_seen')} | {s.get('bitmap_seen')} | {s.get('rdpgfx_seen')} | {s.get('h264_reason','')} | {s.get('h264_write_count',0)} | {s.get('h264_write_bytes',0)} |")
PY
cat >>"$OUT/summary.md" <<'SUMMARY'

## Interpretation

- Bitmap fallback should show active streaming with `bitmap_seen=true` and no RDPGFX.
- RDPGFX Planar should show active streaming with `rdpgfx_seen=true` and no H.264 writes when H.264 is disabled.
- H.264 AVC420 cases are force-mode protocol smoke tests. They prove server/client handling of emitted AVC420 payloads with this FreeRDP build, but do not prove negotiated release compatibility.
SUMMARY

cat "$OUT/summary.md"

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-$ROOT/encoding-matrix-artifacts}"
XFREERDP="${XFREERDP:-$(command -v xfreerdp3 || command -v xfreerdp || true)}"
XVFB="${XVFB:-$(command -v Xvfb || true)}"
XWD="${XWD:-$(command -v xwd || true)}"
PYTHON="${PYTHON:-$(command -v python3 || true)}"
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
if [[ -z "$XWD" ]]; then
  echo "xwd not found; install x11-apps or equivalent" >&2
  exit 127
fi
if [[ -z "$PYTHON" ]]; then
  echo "python3 not found; install Python 3" >&2
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
  DISPLAY="$DISPLAY_NUM" "$XWD" -root -silent -out "$dir/xfreerdp-root.xwd" 2>/dev/null
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
run_case bitmap-rle 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_BITMAP_RLE=1' '' '/sec:nla /bpp:24'
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
"$PYTHON" - "$OUT" >>"$OUT/summary.md" <<'PY'
import json, pathlib, sys
base = pathlib.Path(sys.argv[1])
for label in ["bitmap", "bitmap-rle", "rdpgfx-planar", "h264-avc420-forced", "h264-forced-gfx-fallback"]:
    s = json.load(open(base / label / "summary.json"))
    print(f"| {label} | {s.get('exit_code')} | {s.get('active_seen')} | {s.get('bitmap_seen')} | {s.get('rdpgfx_seen')} | {s.get('h264_reason','')} | {s.get('h264_write_count',0)} | {s.get('h264_write_bytes',0)} |")
PY
"$PYTHON" - "$OUT" <<'PY'
import json, pathlib, sys
base = pathlib.Path(sys.argv[1])
failures = []
def load(label):
    return json.load(open(base / label / "summary.json"))
bitmap = load("bitmap")
if not bitmap.get("active_seen") or not bitmap.get("bitmap_seen") or bitmap.get("rdpgfx_seen"):
    failures.append("bitmap fallback did not produce active bitmap-only evidence")
bitmap_rle = load("bitmap-rle")
if not bitmap_rle.get("active_seen") or not bitmap_rle.get("bitmap_seen") or bitmap_rle.get("rdpgfx_seen"):
    failures.append("bitmap RLE did not produce active bitmap-only evidence")
bitmap_rle_log = (base / "bitmap-rle" / "mock-server.log").read_text(errors="replace")
if "bitmap_rle_" not in bitmap_rle_log:
    failures.append("bitmap RLE case did not emit bitmap_rle trace evidence")
planar = load("rdpgfx-planar")
if not planar.get("active_seen") or not planar.get("rdpgfx_seen") or planar.get("h264_write_count", 0) != 0:
    failures.append("RDPGFX Planar did not produce active RDPGFX evidence without H.264 writes")
for label in ["h264-avc420-forced", "h264-forced-gfx-fallback"]:
    s = load(label)
    if not s.get("active_seen") or not s.get("rdpgfx_seen") or s.get("h264_reason") != "forced-by-env" or s.get("h264_write_count", 0) <= 0 or s.get("h264_write_bytes", 0) <= 0:
        failures.append(f"{label} did not produce active forced H.264 evidence")
if failures:
    for failure in failures:
        print(f"encoding matrix failure: {failure}", file=sys.stderr)
    raise SystemExit(1)
PY
cat >>"$OUT/summary.md" <<'SUMMARY'

## Interpretation

- Bitmap fallback should show active streaming with `bitmap_seen=true` and no RDPGFX.
- RDPGFX Planar should show active streaming with `rdpgfx_seen=true` and no H.264 writes when H.264 is disabled.
- H.264 AVC420 cases are force-mode protocol smoke tests. They prove server/client handling of emitted AVC420 payloads with this FreeRDP build, but do not prove negotiated release compatibility.

## Observed RDPGFX capability advertisements

SUMMARY
"$PYTHON" - "$OUT" >>"$OUT/summary.md" <<'PY'
import pathlib, re, sys
base = pathlib.Path(sys.argv[1])
pattern = re.compile(r"rdpgfx_cap .*index=(\d+) version=(0x[0-9a-fA-F]+).*flags=(0x[0-9a-fA-F]+) supported=(\w+)")
def flag_notes(flags):
    value = int(flags, 16)
    notes = []
    if value & 0x10:
        notes.append("AVC420_ENABLED")
    if value & 0x20:
        notes.append("AVC_DISABLED")
    return "/" + "+".join(notes) if notes else ""
for label in ["rdpgfx-planar", "h264-avc420-forced", "h264-forced-gfx-fallback"]:
    log = base / label / "mock-server.log"
    if not log.exists():
        continue
    caps = []
    for line in log.read_text(errors="replace").splitlines():
        m = pattern.search(line)
        if m:
            caps.append(m.groups())
    if not caps:
        print(f"- {label}: no RDPGFX capability traces found")
        continue
    joined = ", ".join(f"{version}/flags={flags}{flag_notes(flags)}/supported={supported}" for _, version, flags, supported in caps)
    print(f"- {label}: {joined}")
PY
cat >>"$OUT/summary.md" <<'SUMMARY'

## Encoding families not implemented by this server yet

These are tracked explicitly so the matrix does not imply full RDP graphics-codec coverage:

| Encoding family | Matrix status | Rationale |
| --- | --- | --- |
| RDP 5/6 bitmap compression / bitmap RLE | Scaffold only | 24-bpp COPY-order encoder has unit coverage, but runtime negotiation/emission is not enabled yet. |
| NSCodec | Not implemented | Useful for some non-GFX clients; needs capability parsing plus encoder implementation before it can be tested. |
| RemoteFX / RFX | Not implemented | Deprecated/disabled in many clients; only implement if compatibility evidence justifies it. |
| RDPGFX AVC444 / AVC444v2 | Not implemented | Higher-fidelity H.264 variants; defer until AVC420 negotiation/client proof exists. |
| RDPGFX ClearCodec | Not implemented | Text/graphics optimized codec; defer behind Planar and AVC420. |
| RDPGFX Progressive / other progressive codecs | Not implemented | More complex progressive pipeline; not first-APK scope. |
| JPEG/PNG bitmap codecs | Not implemented | No current server output path; add only if client capabilities and performance data justify it. |
SUMMARY

cat >"$OUT/codec-coverage.json" <<'JSON'
{
  "implemented": [
    {"name":"slow-path raw bitmap", "status":"implemented", "matrix_case":"bitmap"},
    {"name":"RDP 5/6 bitmap compression / bitmap RLE", "status":"experimental-opt-in", "matrix_case":"bitmap-rle", "toggle":"GO_RDP_ANDROID_ENABLE_BITMAP_RLE=1"},
    {"name":"RDPGFX Planar", "status":"implemented", "matrix_case":"rdpgfx-planar"},
    {"name":"RDPGFX AVC420 / H.264", "status":"experimental-force-mode", "matrix_cases":["h264-avc420-forced", "h264-forced-gfx-fallback"]}
  ],
  "missing": [
    {"name":"NSCodec", "priority":"evidence-gated"},
    {"name":"RemoteFX / RFX", "priority":"deferred"},
    {"name":"RDPGFX AVC444 / AVC444v2", "priority":"deferred-until-avc420-proof"},
    {"name":"RDPGFX ClearCodec", "priority":"deferred"},
    {"name":"RDPGFX Progressive / other progressive codecs", "priority":"deferred"},
    {"name":"JPEG/PNG bitmap codecs", "priority":"evidence-gated"}
  ]
}
JSON

cat "$OUT/summary.md"

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
run_case bitmap-planar 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR=1' '-test-pattern' '/sec:nla /bpp:32'
run_case bitmap-16bpp 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_BITMAP_BPP=16' '-test-pattern' '/sec:nla /bpp:16'
run_case bitmap-15bpp 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_BITMAP_BPP=15' '-test-pattern' '/sec:nla /bpp:15'
run_case bitmap-8bpp 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_BITMAP_BPP=8' '-test-pattern' '/sec:nla /bpp:8'
run_case nscodec-opt-in 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_NSCODEC=1' '-test-pattern' '/sec:nla /bpp:24'
run_case jpeg-opt-in 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_JPEG_CODEC=1 GO_RDP_ANDROID_JPEG_QUALITY=80' '-test-pattern' '/sec:nla /bpp:24'
run_case png-opt-in 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID=9 GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL=-3' '-test-pattern' '/sec:nla /bpp:24'
run_case rfx-encoded 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_RFX_CODEC=1' '-test-pattern' '/sec:nla /bpp:24'
printf '\x01\x02\x03\x04' >"$OUT/codec-fixture.bin"
run_case rfx-fixture 'GO_RDP_ANDROID_DISABLE_RDPGFX=1 GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_RFX_CODEC=1' "-test-pattern -rfx-file $OUT/codec-fixture.bin" '/sec:nla /bpp:24'
run_case rdpgfx-planar 'GO_RDP_ANDROID_DISABLE_H264=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-planar-stream 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_RDPGFX_STREAM=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-uncompressed 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_RDPGFX_UNCOMPRESSED=1 GO_RDP_ANDROID_ENABLE_RDPGFX_STREAM=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-deferred-codecs 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_CLEARCODEC=1 GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC=1 GO_RDP_ANDROID_ENABLE_AVC444=1 GO_RDP_ANDROID_ENABLE_AVC444V2=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-clearcodec-encoded 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_CLEARCODEC=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-clearcodec-fixture 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_CLEARCODEC=1' "-test-pattern -clearcodec-file $OUT/codec-fixture.bin" '/sec:nla /gfx'
run_case rdpgfx-progressive-encoded 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-progressivev2-encoded 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-progressive-fixture 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC=1' "-test-pattern -progressive-file $OUT/codec-fixture.bin" '/sec:nla /gfx'
run_case rdpgfx-avc444-encoded 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_AVC444=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-avc444-fixture 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_AVC444=1' "-test-pattern -avc444-file $OUT/codec-fixture.bin" '/sec:nla /gfx'
run_case rdpgfx-avc444v2-encoded 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_AVC444V2=1' '-test-pattern' '/sec:nla /gfx'
run_case rdpgfx-avc444v2-fixture 'GO_RDP_ANDROID_DISABLE_H264=1 GO_RDP_ANDROID_ENABLE_AVC444V2=1' "-test-pattern -avc444v2-file $OUT/codec-fixture.bin" '/sec:nla /gfx'
printf '\x00\x00\x00\x01\x67\x42\x00\x1f\x00\x00\x00\x01\x68\xce\x06\xe2\x00\x00\x00\x01\x65\x88\x84' >"$OUT/h264-idr.h264"
run_case h264-negotiated-gfx '' '-test-pattern' '/sec:nla /gfx'
run_case h264-avc420-forced 'GO_RDP_ANDROID_FORCE_H264=1' "-test-pattern -h264-file $OUT/h264-idr.h264 -h264-fps 5" '/sec:nla /gfx:AVC420'
run_case h264-forced-gfx-fallback 'GO_RDP_ANDROID_FORCE_H264=1' "-test-pattern -h264-file $OUT/h264-idr.h264 -h264-fps 5" '/sec:nla /gfx'

cat >"$OUT/summary.md" <<SUMMARY
# RDP encoding matrix

Generated: $(date -Is)
FreeRDP: $("$XFREERDP" /version 2>/dev/null | head -1)
Server: cmd/mock-server test pattern, NLA credentials runner/secret

| Case | Exit | Active | Bitmap | Bitmap RLE | RLE saved bytes | Bitmap Planar | Planar saved % | 8bpp bitmap | Palette | 15bpp bitmap | 16bpp bitmap | NSCodec selected | NSCodec writes | NSCodec raw | NSCodec saved | NSCodec saved % | JPEG selected | JPEG writes | JPEG raw | JPEG saved | JPEG saved % | PNG raw | PNG saved | PNG saved % | Bitmap codec stream stops | RFX selected | RFX writes | RFX saved % | RDPGFX | GFX writes | GFX stream stops | Uncompressed GFX | Deferred GFX codecs | Hook GFX writes | H.264 reason | H.264 writes | H.264 bytes |
| --- | ---: | --- | --- | --- | ---: | --- | ---: | --- | --- | --- | --- | ---: | --- | ---: | ---: | ---: | ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | ---: | ---: | --- | ---: | ---: | --- | ---: | ---: |
SUMMARY
"$PYTHON" - "$OUT" >>"$OUT/summary.md" <<'PY'
import json, pathlib, sys
base = pathlib.Path(sys.argv[1])
for label in ["bitmap", "bitmap-rle", "bitmap-planar", "bitmap-16bpp", "bitmap-15bpp", "bitmap-8bpp", "nscodec-opt-in", "jpeg-opt-in", "png-opt-in", "rfx-encoded", "rfx-fixture", "rdpgfx-planar", "rdpgfx-planar-stream", "rdpgfx-uncompressed", "rdpgfx-deferred-codecs", "rdpgfx-clearcodec-encoded", "rdpgfx-clearcodec-fixture", "rdpgfx-progressive-encoded", "rdpgfx-progressivev2-encoded", "rdpgfx-progressive-fixture", "rdpgfx-avc444-encoded", "rdpgfx-avc444-fixture", "rdpgfx-avc444v2-encoded", "rdpgfx-avc444v2-fixture", "h264-negotiated-gfx", "h264-avc420-forced", "h264-forced-gfx-fallback"]:
    s = json.load(open(base / label / "summary.json"))
    deferred = sum(1 for key in ["rdpgfx_clearcodec_selected", "rdpgfx_progressive_selected", "rdpgfx_progressive_v2_selected", "rdpgfx_avc444_selected", "rdpgfx_avc444v2_selected"] if s.get(key))
    print(f"| {label} | {s.get('exit_code')} | {s.get('active_seen')} | {s.get('bitmap_seen')} | {s.get('bitmap_rle_seen', False)} | {s.get('bitmap_rle_saved_bytes',0)} | {s.get('bitmap_planar_seen', False)} | {s.get('bitmap_planar_saved_percent',0):.1f} | {s.get('bitmap_bpp8_seen', False)} | {s.get('palette_seen', False)} | {s.get('bitmap_bpp15_seen', False)} | {s.get('bitmap_bpp16_seen', False)} | {s.get('nscodec_selected', False)} | {s.get('nscodec_write_count',0)} | {s.get('nscodec_raw_bytes',0)} | {s.get('nscodec_saved_bytes',0)} | {s.get('nscodec_saved_percent',0):.1f} | {s.get('jpeg_codec_selected', False)} | {s.get('jpeg_codec_write_count',0)} | {s.get('jpeg_codec_raw_bytes',0)} | {s.get('jpeg_codec_saved_bytes',0)} | {s.get('jpeg_codec_saved_percent',0):.1f} | {s.get('png_codec_raw_bytes',0)} | {s.get('png_codec_saved_bytes',0)} | {s.get('png_codec_saved_percent',0):.1f} | {s.get('bitmap_codec_stream_stop_count',0)} | {s.get('rfx_codec_selected', False)} | {s.get('rfx_codec_write_count',0)} | {s.get('rfx_codec_saved_percent',0):.1f} | {s.get('rdpgfx_seen')} | {s.get('rdpgfx_frame_write_count',0)} | {s.get('rdpgfx_frame_stream_stop_count',0)} | {s.get('rdpgfx_uncompressed_selected', False)} | {deferred} | {s.get('rdpgfx_clearcodec_write_count',0)+s.get('rdpgfx_progressive_write_count',0)+s.get('rdpgfx_progressive_v2_write_count',0)+s.get('rdpgfx_avc444_write_count',0)+s.get('rdpgfx_avc444v2_write_count',0)} | {s.get('h264_reason','')} | {s.get('h264_write_count',0)} | {s.get('h264_write_bytes',0)} |")
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
if not bitmap_rle.get("bitmap_rle_seen") or bitmap_rle.get("bitmap_rle_count", 0) <= 0 or bitmap_rle.get("bitmap_rle_saved_bytes", 0) <= 0:
    failures.append("bitmap RLE case did not emit shrinking bitmap_rle trace evidence")
bitmap_planar = load("bitmap-planar")
if not bitmap_planar.get("active_seen") or not bitmap_planar.get("bitmap_seen") or bitmap_planar.get("rdpgfx_seen"):
    failures.append("classic bitmap Planar did not produce active bitmap-only evidence")
if not bitmap_planar.get("bitmap_planar_seen") or bitmap_planar.get("bitmap_planar_count", 0) <= 0 or bitmap_planar.get("bitmap_planar_saved_bytes", 0) <= 0:
    failures.append("classic bitmap Planar case did not emit shrinking bitmap_planar trace evidence")
bitmap_16bpp = load("bitmap-16bpp")
if not bitmap_16bpp.get("active_seen") or not bitmap_16bpp.get("bitmap_seen") or bitmap_16bpp.get("rdpgfx_seen"):
    failures.append("16bpp bitmap case did not produce active bitmap-only evidence")
if not bitmap_16bpp.get("bitmap_bpp16_seen"):
    failures.append("16bpp bitmap case did not emit bpp=16 tile evidence")
bitmap_15bpp = load("bitmap-15bpp")
if bitmap_15bpp.get("active_seen"):
    if not bitmap_15bpp.get("bitmap_seen") or bitmap_15bpp.get("rdpgfx_seen"):
        failures.append("15bpp bitmap case reached active state without bitmap-only evidence")
    if not bitmap_15bpp.get("bitmap_bpp15_seen"):
        failures.append("15bpp bitmap case reached active state but did not emit bpp=15 tile evidence")
bitmap_8bpp = load("bitmap-8bpp")
if not bitmap_8bpp.get("active_seen") or not bitmap_8bpp.get("bitmap_seen") or bitmap_8bpp.get("rdpgfx_seen"):
    failures.append("8bpp bitmap case did not produce active bitmap-only evidence")
if not bitmap_8bpp.get("bitmap_bpp8_seen") or not bitmap_8bpp.get("palette_seen"):
    failures.append("8bpp bitmap case did not emit bpp=8 tile and palette evidence")
nscodec = load("nscodec-opt-in")
if not nscodec.get("active_seen"):
    failures.append("NSCodec opt-in case did not reach active state")
if nscodec.get("nscodec_selected") and (not nscodec.get("nscodec_write_seen") or nscodec.get("nscodec_write_count", 0) <= 0 or nscodec.get("nscodec_write_bytes", 0) <= 0):
    failures.append("NSCodec opt-in selected but did not emit write evidence")
jpeg_codec = load("jpeg-opt-in")
if not jpeg_codec.get("active_seen"):
    failures.append("JPEG opt-in case did not reach active state")
if jpeg_codec.get("jpeg_codec_selected") and (not jpeg_codec.get("jpeg_codec_write_seen") or jpeg_codec.get("jpeg_codec_write_count", 0) <= 0 or jpeg_codec.get("jpeg_codec_write_bytes", 0) <= 0):
    failures.append("JPEG opt-in selected but did not emit write evidence")
png_codec = load("png-opt-in")
if not png_codec.get("active_seen"):
    failures.append("PNG opt-in case did not reach active state")
if png_codec.get("png_codec_selected") and (not png_codec.get("png_codec_write_seen") or png_codec.get("png_codec_write_count", 0) <= 0 or png_codec.get("png_codec_write_bytes", 0) <= 0):
    failures.append("PNG opt-in selected but did not emit write evidence")
rfx_encoded = load("rfx-encoded")
if not rfx_encoded.get("active_seen"):
    failures.append("RemoteFX production-encoded case did not reach active state")
if rfx_encoded.get("rfx_codec_selected") and rfx_encoded.get("rfx_codec_write_count", 0) <= 0:
    failures.append("RemoteFX production-encoded case selected codec but did not emit write evidence")
rfx_fixture = load("rfx-fixture")
if not rfx_fixture.get("active_seen"):
    failures.append("RemoteFX fixture case did not reach active state")
if rfx_fixture.get("rfx_codec_selected") and rfx_fixture.get("rfx_codec_write_count", 0) <= 0:
    failures.append("RemoteFX fixture case selected codec but did not emit write evidence")
planar = load("rdpgfx-planar")
if not planar.get("active_seen") or not planar.get("rdpgfx_seen") or planar.get("h264_write_count", 0) != 0:
    failures.append("RDPGFX Planar did not produce active RDPGFX evidence without H.264 writes")
planar_stream = load("rdpgfx-planar-stream")
if not planar_stream.get("active_seen") or not planar_stream.get("rdpgfx_seen") or planar_stream.get("h264_write_count", 0) != 0:
    failures.append("RDPGFX Planar stream probe did not produce active RDPGFX evidence without H.264 writes")
uncompressed_gfx = load("rdpgfx-uncompressed")
if not uncompressed_gfx.get("active_seen") or not uncompressed_gfx.get("rdpgfx_seen") or not uncompressed_gfx.get("rdpgfx_uncompressed_selected"):
    failures.append("RDPGFX uncompressed probe did not produce active uncompressed RDPGFX evidence")
deferred_gfx = load("rdpgfx-deferred-codecs")
if not deferred_gfx.get("active_seen") or not deferred_gfx.get("rdpgfx_seen"):
    failures.append("RDPGFX deferred-codec probe did not produce active RDPGFX evidence")

clearcodec_encoded = load("rdpgfx-clearcodec-encoded")
if not clearcodec_encoded.get("active_seen") or not clearcodec_encoded.get("rdpgfx_seen"):
    failures.append("ClearCodec production case did not produce active RDPGFX evidence")
clearcodec_fixture = load("rdpgfx-clearcodec-fixture")
if not clearcodec_fixture.get("active_seen") or not clearcodec_fixture.get("rdpgfx_seen"):
    failures.append("ClearCodec fixture case did not produce active RDPGFX evidence")

progressive_encoded = load("rdpgfx-progressive-encoded")
if not progressive_encoded.get("active_seen") or not progressive_encoded.get("rdpgfx_seen"):
    failures.append("Progressive production case did not produce active RDPGFX evidence")

progressive_v2_encoded = load("rdpgfx-progressivev2-encoded")
if not progressive_v2_encoded.get("active_seen") or not progressive_v2_encoded.get("rdpgfx_seen"):
    failures.append("ProgressiveV2 production case did not produce active RDPGFX evidence")
if progressive_v2_encoded.get("rdpgfx_progressive_v2_selected") and progressive_v2_encoded.get("rdpgfx_progressive_v2_write_count", 0) <= 0:
    failures.append("ProgressiveV2 production case selected codec but did not emit write evidence")

progressive_fixture = load("rdpgfx-progressive-fixture")
if not progressive_fixture.get("active_seen") or not progressive_fixture.get("rdpgfx_seen"):
    failures.append("Progressive fixture case did not produce active RDPGFX evidence")

avc444_encoded = load("rdpgfx-avc444-encoded")
if not avc444_encoded.get("active_seen") or not avc444_encoded.get("rdpgfx_seen"):
    failures.append("AVC444 production case did not produce active RDPGFX evidence")

avc444_fixture = load("rdpgfx-avc444-fixture")
if not avc444_fixture.get("active_seen") or not avc444_fixture.get("rdpgfx_seen"):
    failures.append("AVC444 fixture case did not produce active RDPGFX evidence")
if avc444_fixture.get("rdpgfx_avc444_selected") and avc444_fixture.get("rdpgfx_avc444_write_count", 0) <= 0:
    failures.append("AVC444 fixture case selected codec but did not emit write evidence")

avc444v2_encoded = load("rdpgfx-avc444v2-encoded")
if not avc444v2_encoded.get("active_seen") or not avc444v2_encoded.get("rdpgfx_seen"):
    failures.append("AVC444v2 production case did not produce active RDPGFX evidence")

avc444v2_fixture = load("rdpgfx-avc444v2-fixture")
if not avc444v2_fixture.get("active_seen") or not avc444v2_fixture.get("rdpgfx_seen"):
    failures.append("AVC444v2 fixture case did not produce active RDPGFX evidence")
if avc444v2_fixture.get("rdpgfx_avc444v2_selected") and avc444v2_fixture.get("rdpgfx_avc444v2_write_count", 0) <= 0:
    failures.append("AVC444v2 fixture case selected codec but did not emit write evidence")
negotiated_h264 = load("h264-negotiated-gfx")
if not negotiated_h264.get("active_seen") or not negotiated_h264.get("rdpgfx_seen") or negotiated_h264.get("h264_write_count", 0) != 0 or negotiated_h264.get("h264_reason") in ["", "forced-by-env"]:
    failures.append("negotiated H.264 probe did not produce active gated no-write evidence")
fallback_forced = load("h264-forced-gfx-fallback")
if not fallback_forced.get("active_seen") or not fallback_forced.get("rdpgfx_seen") or fallback_forced.get("h264_reason") != "forced-by-env" or fallback_forced.get("h264_write_count", 0) <= 0 or fallback_forced.get("h264_write_bytes", 0) <= 0:
    failures.append("h264-forced-gfx-fallback did not produce active forced H.264 evidence")

avc420_forced = load("h264-avc420-forced")
if avc420_forced.get("active_seen"):
    if not avc420_forced.get("rdpgfx_seen") or avc420_forced.get("h264_reason") != "forced-by-env" or avc420_forced.get("h264_write_count", 0) <= 0 or avc420_forced.get("h264_write_bytes", 0) <= 0:
        failures.append("h264-avc420-forced reached active state but did not produce forced H.264 write evidence")
if failures:
    for failure in failures:
        print(f"encoding matrix failure: {failure}", file=sys.stderr)
    raise SystemExit(1)
PY
cat >>"$OUT/summary.md" <<'SUMMARY'

## Interpretation

- Bitmap fallback should show active streaming with `bitmap_seen=true` and no RDPGFX.
- Bitmap RLE should show active bitmap streaming plus `bitmap_rle_seen=true`; it remains opt-in via `GO_RDP_ANDROID_ENABLE_BITMAP_RLE=1`.
- Classic bitmap Planar should show active bitmap streaming plus `bitmap_planar_seen=true`; it remains opt-in via `GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR=1` and is distinct from RDPGFX Planar.
- 16bpp bitmap should show active bitmap streaming plus `bitmap_bpp16_seen=true`; it uses `GO_RDP_ANDROID_ENABLE_BITMAP_BPP=16` to prove the lower-depth encoder path separately from the default 24bpp fallback.
- 15bpp bitmap uses `GO_RDP_ANDROID_ENABLE_BITMAP_BPP=15` to probe the RGB555 encoder path separately from the default 24bpp fallback. Some FreeRDP builds reject `/bpp:15`; when it reaches active state, the matrix requires `bitmap_bpp15_seen=true`.
- 8bpp bitmap should show active bitmap streaming plus `bitmap_bpp8_seen=true` and `palette_seen=true`; it uses `GO_RDP_ANDROID_ENABLE_BITMAP_BPP=8` to prove the paletted encoder path separately from the default 24bpp fallback.
- NSCodec opt-in should at least reach active state. If the client advertises NSCodec, the summary should show `nscodec_selected=true` and positive write evidence; otherwise it documents client capability absence without failing the matrix.
- JPEG opt-in should at least reach active state. If the client advertises JPEG in Bitmap Codecs, the summary should show `jpeg_codec_selected=true` and positive write evidence; otherwise it documents client capability absence without failing the matrix.
- PNG opt-in should at least reach active state. It uses an operator-supplied codec ID for client-specific experiments; if selected, the summary should show `png_codec_selected=true` and positive write evidence.
- RemoteFX production-encoded (`rfx-encoded`) should reach active state. When the client advertises RemoteFX, it should show `rfx_codec_selected=true` plus positive write/raw/saved evidence.
- RemoteFX fixture (`rfx-fixture`) should also reach active state; when selected, it should show write evidence, keeping fixture-vs-production transport paths distinct.
- RDPGFX Planar should show active streaming with `rdpgfx_seen=true` and no H.264 writes when H.264 is disabled.
- RDPGFX Planar stream probe enables `GO_RDP_ANDROID_ENABLE_RDPGFX_STREAM=1` while keeping Planar encoding and no H.264 writes; `GFX stream stops` records whether the client closed the graphics DVC after the first frame.
- RDPGFX uncompressed probe enables `GO_RDP_ANDROID_ENABLE_RDPGFX_UNCOMPRESSED=1` and should show `rdpgfx_uncompressed_selected=true` while remaining diagnostic-only.
- RDPGFX deferred-codec probe enables ClearCodec, Progressive, AVC444, and AVC444v2 selection traces while still emitting safe Planar frames; selected count depends on negotiated RDPGFX version/flags.
- ClearCodec now has both a production-encode case (`rdpgfx-clearcodec-encoded`) and a fixture-hook case (`rdpgfx-clearcodec-fixture`) so matrix evidence distinguishes real encoder output from transport-hook output; write counters are recorded when the client/timing accepts the path, but active RDPGFX evidence is the hard gate.
- Progressive now has production-path cases (`rdpgfx-progressive-encoded`, `rdpgfx-progressivev2-encoded`) and a fixture-hook case (`rdpgfx-progressive-fixture`) so matrix evidence distinguishes V1/V2 production selection from transport-hook output; write counters are recorded when the client/timing accepts the path, but active RDPGFX evidence is the hard gate.
- AVC444 and AVC444v2 now have production-path cases (`rdpgfx-avc444-encoded`, `rdpgfx-avc444v2-encoded`) plus fixture-hook cases. Production cases use the built-in bounded base/aux payload encoders; fixture cases remain transport-hook smoke tests only. Write counters are recorded when selected paths emit.
- H.264 negotiated probe keeps H.264 enabled without force and should show active RDPGFX plus no H.264 writes unless a real client advertises AVC420.
- H.264 force-mode protocol smoke tests use both `/gfx:AVC420` and `/gfx`. Some FreeRDP builds may reject explicit `/gfx:AVC420`; in that case the matrix still requires forced evidence from the `/gfx` fallback case.

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
for label in ["rdpgfx-planar", "rdpgfx-planar-stream", "rdpgfx-uncompressed", "rdpgfx-deferred-codecs", "h264-negotiated-gfx", "h264-avc420-forced", "h264-forced-gfx-fallback"]:
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

## Encoding families not default-enabled by this server yet

These are tracked explicitly so the matrix does not imply default RDP graphics-codec emission coverage:

| Encoding family | Matrix status | Rationale |
| --- | --- | --- |
| RDP 5/6 bitmap compression / bitmap RLE | Experimental opt-in | 24-bpp COPY/color-order encoder, expansion rejection, runtime toggle, diagnostics, and saved-byte matrix evidence exist; negotiated/default emission is still disabled. |
| NSCodec | Experimental opt-in | `go-rdp` exposes NSCodec encode/decode utilities; Android parses Bitmap Codecs, builds SurfaceBits commands, and emits an initial NSCodec update only when `GO_RDP_ANDROID_ENABLE_NSCODEC=1` and the client advertises NSCodec. Local FreeRDP 3.15.0 currently advertises zero bitmap codecs in this non-GFX case, so the matrix records capability absence. |
| RemoteFX / RFX | Implemented opt-in | `go-rdp` exposes RemoteFX GUID metadata and RFX decode package coverage; Android now has a production single-tile encoder path (YCoCg/DWT/quant/RLGR/message assembly) behind capability + `GO_RDP_ANDROID_ENABLE_RFX_CODEC=1`, plus a separate fixture path for transport-hook testing. |
| RDPGFX AVC444 / AVC444v2 | Encoder-hooked experimental seams; no production encoder | Higher-fidelity H.264 variants; shared IDs exist and generic WireToSurface hooks are wired, but production transport remains deferred until AVC420 negotiation/client proof exists. |
| RDPGFX ClearCodec | Experimental production encoder + fixture hook | Text/graphics optimized codec; bounded minimal production encoding now exists (solid rect + RGB565 raw rect splitting with expansion rejection) and matrix includes both production (`rdpgfx-clearcodec-encoded`) and fixture-hook (`rdpgfx-clearcodec-fixture`) cases. Keep off by default until client evidence is stronger. |
| RDPGFX Progressive / other progressive codecs | Experimental production-path gate + fixture hook | Shared IDs exist and generic WireToSurface hooks are wired for CAProgressive/CAProgressiveV2. Matrix now includes both production-path (`rdpgfx-progressive-encoded`) and fixture-hook (`rdpgfx-progressive-fixture`) cases; production path currently falls back unless an encoder is supplied. |
| JPEG/PNG bitmap codecs | Experimental opt-in | JPEG uses advertised Bitmap Codecs GUID evidence; PNG uses an operator-supplied codec ID override because no standard PNG Bitmap Codecs GUID is wired here. Both have SurfaceBits builders, runtime diagnostics, and matrix raw/saved/percent summaries; keep them off by default until client evidence justifies them. |
SUMMARY

cat >"$OUT/codec-coverage.json" <<'JSON'
{
  "runtime_emitters": [
    {"name":"slow-path raw bitmap", "status":"implemented-default-fallback", "matrix_case":"bitmap"},
    {"name":"RDP 5/6 bitmap compression / bitmap RLE", "status":"experimental-opt-in", "matrix_case":"bitmap-rle", "toggle":"GO_RDP_ANDROID_ENABLE_BITMAP_RLE=1", "default_enabled":false},
    {"name":"Classic RDP6 bitmap-update Planar", "status":"experimental-opt-in", "matrix_case":"bitmap-planar", "toggle":"GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR=1", "default_enabled":false},
    {"name":"Classic 16bpp bitmap updates", "status":"experimental-opt-in", "matrix_case":"bitmap-16bpp", "toggle":"GO_RDP_ANDROID_ENABLE_BITMAP_BPP=16", "default_enabled":false},
    {"name":"Classic 15bpp bitmap updates", "status":"experimental-opt-in", "matrix_case":"bitmap-15bpp", "toggle":"GO_RDP_ANDROID_ENABLE_BITMAP_BPP=15", "default_enabled":false},
    {"name":"Classic 8bpp paletted bitmap updates", "status":"experimental-opt-in", "matrix_case":"bitmap-8bpp", "toggle":"GO_RDP_ANDROID_ENABLE_BITMAP_BPP=8", "default_enabled":false},
    {"name":"NSCodec", "status":"experimental-opt-in", "matrix_case":"nscodec-opt-in", "toggle":"GO_RDP_ANDROID_ENABLE_NSCODEC=1", "requires_client_advertisement":true, "client_proof":"missing-in-current-freerdp-ci-profile", "fixture_hook":false, "production_encoder":true, "release_default":false, "default_enabled":false},
    {"name":"JPEG bitmap codec", "status":"experimental-opt-in", "matrix_case":"jpeg-opt-in", "toggles":["GO_RDP_ANDROID_ENABLE_JPEG_CODEC=1", "GO_RDP_ANDROID_JPEG_QUALITY=80"], "requires_client_advertisement":true, "client_proof":"missing-in-current-freerdp-ci-profile", "fixture_hook":false, "production_encoder":true, "release_default":false, "default_enabled":false},
    {"name":"PNG bitmap codec", "status":"operator-override-opt-in", "matrix_case":"png-opt-in", "toggles":["GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID=9", "GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL=-3"], "requires_operator_codec_id":true, "client_proof":"operator-supplied-codec-id-only", "fixture_hook":false, "production_encoder":true, "release_default":false, "default_enabled":false},
    {"name":"RDPGFX Planar", "status":"implemented-default-compressed", "matrix_case":"rdpgfx-planar", "fixture_hook":false, "production_encoder":true, "release_default":true, "default_enabled":true},
    {"name":"RDPGFX Planar streaming", "status":"experimental-opt-in", "matrix_case":"rdpgfx-planar-stream", "toggle":"GO_RDP_ANDROID_ENABLE_RDPGFX_STREAM=1", "default_enabled":false},
    {"name":"RDPGFX Uncompressed", "status":"diagnostic-opt-in", "matrix_case":"rdpgfx-uncompressed", "toggles":["GO_RDP_ANDROID_ENABLE_RDPGFX_UNCOMPRESSED=1", "GO_RDP_ANDROID_ENABLE_RDPGFX_STREAM=1"], "default_enabled":false},
    {"name":"RDPGFX AVC420 / H.264", "status":"experimental-force-mode", "matrix_cases":["h264-negotiated-gfx", "h264-avc420-forced", "h264-forced-gfx-fallback"], "default_enabled":"capability-gated"},
    {"name":"RemoteFX / RFX", "status":"implemented-opt-in", "matrix_cases":["rfx-encoded","rfx-fixture"], "toggle":"GO_RDP_ANDROID_ENABLE_RFX_CODEC=1", "requires_client_advertisement":true, "client_proof":"missing-in-current-freerdp-ci-profile", "fixture_hook":true, "production_encoder":true, "release_default":false, "default_enabled":false},
    {"name":"RDPGFX ClearCodec", "status":"experimental-production-opt-in", "matrix_cases":["rdpgfx-clearcodec-encoded","rdpgfx-clearcodec-fixture","rdpgfx-planar"], "toggle":"GO_RDP_ANDROID_ENABLE_CLEARCODEC=1", "client_proof":"partial-freerdp-fixture-acceptance-only", "fixture_hook":true, "production_encoder":true, "release_default":false, "default_enabled":false, "default_emission":"planar-fallback-when-unsupported-or-non-beneficial"},
    {"name":"RDPGFX Progressive / other progressive codecs", "status":"partial-production-opt-in", "matrix_cases":["rdpgfx-progressive-encoded","rdpgfx-progressivev2-encoded","rdpgfx-progressive-fixture","rdpgfx-planar"], "toggle":"GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC=1", "client_proof":"missing-production-client-proof", "fixture_hook":true, "production_encoder":true, "release_default":false, "default_enabled":false, "default_emission":"bounded-progressive-payload-or-planar-fallback"},
    {"name":"RDPGFX AVC444", "status":"partial-production-opt-in", "matrix_cases":["rdpgfx-deferred-codecs","rdpgfx-avc444-encoded","rdpgfx-avc444-fixture"], "toggle":"GO_RDP_ANDROID_ENABLE_AVC444=1", "client_proof":"missing-production-client-proof", "fixture_hook":true, "production_encoder":true, "release_default":false, "default_enabled":false, "default_emission":"bounded-base-aux-payload-or-planar-fallback"},
    {"name":"RDPGFX AVC444v2", "status":"partial-production-opt-in", "matrix_cases":["rdpgfx-deferred-codecs","rdpgfx-avc444v2-encoded","rdpgfx-avc444v2-fixture"], "toggle":"GO_RDP_ANDROID_ENABLE_AVC444V2=1", "client_proof":"missing-production-client-proof", "fixture_hook":true, "production_encoder":true, "release_default":false, "default_enabled":false, "default_emission":"bounded-base-aux-payload-or-planar-fallback"}
  ],
  "selection_scaffolds": [
  ],
  "upstream_metadata": [
    {"name":"NSCodec", "source":"github.com/rcarmo/go-rdp/pkg/codec", "android_emitter":"experimental-opt-in", "priority":"client-evidence-gated"},
    {"name":"RemoteFX / RFX", "source":"github.com/rcarmo/go-rdp/pkg/codec", "android_emitter":"implemented-opt-in", "priority":"client-evidence-gated"},
    {"name":"RDPGFX AVC444 / AVC444v2", "source":"github.com/rcarmo/go-rdp/pkg/codec", "android_emitter":"encoder-hooked-experimental", "priority":"deferred-until-avc420-proof"},
    {"name":"RDPGFX ClearCodec", "source":"github.com/rcarmo/go-rdp/pkg/codec", "android_emitter":"experimental-production-opt-in", "priority":"client-evidence-gated"},
    {"name":"RDPGFX Progressive / other progressive codecs", "source":"github.com/rcarmo/go-rdp/pkg/codec", "android_emitter":"encoder-hooked-experimental", "priority":"production-encoder-and-client-evidence-needed"},
    {"name":"JPEG bitmap codec", "source":"github.com/rcarmo/go-rdp/pkg/codec", "android_emitter":"experimental-opt-in", "priority":"evidence-gated"}
  ],
  "missing_runtime_emitters": [
    {"name":"RDPGFX AVC444 / AVC444v2", "reason":"production encoder/transport missing; runtime hook exists"},
    {"name":"RDPGFX Progressive / other progressive codecs", "reason":"production encoder missing; runtime hook exists"}
  ],
  "release_defaults": [
    "RDPGFX Planar",
    "slow-path raw bitmap fallback"
  ],
  "non_default_experimental_emitters": [
    "RDP 5/6 bitmap compression / bitmap RLE",
    "NSCodec",
    "JPEG bitmap codec",
    "PNG bitmap codec",
    "RDPGFX Planar streaming",
    "RDPGFX Uncompressed",
    "RDPGFX AVC420 / H.264",
    "RDPGFX ClearCodec",
    "RDPGFX Progressive / other progressive codecs",
    "RDPGFX AVC444",
    "RDPGFX AVC444v2"
  ]
}
JSON

cat "$OUT/summary.md"

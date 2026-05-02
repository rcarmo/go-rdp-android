#!/usr/bin/env bash
set -euo pipefail

mkdir -p emulator-artifacts

PACKAGE=pt.taoofmac.gordpandroid
ACTIVITY="$PACKAGE/.MainActivity"
GO_BACKED="${EMULATOR_GO_BACKED:-false}"
CAPTURE="${EMULATOR_CAPTURE:-false}"

adb install -r android/app/build/outputs/apk/debug/app-debug.apk | tee emulator-artifacts/adb-install.txt
adb shell pm grant "$PACKAGE" android.permission.POST_NOTIFICATIONS >/dev/null 2>&1 || true

if [ "$CAPTURE" = "true" ]; then
  adb shell am start -W -n "$ACTIVITY" --ez start_capture true | tee emulator-artifacts/activity-start.txt
  for _ in $(seq 1 45); do
    adb shell dumpsys window > emulator-artifacts/window-consent-wait.txt || true
    if grep -q 'mCurrentFocus.*MediaProjectionPermissionActivity' emulator-artifacts/window-consent-wait.txt; then
      break
    fi
    sleep 1
  done
  size_line=$(adb shell wm size | tr -d '\r' | awk -F': ' '/Physical size/ {print $2; exit}')
  width=${size_line%x*}
  height=${size_line#*x}
  : "${width:=1080}"
  : "${height:=2400}"
  dropdown_x=$((width / 2))
  dropdown_y=$((height / 2))
  entire_screen_x=$((width / 2))
  entire_screen_y=$((height * 57 / 100))
  start_x=$((width * 82 / 100))
  start_y=$((height * 725 / 1000))
  echo "wm_size=${width}x${height} dropdown_tap=${dropdown_x},${dropdown_y} entire_screen_tap=${entire_screen_x},${entire_screen_y} start_tap=${start_x},${start_y}" | tee emulator-artifacts/capture-consent.txt
  adb exec-out screencap -p > emulator-artifacts/mediaprojection-dialog.png || true
  adb shell input tap "$dropdown_x" "$dropdown_y" || true
  sleep 1
  adb exec-out screencap -p > emulator-artifacts/mediaprojection-scope-menu.png || true
  adb shell input tap "$entire_screen_x" "$entire_screen_y" || true
  sleep 1
  adb shell input tap "$start_x" "$start_y"
  for _ in $(seq 1 30); do
    adb logcat -d > emulator-artifacts/logcat-consent-wait.txt || true
    if grep -q 'Screen capture started' emulator-artifacts/logcat-consent-wait.txt; then
      break
    fi
    sleep 1
  done
else
  adb shell am start -W -n "$ACTIVITY" --ez start_test_pattern true | tee emulator-artifacts/activity-start.txt
  sleep 8
  width=320
  height=240
fi

adb shell pidof "$PACKAGE" | tee emulator-artifacts/pidof.txt || true
adb shell dumpsys package "$PACKAGE" > emulator-artifacts/dumpsys-package.txt || true
adb shell dumpsys activity activities > emulator-artifacts/dumpsys-activity.txt || true
adb logcat -d > emulator-artifacts/logcat.txt || true
grep -E 'GoRdpAndroid|GoRdpAndroidService|Screen capture started|backend=|frame#|FATAL EXCEPTION|AndroidRuntime|ForegroundService|SecurityException|Exception' emulator-artifacts/logcat.txt > emulator-artifacts/logcat-filtered.txt || true

if grep -q 'startServer' emulator-artifacts/logcat-filtered.txt; then
  echo 'startServer=ok'
else
  echo 'startServer=missing'
fi | tee emulator-artifacts/checks.txt

if grep -q 'frame#1' emulator-artifacts/logcat-filtered.txt; then
  echo 'frame1=ok'
else
  echo 'frame1=missing'
fi | tee -a emulator-artifacts/checks.txt

if [ "$CAPTURE" = "true" ]; then
  if grep -q 'Screen capture started' emulator-artifacts/logcat-filtered.txt; then
    echo 'screen_capture=ok'
  else
    echo 'screen_capture=missing'
  fi | tee -a emulator-artifacts/checks.txt
fi

if grep -q 'FATAL EXCEPTION' emulator-artifacts/logcat-filtered.txt; then
  echo 'fatal_exception=present'
else
  echo 'fatal_exception=none'
fi | tee -a emulator-artifacts/checks.txt

adb exec-out screencap -p > emulator-artifacts/screenshot.png || true

capture_rdp_scene() {
  local name="$1"
  local updates="$2"
  adb exec-out screencap -p > "emulator-artifacts/android-${name}.png" || true
  go run ./cmd/probe \
    -addr 127.0.0.1:3390 \
    -updates "$updates" \
    -screenshot-width "$width" \
    -screenshot-height "$height" \
    -screenshot "emulator-artifacts/rdp-${name}.png" \
    -summary "emulator-artifacts/rdp-${name}-summary.json" \
    -dump-packets=false \
    > "emulator-artifacts/rdp-${name}-probe.log" 2>&1
  test -s "emulator-artifacts/rdp-${name}.png"
  grep -q "\"bitmap_updates\": $updates" "emulator-artifacts/rdp-${name}-summary.json"
}

if [ "$GO_BACKED" = "true" ]; then
  adb forward tcp:3390 tcp:3390
  updates=20
  if [ "$CAPTURE" = "true" ]; then
    full_frame_updates=$(( ((width + 79) / 80) * ((height + 79) / 80) ))
    updates=${RDP_CAPTURE_UPDATES:-$full_frame_updates}
    echo "rdp_tile_size=80 full_frame_updates=$full_frame_updates selected_updates=$updates" | tee emulator-artifacts/rdp-capture-plan.txt
  fi

  if [ "$CAPTURE" = "true" ]; then
    adb shell input keyevent HOME || true
    sleep 3
    adb exec-out screencap -p > emulator-artifacts/android-home.png || true
    cat > emulator-artifacts/scene-plan.json <<'JSON'
[
  {
    "name": "settings",
    "command": "adb shell am start -W -a android.settings.SETTINGS | tee emulator-artifacts/settings-start.txt && sleep 3 && adb exec-out screencap -p > emulator-artifacts/android-settings.png",
    "wait_ms": 200,
    "max_updates": 420
  },
  {
    "name": "browser",
    "command": "adb shell am start -W -a android.intent.action.VIEW -d 'https://example.com' | tee emulator-artifacts/browser-start.txt && sleep 8 && adb exec-out screencap -p > emulator-artifacts/android-browser.png",
    "wait_ms": 200,
    "max_updates": 420
  }
]
JSON
    go run ./cmd/probe \
      -addr 127.0.0.1:3390 \
      -screenshot-width "$width" \
      -screenshot-height "$height" \
      -warmup-updates "$updates" \
      -warmup-screenshot emulator-artifacts/rdp-home.png \
      -scene-plan emulator-artifacts/scene-plan.json \
      -artifact-dir emulator-artifacts \
      -scene-idle-timeout-ms 1500 \
      -scene-max-updates "$updates" \
      -summary emulator-artifacts/rdp-probe-summary.json \
      -dump-packets=false \
      > emulator-artifacts/rdp-probe.log 2>&1
    test -s emulator-artifacts/rdp-home.png
    test -s emulator-artifacts/rdp-settings.png
    test -s emulator-artifacts/rdp-browser.png
    cp emulator-artifacts/rdp-browser.png emulator-artifacts/rdp-screenshot.png
  else
    capture_rdp_scene screenshot "$updates"
    cp emulator-artifacts/rdp-screenshot-summary.json emulator-artifacts/rdp-probe-summary.json
    cp emulator-artifacts/rdp-screenshot-probe.log emulator-artifacts/rdp-probe.log
  fi
fi

{
  echo '# RDP emulator performance summary'
  echo
  echo "- go_backed: $GO_BACKED"
  echo "- capture: $CAPTURE"
  echo "- screen: ${width}x${height}"
  if [ -f emulator-artifacts/rdp-capture-plan.txt ]; then
    sed 's/^/- /' emulator-artifacts/rdp-capture-plan.txt
  fi
  if grep -q 'captureStats' emulator-artifacts/logcat-filtered.txt 2>/dev/null; then
    echo
    echo '## Capture pacing/backpressure'
    echo
    echo '```text'
    grep 'captureStats' emulator-artifacts/logcat-filtered.txt | tail -n 5
    echo '```'
  fi
  echo
  for summary in emulator-artifacts/rdp-*-summary.json emulator-artifacts/rdp-probe-summary.json; do
    [ -f "$summary" ] || continue
    echo "## $(basename "$summary")"
    echo
    echo '```json'
    cat "$summary"
    echo '```'
    echo
  done
} > emulator-artifacts/performance-summary.md

grep -q 'startServer=ok' emulator-artifacts/checks.txt
grep -q 'frame1=ok' emulator-artifacts/checks.txt
if [ "$CAPTURE" = "true" ]; then
  grep -q 'screen_capture=ok' emulator-artifacts/checks.txt
fi
grep -q 'fatal_exception=none' emulator-artifacts/checks.txt

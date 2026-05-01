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
  sleep 2
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
  sleep 12
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

if [ "$GO_BACKED" = "true" ]; then
  adb forward tcp:3390 tcp:3390
  updates=20
  if [ "$CAPTURE" = "true" ]; then
    updates=${RDP_CAPTURE_UPDATES:-450}
  fi
  go run ./cmd/probe \
    -addr 127.0.0.1:3390 \
    -updates "$updates" \
    -screenshot-width "$width" \
    -screenshot-height "$height" \
    -screenshot emulator-artifacts/rdp-screenshot.png \
    -summary emulator-artifacts/rdp-probe-summary.json \
    -dump-packets=false \
    > emulator-artifacts/rdp-probe.log 2>&1
  test -s emulator-artifacts/rdp-screenshot.png
  grep -q "\"bitmap_updates\": $updates" emulator-artifacts/rdp-probe-summary.json
fi

grep -q 'startServer=ok' emulator-artifacts/checks.txt
grep -q 'frame1=ok' emulator-artifacts/checks.txt
if [ "$CAPTURE" = "true" ]; then
  grep -q 'screen_capture=ok' emulator-artifacts/checks.txt
fi
grep -q 'fatal_exception=none' emulator-artifacts/checks.txt

#!/usr/bin/env bash
set -euo pipefail

mkdir -p emulator-artifacts

adb install -r android/app/build/outputs/apk/debug/app-debug.apk | tee emulator-artifacts/adb-install.txt
adb shell am start -W -n pt.taoofmac.gordpandroid/.MainActivity --ez start_test_pattern true | tee emulator-artifacts/activity-start.txt
sleep 8
adb shell pidof pt.taoofmac.gordpandroid | tee emulator-artifacts/pidof.txt || true
adb shell dumpsys package pt.taoofmac.gordpandroid > emulator-artifacts/dumpsys-package.txt || true
adb shell dumpsys activity activities > emulator-artifacts/dumpsys-activity.txt || true
adb logcat -d > emulator-artifacts/logcat.txt || true
grep -E 'GoRdpAndroid|GoRdpAndroidService|backend=|frame#|FATAL EXCEPTION|AndroidRuntime|ForegroundService|SecurityException|Exception' emulator-artifacts/logcat.txt > emulator-artifacts/logcat-filtered.txt || true

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

if grep -q 'FATAL EXCEPTION' emulator-artifacts/logcat-filtered.txt; then
  echo 'fatal_exception=present'
else
  echo 'fatal_exception=none'
fi | tee -a emulator-artifacts/checks.txt

adb exec-out screencap -p > emulator-artifacts/screenshot.png || true

if [ "${EMULATOR_GO_BACKED:-false}" = "true" ]; then
  adb forward tcp:3390 tcp:3390
  go run ./cmd/probe \
    -addr 127.0.0.1:3390 \
    -updates 20 \
    -screenshot emulator-artifacts/rdp-screenshot.png \
    -summary emulator-artifacts/rdp-probe-summary.json \
    > emulator-artifacts/rdp-probe.log 2>&1
  test -s emulator-artifacts/rdp-screenshot.png
  grep -q '"bitmap_updates": 20' emulator-artifacts/rdp-probe-summary.json
fi

grep -q 'startServer=ok' emulator-artifacts/checks.txt
grep -q 'frame1=ok' emulator-artifacts/checks.txt
grep -q 'fatal_exception=none' emulator-artifacts/checks.txt

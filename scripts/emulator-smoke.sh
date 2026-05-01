#!/usr/bin/env bash
set -euo pipefail

mkdir -p emulator-artifacts

adb install -r android/app/build/outputs/apk/debug/app-debug.apk | tee emulator-artifacts/adb-install.txt
adb shell am start -W -n pt.taoofmac.gordpandroid/.MainActivity --ez start_test_pattern true | tee emulator-artifacts/activity-start.txt
sleep 8
adb shell pidof pt.taoofmac.gordpandroid | tee emulator-artifacts/pidof.txt
adb shell dumpsys package pt.taoofmac.gordpandroid > emulator-artifacts/dumpsys-package.txt
adb shell dumpsys activity activities > emulator-artifacts/dumpsys-activity.txt
adb logcat -d > emulator-artifacts/logcat.txt
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

adb exec-out screencap -p > emulator-artifacts/screenshot.png

grep -q 'startServer=ok' emulator-artifacts/checks.txt
grep -q 'frame1=ok' emulator-artifacts/checks.txt
grep -q 'fatal_exception=none' emulator-artifacts/checks.txt

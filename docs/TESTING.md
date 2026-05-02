# Testing Matrix

Everything below runs without a physical Android device.

## Default CI gates

- Go vet/build/test with coverage threshold (`make coverage COVERAGE_MIN=75.0`).
- Go race tests and short parser fuzz smoke.
- Mock server + probe TCP smoke test.
- Protocol packet trace artifact from the probe (`mock-probe-artifacts`), including client/server hex dumps and logs.
- Normal Android debug APK build and APK structure inspection, with build/inspection logs uploaded.
- `gomobile bind` AAR generation.
- Generated AAR Java API signature verification (`make check-aar-api`).
- Generated AAR native library/content inspection (`make check-aar-artifact`), with AAR contents uploaded.
- Go-backed APK build against `mobile.aar` and native library/content inspection, with APK contents uploaded.
- FreeRDP compatibility probe log, summary and screenshot capture (`freerdp-compat-probe`) against a mock server with animated test-pattern frames. This job is informational/non-blocking until the mock server fully satisfies real clients.

## Manual-only CI

- Android emulator smoke test (`workflow_dispatch` only): install debug APK, launch `MainActivity` with `start_test_pattern=true`, verify process startup, verify bridge startup and first synthetic frame in logcat, and for Go-backed runs use `adb forward` plus `cmd/probe` to connect over RDP and save `rdp-screenshot.png` from bitmap updates. Optional `emulator_capture=true` requests MediaProjection, captures the emulator display rather than the synthetic test pattern, navigates through home, Settings, and browser intents, and saves paired `android-*.png` / `rdp-*.png` screenshots for each scene. The job collects logcat, dumpsys, emulator screenshot, RDP probe summaries, and RDP screenshot artifacts.
- Workflow inputs:
  - `emulator_api_level` (default `35`)
  - `emulator_go_backed` (`false` for normal APK, `true` to build/install the Go-backed APK)

## Blocked on a physical device

- MediaProjection permission and frame capture behavior.
- AccessibilityService enablement and gesture/key injection UX.
- Real touch latency and frame pacing.
- Network reachability from another client device.

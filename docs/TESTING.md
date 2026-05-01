# Testing Matrix

Everything below runs without a physical Android device.

## Default CI gates

- Go vet/build/test with coverage threshold (`make coverage COVERAGE_MIN=75.0`).
- Mock server + probe TCP smoke test.
- Protocol packet trace artifact from the probe (`mock-probe-protocol-trace`).
- Normal Android debug APK build and APK structure inspection.
- `gomobile bind` AAR generation.
- Generated AAR Java API signature verification (`make check-aar-api`).
- Generated AAR native library/content inspection (`make check-aar-artifact`).
- Go-backed APK build against `mobile.aar` and native library/content inspection.
- FreeRDP compatibility probe log capture. This job is informational/non-blocking until the mock server fully satisfies real clients.

## Manual-only CI

- Android emulator smoke test (`workflow_dispatch` only): install debug APK, launch `MainActivity`, verify process startup.

## Blocked on a physical device

- MediaProjection permission and frame capture behavior.
- AccessibilityService enablement and gesture/key injection UX.
- Real touch latency and frame pacing.
- Network reachability from another client device.

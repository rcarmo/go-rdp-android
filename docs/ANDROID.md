# Android App Notes

The Android app is a Kotlin shell around a gomobile-backed Go RDP server core, with a logging backend fallback that keeps UI/package work buildable when `mobile.aar` is absent.

## Current app responsibilities

- request MediaProjection permission
- start a foreground service
- expose an AccessibilityService declaration
- bridge frames/input/health through `NativeRdpBridge`

## gomobile bridge

`NativeRdpBridge.kt` routes to a gomobile-generated AAR exposing the Go server core when available, with a logging fallback for non-Go-backed builds.

The Android shell now prefers a gomobile-generated Go backend when `android/app/libs/mobile.aar` is present. If the AAR is absent, it falls back to a logging backend so CI and UI work can continue.

CI includes two off-device Android paths:

- default push/PR path: build the normal debug APK and separately generate `mobile.aar` with `gomobile bind`, then build a Go-backed APK against that AAR.
- manual `workflow_dispatch` path: an Android emulator smoke test that installs the debug APK and launches `MainActivity`.

The emulator job is intentionally manual until the gomobile/AAR path is stable. Neither path replaces physical-device validation of MediaProjection/Accessibility, but together they catch packaging, binding and startup failures before device testing is available.

Build the Go AAR and app with:

```bash
make gomobile-init   # first time, installs/initializes gomobile
make android-build-go
```

Current Go mobile-facing API scaffold lives in `mobile/bridge.go`:

```go
func SetCredentials(username, password string)
func StartServer(port int) error
func SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error
func StopServer() error
func Addr() string
func TLSFingerprintSHA256() string
```

It also exposes `type Server` and a bounded `FrameQueue` for tests/non-singleton usage. Captured Android `RGBA_8888` buffers are copied into a bounded Go queue and consumed as `frame.Source` by the RDP server.

Decoded RDP input is surfaced through a gomobile-friendly callback interface:

```go
type InputHandler interface {
    PointerMove(x int, y int)
    PointerButton(x int, y int, buttons int, down bool)
    PointerWheel(x int, y int, delta int, horizontal bool)
    Key(scancode int, down bool)
    Unicode(codepoint int)
    TouchFrameStart(contactCount int)
    TouchContact(contactID int, x int, y int, flags int)
    TouchFrameEnd()
}

func SetInputHandler(handler InputHandler)
```

`NativeRdpBridge` now routes to `GomobileRdpBackend` via reflection when the generated `mobile.Mobile` classes exist, otherwise to `LoggingRdpBackend`. The reflection shim keeps the app buildable before gomobile artifacts are generated while still wiring the runtime path to Go once `mobile.aar` is bundled.

For CI/emulator testing, `MainActivity` accepts `--ez start_test_pattern true` or `--ez start_capture true` plus optional `--es rdp_username <user> --es rdp_password <pass>`. The app now blocks service start until credentials are configured (first-run UI save or intent-provided values), then passes credentials into `RdpForegroundService` and the gomobile bridge before starting the server. Saved credentials are persisted encrypted-at-rest using an Android Keystore AES/GCM key (`RdpCredentialStore`), with one-time migration from legacy plaintext prefs when present. Long-running server starts now always enter foreground-service mode (test pattern and MediaProjection), expose a notification Stop action, and also route the main UI Stop button through the same explicit service action for deterministic cleanup. The service is intentionally `START_NOT_STICKY`: after process death or projection revocation it requires an explicit user/UI restart rather than silently resuming an RDP listener without fresh state. The main UI also surfaces a compact health line from `NativeRdpBridge` (backend, running state, mode, listen address, TLS fingerprint prefix, frame count, input scale) to make service/input/capture state visible during testing. CI asserts that `startServer` and `frame#1` appear in logcat. In Go-backed emulator runs, CI also forwards TCP/3390 from the runner to the emulator, connects with `cmd/probe` using matching credentials, and writes `rdp-screenshot.png` from RDP bitmap updates. With `emulator_capture=true`, CI requests MediaProjection, accepts the emulator consent dialog, uses MediaProjection frames instead of synthetic frames, navigates through home, Settings, and browser intents, and writes paired Android/RDP screenshots for each scene.

`RdpAccessibilityService` includes matching callback landing points. Primary pointer down/move/up is now coalesced into a bounded Accessibility `GestureDescription` path, so simple taps still dispatch on button-up and primary-button drags can become strokes. Pointer wheel events are decoded and surfaced through the bridge, then logged/degraded safely because `AccessibilityService` has no reliable generic wheel injection primitive. Key and Unicode callbacks are still limited to current landing behavior until richer Accessibility injection is implemented.

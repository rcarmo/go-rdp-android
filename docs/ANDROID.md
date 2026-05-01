# Android App Notes

The Android app is currently a Kotlin shell around a future Go binding.

## Current app responsibilities

- request MediaProjection permission
- start a foreground service
- expose an AccessibilityService declaration
- provide a temporary `NativeRdpBridge` stub

## Planned bridge

The stub in `NativeRdpBridge.kt` should be replaced by a gomobile-generated AAR exposing the Go server core.

The Android shell now prefers a gomobile-generated Go backend when `android/app/libs/mobile.aar` is present. If the AAR is absent, it falls back to a logging backend so CI and UI work can continue.

CI includes an Android emulator smoke test that installs the debug APK and launches `MainActivity`. This is not a replacement for physical-device validation of MediaProjection/Accessibility, but it catches packaging/runtime launch failures before device testing is available.

Build the Go AAR and app with:

```bash
make gomobile-init   # first time, installs/initializes gomobile
make android-build-go
```

Current Go mobile-facing API scaffold lives in `mobile/bridge.go`:

```go
func StartServer(port int) error
func SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error
func StopServer() error
```

It also exposes `type Server` and a bounded `FrameQueue` for tests/non-singleton usage. The Kotlin stub should be replaced with calls into the gomobile-generated AAR that maps to those functions. Captured Android `RGBA_8888` buffers are copied into a bounded Go queue and consumed as `frame.Source` by the RDP server.

Decoded RDP input is surfaced through a gomobile-friendly callback interface:

```go
type InputHandler interface {
    PointerMove(x int, y int)
    PointerButton(x int, y int, buttons int, down bool)
    Key(scancode int, down bool)
    Unicode(codepoint int)
}

func SetInputHandler(handler InputHandler)
```

`NativeRdpBridge` now routes to `GomobileRdpBackend` via reflection when the generated `mobile.Mobile` classes exist, otherwise to `LoggingRdpBackend`. The reflection shim keeps the app buildable before gomobile artifacts are generated while still wiring the runtime path to Go once `mobile.aar` is bundled.

`RdpAccessibilityService` includes matching callback landing points. Pointer button down for the primary button is currently mapped to a tap gesture; pointer move, key and Unicode callbacks are logged until richer Accessibility injection is implemented.

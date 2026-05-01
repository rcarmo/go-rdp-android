# Android App Notes

The Android app is currently a Kotlin shell around a future Go binding.

## Current app responsibilities

- request MediaProjection permission
- start a foreground service
- expose an AccessibilityService declaration
- provide a temporary `NativeRdpBridge` stub

## Planned bridge

The stub in `NativeRdpBridge.kt` should be replaced by a gomobile-generated AAR exposing the Go server core.

Current Go mobile-facing API scaffold lives in `mobile/bridge.go`:

```go
func StartServer(port int) error
func SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error
func StopServer() error
```

It also exposes `type Server` and a bounded `FrameQueue` for tests/non-singleton usage. The Kotlin stub should be replaced with calls into the gomobile-generated AAR that maps to those functions. Captured Android `RGBA_8888` buffers are copied into a bounded Go queue and consumed as `frame.Source` by the RDP server.

Input currently terminates in a no-op Go `input.Sink`; the next Android step is to surface those callbacks into Kotlin and forward pointer/key events to `RdpAccessibilityService`.

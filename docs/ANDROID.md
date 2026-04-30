# Android App Notes

The Android app is currently a Kotlin shell around a future Go binding.

## Current app responsibilities

- request MediaProjection permission
- start a foreground service
- expose an AccessibilityService declaration
- provide a temporary `NativeRdpBridge` stub

## Planned bridge

The stub in `NativeRdpBridge.kt` should be replaced by a gomobile-generated AAR exposing the Go server core.

Expected Go API shape:

```go
func StartServer(port int, frameSource FrameSource, inputSink InputSink) error
func StopServer() error
```

The Kotlin service should pass captured frames to Go and receive input callbacks from Go.

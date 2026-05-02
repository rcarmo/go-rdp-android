# Architecture

`go-rdp-android` is split into an Android host application and a Go RDP server core. The design goal is to run as a normal installed Android app and avoid ADB as a runtime dependency.

## High-level runtime

```text
Android display
  → MediaProjection
  → VirtualDisplay
  → ImageReader RGBA frames
  → Kotlin ScreenCaptureManager
  → NativeRdpBridge
  → gomobile mobile.aar
  → Go FrameQueue
  → internal/rdpserver
  → RDP client/probe over TCP

RDP client/probe input
  → Go slow-path input decoder
  → mobile.InputHandler
  → NativeRdpBridge callbacks
  → RdpAccessibilityService landing points
```

The current CI path uses `adb forward tcp:3390 tcp:3390` only as an emulator test convenience. The application architecture itself is native Android + Go and does not require ADB to run on a device.

## Android layer

Important files:

- `android/app/src/main/java/.../MainActivity.kt`
- `android/app/src/main/java/.../service/RdpForegroundService.kt`
- `android/app/src/main/java/.../capture/ScreenCaptureManager.kt`
- `android/app/src/main/java/.../bridge/NativeRdpBridge.kt`
- `android/app/src/main/java/.../input/RdpAccessibilityService.kt`

Responsibilities:

- Ask the user for MediaProjection permission.
- Start/stop the foreground service for real capture.
- Create a downscaled or full-size virtual display.
- Copy throttled `RGBA_8888` frames from `ImageReader` into the Go bridge.
- Prefer a gomobile backend when `mobile.aar` is bundled.
- Fall back to a logging backend when the generated AAR is absent.
- Receive decoded RDP input callbacks and route them toward Accessibility.

## Go mobile bridge

Important files:

- `mobile/bridge.go`
- `android/app/src/main/java/.../bridge/GomobileRdpBackend.kt`

The gomobile API exposes:

```go
func StartServer(port int) error
func SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error
func StopServer() error
func SetInputHandler(handler InputHandler)
```

Frames are copied into a bounded `FrameQueue`. The queue drops old frames when full, keeping the newest frame available for RDP encoding. This is preferable for remote desktop UX because stale frames are less useful than the latest screen state.

## RDP server core

Important packages:

- `internal/rdpserver` — server handshake, channel state, graphics, input decoding.
- `internal/frame` — frame model, pixel formats, test-pattern sources.
- `internal/input` — input sink abstraction.

Implemented protocol path:

```text
TCP
→ TPKT
→ X.224 negotiation
→ MCS Connect
→ GCC response scaffold
→ ErectDomain
→ AttachUser
→ ChannelJoin
→ Client Info
→ Demand Active
→ Confirm Active
→ Synchronize
→ Control
→ FontList / FontMap
→ slow-path bitmap updates
→ slow-path input decoding
```

Graphics currently use classic slow-path bitmap updates. Frames are split into 80x80 tiles to stay within safe packet/length envelopes. After the first frame, a per-session tile cache skips unchanged tiles.

## Capture and graphics pipeline

The default real-capture path is:

```text
MediaProjection → VirtualDisplay → ImageReader RGBA_8888
  → Kotlin byte array
  → Go FrameQueue
  → RGBA to BGRA tile conversion
  → slow-path RDP bitmap update PDUs
```

Performance controls currently include:

- Dirty-tile suppression.
- Single-session scene capture in CI.
- Adaptive capture pacing/backpressure.
- Optional capture downscale (`capture_scale`, `emulator_capture_scale`).

Planned graphics paths include compressed bitmap/RDPGFX and H.264/AVC. Those are separate workstreams because they require different protocol negotiation and, in the H.264 case, likely a MediaCodec/encoder-surface capture path.

## Input architecture

Current input support has two layers:

1. Go decodes RDP slow-path pointer, keyboard, and Unicode events into `input.Sink` calls.
2. gomobile forwards those events to Kotlin `RdpInputCallbacks` and `RdpAccessibilityService` landing methods.

CI currently validates emulator input using scripted Android input commands while RDP capture is running:

- keyboard text in Settings search;
- mouse-source tap at a deterministic target;
- touchscreen swipe to reveal notifications.

Full production input injection still needs richer Accessibility handling and physical-device validation.

## CI architecture

GitHub Actions is the source of truth for validation. It produces build, protocol, emulator, screenshot, and UX-report artifacts.

Key CI jobs:

- Go checks.
- Go race and fuzz smoke.
- Android debug APK build.
- Go mobile AAR build and Go-backed APK build.
- FreeRDP compatibility probe.
- Android emulator smoke/UX job for workflow dispatch, `*-ux` tags, and release tags.

## Release architecture

Release identifiers:

- SemVer: `0.1.0`
- Android application ID: `io.carmo.go.rdp.android`
- Android versionCode: `1`

Tag classes are documented in [RELEASES](RELEASES.md). Release tags (`vX.X.X`) generate normal build artifacts and the UX PDF report.

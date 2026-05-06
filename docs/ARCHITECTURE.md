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
  → Go slow-path / Fast-Path input decoder
  → mobile.InputHandler
  → NativeRdpBridge callbacks
  → RdpAccessibilityService landing points

Future true RDP touch input
  → RDPEI over dynamic virtual channel (drdynvc)
  → touch-contact decoder
  → Android Accessibility gesture strokes
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
func SetCredentials(username, password string)
func SetInputHandler(handler InputHandler)
```

Frames are copied into a bounded `FrameQueue`. The queue drops old frames when full, keeping the newest frame available for RDP encoding. This is preferable for remote desktop UX because stale frames are less useful than the latest screen state.

`SetCredentials` configures the current username/password authenticator for future sessions. The server now has two encrypted authentication paths: TLS-only (`PROTOCOL_SSL`) with classic Client Info credential validation, and Hybrid/NLA (`PROTOCOL_HYBRID`) with a CredSSP/NTLMv2 handshake, TLS public-key binding, encrypted `TSCredentials`, and the same credential gate. The NLA primitives are consumed from `github.com/rcarmo/go-rdp/pkg/auth` rather than duplicated locally.

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
→ TLS when `PROTOCOL_SSL` or `PROTOCOL_HYBRID` is selected
→ CredSSP/NTLMv2 when `PROTOCOL_HYBRID` is selected
→ MCS Connect
→ GCC response with server core/security/network data
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
→ slow-path and Fast-Path input decoding
```

Planned protocol path for true touch input:

```text
Client static channel request for drdynvc
→ Dynamic virtual channel negotiation
→ RDPEI input channel
→ touch contact frames
→ Android gesture dispatch
```

Graphics currently use classic slow-path bitmap updates. Frames are split into 80x80 tiles to stay within safe packet/length envelopes. After the first frame, a per-session tile cache skips unchanged tiles.

## Capture and graphics pipeline

The default real-capture path is:

```text
MediaProjection → VirtualDisplay → ImageReader RGBA_8888
  → Kotlin byte array
  → Go FrameQueue
  → RGBA to 24-bit BGR tile conversion
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

1. Go decodes RDP slow-path and Fast-Path pointer, keyboard, and Unicode events into `input.Sink` calls. FreeRDP normally uses Fast-Path input after activation, so the server reads those transport PDUs directly instead of discarding them.
2. gomobile forwards those events to Kotlin `RdpInputCallbacks` and `RdpAccessibilityService` landing methods.

True RDP touch is separate from mouse/pointer input. Modern clients can send contact frames through MS-RDPEI over the dynamic virtual channel stack (`drdynvc`), so touch support needs dynamic-channel negotiation and a touch-contact event model rather than mapping all input to mouse buttons. The protocol scaffold now parses RDPEI headers, client-ready messages, touch-event frames, touch contacts, optional contact geometry/orientation/pressure fields, and dismiss-hovering-contact PDUs in `internal/rdpserver/rdpei.go`, with explicit PDU/frame/contact count bounds. The static `drdynvc` scaffold in `internal/rdpserver/drdynvc.go` detects the client-advertised `drdynvc` channel, handles capability/create/data PDUs, accepts the `Microsoft::Windows::RDS::Input` dynamic channel, sends RDPEI `SC_READY`, and routes RDPEI dynamic data into the parser. The `drdynvc` path now bounds static payloads, dynamic PDU sizes, RDPEI payload sizes, total fragment buffers, and pending fragment count, expires abandoned `DATA_FIRST` fragments, requires capability negotiation before lifecycle commands, rejects duplicate create requests and unsupported/second RDPEI channels, rejects data for unopened channels, cleans channel state on close, supports close/reopen, and covers variable-length channel ID encodings in tests. Parsed touch contacts, including optional RDPEI rectangle/orientation/pressure metadata, pass through `input.TouchLifecycleCoalescer` before reaching the optional `input.TouchSink` / gomobile `TouchContact` callback, so stray update/up/cancel events do not become Android gestures and metadata is preserved until Android-side degradation. `RdpAccessibilityService` now builds a bounded single-contact `GestureDescription` path from down/update/up contacts; coordinated multi-touch Accessibility strokes remain pending. That work is tracked in `/workspace/workitems/10-next/go-rdp-android-rdpei-touch-input.md`.

CI currently validates emulator input using scripted Android input commands while RDP capture is running:

- keyboard text in Settings search;
- mouse-source tap at a deterministic target;
- touchscreen swipe to reveal notifications.

Pointer input now coalesces primary-button down/move/up into bounded Accessibility gesture paths, preserving tap behavior while enabling basic drag strokes. Wheel events are decoded and carried through the Go/mobile/Kotlin bridge, then logged/degraded safely on Android because generic wheel injection is not available through Accessibility. Full production input injection still needs richer keyboard/text handling, secondary-button behavior, gesture failure callbacks, and physical-device validation.

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

- SemVer: `0.1.1`
- Android application ID: `io.carmo.go.rdp.android`
- Android versionCode: `2`

Tag classes are documented in [RELEASES](RELEASES.md). Release tags (`vX.X.X`) generate normal build artifacts and the UX PDF report.

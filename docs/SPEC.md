# Native Android RDP Server Feasibility: Reusing `rcarmo/go-rdp`

## Goal

Build a native Android RDP server without relying on ADB, while reusing as much as possible from `rcarmo/go-rdp`.

## Summary

`rcarmo/go-rdp` is currently an RDP **client** and browser gateway, not a server. However, it has a substantial amount of reusable protocol machinery:

- TPKT/X.224 framing
- MCS/GCC encoding/decoding primitives
- capability set structures
- PDU serializers/deserializers
- FastPath input/update code
- bitmap/NSCodec/RemoteFX codec code
- audio/dynamic-channel protocol experiments

The practical path is not to “turn it around” wholesale, but to extract/reuse the low-level protocol and codec packages to build a small RDP server profile that exposes the Android screen.

## Native Android architecture

```text
Android app
  ├── Kotlin/Java service layer
  │   ├── MediaProjection permission + foreground service
  │   ├── VirtualDisplay / ImageReader or Surface capture
  │   ├── AccessibilityService for input injection
  │   └── Android lifecycle / permissions / notifications
  │
  ├── Go RDP server core
  │   ├── TCP listener, e.g. 3389 or configurable high port
  │   ├── X.224 / MCS / GCC / security handshake
  │   ├── RDP capability negotiation
  │   ├── framebuffer update path
  │   └── input event decoder
  │
  └── JNI/gomobile bridge
      ├── Android frame source → Go framebuffer updates
      └── Go input events → Accessibility gestures/key injection
```

## What to reuse from `go-rdp`

| Area | Reuse potential | Notes |
|---|---:|---|
| `internal/protocol/tpkt` | High | Framing is symmetric enough to reuse for server. |
| `internal/protocol/x224` | High | Connection confirm/server-side path needs adding. |
| `internal/protocol/mcs` | Medium/High | Many structures are reusable; server state machine must be implemented. |
| `internal/protocol/gcc` | Medium/High | Useful for conference/create response pieces. |
| `internal/protocol/pdu` | High | Capability sets, input events, data PDUs are very useful. |
| `internal/protocol/fastpath` | Medium | Current code is client-focused, but serializers help. |
| `internal/codec` | Medium | Server mostly needs encoding, not decoding; existing encode/RLE pieces help. |
| `internal/codec/rfx` | Medium/Low initially | Useful later; start simpler than RemoteFX. |
| `internal/auth` | Medium | CredSSP/NTLM is client-oriented; server auth is different. |
| `internal/rdp` | Low | Mostly client session orchestration. Good reference, not direct reuse. |
| `internal/handler`, `web/` | Low | Browser gateway/client-side, not relevant for Android server. |

Important packaging issue: most reusable code is under `internal/`, so a separate Android module cannot import it directly. We should either:

1. build the Android server inside the same module initially, or
2. extract protocol primitives into a public package/module, e.g. `pkg/rdp`, `pkg/rdpserver`, `pkg/codec`.

## What scrcpy teaches us

scrcpy’s Android server uses a temporary `app_process` process launched by ADB. We are not using that model, but its architecture is useful:

- video capture path feeds a hardware encoder
- control channel maps desktop input into Android input events
- display rotation/resizing and input coordinate transforms are first-class
- input injection is the hard part

For a non-ADB Android app, we must replace scrcpy’s shell privileges with official APIs:

| Function | scrcpy | Native app equivalent |
|---|---|---|
| Screen capture | shell/system display APIs | `MediaProjection` + foreground service |
| Video encode | `MediaCodec` | `MediaCodec` or Go-side bitmap encoder |
| Input injection | hidden `InputManager.injectInputEvent()` via shell privileges | `AccessibilityService` gestures + text/key APIs |
| Clipboard | system service wrappers | Android ClipboardManager + Accessibility if needed |
| Launch model | ADB `app_process` | installed app + foreground service |

## MVP RDP server profile

Start with the smallest RDP profile that Microsoft Remote Desktop clients will accept.

### Phase 1: LAN prototype

- Android app starts a foreground service.
- User grants MediaProjection permission.
- User enables AccessibilityService.
- Go code listens on configurable TCP port, e.g. `3389` or `3390`.
- Accept exactly one session at a time.
- Implement enough RDP handshake for TLS-only clients and an experimental Hybrid/NLA path.
- Send bitmap framebuffer updates.
- Decode pointer/key input and forward to Android service.

Deliberately skip initially:

- production-grade NLA/CredSSP/client compatibility hardening
- audio
- clipboard
- printer/device redirection
- multi-monitor
- dynamic virtual channels except the RDPEI touch-input subset when touch support becomes a target
- RemoteFX/H.264 advanced graphics

### Phase 2: Security and compatibility

- Add TLS with app-generated cert or user-supplied cert. ✅ (generated self-signed cert scaffold)
- Add password/pairing token auth. ✅ (static credential gate)
- Investigate minimal CredSSP/NLA server support if Microsoft clients require it. ✅ (experimental Hybrid/NLA CredSSP path)
- Add display resize/reconnect semantics.

### Phase 3: Performance

- Start with raw/RLE bitmap updates.
- Add dirty-region detection.
- Add NSCodec/RemoteFX if clients negotiate it and `go-rdp` encoders are mature enough.
- Consider H.264/RDPGFX only later; that is a significantly larger implementation.

## Screen pipeline choices

### Option A: ImageReader frames → Go bitmap updates

Simpler RDP integration:

```text
MediaProjection → VirtualDisplay → ImageReader RGBA/YUV
  → Kotlin frame callback
  → Go buffer
  → RDP bitmap update / RLE / NSCodec
```

Pros:
- easier to map to classic RDP bitmap updates
- avoids RDP H.264/GFX complexity

Cons:
- more CPU/memory bandwidth
- may be slower than scrcpy

### Option B: MediaCodec H.264 → RDP graphics pipeline

```text
MediaProjection → Surface → MediaCodec H.264
  → RDP GFX/H.264 virtual channel
```

Pros:
- more efficient
- close to scrcpy performance

Cons:
- RDP H.264/GFX server-side implementation is substantially harder
- `go-rdp` currently has H.264 GUID awareness but not a full server-side graphics pipeline

Recommendation: start with Option A.

## Input pipeline

Classic RDP keyboard and pointer events from the client can be decoded using existing `go-rdp` PDU/FastPath input structures. True touch input is different: modern RDP clients can send touch contact frames using MS-RDPEI over the dynamic virtual channel stack (`drdynvc`), so it needs channel negotiation and a contact lifecycle decoder. `internal/rdpserver/rdpei.go` contains the initial MS-RDPEI parser scaffold for server-ready/client-ready metadata, touch frames, touch contacts, optional geometry/orientation/pressure fields, and malformed-PDU handling. `internal/rdpserver/drdynvc.go` now detects the static `drdynvc` channel, parses capability/create/data PDUs, accepts `Microsoft::Windows::RDS::Input`, sends RDPEI `SC_READY`, and routes RDPEI payloads into the parser. Parsed contacts are normalized by `input.TouchLifecycleCoalescer` and exposed through a separate `input.TouchSink` / gomobile `TouchContact` callback. Android Accessibility now builds a bounded single-contact stroke path from down/update/up contacts; richer coordinated multi-touch gesture dispatch remains pending.

Map them to Android:

| RDP input | Android native app path |
|---|---|
| Mouse move/click | Accessibility `dispatchGesture()` |
| Touch contacts (RDPEI) | Accessibility gesture path with contact IDs/strokes |
| Keyboard text | Accessibility text input / IME strategy |
| Special keys | Accessibility global actions where possible |
| Clipboard | Android ClipboardManager + focused text injection |

Limitations:
- Accessibility cannot fully emulate shell-level input injection.
- System keys and secure surfaces may be restricted.
- Protected content may not be capturable via MediaProjection.

## Server-side RDP work needed

`go-rdp` is client-side, so we need new server orchestration:

```go
type Server struct {
    Listener net.Listener
    Screen   FrameSource
    Input    InputSink
    Auth     Authenticator
}

type FrameSource interface {
    Frames() <-chan Frame
}

type InputSink interface {
    Pointer(x, y int, buttons ButtonState) error
    Key(scancode uint16, down bool) error
    Unicode(r rune) error
}
```

New packages likely needed:

- `internal/rdpserver` or `pkg/rdpserver`
- server-side X.224/MCS/GCC handshake
- server-side capability negotiation
- server-side update sender
- Android bridge API

## Android integration choices

### Go on Android

Possible approaches:

1. `gomobile bind`
   - expose Go server as an AAR
   - Kotlin starts/stops it
   - callbacks for frame/input

2. Native library via NDK/cgo
   - more complex
   - more control

3. Pure Kotlin RDP server using ported pieces
   - not recommended; wastes `go-rdp` reuse

Recommendation: `gomobile bind` first.

## Existing projects to compare/reuse

- `droidVNC-NG`: good reference for MediaProjection + Accessibility service behavior.
- RustDesk Android: good reference for production remote-control permission/lifecycle UX.
- scrcpy: best reference for high-performance capture/control architecture, but not usable directly without ADB.

## Biggest risks

1. Microsoft RDP client compatibility without NLA/CredSSP.
2. Performance with classic bitmap updates.
3. Accessibility input limitations.
4. Android background/foreground service restrictions.
5. RDP server-side handshake complexity, since `go-rdp` is client-first.

## Recommended immediate next steps

1. Extract a reusable RDP protocol subset from `go-rdp` into server-friendly packages.
2. Build a desktop mock RDP server first that serves a generated framebuffer.
3. Validate Microsoft Remote Desktop can connect to the mock.
4. Only then add Android MediaProjection frame source.
5. Then add Accessibility input sink.

This reduces risk: RDP server protocol compatibility can be debugged on desktop before fighting Android permissions and lifecycle.

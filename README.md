# go-rdp-android

![go-rdp-android icon](docs/icon-256.png)

Native Android RDP server experiment.

The goal is to expose an Android device over RDP **without ADB**, using:

- Android `MediaProjection` for screen capture
- Android `AccessibilityService` for input injection
- Go for the RDP server core
- Kotlin for Android lifecycle, permissions, and foreground services

See [docs/SPEC.md](docs/SPEC.md) for the design notes and feasibility analysis.

## Status

Early scaffold:

- Go server-core skeleton
- initial TPKT/X.224 negotiation
- Android/Kotlin app shell
- foreground service and MediaProjection capture skeleton
- AccessibilityService placeholder
- GitHub Actions for Go and Android builds

## Layout

```text
cmd/mock-server/                 Desktop-side mock for protocol experiments
internal/rdpserver/              Go RDP server core skeleton
internal/frame/                  Frame source abstractions
internal/input/                  Input sink abstractions
android/app/                     Native Android app shell in Kotlin
docs/SPEC.md                     Design/specification
```

## Build

```bash
make test
make build-go
```

Run the current handshake prototype locally:

```bash
# terminal 1
make run-mock
# or use animated synthetic frames:
make run-mock-pattern

# terminal 2
make probe

# or run both as a local smoke test
make smoke
```

The probe exercises:

```text
TCP → TPKT → X.224 → MCS Connect → Domain/Channel Join → Client Info → Demand/Confirm Active → FontMap → Bitmap Update
```

Android debug APKs are built by GitHub Actions and uploaded as workflow artifacts.

The current graphics path converts `frame.Source` RGBA/BGRA frames into TPKT-safe slow-path bitmap update tiles, with a solid-color fallback when no frame is available. Slow-path keyboard, Unicode, and mouse input PDUs are decoded into the internal `input.Sink` interface for the future Android Accessibility bridge.

## Next major steps

1. Extract/reuse protocol pieces from `rcarmo/go-rdp` into server-friendly packages.
2. Build a desktop mock RDP server that serves generated frames.
3. Validate an RDP client can connect to the mock.
4. Generate a Go Android binding via `gomobile bind`.
5. Wire Android `MediaProjection` frames into the Go server.
6. Wire RDP input events into Android Accessibility gestures.

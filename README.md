# go-rdp-android

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
- Android/Kotlin app shell
- foreground service and permission placeholders
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

Android builds are expected to run via GitHub Actions once Gradle/Android SDK setup is available.

## Next major steps

1. Extract/reuse protocol pieces from `rcarmo/go-rdp` into server-friendly packages.
2. Build a desktop mock RDP server that serves generated frames.
3. Validate an RDP client can connect to the mock.
4. Generate a Go Android binding via `gomobile bind`.
5. Wire Android `MediaProjection` frames into the Go server.
6. Wire RDP input events into Android Accessibility gestures.

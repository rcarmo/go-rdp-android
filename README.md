# go-rdp-android

![go-rdp-android icon](docs/icon-256.png)

A native Android RDP server experiment written in Kotlin and Go.

`go-rdp-android` is exploring how far a normal installed Android app can go toward exposing the Android screen over RDP **without using ADB as the runtime architecture**, building on [`rcarmo/go-rdp`](https://github.com/rcarmo/go-rdp) as the original core protocol implementation and reference. The app uses Android `MediaProjection` for screen capture, an `AccessibilityService` landing path for input, and a Go RDP server core bridged into Android with `gomobile`.

The current implementation is a CI-first prototype: it can build a Go-backed APK, launch it in an Android emulator, grant MediaProjection, connect to the embedded Go RDP server over forwarded TCP, render RDP screenshots, exercise keyboard/mouse/touch input scripts, and generate a Gherkin/Playwright UX PDF report.

## Feature list

Implemented or validated today:

- Native Android Kotlin shell with `MainActivity`, `RdpForegroundService`, MediaProjection consent/denial flow, serialized foreground-service mode switching, notification/UI stop actions, credential-refusal cleanup, network-address notification refresh, non-sticky service restart policy, non-secret settings persistence, Android controls for security mode and failed-auth policy, compact UI health/diagnostic sharing, and AccessibilityService declaration.
- Go RDP server core with TPKT, X.224, MCS, GCC server core/security/network data, TLS-only Client Info authentication, Hybrid/NLA CredSSP/NTLMv2 authentication via `rcarmo/go-rdp`, TLS certificate persistence/rotation/fingerprint support, access-policy controls, failed-auth backoff/lockout, Demand Active/Confirm Active finalization, FontMap, slow-path bitmap updates, and slow-path/Fast-Path input decoding.
- `gomobile bind` integration via `mobile.aar`, with Kotlin reflection backend and logging fallback when the AAR is absent.
- Android `MediaProjection` capture pipeline using `VirtualDisplay` + `ImageReader` RGBA frames.
- Synthetic test-pattern frame source for emulator/CI validation without capture permission.
- RDP 24-bit BGR bitmap update tiling sized for safe TPKT/PER envelopes.
- Dirty-tile suppression for post-initial streamed frames.
- Layered capture pacing/backpressure: Android adaptive capture interval, bounded Go queue drops, and server-side queued-frame coalescing.
- Optional MediaProjection downscale mode (`capture_scale` / `emulator_capture_scale`).
- Keyboard, mouse, pointer, wheel decode/degrade, and RDPEI touch validation in CI using scripted emulator input and synthetic dynamic-channel touch packets.
- Gherkin-style UX stories under `features/ux/` and a Playwright-based PDF report generator.
- GitHub Actions coverage for Go tests, race tests, fuzz smoke, classic and NLA authentication smokes, Android APK builds, gomobile AAR/API checks, FreeRDP compatibility probes, emulator capture tests, and UX PDF artifacts.
- Tag-driven CI/CD policy for build, UX, and release tag classes, including signed APK/AAB staging, SBOM, checksum, and release-note artifacts for `v*` tags.
- Security documentation in `docs/THREAT_MODEL.md` plus user-facing privacy/security copy in `docs/PRIVACY.md` covering capture, listening state, credentials, remote input, diagnostics, and recommended defaults.

Partially implemented / experimental:

- Real-client RDP compatibility. The mock server/probe path is stable, and the FreeRDP CI gate now requires `/sec:rdp`, `/sec:tls`, and `/sec:nla` to reach active state, receive bitmap updates, handle Fast-Path input, and stay connected until CI terminates the client; Microsoft-client compatibility is still pending.
- Accessibility input injection. Pointer taps/drags and frame-aware RDPEI touch contacts now reach bounded Accessibility gesture paths with continuation/multi-stroke fallback; richer keyboard/text, secondary-button behavior, gesture failure handling, and physical-device validation still need hardening.
- Performance. Slow-path 24-bit bitmap transport works and is measured; compressed bitmap/RDPGFX/H.264 work is still pending.

## Package and version

- SemVer: `0.1.1`
- Android namespace/application ID: `io.carmo.go.rdp.android`
- Android `versionCode`: `2`
- Go module: `github.com/rcarmo/go-rdp-android`

Android package IDs cannot contain hyphens. The project name `go-rdp-android` is represented as the Android package `io.carmo.go.rdp.android`.

## Repository layout

```text
android/app/                     Native Android Kotlin app
cmd/mock-server/                 Desktop mock RDP server for protocol experiments
cmd/probe/                       Scriptable RDP probe/client used by CI
features/ux/                     Gherkin-style UX user stories
internal/frame/                  Frame source abstractions and test patterns
internal/input/                  Input sink abstractions
internal/rdpserver/              Go RDP server core
mobile/                          gomobile-facing Go bridge API
scripts/                         CI helpers, artifact checks, UX report generator
docs/                            Architecture, testing, performance and release docs
```

## Documentation

- [Documentation index](docs/index.md)
- [Current project status](docs/STATUS.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Android integration](docs/ANDROID.md)
- [Testing and CI](docs/TESTING.md)
- [Debugging](docs/DEBUGGING.md)
- [Threat model](docs/THREAT_MODEL.md)
- [Privacy/security notes](docs/PRIVACY.md)
- [Performance](docs/PERFORMANCE.md)
- [Release/tag policy](docs/RELEASES.md)
- [Specification and feasibility notes](docs/SPEC.md)
- [Milestones](docs/MILESTONES.md)

## Build and test locally

Go checks:

```bash
make test
make build-go
make coverage
```

Run the desktop mock server and probe:

```bash
# terminal 1
make run-mock-pattern

# terminal 2
make probe

# or run both as a smoke test
make smoke
```

Build Android locally if the Android SDK and Gradle are available:

```bash
make android-build
```

Build the Go-backed Android APK:

```bash
make gomobile-init   # first time only
make android-build-go
```

Generate a UX report from an existing `emulator-artifacts/` directory:

```bash
npm ci
npx playwright install --with-deps chromium
make ux-report
```

## CI/CD quick reference

Default push/PR CI runs:

- Go vet/build/test/coverage.
- Race tests and parser fuzz smoke.
- Mock server + probe artifact generation.
- Android debug APK build and inspection.
- gomobile AAR build, API verification, and Go-backed APK build.
- Blocking FreeRDP compatibility probe requiring bitmap/update streaming evidence.

Current blocking FreeRDP compatibility signals are tracked in [docs/STATUS.md](docs/STATUS.md):

| Mode | Active | Bitmap/update | Fast-Path input | Screenshot | Current expected exit |
| --- | --- | --- | --- | --- | --- |
| `/sec:rdp` | ✅ | ✅ | ✅ | ✅ | `131` non-timeout shutdown after capture |
| `/sec:tls` | ✅ | ✅ | ✅ | ✅ | `131` non-timeout shutdown after capture |
| `/sec:nla` | ✅ | ✅ | ✅ | ✅ | `131` non-timeout shutdown after capture |

Manual emulator UX run:

```bash
gh workflow run CI \
  --ref main \
  -f emulator_api_level=35 \
  -f emulator_go_backed=true \
  -f emulator_capture=true \
  -f emulator_capture_scale=2
```

Tag behavior:

| Tag pattern | Behavior |
| --- | --- |
| `*-ux` | Full emulator UX validation and Playwright PDF report. |
| `*-build` | Build/test/artifact production. |
| `vX.X.X` | Release tag: build artifacts plus UX PDF report staged for release files. |

## Feature roadmap

1. **RDP compatibility hardening**
   - Improve real-client compatibility beyond the current probe/mock path.
   - Expand GCC/security/licensing/capability handling.
   - Keep expanding the now-blocking FreeRDP compatibility gate beyond bitmap/update streaming toward full clean-session behavior.

2. **Input injection completion**
   - Map RDP pointer, keyboard, Unicode, and touch events into robust Accessibility gestures and text input.
   - Add coordinate transforms for downscaled capture and rotation.
   - Extend CI scripts and future device tests for more input workflows.

3. **Performance workstreams**
   - Continue dirty-tile suppression improvements.
   - Keep single RDP sessions open for UX navigation and incremental metrics.
   - Capture pacing/backpressure now has the first production-oriented layers in place: Android adaptive capture interval, bounded queue drops, and server-side queued-frame coalescing; remaining validation is on real devices and constrained networks.
   - Expand downscale/quality modes.
   - Investigate compressed bitmap/RDPGFX updates.
   - Investigate H.264/AVC with Android hardware encoding.

4. **Security and release readiness**
   - Security mode, failed-auth backoff controls, and copyable TLS fingerprint are now surfaced in Android UI; release guidance recommends `nla-required` first and reserves `rdp-only` for isolated compatibility testing. CIDR/user allowlists remain server-core/mock-server-only for the first polished APK; continue with TLS rotation controls.
   - Continue hardening TLS Client Info and Hybrid/NLA CredSSP authentication paths against real clients.
   - Validate signed release APK/AAB staging with production secrets.
   - Validate version/tag consistency for `vX.X.X` releases.

5. **Physical-device validation**
   - Validate MediaProjection, AccessibilityService behavior, network reachability, rotation, latency, and sustained capture on real Android devices.

## Current limitations

- The app is not production-ready and should not be exposed to untrusted networks.
- The RDP server profile is intentionally minimal and not yet compatible with every client.
- Hybrid/NLA CredSSP passes current FreeRDP CI gates, but Microsoft Remote Desktop compatibility is still not guaranteed.
- Audio, clipboard, drive redirection, and full multi-monitor semantics are out of scope for the current prototype. Dynamic virtual channels are implemented only for the bounded `drdynvc`/RDPEI touch-input subset.
- MediaProjection cannot capture protected content.
- Accessibility input injection is more restricted than shell/ADB input injection.

# Documentation

Start here when working on `go-rdp-android`.

## Product and design

- [SPEC](SPEC.md) — native Android RDP server feasibility, design constraints, and protocol reuse notes.
- [ARCHITECTURE](ARCHITECTURE.md) — current Android/Kotlin, gomobile, Go RDP server, capture, graphics, input, CI, and release architecture.
- [ANDROID](ANDROID.md) — Android app/gomobile bridge, health diagnostics, capture, and H.264 status notes.
- [THREAT_MODEL](THREAT_MODEL.md) — LAN exposure, Android permissions, Accessibility, MediaProjection, auth, and storage risk model.
- [PRIVACY](PRIVACY.md) — user-facing privacy/security copy for capture, listening state, credentials, input, and diagnostics.
- [MILESTONES](MILESTONES.md) — staged implementation plan.

## Validation and operations

- [STATUS](STATUS.md) — compact current CI evidence, production-readiness matrix, blockers, and documentation update policy.
- [TESTING](TESTING.md) — CI matrix, emulator UX tests, Gherkin/Playwright report pipeline, and artifact map.
- [DEBUGGING](DEBUGGING.md) — protocol, Android, gomobile, emulator, input, UX report, performance, and release troubleshooting.
- [TRACE_PHASES](TRACE_PHASES.md) — structured server/Android trace phase taxonomy, bitmap RLE/SurfaceBits/H.264 graphics traces, and diagnostic bundle sources.
- [PERFORMANCE](PERFORMANCE.md) — RDP capture metrics, bitmap/RDPGFX/SurfaceBits/H.264 baselines/status, known first-APK performance limits, and optimization workstreams.
- [GRAPHICS_CODECS](GRAPHICS_CODECS.md) — implemented graphics paths, opt-in bitmap RLE and NSCodec/JPEG/PNG SurfaceBits diagnostics, missing/deferred RDP codec families, and codec-addition decision rules.
- [CODEC_PRIMITIVES](CODEC_PRIMITIVES.md) — policy for what stays Android-local versus what should move into `go-rdp/pkg/codec`.
- [RELEASES](RELEASES.md) — tag policy and release identifiers.

## Assets

- [icon-256.png](icon-256.png) — README/documentation icon asset derived from the master application icon.

# Project status

Last updated: 2026-05-08
Current evidence commit: `e0b8e22` (`Propagate client desktop settings into bitmap encoding path`)
Latest referenced CI run: `25567460384` (`main` CI, success)

This page is the compact, human-readable status matrix for production readiness. Keep it updated whenever protocol, input, capture, CI, or release-readiness behavior changes.

## Current CI gate summary

| Gate | Current status | Evidence / required signal |
| --- | --- | --- |
| Go unit tests | Passing | `go test ./...` in CI and locally. |
| Go vet | Passing | `go vet ./...` in CI and locally. |
| Go race/fuzz smoke | Passing in CI | Race suite plus short parser fuzz smoke. |
| Android debug APK | Passing in CI | Gradle debug build plus APK inspection artifact. |
| gomobile AAR/API | Passing in CI | `mobile.aar` build plus `scripts/check-aar-api.go`; includes touch callbacks (`TouchFrameStart`, `TouchContact`, `TouchFrameEnd`). |
| Go-backed APK | Passing in CI | Go-backed debug APK build and native-library inspection. |
| FreeRDP `/sec:rdp` | Blocking/pass | `exit_code=131` (non-timeout clean stop), `active_seen=true`, `bitmap_seen=true`, `fastpath_seen=true`, screenshot present. |
| FreeRDP `/sec:tls` | Blocking/pass | `exit_code=131` (non-timeout clean stop), `active_seen=true`, `bitmap_seen=true`, `fastpath_seen=true`, screenshot present. |
| FreeRDP `/sec:nla` | Blocking/pass | `exit_code=131` (non-timeout clean stop), `active_seen=true`, `bitmap_seen=true`, `fastpath_seen=true`, screenshot present; exercises CredSSP/NTLMv2. |
| RDPEI parser | Unit/fuzz covered | RDPEI header, ready PDUs, touch frames/contacts, optional fields, malformed packets, fuzz seed, PDU/frame/contact count bounds; CI now emits `rdpei-test-summary.md`. |
| Protocol regression fixtures | Covered in unit/probe tests | Explicit fixtures now lock in prior bugfix behavior for licensing skip (including NLA path), Client Info external terminators, Fast-Path vs slow-path input equivalence, CredSSP server-nonce `PubKeyAuth`, and `drdynvc` DATA_FIRST fragmentation reassembly. |
| `drdynvc` scaffold | Unit/fuzz covered | Static `drdynvc`, DVC caps/create/data/data-first/close, RDPEI routing, fragment assembly, caps-before-lifecycle enforcement, unsupported/duplicate/second RDPEI create rejection, size bounds, fragment limits, stale-fragment cleanup, unexpected channel IDs, simultaneous fragments, close/reopen, variable-length channel IDs, and synthetic caps→create→RDPEI touch integration sequence. |
| RDPEI touch lifecycle | Unit covered | Down/update/up, cancellation, duplicate contact IDs, reordered/stray events, multi-contact ordering, and optional rectangle/orientation/pressure metadata preservation. |
| Android emulator UX | Optional/tag/manual | Full UX path runs for `*-ux` and release tags; default push does not run the emulator capture path. Scene plans now support synthetic `rdpei-tap` actions (via `drdynvc` + RDPEI) in addition to pointer taps. Latest on-demand `workflow_dispatch` evidence (`25517361134`) passed with Go-backed capture + UX report generation. |

## FreeRDP compatibility snapshot

Latest checked artifact from CI run `25567460384`:

| Mode | TCP | X.224 | MCS | Active | Bitmap/update | Fast-Path input | Screenshot | Exit code |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `/sec:rdp` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | `131` |
| `/sec:tls` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | `131` |
| `/sec:nla` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | `131` |

The compatibility gate now performs a non-timeout clean stop of the FreeRDP client after active streaming/screenshot capture and requires `exit_code != 124`.

## Protocol and input readiness

| Area | Status | Notes |
| --- | --- | --- |
| RDP negotiation | Prototype-compatible | X.224, TLS, Hybrid/NLA, MCS, GCC response, licensing, activation, bitmap streaming. Connect-Initial now parses client core desktop + monitor-layout metadata, Confirm Active parsing summarizes bitmap/input/order/virtual-channel/large-pointer capabilities (including desktop-resize flag), and session desktop sizing is propagated into bitmap encoding scale. |
| Authentication | Prototype-compatible | TLS Client Info auth and Hybrid/NLA CredSSP/NTLMv2 work against current probes/FreeRDP. Defensive CredSSP pubKeyAuth nonce/order binding variants are intentionally retained for interoperability, with explicit comments and table-driven coverage. Production credential storage/policy is pending. |
| Graphics | Functional baseline | 24-bit BGR slow-path bitmap tiles with dirty-tile suppression. Compression/RDPGFX/H.264 pending. |
| Classic input | Functional baseline | Slow-path and Fast-Path pointer/keyboard/Unicode decoding with explicit sink-equivalence coverage; Android coalesces primary pointer down/move/up into bounded Accessibility strokes; wheel events are decoded/bridged/logged with safe Android degradation; keyboard/text, secondary-button behavior, and physical-device validation remain pending. |
| True RDP touch | Frame-aware bridge with continuation scaffolding | RDPEI over `drdynvc` parses bounded payloads and routes contacts plus optional rectangle/orientation/pressure metadata through a lifecycle coalescer; gomobile now forwards touch frame boundaries and Android consumes per-frame contact batches, building bounded `GestureDescription` strokes with `continueStroke(...)` chaining for active contacts and grouped frame dispatch for coordinated contacts, with single-contact fallback when multi-stroke dispatch is rejected. Real-client/physical-device multi-touch evidence is still pending. |
| Android capture | Functional prototype | MediaProjection + ImageReader capture skeleton with test-pattern mode, pacing/backpressure, optional downscale. Physical-device validation pending. |

## Production blockers

- No physical Android device validation yet.
- Microsoft Remote Desktop compatibility is not yet validated.
- FreeRDP CI now enforces non-timeout shutdown; protocol-native logoff/deactivate behavior from diverse real clients still needs broader validation.
- Security defaults are not production-safe yet: credential setup/storage, TLS certificate persistence, auth policy, rate limiting, and threat model remain pending.
- Android Accessibility gesture behavior needs real-device validation, especially for drags, long gestures, text input, and multi-touch degradation.
- Graphics pipeline is still raw/slow-path-first; compressed bitmap/RDPGFX/H.264 are pending.

## Documentation update policy

Update this page together with the relevant feature docs when changing behavior:

- `README.md` for user-facing feature status and CI quick reference.
- `docs/ARCHITECTURE.md` for design or data-flow changes.
- `docs/SPEC.md` for feasibility/protocol scope changes.
- `docs/TESTING.md` for CI gates, artifacts, or validation commands.
- `docs/PERFORMANCE.md` for capture/graphics metrics or performance decisions.
- `docs/MILESTONES.md` and `/workspace/workitems/` for roadmap state.
- `docs/DEBUGGING.md` for new failure modes and troubleshooting recipes.

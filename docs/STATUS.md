# Project status

Last updated: 2026-05-17
Current evidence commit: `e4ca8e0` (`Refresh FreeRDP soak evidence`)
Latest referenced CI run: `26079735177` (`main` CI, success)

This page is the compact, human-readable status matrix for production readiness. Keep it updated whenever protocol, input, capture, CI, or release-readiness behavior changes.

## Current CI gate summary

| Gate | Current status | Evidence / required signal |
| --- | --- | --- |
| Go unit tests | Passing | `go test ./...` in CI and locally. |
| Go vet | Passing | `go vet ./...` in CI and locally. |
| Go race/fuzz smoke | Passing in CI | Race suite plus short parser fuzz smoke. |
| gosec static scan | Passing in CI | `gosec` JSON + markdown summary emitted in Go artifacts; current scan excludes `G115` cast-noise and is clean for remaining rules. |
| Android debug APK | Passing in CI | Gradle debug build plus APK inspection artifact. |
| gomobile AAR/API | Passing in CI | `mobile.aar` build plus `scripts/check-aar-api.go`; includes touch callbacks (`TouchFrameStart`, `TouchContact`, `TouchFrameEnd`). |
| Go-backed APK | Passing in CI | Go-backed debug APK build and native-library inspection. |
| FreeRDP `/sec:rdp` | Blocking/pass | `exit_code=131` (non-timeout clean stop), `active_seen=true`, `bitmap_seen=true`, `fastpath_seen=true`, screenshot present. |
| FreeRDP `/sec:tls` | Blocking/pass | `exit_code=131` (non-timeout clean stop), `active_seen=true`, `bitmap_seen=true`, `fastpath_seen=true`, screenshot present. |
| FreeRDP `/sec:nla` | Blocking/pass | `exit_code=131` (non-timeout clean stop), `active_seen=true`, `bitmap_seen=true`, `fastpath_seen=true`, screenshot present; exercises CredSSP/NTLMv2. |
| FreeRDP `/sec:nla /gfx` | Blocking/pass | RDPGFX proof gate with `rdpgfx_seen=true`, `active_seen=true`, `fastpath_seen=true`, screenshot present, and `exit_code=131`; CI disables RDPGFX only for the three bitmap fallback gates. |
| CI diagnostic artifacts | Passing | Mock/probe, auth, FreeRDP, Android build, gomobile, and emulator/UX paths emit or preserve relevant mock-server/client logs, JSON/Markdown summaries, screenshots, and inspection artifacts where applicable. Server trace logs can be enabled with legacy `GO_RDP_ANDROID_TRACE=1` or `GO_RDP_ANDROID_LOG_LEVEL=trace/debug`. |
| RDPEI parser | Unit/fuzz covered | RDPEI header, ready PDUs, touch frames/contacts, optional fields, malformed packets, fuzz seed, PDU/frame/contact count bounds; CI now emits `rdpei-test-summary.md`. |
| Protocol regression fixtures | Covered in unit/probe tests | Explicit fixtures now lock in prior bugfix behavior for licensing skip (including NLA path), Client Info external terminators, Fast-Path vs slow-path input equivalence, CredSSP server-nonce `PubKeyAuth`, auth success/failure smoke outcomes, and `drdynvc` DATA_FIRST fragmentation reassembly plus DVC fragment counter accounting. |
| `drdynvc` scaffold | Unit/fuzz covered | Static `drdynvc`, DVC caps/create/data/data-first/close, RDPEI routing, fragment assembly, caps-before-lifecycle enforcement, unsupported/duplicate/second RDPEI create rejection, size bounds, fragment limits, stale-fragment cleanup, unexpected channel IDs, simultaneous fragments, close/reopen, variable-length channel IDs, and synthetic caps→create→RDPEI touch integration sequence. |
| RDPEI touch lifecycle | Unit covered | Down/update/up, cancellation, duplicate contact IDs, reordered/stray events, multi-contact ordering, and optional rectangle/orientation/pressure metadata preservation. |
| Android emulator UX | Optional/tag/manual | Full UX path runs for `*-ux` and release tags; default push does not run the emulator capture path. Scene plans now support synthetic `rdpei-tap` actions (via `drdynvc` + RDPEI) in addition to pointer taps. Latest on-demand `workflow_dispatch` evidence (`25517361134`) passed with Go-backed capture + UX report generation. |
| FreeRDP soak (nightly/dispatch) | Optional | Dedicated `FreeRDP soak` workflow runs repeated security-mode sessions and fails on configurable server RSS growth, producing per-iteration CSV/log artifacts for stability analysis. Iterations now have a hard timeout and escalated shutdown path to prevent stuck-client runs; recent scheduled evidence: `26076121092` (success, 30 `/sec:nla` iterations, RSS delta 5,248 KB vs 51,200 KB threshold). |
| Release file checks | Tag-only | `v*` release staging signs APK (`apksigner`) and AAB (`jarsigner`) with production keystore secrets from GitHub Actions, verifies signature reports, emits CycloneDX Go SBOM, and ships SHA-256 checksums with explicit artifact retention. |
| GitHub Actions JS runtime | Updated | Workflows now opt into Node 24 execution (`FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true`) to preempt Node 20 deprecation cutover risk. |

## FreeRDP compatibility snapshot

Latest checked artifact from CI run `26079735177`:

| Mode | TCP | X.224 | MCS | Active | Bitmap/update | RDPGFX | Fast-Path input | Screenshot | Exit code |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `/sec:rdp` | ✅ | ✅ | ✅ | ✅ | ✅ | — | ✅ | ✅ | `131` |
| `/sec:tls` | ✅ | ✅ | ✅ | ✅ | ✅ | — | ✅ | ✅ | `131` |
| `/sec:nla` | ✅ | ✅ | ✅ | ✅ | ✅ | — | ✅ | ✅ | `131` |
| `/sec:nla /gfx` | ✅ | ✅ | ✅ | ✅ | — | ✅ | ✅ | ✅ | `131` |

The compatibility gate now performs a non-timeout clean stop of the FreeRDP client after active streaming/screenshot capture and requires `exit_code != 124`.

## Protocol and input readiness

| Area | Status | Notes |
| --- | --- | --- |
| RDP negotiation | Prototype-compatible | X.224, TLS, Hybrid/NLA, MCS, GCC response, licensing, activation, bitmap streaming. Connect-Initial now parses client core desktop + monitor-layout metadata, Confirm Active parsing summarizes bitmap/input/order/virtual-channel/large-pointer capabilities (including desktop-resize flag), and session desktop sizing is propagated into bitmap encoding scale. |
| Authentication | Prototype-compatible | TLS Client Info auth and Hybrid/NLA CredSSP/NTLMv2 work against current probes/FreeRDP. Defensive CredSSP pubKeyAuth nonce/order binding variants are intentionally retained for interoperability, with explicit comments and table-driven coverage. Android app flow now requires explicit credential setup before service start and persists configured credentials encrypted-at-rest (Android Keystore-backed AES/GCM) for subsequent starts; log user/domain fields are now bounded/sanitized. Server-side bcrypt hashed credential verification is available for TLS Client Info flows, while current NLA path still requires plaintext-equivalent credential input. Server policy controls now support security-mode selection (`negotiate`/`rdp-only`/`tls-only`/`nla-required`) plus allowed-users/CIDR allowlists, TLS cert persistence/rotation with handshake fingerprint logging for trust guidance, and optional failed-auth lockout/backoff policy controls. |
| Graphics | RDPGFX compressed path implemented; experimental H.264-over-RDPGFX emission added | RDPGFX (`Microsoft::Windows::RDS::Graphics`) over `drdynvc` is enabled by default and sends Planar-codec no-alpha RLE frames through server-initiated dynamic channels. The CI `/sec:nla /gfx` proof gate shows active RDPGFX negotiation/streaming while the existing 24-bit BGR slow-path bitmap gates remain available as explicit compatibility fallback and benchmark oracle. The next graphics workstream is full-spectrum Android `MediaCodec` H.264/AVC encoding layered ahead of RDPGFX only after client/transport support is proven; initial Android encoder-surface scaffolding, experimental direct-service `h264_capture`, bounded gomobile encoded-frame submission, diagnostics, access-unit validation, `GO_RDP_ANDROID_DISABLE_H264=1`, and initial RDPGFX AVC420 `WireToSurface_1` emission/streaming from Annex-B-normalized queued H.264 access units wrapped as `RFX_AVC420_BITMAP_STREAM` and gated on confirmed AVC420-capable RDPGFX flags exist; Android now forwards `MediaCodec` format `csd-*` codec config into the encoded-frame queue, server-side config+keyframe combinations are bounded before transport and IDR NAL units are treated as keyframes, H.264 byte counters track encoded access-unit bytes, server traces include `rdpgfx_h264_status` capability/opt-out decisions, FreeRDP summaries expose H.264 status/write trace booleans, H.264 write count/bytes, the status reason, and `/gfx:AVC420` exit code, the mock server can replay an encoded H.264 access-unit file for protocol-only AVC420 experiments, and CI now collects a non-blocking informational `h264-gfx` probe artifact. CI run `26030409374` showed the SPS/PPS/IDR fixture still emitted only IDR bytes because mock replay did not mark SPS/PPS units as codec config; mock replay now marks/coalesces SPS/PPS as `CodecConfig`. CI run `26057766382` confirmed the forced H.264 probe now emits coalesced SPS+PPS+IDR bytes (`h264_write_count=1`, `h264_write_bytes=23`, `h264_reason=forced-by-env`, `avc420_exit_code=24`). CI run `26018773789` showed FreeRDP `/gfx` active with `h264_status_seen=true`, `h264_write_seen=false`, and `h264_reason=client-avc420-not-advertised` (`flags=0x00000020` / AVC disabled), so H.264 client proof remains pending rather than failed. CI run `26019833294` showed the runner FreeRDP rejected `/gfx:AVC420` with exit code `24`, so the informational probe now records that attempt and falls back to `/gfx` for capability-status evidence on older FreeRDP builds. CI run `26020283162` confirmed the fallback path works: the H.264 probe reached active `/gfx`, recorded `avc420-exit-code=24`, `h264_status_seen=true`, `h264_write_seen=false`, and `h264_reason=client-avc420-not-advertised`. The informational H.264 probe now runs with `GO_RDP_ANDROID_FORCE_H264=1` so CI can collect server-side AVC420 emission traces even while true client AVC420 support remains unproven. CI run `26022000155` confirmed that forced path emits H.264 RDPGFX traffic (`h264_status_seen=true`, `h264_write_seen=true`, `h264_write_count=1`, `h264_write_bytes=7`, `h264_reason=forced-by-env`, `avc420_exit_code=24`, active fallback `/gfx` session). User-facing UI, client proof, and physical performance evidence are still pending. See `/workspace/workitems/10-next/go-rdp-android-h264-full-spectrum-encoding.md`. |
| Classic input | Functional baseline | Slow-path and Fast-Path pointer/keyboard/Unicode decoding with explicit sink-equivalence coverage; input/RDPEI metric wrappers now tolerate nil sinks and nil counters for safer test/prototype reuse; Android coalesces primary pointer down/move/up into bounded Accessibility strokes; wheel events are decoded/bridged/logged with safe Android degradation; keyboard/text, secondary-button behavior, and physical-device validation remain pending. |
| True RDP touch | Frame-aware bridge with continuation scaffolding | RDPEI over `drdynvc` parses bounded payloads and routes contacts plus optional rectangle/orientation/pressure metadata through a lifecycle coalescer; gomobile now forwards touch frame boundaries and Android consumes per-frame contact batches, building bounded `GestureDescription` strokes with `continueStroke(...)` chaining for active contacts and grouped frame dispatch for coordinated contacts, with single-contact fallback when multi-stroke dispatch is rejected. Android now avoids retaining in-progress touch frames when Accessibility is disconnected or disappears mid-frame, preventing stale RDPEI state from leaking across service disable/reconnect paths. Real-client/physical-device multi-touch evidence is still pending. |
| Android capture | Functional prototype | MediaProjection + ImageReader capture skeleton with test-pattern mode, pacing/backpressure, optional downscale. Long-running server starts now always run as a foreground service with a notification Stop action, explicit UI stop routing, serialized mode switching, projection-revocation cleanup (last mode reset + notification removal), missing-credential source/listener cleanup, non-sticky restart policy, network-change logging/notification refresh, and permission-denial recovery that leaves the server stopped without storing a failed capture mode/scale. Non-secret server settings (capture scale, selected security mode, failed-auth policy, and last successful/explicit mode) are persisted separately from encrypted credentials and restored across Activity/process recreation; service restarts remain explicit. The UI can select core security policy (`negotiate`, `rdp-only`, `tls-only`, `nla-required`) plus failed-auth backoff/lockout settings, shows first-run/start checklists, inline settings help, compact backend/running/mode/security/auth/listen-address/TLS-fingerprint/client-count/accepted/auth-failure/handshake-failure/input-enabled/input-event/RDPEI-contact/DVC-fragment/submitted-frame/sent-frame/bitmap-byte/queue/drop/input-scale health state plus a bounded selectable in-app debug panel and copyable full TLS fingerprint, and can share bounded redacted diagnostics with that health plus non-secret settings. Native mobile restarts drain stale queued frames before/after server lifecycle transitions, the frame queue drain path handles already-closed queues without hanging, and the mobile bridge now binds synchronously so listen failures are reported before startup is treated as successful, with foreground-service teardown and last-mode reset on native startup failure. Server listener context-watcher goroutines now also stop when `Serve` exits for non-context reasons. The UI guides the user when the AccessibilityService is disabled. Physical-device validation pending. |

## Production blockers

- No physical Android device validation yet.
- Microsoft Remote Desktop compatibility is not yet validated.
- FreeRDP CI now enforces non-timeout shutdown; protocol-native logoff/deactivate behavior from diverse real clients still needs broader validation.
- Security defaults are not fully production-safe yet: release docs now recommend `nla-required` first, `tls-only` for non-NLA clients, and `rdp-only` only for isolated compatibility testing; allowlists are server-core/mock-server-only for the first polished APK, and Android TLS certificate rotation remains pending.
- Android Accessibility gesture behavior needs real-device validation, especially for drags, long gestures, text input, and multi-touch degradation.
- Graphics now has a default RDPGFX Planar path plus explicit slow-path bitmap fallback evidence in CI. Remaining graphics blockers are physical-device/constrained-network validation, Microsoft-client validation, and performance comparison on target hardware.
- Release preflight diagnostic mode passes clean/synced/version/latest-CI checks, but signing secret presence could not be confirmed from automation (`gh secret list --repo rcarmo/go-rdp-android` returned no visible repository secrets again on 2026-05-17); controlled `v*` release-candidate/dry-run tagging is blocked until the repository owner confirms `RELEASE_KEYSTORE_BASE64`, `RELEASE_KEYSTORE_PASSWORD`, `RELEASE_KEY_ALIAS`, and `RELEASE_KEY_PASSWORD`.

## Documentation update policy

Update this page together with the relevant feature docs when changing behavior:

- `README.md` for user-facing feature status and CI quick reference.
- `docs/ARCHITECTURE.md` for design or data-flow changes.
- `docs/SPEC.md` for feasibility/protocol scope changes.
- `docs/TRACE_PHASES.md` for server/Android trace phase and diagnostic-bundle changes.
- `docs/THREAT_MODEL.md` for LAN exposure, Android permission, auth, storage, or security-default changes.
- `docs/PRIVACY.md` for user-facing capture/listening/input/diagnostic/security-default copy.
- `docs/TESTING.md` for CI gates, artifacts, or validation commands.
- `docs/PERFORMANCE.md` for capture/graphics metrics or performance decisions.
- `docs/MILESTONES.md` and `/workspace/workitems/` for roadmap state.
- `docs/DEBUGGING.md` for new failure modes and troubleshooting recipes.

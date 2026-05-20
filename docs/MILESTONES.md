# Milestones

## M0 — Scaffold

- Project layout
- Go RDP server skeleton
- Android Kotlin shell
- MediaProjection/ImageReader capture scaffold ✅
- AccessibilityService scaffold ✅
- GitHub Actions for Go and Android builds ✅

## M1 — Desktop RDP protocol mock

Goal: run a local Go process that accepts an RDP TCP connection and advances through the initial protocol phases.

Tasks:
- TPKT/X.224 Connection Request parsing ✅
- X.224 Connection Confirm response ✅
- MCS Connect-Initial parsing (header/app tag) ✅
- MCS Connect-Response writing ✅ (minimal scaffold)
- MCS ErectDomainRequest handling ✅
- MCS AttachUserRequest/Confirm handling ✅
- MCS ChannelJoinRequest/Confirm handling ✅
- GCC Conference Create Response ✅ (server core/security/network data blocks)
- SendDataRequest parsing ✅
- Security Exchange / Client Info parsing ✅
- TLS-only Client Info credential gate ✅
- Hybrid/NLA CredSSP/NTLMv2 credential gate ✅ (experimental)
- Server Demand Active PDU ✅ (minimal capability scaffold)
- Client Confirm Active PDU parsing ✅
- Synchronize/Control/FontList finalization handling ✅ (minimal scaffold)
- solid-color slow-path bitmap update ✅
- initial frame.Source-to-bitmap conversion ✅
- TPKT-safe frame tiling into bitmap update PDUs ✅
- animated test-pattern frame source ✅
- continuous frame streaming loop ✅ (frame.Source-backed)
- slow-path keyboard/mouse input decoding ✅
- basic capability exchange ✅
- one-session state machine ✅ (listener address cleanup and context-watcher shutdown covered)

## M2 — Android frame bridge

Goal: get actual Android frames across the Kotlin→Go seam.

Tasks:
- generate gomobile AAR ✅
- route `NativeRdpBridge` to gomobile with logging fallback ✅
- move ImageReader buffers into Go frame source ✅
- add frame throttling/downscaling/backpressure ✅ (Android adaptive pacing, bounded queue drops, server-side queued-frame coalescing)

## M3 — Graphics updates

Goal: send visible Android frames to an RDP client with both compatibility fallback and compressed transport.

Tasks:
- raw bitmap update sender ✅
- dirty tile suppression ✅
- negotiated/session desktop sizing ✅
- RDPGFX dynamic-channel negotiation and Planar compressed frames ✅ (FreeRDP `/sec:nla /gfx` CI proof; physical-device/Microsoft-client performance evidence pending)
- experimental H.264/AVC over RDPGFX AVC420 ✅/partial (Android `MediaCodec` scaffold, encoded-frame queue, AVC420 `RFX_AVC420_BITMAP_STREAM` emission/streaming, codec-config/coalescing hardening, `h264Status` diagnostics, and forced non-blocking CI artifact evidence with capability fields; true client AVC420 compatibility and physical-device performance pending)
- optional legacy/extra codecs (bitmap RLE, NSCodec, RemoteFX/RFX, AVC444/AVC444v2, ClearCodec, progressive, JPEG/PNG bitmap codecs) — bitmap RLE is now an experimental opt-in (`GO_RDP_ANDROID_ENABLE_BITMAP_RLE=1`) with COPY/color-order coverage, expansion rejection, local matrix saved-byte evidence, and Android/gomobile diagnostics, but it is still not negotiated or enabled by default; add remaining codecs only if client evidence justifies them; tracked in `/workspace/workitems/10-next/go-rdp-android-graphics-codec-coverage.md`

## M4 — Input

Goal: translate RDP input into Android interactions.

Tasks:
- decode pointer/keyboard input ✅ (slow-path and Fast-Path)
- Kotlin input callback surface ✅
- Accessibility gesture injection ✅/partial (taps/drags/RDPEI strokes with stale-frame cleanup across service disconnects; keyboard/text and failure callbacks pending)
- true RDP touch support via RDPEI/dynamic virtual channels (`drdynvc`) ✅/partial (protocol/sink/Android bridge done; broad real-device validation pending)
- text/clipboard handling plan

## M5 — Usability/security

- TLS and pairing/password auth ✅ (static credentials, bcrypt TLS Client Info, cert persistence/rotation/fingerprint)
- Hybrid/NLA CredSSP auth ✅ (experimental, FreeRDP-gated)
- foreground notification controls ✅ (foreground start for all server modes, serialized mode switching, notification/UI Stop action, missing-credential source/listener cleanup, permission-denial cleanup, projection-revocation shutdown, non-sticky restart policy, native-start failure teardown)
- reconnect handling
- settings UI ✅/partial (credentials, capture scale, security mode, failed-auth policy, copyable TLS fingerprint, first-run/start checklists, inline settings help, last mode, start/stop, compact health, redacted diagnostics sharing, common troubleshooting snippets, failed-start last-mode reset; allowlists/TLS rotation controls pending)
- optional live CI smoke tests with emulator ✅

# Milestones

## M0 — Scaffold (current)

- Project layout
- Go RDP server skeleton
- Android Kotlin shell
- MediaProjection/ImageReader capture scaffold ✅
- AccessibilityService scaffold
- GitHub Actions for Go and Android builds

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
- basic capability exchange
- one-session state machine

## M2 — Android frame bridge

Goal: get actual Android frames across the Kotlin→Go seam.

Tasks:
- generate gomobile AAR
- replace `NativeRdpBridge` stub
- move ImageReader buffers into Go frame source
- add frame throttling/downscaling

## M3 — Bitmap updates

Goal: send visible Android frames to an RDP client.

Tasks:
- raw bitmap update sender
- dirty region detection
- optional RLE encoding
- fixed resolution first

## M4 — Input

Goal: translate RDP input into Android interactions.

Tasks:
- decode pointer/keyboard input ✅ (slow-path and Fast-Path)
- Kotlin input callback surface ✅
- Accessibility gesture injection
- true RDP touch support via RDPEI/dynamic virtual channels (`drdynvc`)
- text/clipboard handling plan

## M5 — Usability/security

- TLS and pairing/password auth ✅ (static credential scaffold)
- Hybrid/NLA CredSSP auth ✅ (experimental)
- foreground notification controls ✅ (foreground start for all server modes plus notification/UI Stop action)
- reconnect handling
- settings UI
- optional live CI smoke tests with emulator

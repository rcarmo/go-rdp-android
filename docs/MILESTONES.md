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
- GCC Conference Create Response ✅ (minimal scaffold)
- SendDataRequest parsing ✅
- Security Exchange / Client Info parsing ✅ (minimal scaffold)
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
- decode pointer/keyboard input
- Kotlin input callback surface
- Accessibility gesture injection
- text/clipboard handling plan

## M5 — Usability/security

- TLS and pairing/password auth
- foreground notification controls
- reconnect handling
- settings UI
- optional live CI smoke tests with emulator

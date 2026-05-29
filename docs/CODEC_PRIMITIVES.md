# Codec primitive sharing policy

This note records which graphics codec primitives should live in `github.com/rcarmo/go-rdp` versus `go-rdp-android` while the Android server remains an experimental implementation.

## Already shared through go-rdp

`go-rdp` currently exports the stable client/interoperability primitives that are safe for other projects to consume:

- Bitmap Codecs GUIDs and symbolic names from `pkg/codec`:
  - NSCodec
  - RemoteFX
  - RemoteFX-Image
  - JPEG
- RDPGFX WireToSurface codec IDs and names:
  - Uncompressed
  - CAVideo
  - ClearCodec
  - CAProgressive
  - Planar
  - AVC420
  - Alpha
  - CAProgressiveV2
  - AVC444
  - AVC444v2
- NSCodec encode/decode helpers:
  - `EncodeNSCodecRawBGRA`
  - `EncodeNSCodecRawRGBA`
  - `DecodeNSCodec`

`go-rdp-android` should continue importing these rather than duplicating GUID/ID/name tables or NSCodec raw-plane helpers.

## Keep Android-local for now

The following encoders are deliberately Android-local until their payload semantics and client compatibility are stable enough to become a reusable API:

- Classic bitmap update emitters:
  - 8/15/16/24 bpp raw bitmap tiling
  - 8 bpp grayscale palette update
  - experimental COPY/color-order bitmap RLE subset
  - classic RDP6 bitmap-update Planar emitter
- SurfaceBits emitters:
  - JPEG command builder and quality policy
  - PNG operator-override command builder and compression-level policy
  - RemoteFX single-tile production encoder and SurfaceBits command wiring
- RDPGFX server encoders:
  - Planar frame PDU assembly
  - Uncompressed diagnostic frame PDU assembly
  - ClearCodec partial/minimal tiled solid/raw encoder
  - CAProgressive/CAProgressiveV2 partial/minimal payload/encoder
  - AVC420 transport wrapper around Android `MediaCodec` access units
  - AVC444/AVC444v2 bounded base/aux placeholder payloads

These pieces are still server-policy-heavy: they depend on Android frame formats, release-default gating, diagnostic environment variables, negotiated fallback behavior, matrix evidence, and partial/minimal encoder caveats. Exporting them from `go-rdp` now would freeze unstable APIs and blur the distinction between client metadata support, fixture transport hooks, partial production encoders, and release defaults.

## Promote later when stable

A primitive is ready to move into `go-rdp/pkg/codec` only when all of the following are true:

1. The wire payload is codec-semantic rather than Android server policy.
2. It has deterministic encode/decode or encode/parse tests independent of Android server metrics and environment toggles.
3. At least one real client advertises and accepts the codec without fixture-only forcing, or upstream `go-rdp` gains a matching client decode path.
4. The API can be expressed as a small data-in/data-out package function without requiring server negotiation state.
5. The Android server can depend on the exported function without losing its current fallback, bounds, and diagnostic behavior.

Likely future promotion candidates are:

- RFX tile/message encoding once multi-tile semantics and client proof are stronger.
- ClearCodec payload building once the supported operation subset is larger and spec-aligned.
- Progressive payload parsing/building if `go-rdp` gains a real progressive decode path.
- AVC444/AVC444v2 payload helpers only after real auxiliary-plane generation replaces deterministic placeholder access units.

## Current decision

No additional codec primitives should be moved during the current parity slice. The stable shared surface remains `go-rdp/pkg/codec`; experimental production encoders and Android-specific transport/gating remain in `go-rdp-android/internal/rdpserver` until the promotion criteria above are met.

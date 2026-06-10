# Codec primitive sharing policy

This note records which graphics codec primitives should live in `github.com/rcarmo/go-rdp` versus `go-rdp-android` while the Android server remains an experimental implementation.

## Already shared through go-rdp

`go-rdp` currently exports stable client/interoperability primitives and reusable wire helpers that are safe for other projects to consume:

- Bitmap Codecs GUIDs, symbolic names, and capability parsing from `pkg/codec`:
  - NSCodec
  - RemoteFX
  - RemoteFX-Image
  - JPEG
  - Bitmap Codecs capability entries/properties
- RDPGFX capability, AVC flag, WireToSurface, SetSurfaceBits, and frame PDU helpers.
- Classic Bitmap Update helpers:
  - raw and compressed bitmap update builders
  - conservative classic bitmap RLE encoder
  - tile splitting and no-compression-header handling
- RDPGFX payload and wire helpers:
  - Planar no-alpha encoder
  - uncompressed bitmap payload conversion
  - JPEG payload and SurfaceBits integration helpers
  - minimal ClearCodec solid/raw subset
  - Progressive/ProgressiveV2 payload parser/builder
  - AVC420 bitmap streams and AVC444/AVC444v2 stream wrappers/validators
- RemoteFX/RFX encoder primitives:
  - RGB/BGRA/RGBA to YCoCg
  - DWT 5/3, quantization, RLGR, component serialization
  - tile block/message and single-tile frame helpers
- NSCodec encode/decode helpers:
  - `EncodeNSCodecRawBGRA`
  - `EncodeNSCodecRawRGBA`
  - `DecodeNSCodec`

`go-rdp-android` should import these rather than duplicating GUID/ID/name tables, protocol-only payload builders, or reusable codec primitives. Android-specific frame capture, `MediaCodec` policy, diagnostics, release-default gates, negotiated fallback behavior, and compatibility evidence remain local.

## Upstream replacement follow-up

After `go-rdp-android` updates its `github.com/rcarmo/go-rdp` dependency to include the upstream helper series, replace local protocol-only code with `go-rdp/pkg/codec` APIs in small reviewable changes:

- [ ] Replace local Bitmap Codecs/RDPGFX capability parsing, AVC flag formatting, SetSurfaceBits, WireToSurface, StartFrame, EndFrame, CreateSurface, and MapSurface helpers with upstream equivalents.
- [ ] Replace local raw/compressed classic Bitmap Update envelope and conservative RLE/tile-splitting helpers with upstream equivalents while preserving Android-local encoder selection/fallback policy.
- [ ] Replace local Planar payload encoding and frame PDU assembly helpers with upstream Planar and RDPGFX frame helpers; keep Android capture normalization and release gating local.
- [ ] Replace local uncompressed diagnostic payload conversion with upstream uncompressed helpers.
- [ ] Replace local JPEG payload/SurfaceBits wire construction with upstream JPEG helpers; keep Android quality policy and diagnostic gates local.
- [ ] Replace reusable RemoteFX/RFX primitives and single-tile message construction with upstream RFX helpers; keep compatibility gating and client evidence local.
- [ ] Replace local minimal ClearCodec solid/raw payload builder with the upstream documented subset; keep operation enablement policy local.
- [ ] Replace local Progressive/ProgressiveV2 payload parse/build utilities with upstream helpers; keep any diagnostic/fixture forcing local.
- [ ] Replace AVC420 bitmap stream and AVC444/AVC444v2 stream wrapper validation with upstream wire helpers; keep Android `MediaCodec` feeding, access-unit queueing, and encoder policy local.

## Keep Android-local for now

The following pieces remain Android-local because they depend on runtime policy or platform behavior rather than reusable protocol payload semantics:

- Android frame capture, stride/rotation/display normalization, and service lifecycle handling.
- Codec selection, fallback, release-default gating, diagnostic environment variables, and compatibility matrix evidence.
- Android `MediaCodec` configuration, access-unit queueing, SPS/PPS handling policy, and hardware encoder availability decisions.
- PNG operator-override command builder and compression-level policy until a corresponding upstream helper exists.
- Any client-specific workaround that is not a small deterministic data-in/data-out protocol helper.

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

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

## Upstream replacement status

`go-rdp-android` now depends on the pushed `github.com/rcarmo/go-rdp/pkg/codec` helper series and uses those APIs for protocol-only helpers. The Android server keeps platform capture, diagnostics, fallback, release-default gating, and client policy local.

- [x] Replaced local Bitmap Codecs/RDPGFX capability parsing, AVC flag formatting, SetSurfaceBits, WireToSurface, StartFrame, EndFrame, CreateSurface, and MapSurface helpers with upstream equivalents.
- [x] Replaced local raw/compressed classic Bitmap Update envelope and conservative RLE helpers with upstream equivalents while preserving Android-local encoder selection/fallback policy and classic Planar’s 32-bpp Bitmap Update exception.
- [x] Replaced local Planar payload encoding and frame PDU assembly helpers with upstream Planar and RDPGFX frame helpers; Android capture normalization and release gating remain local.
- [x] Replaced local uncompressed diagnostic payload conversion with upstream uncompressed helpers.
- [x] Replaced local JPEG payload/SurfaceBits wire construction with upstream JPEG helpers; Android quality policy and diagnostic gates remain local.
- [x] Replaced production RemoteFX/RFX single-tile message construction with upstream RFX helpers; compatibility gating and client evidence remain local.
- [x] Replaced local minimal ClearCodec solid/raw payload builder with the upstream documented subset; operation enablement policy remains local.
- [x] Replaced local Progressive/ProgressiveV2 payload parse/build utilities with upstream helpers; diagnostic/fixture forcing remains local.
- [x] Replaced AVC420 bitmap stream and AVC444/AVC444v2 stream wrapper validation/builders with upstream wire helpers; Android `MediaCodec` feeding, access-unit queueing, SPS/PPS handling, and encoder fallback remain local.

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

Likely future promotion candidates are any remaining codec features that are still Android-policy-coupled today, such as multi-tile RFX scheduling, richer ClearCodec operations, real Progressive encoding, or AVC444 auxiliary-plane generation once they can be expressed as deterministic data-in/data-out helpers.

## Current decision

Reusable protocol primitives should live in `go-rdp/pkg/codec`; Android-specific transport, capture, `MediaCodec` orchestration, diagnostics, fallback, compatibility evidence, and release gates remain in `go-rdp-android/internal/rdpserver`.

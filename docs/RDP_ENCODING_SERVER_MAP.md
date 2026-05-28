# go-rdp client encoding inventory mapped to go-rdp-android server support

This map turns `docs/RDP_ENCODING_INVENTORY.md` into implementation status. Status meanings:

- **production encoder**: real server emission path exists with capability/feature gate, tests, and metrics/matrix hooks.
- **partial/minimal encoder**: emits a useful subset but does not cover the corresponding go-rdp client decode/advertise surface.
- **fixture hook only**: transport can carry operator-provided bytes, but no real server encoder exists.
- **diagnostic override**: works only through an operator override, not negotiated capability parity.
- **missing**: no server encoder/emitter yet.
- **metadata-only pending proof**: go-rdp names/parses the codec but no client decode path has been confirmed yet.

## Classic bitmap update encodings

| Encoding | go-rdp client support | go-rdp-android server status | Gap / next action |
| --- | --- | --- | --- |
| Raw bitmap 24/32 bpp | Decodes uncompressed 24/32 plus other depths | **production encoder** for 24 bpp BGR fallback | Keep as compatibility fallback/oracle. |
| Raw bitmap 8 bpp palette | Decodes palette-indexed 8 bpp using palette state | **missing / intentionally not negotiated pending proof** | Either constrain negotiation to 24/32 bpp and document, or implement palette update + 8 bpp emission. |
| Raw bitmap 15 bpp RGB555 | Decodes RGB555 | **missing / intentionally not negotiated pending proof** | Either constrain negotiation to 24/32 bpp and document, or implement RGB555 emission. |
| Raw bitmap 16 bpp RGB565 | Decodes RGB565 | **missing / intentionally not negotiated pending proof** | Either constrain negotiation to 24/32 bpp and document, or implement RGB565 emission. |
| Interleaved bitmap RLE 24 bpp | Decodes full RLE order set | **partial/minimal encoder** | Current server encoder is 24 bpp COPY/color-run subset with expansion rejection. Need full order coverage or documented subset. |
| Interleaved bitmap RLE 8/15/16 bpp | Decodes full RLE order set for each depth | **partial/minimal encoder** | Conservative COPY/COLOR-order encoder now covers these depths, but production negotiation is still constrained to 24 bpp until lower-bpp raw tile emission/palette behavior is wired and documented. |
| Interleaved bitmap RLE 32 bpp-as-24 | Decodes 32 bpp compressed stream as 24 bpp RLE | **partial via 24 bpp fallback only** | Confirm whether server ever negotiates 32 bpp compressed slow-path; implement or document. |
| Classic RDP6 bitmap-update Planar (`NO_BITMAP_COMPRESSION_HDR`) | Decodes 32 bpp Planar bitmap updates | **partial/minimal encoder** | Classic bitmap-update Planar payload/update builder now exists separately from RDPGFX Planar, with expansion rejection and round-trip tests. Runtime negotiation/emission is still pending. |

## Bitmap Codecs / SurfaceBits encodings

| Encoding | go-rdp client support | go-rdp-android server status | Gap / next action |
| --- | --- | --- | --- |
| NSCodec | GUID + decoder + raw encoder helper | **production encoder, opt-in/capability-gated** | Existing SurfaceBits builder uses negotiated codec ID and metrics. Need matrix/client proof when clients advertise. |
| JPEG bitmap codec | GUID + capability parsing | **production encoder, opt-in/capability-gated** | Existing JPEG SurfaceBits builder with quality knob and metrics. Need client advertisement proof. |
| PNG bitmap codec | GUID appears in go-rdp client private parsing, not exported in `pkg/codec` | **diagnostic override** | Server has PNG SurfaceBits builder but only `GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID`. Need confirm/export real go-rdp PNG GUID support, then negotiated path. |
| RemoteFX / RFX | GUID + RFX decoder package | **production encoder, opt-in/capability-gated** | Existing single-tile encoder/message assembly and SurfaceBits wrapper. Need distinguish RemoteFX vs RemoteFX-Image advertisement and matrix proof where advertised. |
| RemoteFX-Image | GUID + RFX decoder package advertised by client helper | **production encoder, opt-in/capability-gated** | Existing RFX payload likely serves this path; confirm exact GUID semantics and selection priority. |
| Bitmap Codecs ClearCodec GUID | GUID appears in go-rdp client capability parsing/tests | **metadata-only pending proof** | No confirmed go-rdp Bitmap Codecs ClearCodec decode path. Do not conflate with RDPGFX ClearCodec. |
| Bitmap Codecs RemoteFX Progressive GUID | GUID appears in go-rdp client capability parsing | **metadata-only pending proof** | Confirm whether there is a decode path or only capability logging. |
| Bitmap Codecs H264 GUID | GUID appears in go-rdp client capability parsing | **metadata-only pending proof** | Confirm whether go-rdp decodes this path; keep separate from RDPGFX AVC420/444. |

## RDPGFX WireToSurface codecs

| Encoding | go-rdp client support | go-rdp-android server status | Gap / next action |
| --- | --- | --- | --- |
| Uncompressed (`0x0000`) | Codec ID/name known | **production/diagnostic encoder** | Encoder exists behind env gate. Need decide if metadata-only client support is enough or document diagnostic-only. |
| CAVideo (`0x0003`) | Codec ID/name known | **metadata-only pending proof** | Need confirm go-rdp decode support. If none, document metadata-only; if yes, implement. |
| ClearCodec (`0x0008`) | Codec ID/name known | **partial/minimal encoder** | Current encoder supports solid rect and RGB565 raw-rect splitting with expansion rejection. Need full useful subset based on client expectations/spec. |
| CAProgressive (`0x0009`) | Codec ID/name known | **fixture hook only / payload parser-builder only** | No production encoder. Implement real progressive region/layer/chunk generation. |
| Planar (`0x000A`) | Codec ID/name known | **production encoder** | Default compressed path. Keep evidence current. |
| AVC420 (`0x000B`) | Codec ID/name known | **partial experimental encoder** | AVC420 wrapper/forced path exists. Need negotiated production emission where client supports AVC420. |
| Alpha (`0x000C`) | Codec ID/name known | **metadata-only pending proof** | Need confirm decode support. Implement alpha-capable path or document metadata-only. |
| CAProgressiveV2 (`0x000D`) | Codec ID/name known | **missing production encoder** | Selection mentions V2 ID but emission uses CAProgressive encoder hook. Need V2-specific production encoder or document unsupported. |
| AVC444 (`0x000E`) | Codec ID/name known | **fixture hook + bounded payload builders only** | Need production encoder input path with auxiliary plane/region metadata. |
| AVC444v2 (`0x000F`) | Codec ID/name known | **fixture hook + bounded payload builders only** | Need production encoder input path with v2-specific auxiliary plane/region metadata. |

## Highest-priority implementation sequence

1. Resolve negotiation/depth constraints for classic bitmap output, then implement or explicitly document lower-bpp raw/RLE and classic bitmap-update Planar.
2. Confirm go-rdp metadata-only codecs by tracing actual decode paths for Bitmap Codecs PNG/H264/ClearCodec/Progressive and RDPGFX CAVideo/Alpha.
3. Implement production RDPGFX AVC444/AVC444v2 input/encoder path.
4. Implement production CAProgressive/CAProgressiveV2 encoder path.
5. Expand ClearCodec beyond the current minimal subset.
6. Add/extend matrix cases and CI gates only after the corresponding production write path exists.

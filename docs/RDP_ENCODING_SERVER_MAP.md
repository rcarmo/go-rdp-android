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
| Raw bitmap 8 bpp palette | Decodes palette-indexed 8 bpp using palette state | **partial/minimal encoder** | Server can now build grayscale 8 bpp bitmap rectangles, prepend a grayscale palette update, and emit when negotiated/forced via `GO_RDP_ANDROID_ENABLE_BITMAP_BPP=8`; local matrix evidence: `bitmap-8bpp` active with `bitmap_bpp8_seen=true` and `palette_seen=true`. |
| Raw bitmap 15 bpp RGB555 | Decodes RGB555 | **partial/minimal encoder** | Server can now build RGB555 bitmap rectangles and emit them when negotiated/forced via `GO_RDP_ANDROID_ENABLE_BITMAP_BPP=15`; local FreeRDP 3.5.1 rejects `/bpp:15`, so matrix case is opportunistic and requires `bitmap_bpp15_seen=true` only if active. |
| Raw bitmap 16 bpp RGB565 | Decodes RGB565 | **partial/minimal encoder** | Server can now build RGB565 bitmap rectangles and emit them when negotiated/forced via `GO_RDP_ANDROID_ENABLE_BITMAP_BPP=16`; local matrix evidence: `bitmap-16bpp` active with `bitmap_bpp16_seen=true`. |
| Interleaved bitmap RLE 24 bpp | Decodes full RLE order set | **partial/minimal encoder** | Current server encoder is 24 bpp COPY/color-run subset with expansion rejection. Need full order coverage or documented subset. |
| Interleaved bitmap RLE 8/15/16 bpp | Decodes full RLE order set for each depth | **partial/minimal encoder** | Conservative COPY/COLOR-order encoder now covers these depths, but production negotiation is still constrained to 24 bpp until lower-bpp raw tile emission/palette behavior is wired and documented. |
| Interleaved bitmap RLE 32 bpp-as-24 | Decodes 32 bpp compressed stream as 24 bpp RLE | **partial via 24 bpp fallback only** | Confirm whether server ever negotiates 32 bpp compressed slow-path; implement or document. |
| Classic RDP6 bitmap-update Planar (`NO_BITMAP_COMPRESSION_HDR`) | Decodes 32 bpp Planar bitmap updates | **partial/minimal encoder** | Classic bitmap-update Planar payload/update builder now exists separately from RDPGFX Planar, with expansion rejection, round-trip tests, and opt-in emission via `GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR=1`; local matrix evidence: `bitmap-planar` active with `bitmap_planar_seen=true` and positive saved-percent evidence. |

## Bitmap Codecs / SurfaceBits encodings

| Encoding | go-rdp client support | go-rdp-android server status | Gap / next action |
| --- | --- | --- | --- |
| NSCodec | GUID + decoder + raw encoder helper | **production encoder, opt-in/capability-gated** | Existing SurfaceBits builder uses negotiated codec ID and metrics. Need matrix/client proof when clients advertise. |
| JPEG bitmap codec | GUID + capability parsing | **production encoder, opt-in/capability-gated** | Existing JPEG SurfaceBits builder with quality knob and metrics. Need client advertisement proof. |
| PNG bitmap codec | GUID appears in go-rdp client capability parsing only, not exported in `pkg/codec` and no decode path found | **diagnostic override / metadata-only upstream** | Server has PNG SurfaceBits builder but only `GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID`. Need upstream GUID export only if go-rdp intends real PNG client support; otherwise this remains non-parity/operator-only. |
| RemoteFX / RFX | GUID + RFX decoder package | **production encoder, opt-in/capability-gated** | Existing single-tile encoder/message assembly and SurfaceBits wrapper. Need distinguish RemoteFX vs RemoteFX-Image advertisement and matrix proof where advertised. |
| RemoteFX-Image | GUID + RFX decoder package advertised by client helper | **production encoder, opt-in/capability-gated** | Existing RFX payload likely serves this path; confirm exact GUID semantics and selection priority. |
| Bitmap Codecs ClearCodec GUID | GUID appears in go-rdp client capability parsing/tests only | **metadata-only / no client decode path found** | Repository search found GUID naming/capability-summary tests, but no ClearCodec bitmap-codec decoder. Do not conflate with RDPGFX ClearCodec. |
| Bitmap Codecs RemoteFX Progressive GUID | GUID appears in go-rdp client capability parsing only | **metadata-only / no client decode path found** | Repository search found GUID naming only. Treat as capability metadata unless a decoder is added upstream. |
| Bitmap Codecs H264 GUID | GUID appears in go-rdp client capability parsing only | **metadata-only / no client decode path found** | Repository search found GUID naming only. Keep separate from RDPGFX AVC420/444. |

## RDPGFX WireToSurface codecs

| Encoding | go-rdp client support | go-rdp-android server status | Gap / next action |
| --- | --- | --- | --- |
| Uncompressed (`0x0000`) | Codec ID/name known | **production/diagnostic encoder** | Encoder exists behind env gate. Need decide if metadata-only client support is enough or document diagnostic-only. |
| CAVideo (`0x0003`) | Codec ID/name known only | **metadata-only / no client decode path found** | Repository search found only ID/name tests. No server encoder required for parity until go-rdp gains decode support. |
| ClearCodec (`0x0008`) | Codec ID/name known | **partial/minimal encoder** | Current encoder supports full-frame solid rects, solid row-band detection, RGB565 raw-rect splitting, and expansion rejection. Need broader spec/client-driven subset before calling complete. |
| CAProgressive (`0x0009`) | Codec ID/name known | **partial/minimal production encoder** | Default encoder now emits a bounded single-layer full-frame progressive payload with RGB565 data, region metadata, fallback/expansion rejection, plus fixture-hook support and production-vs-fixture matrix cases. Broader progressive layer/chunk fidelity remains pending. |
| Planar (`0x000A`) | Codec ID/name known | **production encoder** | Default compressed path. Keep evidence current. |
| AVC420 (`0x000B`) | Codec ID/name known | **partial experimental encoder** | AVC420 wrapper/forced path exists. Need negotiated production emission where client supports AVC420. |
| Alpha (`0x000C`) | Codec ID/name known only | **metadata-only / no client decode path found** | Repository search found only ID/name tests and unrelated alpha-plane support in NSCodec/classic Planar. No RDPGFX Alpha server encoder required for parity until go-rdp gains decode support. |
| CAProgressiveV2 (`0x000D`) | Codec ID/name known | **partial/minimal production encoder** | Default V2 encoder now emits a distinct bounded single-layer full-frame progressive payload and is selected for RDPGFX 10.4+ when progressive is enabled; matrix now has a separate `rdpgfx-progressivev2-encoded` case. Broader V2 layer semantics remain pending. |
| AVC444 (`0x000E`) | Codec ID/name known | **partial/minimal production encoder** | Default encoder now emits bounded base/aux access-unit payloads with full-frame region metadata, plus fixture-hook support and production-vs-fixture matrix cases. Real Android auxiliary-plane AVC generation/client proof remains pending. |
| AVC444v2 (`0x000F`) | Codec ID/name known | **partial/minimal production encoder** | Default v2 encoder now emits distinct bounded base/aux access-unit payloads with full-frame region metadata, plus fixture-hook support and production-vs-fixture matrix cases. Real Android auxiliary-plane AVC generation/client proof remains pending. |

## Highest-priority implementation sequence

1. Expand ClearCodec beyond the current minimal subset using spec/client-driven operations.
2. Replace deterministic placeholder AVC444/AVC444v2 base/aux access units with real Android auxiliary-plane AVC generation when MediaCodec plumbing is available.
3. Export/consume the go-rdp PNG Bitmap Codecs GUID only if upstream formalizes it as real client support; otherwise keep the Android operator override marked non-parity.
4. Add/extend CI gates only after the corresponding production write path is accepted by real clients and does not destabilize release defaults.

# RDP encoding inventory against go-rdp client support

This inventory is the source of truth for the server-encoder parity effort. It lists every graphics encoding/codec that `/workspace/projects/go-rdp` can currently identify, advertise, parse, or decode as a client, then records the expected `go-rdp-android` server-side obligation.

Do not mark the parity plan complete until each non-decode-only item has production server emission or an explicit partial/metadata-only finding, capability gating, tests, matrix evidence, and docs.

## Classic bitmap update encodings

| go-rdp client-side support | Class | Server obligation |
| --- | --- | --- |
| Uncompressed bitmap updates: 8 bpp palette, 15 bpp RGB555, 16 bpp RGB565, 24 bpp BGR, 32 bpp BGRA | Classic slow-path bitmap | Server must emit at least one safe release fallback. Additional lower-bpp palette/RGB555/RGB565 emission is only required if server negotiates those depths rather than normalizing to 24/32 bpp. |
| Interleaved bitmap RLE: 8 bpp, 15 bpp, 16 bpp, 24 bpp, and 32 bpp-as-24 bpp compressed stream | Classic slow-path bitmap compression | Server has an experimental 24 bpp COPY/color-run subset with expansion rejection and matrix saved-byte evidence; lower compressed depths remain out of default negotiation and should stay documented as unsupported unless implemented. |
| RDP6 Planar bitmap compression via `NO_BITMAP_COMPRESSION_HDR` for 32 bpp bitmap updates | Classic bitmap-update compression, distinct from RDPGFX Planar | Server has an experimental classic bitmap-update Planar emitter gated by `GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR=1`, with matrix raw/saved/saved-percent evidence. Keep it separate from default RDPGFX Planar. |
| Palette update + 8 bpp paletted bitmap decode | Classic bitmap support | Server emits a 256-entry grayscale palette when the experimental 8 bpp path is selected via `GO_RDP_ANDROID_ENABLE_BITMAP_BPP=8`; matrix evidence requires both 8 bpp tile traces and palette writes. |

## Bitmap Codecs Capability Set / SurfaceBits encodings

| go-rdp client-side support | Class | Server obligation |
| --- | --- | --- |
| NSCodec GUID and decode/encode helpers | Bitmap Codecs SurfaceBits | Production SurfaceBits encoder with negotiated codec ID, raw/saved/percent metrics, malformed-dimension rejection, fallback, matrix case. |
| RemoteFX GUID | Bitmap Codecs SurfaceBits | Production SurfaceBits RFX encoder or explicit evidence that only RemoteFX-Image is advertised by go-rdp client paths. |
| RemoteFX-Image GUID and RFX tile/message decoder | Bitmap Codecs SurfaceBits | Production SurfaceBits RemoteFX-Image encoder with tile/message assembly, negotiated codec ID, fallback, metrics, matrix evidence. |
| JPEG GUID | Bitmap Codecs SurfaceBits | Production JPEG SurfaceBits encoder with negotiated codec ID, quality bounds, fallback, metrics, matrix evidence. |
| ClearCodec GUID appears in go-rdp client capability parsing/tests | Bitmap Codecs Capability Set metadata, not the RDPGFX ClearCodec codec ID | Determine whether go-rdp has an actual Bitmap Codecs ClearCodec decode path. If yes, implement SurfaceBits ClearCodec. If metadata-only, document decode-only/client-metadata-only absence. |
| RemoteFX Progressive GUID appears in go-rdp client capability parsing | Bitmap Codecs metadata / progressive family | Determine whether it maps to RDPGFX CAProgressive or a Bitmap Codecs path in go-rdp. Implement if decode path exists; otherwise document metadata-only. |
| H264 GUID appears in go-rdp client capability parsing | Bitmap Codecs metadata / H.264 | Determine whether go-rdp can decode a Bitmap Codecs H.264 path. Implement if real client path exists; otherwise document metadata-only and keep RDPGFX AVC paths separate. |
| PNG GUID appears in go-rdp client private capability parsing | Bitmap Codecs metadata / PNG | Export GUID/name from go-rdp if it is real client support, then implement negotiated PNG SurfaceBits. If it is only parser metadata, keep operator override documented as non-parity. |

## RDPGFX WireToSurface codecs

| go-rdp client-side support | Class | Server obligation |
| --- | --- | --- |
| RDPGFX Uncompressed (`0x0000`) | RDPGFX WireToSurface | Production/diagnostic encoder exists but must have negotiated path, tests, metrics, matrix evidence. |
| RDPGFX CAVideo (`0x0003`) | RDPGFX WireToSurface metadata | Determine whether go-rdp decodes CAVideo or only names the codec. Implement or explicitly document metadata-only/decode-only absence. |
| RDPGFX ClearCodec (`0x0008`) | RDPGFX WireToSurface | Expand from minimal subset to full useful production subset supported by go-rdp client expectations, with fallback and matrix proof. |
| RDPGFX CAProgressive (`0x0009`) | RDPGFX WireToSurface | Partial/minimal opt-in production encoder exists for bounded single-layer full-frame RGB565 payloads, with parser/fuzz coverage, fallback, and production-vs-fixture matrix evidence; broader progressive semantics remain pending. |
| RDPGFX Planar (`0x000A`) | RDPGFX WireToSurface | Production encoder exists and remains default compressed path; keep tests/matrix/docs current. |
| RDPGFX AVC420 (`0x000B`) | RDPGFX WireToSurface | Existing AVC420 wrapper/forced path must become negotiated production emission where client support exists, or remain explicitly force-mode until proof exists. |
| RDPGFX Alpha (`0x000C`) | RDPGFX WireToSurface metadata | Determine whether go-rdp decodes Alpha. Implement alpha-capable server emission or document metadata-only absence. |
| RDPGFX CAProgressiveV2 (`0x000D`) | RDPGFX WireToSurface | Partial/minimal opt-in V2 encoder exists for bounded single-layer full-frame RGB565 payloads and has a separate `rdpgfx-progressivev2-encoded` matrix case; broader V2 semantics/client proof remain pending. |
| RDPGFX AVC444 (`0x000E`) | RDPGFX WireToSurface | Partial/minimal opt-in production encoder exists with bounded full-frame base/aux payloads and production-vs-fixture matrix evidence; real Android auxiliary-plane AVC input and client proof remain pending. |
| RDPGFX AVC444v2 (`0x000F`) | RDPGFX WireToSurface | Partial/minimal opt-in production encoder exists with distinct v2 bounded base/aux payloads and production-vs-fixture matrix evidence; real Android auxiliary-plane AVC input and client proof remain pending. |

## Immediate parity gaps to resolve

1. Classic bitmap RLE is currently only a 24 bpp COPY/color-run subset on the server; go-rdp client decodes 8/15/16/24 and 32-as-24.
2. Bitmap Codecs PNG/H264/ClearCodec/RemoteFX Progressive GUIDs are present in go-rdp client capability parsing but not all are exported or mapped to actual decode paths; this must remain documented before claiming negotiated parity.
3. RDPGFX ClearCodec, CAProgressive/CAProgressiveV2, and AVC444/AVC444v2 have partial/minimal opt-in production encoders, but still need broader spec/client proof before release-default status.
4. RDPGFX AVC444/AVC444v2 still need real Android auxiliary-plane AVC generation rather than deterministic bounded test access units.
5. RDPGFX CAVideo and Alpha are named by go-rdp; server parity requires implementation or an explicit metadata-only/decode-only finding.

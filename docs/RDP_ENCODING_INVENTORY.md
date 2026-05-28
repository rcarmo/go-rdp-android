# RDP encoding inventory against go-rdp client support

This inventory is the source of truth for the server-encoder parity effort. It lists every graphics encoding/codec that `/workspace/projects/go-rdp` can currently identify, advertise, parse, or decode as a client, then records the expected `go-rdp-android` server-side obligation.

Do not mark the parity plan complete until each non-decode-only item has production server emission, capability gating, tests, matrix evidence, and docs.

## Classic bitmap update encodings

| go-rdp client-side support | Class | Server obligation |
| --- | --- | --- |
| Uncompressed bitmap updates: 8 bpp palette, 15 bpp RGB555, 16 bpp RGB565, 24 bpp BGR, 32 bpp BGRA | Classic slow-path bitmap | Server must emit at least one safe release fallback. Additional lower-bpp palette/RGB555/RGB565 emission is only required if server negotiates those depths rather than normalizing to 24/32 bpp. |
| Interleaved bitmap RLE: 8 bpp, 15 bpp, 16 bpp, 24 bpp, and 32 bpp-as-24 bpp compressed stream | Classic slow-path bitmap compression | Server must implement compression for each negotiated bpp it can emit, or explicitly constrain negotiation/emission and document unsupported lower-bpp server output. Current known server path is 24 bpp COPY/color-run subset only. |
| RDP6 Planar bitmap compression via `NO_BITMAP_COMPRESSION_HDR` for 32 bpp bitmap updates | Classic bitmap-update compression, distinct from RDPGFX Planar | Server must either implement classic bitmap-update Planar emission or prove it never negotiates/emits the client-side classic Planar path. This is separate from existing RDPGFX Planar. |
| Palette update + 8 bpp paletted bitmap decode | Classic bitmap support | Server must emit palette updates if it ever emits 8 bpp bitmap updates. Otherwise document that server output is constrained to 24/32 bpp. |

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
| RDPGFX CAProgressive (`0x0009`) | RDPGFX WireToSurface | Implement production progressive encoder, not only payload parser/fixture hook. |
| RDPGFX Planar (`0x000A`) | RDPGFX WireToSurface | Production encoder exists and remains default compressed path; keep tests/matrix/docs current. |
| RDPGFX AVC420 (`0x000B`) | RDPGFX WireToSurface | Existing AVC420 wrapper/forced path must become negotiated production emission where client support exists, or remain explicitly force-mode until proof exists. |
| RDPGFX Alpha (`0x000C`) | RDPGFX WireToSurface metadata | Determine whether go-rdp decodes Alpha. Implement alpha-capable server emission or document metadata-only absence. |
| RDPGFX CAProgressiveV2 (`0x000D`) | RDPGFX WireToSurface | Implement production V2 progressive encoder or document unsupported decode-only/client metadata constraints. |
| RDPGFX AVC444 (`0x000E`) | RDPGFX WireToSurface | Implement production AVC444 encoder input path including auxiliary plane/region metadata, not only bounded payload builders. |
| RDPGFX AVC444v2 (`0x000F`) | RDPGFX WireToSurface | Implement production AVC444v2 encoder input path including auxiliary plane/region metadata, not only bounded payload builders. |

## Immediate parity gaps to resolve

1. Classic bitmap RLE is currently only a 24 bpp COPY/color-run subset on the server; go-rdp client decodes 8/15/16/24 and 32-as-24.
2. Classic RDP6 bitmap-update Planar is decoded by go-rdp but is distinct from the existing RDPGFX Planar server encoder.
3. Bitmap Codecs PNG/H264/ClearCodec/RemoteFX Progressive GUIDs are present in go-rdp client capability parsing but not all are exported or mapped to actual decode paths; this must be resolved before claiming parity.
4. RDPGFX AVC444/AVC444v2 and CAProgressive/CAProgressiveV2 are not production encoders yet.
5. RDPGFX CAVideo and Alpha are named by go-rdp; server parity requires implementation or an explicit metadata-only/decode-only finding.

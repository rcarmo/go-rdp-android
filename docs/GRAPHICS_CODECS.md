# Graphics codec coverage

This page separates the graphics paths the server implements today from RDP codec families that are explicitly missing/deferred. The first public APK baseline remains RDPGFX Planar with slow-path bitmap fallback; additional codecs should be added only when client capability evidence or performance data justifies the complexity. Shared RDP bitmap/RDPGFX codec identifiers now come from upstream `github.com/rcarmo/go-rdp/pkg/codec` so server-side encoder work can be pushed there before being wired into Android.

## Implemented and tested

| Path | Transport | Status | Evidence |
| --- | --- | --- | --- |
| Slow-path raw bitmap updates | Share Data bitmap updates | Implemented fallback | Blocking FreeRDP `/sec:rdp`, `/sec:tls`, `/sec:nla` gates; local `make encoding-matrix` bitmap case. |
| RDPGFX Planar | `drdynvc` + `Microsoft::Windows::RDS::Graphics` + Planar codec | Default compressed baseline | Blocking FreeRDP `/sec:nla /gfx` gate; local `make encoding-matrix` RDPGFX Planar case. The RDPGFX frame path has an opt-in streaming path (`GO_RDP_ANDROID_ENABLE_RDPGFX_STREAM=1`) for subsequent frame updates while CI/release gates keep the conservative initial-frame proof path until broader client soak evidence is collected; matrix artifacts record `rdpgfx_frame_stream_stop_count` when a client closes the graphics DVC after streamed writes. RDPGFX uncompressed is also available as an opt-in diagnostic path via `GO_RDP_ANDROID_ENABLE_RDPGFX_UNCOMPRESSED=1`; Planar remains the release default. |
| RDPGFX AVC420 / H.264 | `drdynvc` + RDPGFX AVC420 `RFX_AVC420_BITMAP_STREAM` | Experimental / force-mode smoke only | Android `MediaCodec` scaffold, gomobile encoded-frame queue, server AVC420 emission, `h264Status`, CI forced artifact, local FreeRDP 3.15.0 forced `/gfx:AVC420` smoke, plus `h264-negotiated-gfx` matrix evidence that unforced H.264 stays gated with no writes when the client advertises `AVC_DISABLED`/no usable AVC420 support. |

## Missing/deferred codec families

| Codec family | Implemented? | First-APK blocker? | Notes |
| --- | --- | --- | --- |
| RDP 5/6 bitmap compression / bitmap RLE | Experimental opt-in | No | A conservative 24-bpp COPY/color-order encoder and compressed bitmap-update builder exist with expansion rejection; runtime emission is guarded by `GO_RDP_ANDROID_ENABLE_BITMAP_RLE=1`, the local matrix has a dedicated `bitmap-rle` case and records `bitmap_rle_seen`, compressed bytes, and saved bytes; gomobile/Android diagnostics expose `bitmapRleFrames`, `bitmapRleBytes`, and `bitmapRleSavedBytes` with unit coverage for malformed stats input; and the path should remain off by default while RDPGFX Planar is green. |
| NSCodec | Raw-plane encoder/decoder/capability metadata upstream; Android capability-gated SurfaceBits builder scaffold | No | `go-rdp` now has public NSCodec encode/decode utilities and GUID metadata; Android parses advertised NSCodec IDs, has an opt-in `GO_RDP_ANDROID_ENABLE_NSCODEC=1` selection gate plus shared SurfaceBits stream gate `GO_RDP_ANDROID_ENABLE_BITMAP_CODEC_STREAM=1`, has a tested `SetSurfaceBits` builder for NSCodec payloads, emits an initial opt-in NSCodec update with `nscodec_selected`/`nscodec_write` traces including raw/saved-byte fields, exposes `graphics=nscodec`, `nsCodecFrames`, `nsCodecBytes`, `nsCodecRawBytes`, `nsCodecSavedBytes`, and `nsCodecSavedPercent` diagnostics, and the FreeRDP summarizer/matrix now records NSCodec selected/write/raw/saved/percent fields when experiments trigger it; default behavior remains unchanged when the gate is off. |
| RemoteFX / RFX | Upstream decoder/capability metadata plus Android production single-tile encoder | No | `go-rdp` has RFX package coverage and public GUID metadata; Android now has a production single-tile RFX encoder path (`RGB→YCoCg→DWT→quant→RLGR→tile/message assembly`) gated by `GO_RDP_ANDROID_ENABLE_RFX_CODEC=1` plus Bitmap Codecs advertisement checks. Local matrix keeps separate `rfx-encoded` and `rfx-fixture` cases; CI evidence from run `26543284313` (`freerdp-compat-probe` + `encoding-matrix-artifacts`) still shows FreeRDP Confirm Active `bitmap_codecs=0`, so modern FreeRDP in this profile is not advertising RemoteFX and selection/write evidence remains capability-gated. |
| RDPGFX AVC444 / AVC444v2 | Codec IDs upstream; Android deferred probes plus encoder-hooked WireToSurface seams | No | Shared IDs exist; Android now has `GO_RDP_ANDROID_ENABLE_AVC444=1` and `GO_RDP_ANDROID_ENABLE_AVC444V2=1` selection traces plus generic RDPGFX encoded-frame hooks that can emit `WireToSurface_1(codecID=AVC444/AVC444v2)` when future encoders are supplied. Current AVC path only builds the AVC420 wrapper (`numRegionRects=1`, one rect, `qpVal`, `qualityVal`, single access unit). `MediaCodec` capture currently supplies only the base H.264 access unit stream; it does not yet supply the required AVC444 auxiliary-plane access units/metadata, so production AVC444/AVC444v2 transport remains deferred behind explicit aux-plane support. |
| RDPGFX ClearCodec | Codec ID upstream; Android deferred probe plus encoder-hooked WireToSurface seam | No | Shared ID exists and `GO_RDP_ANDROID_ENABLE_CLEARCODEC=1` can trace plausible negotiated support. The server now has a bounded generic RDPGFX encoded-frame wrapper and ClearCodec encoder hook that can emit `WireToSurface_1(codecID=ClearCodec)` when a future encoder is supplied, but no production encoder is implemented. |
| RDPGFX Progressive / other progressive codecs | Codec IDs upstream; Android deferred probe plus encoder-hooked WireToSurface seam | No | Shared IDs exist and `GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC=1` can trace plausible negotiated support. The server now has a generic RDPGFX encoded-frame wrapper and Progressive encoder hook that can emit `WireToSurface_1(codecID=CAProgressive)` when a future encoder is supplied, but no production encoder is implemented. |
| JPEG/PNG bitmap codecs | JPEG GUID upstream; Android JPEG/PNG SurfaceBits builders | No | `go-rdp` exposes JPEG bitmap-codec GUID metadata. Android now has an opt-in `GO_RDP_ANDROID_ENABLE_JPEG_CODEC=1` selection gate, tunable `GO_RDP_ANDROID_JPEG_QUALITY=<1..100>` quality, tested JPEG `SetSurfaceBits` builder, opt-in initial emission path, Android diagnostics (`graphics=jpeg-codec`, `jpegCodecFrames`, `jpegCodecBytes`, `jpegCodecRawBytes`, `jpegCodecSavedBytes`, `jpegCodecSavedPercent`), and JSON/Markdown matrix/summarizer selected/write/raw/saved/percent fields; real client proof remains pending client Bitmap Codecs evidence. PNG now has a tested `SetSurfaceBits` payload builder plus an operator-only `GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID=<1..255>` emission override, tunable `GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL=<0|-1|-2|-3>`, and Android diagnostics (`graphics=png-codec`, `pngCodecFrames`, `pngCodecBytes`, `pngCodecRawBytes`, `pngCodecSavedBytes`, `pngCodecSavedPercent`) for client-specific codec-ID experiments, but no current negotiated automatic RDP output path. |

## Client capability evidence to collect

When testing FreeRDP, Remmina, or Microsoft Remote Desktop, preserve enough logs to answer these questions before adding another encoder:

- Does the client advertise RDPGFX and which RDPGFX version/flags are present?
- Does it advertise AVC420 without `AVC_DISABLED`?
- Does it advertise AVC444/AVC444v2, ClearCodec, progressive codecs, RemoteFX/RFX, NSCodec, or classic bitmap compression preferences?
- Does the client fail or degrade on RDPGFX Planar but remain capable of another codec family?
- Does a constrained-network measurement show a meaningful bandwidth/latency/CPU win over RDPGFX Planar?

For FreeRDP, keep `xfreerdp` TRACE logs plus `mock-server.log` and `summary.json` from `make encoding-matrix` or the CI probe. For Remmina, keep the Remmina debug log, FreeRDP library version, profile settings, screenshot, and the server trace log for the same session. For Microsoft Remote Desktop, keep client version/platform, screenshots, Android diagnostics text, and server trace summaries.

## Decision rule for adding a codec

Before implementing a missing codec, collect at least one of:

- a real FreeRDP/Remmina/Microsoft Remote Desktop capability trace showing the codec is advertised and preferred,
- a compatibility gap where bitmap fallback or RDPGFX Planar fails but the codec is expected to work,
- a performance/bandwidth measurement showing material benefit on a target device/client,
- a release requirement that cannot be satisfied with RDPGFX Planar or H.264/AVC.

Any new codec path must include:

- strict negotiated enablement and safe fallback,
- bounded payload parsing/building,
- encoder/payload unit tests,
- FreeRDP/Remmina/manual-client evidence where feasible,
- diagnostics in server traces, Android health, and summaries if selectable at runtime,
- updates to `docs/STATUS.md`, `docs/TESTING.md`, `docs/PERFORMANCE.md`, and this page.

## Local matrix

Run:

```bash
make encoding-matrix
```

The matrix currently exercises implemented paths, records observed RDPGFX capability advertisements from server traces, annotates the AVC-related flag bits (`0x10` = `AVC420_ENABLED`, `0x20` = `AVC_DISABLED`), reports SurfaceBits codec write/raw/saved/percent fields when NSCodec/JPEG/PNG/RemoteFX experiments emit payloads, runs separate RemoteFX production (`rfx-encoded`) and fixture (`rfx-fixture`) cases plus fixture-payload transport-hook cases for ClearCodec, Progressive, AVC444, and AVC444v2, writes a machine-readable `codec-coverage.json`, and then lists missing production encoder families so reports are explicit about coverage boundaries. CI now runs this matrix in the `encoding-matrix` job and uploads `encoding-matrix-artifacts` (latest passing evidence: run `26543284313`, job `78189527222`).

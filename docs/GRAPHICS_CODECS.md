# Graphics codec coverage

This page separates the graphics paths the server implements today from RDP codec families that are explicitly missing/deferred. The first public APK baseline remains RDPGFX Planar with slow-path bitmap fallback; additional codecs should be added only when client capability evidence or performance data justifies the complexity.

## Implemented and tested

| Path | Transport | Status | Evidence |
| --- | --- | --- | --- |
| Slow-path raw bitmap updates | Share Data bitmap updates | Implemented fallback | Blocking FreeRDP `/sec:rdp`, `/sec:tls`, `/sec:nla` gates; local `make encoding-matrix` bitmap case. |
| RDPGFX Planar | `drdynvc` + `Microsoft::Windows::RDS::Graphics` + Planar codec | Default compressed baseline | Blocking FreeRDP `/sec:nla /gfx` gate; local `make encoding-matrix` RDPGFX Planar case. |
| RDPGFX AVC420 / H.264 | `drdynvc` + RDPGFX AVC420 `RFX_AVC420_BITMAP_STREAM` | Experimental / force-mode smoke only | Android `MediaCodec` scaffold, gomobile encoded-frame queue, server AVC420 emission, `h264Status`, CI forced artifact, local FreeRDP 3.15.0 forced `/gfx:AVC420` smoke. |

## Missing/deferred codec families

| Codec family | Implemented? | First-APK blocker? | Notes |
| --- | --- | --- | --- |
| RDP 5/6 bitmap compression / bitmap RLE | Experimental opt-in | No | A conservative 24-bpp COPY/color-order encoder and compressed bitmap-update builder exist with expansion rejection; runtime emission is guarded by `GO_RDP_ANDROID_ENABLE_BITMAP_RLE=1`, the local matrix has a dedicated `bitmap-rle` case and records `bitmap_rle_seen`, compressed bytes, and saved bytes; gomobile/Android diagnostics expose `bitmapRleFrames`, `bitmapRleBytes`, and `bitmapRleSavedBytes`; and the path should remain off by default while RDPGFX Planar is green. |
| NSCodec | No | No | Could help some legacy/non-GFX clients; requires capability parsing and encoder implementation. |
| RemoteFX / RFX | No | No | Deprecated/disabled in many clients; only worth implementing if real-client evidence requires it. |
| RDPGFX AVC444 / AVC444v2 | No | No | Higher-fidelity H.264 family; defer until AVC420 negotiated-client proof exists. |
| RDPGFX ClearCodec | No | No | Text/graphics optimized codec; defer behind Planar and AVC420. |
| RDPGFX Progressive / other progressive codecs | No | No | More complex progressive pipeline; not first-APK scope. |
| JPEG/PNG bitmap codecs | No | No | No current server output path; add only if capability/performance data justifies it. |

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

The matrix currently exercises implemented paths, records observed RDPGFX capability advertisements from server traces, annotates the AVC-related flag bits (`0x10` = `AVC420_ENABLED`, `0x20` = `AVC_DISABLED`), writes a machine-readable `codec-coverage.json`, and then lists the missing families above so reports are explicit about coverage boundaries.

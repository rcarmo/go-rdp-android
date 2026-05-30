# Performance measurement

The CI emulator workflow can collect repeatable RDP performance baselines from the Go-backed Android app.

Run it manually with:

```sh
gh workflow run CI \
  --ref main \
  -f emulator_api_level=35 \
  -f emulator_go_backed=true \
  -f emulator_capture=true \
  -f emulator_capture_scale=1
```

The `Android emulator smoke` job then:

1. Starts the APK with MediaProjection capture.
2. Accepts the emulator capture consent dialog.
3. Starts the gomobile-backed RDP server on TCP/3390.
4. Uses `adb forward tcp:3390 tcp:3390` from the runner.
5. Navigates through home, Settings, and browser intents.
6. Runs `cmd/probe` for each scene and stores paired Android/RDP screenshots plus JSON metrics. The single-session scene plan also exercises emulator keyboard text entry in Settings search, a mouse-source tap target, and a touchscreen swipe to open notifications.

## Artifacts

Expected artifact files include:

- `rdp-home.png`, `rdp-settings.png`, `rdp-browser.png` — RDP-rendered screenshots.
- `android-home.png`, `android-settings.png`, `android-browser.png` — emulator screenshots for comparison.
- `rdp-*-summary.json` — per-scene RDP performance counters.
- `performance-summary.md` — markdown roll-up of the JSON summaries.
- `rdp-capture-plan.txt` — selected tile/update count.

## Local bitmap encoder benchmarks

The slow-path 24-bit BGR tile encoder has a Go benchmark for repeatable local comparisons across representative desktop sizes:

```sh
GOTMPDIR="$PWD/.gotmp" go test ./internal/rdpserver -run '^$' -bench BenchmarkBuildFrameBitmapUpdates24BGR -benchmem
```

It currently covers 320x240, 1280x720, and 1920x1080 RGBA input frames, reports allocations, and is intended for before/after comparisons when changing tile size, dirty-region logic, compression, or scaling. It is not a CI gate because shared runners are too noisy for stable absolute performance thresholds.

Local baseline on the current workspace host (`12th Gen Intel(R) Core(TM) i7-12700`, `benchtime=3x`, commit `36d8d6e`):

| Input size | Time/op | Throughput | Allocated/op | Allocs/op |
| ---: | ---: | ---: | ---: | ---: |
| 320x240 | 0.858 ms | 358 MB/s | 0.49 MB | 73 |
| 1280x720 | 10.02 ms | 368 MB/s | 5.91 MB | 1053 |
| 1920x1080 | 25.11 ms | 330 MB/s | 13.28 MB | 2537 |

## Graphics codec encoder microbenchmarks

Codec-builder benchmarks are available in `internal/rdpserver`:

```sh
GOTMPDIR="$PWD/.gotmp" go test ./internal/rdpserver -run '^$' -bench 'BenchmarkBuild(RDPGFX|RFXProductionEncoder|ClearCodecEncoder|NSCodec|JPEG|PNG)' -benchmem
```

Latest local 320x240 smoke on 2026-05-30 after the Planar scratch-buffer, RDPGFX PDU-header, uncompressed direct-write, single-plane reuse, in-place Planar delta, direct Planar WireToSurface, Planar scratch-in-backing-buffer, fully coalesced Planar frame backing, shared RDPGFX frame-boundary PDU backing buffers, fully coalesced uncompressed RDPGFX frame backing, direct AVC420/H.264 and AVC444 frame backing, RFX bit-writer, RFX bit-buffer preallocation, RFX direct tileset assembly, RFX shared RLGR output buffering, ClearCodec payload pre-sizing, RGBA image-aliasing, direct JPEG/PNG SurfaceBits header, JPEG/PNG output pre-sizing, direct JPEG byte writing, PNG encoder-buffer pooling, and direct NSCodec SurfaceBits passes showed these relative costs on the workspace host: RDPGFX Planar now delta-encodes each reused color-plane buffer in place, no longer allocates a scratch row per scanline while encoding each color plane, uses the WireToSurface backing allocation for its reusable color-plane scratch, and writes the StartFrame, WireToSurface, and EndFrame PDUs out of one backing allocation instead of allocating separate boundary buffers. Common RDPGFX frame wrappers now build final PDUs directly and share a single backing allocation for StartFrame/EndFrame, the uncompressed diagnostic encoder writes pixels directly into one backing allocation that also contains its StartFrame/EndFrame PDUs, the AVC420/H.264 and AVC444 builders write AVC metadata and access units directly into one frame backing allocation instead of building/copying intermediate bitmap streams and H.264/experimental bitmap-codec byte metrics are recorded without allocating temporary payload grouping slices, the RemoteFX RLGR bit writer returns/preallocates its backing buffer instead of growing and copying multiple temporary slices, RemoteFX message assembly appends tile blocks directly into the final message buffer, RemoteFX reuses one backing buffer for all three RLGR component outputs, ClearCodec pre-sizes tiled raw-rect payloads to avoid growth reallocations, RGBA-backed image encoders alias validated frame memory instead of allocating a full intermediate `image.RGBA` copy, JPEG/PNG SurfaceBits encoders reserve the SurfaceBits header before compression so JPEG avoids a final encoded-payload copy, JPEG and PNG pre-size output buffers to avoid repeated buffer growth, JPEG writes directly to an in-place byte writer instead of allocating a `bufio.Writer`, PNG reuses the standard-library encoder workspace through a pool, and NSCodec writes AYCoCg planes directly into the final SurfaceBits command buffer instead of allocating three plane slices plus a separate command copy. This reduced the Planar hot-path allocation count from roughly 743 to 2 allocations/op for the synthetic gradient frame, lowered uncompressed RDPGFX from 7 to 2 allocations/op, lowered RemoteFX production from 24 to 2 allocations/op, lowered ClearCodec production from 2 to 1 allocation/op, lowered NSCodec encoded SurfaceBits from about 0.72 MB/5 allocs to about 0.24 MB/1 alloc, lowered JPEG encoded SurfaceBits from about 0.36 MB/14 allocs to about 0.021 MB/8 allocs, and lowered PNG encoded SurfaceBits from about 1.17 MB/35 allocs to about 0.011 MB/4 allocs. NSCodec raw-plane remains comparatively fast, JPEG trades CPU for smaller payloads on some inputs, and PNG is still diagnostic/operator-only. Sample smoke output:

| Benchmark | Time/op | Allocated/op | Allocs/op |
| --- | ---: | ---: | ---: |
| RDPGFX Planar 320x240 | 1.32-1.78 ms | 0.33 MB | 2 |
| RemoteFX production 320x240 | 0.19-0.26 ms | 0.007 MB | 2 |
| ClearCodec production 320x240 | 0.27 ms | 0.19 MB | 1 |
| RDPGFX Uncompressed 320x240 | 0.39-0.45 ms | 0.31 MB | 2 |
| RDPGFX H.264/AVC420 frame 320x240 + 4 KiB AU | 0.0004-0.0009 ms | 0.005 MB | 2 |
| RDPGFX AVC444 frame 320x240 + 6 KiB AU | 0.0006-0.0014 ms | 0.007 MB | 2 |
| NSCodec SurfaceBits 320x240 | 0.40 ms | 0.24 MB | 1 |
| JPEG SurfaceBits 320x240 | 1.48-1.97 ms | 0.021 MB | 8 |
| PNG SurfaceBits 320x240 | 2.65-3.74 ms | 0.011 MB | 4 |

`TestRDPGFXPlanarBuilderAllocationSmoke` keeps the Planar allocation reduction from regressing above 20 allocations/op for a 320x240 frame. `TestGraphicsCodecBuilderSizeSmoke` also keeps a simple solid-frame regression check that Planar, NSCodec, JPEG, and PNG builders produce payloads smaller than the raw 32-bpp source while uncompressed RDPGFX records expected protocol overhead. `TestJPEGQualityAffectsPayloadSize` verifies the JPEG quality knob changes payload size while remaining below raw 32-bpp size on a synthetic frame. `TestPNGCompressionLevelAffectsPayloadSize` verifies the PNG compression-level knob reduces payload size versus uncompressed PNG on a solid synthetic frame. Treat these as local encoder-cost/size smoke numbers only; release decisions still require target Android device FPS/CPU/battery/bandwidth measurements and real client compatibility evidence.

## Compressed graphics path

The first public APK includes a negotiated compressed graphics path rather than relying only on raw slow-path bitmap updates. RDPGFX (`Microsoft::Windows::RDS::Graphics`) over `drdynvc` is enabled by default and currently uses the Planar codec with no-alpha RLE planes. The existing slow-path 24-bit BGR bitmap transport remains as a fallback, compatibility gate, and benchmark oracle.

Current evidence and remaining work:

- CI run `26186872292` keeps `/sec:rdp`, `/sec:tls`, and `/sec:nla` bitmap fallback gates passing and proves `/sec:nla /gfx` reaches active RDPGFX streaming (`rdpgfx_seen=true`, screenshot present, `exit_code=131`).
- Go unit coverage includes bitmap fallback conversion/tiling safety, RDPGFX Planar round-trip encoding for repeated spans/signed deltas/wrap cases, and bitmap RLE stats parsing for valid and malformed compressed bitmap updates.
- Android health/diagnostics expose `graphics=rdpgfx-planar`, `graphics=bitmap-rle`, `graphics=nscodec`, `graphics=jpeg-codec`, `graphics=png-codec`, `graphics=rfx-codec`, or `graphics=bitmap-fallback` plus RDPGFX frame/byte counters, opt-in bitmap RLE frame/byte/saved-byte counters, and opt-in SurfaceBits codec write/raw/saved/percent counters for NSCodec/JPEG/PNG/RemoteFX experiments.
- Bitmap RLE is available only as an experimental fallback toggle (`GO_RDP_ANDROID_ENABLE_BITMAP_RLE=1`). The latest local matrix solid-fallback case recorded `bitmap_rle_seen=true`, `bitmap_rle_bytes=342`, and `bitmap_rle_saved_bytes=11968`; it is useful fallback-compression evidence but remains off by default because RDPGFX Planar is the release compressed path.
- The experimental H.264/AVC path now has Android `MediaCodec` encoder-surface scaffolding, a bounded gomobile encoded-frame queue, RDPGFX AVC420 `RFX_AVC420_BITMAP_STREAM` emission/streaming, encoded access-unit byte counters, and a non-blocking CI `h264-gfx` artifact. CI run `26084530884` forced server-side AVC420 emission with coalesced SPS/PPS config preserved across latest-frame coalescing and prepended to IDR (`h264_ready=true`, `h264_version=0x000a0600`, `h264_flags=0x00000020`, `h264_write_seen=true`, `h264_write_count=1`, `h264_write_bytes=23`, `h264_reason=forced-by-env`) but did not prove client support because the runner FreeRDP package rejects `/gfx:AVC420` and advertises AVC disabled in fallback `/gfx` mode. A local FreeRDP 3.15.0 encoding matrix on 2026-05-20 reached active state for bitmap fallback, opt-in bitmap RLE, RDPGFX Planar, forced `/gfx:AVC420`, and forced `/gfx` fallback; the current asserted matrix emitted 30 writes / 690 bytes for forced `/gfx:AVC420` and 30 writes / 690 bytes for forced `/gfx`, which is useful protocol smoke evidence but still not negotiated-client proof.
- Physical-device measurement still needs to compare bandwidth, latency feel, CPU/battery, and memory stability for H.264/AVC, RDPGFX Planar, and bitmap fallback before final release performance claims.

Current raw-bitmap baseline limitations that RDPGFX/compression must address:

- Full-screen changes are bandwidth-heavy: a 1080x2400 full frame is roughly 7.8 MB after the 24-bit BGR reduction, before transport overhead.
- Full-frame changes are split into hundreds of bitmap rectangles at the current 80x80 tile size, which favors compatibility and debuggability over latency.
- Dirty-tile suppression, optional downscale, Android capture pacing, bounded queue drops, and server-side queued-frame coalescing reduce stale work but do not change the raw-bitmap ceiling.

## Probe metrics

`cmd/probe` records:

- packet counts and bytes read/written;
- bitmap update count;
- bitmap payload bytes, rectangles, and pixels composed;
- total duration, handshake time, bitmap-read time, and first-bitmap latency;
- aggregate read and bitmap throughput in Mbps;
- average update size and update read time.

The current slow-path bitmap renderer sends uncompressed 24-bit BGR tiles with 4-byte row alignment. This is still classic bitmap update transport, but it avoids sending an unused alpha byte and cuts payload by roughly 25% versus the initial 32-bit BGRA baseline. Android frame ingress and the Go bitmap tiling/scaling paths validate dimensions, row stride, minimum backing data length, and integer-overflow cases before queueing, reading, or allocating frame buffers; Android-style unpadded final rows are accepted, while malformed frame metadata is dropped rather than silently producing partial tiles or oversized allocations. For a 1080x2400 emulator screen and an 80x80 tile size, one full frame is:

```text
ceil(1080/80) * ceil(2400/80) = 14 * 30 = 420 bitmap updates
```

This is intentionally simple and measurable. The bitmap-update builder now pre-sizes classic bitmap update payloads, the hot Share Data write path now emits TPKT/X.224/MCS/Share Data in one final allocation, the DRDYNVC static-channel write path now emits TPKT/X.224/MCS/static-channel wrapping in one final allocation, the RDPGFX-over-DRDYNVC write path now writes the DVC header, RDPGFX `0xe0 0x00` transport prefix, and payload directly into coalesced static-channel output buffers without constructing an intermediate wrapped payload or allocating per-fragment output slices, the common RDPEI SC_READY, license-valid-client, fixed server capability leaves, default MCS domain parameters, GCC server user-data blocks, MCS Connect Response envelope/GCC Conference Create Response, H.264 Annex-B access units, solid bitmap-RLE/ClearCodec rectangles, pseudo-AVC NAL placeholders, progressive payload parsing, slow-path input dispatch, and Demand Active response reuse constant/direct/pre-sized writers and avoid avoidable intermediate PDU buffers/copies, the standalone Share Data, MCS Send Data, MCS Domain, DRDYNVC Data/DataFirst, attach-user-confirm, and channel-join-confirm wrappers write directly into final one-allocation buffers, common DRDYNVC Caps/Create-response/RDPGFX-create-request control PDUs and constant Synchronize/Control/FontMap payloads reuse package templates, the default non-RLE fallback path writes each tile directly into its final bitmap-update payload while hashing the same bytes for dirty-tile suppression, disabled trace call sites avoid per-tile/per-PDU/RDPGFX-frame variadic argument allocations, and full-frame fallback builds all tile updates into one backing buffer for both no-cache and initial-cache cases. The DRDYNVC control helper benchmark now records 0 B/op and 0 allocs/op for common Caps, small-channel CreateResponse, and RDPGFX CreateRequest builders, the DRDYNVC static-payload write benchmark records one final write-buffer allocation instead of separate static-channel, MCS, and domain buffers, small and fragmented RDPGFX DRDYNVC writes now record one allocation for the final coalesced output buffer, the RDPEI SC_READY and license-valid-client response paths record one tiny final write-buffer allocation each, fixed server capability leaf builders and default MCS domain-parameter serialization record 0 B/op and 0 allocs/op, GCC server user data now records one final output allocation, the MCS Connect Response write path writes GCC server user data, GCC Conference Create, and the BER application envelope directly into the final TPKT/X.224 output buffer, Annex-B H.264 keyframe preparation aliases the submitted access unit instead of copying it and records 0 B/op while length-prefixed H.264 conversion pre-sizes its exact Annex-B output, solid bitmap-RLE rectangles now use an exact small color-order buffer instead of reserving full visible-row payload capacity, solid ClearCodec payloads and ClearCodec raw-rect headers now write directly into pre-sized output buffers, pseudo-AVC placeholder NALs now write directly into their exact 10-byte output buffer, progressive payload parsing aliases the encoded data slice instead of copying it, slow-path input dispatch parses and dispatches events without allocating an intermediate event slice, and Demand Active construction/write paths record one final output allocation. The 320x240 no-cache fallback benchmark dropped from roughly 73 to 2 allocations/op and from roughly 0.49 MB/op to 0.24 MB/op while preserving the same tile/update shape. The new initial-cache benchmark, which matches the first slow-path desktop frame before dirty-tile suppression starts, records roughly 0.24 MB/op and 5 allocs/op while still populating tile hashes for later incremental frames. Larger no-cache full-frame fallbacks also avoid the per-update backing allocation, reducing 1280x720 from roughly 5.91 MB/621 allocs to 2.77 MB/2 allocs and 1920x1080 from roughly 13.28 MB/1529 allocs to 6.24 MB/2 allocs. The dirty-cache incremental path still emits individual update buffers so unchanged-tile suppression can skip safely. The primary optimization targets are unchanged-tile suppression, adaptive frame pacing, optional downscaling, compressed bitmap/RDPGFX-style updates, and H.264/AVC video transport.

## FreeRDP soak snapshot: 2026-05-21 scheduled run

Scheduled workflow run `26205491210` exercised 30 `/sec:nla` FreeRDP sessions against the mock server with an 8-second per-iteration hold and 45-second timeout guard. The run passed with RSS ranging from 15,256 KB to 20,212 KB, a 4,956 KB delta against the 51,200 KB threshold. Each iteration remained bounded by the controlled timeout/escalation path rather than a stuck-client hang.

This is not a substitute for physical Android device stability testing, but it is the current close-to-release soak evidence for server-side session cleanup and gross RSS growth.

## Downscale baseline: 2026-05-02 emulator run

Manual workflow run `25247259184` used `emulator_capture_scale=2`, reducing MediaProjection/RDP capture from 1080x2400 to 540x1200.

| Scale | RDP size | Full-frame tiles | Full-frame payload |
| ---: | ---: | ---: | ---: |
| 1 | 1080x2400 | 420 | 7,776,000 bytes |
| 2 | 540x1200 | 105 | 1,944,000 bytes |

That is a **75% reduction** in uncompressed full-frame payload and tile count for the same device screen at the same bits-per-pixel. The 24-bit BGR encoder adds a further **25% payload reduction** over the original 32-bit baseline. The run passed with `startServer=ok`, `frame1=ok`, `screen_capture=ok`, and `fatal_exception=none`.

## Single-session baseline: 2026-05-02 emulator run

Manual workflow run `25246034129` kept one RDP connection open while capturing home, navigating to Settings, and navigating to browser. The home capture used one 420-update warmup full frame; Settings and browser were captured as incremental scenes on the same session.

| Phase | Updates | Payload | Notes |
| --- | ---: | ---: | --- |
| Home warmup | 420 | ~10.37 MB | Initial full-frame canvas. |
| Settings scene | 420 | 10.38 MB | Effectively a full-screen change from launcher to Settings. |
| Browser scene | 378 | 9.33 MB | Dirty-tile suppression skipped 42 unchanged tiles vs. a 420-tile full frame. |

The run produced `rdp-home.png`, `rdp-settings.png`, `rdp-browser.png`, and a single `rdp-probe-summary.json` with per-scene counters.

## Baseline: 2026-05-02 emulator run

Manual workflow run `25245076441` captured home, Settings, and browser scenes via MediaProjection and RDP. Each scene used the exact 420-update full-frame count instead of the earlier 450-update over-capture.

| Scene | Updates | Payload | Total duration | Bitmap read | First bitmap | Bitmap throughput |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Home | 420 | 10,368,000 bytes | 1968 ms | 908 ms | 877 ms | 91.35 Mbps |
| Settings | 420 | 10,368,000 bytes | 673 ms | 456 ms | 343 ms | 181.89 Mbps |
| Browser | 420 | 10,368,000 bytes | 467 ms | 321 ms | 173 ms | 258.39 Mbps |

Immediate findings:

- The original 32-bit uncompressed full-resolution frame cost about **10.4 MB** on the wire; the 24-bit BGR encoder reduces that to about **7.8 MB** before dirty-tile suppression or downscaling.
- A full frame is **420 separate slow-path bitmap PDUs** at the current 80x80 tile size.
- The first scene after capture startup is slower; later scenes are dominated by bitmap transfer/composition.
- Exact full-frame update counting removes 30 unnecessary update reads compared with the earlier 450-update probe default.

Performance workstreams:

1. **Dirty-tile suppression** after the initial frame to avoid resending unchanged tiles during idle periods. Status: implemented for post-initial stream frames using per-tile hashes; unit coverage verifies unchanged frames emit no updates and one-pixel changes emit one tile.
2. **Adaptive probe/session mode** to keep one RDP connection open while driving navigation, measuring incremental scene changes rather than reconnecting for every screenshot. Status: implemented and validated as the default Go-backed MediaProjection CI path via `cmd/probe -scene-plan`.
3. **Capture pacing/backpressure** so MediaProjection does not copy frames faster than the RDP encoder can drain them. Status: implemented in layers: Android adapts capture interval based on bridge submission time, the bounded Go `FrameQueue` drops oldest frames when full, and the RDP stream loop coalesces queued backlog to the latest frame before encoding. Unit tests cover queued-frame coalescing, and emulator validation from CI run `25246333819` stayed green with browser scene dirty updates reduced to 302. This satisfies the current prototype frame-pacing/backpressure plan item; real-device/network backpressure validation remains pending under production acceptance.
4. **Optional downscale mode** for low-bandwidth viewing. Status: implemented and emulator-validated as `emulator_capture_scale` / `capture_scale` plumbing through CI, Android intent extras, and MediaProjection virtual display sizing; scale=2 run `25247259184` captured 540x1200 frames with 105 full-frame tiles and 2.59 MB/frame payload.
5. **Compression/RDPGFX** once the slow-path baseline is stable. Status: implemented as the default RDPGFX Planar path over `drdynvc`, with no-alpha RLE planes and a blocking FreeRDP `/sec:nla /gfx` CI proof (`rdpgfx_seen=true`, screenshot present, non-timeout shutdown). The compatibility-safe 24-bit BGR bitmap encoder and 320x240/1280x720/1920x1080 benchmark baselines remain the fallback/oracle; real-device RDPGFX-vs-bitmap measurements are still pending.
6. **H.264/AVC video path** using Android hardware encoding where possible. Status: experimental transport path implemented but not validated as client-compatible. Android can route MediaProjection into `MediaCodec`, gomobile can queue encoded access units, and the server can emit Annex-B-normalized access units as RDPGFX AVC420 `RFX_AVC420_BITMAP_STREAM` frames with H.264 frame/byte counters. Non-blocking CI artifacts prove server-side forced emission, but the current runner FreeRDP build does not advertise AVC420 support; real-client proof and physical-device performance measurement remain pending.

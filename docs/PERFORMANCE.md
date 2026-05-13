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

## Probe metrics

`cmd/probe` records:

- packet counts and bytes read/written;
- bitmap update count;
- bitmap payload bytes, rectangles, and pixels composed;
- total duration, handshake time, bitmap-read time, and first-bitmap latency;
- aggregate read and bitmap throughput in Mbps;
- average update size and update read time.

The current slow-path bitmap renderer sends uncompressed 24-bit BGR tiles with 4-byte row alignment. This is still classic bitmap update transport, but it avoids sending an unused alpha byte and cuts payload by roughly 25% versus the initial 32-bit BGRA baseline. Android frame ingress and the Go bitmap tiling/scaling paths validate dimensions, row stride, backing data length, and integer-overflow cases before queueing, reading, or allocating frame buffers; malformed frame metadata is dropped rather than silently producing partial tiles or oversized allocations. For a 1080x2400 emulator screen and an 80x80 tile size, one full frame is:

```text
ceil(1080/80) * ceil(2400/80) = 14 * 30 = 420 bitmap updates
```

This is intentionally simple and measurable. The primary optimization targets are unchanged-tile suppression, adaptive frame pacing, optional downscaling, compressed bitmap/RDPGFX-style updates, and H.264/AVC video transport.

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
3. **Capture pacing/backpressure** so MediaProjection does not copy frames faster than the RDP encoder can drain them. Status: implemented in layers: Android adapts capture interval based on bridge submission time, the bounded Go `FrameQueue` drops oldest frames when full, and the RDP stream loop coalesces queued backlog to the latest frame before encoding. Emulator validation from CI run `25246333819` stayed green and browser scene fell to 302 dirty updates; real-device/network backpressure validation remains pending.
4. **Optional downscale mode** for low-bandwidth viewing. Status: implemented and emulator-validated as `emulator_capture_scale` / `capture_scale` plumbing through CI, Android intent extras, and MediaProjection virtual display sizing; scale=2 run `25247259184` captured 540x1200 frames with 105 full-frame tiles and 2.59 MB/frame payload.
5. **Compression/RDPGFX** once the slow-path baseline is stable. Status: started with a compatibility-safe 24-bit BGR bitmap encoder; local 320x240/1280x720/1920x1080 benchmark baselines are now recorded above; RLE/RDPGFX negotiation and compressed bitmap streams are still pending.
6. **H.264/AVC video path** using Android hardware encoding where possible. Status: not started. This should be tracked separately from bitmap/RDPGFX work because it changes the capture pipeline from `ImageReader` RGBA frames toward encoder surfaces or RGBA-to-encoder conversion, and it requires client/protocol capability negotiation for video-oriented graphics updates.

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
6. Runs `cmd/probe` for each scene and stores paired Android/RDP screenshots plus JSON metrics.

## Artifacts

Expected artifact files include:

- `rdp-home.png`, `rdp-settings.png`, `rdp-browser.png` — RDP-rendered screenshots.
- `android-home.png`, `android-settings.png`, `android-browser.png` — emulator screenshots for comparison.
- `rdp-*-summary.json` — per-scene RDP performance counters.
- `performance-summary.md` — markdown roll-up of the JSON summaries.
- `rdp-capture-plan.txt` — selected tile/update count.

## Probe metrics

`cmd/probe` records:

- packet counts and bytes read/written;
- bitmap update count;
- bitmap payload bytes, rectangles, and pixels composed;
- total duration, handshake time, bitmap-read time, and first-bitmap latency;
- aggregate read and bitmap throughput in Mbps;
- average update size and update read time.

The current slow-path bitmap renderer sends uncompressed 32-bit tiles. For a 1080x2400 emulator screen and an 80x80 tile size, one full frame is:

```text
ceil(1080/80) * ceil(2400/80) = 14 * 30 = 420 bitmap updates
```

This is intentionally simple and measurable. The primary optimization targets are unchanged-tile suppression, adaptive frame pacing, optional downscaling, compressed bitmap/RDPGFX-style updates, and H.264/AVC video transport.

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

- One uncompressed full-resolution frame costs about **10.4 MB** on the wire.
- A full frame is **420 separate slow-path bitmap PDUs** at the current 80x80 tile size.
- The first scene after capture startup is slower; later scenes are dominated by bitmap transfer/composition.
- Exact full-frame update counting removes 30 unnecessary update reads compared with the earlier 450-update probe default.

Performance workstreams:

1. **Dirty-tile suppression** after the initial frame to avoid resending unchanged tiles during idle periods. Status: implemented for post-initial stream frames using per-tile hashes; unit coverage verifies unchanged frames emit no updates and one-pixel changes emit one tile.
2. **Adaptive probe/session mode** to keep one RDP connection open while driving navigation, measuring incremental scene changes rather than reconnecting for every screenshot. Status: implemented and validated as the default Go-backed MediaProjection CI path via `cmd/probe -scene-plan`.
3. **Capture pacing/backpressure** so MediaProjection does not copy frames faster than the RDP encoder can drain them. Status: first pass implemented and emulator-validated with adaptive capture interval based on bridge submission time plus capture telemetry; CI run `25246333819` stayed green and browser scene fell to 302 dirty updates.
4. **Optional downscale mode** for low-bandwidth viewing. Status: implemented as `emulator_capture_scale` / `capture_scale` plumbing through CI, Android intent extras, and MediaProjection virtual display sizing; pending emulator validation.
5. **Compression/RDPGFX** once the slow-path baseline is stable. Status: not started.
6. **H.264/AVC video path** using Android hardware encoding where possible. Status: not started. This should be tracked separately from bitmap/RDPGFX work because it changes the capture pipeline from `ImageReader` RGBA frames toward encoder surfaces or RGBA-to-encoder conversion, and it requires client/protocol capability negotiation for video-oriented graphics updates.

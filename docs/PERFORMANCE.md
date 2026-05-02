# Performance measurement

The CI emulator workflow can collect repeatable RDP performance baselines from the Go-backed Android app.

Run it manually with:

```sh
gh workflow run CI \
  --ref main \
  -f emulator_api_level=35 \
  -f emulator_go_backed=true \
  -f emulator_capture=true
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

Next optimization candidates:

1. **Dirty-tile suppression** after the initial frame to avoid resending unchanged tiles during idle periods.
2. **Adaptive probe/session mode** to keep one RDP connection open while driving navigation, measuring incremental scene changes rather than reconnecting for every screenshot.
3. **Capture pacing/backpressure** so MediaProjection does not copy frames faster than the RDP encoder can drain them.
4. **Optional downscale mode** for low-bandwidth viewing.
5. **Compression/RDPGFX** once the slow-path baseline is stable.
6. **H.264/AVC video path** using Android hardware encoding where possible. This should be tracked separately from bitmap/RDPGFX work because it changes the capture pipeline from `ImageReader` RGBA frames toward encoder surfaces or RGBA-to-encoder conversion, and it requires client/protocol capability negotiation for video-oriented graphics updates.

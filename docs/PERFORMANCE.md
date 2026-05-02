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

This is intentionally simple and measurable. The primary optimization targets are unchanged-tile suppression, adaptive frame pacing, optional downscaling, and eventually compressed bitmap or RDPGFX-style updates.

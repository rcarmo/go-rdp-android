# Testing and CI

`go-rdp-android` is CI-first. Physical device testing is still required before real-world use, but GitHub Actions already exercises the protocol stack, Android packaging, gomobile binding, emulator capture, input scripting, RDP screenshot generation, and UX report production.

## Test layers

| Layer | Purpose | Main commands / artifacts |
| --- | --- | --- |
| Go unit tests | Parser, graphics, input, bridge, lifecycle coverage, including bitmap fallback conversion/tiling, RDPGFX Planar round-trip encoding, and slow-path/Fast-Path input sink equivalence | `go test ./...` |
| RDPEI/drdynvc summary | Machine-readable evidence for RDPEI parser, DVC routing, synthetic touch sequence, touch lifecycle metadata, and DVC fragment counter behavior | `test-artifacts/go/rdpei-tests.json`, `test-artifacts/go/rdpei-test-summary.md` |
| Go coverage | Enforce minimum project coverage | `make coverage COVERAGE_MIN=75.0` |
| gosec scan | Static security scan with triaged overflow-noise exclusion (`G115`) | `test-artifacts/go/gosec-report.json`, `test-artifacts/go/gosec-summary.md` |
| Race/fuzz smoke | Catch concurrency and parser edge issues | `go test -race ./...`, short fuzz run |
| Mock/probe smoke | Exercise desktop RDP handshake, bitmap path, TLS-only auth, and Hybrid/NLA auth; emits JSON/Markdown probe summaries, logs/traces, and an RDP screenshot | `mock-probe-artifacts` |
| Android build | Build and inspect normal debug APK | `android-build-artifacts` |
| gomobile build | Build `mobile.aar`, verify API, build Go-backed APK + AAB | `gomobile-build-artifacts` |
| FreeRDP probe | Blocking real-client compatibility gate; retries and requires bitmap fallback streaming plus an RDPGFX `/gfx` proof gate | `freerdp-compat-probe` |
| Emulator capture | Validate app startup, MediaProjection, RDP screenshots | `android-emulator-artifacts` |
| UX report | Validate Gherkin stories and produce PDF | `ux-report/ux-report.pdf` |

## Default CI gates

Default push/PR CI runs without a physical Android device:

- Go vet/build/test with coverage threshold, including bitmap fallback encoder tests and RDPGFX Planar round-trip encoding coverage for signed delta/wrap cases.
- RDPEI/`drdynvc` test JSON plus Markdown summary covering parser, synthetic channel sequence, touch lifecycle metadata, and DVC fragment counter behavior.
- Go race tests and short parser fuzz smoke.
- gosec static security scan (currently excluding `G115` cast-noise with findings triaged and documented).
- Mock server + probe TCP smoke test.
- TLS-only Client Info and Hybrid/NLA CredSSP authentication smoke tests, including bad-password rejection and an `auth-summary.md` artifact that records expected success/failure outcomes.
- Protocol packet trace artifact from the probe, including client/server hex dumps, logs, JSON/Markdown probe summaries, and an RDP screenshot from bitmap updates.
- Always-uploaded CI diagnostics now include mock-server logs, client/probe logs, JSON and Markdown summaries, and screenshots for the mock/probe, auth, FreeRDP, Android build, gomobile, and emulator/UX paths where applicable.
- Normal Android debug APK build and APK structure inspection.
- `gomobile bind` AAR generation.
- Generated AAR Java API signature verification.
- Generated AAR native library/content inspection.
- Go-backed APK and AAB builds against `mobile.aar`, with native library/content inspection plus AAB signature report.
- FreeRDP compatibility probe log, summary, and screenshot capture against a mock server with animated test-pattern frames.

The FreeRDP job is now a blocking compatibility gate for `/sec:rdp`, `/sec:tls`, `/sec:nla`, and `/sec:nla /gfx`. Each mode retries up to three isolated Xvfb/FreeRDP attempts and preserves per-attempt logs under `freerdp-artifacts/<mode>/attempt-*`. The first three modes explicitly disable RDPGFX to keep the slow-path bitmap fallback honest and require active state, bitmap updates (`active_seen=true`, `bitmap_seen=true`), Fast-Path input traffic, screenshot capture, and non-timeout client shutdown (`exit_code != 124`). The `nla-gfx` mode runs a small-geometry mock server with default RDPGFX enabled and requires active RDPGFX negotiation/streaming (`rdpgfx_seen=true`) plus screenshot capture and non-timeout shutdown. The NLA modes use static credentials (`runner` / `secret`) and exercise CredSSP/NTLMv2 plus TLS public-key binding with a real FreeRDP client. CI also emits a non-blocking `h264-gfx` informational artifact: it replays a tiny H.264 access-unit fixture through the mock server, tries `/gfx:AVC420`, falls back to `/gfx` if the runner FreeRDP build rejects the explicit option, and records `h264_status_seen`, `h264_write_seen`, `h264_write_count`, `h264_write_bytes`, `h264_reason`, and `avc420_exit_code`. This artifact is for H.264 transport bring-up only and is not a compatibility gate until a client actually advertises AVC420 without `GO_RDP_ANDROID_FORCE_H264=1`. The current human-readable status matrix for these gates lives in [STATUS](STATUS.md) and should be refreshed whenever gate semantics or evidence changes.

Workflows now set `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` to run JavaScript-based actions on Node 24 ahead of GitHub’s Node 20 deprecation timeline.

## Manual and tag-driven emulator UX testing

Run manually:

```bash
gh workflow run CI \
  --ref main \
  -f emulator_api_level=35 \
  -f emulator_go_backed=true \
  -f emulator_capture=true \
  -f emulator_capture_scale=2
```

The emulator job:

1. Builds the Go-backed APK if requested or required by tag policy.
2. Installs and launches the app (passing explicit test credentials via intent extras).
3. Requests and accepts MediaProjection permission.
4. Starts the Go RDP server inside the app (startup is now blocked unless credentials are configured).
5. Enables the app AccessibilityService in the emulator so RDP input callbacks can execute Home, pointer, and RDPEI touch gestures.
6. Forwards runner TCP/3390 to emulator TCP/3390.
7. Captures a home RDP screenshot.
8. Drives Android Settings, Settings search, mouse tap, notification swipe, and browser scenes. Browser launch is driven by RDP Home scancode `0x47` plus a synthetic RDPEI touch tap on the browser icon.
9. Captures paired Android and RDP screenshots.
10. Generates `rdp-probe-summary.json` and `performance-summary.md`.
11. Validates `features/ux/*.feature` and generates the Playwright PDF report.

The same full UX path runs automatically for `*-ux` tags and release tags (`vX.X.X`).

Release-tag staging now expects production signing secrets in GitHub Actions (`RELEASE_KEYSTORE_BASE64`, `RELEASE_KEYSTORE_PASSWORD`, `RELEASE_KEY_ALIAS`, `RELEASE_KEY_PASSWORD`) and fails fast if they are absent.

## Local encoding matrix

Use the local FreeRDP encoding matrix when changing graphics transport or before release-candidate testing:

```bash
make encoding-matrix
# or choose an output directory:
scripts/encoding-matrix.sh /workspace/tmp/rdp-encoding-matrix-$(date +%Y%m%d-%H%M%S)
```

Requirements: `xfreerdp3`/`xfreerdp`, `Xvfb`, and `xwd`. The matrix starts `cmd/mock-server` with a test pattern and runs four NLA cases: slow-path bitmap fallback, RDPGFX Planar with H.264 disabled, forced H.264 with `/gfx:AVC420`, and forced H.264 with `/gfx`. It writes per-case FreeRDP logs, mock-server logs, screenshots, JSON/Markdown summaries, a top-level `summary.md`, and a machine-readable `codec-coverage.json`; it exits non-zero if bitmap, RDPGFX Planar, or forced H.264 evidence is missing. Treat the H.264 cases as protocol smoke evidence only unless the client advertises AVC420 without `GO_RDP_ANDROID_FORCE_H264=1`. The summary also lists observed RDPGFX capability advertisements plus missing/unimplemented encoding families so the matrix is explicit about not covering RDP bitmap RLE, NSCodec, RemoteFX/RFX, AVC444/AVC444v2, ClearCodec, progressive codecs, or JPEG/PNG bitmap codecs yet.

## Nightly/optional FreeRDP soak

A dedicated `FreeRDP soak` workflow now runs nightly (cron) and supports manual `workflow_dispatch` runs. It repeatedly connects `xfreerdp` to the mock server in one selected security mode (`rdp`/`tls`/`nla`), captures per-iteration exit codes and server RSS, and fails when memory growth exceeds a configurable threshold. Each iteration is now bounded by a hard timeout (`SOAK_ITERATION_TIMEOUT_SEC`, default `45`) and escalates client shutdown (`INT` → `TERM` → `KILL`) to avoid stuck `xfreerdp` attempts stalling the entire soak run.

Primary outputs:

- `soak-artifacts/soak.csv` (iteration, mode, exit code, RSS KB)
- `soak-artifacts/summary.md` (min/max/delta RSS and pass/fail)
- per-iteration `soak-artifacts/attempts/*/xfreerdp.log`

## Workflow inputs

| Input | Default | Notes |
| --- | --- | --- |
| `emulator_api_level` | `35` | Android emulator API level. |
| `emulator_go_backed` | `false` | Build/install Go-backed APK with `mobile.aar`. Required for RDP proof. |
| `emulator_capture` | `false` | Request MediaProjection and capture the emulator display. |
| `emulator_capture_scale` | `1` for manual unless set, `2` in tag defaults | Downscale factor for MediaProjection/RDP capture. |

## UX stories

Feature files live under:

```text
features/ux/
```

Current scenarios cover:

- starting a captured RDP desktop session;
- searching Android Settings with keyboard text input;
- hitting a deterministic Settings target using mouse input;
- swiping down to reveal notifications using touchscreen input;
- returning to the Android home screen with the RDP Home scancode and opening the browser with a synthetic RDPEI touch tap, verifying the browser comes foreground;
- validating per-scene performance and screenshot sections in the report.

## UX report artifacts

The report generator reads emulator artifacts and writes:

```text
ux-report/ux-report.md
ux-report/ux-report.html
ux-report/ux-report.pdf
ux-report/ux-validation.json
```

`ux-validation.json` is the machine-readable pass/fail source. The PDF is the human release artifact.

Local generation from a downloaded artifact directory:

```bash
npm ci
npx playwright install --with-deps chromium
npm run ux:report -- --artifacts /path/to/emulator-artifacts --out /path/to/emulator-artifacts/ux-report
```

## Expected emulator checks

`checks.txt` should include:

```text
startServer=ok
frame1=ok
screen_capture=ok
fatal_exception=none
keyboard_settings_search=ok
mouse_target_tap=ok
rdpei_browser_tap=ok
touch_notification_swipe=ok
rdp_input_screenshots=ok
```

## Important artifacts

| Artifact | Meaning |
| --- | --- |
| `logcat-filtered.txt` | App-focused logcat subset with startup/capture/frame lines. |
| `capture-consent.txt` | Emulator size, capture scale, and MediaProjection tap coordinates. |
| `rdp-capture-plan.txt` | Tile size, capture scale, and selected update count. |
| `input-validation-plan.txt` | Keyboard/mouse/touch script coordinates and actions. |
| `rdp-*.png` | RDP-rendered screenshots. |
| `android-*.png` | Emulator screenshots for comparison. |
| `rdp-probe-summary.json` | Packet, bitmap, timing, and per-scene metrics. |
| `performance-summary.md` | Human-readable metrics rollup. |
| `ux-report/ux-report.pdf` | Final UX report deliverable. |

## Real-client validation matrix

Use this matrix for manual client evidence before a public APK. Keep screenshots/logs alongside the run notes and copy any surprising behavior into [STATUS](STATUS.md).

| Date | Android device/build | Client platform/version | Security mode | Result | Evidence | Known limitations |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-05-18 | mock server/test pattern | FreeRDP CI `/sec:rdp`, `/sec:tls`, `/sec:nla` | rdp/tls/nla | automated bitmap fallback active-streaming gates pass | `freerdp-compat-probe` artifact from CI `26022579115` | Mock/test-pattern evidence, not physical Android hardware. |
| 2026-05-18 | mock server/test pattern | FreeRDP CI `/sec:nla /gfx` | nla/gfx | automated RDPGFX active-streaming proof gate passes | `freerdp-compat-probe` artifact from CI `26022579115`; `rdpgfx_seen=true` | Mock/test-pattern evidence, not physical Android hardware; small geometry for proof stability. |
| 2026-05-19 | mock server/H.264 fixture | FreeRDP CI informational `h264-gfx` | nla/gfx fallback | server-side forced AVC420 emission trace captured with coalesced SPS/PPS config preserved across latest-frame coalescing and prepended to IDR; client AVC420 support not proven | `freerdp-compat-probe` artifact from CI `26084530884`; `h264_ready=true`, `h264_version=0x000a0600`, `h264_flags=0x00000020`, `h264_write_seen=true`, `h264_write_count=1`, `h264_write_bytes=23`, `h264_reason=forced-by-env`, `avc420_exit_code=24` | Non-blocking diagnostic only; runner FreeRDP rejects `/gfx:AVC420` and falls back to `/gfx`. |
| 2026-05-19 | mock server/local FreeRDP 3.15.0 | Manual local encoding matrix | bitmap, rdpgfx-planar, forced AVC420, forced `/gfx` fallback | all four paths reached active state; bitmap produced slow-path updates, RDPGFX Planar produced graphics-channel activity without H.264 writes, and forced H.264 emitted AVC420 payloads under `/gfx:AVC420` and `/gfx`; matrix lists unimplemented codec families separately | `encoding-matrix-current.tar.gz`; bitmap `active_seen=true`, `bitmap_seen=true`; RDPGFX `active_seen=true`, `rdpgfx_seen=true`, `h264_reason=disabled-by-env`; forced H.264 `/gfx:AVC420` `h264_write_count=31`, `h264_write_bytes=713`; forced H.264 `/gfx` `h264_write_count=30`, `h264_write_bytes=690`, `h264_reason=forced-by-env` | Local FreeRDP 3.15.0 accepts `/gfx:AVC420`, unlike the GitHub runner package; still forced-mode protocol smoke evidence, not negotiated client proof. |
| pending | pending | Microsoft Remote Desktop | nla-required | pending | screenshot, client log, app diagnostics | Release blocker until active streaming and disconnect behavior are recorded. |
| pending | pending | Microsoft Remote Desktop | tls-only | pending | screenshot, client log, app diagnostics | Compatibility fallback only for non-NLA behavior. |

For each manual row, record the app diagnostics text, TLS fingerprint/certificate warning behavior, client screenshot or log, security mode, selected graphics path (`h264-avc`, `rdpgfx-planar`, or `bitmap-fallback`), `h264Status` when present, whether streaming became active, input behavior, and logoff/disconnect cleanup. For codec-priority decisions, also capture the client graphics capability evidence described in [GRAPHICS_CODECS](GRAPHICS_CODECS.md).

## Blocked on physical devices

Still requires real Android hardware validation:

- MediaProjection behavior outside emulator.
- AccessibilityService enablement and actual gesture/key injection UX.
- Real touch latency and sustained frame pacing/backpressure under screen changes and constrained networks.
- Network reachability from a separate RDP client device.
- Rotation, screen lock, protected content, OEM restrictions.

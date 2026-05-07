# Testing and CI

`go-rdp-android` is CI-first. Physical device testing is still required before real-world use, but GitHub Actions already exercises the protocol stack, Android packaging, gomobile binding, emulator capture, input scripting, RDP screenshot generation, and UX report production.

## Test layers

| Layer | Purpose | Main commands / artifacts |
| --- | --- | --- |
| Go unit tests | Parser, graphics, input, bridge, lifecycle coverage, including slow-path/Fast-Path input sink equivalence | `go test ./...` |
| RDPEI/drdynvc summary | Machine-readable evidence for RDPEI parser, DVC routing, synthetic touch sequence, and touch lifecycle metadata | `test-artifacts/go/rdpei-tests.json`, `test-artifacts/go/rdpei-test-summary.md` |
| Go coverage | Enforce minimum project coverage | `make coverage COVERAGE_MIN=75.0` |
| Race/fuzz smoke | Catch concurrency and parser edge issues | `go test -race ./...`, short fuzz run |
| Mock/probe smoke | Exercise desktop RDP handshake, bitmap path, TLS-only auth, and Hybrid/NLA auth | `mock-probe-artifacts` |
| Android build | Build and inspect normal debug APK | `android-build-artifacts` |
| gomobile build | Build `mobile.aar`, verify API, build Go-backed APK | `gomobile-build-artifacts` |
| FreeRDP probe | Blocking real-client compatibility gate; retries and requires bitmap/update streaming | `freerdp-compat-probe` |
| Emulator capture | Validate app startup, MediaProjection, RDP screenshots | `android-emulator-artifacts` |
| UX report | Validate Gherkin stories and produce PDF | `ux-report/ux-report.pdf` |

## Default CI gates

Default push/PR CI runs without a physical Android device:

- Go vet/build/test with coverage threshold.
- RDPEI/`drdynvc` test JSON plus Markdown summary covering parser, synthetic channel sequence, and touch lifecycle metadata.
- Go race tests and short parser fuzz smoke.
- Mock server + probe TCP smoke test.
- TLS-only Client Info and Hybrid/NLA CredSSP authentication smoke tests, including bad-password rejection.
- Protocol packet trace artifact from the probe, including client/server hex dumps and logs.
- Normal Android debug APK build and APK structure inspection.
- `gomobile bind` AAR generation.
- Generated AAR Java API signature verification.
- Generated AAR native library/content inspection.
- Go-backed APK build against `mobile.aar` and native library/content inspection.
- FreeRDP compatibility probe log, summary, and screenshot capture against a mock server with animated test-pattern frames.

The FreeRDP job is now a blocking compatibility gate for `/sec:rdp`, `/sec:tls`, and `/sec:nla`. Each mode retries up to three isolated Xvfb/FreeRDP attempts, preserves per-attempt logs under `freerdp-artifacts/<mode>/attempt-*`, and requires at least one attempt per mode to reach FreeRDP active state, stream bitmap updates (`active_seen=true`, `bitmap_seen=true`), handle Fast-Path input traffic, and remain connected until the CI timeout terminates the client (`exit_code=124`). The NLA mode uses static credentials (`runner` / `secret`) and exercises CredSSP/NTLMv2 plus TLS public-key binding with a real FreeRDP client. The current human-readable status matrix for these gates lives in [STATUS](STATUS.md) and should be refreshed whenever gate semantics or evidence changes.

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
2. Installs and launches the app.
3. Requests and accepts MediaProjection permission.
4. Starts the Go RDP server inside the app.
5. Enables the app AccessibilityService in the emulator so RDP input callbacks can execute Home, pointer, and RDPEI touch gestures.
6. Forwards runner TCP/3390 to emulator TCP/3390.
7. Captures a home RDP screenshot.
8. Drives Android Settings, Settings search, mouse tap, notification swipe, and browser scenes. Browser launch is driven by RDP Home scancode `0x47` plus a synthetic RDPEI touch tap on the browser icon.
9. Captures paired Android and RDP screenshots.
10. Generates `rdp-probe-summary.json` and `performance-summary.md`.
11. Validates `features/ux/*.feature` and generates the Playwright PDF report.

The same full UX path runs automatically for `*-ux` tags and release tags (`vX.X.X`).

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

## Blocked on physical devices

Still requires real Android hardware validation:

- MediaProjection behavior outside emulator.
- AccessibilityService enablement and actual gesture/key injection UX.
- Real touch latency and sustained frame pacing.
- Network reachability from a separate RDP client device.
- Rotation, screen lock, protected content, OEM restrictions.

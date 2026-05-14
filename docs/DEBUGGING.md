# Debugging notes

This document collects practical debugging procedures for the desktop Go protocol path, Android packaging path, gomobile bridge, emulator capture path, and UX report pipeline.

## Quick triage

Start with the latest GitHub Actions run:

```bash
make ci-list
make ci-status
make ci-jobs
make ci-log        # failed logs only
make ci-log-all    # full logs
```

For a specific run:

```bash
gh run view <run-id> --json status,conclusion,jobs
gh run view <run-id> --log-failed
```

Download artifacts:

```bash
gh run download <run-id> -n android-emulator-artifacts -D /tmp/emulator-artifacts
gh run download <run-id> -n mock-probe-artifacts -D /tmp/mock-probe
gh run download <run-id> -n gomobile-build-artifacts -D /tmp/gomobile
```

## Desktop protocol debugging

Run the mock server and probe locally:

```bash
make run-mock-pattern
make probe
```

For the structured server trace phase taxonomy and diagnostic bundle sources, see [TRACE_PHASES](TRACE_PHASES.md).

For packet traces in CI, inspect `mock-probe-artifacts/protocol-trace/`. The probe emits client/server hex dumps, JSON/Markdown summaries, and an RDP screenshot from bitmap updates. Useful files:

- `probe.log`
- `probe-summary.json`
- `probe-summary.md`
- `rdp-screenshot.png`
- `protocol-trace/index.txt`
- `mock-server.log`
- `../auth/auth-summary.md` plus `probe-good.log`, `probe-nla-good.log`, `probe-bad.log`, and `probe-nla-bad.log` for authentication smoke details.

Enable server trace logs with either the legacy boolean knob or the runtime log-level knob:

```bash
GO_RDP_ANDROID_TRACE=1 go run ./cmd/mock-server -test-pattern
GO_RDP_ANDROID_LOG_LEVEL=debug go run ./cmd/mock-server -test-pattern
```

Common protocol failure points:

- X.224 negotiation mismatch.
- MCS channel join sequence mismatch.
- Client Info / security header parsing.
- Demand Active / Confirm Active finalization.
- FontList / FontMap ordering.
- Oversized bitmap updates.

## FreeRDP compatibility debugging

The `FreeRDP compatibility probe` job is a blocking CI gate for `/sec:rdp`, `/sec:tls`, and `/sec:nla`. It launches the mock server under Xvfb, retries up to three FreeRDP attempts for each mode, and requires at least one attempt per mode to reach active bitmap/update streaming. It captures:

- top-level best-attempt `xfreerdp.log`
- top-level best-attempt `mock-server.log`
- top-level best-attempt `summary.md`
- top-level best-attempt `summary.json`
- top-level best-attempt `xfreerdp-root.png`
- per-attempt logs under `<mode>/attempt-*`

For all security modes, the gate requires `active_seen=true`, `bitmap_seen=true`, and a non-timeout shutdown (`exit_code != 124`) after screenshot capture. Use the summaries to locate the last successful server trace phase and the per-attempt logs to distinguish real protocol regressions from Xvfb/client startup flakiness. If a job hits the workflow timeout while traces show active streaming/screenshot evidence, rerun the failed FreeRDP job once before treating it as a protocol regression; persistent timeout or repeated missing `active_seen`/`bitmap_seen` should be triaged as blocking.

## Android build debugging

Normal Android APK:

```bash
make android-build
make check-apk-artifact
```

Go-backed Android APK:

```bash
make gomobile-init
make android-build-go
make check-aar-api
make check-aar-artifact
make check-apk-artifact REQUIRE_GO_LIBS=1
```

CI artifacts to inspect:

- `android-build-artifacts/apk-summary.md`
- `android-build-artifacts/apk-contents.txt`
- `gomobile-build-artifacts/aar-api.log`
- `gomobile-build-artifacts/go-backed-apk-summary.md`

Common issues:

- Android 16 KB page-size warnings such as `lib/arm64-v8a/libgojni.so: LOAD segment not aligned` mean the gomobile native library was not linked with 16 KB-compatible ELF segment alignment. The Makefile passes `-ldflags="-extldflags=-Wl,-z,max-page-size=16384"`, and `check-android-artifact.go` fails CI if bundled `libgojni.so` PT_LOAD alignments are below `0x4000`.
- Missing `mobile.aar` means the app falls back to `LoggingRdpBackend`.
- Generated gomobile API drift is caught by `make check-aar-api`.
- Native library packaging issues are caught by `make check-aar-artifact` and `make check-apk-artifact REQUIRE_GO_LIBS=1`.
- Kotlin/Java toolchain mismatches should keep Java/Kotlin at JVM 17.

## Emulator capture debugging

Run a Go-backed MediaProjection emulator test manually:

```bash
gh workflow run CI \
  --ref main \
  -f emulator_api_level=35 \
  -f emulator_go_backed=true \
  -f emulator_capture=true \
  -f emulator_capture_scale=2
```

Important artifacts:

- `checks.txt`
- `logcat.txt`
- `logcat-filtered.txt`
- `capture-consent.txt`
- `mediaprojection-dialog.png`
- `mediaprojection-scope-menu.png`
- `screenshot.png`
- `rdp-home.png`, `rdp-settings.png`, `rdp-browser.png`
- `rdp-probe-summary.json`
- `performance-summary.md`

Expected checks:

```text
startServer=ok
frame1=ok
screen_capture=ok
fatal_exception=none
```

If `screen_capture=missing`:

1. Inspect `mediaprojection-dialog.png` and `mediaprojection-scope-menu.png`.
2. Confirm the tap coordinates in `capture-consent.txt` still match the current emulator UI.
3. Check logcat for `MediaProjectionPermissionActivity` and `Screen capture started`.

If RDP probe fails:

1. Check `adb forward` output and `rdp-probe.log`.
2. Look for `startServer(... backend=gomobile)` in logcat.
3. Confirm `frame#1` dimensions and byte count match the expected capture scale.
4. Inspect `rdp-capture-plan.txt` for tile count.

## Input validation debugging

The emulator UX path writes `input-validation-plan.txt` and explicit checks:

```text
keyboard_settings_search=ok
mouse_target_tap=ok
touch_notification_swipe=ok
rdp_input_screenshots=ok
```

Scripted inputs currently cover:

- Settings search with `wifi` typed from the emulator keyboard path.
- Mouse-source tap at a deterministic coordinate.
- Touchscreen swipe from top to notification shade.

Artifacts:

- `android-settings-search.png`
- `android-mouse-target.png`
- `android-notifications.png`
- `rdp-settings-search.png`
- `rdp-mouse-target.png`
- `rdp-notifications.png`

If an RDP scene has zero updates, the screenshot can still be useful: it is the current RDP canvas after dirty-tile suppression. Compare it with the paired Android screenshot to decide whether the scene was actually stale or simply unchanged.

## UX report debugging

The Playwright report pipeline reads:

- `features/ux/*.feature`
- `checks.txt`
- `input-validation-plan.txt`
- `rdp-probe-summary.json`
- required Android/RDP screenshots

It writes:

- `ux-report/ux-report.md`
- `ux-report/ux-report.html`
- `ux-report/ux-report.pdf`
- `ux-report/ux-validation.json`

Local report generation from downloaded artifacts:

```bash
npm ci
npx playwright install --with-deps chromium
npm run ux:report -- --artifacts /path/to/emulator-artifacts --out /path/to/emulator-artifacts/ux-report
```

If PDF rendering times out, check that image paths in `ux-report.html` are file URLs and that Playwright dependencies are installed. CI uses:

```bash
npx playwright install --with-deps chromium
```

## Performance debugging

Use [PERFORMANCE](PERFORMANCE.md) for baseline numbers. Key fields in `rdp-probe-summary.json`:

- `bitmap_updates`
- `bitmap_payload_bytes`
- `bitmap_rectangles`
- `duration_ms`
- per-scene `updates`

Logcat capture telemetry is emitted as `captureStats`, for example:

```text
captureStats submitted=1 throttled=1 copiedBytes=2592000 avgSubmitMs=18 maxSubmitMs=18 adaptiveIntervalMs=66 targetIntervalMs=66
```

If capture is too slow or memory-heavy:

- Use `emulator_capture_scale=2` or higher.
- Inspect `captureStats` for high submit times.
- Inspect per-scene tile counts for dirty-tile suppression effectiveness.
- Check Android-side frame geometry if frames stop appearing: the bridge accepts Android-style unpadded final rows, but still rejects invalid dimensions, pixel stride, row stride, data length, and overflow cases before queueing.

## Android service/network debugging

When the foreground service is running, its notification includes the current local IPv4 address list. Network changes also log lines similar to:

```text
Network available: ... local=192.168.1.23
Network lost: ... local=no IPv4 address
```

Use those lines to troubleshoot Wi-Fi reconnects, IP changes, hotspot mode, and VPN routing. The server still binds all interfaces; the notification/log refresh is diagnostic and does not silently restart the listener.

If the service is started without credentials, it logs a refusal, removes the temporary foreground notification, and stops without opening a listener.

If native startup fails after credentials are present, look for `Native RDP server failed to start` in logcat and a `startServer failed(...)` line from `GoRdpAndroid`. The mobile bridge binds the listener synchronously before reporting success, so occupied ports or other listen errors should leave health as `running=false`, remove the foreground notification, reset the persisted last mode to `none`, and avoid starting MediaProjection/test-pattern capture.

If listener shutdown behavior looks suspicious, check that `addr=n/a` or an empty gomobile address is reported after Stop. Core `Server.Serve` clears its listener address and stops its context-watcher goroutine on all exit paths, so stale addresses after Stop should be treated as a regression.

For RDPEI touch issues after toggling Accessibility, verify that new touch frames start only while `input=enabled` in the health line. The Android input service clears any pending frame batch when Accessibility is disconnected at frame start or disappears before frame end; stale contacts affecting a later reconnect should be treated as a regression.

Use the app's **Refresh Status** button before collecting a report, then **Share Diagnostics** to export bounded text diagnostics through Android's share sheet. The shared text is intended for bug reports and includes the compact bridge health, configured username, password-present flag, capture scale, selected security mode, failed-auth backoff/lockout policy, last mode, and Accessibility state; it deliberately omits the password and raw frame data.

## Authentication debugging

The current authentication hook is a username/password check used by both the classic Client Info path and the Hybrid/NLA CredSSP path:

```go
rdpserver.Config{Authenticator: rdpserver.StaticCredentials{Username: "user", Password: "pass"}}
```

or through gomobile:

```go
mobile.SetCredentials("user", "pass")
```

The mock server can require credentials:

```bash
go run ./cmd/mock-server -username user -password pass
```

Security/access policy controls are also available on the mock server:

```bash
go run ./cmd/mock-server \
  -security-mode tls-only \
  -allowed-users user,admin \
  -allowed-cidrs 192.168.1.0/24,127.0.0.0/8 \
  -username user -password pass
```

Valid `-security-mode` values are: `negotiate`, `rdp-only`, `tls-only`, `nla-required`.

Failed-auth lockout/backoff can be enabled for brute-force resilience:

```bash
go run ./cmd/mock-server \
  -username user -password pass \
  -failed-auth-limit 3 \
  -failed-auth-backoff 2s \
  -failed-auth-backoff-max 1m
```

When enabled, failed auth attempts are exponentially backed off per remote/user identity (with host-level fallback for early CredSSP failures), and logs include retry hints.

For TLS Client Info-only experiments, the mock server also accepts a bcrypt hash (avoids storing plaintext in scripts/files):

```bash
go run ./cmd/mock-server -username user -password-hash '$2a$10$...'
```

(`-password-hash` is not valid for NLA/CredSSP flows, which still require plaintext-equivalent input for NTLM verification.)

Probe credentials can be sent through the TLS-only Client Info path or through NLA:

```bash
go run ./cmd/probe -username user -password pass
go run ./cmd/probe -nla -username user -password pass
```

CI includes authentication smoke tests for both paths. Good credentials complete the probe; bad classic credentials log `auth failed`; bad NLA credentials fail during CredSSP/NTLMv2 verification and log `NLA/CredSSP failed`.

The server negotiates `PROTOCOL_SSL` for TLS-only clients and `PROTOCOL_HYBRID` for NLA-capable clients. Hybrid sessions run CredSSP/NTLMv2 before MCS Connect, validate TLS public-key binding, decrypt `TSCredentials`, and then apply the same static credential gate.

## TLS certificate persistence/rotation debugging

By default the server uses an in-memory self-signed cert. For persistent certs and explicit rotation in mock-server runs:

```bash
go run ./cmd/mock-server \
  -tls-cert /tmp/go-rdp-android/server.crt \
  -tls-key /tmp/go-rdp-android/server.key \
  -security-mode tls-only \
  -username user -password pass
```

Force rotation at startup:

```bash
go run ./cmd/mock-server \
  -tls-cert /tmp/go-rdp-android/server.crt \
  -tls-key /tmp/go-rdp-android/server.key \
  -tls-rotate \
  -security-mode tls-only \
  -username user -password pass
```

The server logs the SHA-256 certificate fingerprint (`tls_fp=...`) on handshake; use it as the trust-check value when validating cert changes in client troubleshooting notes.

## Release debugging

Tag policy is documented in [RELEASES](RELEASES.md).

Release tags (`vX.X.X`) should produce:

- APK artifacts from Android/gomobile jobs.
- `go-rdp-android-release-ux-report` containing `go-rdp-android-vX.X.X-ux-report.pdf`.
- Full emulator artifacts if the release UX path ran.

Before tagging a release, verify:

```bash
cat VERSION
grep -n "versionName\|versionCode\|applicationId" android/app/build.gradle.kts
cat package.json | grep '"version"'
```

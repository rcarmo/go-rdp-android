# Android App Notes

The Android app is a Kotlin shell around a gomobile-backed Go RDP server core, with a logging backend fallback that keeps UI/package work buildable when `mobile.aar` is absent.

## Current app responsibilities

- request MediaProjection permission
- start a foreground service
- expose an AccessibilityService declaration
- bridge frames/input/health through `NativeRdpBridge`

## gomobile bridge

`NativeRdpBridge.kt` routes to a gomobile-generated AAR exposing the Go server core when available, with a logging fallback for non-Go-backed builds.

The Android shell now prefers a gomobile-generated Go backend when `android/app/libs/mobile.aar` is present. If the AAR is absent, it falls back to a logging backend so CI and UI work can continue.

CI includes two off-device Android paths:

- default push/PR path: build the normal debug APK and separately generate `mobile.aar` with `gomobile bind`, then build a Go-backed APK against that AAR.
- manual `workflow_dispatch` path: an Android emulator smoke test that installs the debug APK and launches `MainActivity`.

The emulator job is intentionally manual until the gomobile/AAR path is stable. Neither path replaces physical-device validation of MediaProjection/Accessibility, but together they catch packaging, binding and startup failures before device testing is available.

Build the Go AAR and app with:

```bash
make gomobile-init   # first time, installs/initializes gomobile
make android-build-go
```

Current Go mobile-facing API scaffold lives in `mobile/bridge.go`:

```go
func SetCredentials(username, password string)
func StartServer(port int) error
func SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error
func StopServer() error
func Addr() string
func TLSFingerprintSHA256() string
func ActiveConnections() int64
func AcceptedConnections() int64
func AuthFailures() int64
func HandshakeFailures() int64
func InputEvents() int64
func RDPEIContacts() int64
func FramesSent() int64
func BitmapBytes() int64
func DVCFragments() int64
func SubmittedFrames() int64
func QueuedFrames() int64
func DroppedFrames() int64
```

It also exposes `type Server` and a bounded `FrameQueue` for tests/non-singleton usage. Captured Android `RGBA_8888` buffers are copied into a bounded Go queue and consumed as `frame.Source` by the RDP server. The bridge validates frame dimensions, pixel stride, row stride, minimum backing data length (allowing Android-style unpadded final rows), and overflow cases before queueing a buffer, drains stale queued frames around native server restarts, handles already-closed queue drains safely, and now binds the listener synchronously so listen failures (for example, an occupied port) are reported back to Kotlin before the service treats startup as successful; the foreground service tears itself down if native startup fails and does not persist a successful last mode for failed native starts.

Decoded RDP input is surfaced through a gomobile-friendly callback interface:

```go
type InputHandler interface {
    PointerMove(x int, y int)
    PointerButton(x int, y int, buttons int, down bool)
    PointerWheel(x int, y int, delta int, horizontal bool)
    Key(scancode int, down bool)
    Unicode(codepoint int)
    TouchFrameStart(contactCount int)
    TouchContact(contactID int, x int, y int, flags int)
    TouchFrameEnd()
}

func SetInputHandler(handler InputHandler)
```

`NativeRdpBridge` now routes to `GomobileRdpBackend` via reflection when the generated `mobile.Mobile` classes exist, otherwise to `LoggingRdpBackend`. The reflection shim keeps the app buildable before gomobile artifacts are generated while still wiring the runtime path to Go once `mobile.aar` is bundled.

For CI/emulator testing, `MainActivity` accepts `--ez start_test_pattern true` or `--ez start_capture true` plus optional `--es rdp_username <user> --es rdp_password <pass>`. The app now blocks service start until credentials are configured (first-run UI save or intent-provided values), then passes credentials, the selected security policy (`negotiate`, `rdp-only`, `tls-only`, or `nla-required`), and normalized failed-auth backoff/lockout settings into `RdpForegroundService` and the gomobile bridge before starting the server. For polished release testing, prefer `nla-required`; use `tls-only` only when a client lacks NLA support, and treat `rdp-only` as isolated compatibility-test mode; persisted settings are also normalized on save/load so stale max-backoff values cannot remain below initial backoff; if the service is invoked without credentials, it enters foreground mode only long enough to satisfy Android foreground-service startup rules, stops any existing capture/test-pattern source and native listener, resets persisted last mode to `none`, removes the temporary notification, and stops without opening a listener. Saved credentials are persisted encrypted-at-rest using an Android Keystore AES/GCM key (`RdpCredentialStore`), with one-time migration from legacy plaintext prefs when present. Long-running server starts now always enter foreground-service mode (test pattern and MediaProjection), expose a notification Stop action, and also route the main UI Stop button through the same explicit service action for deterministic cleanup. Screen-capture permission denial now preserves the previous last successful mode, avoids persisting a new capture scale or failed capture mode, clears pending in-memory credentials from the permission round-trip, and shows a short user-facing message. Repeated service start commands are serialized through one lifecycle lock: the old capture/test-pattern source and native server are stopped before a new mode is started. Explicit Stop resets the persisted last mode to `none`; MediaProjection revocation does the same, removes the foreground notification, and stops the service. The service is intentionally `START_NOT_STICKY`: after process death or projection revocation it requires an explicit user/UI restart rather than silently resuming an RDP listener without fresh state. The main UI checks Android secure settings for the app AccessibilityService and shows an explicit enablement hint when remote input is disabled. The foreground service registers a default-network callback, logs local IPv4 changes, and refreshes the notification with current local IPv4 addresses on network changes; because the Go listener binds all interfaces, this gives testers a visible recovery trail for Wi-Fi reconnect/IP-change/hotspot/VPN scenarios without silently restarting the server. The main UI also surfaces a compact health line from `NativeRdpBridge` (backend, running state, mode/capture source, selected security policy, credential/auth presence, listen address, TLS fingerprint prefix, active/accepted client counts, auth/handshake failures, Accessibility input enabled/disabled, decoded input events, RDPEI contacts, DVC fragments, submitted/sent frames, bitmap bytes, queue depth, dropped frames, input scale) plus a bounded selectable debug panel for current-session health/settings, making service/input/capture state visible during testing. The full TLS SHA-256 fingerprint is available in the debug panel and through a **Copy TLS Fingerprint** button while the native server is running, so users can compare it with client certificate warnings. Allowed-user and CIDR allowlists are supported in the Go server core and desktop/mock-server flags, but are intentionally not exposed in the Android UI for this first polished APK; keep the listener on a trusted LAN/VPN and use security mode plus strong credentials until an Android allowlist editor exists. A bounded Share Diagnostics action exports that same redacted health state plus non-secret settings (bounded configured username, password-present flag, capture scale, security mode, failed-auth policy, TLS fingerprint, last mode, Accessibility state) through Android's standard share sheet; it does not include the password or raw frame data. Recent bridge hardening keeps this health state consistent across start/stop races and catches reflected gomobile input-callback failures so diagnostics do not disrupt input delivery. CI asserts that `startServer` and `frame#1` appear in logcat. In Go-backed emulator runs, CI also forwards TCP/3390 from the runner to the emulator, connects with `cmd/probe` using matching credentials, and writes `rdp-screenshot.png` from RDP bitmap updates. With `emulator_capture=true`, CI requests MediaProjection, accepts the emulator consent dialog, uses MediaProjection frames instead of synthetic frames, navigates through home, Settings, and browser intents, and writes paired Android/RDP screenshots for each scene.

`RdpAccessibilityService` includes matching callback landing points. Primary pointer down/move/up is now coalesced into a bounded Accessibility `GestureDescription` path, so simple taps still dispatch on button-up and primary-button drags can become strokes. RDPEI touch frame callbacks are batched into coordinated Accessibility gesture dispatches where possible, and stale in-progress touch frames are cleared if Accessibility is disconnected at frame start or disappears before frame end. Pointer wheel events are decoded and surfaced through the bridge, then logged/degraded safely because `AccessibilityService` has no reliable generic wheel injection primitive. Key and Unicode callbacks are still limited to current landing behavior until richer Accessibility injection is implemented.

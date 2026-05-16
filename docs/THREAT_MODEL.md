# Threat model

Last updated: 2026-05-11

This document captures the current production-readiness threat model for `go-rdp-android`, a native Android RDP server that exposes the device screen and Android input surfaces to LAN RDP clients.

## Scope

In scope:

- The Android app, foreground service, gomobile bridge, and embedded Go RDP server.
- RDP listener exposure on local networks, hotspot networks, VPN interfaces, and developer test networks.
- TLS Client Info authentication and Hybrid/NLA CredSSP authentication.
- Android `MediaProjection` capture and `AccessibilityService` input injection.
- Local credential and TLS certificate storage.
- CI/mock-server validation paths that exercise protocol and auth behavior.

Out of scope for the current prototype:

- Internet-facing operation without an external VPN/firewall.
- Multi-tenant Android devices with hostile local apps.
- Strong DRM/content-protection bypass resistance beyond what Android platform APIs enforce.
- Enterprise MDM policy integration.

## Assets

| Asset | Why it matters | Current protection |
| --- | --- | --- |
| Device screen contents | RDP clients can view sensitive on-screen data. | `MediaProjection` user consent and foreground service visibility. |
| Android input capability | Accessibility gestures can operate apps on behalf of the user. | Explicit AccessibilityService enablement, app-side input sink separation, bounded gesture handling. |
| RDP credentials | Password compromise allows remote sessions. | Explicit first-run setup, Android Keystore-backed encrypted-at-rest storage, bcrypt support for TLS Client Info server auth. |
| NLA plaintext-equivalent secret | NTLM verification needs plaintext-equivalent credentials. | Documented limitation; avoid hash-only expectation for NLA. |
| TLS private key | Stable cert trust depends on key secrecy. | Optional persisted key path; fingerprint exposure for trust checks; rotation support. |
| Server availability | LAN clients and local service stability depend on bounded resource use. | Size bounds, fragment cleanup, failed-auth backoff, graceful disconnect handling, CI soak path. |
| Logs/artifacts | Logs can leak users, domains, paths, screenshots, or protocol details. | Bounded username/domain sanitization, no session-key logging, artifact permission tightening. |

## Actors and trust boundaries

| Actor | Capability | Trust level |
| --- | --- | --- |
| Device owner | Installs app, grants projection/accessibility, configures credentials. | Trusted admin. |
| Authenticated RDP client | Can view screen and inject permitted input for the session. | Trusted for the duration of a session. |
| LAN peer | Can scan/connect to the RDP port and attempt authentication. | Untrusted. |
| Malicious Wi-Fi/hotspot peer | Can probe, replay connection attempts, and attempt downgrade or brute force. | Untrusted. |
| Local Android app | May observe network state or attempt UI/social-engineering attacks. | Mostly untrusted; platform sandbox assumed. |
| CI/test client | Exercises protocol in controlled workflows. | Trusted but artifacts must still avoid secrets. |

Primary trust boundaries:

1. Network boundary: TCP listener accepts untrusted input before authentication.
2. Auth boundary: RDP/TLS/NLA negotiation and credential validation gate session activation.
3. Android permission boundary: `MediaProjection` and Accessibility require explicit platform-level user consent.
4. Storage boundary: credentials/certificates cross from user setup into persistent local storage.
5. Log/artifact boundary: debug output leaves the runtime and may be uploaded to CI artifacts.

## Major threats and mitigations

### LAN exposure and unauthenticated probing

Threats:

- Port scanning discovers a listening RDP service.
- Attackers attempt protocol fuzzing, oversized packets, abandoned fragments, or brute-force credentials.
- Users accidentally expose the listener on hotspot/VPN/untrusted networks.

Current mitigations:

- Explicit credential setup is required before Android service start.
- `AccessPolicy.SecurityMode` can require TLS-only or NLA-required operation.
- CIDR allowlists can restrict accepted client networks in server-core/mock-server deployments; Android UI intentionally defers allowlist editing for the first polished APK.
- Failed-auth lockout/backoff can slow brute-force attempts.
- Static and dynamic virtual-channel payloads, RDPEI payloads, and fragment buffers have bounded sizes and cleanup.
- FreeRDP compatibility gates plus parser/fuzz smoke cover active protocol paths.

Required before public production use:

- Android UI controls for network/user allowlists after the first polished APK, or another explicit release decision to keep allowlists server-core-only.
- Clear in-app warning when listening on hotspot/VPN/mobile interfaces.
- User-visible connected-client count and stop-server action.

### Authentication and downgrade risk

Threats:

- User enables weaker RDP security than intended.
- TLS certificate changes are accepted blindly by clients.
- NLA behavior is misunderstood as compatible with hash-only storage.

Current mitigations:

- Security modes are explicit: `negotiate`, `rdp-only`, `tls-only`, `nla-required`.
- TLS certificate fingerprint is exposed/logged (`tls_fp=...`) and cert persistence/rotation are configurable.
- TLS Client Info can use bcrypt server-side password hashes.
- NLA/CredSSP uses NTLMv2 and validates TLS public-key binding; its plaintext-equivalent credential requirement is documented.

Required before public production use:

- Keep the release recommendation explicit: prefer `nla-required`, use `tls-only` only for clients without NLA, and reserve `rdp-only` for isolated compatibility testing.
- Documentation must warn that changing/rotating certs requires client trust revalidation.

### MediaProjection capture

Threats:

- Screen contents are captured while the user does not realize the server is active.
- Projection is revoked mid-session and stale frames continue or service state becomes misleading.
- Sensitive system dialogs or third-party app content appears to remote clients.

Current mitigations:

- Capture depends on Android `MediaProjection` consent and a foreground service.
- CI validates service startup, capture test-pattern mode, and emulator capture flows.
- Capture/service code has bounded frame queue/drop behavior with submitted/queued/dropped counters, active/accepted connection, auth/handshake failure, decoded input event, RDPEI contact, DVC fragment, sent frame, and bitmap byte counters, serialized mode switching, projection-revocation shutdown, foreground notification stop action, missing-credential source/listener cleanup, permission-denial cleanup, network-change address refresh, non-sticky restart behavior, non-secret settings persistence, and compact UI health state.

Required before public production use:

- Stronger permission/onboarding copy explaining exactly what is captured.
- Fuller UI health state for projection active/revoked/error beyond current mode/input/client-count indicators.
- Recovery flow when projection is revoked beyond stopping the service and requiring explicit restart.
- Screen-off/lock/Doze behavior validation on physical devices.

### Accessibility input injection

Threats:

- Authenticated clients operate apps, send messages, or approve prompts through Accessibility gestures.
- Multi-touch or continuation handling misfires and performs unexpected input.
- Accessibility remains enabled after the user thinks remote control is off.

Current mitigations:

- Touch path is separated from pointer/mouse sinks and preserves contact lifecycle state.
- Gesture construction is bounded and cancellation-aware; fallback behavior degrades safely when multi-stroke injection is rejected.
- Emulator synthetic RDPEI tap validation exists.

Required before public production use:

- Physical-device validation for taps, drags, multi-touch, screen off/on, and failure callbacks.
- Clear UI indication when input injection is available/enabled.
- One-tap disable/stop path for remote control.

### Local storage and secrets

Threats:

- Stored credentials or TLS private key are exfiltrated from device storage or backups.
- Debug logs expose usernames, passwords, session keys, cert keys, or screenshots.

Current mitigations:

- Android credentials are persisted encrypted-at-rest via Android Keystore-backed AES/GCM, with legacy plaintext migration.
- Logs bound/sanitize user/domain fields and avoid secrets/session keys.
- TLS private key persistence is explicit and fingerprint-based trust checks are documented.

Required before public production use:

- Review backup/export behavior for Android credential files and TLS key files.
- Add user-triggered credential reset and TLS cert rotation UX.
- Keep CI artifacts redacted and avoid uploading sensitive screenshots outside controlled test patterns.

### Denial of service and stability

Threats:

- Malformed RDP/DVC/RDPEI packets cause panics or unbounded memory growth.
- Abandoned sessions leak sockets, goroutines, capture surfaces, or frame buffers.
- Repeated auth failures consume CPU.

Current mitigations:

- Parser tests, fuzz seeds, bounded payload/fragment sizes, idle fragment cleanup, graceful disconnect handling.
- Failed-auth backoff for configured policies.
- Nightly/dispatch FreeRDP soak workflow checks repeated sessions and RSS growth.

Required before public production use:

- Broader long-running physical-device soak with real capture and input.
- Runtime counters for connections, auth failures, frames, bitmap bytes, input events, RDPEI contacts, and DVC fragments.
- Diagnostic bundle with redaction.

## Production security defaults

Recommended production posture:

1. Require explicit credentials before service start.
2. Prefer `nla-required`; otherwise use `tls-only` with strong password policy.
3. Disable plain `rdp-only` outside controlled lab testing.
4. Enable failed-auth lockout/backoff.
5. Restrict clients with CIDR allowlists where possible.
6. Persist TLS certs so clients can pin/recheck fingerprints; rotate only intentionally.
7. Show foreground notification and in-app health while listening/capturing/injecting input.
8. Provide clear stop-server and disable-input controls.

## Open security work

- Surface security mode, allowlist, failed-auth policy, TLS fingerprint, and rotation controls in Android UI.
- Validate Microsoft Remote Desktop clients with NLA to active streaming.
- Validate physical-device MediaProjection and Accessibility behavior, including screen lock/off and Doze.
- Add runtime counters and a redacted diagnostic bundle.
- Review Android backup behavior for credential and cert/key persistence.
- Revisit `G115` gosec exclusion with targeted cast-safety remediation.

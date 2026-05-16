# Privacy and security notes

`go-rdp-android` is a prototype RDP server for Android. It can expose the Android screen and selected remote input controls to any authenticated RDP client that can reach the listening TCP port.

## What is captured

- Screen capture uses Android MediaProjection after explicit user consent.
- When screen capture mode is active, the current Android display is copied into the native bridge and encoded as RDP bitmap updates.
- Test-pattern mode sends synthetic frames only and does not capture the screen.
- The current implementation does not capture microphone, camera, location, contacts, files, or notifications directly, but anything visible on screen can be transmitted while MediaProjection is active.

## When the server listens

- The server listens only after credentials are configured and the user starts test-pattern or screen-capture mode.
- Long-running modes run as an Android foreground service with a persistent notification and Stop action.
- Explicit Stop, MediaProjection revocation, missing credentials, native startup failure, or invalid policy configuration stop capture/test-pattern sources and leave the server stopped.
- The service is `START_NOT_STICKY`: after process death it requires an explicit user/UI restart rather than silently reopening a listener.

## Remote input

- Remote pointer, keyboard, Unicode, and RDPEI touch events are decoded by the Go server and forwarded to Android Accessibility landing points.
- Accessibility must be enabled by the user before remote input can affect the device.
- Pointer/touch support is intentionally conservative and degrades when Android does not expose a safe injection primitive.

## Credentials and policy

- The Android app refuses to start the server until a username and password are configured.
- Credentials are stored through Android Keystore-backed encrypted preferences.
- Non-secret settings such as capture scale, selected security mode, failed-auth backoff policy, and last mode are stored separately.
- Supported security-policy modes are `negotiate`, `rdp-only`, `tls-only`, and `nla-required`.
- Failed-auth backoff/lockout can be configured in the UI; max backoff is normalized so it is never lower than the initial backoff.

## Diagnostics

- The in-app debug panel and Share Diagnostics action are bounded and redacted.
- Diagnostics include health counters, selected security mode, failed-auth policy, capture scale, bounded username, password-present status, and the TLS fingerprint when the native server is running.
- Diagnostics do not include the password or raw frame data.

## Recommended defaults before broader use

- Release recommendation: use `nla-required` whenever the client supports it; use `tls-only` only for clients without NLA.
- Avoid `rdp-only` except for isolated compatibility testing because it does not provide the TLS/NLA protections expected on shared networks.
- Use a strong unique password.
- Keep the foreground notification visible while testing and stop the service when done.
- Do not expose the listening port beyond a trusted LAN/VPN. Android allowlist editing is deferred for the first polished APK even though the Go server core supports allowlists.
- Verify the TLS fingerprint from the app/logs when connecting from clients that show certificate warnings.

# Trace phase taxonomy

`GO_RDP_ANDROID_TRACE=1` enables bounded server trace lines in the form:

```text
trace phase=<phase> key=value ...
```

Trace lines are intended for CI artifacts and local debugging. They must not log credentials, session keys, raw pixel buffers, or unbounded client-controlled strings. User/domain/cookie-like fields must be sanitized with the same bounded log helpers used by normal logs.

## Phase families

| Family | Current phases | Purpose |
| --- | --- | --- |
| TCP / TPKT / X.224 negotiation | `tpkt_read`, `tpkt_write`, `x224_confirm`, `fastpath_ignore` | Packet envelope visibility, security protocol selection, and unexpected early Fast-Path bytes. |
| Auth / CredSSP / Client Info | `credssp_pubkeyauth_mismatch`, `client_info_parse`, `client_info` | NLA public-key binding diagnostics and bounded Client Info parse/auth context. |
| MCS / GCC / channel lifecycle | `mcs_domain_pdu`, `mcs_domain_disconnect`, `mcs_attach_user_confirm`, `mcs_channel_join_confirm`, `share_control_disconnect` | MCS domain sequencing, graceful disconnect/logoff, attach/join progress. |
| Activation / share data | `confirm_active`, `share_data`, `share_data_write`, `frame_stream_stop` | Confirm Active capability summary, Share Data reads/writes, and streaming shutdown reason. |
| Licensing | normal logs plus `share_data`/`share_data_write` around the license-valid path | Licensing is intentionally minimal today; add explicit `licensing_*` phases when more licensing state is implemented. |
| Graphics | `bitmap_tile`, `bitmap_tile_skip`, `frame_resize` | Bitmap tile emission, dirty-tile suppression, and desktop-size scaling. |
| Classic input | `fastpath_input`, `fastpath_input_skip` | Fast-Path input decode counts and skipped/unsupported payload metadata. |
| Dynamic virtual channels | `drdynvc_pdu`, `drdynvc_caps`, `drdynvc_create`, `drdynvc_data_first`, `drdynvc_close`, `drdynvc_fragment_expired` | Static `drdynvc` routing, DVC negotiation/lifecycle, fragmentation, cleanup. |
| RDPEI | `rdpei_cs_ready`, `rdpei_touch`, `rdpei_dismiss_hovering` | True RDP touch negotiation and bounded touch-frame/contact metadata. |
| Android bridge | Android logcat tags `GoRdpAndroid`, `GoRdpAndroidService`, `GoRdpAndroidCapture`, `GoRdpAndroidGo` | Bridge/service/capture health. The health string exposes backend, mode, auth, address/fingerprint, connection/auth/input/RDPEI/DVC/frame/bitmap counters, queue depth/drop count, and input scale. |

## Adding new phases

- Prefer `snake_case` phase names.
- Keep phase names stable once CI or docs reference them.
- Use short bounded metadata only: counts, dimensions, flags, IDs, lengths, booleans, sanitized usernames/domains.
- Do not emit raw packet bytes in server trace logs; use probe `trace-dir` hex artifacts for packet dumps.
- Add docs here when introducing a new phase family or a phase that CI/artifact summaries depend on.

## Diagnostic bundle sources

Until an in-app diagnostic export exists, collect the same components manually:

- mock/probe logs and summaries from CI artifacts.
- `freerdp-compat-probe` summaries, screenshots, and best-attempt logs.
- Android logcat lines for the `GoRdpAndroid*` tags.
- `NativeRdpBridge.healthStatus()` output from the UI.
- AAR/API and APK artifact summaries when investigating packaging issues.

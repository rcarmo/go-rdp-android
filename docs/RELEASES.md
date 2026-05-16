# Release and tag policy

The repository uses tag suffixes to select CI/CD depth.

## Tag classes

| Tag pattern | Purpose | Expected CI behavior |
| --- | --- | --- |
| `*-ux` | Full UX validation | Runs the Go-backed Android emulator MediaProjection flow, scripted input validation, RDP screenshot capture, Gherkin validation, and Playwright PDF report generation. |
| `*-build` | Build validation/artifact production | Runs normal build/test jobs and uploads build artifacts/APKs/AARs. Emulator UX is not run by default. |
| `vX.X.X` | Release APK/AAB | First prunes pre-existing GitHub Actions artifacts from prior inter-release/non-release runs, then runs normal build/test jobs, Go-backed emulator UX validation, Playwright PDF report generation, and final release-file staging. Release files include `go-rdp-android-vX.X.X.apk`, `go-rdp-android-vX.X.X.aab`, `go-rdp-android-vX.X.X-ux-report.pdf`, `go-rdp-android-vX.X.X-release-notes.md`, `go-rdp-android-vX.X.X-apk-signature.txt`, `go-rdp-android-vX.X.X-aab-signature.txt`, `go-rdp-android-vX.X.X-sbom-go.cdx.json`, and `go-rdp-android-vX.X.X-sha256.txt`. Version should match `VERSION`, Android `versionName`, and package metadata. |

Examples:

```sh
git tag 0.1.1-ux
git push origin 0.1.1-ux

git tag 0.1.1-build
git push origin 0.1.1-build

git tag v0.1.1
git push origin v0.1.1
```

## Release artifact cleanup

Release tags (`vX.X.X`) run a `release-cleanup` gate before artifact-producing jobs. It deletes previous GitHub Actions artifacts for the repository so release APKs, AARs, logs, screenshots, and the UX PDF are not mixed with stale artifacts from `main`, `*-build`, `*-ux`, or manual workflow runs.

This cleanup only runs for release tags. Non-release CI keeps its normal artifacts for debugging.

## Release files and notes

Release tags (`vX.X.X`) produce a consolidated `go-rdp-android-release-files` artifact (retention: 180 days) containing:

- `go-rdp-android-vX.X.X.apk`
- `go-rdp-android-vX.X.X.aab`
- `go-rdp-android-vX.X.X-ux-report.pdf`
- `go-rdp-android-vX.X.X-release-notes.md`
- `go-rdp-android-vX.X.X-apk-signature.txt` (from `apksigner verify --print-certs`)
- `go-rdp-android-vX.X.X-aab-signature.txt` (from `jarsigner -verify -certs`)
- `go-rdp-android-vX.X.X-sbom-go.cdx.json` (CycloneDX Go module SBOM)
- `go-rdp-android-vX.X.X-sha256.txt` (checksums for release assets)

Release notes are generated from Git history and list commits since the previous `v*` release tag, with GitHub commit links. For the first release, notes list commits reachable from the release tag.

Release-tag staging requires production signing secrets in GitHub Actions (never in-repo):

- `RELEASE_KEYSTORE_BASE64` (base64-encoded JKS/PKCS12 keystore)
- `RELEASE_KEYSTORE_PASSWORD`
- `RELEASE_KEY_ALIAS`
- `RELEASE_KEY_PASSWORD`

If these are missing, the `release-files` job fails before publishing artifacts. As of 2026-05-16, `gh secret list --repo rcarmo/go-rdp-android` from the automation token returned no visible repository secrets, so a controlled `v*` release-candidate/dry-run tag remains blocked until the repository owner confirms those four signing secrets are present and correct.

## Current identifiers

- SemVer: `0.1.1`
- `VERSION`: `0.1.1`
- Android `versionName`: `0.1.1`
- Android package/application ID: `io.carmo.go.rdp.android`
- Android versionCode: `2`

Checked on 2026-05-16: `VERSION`, Android `versionName`, README package/version notes, and this release policy are aligned for `0.1.1` / `versionCode=2`.

## Notes

Android package IDs cannot contain hyphens. The requested project namespace `io.carmo.go-rdp-android` is represented as the valid Android ID `io.carmo.go.rdp.android`.

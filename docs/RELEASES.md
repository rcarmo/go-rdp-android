# Release and tag policy

The repository uses tag suffixes to select CI/CD depth.

## Tag classes

| Tag pattern | Purpose | Expected CI behavior |
| --- | --- | --- |
| `*-ux` | Full UX validation | Runs the Go-backed Android emulator MediaProjection flow, scripted input validation, RDP screenshot capture, Gherkin validation, and Playwright PDF report generation. |
| `*-build` | Build validation/artifact production | Runs normal build/test jobs and uploads build artifacts/APKs/AARs. Emulator UX is not run by default. |
| `vX.X.X` | Release APK | First prunes pre-existing GitHub Actions artifacts from prior inter-release/non-release runs, then runs normal build/test jobs, Go-backed emulator UX validation, Playwright PDF report generation, and final release-file staging. Release files include `go-rdp-android-vX.X.X.apk`, `go-rdp-android-vX.X.X-ux-report.pdf`, and `go-rdp-android-vX.X.X-release-notes.md`. Version should match `VERSION`, Android `versionName`, and package metadata. |

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

Release tags (`vX.X.X`) produce a consolidated `go-rdp-android-release-files` artifact containing:

- `go-rdp-android-vX.X.X.apk`
- `go-rdp-android-vX.X.X-ux-report.pdf`
- `go-rdp-android-vX.X.X-release-notes.md`

Release notes are generated from Git history and list commits since the previous `v*` release tag, with GitHub commit links. For the first release, notes list commits reachable from the release tag.

## Current identifiers

- SemVer: `0.1.1`
- Android package/application ID: `io.carmo.go.rdp.android`
- Android versionCode: `2`

## Notes

Android package IDs cannot contain hyphens. The requested project namespace `io.carmo.go-rdp-android` is represented as the valid Android ID `io.carmo.go.rdp.android`.

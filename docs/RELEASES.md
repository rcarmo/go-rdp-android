# Release and tag policy

The repository uses tag suffixes to select CI/CD depth.

## Tag classes

| Tag pattern | Purpose | Expected CI behavior |
| --- | --- | --- |
| `*-ux` | Full UX validation | Runs the Go-backed Android emulator MediaProjection flow, scripted input validation, RDP screenshot capture, Gherkin validation, and Playwright PDF report generation. |
| `*-build` | Build validation/artifact production | Runs normal build/test jobs and uploads build artifacts/APKs/AARs. Emulator UX is not run by default. |
| `vX.X.X` | Release APK | Runs normal build/test jobs and should be used for release APK artifact publication. Version should match `VERSION`, Android `versionName`, and package metadata. |

Examples:

```sh
git tag 0.1.0-ux
git push origin 0.1.0-ux

git tag 0.1.0-build
git push origin 0.1.0-build

git tag v0.1.0
git push origin v0.1.0
```

## Current identifiers

- SemVer: `0.1.0`
- Android package/application ID: `io.carmo.go.rdp.android`
- Android versionCode: `1`

## Notes

Android package IDs cannot contain hyphens. The requested project namespace `io.carmo.go-rdp-android` is represented as the valid Android ID `io.carmo.go.rdp.android`.

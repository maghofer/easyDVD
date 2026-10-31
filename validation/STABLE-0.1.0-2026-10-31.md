# easyDVD 0.1.0 release validation

Prepared: 2026-10-31, Windows 11 x64. The maintainer requested publication as
the first stable release, with the installer, portable ZIP, matching sources and
SHA-256 files. `VERSION` is `0.1.0`; the release tag is `v0.1.0`.

## Completed for this release

- Reviewed and integrated the pending installer, HCEnc pipeline, source-built
  support runtime, first-launch setup, repair action, profile labels and tools icon.
- All Go tests passed, including the CI build-tag configuration. `go vet` passed
  for both configurations. PowerShell scripts parsed successfully; source and
  whitespace audits passed.
- The hosted Windows build encountered an unavailable FFmpeg source server.
  CI now preseeds the original archive from Buildroot's source mirror and checks
  it against the unchanged upstream SHA-256 pin before the builder uses it.
  The mirrored archive was independently downloaded and matched the existing pin;
  compiler inputs, support binaries and encoding behavior are unchanged.
- The real Windows integration suite passed in 33.291 seconds, including
  Balanced and Quality PAL/NTSC encoding, actual DVD authoring, menus, geometry,
  field handling, audio and recovery checks. Tests used isolated queues and work.
- Public preflight packaging verified all authoring/support binary-to-source
  records and produced the 0.1.0 installer, runtime ZIP and sources ZIP.
- A fresh installer lifecycle run upgraded rc.3 to 0.1.0 using the separate
  installer-test identity and a path with spaces and an accented character.
  Payload hashes, installed CLI versions, shortcuts and registration passed.
- The installed CLI downloaded and verified its private HCEnc copy, checked the
  installed tools, and encoded/authored an actual Quality DVD ISO. The installed
  GUI opened using an isolated application profile.
- Upgrade and uninstall correctly refused to alter a running application.
  Upgrade reused the previous install directory. Same-version reinstall passed.
  Upgrade and uninstall preserved queue/settings, source, output, retained work,
  downloaded encoder and user-added files. Uninstall removed installed payload,
  shortcuts and registration.
- Prior real-media decoding and source-built DLL loader checks are documented in
  [the packaging record](PUBLIC-PACKAGE-2026-10-31.md). Direct publisher downloads
  and automatic setup are documented in
  [the first-launch record](FIRST-LAUNCH-HCENC-2026-10-31.md).

Final distributables are built from the committed release source. Their embedded
`release.json` identifies that commit; each download has a SHA-256 sidecar. GitHub
checks and the published release are the upstream record of final publication.

## Remaining coverage and limitations

Clean Windows 10/11 VM installation, interrupted-install recovery,
multi-account/session scenarios and physical DVD-player/full-capacity media
coverage have not been established by these local checks. Sandbox preparation
does not count as a completed clean-machine test. These limitations are disclosed
in the release notes rather than represented as completed validation.

The installer is unsigned. FFmpeg/FFprobe remain separately selected/downloaded;
HCEnc is downloaded from its author and is excluded from published packages.
The application-update checker remains manual; automatic startup application
updates are outside this release.

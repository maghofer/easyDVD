# Release-candidate packaging validation

Completed: 2026-10-31, Windows 11 x64. Candidate: `0.1.0-rc.1`.
No public release, tag or upload was created. This record supersedes the
pending Quality bundling status in the earlier October 30 validation notes.

## Distribution changes

- The installed and portable payloads include source-built x86 AviSynth+ 3.7.6
  and FFMS2 5.0. FFMS2 statically includes FFmpeg 6.1.3 in an LGPL 2.1-or-later
  configuration and zlib 1.3.2. Its private FFmpeg enables FFV1, Matroska and
  local files for easyDVD's prepared input. It does not decode original media;
  original-media preparation still uses the external FFmpeg/FFprobe pair.
- AviSynth optional plugins and CUDA are disabled. DevIL is excluded. The
  support DLLs import only Windows system DLLs; Microsoft CRT is linked statically.
  There is no codec registration, service, PATH change or external runtime DLL
  installation. Existing legacy custom runtimes remain supported.
- Both support DLLs were built through the checked-in recipe using MSVC 19.44
  (Visual Studio 2022) and Windows SDK 10.0.26100.0. A fresh scripted build from
  the four pinned original source archives completed successfully. Binary hashes
  map to exact source archives, notices/linking exceptions and build instructions.
  Public packaging verifies those records. No bit-identical reproduction across
  compiler versions or source directories is claimed.
- The original HCEnc 0.28 publisher archive was inspected. Its encoder SHA-256
  matches the old AVStoDVD supplier copy exactly. A general redistribution grant
  was not established. HCEnc and the supplier binary archives are therefore
  excluded from both runtime and source packages; their old permission/source
  gaps have not been relabelled as cleared.
- Quality setup and CLI `-download-hcenc` explicitly obtain the pinned upstream
  AVStoDVD archive from SourceForge over HTTPS. Both archive and extracted
  encoder hashes are verified. Only HCEnc and its original notices are retained
  in a private user cache; AVStoDVD and its other components are not installed.
  The publisher's direct HTTP endpoint was unsuitable for the app downloader.
- A verified cache is reused. Corrupt versions are not executed or overwritten;
  a replacement is installed separately. Downloads are bounded, cancellable and
  staged before publication. No download starts merely from opening the app.
- The existing source-built authoring-helper records remain in force. Normal
  public packaging now passes. The separate full offline FFmpeg distribution
  gate remains; the normal installer obtains FFmpeg through existing setup.

## Completed checks

- All Go unit tests and `go vet ./...` passed. Download regression tests cover
  archive/executable checksum rejection, cache reuse, corruption recovery,
  cancellation, size limits and staging cleanup.
- The complete Windows integration suite passed, including Balanced and Quality
  PAL/NTSC pipelines and existing geometry, field-order, audio, menu, authoring
  and recovery coverage. The test runtime used isolated copies of the new DLLs
  plus a separately obtained HCEnc; the user's GUI queue was not used.
- The real 94.68-second 1080p25 ProRes source completed through the packaged
  app with DLLs matching the shipped manifest and an explicit isolated CLI state.
  The 92,129,280-byte ISO contained progressive MPEG-2 720x576/25 fps at 16:9
  and stereo AC-3 48 kHz/448 kbps. Its authored title fully decoded with FFmpeg
  `-xerror -err_detect explode` without errors. Share media was read-only.
- A fresh 32-bit loader probe restricted DLL lookup to the selected package
  directory and Windows System32. Both shipped DLLs loaded from the package;
  no DevIL or supplier runtime was required.
- The actual HTTPS download and native Windows tar extraction succeeded from
  the application CLI. The installed copy repeated this under a scratch profile.
- Native desktop inspection confirmed the new Quality setup entry and dialog,
  download source/size explanation and explicit Download action. The test app
  used an isolated Windows profile; no real user preferences were changed.
- Installer lifecycle tests used a separate test application identity and a
  directory containing spaces and an accented character. Installation, shortcut
  targets, HKCU registration, payload hashes, installed GUI startup and a Quality
  DVD authoring job passed. Upgrade from preview.9 to preview.10 test binaries
  reused the previous directory; same-version reinstall succeeded.
- Setup and Uninstall refused to proceed while the test app ran. Two preliminary
  test starts also correctly refused while other validation app instances were
  active. The complete lifecycle test passed when run without unrelated instances.
- Upgrade and uninstall preserved hashes of the real test queue/settings, source,
  retained work, finished ISO, downloaded HCEnc and a user-added file. Uninstall
  removed its program/runtime files, shortcuts and test registration.
- `package.ps1 -PublicRelease -Installer` produced setup, portable runtime and
  matching sources packages. Inspection confirmed that the runtime support
  directory contains only the two new DLLs and that the Quality source archive
  set contains only the four reviewed source archives, not HCEnc/AVStoDVD/DevIL.
- PowerShell parsing, source audit and whitespace checks passed. Detailed logs,
  supplier research, fixtures and test outputs remain in designated scratch space.

## Remaining stable-release validation

This is an unsigned release candidate. Clean Windows 10/11 VM installation,
interrupted-install recovery and multi-account/session behavior still need
coverage. Source-record checks do not certify every legal question or physical
DVD-player compatibility. HCEnc remains an external dependency, not a bundled
component with newly asserted permission. Startup application-update download,
verification and restart are separate work; the manual release checker remains.

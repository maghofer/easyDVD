# Windows installer validation

Date: 2026-10-30. Local Windows 11 x64 workstation, standard current-user
installation privileges. This is a local preview candidate, not a stable or
public release. Application VERSION remains `0.1.0-preview.9`.

## Implementation

- Inno Setup 7.1.1 compiler, pinned by SHA-256. Its downloaded installer had a
  valid Authenticode signature from Pyrsys B.V. The compiler is a build tool;
  it is not installed by easyDVD Setup.
- Current-user installation, fixed easyDVD identity, default directory under
  LocalAppData/Programs, Start menu shortcut and optional desktop shortcut.
- Complete existing runtime payload, notices and provenance metadata, plus
  Windows uninstall registration. No machine-wide codec or PATH changes.
- GUI, CLI and benchmark hold a Windows activity marker for their process
  lifetime. Setup and Uninstall refuse to proceed while that marker exists.
- Package manifest validation precedes installer compilation. Public installers
  require a runtime package built through the existing public-release gates.

## Completed checks

- `go test ./...` passed during packaging, including the new Windows activity
  marker lifecycle test. That test verifies that closing one handle does not
  remove another instance's marker, and that the last close removes it.
- `go vet ./...`, PowerShell parsing, source audit and `git diff --check` passed.
- Normal `package.ps1 -Installer` produced runtime ZIP, sources/provenance ZIP,
  setup executable and setup SHA-256 file.
- `prepare-installer.ps1` verified and reused the pinned compiler. Version is
  queried through ISCC's `--version`; its PE product version is not meaningful.
- A separate installer test identity installed the actual application into a
  scratch path containing spaces and an accented character. The log recorded
  user privileges `None`, administrative install mode `No`, and HKCU registration.
- Every installed payload file matched the runtime package's SHA-256 manifest.
- Start menu and selected desktop shortcuts were created; the Start menu shortcut
  resolved to the installed GUI. Registration and CLI reported the expected
  initial version.
- The installed CLI found the bundled authoring and Quality tools in its own
  installation directory. FFmpeg was supplied explicitly for the test.
- The installed CLI encoded a synthetic one-second clip and authored a DVD ISO
  using explicit isolated queue, work and output paths.
- The installed GUI opened with a separate process-local profile. The existing
  user GUI and its saved queue were left running and unchanged.
- While that test GUI ran, upgrade and uninstall both exited unsuccessfully;
  all initial payload hashes remained intact.
- After normal GUI shutdown, an installer with `0.1.0-preview.10` test binaries
  upgraded the existing test location without a `/DIR` argument. Registration,
  CLI version and all payload hashes matched the upgrade fixture. This version
  was solely a test fixture; the repository release version was not changed.
- Re-running the same upgrade installer succeeded as a same-version reinstall.
- Uninstall removed installed program/runtime files, shortcuts and registration.
  Hashes of the real saved queue/settings, test source, retained work, finished
  ISO and a user-added file were unchanged after both upgrade and uninstall.
- Negative build checks rejected a local-only package requested as public, an
  unlisted payload file, a modified payload file and output inside the payload.
  Modified scratch files were restored after each negative check.
- Installer wizard welcome, license and destination pages were inspected with
  the computer-use skill. No production easyDVD installation was performed.

Reusable procedure: [Windows installer](../docs/INSTALLER.md), with the actual
Setup/Uninstall test harness in `scripts/test-installer.ps1`. Detailed logs and
fixtures were retained in the workstation's designated scratch area.

## Limits and remaining release work

The local setup executable is unsigned. Code signing, clean Windows 10/11 VM
coverage, interrupted-install/recovery behavior and multi-account/session checks
remain before stable distribution. Setup does not implement transactional
rollback or prohibit a user from manually installing an older version.

Startup update checks, trusted update downloads and automatic restart are not
implemented by this installer change. The existing manual release checker remains.
The Quality runtime's public redistribution review is still incomplete, and
public packaging continues to refuse that bundle. No public release was created.

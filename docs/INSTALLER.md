# Windows installer

The installer is an alternative to the portable ZIP. It contains the same GUI,
CLI, benchmark, DVD helpers, Quality support libraries, the private experimental
Blu-ray authoring runtime and notices. FFmpeg remains
an explicit first-run download or external installation. After that check/setup,
HCEnc downloads automatically from its author when missing; it is not included
in the installer. Quality setup provides cancellation and manual retry.

Setup installs for the current Windows account without requesting administrator
rights. The default directory is `%LOCALAPPDATA%\Programs\easyDVD`. It adds a
Start menu shortcut, an optional desktop shortcut, and a Windows Installed Apps
entry with an uninstaller. No PATH changes, codec registration, services or
scheduled tasks are installed.

Later installers reuse the same application identity and installation location.
Settings, queue and menu presets continue to use the existing easyDVD profile;
switching from a portable copy does not require copying that data. Close all
copies before installing. Old portable program folders can be removed manually
once the installed copy has been checked; Setup does not delete them.

The GUI, CLI and benchmark create a shared Windows activity marker. Setup and
Uninstall refuse to proceed while one is active; they never force an encoder to
close. User data, private FFmpeg downloads, sources, work and output are outside
Setup's payload and are retained on uninstall. Unrecognized files added to the
installation directory are retained too. Avoid choosing an output/work directory
inside the installation directory.

## Build

Install [Inno Setup 7](https://jrsoftware.org/isdl.php), or use the pinned compiler
bootstrap for a build account:

```powershell
$compiler = ./scripts/prepare-installer.ps1 -Destination (Join-Path $env:TEMP 'easyDVD-inno')
./scripts/package.ps1 -Installer -ISCCPath $compiler -OutputDir $packageOutput
```

The bootstrap verifies both the pinned SHA-256 and the publisher's Authenticode
signature before installing the compiler for the current account. It is only a
build tool; users do not need Inno Setup. The compiler's own current-user uninstall
entry is separate from easyDVD's.

To build from an already prepared runtime package:

```powershell
./scripts/build-installer.ps1 -BundleDirectory $runtimeBundle -OutputDir $installerOutput -ISCCPath $compiler
```

The build verifies every payload file against `SHA256SUMS.json`, rejects unlisted
files, links and version mismatches, and refuses to overwrite an existing setup
executable. Output must be outside the payload directory. It writes
`easyDVD-<version>-win64-setup.exe` and its `.sha256` file alongside the ZIPs.

Public packaging uses `-PublicRelease -Installer`. A public installer requires
a runtime package that passed the existing public-release gates; adding an
installer does not bypass the library/source checks or offline FFmpeg review.
The former Quality supplier binaries were replaced with source-built libraries;
HCEnc is an external download. The manual GitHub package workflow builds and retains
the setup executable and its hash in addition to the runtime and sources ZIPs.

## Repeat installation tests

Prepare complete runtime bundles with two different versions, then run:

```powershell
./scripts/test-installer.ps1 -InitialBundle $olderBundle -UpgradeBundle $newerBundle -ISCCPath $compiler -Scratch $testScratch -FFmpegDirectory $ffmpegDirectory
```

The script compiles installers with a separate `easyDVD.InstallerTest` identity,
uses a unique scratch directory with spaces and an accented name, and isolates
the GUI profile and CLI queue. It checks shortcuts, registration, payload hashes,
tool discovery, an actual DVD authoring job, GUI startup, running-app refusal,
upgrade location discovery, same-version reinstall and uninstall. It verifies
that the saved queue/settings, source, work, ISO and user-added files retain
their hashes. Logs and a `PASS.txt` summary remain under the scratch directory.
An existing test registration causes a refusal rather than being overwritten.

Clean Windows 10/11 VM coverage, code signing and interrupted-install/recovery
exercises are still required before calling this a stable distribution channel.

## Updates

This change supplies the installer and its silent upgrade mechanism. The update
checker opens the release page. Optional background startup checks announce a
newer stable release without blocking startup; downloading, update authenticity
verification and automatic restart remain separate application work. An updater
must finish downloading and verifying before closing easyDVD,
wait for all activity to stop, and invoke Setup with `/SILENT /NORESTART` and
appropriate error handling. The installer does not promise transactional rollback
or prevent manually installing an older version. Database migrations and recovery
must be designed alongside the future updater.

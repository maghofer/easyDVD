# easyDVD 0.1.0

The first stable release of easyDVD for Windows 10/11 x64 includes a normal
Windows installer alongside the portable ZIP.

## Installation

- **Recommended:** download `easyDVD-0.1.0-win64-setup.exe`, close any running
  easyDVD instances, and run Setup. Installation is for the current Windows
  account, with shortcuts and an uninstaller. Subsequent installers reuse the
  installation directory and preserve the queue and settings.
- **Portable:** extract the complete `easyDVD-0.1.0-win64.zip` and run
  `easyDVD.exe`. Keep the included `tools` folder beside it.
- The matching `easyDVD-0.1.0-win64-sources.zip` contains application/dependency
  sources, notices and build records; it is not needed to run easyDVD. Keep it
  available with redistributed binaries. SHA-256 files accompany all downloads.

## Changes since preview.9

- Quality encoding uses HCEnc BEST two-pass VBR with an 8.5 Mbps video target,
  9 Mbps peak and 448 kbps stereo AC-3. Balanced keeps its FFmpeg two-pass profile
  and remains the default. The interface identifies the encoder for each profile.
- Source-built AviSynth+ and FFMS2 support libraries are included. After the
  FFmpeg check/setup, HCEnc downloads automatically from its author when missing,
  with archive and executable checksums verified before installation. A verified
  private copy is reused on later launches. AVStoDVD and a system-wide AviSynth
  installation are not required.
- Settings offers a small **Repair dependencies** action at the bottom for
  failed downloads or missing tools. Dependency setup is removed from Job settings.
- Windows Setup supports install, upgrade and uninstall, and refuses replacement
  while an easyDVD process is running. User queues, settings, media, output and
  downloaded encoders are retained. No PATH changes or system codec registration
  are required.

## Setup and compatibility notes

- FFmpeg/FFprobe remain separate: first launch can locate a compatible pair or
  download the recommended private installation. Internet is needed for downloads.
- The installer is unsigned, so Windows may display a publisher warning.
- The application update check remains manual and opens the release page.
  Automatic startup application updates and installation are not included.
- Quality uses additional temporary space for its lossless intermediate input.
  Saved Quality jobs from the FFmpeg-based preview need fresh video encodes.
- easyDVD creates DVD-Video ISO files; burning is handled by other software.
  Physical DVD-player compatibility, clean Windows 10/11 VM coverage and
  interrupted-install recovery remain areas for further validation. Local
  automated pipeline, real-media and installer lifecycle checks are documented
  under `validation`; they do not establish those additional results.

Report problems at <https://github.com/maghofer/easyDVD/issues>, including the
application version, steps to reproduce and relevant logs. Review logs for source
filenames and local paths before posting them publicly.

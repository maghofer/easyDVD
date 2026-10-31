# Windows Sandbox preparation

Prepared: 2026-10-31, Windows 11 Pro.

- Enabled the Windows optional feature `Containers-DisposableClientVM` and its
  required dependencies through an administrator-approved helper. Windows
  reported success with `RestartNeeded: true`. No restart was initiated.
- Hardware supports virtualization, but Windows reported firmware virtualization
  disabled and no active hypervisor. The workstation's AMD SVM setting must be
  enabled in BIOS before the clean-machine test can run.
- Prepared online and offline `.wsb` configurations under the workstation's
  prescribed scratch directory. Each assigns 8 GB memory and opens the test
  input folder on login. The input folder is mapped read-only; a separate results
  folder is writable so logs, notes and output can survive closing Sandbox.
- Staged checksum-verified rc.1 and rc.2 installers for fresh-install and upgrade
  testing, with their checksum files. Staged two generated three-second PAL/NTSC
  videos with audio and a plain-language test checklist. No host FFmpeg, AviSynth,
  AVStoDVD or development-tool directories are shared into Sandbox.
- Verified both configurations parse as XML and the Windows feature helper
  parses as PowerShell. Feature status and installation logs remain in scratch.

The Sandbox has not been launched and no clean-machine pass is claimed. Next:
enable SVM in BIOS, save/restart, and open the online test configuration. Install
rc.2 inside Sandbox and exercise first-run downloads and both encoding profiles.
Save results into the mapped results folder before closing the Sandbox.

References:
- [Microsoft Sandbox configuration](https://learn.microsoft.com/en-us/windows/security/application-security/application-isolation/windows-sandbox/windows-sandbox-configure-using-wsb-file)
- [MSI Click BIOS instructions](https://www.msi.com/Landing/msi-click-bios-intel-amd-motherboard)

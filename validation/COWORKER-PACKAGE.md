# Coworker preview handoff

Historical preview.7 workaround. Preview.8 returns to a portable bundle of
source-built helpers; current setup is in docs/COWORKER-TEST.md and no longer
requires DVDStyler.

Date: 2026-10-28. Version: `0.1.0-preview.7`, variant `external-tools`.

The prepared runtime and matching application/Go source ZIPs are for the planned
2026-10-29 workplace test. The runtime ZIP contains only easyDVD's three
executables, its notices/documentation and launcher. No external DVD/FFmpeg
executables or DLLs are included. Setup is described in docs/COWORKER-TEST.md,
which is also copied to START-HERE.md at the root of the runtime ZIP.

Passed:

- Full Go unit suite during packaging and PowerShell syntax checks.
- Source/privacy audit; build/runtime notices included, empty external-binary
  manifest and no unexpected executable/DLL files in the kit.
- Launcher uses an explicitly selected original upstream tool installation.
- Launcher discovers the standard Program Files/DVDStyler/bin layout, including
  spaces in the path, and rejects a missing installation. The layout was simulated
  locally; the official installer was not installed into the system.
- The actual packaged GUI executable ran in headless mode against two synthetic
  videos (4:3 with audio, widescreen without audio), using external authoring
  tools from the original pinned supplier installation and separate FFmpeg 9.0.2.
  It completed a PAL DVD ISO of 2,430,976 bytes. Queue, work, output and application
  data were isolated from the user's GUI state.
- The strengthened source verifier rejects the actual incomplete supplier review.
  A synthetic complete fixture passes; changing its source archive makes it fail.
- All ten source-candidate downloads match their recorded hashes.

The bundled dependency review remains incomplete for the reasons recorded in
THIRD-PARTY-REVIEW.md. This handoff does not certify the external distributors'
source compliance. Clean Windows VM, network-interruption, long-job, full-capacity
DVD-9 and physical-player results remain useful work for the coworker test.

No GitHub repository, release or message to the coworker was created or sent.

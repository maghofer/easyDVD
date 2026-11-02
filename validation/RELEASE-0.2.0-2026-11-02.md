# 0.2.0 release preparation

Validated on Windows x64 on 2026-11-02. Generated media, installer/compiler
outputs, temporary builds and queue state were kept in isolated local scratch
storage. The GUI queue and preferences were not modified.

- All-package unit tests and vet passed, as did headless CI-tag tests/vet and
  a Linux headless cross-build.
- Race checks passed for engine, media, subprocess handling, model, packing
  and storage.
- The complete real-tool DVD/Blu-ray integration suite passed in 54.236 s,
  including the independent libbluray reader, both DVD profiles/standards,
  HCEnc mux frame preservation, mixed formats/audio, copying/re-encoding,
  menus, autoplay, cancellation, resume and 36/99-title DVD authoring.
- The build script now prepares a private Blu-ray runtime and enables these
  tests during its integration phase. It restores the caller's short temporary
  directory after preparation so legacy DVD helpers do not encounter the
  preparation directory's longer paths.
- Source-built Quality support and DVD-helper corresponding-source records
  verified. Public-release portable and source ZIPs and the Windows installer
  compiled successfully. No production installer was executed in this check.
- With PATH restricted to Windows system directories, the packaged CLI reported
  0.2.0, discovered its private Blu-ray runtime and completed a Quality BD-25
  job from a generated 720p24/AAC source. FFmpeg was supplied explicitly; system
  Python/Java and HCEnc were unavailable to that run.
- Workstation paths in the validation notes were deidentified without changing
  their measurements. The complete repository source audit and whitespace
  checks passed.

The installer uses the existing application identity and packaging recipe.
Blu-ray remains experimental. Standalone players, burned optical media, clean
Windows VM coverage and full-capacity dual-layer hardware behavior remain
outside these checks. See [Blu-ray validation](BLURAY-2026-11-02.md),
[workflow validation](WORKFLOW-2026-11-02.md) and [installer validation](../docs/INSTALLER.md).

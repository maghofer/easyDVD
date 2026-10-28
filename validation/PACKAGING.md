# External FFmpeg packaging validation

Candidate: `easyDVD-0.1.0-preview.6-win64.zip`.

- Runtime ZIP: approximately **27.7 MB** (decimal); extracted runtime about **60.5 MB**.
- Bundled authoring tools and required DLLs: about **7.3 MB** extracted.
- No FFmpeg/FFprobe executables or source archives in the normal runtime package.
- Separate matching sources ZIP: approximately **90.2 MB**.

Passed checks:

- Unit suite and `go vet ./...`, plus race-instrumented media, engine and real
  DVD-pipeline tests.
- Matching release versions, tested 9.x range, required codecs/filters/muxers,
  missing tools, mismatched pairs, and incompatible-candidate fallback.
- Stray current-directory executables are not accepted through an ignored
  `exec.LookPath` / `ErrDot` error.
- TLS download tests: archive checksum rejection before extraction, cancellation
  and cleanup, failed-compatibility rejection, and no overwrite of an existing
  installation.
- Actual pinned FFmpeg 9.0.2 download into an isolated local profile, Windows tar
  extraction, both executable hashes, capability validation and saved choice.
- Authoring tools run with only Windows system directories on PATH and without
  FFmpeg installed alongside the app.
- Missing FFmpeg reports setup guidance and does not initiate a download.
- An incompatible PATH build falls back to the validated private cache.
- A saved external location works with no FFmpeg on PATH.
- The slim GUI executable's headless workers produce a complete ISO set using
  external FFmpeg while PATH contains only Windows system directories.
- Native first-run dialog displays Download and Locate choices; selecting an
  already-installed pair validates it, saves the isolated profile choice and
  enables the application without a download.

The download/location checks used isolated profiles; the user's installed FFmpeg
and system PATH were not modified. Physical-player and clean-VM release checks
listed in RESULTS.md remain separate work.

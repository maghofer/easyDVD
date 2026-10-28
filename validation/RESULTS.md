# Local validation results

The source-built preview.8 helper bundle and its later validation are documented
in [SOURCE-BUILT-TOOLS.md](SOURCE-BUILT-TOOLS.md). It supplies the matching authoring
source/build material; the supplier-provenance gap described by earlier reports
does not apply to those replacement binaries.

Original self-contained candidate: `easyDVD-0.1.0-preview.4-win64.zip`.
The smaller external-FFmpeg revision is documented in [PACKAGING.md](PACKAGING.md).

Passed on this Windows development machine:

- `go test ./...` and `go vet ./...`.
- Race-instrumented engine, store, process-tree and real pipeline tests.
- PAL/DVD-5/text/staged and NTSC/DVD-9/thumbnail/direct pipelines using actual
  FFmpeg, spumux, dvdauthor and mkisofs binaries.
- Pause and engine restart with completed encoding artifacts reused unchanged.
- ISO SHA-256, source-file preservation, DVD stream geometry/audio/aspect,
  matching interlacing and cross-standard deinterlacing, duration and pagination.
- Windows parent/grandchild process cancellation and queue exclusivity.
- GUI layout tests and native visual checks of the central drop area, queue,
  native folder dialog, menu preview and settings.
- Non-overlapping button rectangles at maximum PAL/NTSC menu density.
- Running the packaged **GUI executable** in headless worker mode with PATH
  restricted to Windows system directories: complete three-title ISO set.
- libVLC navigation fixture: title selection/return, all three titles through
  Play All, next page, selecting title three and returning to page two.
- Linux cross-compilation using the Fyne `ci` build tag.

The FFmpeg benchmark smoke result is in `encoder-baseline.json`. mpeg2enc and
HCenc were not available; actual film-scan samples will be supplied later.

Not yet completed: physical DVD-player/burn tests, full-capacity DVD-9 and layer
break verification, clean Windows 10/11 VM tests, real disconnected-SMB and
volume-exhaustion exercises, the complete optional-encoder comparison, and the
toolchain corresponding-source/license review needed for public binary release.
The system-only PATH exercise does not substitute for clean-machine testing.

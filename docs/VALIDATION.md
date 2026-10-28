# Validation

## Automated checks

- Natural numeric ordering, duplicate basenames and nested paths.
- Strict-order versus optimized packing, unique inclusion, capacity bounds,
  oversized titles, cancellation and the 99-title limit.
- SQLite round-trip/order/settings persistence and resumed locked settings.
- Independent GUI snapshots and encoding-profile invalidation rules.
- Aspect-preserving scaling, frame-rate/interlace handling, audio stream mapping,
  silent footage, DVD arguments and authoring navigation generation.
- PAL/NTSC text and thumbnail pagination, button bounds, subpicture XML and preview
  dimensions.
- Actual Windows parent/grandchild cancellation through a Job Object.
- FFmpeg release matching, missing capabilities, incompatible-candidate fallback,
  saved folder selection, checksum rejection, canceled downloads and atomic
  installation without replacing existing tools.

`go test ./...` runs these without authoring a DVD. Set `EASYDVD_INTEGRATION=1`
and `EASYDVD_TOOLS` to a prepared tool directory to enable the real pipeline tests.
The pipeline test executable includes the same filesystem/discovery helper
dispatch as the app. Use `GORACE=atexit_sleep_ms=0` to reduce child-process startup
overhead when running instrumented integration tests.

## Real pipeline exercise

For a read-only inventory of an existing media collection, PowerShell 7 can probe
each video with a 45-second timeout and checkpoint the metadata in a selected
scratch directory:

```powershell
./scripts/inspect-testdata.ps1 -Source $MediaRoot -Scratch (Join-Path $ScratchRoot inventory)
```

Use `-state`, `-work`, and `-output` with paths under the scratch directory for
isolated CLI tests. Set process-local `TEMP` and `TMP` there too when running
tests or builds. The inventory checks metadata, not full decodability. Local
real-media results are recorded in `validation/REAL-MEDIA.md`.

The integration fixture includes progressive 4:3 footage with audio, interlaced
SD footage, and silent widescreen HD footage inside a nested directory named
with accented characters and an ampersand. It validates:

- PAL/DVD-5/text/staged input and NTSC/DVD-9/thumbnails/direct input.
- Multiple pages, completed ISO hashes and source-file hashes.
- DVD demuxing of every output title: MPEG-2 geometry, AC-3 stereo, aspect and
  duration within DVD frame/audio-packet tolerance.
- Pause, shutdown and restoration of completed encoding checkpoints.
- A missing folder alongside a valid set, without stopping the valid work.
- ISO-only output directories, persistent logs/manifests outside output, and
  temporary preview caching.
- Recovery after the atomic output-directory rename using the local manifest.

The libVLC navigation fixture uses three titles and two menu entries per page.
It checks selection/return, Play All across every title, Next page, selection
from the second page, and return. This is software-player evidence, not physical
DVD-player certification.

The many-title authoring regression creates actual DVDs with 36 titles on four
pages and 99 titles on 99 pages, then reads their first and last titles through
the DVD-Video demuxer. This covers the former generated jump-pad command overflow
and the maximum supported menu-page count without encoding 99 separate sources.

With an unpacked VLC Windows x64 distribution, run the three-title fixture:

```powershell
go run ./cmd/easydvd-navcheck C:\Tools\VLC C:\Fixtures\DVD-001-of-001.iso
```

## Manual acceptance

- Add thousands of files; keep scrolling, resizing, changing selection and opening
  settings responsive while enumeration/probing continues.
- Add an unavailable UNC share; cancel scanning and continue using the queue.
- Disconnect a share during staging/direct encoding and during output copying.
  Verify cancellation and retry with completed work retained.
- Fill the work/output volume mid-operation; verify the source files are unchanged
  and no partial output is presented as a completed set.
- Change a source between attempts; verify the affected encode is invalidated.
- Validate 100+ short titles, very long labels, mixed portrait/4:3/widescreen files,
  DVD capacity boundaries and overflow correction.
- Verify dark/light/system appearance, high DPI, keyboard focus, native multi-folder
  selection and folder drag/drop on Windows 10/11.
- On a clean VM without Go/GCC/FFmpeg/DVDStyler installed, extract the package,
  confirm the authoring tools are included and FFmpeg setup is offered. Test
  Locate and Download separately, inspect Tools, process the mixed fixture,
  reopen and resume. Repeat with PATH FFmpeg and the optional offline package.
- Burn the resulting DVD-5/DVD-9 images externally and exercise navigation, aspect
  ratio and audio sync in physical players. easyDVD does not burn discs itself.

## Current limitations

Physical-player, clean-VM, disconnected real-SMB, and actual full-capacity media
tests require those environments. They are not claimed as completed by local
unit tests. HDR needs external SDR conversion. Film cadence is preserved in
duration using frame duplication/dropping; optical-flow interpolation and
specialized film-restoration cadence processing are intentionally absent.

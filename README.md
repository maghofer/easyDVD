# easyDVD

A focused Windows utility for turning folders of video files into playable
DVD-Video ISO sets. Built with Go and Fyne. Quality comes before disc count.

The current development version is recorded in [VERSION](VERSION). The app shows
it in its title and Settings; the CLI accepts `-version`. **Settings → Check for
updates** checks the latest stable release on GitHub only when clicked. If a
newer release exists, **Open release page** opens it in your browser. There are
no automatic checks, downloads or installations. Preview releases are not offered.

## Run

Extract the portable package and open **easyDVD.exe**. Keep its `tools` folder
beside the executable: the small DVD authoring tools are included. Go and a
compiler are not needed on the destination computer.

The three DVD helper programs are built from pinned source and bundled with
easyDVD. DVDStyler is not required. See [coworker testing](docs/COWORKER-TEST.md)
and the [helper build recipe](scripts/toolchain/README.md).

FFmpeg and FFprobe are external in the normal package. easyDVD checks an existing
installation on PATH. If it cannot find a compatible pair, **FFmpeg setup** offers
**Use this folder** or **Download recommended 9.0.2**. Nothing is downloaded
automatically. An optional offline package includes FFmpeg as well.

1. Drop one or more folders into the window, or choose **Add folders**.
2. Review the discovered videos, set title, menu preview, and job settings.
3. Select **Start / resume**. Finished sets appear in the selected output folder.

Each explicitly added folder is a separate set, including its subfolders.
Folders are never combined. Videos are naturally sorted by filename (2 before
10), with relative paths breaking ties. Each file is a separate DVD title.

**Strict order** preserves the complete sequence across discs. **Space
efficiency** can move whole videos between discs; menus within each disc remain
in natural order. It uses best-fit decreasing followed by up to ten seconds of
search. A proven minimum applies to the conservative packing budget, including
the reserved DVD overhead. Final ISO byte size is always checked.

Disc counts shown before encoding are estimates. The final layout uses encoded
file sizes. Neither packing mode lowers the encoding profile, splits files,
nor silently excludes failed videos. A video too large for one disc blocks
that set. Other sets can continue.

## Defaults and supported behavior

- PAL, DVD-5, strict order, text menus with ten titles per page.
- NTSC and DVD-9 are supported. Text menus allow 1–12 titles per page;
  thumbnail menus allow 1–6, defaulting to six when selected.
- Two-pass software MPEG-2, 8 Mbps average / 9 Mbps peak, stereo AC-3 at
  48 kHz / 256 kbps. At most two videos encode concurrently.
- Preserve 4:3/16:9 display geometry; pad other ratios. Preserve duration;
  conform video to PAL 25 fps or NTSC 29.97 fps without changing playback speed.
- Preserve known, matching-standard interlacing; deinterlace other interlaced
  inputs. No automatic grain reduction, sharpening, cropping, interpolation,
  or volume normalization.
- Use the default audio track or first track; synthesize silence if absent.
- Static menus start on insertion. A selected title returns to its page and
  selection. **Play All** plays the current disc in menu order, then returns
  to its first page. Previous/Next and remote directional navigation work.
- **Menu** opens parameter-based menu settings with an automatic preview. Choose
  4:3 or 16:9, text or thumbnail layout, a PNG/JPEG background, built-in or
  imported TTF/OTF fonts, colors, label/heading sizes, line spacing and dimming.
  Named presets persist across restarts. See [menu settings](docs/MENUS.md).
- Local per-file staging is the default; direct network reading is optional.
  Use a fast local work directory with enough room for encoded media, staged
  sources, an authored disc, and an ISO.

DVD is standard definition. HDR inputs are explicitly blocked until converted
to SDR externally. Unknown or corrupt streams, missing duration metadata, and
unreadable files are reported rather than guessed. Multiple audio tracks,
subtitles, trimming, animated menus, disc burning, and freeform menu design are
outside v1. A preview is artwork, not a DVD navigation simulator.

## Queue and recovery

Queue/preferences are stored in `%APPDATA%\easyDVD\queue.db`. Job settings lock
when processing starts. Global changes apply to new and unstarted jobs.

**Pause** finishes active stages and saves checkpoints. **Cancel job** interrupts
the process tree and preserves completed encodes. **Resume / retry** checks the
sources and reusable media before continuing. After a restart, processing waits
for an explicit Start/Resume action.

Failures block only their folder. Select a video and use **Edit / exclude** to
explicitly omit it, then retry. A failed directory scan must be retried; missing
subdirectories are never silently treated as an empty collection. **Details**
contains full errors, disc assignments, and the latest log output.

Completed set folders contain only `DVD-001-of-00N.iso` files. Logs are retained
under `%LOCALAPPDATA%\easyDVD\logs`; **Settings → Open logs folder** opens them.
Source-to-disc manifests with ISO SHA-256 values are retained under
`%LOCALAPPDATA%\easyDVD\jobs\JOB_ID\manifest.json`. Menu graphics and previews
are temporary work files; previews can be regenerated if their cache is removed.
The whole ISO set is published after validation by renaming a private staging
directory. Intermediate media is cleaned after
success unless **Keep intermediate media** is selected. Removing a queue entry
does not delete its source files, finished outputs, or retained work.

## Develop on Windows

PowerShell, Git, and network access are needed for the first bootstrap. Scripts
install pinned tools under `.tools`; they do not install system-wide software.

```powershell
./scripts/bootstrap.ps1
./scripts/build.ps1 -Test -Integration
./bin/easyDVD.exe
```

`scripts/tools.lock.json` pins Go 1.27.3, GCC 16.2.0, the recommended
FFmpeg/FFprobe download, and the source-built authoring toolchain ID. The runtime embeds
that same lock file. `go.mod` and `go.sum` pin Go dependencies. Development
bootstrap prepares FFmpeg for tests; the default package still excludes it.
Runtime and source ZIPs are generated separately.

```powershell
./bin/easydvd-cli.exe -check-tools
./bin/easydvd-cli.exe -output D:\DVDs -work C:\DVD-work -titles 10 C:\Videos\Folder1 C:\Videos\Folder2
./bin/easydvd-cli.exe -resume
./bin/easydvd-cli.exe -standard NTSC -media DVD-9 -efficient -thumbnails -titles 6 -direct C:\Videos\Folder1
```

The CLI shares the engine and default queue with the GUI. An exclusive queue
lock prevents simultaneous writers. Close the GUI before using the default
CLI queue, or use `-state ABSOLUTE_DB_PATH` for
isolated automation/testing. Isolated queues keep their logs and job manifests
beside their database. `-keep-work` retains completed intermediate media.

## FFmpeg setup

Currently supported: matching FFmpeg and FFprobe release versions **9.0.2 through
9.x**, with MPEG-2/AC-3 encoding and the required filters and DVD muxing features.
Older releases, future major versions, nightly version strings, mismatched pairs,
and incomplete builds trigger setup instead of being accepted just by filename.

An explicit saved folder overrides automatic discovery. Otherwise PATH is tried
before easyDVD's private downloaded installation and an optional offline sidecar.
The selected folder is saved in `%APPDATA%\easyDVD\ffmpeg.json`. Managed downloads
live under `%LOCALAPPDATA%\easyDVD\ffmpeg`; they do not modify the system PATH.
Downloads use HTTPS, verify the pinned archive and both executable hashes, and
publish an installation only after validation. Canceling preserves existing
installations. Use **Tools → FFmpeg setup** while the queue is stopped to change
the selected folder.

```powershell
./bin/easydvd-cli.exe -check-authoring-tools
./bin/easydvd-cli.exe -download-ffmpeg
./bin/easydvd-cli.exe -ffmpeg-dir C:\Tools\ffmpeg\bin -check-tools
```

`-ffmpeg-dir` applies to that CLI run; downloading explicitly remembers the
private installation. `EASYDVD_FFMPEG` provides a folder override for automation.
`EASYDVD_TOOLS` can still override the authoring-tool directory.

## Tests, packaging, and encoder research

```powershell
./scripts/build.ps1 -Test -Integration
./scripts/package.ps1
./scripts/package.ps1 -IncludeFFmpeg
./bin/easydvd-benchmark.exe -input C:\Scans\sample.mov -output C:\Benchmarks\sample-01 -seconds 10
```

See [architecture](docs/ARCHITECTURE.md), [validation](docs/VALIDATION.md),
[encoder comparison](docs/ENCODERS.md), and [release preparation](docs/RELEASING.md).
The package is a preview for validation. Its matching sources ZIP includes the
DVD toolchain source/license material described in [third-party notices](THIRD_PARTY.md).

easyDVD's own code is MIT licensed. External components retain their licenses.

# DVD encoding quality

This page describes DVD profiles. Experimental Blu-ray uses libx264 for both
profiles; its processing and copying rules are in [Blu-ray support](BLURAY.md).

Balanced uses FFmpeg. Quality uses HCEnc 0.28 with the encoder settings from
the supplied AVStoDVD 2.8.9 log. FFmpeg/FFprobe remain external: locate a
compatible installation or accept the app's private download. Quality's source-built
32-bit AviSynth+ / FFMS2 support libraries are included; HCEnc is a separate
verified download through Quality setup.

**Balanced is the default for new jobs.** Choose Balanced or Quality directly
in the job header before processing starts. The choice is saved per job; started
jobs keep their locked profile. Encoding preset infrastructure is retained internally
for future custom presets; it has no UI entry point.
CLI and benchmark accept `-profile Balanced` or `-profile Quality`.

| Profile | Video target / peak | Stereo AC-3 | Intended use |
|---|---|---|---|
| Balanced (default) | 7,500 / 9,000 kbps | 256 kbps / 48 kHz | Faster encoding, smaller files, good DVD quality |
| Quality | 8,500 / 9,000 kbps | 448 kbps / 48 kHz | HCEnc BEST, AVStoDVD two-pass VBR settings |

Balanced's parameters and cache identity are unchanged. Its earlier measurements
are in [the Balanced report](../validation/BALANCED-SCOPE-2026-10-29.md); the
FFmpeg Quality comparison in that report describes the previous encoder.

## Bitrate and disc space

Each profile has its own VBR target and a **9,000 kbps maximum**. easyDVD
never lowers the selected profile's bitrate to fit more footage on a disc: it packs the actual
encoded sizes and uses additional discs when needed. A title that cannot fit
whole on the selected medium is reported; choose DVD-9 or split the source.

The ceiling follows FFmpeg's DVD target, leaving room for AC-3,
navigation and multiplexing within the 10,080 kbps programme-stream mux rate.
The DVD video buffer is 1,835,008 bits. A short programme may leave much of a
disc empty: exceeding the bitrate ceiling or adding padding cannot improve it.
Very simple material can also use fewer bits after reaching the profile's
minimum quantizer.
ISO size checks and packing reserves remain authoritative for disc capacity.

## Quality runtime setup

Install or extract the complete package, including `tools/quality`. It contains
source-built 32-bit `avisynth.dll` and `ffms2.dll`. After FFmpeg setup/checking,
the GUI automatically downloads HCEnc if it is missing. The 2.2 MB ZIP comes
directly from the author at `http://hank315.nl/`. The legacy site uses HTTP;
pinned SHA-256 checks on both archive and encoder must pass before installation.
Only HCEnc and its original notices are extracted.
AVStoDVD is not installed. No codec registration or PATH change is required.
The private encoder cache survives upgrades and uninstall. CLI setup uses
`-download-hcenc`. Cancel or close the setup dialog to stop the download. If offline,
use Balanced and retry from **Settings → Repair dependencies → Quality (hcenc) setup**
or **Media tools → Quality (hcenc) setup**. A verified
cached encoder is reused without another download. Missing HCEnc is retried on
the next launch; an explicit `EASYDVD_HCENC` selection is left to the user.
Bundled notices are in `notices/quality`, and DLL hashes in `quality-binaries.json`.
The matching source package includes all four library source archives and the
build recipe. DevIL is no longer required by the distributed runtime.
See [the runtime record](../scripts/quality-runtime/README.md).

For a custom runtime, set `EASYDVD_HCENC` to a folder containing `HCenc.exe`,
`ffms2.dll` and a portable `avisynth.dll` beside the executable. Classic
AviSynth also requires `DevIL.dll` beside its DLL.
The AVStoDVD layout (`HCenc/HCenc.exe`, `Lib/ffms2.dll`, installed AviSynth)
also works. Explicit `EASYDVD_HCENC` / `EASYDVD_TOOLS` choices take precedence;
automatic discovery prefers `tools/quality` before external installations. A
partial bundled runtime is reported rather than mixed with a system AviSynth.
The `quality` child of an override folder is supported. All runtime binaries must be 32-bit.
Missing components block Quality with a setup message; there is no automatic
fallback to FFmpeg Quality.
The Tools dialog reports the optional Quality runtime. CLI
`-check-tools -profile Quality` also requires it to be present.

## Quality profile

The generated HCEnc INI uses:

```text
*AVSRELOAD
*MAXBITRATE 9000
*BITRATE 8500
*PROFILE BEST
*ASPECT 16:9
*PROGRESSIVE
*TFF
*BIAS 80
*DC_PREC 10
*MATRIX MPEG
*AQ 1
*LUMGAIN 1
*SMP
*PRIORITY IDLE
```

Aspect, frame type and field order follow the source and selected DVD standard:
4:3 sources use `*ASPECT 4:3`; retained interlaced footage uses `*INTERLACED`
and TFF/BFF as appropriate. HCEnc's automatic GOP and scene detection remain
enabled, as in the log. PAL uses 720×576 at 25 fps; NTSC uses 720×480 at
30000/1001 fps. The command explicitly selects `-2pass`, runs without the
encoder GUI and exits immediately (`-silent -wait 0`). Database/input/output
paths belong to the title's private work directory. The complete INI, input
script and HCEnc report are recorded in the job log.

FFmpeg prepares a temporary lossless FFV1/YUV420 SD input. FFMS2 serves this
through AviSynth to HCEnc, supporting both the C plugin supplied by AVStoDVD
and C++ plugin builds. This preserves easyDVD's existing source selection,
rotation, crop, scaling, color and frame-rate handling. It matches the HCEnc
encoder settings, rather than duplicating AVStoDVD's A2DSource/DirectShow,
ColorMatrix and Spline36Resize script. Both profiles currently retain easyDVD's
Lanczos/BT.601 preparation; their pixels need not match AVStoDVD exactly.

The temporary lossless input and elementary stream are removed after each
attempt, including cancellation/failure. Free-space checks reserve a conservative
raw-frame bound for concurrent Quality workers; long titles can require
substantial additional work space. HCEnc uses SMP at idle priority, with
concurrent titles according to the worker setting.

FFmpeg stream-copies HCEnc video and encodes the selected audio as stereo AC-3
at **448 kbps / 48 kHz**. It pads short audio or creates silence, and muxes with
`-f dvd -muxrate 10080k -packetsize 2048`. The source duration limits lossless
preparation; the final mux drains the complete HCEnc video to EOF. Padded audio
follows the video with `-shortest`. An output `-t` cutoff is deliberately omitted
because it can discard the tail of a stream-copied MPEG-2 stream. The `setts`
bitstream filter subtracts the first video PTS from both PTS and DTS, aligning
the first displayed frame with audio while preserving B-picture decode order.
The encoded video bytes are unchanged.

The muxed video packet count must match HCEnc's completed-frame report before
the MPG is fully decoded, hashed and published as a checkpoint. Quality's v2
cache identity invalidates earlier HCEnc v1 checkpoints in unfinished jobs:
they may already have lost end frames, which remuxing cannot restore. Old
FFmpeg Quality checkpoints are also incompatible. Balanced checkpoints keep
their identity. Completed ISO sets are not rebuilt automatically. Saved jobs
predating the profile selector retain Quality, requiring HCEnc on resume.
See [the mux regression record](../validation/HCENC-MUX-2026-11-02.md).

## Balanced profile

Balanced retains FFmpeg two-pass MPEG-2 at 7,500 kbps target, 9,000 kbps peak,
256 kbps / 48 kHz stereo AC-3, and these analysis options:

```text
-qmin 2 -lmin 236 -mbd rd -intra_vlc 1
```

Trellis, SATD searches, motion prepass, larger search patterns, extra predictors,
bidirectional refinement and `mpv_flags` overrides are left at FFmpeg defaults.
Balanced keeps closed GOPs, two B pictures and a 15/18-picture PAL/NTSC GOP.
The DVD raster, peak rate, VBV, aspect and field handling are retained.
The quantizer floor avoids spending unnecessary bits on clean/simple scenes.

Resizing uses Lanczos and BT.601 limited-range output, with BT.709 input assumed
for HD and BT.601 for SD. PAL tags are `bt470bg`, NTSC tags `smpte170m`, with
`bt709` transfer. No denoising, sharpening or custom quantization matrix is
applied. Matching interlaced footage retains field-aware scaling, field order
and `+ildct+ilme`; other interlaced footage is deinterlaced before conversion.

The [earlier FFmpeg Quality report](../validation/FFMPEG-QUALITY-2026-10-29.md)
is historical and does not measure the current HCEnc profile.

## Encoding performance

**Encoding workers** offers Auto or a fixed count from 1 to 32 in application
Settings. The next job or resume uses this global choice; active stages keep
their current allocation. Completed encodes are reused and quality settings stay
locked. The first migration to application-wide workers selects Auto; explicit
choices subsequently saved in Settings persist. Encoding presets exclude workers.
CLI uses `-encoders 0` for Auto, or e.g.
`-encoders 4` for four workers.

DVD Auto starts with approximately one worker per two logical CPUs for both profiles.
The count is capped at 32 and at the number of titles still needing encoding.
On Windows it also reserves 2 GiB of currently available physical memory and
budgets 1 GiB for each worker. Unknown memory limits Auto to two workers.
Auto reduces concurrency further if the conservative work-volume reservation
(including concurrent Quality lossless inputs and staged originals) would not
fit. Insufficient space even for one worker still blocks the job. Fixed worker
counts keep the normal space checks. The job log records the resolved count.

This is a resource-based starting estimate, not a measured maximum or a live
CPU-utilization controller. Source decoding, encoder scaling, storage speed,
SMT, other applications and the number of remaining titles affect the best
count. Blu-ray has a separate resource policy described in [Blu-ray support](BLURAY.md).
Benchmark the same representative files at several fixed counts when
tuning a workstation; compare completion time rather than CPU percentage alone.
One long remaining title still uses one encoder.

Balanced retains its maximum of eight MPEG-2 encoding threads per process.
FFmpeg input decoding and filtering, and Quality's FFV1 preparation, now use
the same per-worker thread budget instead of each defaulting to the whole CPU.
Quality's BEST/two-pass settings, SMP, automatic GOPs, bitrates, matrices and
field handling are unchanged. FFMS2 still uses one decoding thread for the
prepared FFV1 level-1 input. The
[HCEnc 0.28 manual](https://hank315.nl/files/HC_028/HC028.pdf) documents SMP
on/off and multiple instances, but no configurable encoder thread count.

Worker selection does not change either profile's cache identity: compatible
completed encodes are reused. Processing whole titles concurrently adds no
extra video encoding pass or generation of lossy compression.

### Chunking a single title

Chunked encoding is not enabled. It would need its own validation of frame
counts, interlaced fields, GOP references, bitrate/VBV behavior, timestamps and
audio continuity. Independently encoding chunks changes GOP boundaries and
two-pass bitrate allocation even with identical quality options, so it cannot
be described as the same encode made faster.

Input keyframe alignment is necessary for independently stream-copied chunks
of many inter-frame formats, with open GOPs requiring additional care. It is
not necessary just to begin transcoding at an exact frame: FFmpeg can
[seek to an earlier seek point and decode/discard up to the requested position](https://ffmpeg.org/ffmpeg.html#Main-options).
That adds decoding work, not another lossy encode. Alternatively, Quality
already has a lossless SD intermediate that could feed frame ranges to HCEnc's
documented `*FRAMES` command. Independent output chunks would need safe
[GOP boundaries](https://ffmpeg.org/ffmpeg-formats.html#segment_002c-stream_005fsegment_002c-ssegment)
and a stream-copy/remux join; audio should be encoded continuously once.
This is a possible follow-up for single-title workloads after whole-title
parallelism is measured.

## Automatic pillarbox removal

For 16:9 input, easyDVD checks every decoded frame before resizing. It removes
only symmetric black side borders that leave the full centered 4:3 picture.
Small variations at the boundary expand the kept image to include every
observed pixel, rounded outward for chroma alignment. The cropped picture's
actual display aspect controls scaling and DVD aspect; rounding differences
are padded rather than stretched. Wider content or small visible marks deeper
in a border prevent cropping. Letterbox bars are retained.

The check uses per-pixel luminance with a two-level tolerance above nominal
8-bit black, retaining 16-bit analysis precision. Uncertain or noisier borders
keep the original frame. This adds a decoding pass to new 16:9 encodes;
completed compatible checkpoints are reused.

## Scope footage and playback windows

A 1920×796 source has a 2.41:1 frame. DVD-Video stores this inside a 16:9 frame,
with top/bottom letterboxing. PAL uses the full 720-pixel width and 424 picture
lines in the 576-line frame. The output signals 16:9 with 64:45 sample aspect.
The picture is not cropped or stretched horizontally.

In a playback window wider than 16:9, the player fits the whole DVD frame by
height, leaving side bars around it. The original 2.41:1 frame can instead fit
by width and look larger. This can produce visible bars on all four sides even
when the encoded picture reaches both horizontal edges. Zooming/cropping the
letterbox during playback can fill that window; a 2.41:1 DVD display-aspect flag
would not be a compatible DVD-Video solution. The supported aspect flags are
documented by [dvdauthor](https://github.com/ldo/dvdauthor/blob/master/doc/dvdauthor.sgml).

## Repeatable benchmark

`easydvd-benchmark` prepares a progressive DVD-resolution YUV4MPEG reference,
runs the selected production profile, measures video size and PSNR/SSIM, and authors
a test ISO. The report records reference/source hashes, per-process wall and CPU
time, and MPEG-2 encoding timings (one HCEnc process contains both passes).
Audio and muxing are excluded
from the reported elementary video size. Preparation, metrics and authoring
are excluded from the encode timing.

```powershell
./bin/easydvd-benchmark.exe -input C:\Scans\sample.mov -output C:\Benchmarks\trial-01 -profile Balanced -seconds 20 -repeats 3
```

Use `-offset` for another scene. For a complete short progressive source,
`-source-pipeline -seconds 1000` includes original decoding and resizing in each
timed encode. Otherwise the prepared reference is the encode input. Interlaced
sources are deinterlaced for the prepared-reference measurement; retained-field
encoding is validated separately by integration tests. Runs are sequential;
CPU percentages are normalized to all logical processors.

Review grain, texture, dark scenes, pans and transitions in motion as well as
PSNR/SSIM. The prepared SD reference is the quality comparison, since DVD
downscaling itself discards HD detail. Software authoring and decoding checks
do not replace burned-disc tests in standalone players.

References: [FFmpeg DVD targets](https://ffmpeg.org/ffmpeg.html#target),
[codec options](https://ffmpeg.org/ffmpeg-codecs.html),
[MPlayer/MEncoder MPEG-2 encoding guidance](https://www.mplayerhq.hu/DOCS/HTML/en/MPlayer.html).

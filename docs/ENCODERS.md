# DVD encoding quality

FFmpeg is the sole video encoder. The normal package keeps FFmpeg/FFprobe
external: locate a compatible installation or accept the app's private download.
There is no separate encoder runtime to install.

**Balanced is the default for new jobs.** Choose Balanced or Quality directly
in Job settings before processing starts. The choice is saved per job; started
jobs keep their locked profile. Default settings also offers the default for new
jobs. CLI and benchmark accept `-profile Balanced` or `-profile Quality`.

| Profile | Video target / peak | Stereo AC-3 | Intended use |
|---|---|---|---|
| Balanced (default) | 7,500 / 9,000 kbps | 256 kbps / 48 kHz | Faster encoding, smaller files, good DVD quality |
| Quality | 9,000 / 9,000 kbps | 448 kbps / 48 kHz | More detail analysis and the full conservative rate budget |

Both profiles use two passes, RD macroblock decisions and intra VLC. Balanced
uses a quantizer floor of 2 and the fast default motion search without trellis
or the extra searches in Quality. On the measured 60-second film sample it
encoded in 2.96 s versus 12.47 s for Quality, with 20% less video data. See
[the Balanced measurements and Scope check](../validation/BALANCED-SCOPE-2026-10-29.md).

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

## Quality profile

Both passes use these quality settings:

```text
-c:v mpeg2video -profile:v 4 -level:v 8 -pix_fmt yuv420p
-b:v 9000k -maxrate:v 9000k -minrate:v 0 -bufsize:v 1835008
-qmin 1 -lmin 118 -mbd rd -trellis 1 -intra_vlc 1
-cmp satd -subcmp satd -mepre 1 -precmp satd
-dia_size 2 -pre_dia_size 2 -last_pred 3 -bidir_refine 2
-mpv_flags +mv0+cbp_rd
-bf 2 -flags +cgop -sc_threshold 1000000000 -dc 10
```

This is MPEG-2 Main Profile at Main Level. PAL uses 720×576 at 25 fps with a
maximum 15-picture GOP; NTSC uses 720×480 at 30000/1001 fps with an 18-picture
GOP. Closed GOPs and two B pictures are retained. The display aspect is 4:3 or
16:9 according to the actual picture. Each pass gets up to eight software
encoding threads, scaled down for the configured one or two concurrent titles.

`qmin 1` and `lmin 118` permit finer quantization when space permits. Raising
only the target bitrate left the previous profile limited by quantizer 2 on
the tested footage. Rate-distortion macroblock decisions, trellis quantization,
intra VLC coding and the motion-search settings improve the use of available
bits. Nonlinear quantization and quantizer RD search were tested but omitted
because they regressed some of the supplied film samples.

Pass one writes statistics without audio. Pass two encodes the selected audio
track to stereo AC-3 at **448 kbps / 48 kHz** in Quality or **256 kbps / 48 kHz**
in Balanced, pads short audio, and creates silence
when needed. DVD muxing uses `-f dvd -muxrate 10080k -packetsize 2048`. The
source duration limits the result; every encoded MPG is fully decoded and
hashed before it can become a reusable checkpoint. A profile version change
invalidates incompatible encoded checkpoints when an unfinished job resumes.
Existing Quality encodes retain their cache identity; saved jobs predating the
profile selector retain Quality. Switching profiles produces a different key.

Balanced replaces the target with `-b:v 7500k` and uses these analysis options:

```text
-qmin 2 -lmin 236 -mbd rd -intra_vlc 1
```

Trellis, SATD searches, motion prepass, larger search patterns, extra predictors,
bidirectional refinement and `mpv_flags` overrides are left at FFmpeg defaults.
The DVD raster, peak rate, VBV, GOPs, aspect and field handling are shared.
The quantizer floor avoids spending unnecessary bits on clean/simple scenes;
Quality permits quantizer 1 when its slower analysis can preserve more detail.

Resizing uses Lanczos and BT.601 limited-range output, with BT.709 input assumed
for HD and BT.601 for SD. PAL tags are `bt470bg`, NTSC tags `smpte170m`, with
`bt709` transfer. No denoising, sharpening or custom quantization matrix is
applied. Matching interlaced footage retains field-aware scaling, field order
and `+ildct+ilme`; other interlaced footage is deinterlaced before conversion.

The measured improvements, costs and validation scope are recorded in
[the quality report](../validation/FFMPEG-QUALITY-2026-10-29.md). These are
measurements on particular samples, not a universal claim about every source.

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
runs the production FFmpeg path, measures video size and PSNR/SSIM, and authors
a test ISO. The report records reference/source hashes, per-process wall and CPU
time, and separate timings for both MPEG-2 passes. Audio and muxing are excluded
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

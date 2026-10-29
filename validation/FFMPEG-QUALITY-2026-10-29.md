# FFmpeg-only DVD quality — 2026-10-29

HCenc, its AviSynth/FFMS2 preparation path, setup script, settings, CLI flags,
benchmark candidates and CI installation have been removed. FFmpeg/FFprobe
remain external in the normal portable package. The only bundled executables
beside the app/CLI/benchmark are the three existing DVD authoring helpers.
Old preview preferences/queues import without retaining backend paths; a saved
three/four-worker backend setting becomes two workers. The new encoding key
prevents reuse of the old quality profile for unfinished jobs.

## Selected profile

Two-pass software MPEG-2, Main Profile / Main Level, 4:2:0, **9 Mbps target and
peak**, 1,835,008-bit VBV, closed GOPs (15 PAL / 18 NTSC maximum), two B pictures,
10-bit intra DC precision. PAL/NTSC raster, cadence, aspect and field handling
remain intact. Stereo AC-3 is now **448 kbps at 48 kHz**, up from 256 kbps.
Mux rate remains 10,080 kbps with 2,048-byte DVD packets.

Finer quantization (`qmin 1`, `lmin 118`), rate-distortion macroblock decisions,
trellis, intra VLC and tested SATD/motion-search options are enabled in both
passes. Full arguments and processing order are in [the encoding guide](../docs/ENCODERS.md).
There is no denoising, sharpening or custom quantization matrix.

Packing uses actual encoded sizes, adds discs rather than reducing bitrate,
and checks the mastered ISO size. The initial size estimate now accounts for
9 Mbps video plus 448 kbps audio. Short programmes cannot fill a whole disc
with useful picture data once the DVD rate/quantization ceiling is reached.

## Measurements

Windows, Ryzen 7 9800X3D (8 cores / 16 logical CPUs), FFmpeg 9.0.2. Three
20-second progressive PAL references were prepared once at 720×576, 25 fps,
4:3, YUV420P. Two scenes come from one H.264 film scan; the third is from a
separate ProRes film scan. Each candidate ran two passes with eight threads.
Quality compares decoded MPEG-2 against these identical SD reference frames.
All media, full paths, raw logs and ISOs stay in isolated local scratch space;
[deidentified results](FFMPEG-QUALITY-2026-10-29.json) record all 39 trials.

| Scene | Old → selected video Mbps | Old → selected PSNR dB | Old → selected SSIM | Old → selected two-pass seconds |
|---|---:|---:|---:|---:|
| Movement | 6.737 → 8.938 | 43.899 → 45.044 | 0.973753 → 0.979444 | 0.447 → 4.039 |
| Grain | 7.808 → 8.991 | 44.122 → 45.124 | 0.976103 → 0.980790 | 0.478 → 3.760 |
| ProRes scene | 4.383 → 8.879 | 45.632 → 48.612 | 0.981661 → 0.989915 | 0.471 → 4.198 |

Sweep timings are single observations and exclude preparation, metrics, audio
and DVD authoring. The objective improvement is consistent across these scenes;
it is not a guarantee of a universal subjective optimum. Native-resolution
reference/old/selected frame crops were also inspected: the selected output
retains finer texture and edges without adding sharpening or stretching.

Raising the old 8 Mbps target to 9 Mbps alone barely changed the output: the
default quantizer floor was the limiting factor. Allowing quantizer 1 increased
useful bitrate; RD/trellis and the selected search/VLC settings further improved
metrics. Nonlinear quantization regressed both initial scenes substantially;
quantizer RD search also regressed the movement scene and underused the budget.
Those options were omitted. Every trial completed without logged buffer
underflow/overflow diagnostics.

Three sequential production runs on a complete 60-second original H.264 sample
included decoding, crop/resize/cadence conversion, both MPEG-2 passes, AC-3/mux,
full decode validation and checkpoint hashing. Median elapsed time was **12.48 s**,
with **12.24 s** in the two MPEG-2 passes and **46.6%** mean CPU utilization during
those passes (normalized to all 16 logical CPUs). Each run authored a test ISO.
The previous fast FFmpeg profile took 2.02 s in the earlier comparison, so the
quality gain has a clear throughput cost. The new profile still encoded this
sample at approximately 4.8 times real time. The earlier HCenc comparison used
the old profiles and should not be read as a comparison at the new quality.

## Validation scope

- All-package Go tests and vet, plus race checks for media, model, engine,
  process, Windows platform helpers and store.
- Actual PAL/NTSC pipelines check MPEG-2 Main Profile / Main Level, YUV420P,
  cadence, 48 kHz / 448 kbps stereo AC-3, mixed aspect ratios, retained matching
  interlacing and cross-standard deinterlacing. DVD's IFO-derived probe omits
  sequence profile/level, so those are checked on the encoded MPG itself.
- Menus, thumbnails, autoplay, Unicode work paths, scan/exclusion recovery,
  checkpoint reuse/publication recovery and 36/99-title authoring pass.
- Three 60-second streams contain 1,500 frames each, closed GOPs of at most
  13 pictures and at most two consecutive B pictures. Sequence headers declare
  9 Mbps, the 1,835,008-bit buffer and Main Profile / Main Level. An elementary
  packet/DTS token-bucket check stays within that rate/buffer envelope; this is
  an additional sanity check, not a complete DVD/VBV conformance certification.
- The complete 795.111-second pillarboxed H.264 fixture was regenerated with
  this profile. The saved crop retains the full 964-pixel picture, removing
  158 pixels per side. MPG and authored DVD report 4:3; the DVD duration is
  795.12 s. Its 19,877 encoded pictures use closed GOPs of at most 13 pictures,
  two B pictures maximum and the same rate/buffer envelope. The 955,922,432-byte
  ISO hash matches the manifest, and the entire DVD title decodes successfully.
- A separate 20-second prepared film reference completed the NTSC CLI pipeline
  and full DVD-title decode: 720×480, 30000/1001 fps, 4:3, closed GOPs of at most
  16 pictures, 448 kbps stereo AC-3, matching ISO/manifest hash. Both DVD decode
  logs contain only libdvdnav's ISO name-block diagnostic, with no video/audio
  decode errors; the decoder exits successfully with `-xerror`.
- The final benchmark binary passed a separate reference/offset/metrics/ISO
  smoke run. Local build version: `0.1.0-preview.8-quality.1`.

Physical-player playback has not been tested. The quality choice is supported
by these film samples and software checks; it does not promise perfection for
every source or standalone player.

The bitrate/mux/buffer envelope and AC-3 ceiling follow
[FFmpeg's DVD target](https://ffmpeg.org/ffmpeg.html#target).
Quality options are described by [FFmpeg](https://ffmpeg.org/ffmpeg-codecs.html)
and the [MPlayer/MEncoder manual](https://www.mplayerhq.hu/DOCS/HTML/en/MPlayer.html).

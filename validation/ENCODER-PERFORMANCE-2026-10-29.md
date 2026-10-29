# Encoder performance and pillarbox follow-up — 2026-10-29

Measured on an AMD Ryzen 7 9800X3D (8 cores / 16 logical processors), Windows,
FFmpeg 9.0.2 and HCenc 0.28.0.2 BEST. Inputs came from a read-only real film-scan
fixture: H.264, 1280×720, 18 fps, with baked-in black side borders. All benchmark
media, references, complete logs and ISOs remain in isolated local scratch space.
Deidentified measurements are in [the JSON results](ENCODER-PERFORMANCE-2026-10-29.json).

## One-title comparison

Each encoder ran three times, alternating order, without concurrent encoders.
PAL output is 720×576 at 25 fps, 4:3, two-pass video at 8 Mbps average / 9 Mbps
maximum and AC-3 audio. CPU use is process user+kernel CPU time divided by wall
time and all 16 logical CPUs. These are averages, not instantaneous peaks.

| 60-second test | FFmpeg elapsed | HCenc elapsed | FFmpeg average CPU | HCenc average CPU |
|---|---:|---:|---:|---:|
| Source decoding/scaling included | 2.02 s | 17.78 s | 77.7% | 22.5% |
| Same prepared DVD-resolution frames | 1.50 s | 15.72 s | 33.7% | 21.2% |

Numbers are medians. Elapsed time includes the production encode path: MPEG-2
passes, HCenc's lossless FFV1 preparation, audio/mux, full decode validation and
checkpoint hashing. Shared source analysis/reference creation, network staging,
quality metrics and DVD/ISO authoring are outside this timing. The prepared test
uses a scene beginning at 60 seconds; the source-path test uses the first minute.

HCenc took **8.8×** as long in the source-path test. Its MPEG-2 phase averaged
19.5% CPU, equivalent to about 3.1 fully occupied logical processors. FFmpeg's
two passes including source decoding/scaling averaged 81.5%. The user's CPU
observation was reproduced. HCenc logs confirm SMP is active and both passes
finish; changing IDLE to NORMAL priority left utilization near 20% and produced
overlapping runtime ranges. There is no evidence that an application CPU cap or
process priority explains the difference.

The source-path video streams were 55.42 MB for FFmpeg and 59.73 MB for HCenc.
HCenc's SSIM was slightly higher (0.976982 vs 0.973764), while PSNR was slightly
lower (43.60 vs 43.86 dB). On the other scene HCenc improved both metrics. Its
files were also larger. These measurements do not establish a universal quality
winner or compare equal-sized files; grain/motion still need visual review.

## HCenc concurrency

The same 1,500 prepared frames were encoded per title with private output and
log files. These numbers isolate native HCenc; normalization, audio and authoring
are outside the timing. All rows below use the existing IDLE priority.

| Concurrent titles | Batch elapsed | Aggregate frames/second | Average total CPU |
|---|---:|---:|---:|
| 1 | 13.83 s (median of 3) | 108.5 | 19.9% |
| 2 | 14.31 s | 209.6 | 40.6% |
| 4 | 17.43 s | 344.2 | 72.5% |

Four titles delivered about **3.2×** the throughput of one. Two/four-title batch
measurements are single runs, so small differences should not be overinterpreted.
NORMAL-priority batches gave similar results. The app now permits up to four
HCenc workers; the default stays two and FFmpeg retains its two-worker limit.
This improves multi-title jobs. A single video still uses one native encoder.

## Why this file retained pillarboxes

The earlier detector demanded that every frame fit exactly between x=160 and
x=1120. Across the complete 795.111111-second source, 12,591 frames extended
outside those exact bounds by only 1–2 pixels. The full observed horizontal union
was **[158,1122)** across 14,312 frames. Consequently the old code rejected the
crop and encoded/flagged the original frame as 16:9. The user's report was correct.

The detector now allows a narrow boundary variation and expands the kept image
to include every observed pixel. This file removes 158 pixels from each side,
keeping a 964×720 source rectangle. Its actual aspect controls fitting into a
4:3 DVD frame; a tiny amount of vertical padding handles the difference without
stretching or discarding the edge. Wider content and a small visible mark deeper
inside a side border still prevent cropping.

The detector/cache version changes so an old rejected crop is not reused for a
new encode. Completed outputs are preserved; a new job produces corrected media.
The complete failing source was re-encoded with FFmpeg. Both its MPG metadata and
DVD-demuxed title report **4:3**. Full DVD-title decoding passed; output duration
is 795.120 seconds (difference 0.008889 seconds), and the ISO hash matches the
manifest. The decoded frame was visually checked: the large baked-in side bars
are gone. The HCenc benchmark outputs also report 4:3 and author successfully.

## Regression checks

Unit tests/vet, media/engine/process/platform race tests, PAL/NTSC pipelines with
both encoders, the new edge-variation fixture, and an actual four-worker HCenc
job passed. The benchmark now explicitly chooses each backend; its older FFmpeg
case could inherit the HCenc default. Per-process CPU/wall timing and repeated,
alternating runs are now recorded.

The preceding Windows CI run also exposed Unicode working-directory enumeration
in the legacy DVD helper and missing HCenc discovery from a test-package working
directory. Windows helpers now use short paths or temporary junctions when
needed, retaining relative media references; cleanup removes only the alias.
CI exports the prepared HCenc directory explicitly. No fixture or GUI queue was
modified by these checks.

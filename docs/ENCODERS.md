# MPEG-2 encoder comparison

HCenc is the default video encoder for new jobs. It uses the BEST profile and
two-pass VBR at 8 Mbps average / 9 Mbps maximum. FFmpeg remains available in
Settings / Video encoder. Existing started jobs retain their previous encoder
and compatible saved encodes. Neither encoder is declared a universal quality
winner; review actual film scans, especially moving grain and fine texture.

## HCenc setup

HCenc 0.28 requires **32-bit** AviSynth and FFMS2 even though easyDVD itself is
64-bit. These components are installed separately from the normal portable app.
The pinned setup helper downloads and verifies all three components, preserves
their notices and creates a private runtime without changing system AviSynth:

```powershell
./scripts/prepare-hcenc.ps1 -OutputDir C:\Tools\easyDVD-HCenc
```

7-Zip is required for setup. Choose the resulting directory in Settings / HCenc
folder, or put it at `tools/hcenc` beside easyDVD.exe. Developers can run the
script without `-OutputDir` to use the detected `.tools/hcenc/bin` directory.
The runtime ZIP includes the same setup script; running it without `-OutputDir`
there installs into `tools/hcenc`, which the app detects automatically.
`EASYDVD_HCENC` and CLI `-hcenc-dir` are also supported. The directory contains
`HCenc_028.exe`, `AviSynth.dll` and `ffms2.dll`; mismatched 64-bit components are
rejected. Windows may need the Microsoft Visual C++ x86 runtime used by these
upstream components. A missing HCenc setup is reported explicitly; choose
FFmpeg in Settings to use the existing encoder instead.

FFmpeg prepares DVD-resolution frames in a lossless FFV1 intermediate, preserving
the chosen crop, aspect, cadence and interlacing. HCenc performs both VBR passes
in one invocation. Its log must confirm both passes and the full prepared frame
count. FFmpeg encodes AC-3 audio (or silence for silent sources), muxes the DVD
stream and fully decodes it before checkpointing. Temporary lossless files are
removed after each encode attempt; the space estimate includes simultaneous
lossless intermediates. HCenc's progress advances at preparation and encoding
boundaries, so the displayed percentage can remain steady during either pass.

## Automatic pillarbox removal

For 16:9 input, easyDVD checks every decoded frame before resizing. It removes
only symmetric black side borders that leave the full centered 4:3 picture.
A frame with picture content beyond the accepted border area prevents cropping.
Small variations at the 4:3 boundary are accepted by expanding the kept image to
include every observed pixel. Crop coordinates are rounded outward for chroma
alignment; the extra pixels are fitted with padding rather than stretched.
Content beyond this narrow edge margin still prevents cropping.
The check uses per-pixel luminance with a two-level tolerance above nominal
8-bit black for compression residue, retaining 16-bit analysis precision.
Uncertain or noisier borders keep the original frame. Letterbox bars are retained.
The cropped picture's actual display aspect controls scaling and DVD aspect;
rounding differences are padded rather than stretched. The border check adds a
decoding pass for new 16:9 encodes; completed compatible checkpoints are reused.

## Repeatable harness

`easydvd-benchmark` normalizes a short source exactly once to a YUV4MPEG reference.
Each encoder consumes those prepared frames. The directory contains the
reference and source hashes, encoded samples, elapsed encoding times, elementary
video sizes, PSNR/SSIM measurements, logs, and a basic authoring-test ISO per
successful encoder. A missing candidate is recorded as unavailable rather than
represented by fabricated results.

```powershell
./bin/easydvd-benchmark.exe -input C:\Scans\sample.mov -output C:\Benchmarks\trial-01 -seconds 10
./bin/easydvd-benchmark.exe -input C:\Scans\sample.mov -output C:\Benchmarks\trial-02 -mpeg2enc C:\Tools\mpeg2enc.exe -hcenc C:\Tools\HCenc.exe -ffms2 C:\Tools\ffms2.dll
```

Use `-repeats 3` for repeated runs with alternating encoder order and `-offset`
to select another scene. The report includes per-process wall/CPU time and
separates MPEG-2 passes from preparation/muxing/validation. For a complete short
source clip, `-source-pipeline -seconds 1000` includes its original decoding and
scaling costs. The normal benchmark uses the same DVD-resolution reference for
both encoders. CPU percentages are normalized to all logical processors.

HCenc can process up to four titles concurrently under Encoding workers;
FFmpeg retains its two-worker limit. A single title uses one HCenc process,
whose CPU utilization depends on its own threading. The default remains two
workers. Raising HCenc process priority does not remove a thread-scaling limit.

The benchmark's optional `-hcenc` / `-ffms2` paths are independent of the app's
encoder setting and require the same matching-bitness AviSynth runtime. The
harness does not switch the app's encoder automatically.

| Candidate | Initial settings | Integration considerations |
|---|---|---|
| FFmpeg software MPEG-2 | Two-pass, 8 Mbps average, 9 Mbps peak, DVD VBV/GOP | Available alternative; implemented and tested |
| mpeg2enc | DVD preset, quality 2, 8 Mbps maximum, progressive reference | Direct Y4M input; compare actual sizes because its rate-control target differs |
| HCenc | BEST profile, two-pass, 8 Mbps average, 9 Mbps maximum | Default for new jobs; private x86 AviSynth/FFMS2 runtime |

## Evaluation

Use several 10–30 second clips: stable grain, strong motion, pans, fine textures,
dark footage, fades, saturated colors, and scene changes. Include 4:3 SD and HD
scans and unusual stored frame rates. Avoid denoising and sharpening. Review
decoded results against the prepared DVD-resolution reference, not the HD original.

Compare at approximately equal file sizes as well as the initial settings. A
smaller sample with slightly lower PSNR is not automatically better, and grain
can make metric-only rankings misleading. Inspect side-by-side motion playback
and full-resolution crops. A larger encoder win must survive DVD authoring,
software navigation and physical-player playback.

The baseline synthetic smoke measurement is in `validation/encoder-baseline.json`.
It verifies the harness; synthetic test patterns do not determine the best encoder
for 8mm/16mm scans. Full three-encoder comparison on representative scans remains
a release/research task until the optional encoders and footage are provided.

References: [FFmpeg codecs](https://ffmpeg.org/ffmpeg-codecs.html),
[DVD targets](https://ffmpeg.org/ffmpeg.html),
[MJPEG tools](https://mjpeg.sourceforge.io/),
[HCenc manual](https://hank315.nl/files/HC_028/HC028.pdf).

# MPEG-2 encoder comparison

FFmpeg software `mpeg2video` is the implemented production default. It is not
declared the universal quality winner. Final choice should be based on actual
film scans, especially moving grain and fine texture.

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

HCenc requires a working, matching-bitness AviSynth installation and FFMS2 plugin.
These are optional benchmark tools and are not bundled into the app. Candidate
backends must be validated with the user's installed versions before using their
results. The harness does not switch the app's production encoder automatically.

| Candidate | Initial settings | Integration considerations |
|---|---|---|
| FFmpeg software MPEG-2 | Two-pass, 8 Mbps average, 9 Mbps peak, DVD VBV/GOP | Native current toolchain; implemented and tested |
| mpeg2enc | DVD preset, quality 2, 8 Mbps maximum, progressive reference | Direct Y4M input; compare actual sizes because its rate-control target differs |
| HCenc | BEST profile, 8 Mbps average, 9 Mbps maximum | DVD-focused; AviSynth/FFMS2 dependency and bitness requirements |

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

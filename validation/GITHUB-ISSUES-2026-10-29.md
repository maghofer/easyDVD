# GitHub issue validation — 2026-10-29

Historical implementation record: the encoder work for issue #7 was subsequently
superseded by the [FFmpeg-only quality profile](FFMPEG-QUALITY-2026-10-29.md).
The other issue fixes and regression checks remain in place.

Follow-up: [performance measurements and the additional H.264 pillarbox
regression](ENCODER-PERFORMANCE-2026-10-29.md). That report records the stricter
boundary check's failure on the user's additional fixture and its correction.

Addresses issues [#1](https://github.com/maghofer/easyDVD/issues/1),
[#2](https://github.com/maghofer/easyDVD/issues/2),
[#3](https://github.com/maghofer/easyDVD/issues/3),
[#5](https://github.com/maghofer/easyDVD/issues/5),
[#6](https://github.com/maghofer/easyDVD/issues/6), and
[#7](https://github.com/maghofer/easyDVD/issues/7).

## Behavior

- Scans ignore AppleDouble `._` sidecars and `__MACOSX` metadata directories.
  Excluding a failed file returns an otherwise valid, fully scanned job to Ready;
  including it again blocks the job. Genuine hidden videos remain eligible.
- Shift selects the inclusive range from the last ordinary selection, including
  rows outside the visible list. Switching jobs clears the selection anchor.
- Single file drops create one job. Multiple file drops offer a combined set or
  separate sets. Exact file selections persist through rescans/restarts, including
  equal basenames in different directories; unselected siblings stay out.
- Single-title jobs offer No menu in Job settings. DVD first play jumps directly
  to title 1 and playback stops at the end; no menu media/images are generated.
  Invalid multi-title requests are rejected before settings are locked.
- New jobs default to HCenc BEST, two-pass VBR, 8/9 Mbps, DVD GOP/VBV settings.
  FFmpeg normalizes lossless frames and handles AC-3/muxing. Logs must prove both
  passes and the complete prepared frame count. Legacy started jobs retain their
  FFmpeg profile and compatible checkpoints. Setup is described in
  [Encoder setup](../docs/ENCODERS.md).
- Automatic 16:9-to-4:3 conversion checks every frame's pixel bounding box before
  accepting symmetric black side borders. Analysis keeps 16-bit precision and
  allows two nominal 8-bit luma levels above black for compression residue.
  Wider picture content or a tiny visible border mark vetoes cropping. Actual
  content aspect controls scaling; rounding differences are padded.

## Completed checks

- All-package unit tests and `go vet` passed on Windows.
- Race tests passed for media, engine, store and subprocess packages.
- Linux compilation with the headless `ci` tag passed.
- Actual FFmpeg and HCenc PAL/NTSC pipelines passed, including title/audio
  properties, menu navigation generation, pause/restart encode reuse, publication
  recovery, isolated file selection/rescans, Unicode source/work paths and
  menu-free ISO output. Tests also author 36-title and 99-title discs.
- A final-frame transition to genuine widescreen and a 2×2 visible mark in a
  border both prevented cropping. Unit tests cover split diagnostic writes.
- libVLC 3.0.24 confirmed automatic first play and normal end-of-playback for the
  synthetic two-second disc and a real 6.875-second source. The check explicitly
  selects software decoding to avoid a local GPU-driver teardown crash.
- The pinned HCenc/AviSynth+/FFMS2 setup helper passed from both repository and
  simulated extracted-package locations. x86 PE checks reject mixed bitness.
- GUI selection/grouping/playback tests passed. Light and dark file-list
  renderings were generated; the light rendering was visually inspected.
- Windows GUI/CLI/benchmark builds, whitespace checks and source audit passed.

## Real media

A 291.0625-second, 1280×720 ProRes source at 16 fps with PCM audio was processed
independently as a single-file PAL, menu-free HCenc job. All 4,657 source frames
were checked; 160 pixels per side were removed. HCenc completed both passes for
all 7,277 normalized frames. The authored title is MPEG-2 720×576, 25 fps, **4:3**,
with stereo AC-3 at 48 kHz. DVD-demuxed duration is 291.080 seconds (difference
0.0175 seconds), and full DVD-title decoding completed without errors. The ISO
is 303,040,512 bytes. The shorter real DNxHD source also completed HCenc authoring
and autoplay validation.

All queues, runtime downloads, test media, builds and ISO files used isolated
scratch locations. The user's GUI queue was not opened or changed, and share
fixtures were read only. Physical-player compatibility and comparative encoder
quality remain outside these checks.

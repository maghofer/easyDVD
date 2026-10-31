# HCEnc Quality restoration

Date: 2026-10-30. Windows x64, FFmpeg/FFprobe 9.0.2, source-built authoring
helpers, externally installed AVStoDVD HCEnc 0.28.0.2, AviSynth 2.60 and its
32-bit FFMS2 C plugin. Test media, queues, builds and intermediate files were
isolated in the workstation scratch directory. The GUI queue was not used.

## Behavior

Quality now uses HCEnc BEST two-pass VBR with the supplied AVStoDVD 2.8.9 log's
8,500 kbps target, 9,000 kbps maximum, bias 80, DC precision 10, MPEG matrix,
AQ 1, luminance gain 1, SMP and idle priority. Aspect and progressive/interlaced
TFF/BFF follow the prepared source. Automatic GOP/scene detection remain active.

Balanced retains its FFmpeg arguments, 7,500/9,000 kbps video, 256 kbps AC-3,
default selection and cache identity. Quality has a new cache identity, so
unfinished jobs cannot reuse former FFmpeg Quality encodes.

FFmpeg still prepares the source using easyDVD's existing crop, Lanczos resize,
BT.601 conversion and frame/field handling. A temporary lossless FFV1 SD video
is served through FFMS2/AviSynth. This reproduces the HCEnc encoder parameters,
not AVStoDVD's entire DirectShow/ColorMatrix/Spline36Resize preprocessing chain.
Quality uses additional temporary disk space, reserved in the free-space check.

HCEnc/AviSynth/FFMS2 are external and not bundled. Installed AVStoDVD discovery
worked without environment overrides. Custom runtime binaries must be 32-bit.
Each encode runs private temporary HCenc.exe/avisynth.dll copies, preventing
the installed encoder's default HC.ini from changing the selected parameters.

## Verification

- `go test ./...` passed after the implementation changes.
- `go vet ./...` passed.
- `git diff --check` passed.
- Windows GUI, CLI and benchmark builds succeeded using the normal build script.
- Real `TestPipeline` passed for Balanced/PAL, Balanced/NTSC, Quality/PAL and
  Quality/NTSC. This exercises progressive/interlaced video, selected audio,
  silence, source staging, text/thumbnail menus, authoring, DVD demux/decode,
  profile/audio constraints, ISO hashes and checkpoint recovery. Quality now
  explicitly requires an HCEnc second-pass report and checks temporary cleanup.
- A separate two-worker Quality job used TFF and BFF sources and an accented
  work-directory path. It completed; FFprobe reported `tt` and `bb` respectively,
  both at 720x576/25 fps.
- An explicit nonexistent HCEnc runtime produced exit 1 for
  `-check-tools -profile Quality`, while Balanced diagnostics returned exit 0.
- A two-second Quality benchmark authored an ISO, labeled its encoder HCEnc and
  recorded a nonzero MPEG-2 encoding duration for the single two-pass process.

The local AVStoDVD FFMS2 build requires `LoadCPlugin`, whereas other FFMS2 builds
use `LoadPlugin`; both loading forms are supported. HCEnc 0.28.0.2's `-noini`
also disables the explicit INI, so isolated executable copies are used instead.

## Full source from the supplied log

The complete 94.68-second 1080p25 ProRes HQ clip completed an isolated Quality
CLI job and produced one menu-free DVD ISO. The HCEnc report confirmed BEST,
8,500/9,000 kbps, automatic 15-picture GOPs, progressive TFF, MPEG adaptive
matrix, AQ 1 and luminance gain 1.

| Measurement | Result |
|---|---|
| Encoded source frames | 2,367 |
| HCEnc first pass | 19.69 s |
| HCEnc second pass | 30.08 s |
| HCEnc elapsed time | 50.13 s |
| Reported average video bitrate | 7,120 kbps |
| MPG format duration | 94.688 s |
| Video | MPEG-2, 720x576, 25 fps, progressive, 16:9 |
| Audio | AC-3, 448,000 bit/s, stereo, 48 kHz |
| Authored ISO size | 92,129,280 bytes |

The encoded MPG passed the production full decode/hash check. The authored
ISO's complete first title also decoded with FFmpeg `-xerror` without errors.
Temporary HCEnc input directories were removed after success, including when
the job retained its normal work artifacts.

These checks establish encoder selection, parameters and pipeline compatibility.
They do not claim a subjective visual-quality result or physical-player testing.

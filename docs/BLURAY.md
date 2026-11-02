# Blu-ray (experimental)

Click **Blu-ray** in the job header and choose BD-25 or BD-50. Encoding presets
can select Blu-ray for future jobs. Each folder remains
its own set, with whole videos distributed across discs. Finished sets contain
`BD-001-of-001.iso` style filenames. Burning remains external.

Software playback and filesystem checks are covered by automated validation.
Standalone-player compatibility has not been verified. This creates conventional
SD/HD Blu-ray BDMV, not UHD Blu-ray or AVCHD on DVD. HDR inputs still require an
external SDR conversion. SDR sources above HD are reduced to HD.

## Video and quality

The video format follows each title rather than the DVD PAL/NTSC setting.
Standard SD and 720 sources keep their resolution class. Nonstandard dimensions
fit inside the selected standard raster, with bars as needed and no stretching.
Confirmed pillarboxing uses the existing border detection.

| Picture class | Disc modes |
|---|---|
| SD | 720×576 at 50 fields/s; 720×480 at 59.94 fields/s; 4:3 or 16:9 |
| 720 | 1280×720 progressive at 23.976, 24, 50 or 59.94 fps |
| 1080 | 1920×1080 progressive at 23.976/24 fps, or interlaced at 50/59.94 fields/s |

Progressive 25/29.97 fps at 1080 is carried with interlaced signaling. Progressive
50/59.94 fps at 1080 becomes true interlaced video, retaining the motion samples.
Existing interlaced footage keeps fields where its target allows them. Unsupported
frame rates are normalized by frame repetition/dropping, without speeding up
playback. At 720, 25/29.97 fps becomes 50/59.94 fps with repeated frames.

Both profiles use FFmpeg/libx264: Balanced is medium/CRF 20, Quality is slow/CRF
18. New encodes use High@4.1, 8-bit 4:2:0, closed GOPs of at most one second,
four slices, HRD signaling, a 38 Mbps video ceiling and a 30 Mbit video buffer.
HCEnc and AviSynth are used only for DVD Quality. Disc capacity never lowers
the chosen Blu-ray quality; actual prepared file sizes determine packing.

## Verified stream copying

Copying is automatic when the full-stream verifier establishes compatibility.
Codec/profile labels alone do not authorize copying. File details and the job log
show the video and audio decisions, including the reason for encoding.

Verifier v1 deliberately supports a conservative subset: progressive 720 or
1080p23.976/24 AVC with Main/High level 4.0/4.1 signaling, one reference,
I/P pictures, repeated parameter sets, access-unit delimiters, at least four
slices, short closed IDR GOPs, constant packet timing and single-buffer HRD
signaling. It checks every parsed header and the packet bitrate/buffer envelope.
Missing or unfamiliar evidence triggers re-encoding. Valid Blu-ray streams with
B pictures or interlaced signaling are therefore also re-encoded in this version.
This is a compatibility policy, not a Blu-ray certification claim.
Inspection stops as soon as incompatibility is established. Candidates accepted
for copying still require inspection through the end of the stream.

Cropping, scaling, rotation or color normalization requires encoding. Compatible
video is remuxed into fresh disc media. Each prepared title is fully decoded and
checked before it becomes a reusable checkpoint.

One audio track is selected with the existing default-track behavior. Compatible
48 kHz AC-3 up to 5.1 is copied when its timing covers the picture. Other audio
becomes AC-3, preserving mono/stereo/surround up to 5.1 and downmixing larger
layouts. Short audio is padded; silent inputs receive silent stereo audio.
New encodes use 192 kbps mono, 448 kbps stereo or 640 kbps multichannel.

## Menus and recovery

The existing menu editor, presets, fonts, backgrounds, text/thumbnail layouts and
density controls are shared. Blu-ray artwork is rendered at 1920×1080; a 4:3
composition is centered inside that canvas without stretching. Buttons use HDMV
interactive graphics, with the same cyclic arrow navigation as DVD. Blu-ray
thumbnail extraction uses a larger 640×480 canvas.

Individual titles return to their page with their button selected. Play All uses a non-seamless playlist
through the disc's titles and returns to page one. No-menu playback plays the
titles in order and stops. Mixed formats and audio layouts remain separate clips.
The application keeps its 99-source-title limit per disc.

Open the ISO as a Blu-ray disc in a player with HDMV menu support. Opening an
individual media file, or using a player that selects a movie playlist directly,
bypasses the authored menus. [MPC-HC explicitly does not support Blu-ray menus](https://github.com/clsid2/mpc-hc/releases/).
Software validation includes libbluray navigation and libVLC menu playback,
selection graphics, title selection and return. PotPlayer menu playback has not
been independently verified. Single-video jobs keep automatic playback by default.

Workers are chosen in application Settings. Blu-ray uses a separate per-process
thread budget, up to 16 threads divided across concurrent titles, leaving CPU
headroom. DVD retains its existing eight-thread ceiling. The x264 presets, CRF
targets and compatibility limits are unchanged.
Auto starts from one worker per four logical CPUs for Balanced, or per eight for
Quality, capped at four. Pending titles, memory and scratch space further limit
this estimate. An explicit worker setting overrides the automatic estimate.

Packing budgets 64 MiB per disc plus 8 MiB per title with menus, or 1 MiB per
title without menus. Capacities are conservatively 25,000,000,000 and
50,000,000,000 bytes. Final ISO size is authoritative; overflow moves whole
titles and regenerates authoring without changing video quality.
Workspace checks reserve the remaining prepared media, BDMV and ISO copies of the
projected disc set with those structure reserves, staging, final output when it
shares the work volume, and safety headroom. Short jobs do not reserve the full
blank BD-25/BD-50 capacity. Existing DVD workspace reservations are unchanged.

Encoding checkpoints include the format, versioned processing decisions and
profile. Menu edits or capacity changes reuse prepared titles. DVD encoding and
generation identities remain compatible with existing saved queues.

## Toolchain and CLI

The package includes an isolated CPython runtime, patched bdmvkit 0.7.0 for HDMV
authoring, and udfkit 0.1.0 for UDF 2.50 images. System Python and Java are not
needed. FFmpeg/FFprobe remain external and must include libx264 and the required
filters/bitstream filters. Requirements are checked per job format.

```powershell
easydvd-cli -format Blu-ray -media BD-25 -output OUTPUT -work WORK -state QUEUE.db SOURCE
easydvd-cli -format Blu-ray -check-authoring-tools
```

Paths above are placeholders. Use absolute work/output paths and an isolated
state file for automated tests. `-media` defaults to BD-25 for Blu-ray and DVD-5
for DVD. Development runs can select a prepared runtime with `EASYDVD_BLURAY`.
Prepare it with `scripts/prepare-bluray-runtime.ps1 -Destination ... -Scratch ...`.
The pinned downloads, deterministic patch recipe and licenses live in
`scripts/bluray`; release sources include the exact upstream archives and recipe.

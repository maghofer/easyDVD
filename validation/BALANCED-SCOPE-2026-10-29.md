# Balanced profile and Scope playback — 2026-10-29

Balanced is the default for new jobs. The job header has an always-visible
Balanced / Quality choice, saved per job and disabled once processing locks
the settings. Global settings also provides the new/pending-job default.
CLI and benchmark accept `-profile Balanced` or `-profile Quality`.
Existing jobs without a profile retain Quality, including their compatible
saved encodes. Preferences loaded over current defaults adopt Balanced for new
jobs. Profile-specific cache keys and size estimates prevent cross-profile
checkpoint reuse and incorrect disc/work-space estimates.

## Selected settings

| | Balanced | Quality |
|---|---|---|
| Video target / peak | 7,500 / 9,000 kbps | 9,000 / 9,000 kbps |
| Stereo AC-3 | 256 kbps, 48 kHz | 448 kbps, 48 kHz |
| Minimum quantizer / lambda | 2 / 236 | 1 / 118 |
| Macroblock decision / intra VLC | RD / enabled | RD / enabled |
| Trellis and additional motion search | FFmpeg defaults | Existing full Quality analysis |

Both use two passes, MPEG-2 Main Profile/Main Level, YUV420P, the same DVD VBV,
closed GOP limits, aspect, scaling and field handling. Neither applies denoising
or sharpening. Packing uses the chosen profile and actual encoded sizes; it
does not lower the selected target to force more content onto a disc.
Exact shared and Quality options are in [the encoding guide](../docs/ENCODERS.md).

## Measurements

Windows, Ryzen 7 9800X3D, 8 cores / 16 logical processors, FFmpeg 9.0.2.
Thirty-three candidate/scene trials used identical 20-second progressive PAL
references. Movement and grain share an H.264 film source; ProRes and Scope are
separate sources. The inexpensive RD decision consistently improved the old
profile, while trellis and the full motion searches cost much more time.
The [deidentified JSON](BALANCED-SCOPE-2026-10-29.json) records the candidates,
settings, metrics and timings. Raw media, file paths and logs remain local.

| 20-second scene | Old fast profile PSNR / SSIM | Balanced PSNR / SSIM | Quality PSNR / SSIM |
|---|---|---|---|
| Movement | 43.899 / 0.973753 | 44.034 / 0.974568 | 45.046 / 0.979460 |
| Grain | 44.122 / 0.976103 | 44.298 / 0.977127 | 45.114 / 0.980747 |
| ProRes scene | 45.632 / 0.981661 | 45.752 / 0.982153 | 48.609 / 0.989908 |

Balanced video rates were 6.578, 7.501 and 4.395 Mbps respectively, versus
6.737, 7.808 and 4.383 Mbps in the old fast profile. The quantizer floor lets
simple material use fewer bits instead of forcing the nominal target. These
metrics support decent quality with improved coding efficiency on these scenes;
they are not a universal subjective quality guarantee.

An earlier candidate allowed quantizer 1 in Balanced. On the complete clean
Scope film it used more bits than Quality's slower analysis needed. Retaining
the old floor of 2 reduced that unnecessary bitrate substantially while still
improving the old fast profile's measured quality in all three film scenes.

Three sequential production runs per selected profile used the same complete
60-second H.264 source, with source decoding/resizing, both passes, AC-3/mux,
full decode validation and hashing. Reference/crop analysis, quality metrics
and ISO authoring are outside the timing; each run also authored a test ISO.

| Median | Balanced | Quality |
|---|---:|---:|
| Encode path | 2.96 s | 12.47 s |
| Elementary video bytes | 54,033,007 | 67,357,990 |
| MPG bytes | 56,985,600 | 71,946,240 |

Balanced was approximately **4.2 times faster**, with **20% less video data**
and **21% smaller MPG files** on this sample. The old fast profile's earlier
2.02-second timing remains historical; Balanced deliberately spends a little
more effort on RD decisions. Sizes and speed depend on the source.

## Scope investigation

The supplied full source is 1920×796, square pixels, 24 fps, 2.412:1, lasting
422.125 s. Both profiles were encoded and authored with the normal menu path.
The complete titles decode successfully through the DVD demuxer with `-xerror`,
and both output ISO hashes match their manifests. Sources were read only;
isolated CLI queues were used, preserving the user's GUI queue/preferences.

Both MPG and DVD titles report 720×576, 16:9 display aspect and 64:45 sample
aspect. The conversion scales the source to **720×424**, then adds **76 lines
above and below**. It does not add horizontal padding or crop the source.
At the sampled title frame, visible bounds are 717×424 at x=2, y=76 in both
profiles; the few dark edge pixels belong to the source vignette/compression,
not the large side margins in the playback screenshot.

The screenshots show the same picture proportions at different scales:
one picture is 1,868×776 inside a 1,872×966 viewport; the smaller one is
1,721×716 inside a 1,869×967 viewport. Those picture ratios are both about
2.41:1. Fitting the complete 16:9 DVD frame by height in that approximately
1.93:1 window leaves side bars. The original 2.41:1 frame can instead fit by
width. The visible windowboxing follows from the compliant DVD frame and
player fit behavior, rather than encoded side padding.

A compatible DVD cannot signal a 2.41:1 display aspect. Player zoom/cropping of
the letterbox can fill a wider playback window. The encode retains the full
picture and correct proportions for ordinary 16:9 playback.

| Complete Scope film | Balanced | Quality |
|---|---:|---:|
| Authored DVD duration | 422.12 s | 422.12 s |
| ISO bytes | 162,930,688 | 317,564,928 |

## Checks

- All-package functional tests and vet; headless CI-tag checks.
- Both profiles in actual PAL/NTSC pipelines, including frame rate, profile/
  level, pixel format, audio rate, matching interlace and cross-standard conversion.
- New Scope regression checks every decoded frame's horizontal picture bounds
  in both profiles/standards, using an edge-marked 1920×796/24 fps source.
- Header selection/persistence/default isolation and started-job locking;
  legacy profile/cache migration; profile-aware estimates.
- Existing scan/exclusion, menus, autoplay, Unicode folders, recovery and
  36/99-title authoring checks. Header previews inspected at normal window size.
- Race checks on media/model/engine/process/platform/store and the job-header
  tests pass. A broader UI race run reports existing timer-driven hover/menu
  preview test races with Fyne's headless driver; the same failures reproduce
  on the unmodified preceding commit. They are outside this profile change.

No physical standalone DVD player was tested.

References: [dvdauthor video attributes](https://github.com/ldo/dvdauthor/blob/master/doc/dvdauthor.sgml),
[FFmpeg codec options](https://ffmpeg.org/ffmpeg-codecs.html),
[scaling/aspect filters](https://ffmpeg.org/ffmpeg-filters.html#scale), and
[DVD target defaults](https://ffmpeg.org/ffmpeg.html#target).

# Local real-media validation

Date: 2026-10-28. Windows host, real SMB input, external FFmpeg/FFprobe 9.0.2
and bundled DVD authoring tools.

The user supplied eight folders containing 126 videos (61,946,132,262 bytes).
Sequential inspection returned metadata without probe errors for every file.
The collection includes H.264, ProRes and DNxHD; 16/18/20/24/25 fps;
960x720, 1280x720, 1920x1080, 2048x1556 and 1920x796; sources both with and
without audio. Probing alone does not establish complete source integrity.

The two numbered collections contain 74 and 42 files, approximately 8.5 hours
each. Both were added independently through the GUI's native multi-folder
picker. Progressive scan counts were visible, both reached Ready with their
expected counts, and filenames appeared in natural order. Encoding was left
stopped for these long jobs.

Three short folders completed the actual PAL/DVD-5/text-menu pipeline with
local staging: 1080p24 DNxHD without audio (6.875 seconds), 1080p25 ProRes with
audio (94.68 seconds), and 2048x1556 ProRes24 with audio (67.333 seconds).
Each produced one ISO, a manifest, menu preview and log. ISO SHA-256 hashes
matched the manifests. Encoded title inspection confirmed MPEG-2 720x576 at
25 fps, 16:9/4:3 presentation as appropriate, and stereo AC-3 at 48 kHz.
Duration differences were under 0.04 seconds. Every authored title was also
probed and fully decoded through FFmpeg's DVD-Video demuxer without errors.

All test queues, metadata reports, retained intermediate media, test ISOs and
process-local temporary files are in an isolated scratch directory. The GUI's
saved work and output preferences were retained.
Detailed inventories and filenames remain in the local scratch directory rather
than being copied into the repository. Sources on the share were only read.

## Long-job recovery and navigation regression

The 74-title set initially failed on DVD 2 after encoding. The authoring log
ended with `ERR: Can only have 128 commands for pre, post, and cell commands.`
The generated jump-pad navigation table exceeded the per-PGC command limit
when that disc contained 36 titles. Encoding itself had completed.

Navigation now uses direct VMGM global-title jumps and page-local Play All
dispatch, with no generated jump-pad table or separate dispatcher PGCs. The DVD
template generation key changed to v3; the encoding profile key did not change.
Explicit fatal tool diagnostics are placed before earlier warning context so
the GUI's short error summary shows the cause.

Recovery completed using all 74 saved encodes. Their checkpoint SHA-256 values
matched the original queue, and the cumulative encoding-command count stayed at
148 (two original passes per video): recovery performed no video encoding.
The final strict-order layout has 15, 36 and 23 titles. All 74 authored titles
were probed through the DVD-Video demuxer; PAL MPEG-2/AC-3 properties matched and
duration differences from source metadata stayed within 0.023 seconds.

| DVD | ISO bytes |
|---|---:|
| 1 | 4,500,934,656 |
| 2 | 4,110,227,456 |
| 3 | 4,140,832,768 |

The reduction from approximately eight estimated discs to three final discs
reflects 12,744,351,744 bytes of actual encoded titles versus the conservative
fixed-rate pre-encode estimate. No files were omitted, durations were retained,
and packing did not alter the encoding settings.

Validation of the fix: unit tests and vet for all packages; race tests for media,
engine and subprocess handling; real PAL/NTSC integration pipelines; actual
authoring of 36 titles/four pages and 99 titles/99 pages; and the libVLC three-title
fixture covering selection/return, Play All, Next page and second-page return.

Disconnected-share behavior, full-capacity media, physical-player compatibility
and comparative encoder quality remain unvalidated by this run.

## ISO-only output and application logs

The completed 74-title set was migrated without touching the ISO files: its log
and manifest moved to application data, and nine PNG previews (564,123 bytes)
moved to the temporary work cache. The set directory now contains exactly three
ISO files, with their original byte sizes.

New jobs append logs directly under the application data `logs` directory and
checkpoint manifests under `jobs/JOB_ID/manifest.json`. PAL and NTSC integration
tests, including a race-instrumented run, verify ISO-only publication and restart
recovery after the output rename without an output-side manifest.

Native desktop validation confirmed Open output opens the ISO folder and
Settings / Open logs folder opens the persistent log directory. The opener
resolves filesystem handles before invoking Explorer: packaged desktop hosts
such as Codex can redirect AppData, which otherwise makes Explorer report that
the logical path is unavailable. Standalone app defaults remain the user's local
application-data folder.

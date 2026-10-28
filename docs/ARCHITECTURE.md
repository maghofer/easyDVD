# Architecture

The Fyne desktop app and headless CLI use the same engine. There is no server,
network API, telemetry, automatic updater, or dependency on a browser runtime.
The only application download is an explicit user-requested FFmpeg setup action.

## Components and data flow

- `model`: settings, folder jobs, source videos, disc plans and stage artifacts.
- `store`: transactional SQLite snapshots, persisted ordering and preferences.
- `engine`: queue scheduling, bounded work, commands, checkpoints and publishing.
- `media`: FFprobe inspection, explicit FFmpeg argument generation, authoring XML,
  tool discovery, media validation and hashing.
- `packing`: natural ordering, sequential packing, best-fit decreasing, bounded
  branch-and-bound search, title-count and conservative capacity constraints.
- `menu`: Go-rendered backgrounds, television-safe button regions, highlight and
  activation palettes, preview images and spumux XML.
- `process`, `fsops`, `platform`: hidden subprocesses, Windows Job Objects,
  cancellation, isolated network I/O, native multi-folder selection and disk space.
- `ui`: virtual lists, label editing, settings, previews and job diagnostics.

Pipeline: discover → probe → validate source/checkpoints → stage and encode →
pack actual sizes → render/multiplex menus → author DVD → create ISO → validate
size/hash → publish the complete set.

## Concurrency and persistence

The coordinator processes one folder job at a time and continues after blocked
jobs. Two folder-discovery helpers and two probes may run globally. A set uses
one or two encoders and one staging producer with one-file prefetch. Graphics
and thumbnails are prepared before final menu encoding. Final menus depend on
the encoded layout and total disc count.

All long operations run away from Fyne's event goroutine. Progress is coalesced
and marshaled with `fyne.Do`; lists are virtualized. Snapshot copies do not
serialize entire jobs just to update the GUI. SQLite snapshots occur at stage
boundaries and batches of scanned files, not every encoded frame.

Discovery and source stat/copy/hash/free-space operations run in helper
processes. A Windows Job Object contains each tool/helper tree. Canceling a
blocked SMB operation terminates its helper instead of waiting for a blocked
Go filesystem call. The destination paths for work remain user-configurable;
the work directory should always be on a local drive.

A source fingerprint uses absolute path, size and nanosecond modification time.
Encoding checkpoints also include a versioned encoding-profile key, actual size,
and SHA-256 of the completed encode. Settings and source changes invalidate
dependent artifacts. Menu labels and packing do not invalidate video encoding.
Generation keys include artwork/layout inputs, so altered assignments regenerate
all affected disc numbering. Recovered ISOs are hashed before reuse.

## Media tool discovery and setup

Authoring tools stay bundled. FFmpeg/FFprobe are external in the normal package.
Explicit environment/saved folder choices take precedence; automatic discovery
then checks PATH, a managed private download, and an optional offline sidecar.
Every candidate is validated as a pair, including the supported release family
and required encoders, decoders, filters and muxers. An incompatible PATH entry
does not prevent using a compatible private or offline installation.

GUI discovery runs in the background. Missing/incompatible FFmpeg triggers a
setup dialog; queue start and input probing wait for a usable pair. Changing
tools is rejected while scans or jobs are active. CLI overrides are per run;
explicit setup choices and downloads save the chosen folder for future launches.

The runtime embeds `scripts/tools.lock.json`, sharing the recommended version,
HTTPS URL and archive/executable SHA-256 pins with the build scripts. Download
and extraction are cancelable. A verified pair is validated in staging and then
published atomically into a private cache. Existing tool versions are never
overwritten. Discovery itself never initiates a download.

## DVD structure

Menu appearance is a value in each settings snapshot. Named presets are stored
in SQLite, while imported backgrounds/fonts are validated and copied to immutable
content-addressed assets under application data. A single Go renderer produces
square-pixel, single-page previews in memory and DVD-size artwork/highlight masks
for authoring. Debounced preview requests cancel obsolete thumbnail work and
publish UI changes through Fyne's dispatch. Appearance changes do not alter the
video encoding cache key.

Widescreen menus use both widescreen and letterbox subpicture streams, multiplexed
separately by spumux, and disable pan-and-scan. Menu aspect is independent of each
source video's presentation aspect.

Each source file is one title in its own titleset, preserving title-specific
4:3/16:9 attributes. A disc is capped at 99 titles. Shared VMGM pages contain
title buttons, Play All and page navigation. Buttons use direct global-title
jumps. Each page dispatches Play All transitions for only its own titles, keeping
command counts bounded by the titles-per-page setting. Generated jump pads and
extra dispatcher PGCs are unnecessary, including at 99 one-title pages. Titles call
their originating page on completion; a playback-mode register selects the
Play All chain. The title-set root menu returns to the correct page.

Packing reserves 32 MiB per disc and 256 KiB per title for structure/menu/mux
overhead. Capacities are conservatively 4,700,000,000 and 8,540,000,000 bytes.
The final image byte size overrides the estimate. Overflow moves a whole title
and regenerates the layout; video encoding is unchanged. The reported optimality
proof concerns this safe packing budget, not arbitrary custom DVD layouts.

Publishing copies verified ISOs to a private output directory. Persistent job
manifests live in the application data directory and are checkpointed before the
atomic output-directory rename, allowing recovery without output sidecars.
Logs are appended directly in the application logs directory. Temporary menu
previews stay under the work directory and can be regenerated after removal.
The published set contains only ISO files.

The local manifest recovers a crash between the final rename and the SQLite
update. Interrupted partial directories are not presented as completed output.

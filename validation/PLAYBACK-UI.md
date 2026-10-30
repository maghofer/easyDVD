# Playback options and compact header

Validated on 2026-10-30 on Windows. Generated media, isolated queue databases,
temporary builds and UI previews were placed in isolated local scratch space.
The user's GUI queue and read-only Exchange media were not used for these tests.

- Header: removed menu, encoding-profile controls and PAL/disc/packing text;
  Job settings and Open output share the lower action row. Tools is labeled `!`.
- Job settings: Balanced/Quality radios persist independently of defaults.
  No menu can be toggled for single- and multi-video jobs. The menu editor
  disables immediately when No menu is checked, including after reopening.
- Playback defaults: one included video selects no menu; multiple included
  videos select menus. Explicit overrides persist and survive inclusion changes.
- `go test ./...`, `go vet ./...` and `git diff --check` passed.
- Real toolchain integration: generated three two-second clips, with 4:3,
  16:9 and 4:3 aspect ratios, authored an autoplay DVD ISO and decoded every
  title using FFmpeg's DVD input. No visible menu assets were generated.
- The libVLC navigation checker observed titles **1 → 2 → 3**, then playback
  ended without displaying a menu. Single-title autoplay regression tests
  also passed for PAL and NTSC.

Autoplay across aspect-specific titlesets uses command-only VMGM navigation
chains, following the domain rules in the
[dvdauthor documentation](https://dvdauthor.sourceforge.net/doc/dvdauthor.html).
Each chain contains one jump to keep command counts bounded.

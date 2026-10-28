# Parameter-based menus

Validated on 2026-10-28 with FFmpeg 9.0.2 and bundled authoring tools.

- Job Menu button and global Settings entry open the same parameter window.
- Renderer tests cover PAL/NTSC, 4:3/16:9, maximum text/thumbnail density,
  large fonts/spacing, nonoverlapping button bounds, custom fonts, colors,
  image backgrounds, dimming and display-correct preview dimensions.
- Single-page preview creates no files. Local development benchmark:
  approximately 0.414 milliseconds per default text page, excluding debounce.
- UI tests cover applying a pending job's appearance without changing global
  defaults, saving/loading presets, resetting defaults and protecting started
  jobs. The dialog was rendered and visually inspected; the native desktop
  session was locked during this run, so file-picker interaction was not tested.
- SQLite preset tests cover restart persistence, case-insensitive replacement
  and rejected invalid updates. Asset integration imports a background/font,
  removes the originals, restarts, and renders successfully from retained assets.
- Real PAL/NTSC pipelines pass with ISO-only output and publication recovery.
  The NTSC fixture uses widescreen menus and verifies both widescreen and
  letterbox highlight streams in the multiplexed menu video.
- Actual 36-title/four-page and 99-title/99-page authoring still passes.
- A persistent PAL widescreen three-title ISO passed the libVLC navigation
  fixture: selection/return, Play All, Next page and second-page return.
- Race-instrumented menu, model, store, engine and real integration tests pass.

Generated UI captures, benchmark fixtures and test ISOs are under
an isolated scratch directory. The completed user ISO set was not re-authored.
Physical-player validation remains pending.

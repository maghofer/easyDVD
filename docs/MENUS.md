# Menu settings

Choose **Menu** on a folder job, or **Settings → Menu settings** for defaults.
The window contains parameters and an automatic preview, with DVD and page
selectors. It is not a freeform editor. Only the requested page is rendered;
typing and sliders use a short debounce. Thumbnail extraction happens in the
background and is cached for the lifetime of the window.

Defaults are 4:3, the existing dark generic background, Sans, light text, a blue
selection outline, 20-pixel labels, a 28-pixel heading and 12-pixel line spacing.
Sizes refer to a 576-pixel-high display canvas. Fonts and spacing fit down when
needed to keep every title and navigation button inside television-safe bounds.
Long labels are shortened visually; source files and stored labels are unchanged.

- **Menu aspect:** 4:3 or 16:9, independent of the video titles. Widescreen menus
  include separate highlights for widescreen and letterboxed playback.
- **Layout/density:** text (1–12 titles) or thumbnails (1–6).
- **Background:** PNG or JPEG, centered to fill the menu without stretching;
  edges may be cropped. Clear restores the generic background. Dimming improves
  text contrast without modifying the imported image.
- **Font:** included Sans/Monospace, or a custom TTF/OTF file. Custom fonts use
  the selected face for both heading and labels. Font and highlight colors accept
  hexadecimal values or the color picker.
- **Presets:** Save preset stores appearance, layout and titles per page. Reusing
  a name updates that preset. Presets are stored with the queue in SQLite.

Import limits are 32 MB / 24 million pixels for backgrounds and 20 MB for fonts.
Validated assets are copied into local application data so a removed or changed
original does not break queued jobs or presets. Keep these assets with application
data; they are persistent, unlike generated menu graphics in the work directory.

**Apply to job** updates a pending job. **Save as defaults** updates new and
unstarted jobs. Started/finished jobs keep their snapshot; their Menu window can
preview alternative settings and save presets without rewriting their ISO files.
Nothing changes until an Apply/Save action is selected. Finished output folders
continue to contain only ISOs.

The CLI accepts `-menu-aspect 4:3` or `-menu-aspect 16:9`.

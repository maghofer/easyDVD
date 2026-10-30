<p align="center">
  <img src="internal/ui/assets/logo-retro-gold.png" alt="easyDVD" width="420">
</p>

easyDVD is a Windows app for turning folders of video files into DVD-Video ISO
files with menus. Add your folders, choose your disc and menu settings, and let
the queue process them into DVD sets.

**[Download for Windows (64-bit)](https://github.com/maghofer/easyDVD/releases)**

The current release is a preview. Choose the `easyDVD-…-win64.zip` download;
the separate sources ZIP is not needed to run the app.

## What it does

- **Batch processing:** queue several folders at once. Each added folder becomes
  its own DVD set, including its subfolders. Each video file becomes a DVD title.
  Drop individual videos too; multiple files can share a set or become separate jobs.
- **Automatic disc layout:** spread videos across as many discs as needed. Keep
  filename order, or choose space efficiency to make better use of disc space.
  Videos stay whole; a file that cannot fit on one disc is reported.
- **Simple, customizable menus:** choose text or thumbnails, set backgrounds,
  fonts and colors, and save presets. Menus include title selection, page
  navigation and Play All.
- **DVD format choices:** PAL or NTSC, DVD-5 or DVD-9, with 4:3 and 16:9 video
  handled automatically. Confirmed 4:3 footage inside black 16:9 side borders is
  cropped without stretching. Single-title jobs can start playback without a menu.
- **Pause and resume:** save the queue and continue later, reusing completed
  encodes. Errors are shown against the affected folder so other sets can continue.

## What it doesn't do

easyDVD focuses on batch conversion and automatic DVD layout. It is not a full
DVD authoring suite with a freeform title or menu designer.

- No timeline editing, trimming or transitions.
- No freely positioned menu buttons, animated menus or custom navigation logic.
- No subtitle authoring or multiple audio tracks.
- No disc burning. It creates ISO files to burn with your usual DVD burning
  software or open in a compatible player.

DVD output is standard definition. HDR video needs to be converted to SDR before
using easyDVD.

## Getting started

1. Extract the complete Windows ZIP and open **easyDVD.exe**. Keep the `tools`
   folder beside it.
2. Drag folders into the window or use **Add folders**. Review the videos and
   choose an output folder and a local work folder with enough free space. You can
   also drop video files. Hold Shift while selecting files to select a range.
3. Choose your TV standard, disc capacity and menu style, then select
   **Start / resume**. Finished ISO files appear in your output folder.

Save **Default settings** for future jobs. Each new job takes a copy; existing
jobs keep their settings. Use **Job settings** to override the TV standard,
disc capacity, output folder and menu options for one job. Menu layout and
titles per page live in **Menu**.

Job tiles show status and completion percentage. Click a tile to select it;
Ctrl-click toggles additional jobs and Shift-click selects a
range. Files use the same selection behavior. Use **Remove selected** or **Delete**.
The X on a tile removes that job. Removing active jobs asks to stop them first;
source and output files are kept. Overall progress appears in the bottom bar.

FFmpeg performs two-pass MPEG-2 encoding. **Balanced** is the default, targeting
7.5 Mbps video with 256 kbps AC-3 audio for speed and smaller files. Choose
**Quality** in Job settings for 9 Mbps video, 448 kbps audio and more detailed
analysis. easyDVD adds discs instead of reducing the chosen profile's bitrate to
squeeze more footage onto one disc. FFmpeg remains an external dependency;
first run can locate it or download a private copy. See [Encoding quality](docs/ENCODERS.md).
Single-video jobs default to automatic playback without a menu; multi-video
jobs default to a menu. Override either choice in **Job settings → Playback**.
With **No menu**, every disc plays its included videos in order and stops at
the end. Menu editing is disabled until menus are enabled again.

For more on menu appearance and presets, see [Menu settings](docs/MENUS.md).
Report problems or suggest changes through [GitHub Issues](https://github.com/maghofer/easyDVD/issues).

---

easyDVD is [MIT licensed](LICENSE). Bundled components retain their own licenses;
see [Third-party notices](THIRD_PARTY.md).

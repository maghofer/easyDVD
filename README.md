<p align="center">
  <img src="internal/ui/assets/logo-retro-gold.png" alt="easyDVD" width="420">
</p>

easyDVD is a Windows app for turning folders of video files into DVD-Video and
experimental Blu-ray ISO files with menus. Add your folders, choose your disc
and menu settings, and let the queue process them into disc sets.

Blu-ray support in 0.2.1 is experimental; standalone-player compatibility has
not been verified.

**[Download for Windows (64-bit)](https://github.com/maghofer/easyDVD/releases/latest)**

The current release is **0.2.1**. Choose `easyDVD-…-win64-setup.exe`
for normal installation, or `easyDVD-…-win64.zip` for portable use. The separate
sources ZIP is not needed to run the app.

## What it does

- **Batch processing:** queue several folders at once. Each added folder becomes
  its own DVD set, including its subfolders. Each video file becomes a DVD title.
  Drop individual videos too; multiple files can share a set or become separate jobs.
- **Automatic disc layout:** spread videos across as many discs as needed,
  preserving file order.
  Videos stay whole; a file that cannot fit on one disc is reported.
- **Simple, customizable menus:** choose text or thumbnails, set backgrounds,
  fonts and colors, and save presets. Menus include title selection, page
  navigation and Play All.
- **DVD format choices:** PAL or NTSC, DVD-5 or DVD-9, with 4:3 and 16:9 video
  handled automatically. Confirmed 4:3 footage inside black 16:9 side borders is
  cropped without stretching. Single-title jobs can start playback without a menu.
- **Experimental Blu-ray:** BD-25/BD-50, native SD/720 and HD video, shared HD
  menus, automatic verified H.264 copying, and stereo/5.1 audio. Standalone-player
  compatibility is unverified. See [Blu-ray support](docs/BLURAY.md).
- **Pause and resume:** save the queue and continue later, reusing completed
  encodes. Errors are shown against the affected folder so other sets can continue.

## What it doesn't do

easyDVD focuses on batch conversion and automatic DVD layout. It is not a full
DVD authoring suite with a freeform title or menu designer.

- No timeline editing, trimming or transitions.
- No freely positioned menu buttons, animated menus or custom navigation logic.
- No subtitle authoring or multiple audio tracks, UHD Blu-ray or AVCHD-on-DVD.
- No disc burning. It creates ISO files to burn with your usual DVD burning
  software or open in a compatible player.

DVD output is standard definition. HDR video needs to be converted to SDR before
using easyDVD.

## Getting started

Run `easyDVD-…-win64-setup.exe` for a normal Windows
installation with shortcuts and an uninstaller. Later installers upgrade the
same location and keep your settings and queue. The ZIP remains available for
portable use; follow the extraction steps below for that version.

1. Install easyDVD and open its shortcut, or extract the complete Windows ZIP
   and open **easyDVD.exe**. For portable use, keep the `tools` folder beside it.
2. Drag folders into the window or use **Add folders**. Review the videos and
   choose an output folder and a local work folder with enough free space. You can
   also drop video files. Hold Shift while selecting files to select a range.
3. Choose your TV standard, disc capacity and menu style, then select
   **Start / resume**. Finished ISO files appear in your output folder.

Use the job header to select DVD or Blu-ray, capacity, encoding profile and
playback mode. Click the underlined output path to change it. Selecting several
jobs displays **Editing N jobs** and applies the edited setting to that selection;
mixed values stay unchanged until you edit that control. **Edit menu** opens the
shared menu editor for the selected jobs.

**Settings** contains application options: workers (Auto by default), temporary
work folder, appearance, staging, retained work, logs and updates. Worker choices
apply to the next job or resume. **Encoding presets** and **Menu presets** save
reusable job settings, assign them to selected jobs, or set defaults for new jobs.
Encoding presets contain disc format, capacity, TV standard and quality profile;
workers remain application-wide. Startup checks run in the background and only
show a notice when a newer stable release exists. Disable them in Settings.

Job tiles show status and completion percentage. Click a tile to select it;
Ctrl-click toggles additional jobs and Shift-click selects a
range. Files use the same selection behavior. Use **Remove selected** or **Delete**.
The X on a tile removes that job. Removing active jobs asks to stop them first;
source and output files are kept. Drag selected files or jobs up/down to reorder
them. Whole titles always follow strict file order across discs. Queue tiles also
show processing time; the bottom bar shows the number of files currently being
processed in parallel. Right-click a completed or failed job and choose **Clear
render status** to render it from scratch. Previous ISO sets are kept in their
existing folders; new renders receive a numbered output folder.

**Balanced** uses FFmpeg two-pass MPEG-2 and remains the default, targeting
7.5 Mbps video with 256 kbps AC-3 audio for speed and smaller files. Choose
**Quality** in the job header for HCEnc BEST two-pass VBR using AVStoDVD's settings:
8.5 Mbps video target, 9 Mbps peak and 448 kbps audio. AviSynth+ and FFMS2 are
included; first-launch setup downloads and verifies HCEnc once into a private
cache. AVStoDVD and a separate AviSynth installation are not needed.
easyDVD adds discs instead of reducing the chosen profile's bitrate to
squeeze more footage onto one disc. FFmpeg remains an external dependency;
first run can locate it or download a private copy. See [Encoding quality](docs/ENCODERS.md).
Single-video jobs default to automatic playback without a menu; multi-video
jobs default to a menu. Override either choice with **Menu / Autoplay** in the job header.
With **Autoplay**, every disc plays its included videos in order and stops at
the end. Menu editing is disabled until menus are enabled again.

For more on menu appearance and presets, see [Menu settings](docs/MENUS.md).
The **!** media-tools box includes FFmpeg/HCEnc command templates for every
profile, generated from the production settings. Job logs contain the resolved
commands with quoted paths.
Report problems or suggest changes through [GitHub Issues](https://github.com/maghofer/easyDVD/issues).

---

easyDVD is [MIT licensed](LICENSE). Bundled components retain their own licenses;
see [Third-party notices](THIRD_PARTY.md).

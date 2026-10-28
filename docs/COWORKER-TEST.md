# easyDVD preview test

This is an unsigned Windows x64 preview for practical testing. No GitHub account,
Go installation or compiler is needed. Extract the complete ZIP before running it.

## One-time setup

1. Extract the complete runtime ZIP, including its `tools` folder, and open
   **easyDVD.exe**. The DVD tools are included; DVDStyler is not required.
2. If FFmpeg setup appears, choose an existing compatible FFmpeg/FFprobe folder,
   or click **Download recommended 9.0.2**. This download needs internet access.
3. In Settings, choose a fast local **work folder** and a suitable **output folder**
   with enough free space. Add folders to the queue and start a short test first.

This package supplies easyDVD, the source-built DVD tools and their notices.
The separate matching sources ZIP includes application/Go sources, the helper
source archives, local changes and build recipes. Keep it available alongside the
runtime when passing the preview on. Users do not need to compile or install the
DVD tools. FFmpeg remains an explicitly selected/downloaded external dependency.

## Useful tests

- Start with a few short videos. Check picture geometry, audio, menu selection,
  Play All and return-to-menu in a player. Use a burned DVD if equipment is available.
- Add the real folder collections separately. One added folder is one DVD set;
  subfolders belong to that set. Confirm ordering and expected video counts.
- Try pause, close/reopen and resume. Check that completed encodes are reused.
- Exercise a network-source interruption and recovery with expendable test jobs.
- Try PAL/NTSC, 4:3/16:9 menus, thumbnails, different backgrounds and longer titles.
- Record errors, unexpected behavior, slow steps and any confusing controls.

Settings → Open logs folder opens the logs. Include the app version, what you did,
what happened and what you expected with a report. Logs/manifests can contain
source filenames and folder paths, so review them before sharing outside work.

The preview produces ISO files. Burning is done with your usual burning software.
Clean-machine, full-capacity DVD-9 and standalone-player behavior are still being
tested; please report results rather than treating them as established guarantees.

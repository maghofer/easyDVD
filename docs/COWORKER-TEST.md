# easyDVD setup and testing

easyDVD 0.1.0 is an unsigned Windows x64 release. No GitHub
account, Go installation or compiler is needed. Use the installer, or extract the
complete portable ZIP before running it.

## One-time setup

1. Run **easyDVD-…-win64-setup.exe** for a normal current-user installation,
   or extract the complete runtime ZIP, including `tools`, and open **easyDVD.exe**.
   DVD tools and Quality support libraries are included. DVDStyler, AVStoDVD and
   a system-wide AviSynth installation are not required.
2. If FFmpeg setup appears, choose an existing compatible FFmpeg/FFprobe folder,
   or click **Download recommended 9.0.2**. This download needs internet access.
3. HCEnc setup starts automatically after the FFmpeg check/setup. It downloads
   2.2 MB from the author and privately stores a verified copy for Quality encoding.
   If offline, close setup and use Balanced; retry later from
   **Settings → Repair dependencies → Quality (hcenc) setup**.
4. In Settings, choose a fast local **work folder** and a suitable **output folder**
   with enough free space. Add folders to the queue and start a short test first.

The package supplies easyDVD, source-built DVD tools and 32-bit Quality support
libraries, with their notices. HCEnc is downloaded automatically when missing;
FFmpeg can be selected or downloaded during its setup.
The matching sources ZIP includes application/Go sources and all bundled helper
and library source/build records. Keep it available alongside the installer or
runtime ZIP when distributing easyDVD. Users do not need to compile tools.
See `THIRD_PARTY.md` for component details.

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

easyDVD produces ISO files. Burning is done with your usual burning software.
Clean-machine, full-capacity DVD-9 and standalone-player behavior are still being
tested; please report results rather than treating them as established guarantees.

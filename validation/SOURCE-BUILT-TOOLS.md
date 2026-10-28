# Source-built portable authoring tools

Validated on 2026-10-28 for easyDVD `0.1.0-preview.8` and helper toolchain
`win64-2026.10.1`. This replaces the earlier DVDStyler-derived binaries and the
external-tools workaround. The normal runtime ZIP contains three helper EXEs;
DVDStyler and extra authoring DLLs are not required.

## Build/source result

- Native Windows x64/UCRT `dvdauthor.exe`, image-only `spumux.exe` and file-based
  `mkisofs.exe`, all built with the pinned WinLibs GCC 16.2.0 toolchain.
- Current libxml2 2.15.4, libpng 1.6.59 and zlib 1.3.2 are linked into the helpers.
  FreeType, Fontconfig, FriBidi, ImageMagick, iconv and SCSI/ASPI access are unused
  by easyDVD's image-menu/ISO pipeline and omitted from these builds.
- Original source archives, every local modification, build configuration,
  scripts and notices are supplied in the matching sources ZIP's `toolchain`
  folder. Additional cdrtools/support-library copyright notices accompany the
  runtime. The source verifier checks archive and material hashes against each
  exact helper binary. Full hashes/imports/versions: SOURCE-BUILT-TOOLS.json.
- Only Windows system DLLs are imported. The final three EXEs and the portable
  runtime package passed a scan for this workstation's profile/scratch paths.
  Unused XML catalog support was disabled to remove an install-prefix default.
- A clean recipe build completed. Subsequent verification/cache reuse took about
  0.2 seconds and performed no compilation. A modified source companion was
  rejected instead of silently reused. CI caches the bundle by recipe/lock inputs.
  A missing cache is rebuilt automatically; new helper inputs need a new ID after
  publication. Ordinary easyDVD pushes do not require new helper compilation.

## Validation

- All Go unit tests and vet passed. The real integration suite also passed with
  race instrumentation using the final source-built tools: PAL/NTSC pipelines,
  asset/preset recovery, 36-title/four-page and 99-title/99-page authoring.
- The actual packaged GUI EXE completed a three-title, two-page PAL fixture with
  only Windows system directories on PATH, explicit external FFmpeg and no
  authoring override. Its sidecar contained only our three helpers. Source/work
  paths included spaces, punctuation, accented characters and a non-Latin work
  folder. An explicit isolated queue protected the user's GUI state.
- The resulting ISO was 4,585,472 bytes. All three DVD titles were fully decoded
  through FFmpeg's DVD-Video demuxer without errors.
- libVLC navigation passed initial menu, title selection/return, Play All through
  all three titles, return to menu, next page and second-page title/return.
- A separate UDF ISO test wrote **4,295,829,504 bytes**, matched mkisofs' predicted
  sector count, and located/verified the last input's marker beyond the 4 GiB
  boundary using its ISO directory record. This checks large offsets/output;
  it is not a burned/full-capacity DVD-9 or layer-break compatibility result.
- Public-mode packaging generated the portable ZIP and matching application/Go/
  toolchain sources ZIP. The shipped source companion passed verification again
  after copying into the actual package.

## Remaining scope

The source/provenance gap for the old supplier binaries is avoided by building
and supplying our own matching material; the old supplier manifest remains
historical. FFmpeg stays external with its existing manual download/location
setup. Public offline FFmpeg bundling still needs its separate complete source
review and is rejected by public-mode packaging.

Clean Windows VM, physical DVD-player, full-capacity/layer-break, real network
disconnect and comparative encoder-quality checks remain open. No GitHub release
or message to the coworker was published/sent. Coworker setup is now simply
extracting the portable runtime ZIP and opening easyDVD.exe.

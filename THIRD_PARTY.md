# Third-party components

The MIT license covers easyDVD's own source. Bundled programs, fonts, and Go
dependencies retain their upstream licenses; they are separate components.

| Component | Use | Upstream |
|---|---|---|
| Fyne 2.8.2 | Desktop GUI (BSD-3-Clause) | https://github.com/fyne-io/fyne |
| modernc SQLite | Queue storage (see module license) | https://modernc.org/sqlite |
| Go fonts / x/image | Menu typography and rendering | https://go.googlesource.com/image |
| FFmpeg / FFprobe 9.0.2 Gyan full build | External runtime dependency; optional download/offline package (GPL-enabled build) | https://www.gyan.dev/ffmpeg/builds/ |
| dvdauthor / spumux 0.7.2-easyDVD.1 | DVD structures and PNG menu subpictures (GPLv2 or later) | https://dvdauthor.sourceforge.net/ |
| mkisofs 2.01-bootcd.ru-easyDVD.1 | DVD UDF/ISO mastering (GPLv2 or later) | https://sourceforge.net/projects/mkisofs-md5/files/mkisofs-md5-v2.01/ |
| libxml2 2.15.4 | XML parsing, statically linked into helpers | https://download.gnome.org/sources/libxml2/2.15/ |
| libpng 1.6.59 / zlib 1.3.2 | PNG highlights, statically linked into spumux | https://libpng.org/pub/png/libpng.html / https://zlib.net/ |

Exact archive checksums and binary versions are pinned in `scripts/tools.lock.json`.
The application build uses Go 1.27.3 and WinLibs GCC 16.2.0. The helper build also
uses an isolated, pinned MSYS2 shell/make environment. These development tools
are not included in the application package. Helper source/build inputs are
pinned separately in `scripts/toolchain/lock.json`.

Normal portable packages contain three source-built helper executables, their
licenses, Go/build-runtime notices, build information and a binary hash manifest.
The matching sources ZIP includes application/Go sources and a `toolchain` folder
with the exact original helper/library archives, local changes and build recipe.
`scripts/package.ps1 -PublicRelease` verifies those source records automatically.
No authoring DLLs or DVDStyler files are redistributed. FFmpeg is external by
default; its license/readme accompany its explicit managed download. Public offline
FFmpeg bundling remains blocked pending its separate complete dependency/source
review. See `docs/RELEASING.md` for the remaining release validation.

The earlier supplier review in `validation/THIRD-PARTY-REVIEW.md` and
`third_party/dvd-tools/review.json` is historical and concerns the DVDStyler-derived
files used by older candidates. The current helper source recipe is in
`scripts/toolchain/README.md`. No bit-identical reproduction claim is made; the
matching source and all our build changes are supplied.

HCenc, AviSynth, FFMS2, and mpeg2enc are optional benchmark inputs and are not
distributed by easyDVD. No encoder superiority claim is made without a
completed comparison on representative material.

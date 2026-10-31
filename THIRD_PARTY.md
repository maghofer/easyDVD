# Third-party components

The MIT license covers easyDVD's own source. Bundled programs, fonts, and Go
dependencies retain their upstream licenses; they are separate components.

| Component | Use | Upstream |
|---|---|---|
| Fyne 2.8.2 | Desktop GUI (BSD-3-Clause) | https://github.com/fyne-io/fyne |
| modernc SQLite | Queue storage (see module license) | https://modernc.org/sqlite |
| Go fonts / x/image | Menu typography and rendering | https://go.googlesource.com/image |
| FFmpeg / FFprobe 9.0.2 Gyan full build | External runtime dependency; optional download/offline package (GPL-enabled build) | https://www.gyan.dev/ffmpeg/builds/ |
| HCEnc 0.28.0.2 | External Quality encoder; automatic private download, not redistributed; copyright hank315 | http://hank315.nl/ |
| AviSynth+ 3.7.6 | Source-built 32-bit HCEnc input runtime (GPLv2-or-later plus linking exceptions) | https://github.com/AviSynth/AviSynthPlus |
| FFMS2 5.0 | Source-built input plugin (MIT), with minimal FFmpeg 6.1.3 (LGPLv2.1-or-later) and zlib 1.3.2 | https://github.com/FFMS/ffms2 / https://ffmpeg.org/ |
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

Quality support is built with Microsoft's x86 C++ compiler and static CRT.
AviSynth+ optional plugins are disabled; DevIL is excluded. FFMS2's embedded
FFmpeg enables only the FFV1 decoder, Matroska input and local-file protocol;
it reads easyDVD's prepared lossless input. General source decoding continues
to use the separately obtained FFmpeg/FFprobe pair.

The support-library source companion includes all original source archives,
license/linking exceptions and complete build recipes, mapped to DLL hashes.
Public packaging verifies these records. HCEnc and the original AVStoDVD binary
archive are excluded from both packages because a general redistribution grant
was not established. On GUI startup (after FFmpeg checking/setup), the app downloads
the author's original 2.2 MB ZIP from `http://hank315.nl/` if HCEnc is missing.
The legacy site uses HTTP; pinned SHA-256 hashes verify both archive and encoder
before installation. Manual retry and CLI `-download-hcenc` are also available.
Only HCEnc and its original notices are extracted to a private user cache.
No AVStoDVD application is installed
or republished. See `scripts/quality-runtime/README.md`.

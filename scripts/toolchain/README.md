# easyDVD DVD authoring tools — win64-2026.10.1

The portable application includes only `dvdauthor.exe`, `spumux.exe` and
`mkisofs.exe`. They are native Windows x64/UCRT programs built from the source
listed in lock.json. Required non-system libraries are linked into these helper
programs; no DVDStyler installation or external authoring DLLs are needed.

Inputs: dvdauthor 0.7.2, the GPLv2-or-later bootcd.ru-patched cdrtools 2.01
source, libxml2 2.15.4, libpng 1.6.59 and zlib 1.3.2. The original archives and
their hashes are retained. The combined helper programs are GPLv2 or later;
the libraries also retain their individual permissive notices. The easyDVD
application remains a separate program with its own license.

## Build and reuse

On Windows x64 with PowerShell 7, Git and curl/tar available:

```powershell
./scripts/build-authoring.ps1 -ScratchDir $BuildScratch
```

The script verifies the pinned source/development archives, prepares an isolated
MSYS build shell in the scratch directory, and builds with the pinned WinLibs GCC
compiler. MSYS and the compiler are development tools and are not shipped with
easyDVD. No global installation, registry setting or persistent PATH change is
made. Supply an absolute scratch directory with enough space for the compiler,
build shell and objects. TEMP/TMP changes are process-local.

The default reusable output is `.tools/authoring/win64-2026.10.1`. Its binaries,
recipe digest and corresponding-source records are checked before reuse. A
normal easyDVD build/package does not rebuild these helpers when that cache is
valid. The CI cache uses the recipe/lock content as its key. To test a clean
rebuild, use `-Force -OutputDir $NewEmptyOutput` with a different output directory.
Keep one toolchain ID immutable after publishing it; bump it for changed source,
patches, configuration or compiler inputs.

## Rebuild from the corresponding-source package

This source folder contains `archives/` (the original source releases), `recipe/`
(the scripts/configuration used), `notices/` and review.json. From this folder:

1. Create an absolute scratch directory. Copy `archives/*` into its `downloads`
   subdirectory to reuse the supplied sources without downloading them again.
2. Run `./recipe/scripts/build-authoring.ps1 -ScratchDir $BuildScratch -OutputDir $NewEmptyOutput`.
   The script can obtain its pinned compiler and build-shell inputs over HTTPS.
3. Inspect the generated binary/source manifests and run easyDVD's integration
   tests with `EASYDVD_TOOLS` pointing at the resulting `bin` directory.

All local changes are provided: the dvdauthor Windows patch, image-only globals,
CMake configuration and the cdrtools porting script. The recipe is intended to
reproduce the programs and configuration; byte-identical executable hashes are
not promised across different build roots/tool environments.

## Configuration and changes

- dvdauthor's DVD structures and navigation are retained. Its Windows adaptations
  use native directory/sync calls, standard memset and Winsock byte-order helpers.
- spumux reads PNG subpictures. easyDVD renders every menu label, font and
  highlight before invoking it, so its unused FreeType/text-subtitle, Fontconfig,
  FriBidi, ImageMagick and iconv paths are omitted. Requests for `<textsub>` fail
  explicitly. This does not remove text or custom fonts from easyDVD's menus.
- libxml2 keeps XML reader functionality with external encoding libraries, Python,
  catalogs, dynamic modules and standalone programs disabled. Catalog support is
  unused and would otherwise embed an installation-specific default catalog path.
  libpng and zlib are static.
- mkisofs keeps DVD-Video, UDF, large-file and filesystem-image support. Physical
  SCSI/ASPI-drive access is omitted; easyDVD produces ISOs rather than burning.
- The cdrtools Windows port avoids a CRT header-name collision, uses current
  Windows time/type declarations, and fixes pointer-sized opaque formatter and
  condition arguments for Windows' LLP64 model. Original numeric long formatting
  is retained. Compilation treats pointer truncation warnings as errors.

The source changes and build records address the earlier supplier-provenance
gap. Validation of the generated tools is recorded separately in the repository;
source-record verification alone does not test DVD compatibility.

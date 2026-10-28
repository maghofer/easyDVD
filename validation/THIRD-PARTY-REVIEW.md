# Third-party redistribution review

Historical supplier review: [preview.8](SOURCE-BUILT-TOOLS.md) replaces the files considered below with
helpers built from our own pinned source recipe. The old supplier mapping is not
used by the normal package. See scripts/toolchain/README.md for the replacement.

Date: 2026-10-28. Scope: Windows x64 easyDVD preview. Outcome: the external-tools
package is prepared for coworker testing; the bundled DVD/FFmpeg variants are
not cleared by this review.

## What corresponding source means

For the GPL-covered programs we redistribute, a general license and a link to a
project homepage do not establish that we supply the source used for those
binaries. GPLv2 section 3 describes source including interface definitions and
the scripts controlling compilation and installation. LGPL libraries have their
own source/replacement requirements. We use the source-alongside-binaries route
for eventual bundles rather than making an unverified written source offer.

This is a practical provenance and packaging review. It records established facts
and gaps; the existence of a source tarball is not treated as proof of an exact
supplier build, and byte-for-byte reproduction is not itself a GPL requirement.

## Actual bundled inventory

The normal preview contains these 11 external files from the hash-pinned DVDStyler
3.2.1 installer. Hashes, DLL imports, version evidence and candidate mappings are
recorded in `third_party/dvd-tools/review.json`.

| File(s) | Version established from the actual binary | Upstream terms inspected |
|---|---|---|
| dvdauthor.exe, spumux.exe | 0.7.2, executable diagnostics | GPLv2 or later |
| mkisofs.exe | 2.01-bootcd.ru, executable diagnostics | GPLv2 or later; bootcd patch candidate found |
| libfontconfig-1.dll | 2.10.2, FcGetVersion | Fontconfig permissive notice |
| libfreetype-6.dll | 2.4.11, FT_Library_Version | Dual FreeType License / GPLv2 or later; account for linked-license compatibility |
| libfribidi-0.dll | 0.19.6, exported version information | LGPLv2.1 or later |
| libiconv-2.dll | 1.13.1, PE version resource and exported major/minor | LGPLv2 or later for the library |
| liblzma-5.dll | 5.0.4, lzma_version_string | liblzma public domain; other source-archive files differ |
| libpng14-14.dll | 1.4.0, png_get_libpng_ver | libpng notice |
| libxml2-2.dll | 2.8.0, __xmlParserVersion getter | MIT-style notices, including additional file notices |
| zlib1.dll | 1.2.8, zlibVersion | zlib notice |

The supplier archive was verified against its pinned SHA-256. Original files were
not modified. Windows system DLLs are imported and are not copied into our ZIP.
In particular, libxml2's version was measured; its 2013 timestamp did not imply
the later 2.9.1 release.

## Material acquired and changes made

- Downloaded and hash-recorded ten version-matched upstream source candidates,
  including the bootcd.ru-patched cdrtools source and its MinGW build instructions.
  `source-candidates.json` pins them, and `fetch-toolchain-source-candidates.ps1`
  retrieves/verifies them into an explicitly chosen directory.
- Inspected the DVDStyler source tarball and its release script. It does not
  establish the complete build provenance of the Windows helper binaries/DLLs.
- Preserved individual tool/library licenses under `third_party/dvd-tools/notices`;
  bundled packaging now includes these instead of relying on one generic GPL file.
- Added Go, GCC runtime exception and MinGW runtime/header notices to all packages.
  Go module sources and recursively collected notices remain in the matching
  source/runtime packages. The pinned compiler is unmodified and uses standard
  Go/cgo compilation; no GCC compiler binary is redistributed in the test kit.
- Checked easyDVD/CLI/benchmark imports: Windows system DLLs only, with no external
  DVD/FFmpeg or GCC DLL imported into the application. Record: APPLICATION-IMPORTS.json.
- Strengthened the bundled public-package check: an existing directory is no longer
  sufficient. It needs a complete review.json with each binary/hash, hash-verified
  source archives, license files, build instructions and an explicit patch record.
  This verifies records and files, not the truth of an unsupported approval.

## Unresolved for a bundled public release

The exact Windows supplier patches, configuration and build recipes have not been
established for every DLL/helper. The downloaded archives are source candidates,
not an approved corresponding-source bundle. The current review manifest remains
explicitly incomplete. Completion requires supplier confirmation of the full
mapping/material, or building a replacement toolchain from pinned source and
retaining its recipes, patches and notices. Replacing the toolchain also requires
the media/navigation regression suite. No upstream message has been sent.

Offline FFmpeg packaging has a separate open review: the Gyan build is GPLv3 and
links many external projects. Its README's FFmpeg commit/configuration is useful
evidence but not the complete source/build material for all linked dependencies.
The coworker kit ships neither FFmpeg nor the DVD tools.

## Coworker test package

`package.ps1 -ExternalTools` produces easyDVD and its matching application/Go
sources, with build/runtime notices and START-HERE.md. Its launcher uses a
separately installed DVDStyler 3.2.1, and the app offers its existing explicit
FFmpeg download/location setup. The test computer obtains the external programs
from their upstream distributors. The upstream distributors' compliance has not
been certified by this review.

The GNU FAQ distinguishes copies used within one organization for its work from
copies transferred to outsiders. We do not assume that a personal project's
handoff to a workplace automatically falls into the internal-use case. The
external-tools kit avoids redistributing those unreviewed external binaries.
VM and physical-player tests are useful preview work, not a prerequisite to
starting a coworker test. A preview test is not a claim of completed validation.

Primary references: [GPLv2 section 3](https://www.gnu.org/licenses/old-licenses/gpl-2.0.html#section3),
[GNU FAQ on internal copies](https://www.gnu.org/licenses/gpl-faq.html#InternalDistribution),
[DVDStyler upstream source](https://sourceforge.net/projects/dvdstyler/files/dvdstyler/3.2.1/),
[bootcd.ru source project](https://sourceforge.net/projects/mkisofs-md5/files/mkisofs-md5-v2.01/),
[GCC runtime exception](https://www.gnu.org/licenses/gcc-exception-3.1.html).

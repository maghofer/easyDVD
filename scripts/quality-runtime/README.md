# Source-built Quality support

The distributable contains two 32-bit DLLs: AviSynth+ 3.7.6 and FFMS2 5.0.
FFMS2 statically includes a minimal FFmpeg 6.1.3 (LGPL 2.1-or-later configuration)
and zlib 1.3.2. Only FFV1 decoding, Matroska input and local files are enabled;
this plugin reads easyDVD's prepared lossless input, not arbitrary original
media. Original-media decoding and preparation continue to use external FFmpeg.
The HCEnc BEST configuration and prepared video pixels are unchanged.

AviSynth's optional plugins and CUDA are disabled. No DevIL, system AviSynth,
FFmpeg DLLs or Visual C++ runtime DLL installation is needed. The Microsoft CRT
is linked statically. Windows 10/11 system libraries remain external.

## Build and sources

Run on Windows x64 with PowerShell 7, Git for Windows, the project's pinned
CMake/Ninja tools and Microsoft's C++ build tools (including the x86 compiler
and a Windows SDK). Visual Studio 2022 MSVC 19.44 and SDK 10.0.26100.0 were used
for validation. The 32-bit GCC AviSynth C ABI is incompatible with HCEnc, so the
script explicitly selects the x86 Microsoft ABI.

```powershell
./scripts/build-quality-runtime.ps1 -ScratchDir $buildScratch -OutputDir $qualityCache
```

Visual Studio is discovered through vswhere. Alternatively pass `-MSVCRoot`
pointing at a portable build-tool tree containing `VC/Tools/MSVC` and
`Windows Kits/10`. Compiler tools are build prerequisites and are not shipped
with easyDVD. Build logs stay in the selected scratch directory. Process-local
TEMP/TMP/TMPDIR and compiler environment settings are restored on completion.

`lock.json` pins every source archive and GNU make download. The source companion
contains all four original source archives, these scripts/configuration, notices,
and a review manifest mapping binary hashes to source/build records. Source
files are not modified; the local CMake plugin recipe and FFmpeg configuration
are included. To rebuild from a source companion, copy `archives/*` into your
scratch `downloads` folder and run `recipe/scripts/build-quality-runtime.ps1`.
Byte-identical builds across compiler versions/build directories are not claimed.

The builder rejects unexpected runtime imports and nonempty replacement caches.
Normal builds reuse a hash-verified cache only when the recipe matches. Use a
new runtime ID for changed recipes. `Assert-QualityRuntime` also verifies all
source records before public packaging.

## HCEnc is an end-user dependency

HCEnc is closed-source freeware. Its original 2015 archive was inspected, and
the executable matches the former AVStoDVD-supplied copy exactly. An explicit
general redistribution grant was not established. Consequently neither the
encoder nor its original binary archive is included in runtime/source packages.
This is not a claim that redistribution is prohibited.

The GUI downloads the author's original 2.2 MB ZIP from `http://hank315.nl/`
automatically after FFmpeg setup/checking if HCEnc is missing. Quality setup
supports cancellation/retry; CLI setup uses `-download-hcenc`. The legacy site
uses HTTP, so pinned SHA-256 hashes of both archive and encoder are required
before installation. A named easyDVD user-agent avoids the site's rejection
of Go's default client. Verified cached encoders are reused without downloading.
Only HCEnc and its original notices are extracted; no downloaded program is run
during installation. The private user cache survives upgrades and uninstall.
No AVStoDVD application, installer, supplier FFMS2 binary or DevIL is republished.

The former `win32-avstodvd-2026.10.1` runtime and its pending review are historical.
Its permission/source gaps have not been relabelled as complete: those binaries
and supplier archives were replaced or excluded from the public payload.

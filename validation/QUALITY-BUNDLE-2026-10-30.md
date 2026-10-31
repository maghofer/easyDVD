# Portable Quality runtime validation

Historical supplier-runtime record. The distributable now uses source-built
support libraries and an external HCEnc download; see
[release-candidate packaging validation](PUBLIC-PACKAGE-2026-10-31.md).

Date: 2026-10-30. Local Windows x64 preview. This records functional packaging
checks; public redistribution review remains incomplete.

## Included runtime

The build and local package now include `tools/quality` with four unchanged
32-bit binaries: HCenc.exe, avisynth.dll, ffms2.dll and DevIL.dll. The HCEnc and
FFMS2 bytes match the installed AVStoDVD copies used by the earlier Quality
validation. AviSynth/DevIL come from the official AviSynth 2.6.0 installer.

The original supplier/installer archives and individual binaries are pinned by
SHA-256 in `scripts/quality-runtime/lock.json`. Preparation only extracts files;
it does not execute installers, register codecs, modify system DLLs or change
saved application preferences. The Quality encoding settings, Balanced encoder
arguments and both profiles' current cache identities are unchanged by bundling.

Normal builds and local package ZIPs copy the runtime and notices automatically.
`quality-binaries.json` records these hashes separately from the three source-built
64-bit authoring helpers. The source/provenance ZIP includes the exact AviSynth
source archive, original supplier archives and runtime record. Supplier binary
archives are explicitly identified as provenance, not corresponding source.

## Completed checks

- `go test ./...` and `go vet ./...` passed.
- The complete real integration suite passed using the staged portable runtime,
  including Balanced/Quality PAL/NTSC, scope geometry, audio/silence, interlace,
  menus, autoplay, source selection, checkpoint recovery and 36/99-title authoring.
- Windows GUI, CLI and benchmark builds succeeded. Local runtime and
  source/provenance ZIP generation succeeded through the normal package script.
- A fresh extraction selected HCEnc from its own `tools/quality` directory.
  PATH was limited to Windows system tools, application discovery locations
  were redirected to empty test directories, and FFmpeg was supplied explicitly.
- A separate 32-bit loader probe loaded AviSynth and FFMS2 using only the
  selected DLL directory and Windows System32 as search locations. Module paths
  confirmed that AviSynth, DevIL and FFMS2 all came from the bundle. The probe
  is retained in `validation/quality-runtime-probe.ps1`.
- A missing DevIL DLL blocked Quality even though a system copy existed.
  Balanced's tool check still succeeded with the damaged Quality runtime.
- Discovery tests verify that a partial `quality` directory cannot silently
  borrow the system AviSynth or fall back to an installed HCEnc.
- A PE import-table regression test covers AviSynth's descriptors with no
  OriginalFirstThunk. Go's ImportedLibraries function is unimplemented and its
  ImportedSymbols function does not handle this layout; the dependency check
  reads import names directly instead.
- The public-package gate rejected the pending distribution review as intended.
- Source audit and whitespace validation passed.

## Real media and loaded-module proof

A new isolated CLI job encoded the complete 94.68-second 1080p25 ProRes HQ
source plus synthetic TFF and BFF clips, with two workers and an accented work
directory. Only bundled Quality components were selected; all outputs used
720x576/25 fps. Probe results retained progressive 16:9 for the ProRes source
and 4:3 `tt`/`bb` for the two interlaced sources.

While the full source was encoding, HCEnc's actual loaded-module list showed
AviSynth and DevIL in its private temporary directory (through the Unicode-path
junction) and FFMS2 in the build's `tools/quality` directory. None came from the
installed AVStoDVD or system AviSynth directories.

The job authored a 96,256,000-byte three-title menu-free ISO. All three authored
titles fully decoded with FFmpeg `-xerror` without errors. No temporary HCEnc
input/runtime directories survived successful completion.

## Distribution limit

The bundle is functionally self-contained for local testing. Public packaging
remains blocked until HCEnc redistribution terms and the complete matching
supplier FFMS2/FFmpeg and DevIL source/build records are verified. The supplier
plugin identifies as 2.27.0.0 and embeds FFmpeg N-89607-g52d70aded6 with zlib
1.2.11; its supplied documentation is insufficient to identify all source and
patches. No public redistribution clearance or clean-VM test is claimed.

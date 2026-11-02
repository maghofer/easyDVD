# Private Blu-ray authoring runtime

Prepare with `../prepare-bluray-runtime.ps1 -Destination RUNTIME -Scratch SCRATCH`.
Both paths are explicit; normal builds use their temporary build directory.
No pip invocation, system Python installation, registry change or global PATH
change is involved. Downloaded archives are SHA-256 pinned in `lock.json`.

`patch_bdmvkit.py` applies guarded substitutions to the pinned source: per-title
format/aspect, SD modes, exact 24 fps, correct mono signaling and explicit menu
color signaling. Changed upstream files retain their LGPL notices and receive
a modification notice. Original archives and the recipe accompany source bundles.
`author.py` exposes only the private version-1 authoring manifest and JSON-line
stage notifications. Encoding is controlled by the Go application.

For the independent validation gate, compile `validation/bluray-navcheck.c`
against libbluray and run `validation/bluray-feasibility.py` with the prepared
runtime, explicit scratch directory, FFmpeg and reader paths. `--large` checks a
file marker beyond 4 GiB through the independent UDF reader. Go integration tests
use `EASYDVD_BLURAY_INTEGRATION=1`, `EASYDVD_BLURAY`, and optionally
`EASYDVD_BLURAY_READER`. None use the GUI queue.

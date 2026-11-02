# Encoding concurrency validation

2026-11-02, Windows, AMD Ryzen 7 9800X3D (8 cores / 16 logical CPUs),
64 GiB installed memory, external FFmpeg/FFprobe 9.0.2 and source-built
AviSynth+ / FFMS2 with HCEnc 0.28.0.2. This is not a Threadripper measurement.

## Implemented behavior

- Auto is represented by `encoders: 0`; fixed choices are 1–32.
- Auto starts with one worker per two logical CPUs (rounded up), capped at 32
  and the number of unencoded titles. It budgets 1 GiB per worker after a
  2 GiB reserve from available physical RAM. Unknown memory caps Auto at two.
- Auto reduces concurrency until conservative staging and Quality scratch
  reservations fit. Normal work/output free-space checks remain authoritative.
- The resolved count is local to a run. Persisted Auto stays Auto on resume.
- FFmpeg decoder/filter/encoder pools share a per-worker thread budget, bounded
  at eight threads per pool. HCEnc SMP and all quality options are unchanged.
- Existing worker choices are retained. Paused jobs can select Auto or a new
  fixed count without changing locked quality settings or saved encodes.
- The scheduling change alone does not change either encoding cache identity.
  The subsequent mux fix versions Quality's identity separately (see below).
  No chunk encoding or lossy intermediate was added.

## Throughput sample

Eight local copies of one 12-second PAL SD lossless FFV1 sample were processed
as one DVD set. The sample uses seconds 20–32 of the read-only real ProRes25
fixture, prepared once outside the measurements at 720×576 / 25 fps, with
4:3 presentation and no audio. The app generates silence. This tests throughput,
not scaling fidelity or picture quality. Prepared sample SHA-256:

`9df2b189bd9498cc6ec606302c3abcca591b1115be02b9da9505b16638d01d7f`

Each measurement used a fresh explicit CLI queue, work and output directory
in isolated local scratch storage, with local staging, retained work and no
menu. The user's GUI queue and preferences were not used. Source preparation
is excluded. Times include scanning, staging, both encoding passes, encode
validation, authoring and ISO publication. All fixed-count runs use the new
bounded thread pools; this is not a before/after comparison with a released
binary. These are single exploratory runs, not statistically repeated results.

| Requested workers | Balanced seconds | Quality / HCEnc seconds |
|---|---:|---:|
| 1 | 79.288 | 92.255 |
| 2 | 41.247 | 48.251 |
| 4 | 33.253 | 31.254 |
| 8 | 22.256 | 20.250 |
| Auto (resolved to 8) | 22.270 | 20.255 |

For this workload Auto has approximately 1.85× Balanced throughput and 2.38×
HCEnc throughput compared with two workers. Small CPU-load variations and the
CLI's one-second completion polling affect these timings. Initial conservative
Auto experiments were superseded before the final Auto measurements above.

Eight workers were best among the tested fixed choices. This does not establish
a universal optimum: original HD decoding, storage/network traffic, longer or
different footage, SMT and other applications can change the result. Auto is
a resource-based heuristic, not runtime benchmark calibration or a guarantee
of maximum CPU utilization. One pending title still has only one encoder.

## Output checks and observed limitation

Both final Auto jobs produced eight MPEG-2 720×576 / 25 fps titles and a single
ISO. Encodes passed the production full decode and hash checks. ISO SHA-256
values matched the published manifests:

| Profile | ISO bytes | SHA-256 |
|---|---:|---|
| Balanced | 81,397,760 | `2644831824a6f9e99a283b42c135eaad224d5e40fb0f479d02475c6c615774f2` |
| Quality | 104,794,112 | `219d3ddf68580e305477d9d433ba636a42ad693a6efe7186cac889b5ffa6cb92` |

Frame counting found 300 video frames in every Balanced Auto title and 297 in
every Quality Auto title. Spot checks of the one-, two-, four- and eight-worker
outputs matched each profile's result. HCEnc's report says it encoded all 300
frames, so the Quality path has a separate end-of-title muxing issue: its MPG
video contains 297 frames / 11.88 seconds while audio spans 12 seconds. The mux
arguments are unchanged by this work, and the one-worker Quality result has
the same discrepancy. This issue was subsequently fixed and regression-tested
in [the mux correction](HCENC-MUX-2026-11-02.md). The measurements and hashes
above describe the earlier run. Full decoding alone does not prove exact frame
preservation. No chunk/join correctness claim is made.

## Automated checks

- `go test ./...` and `go vet ./...` passed.
- `go test -race ./internal/engine ./internal/media ./internal/process` passed.
- Real `TestPipeline` passed Balanced/Quality × PAL/NTSC, with NTSC requesting
  four workers for three concurrent titles. It checks authoring, media
  properties, interlaced/progressive sources and saved-artifact reuse.
- PAL pause/reopen/resume now switches to Auto before continuing. The saved
  encode modification time remains unchanged, proving checkpoint reuse.
- Unit tests cover CPU/worker limits, memory fallback and limits, exact scratch
  boundaries, pending-title limits, settings persistence and cache identity.
- GUI tests cover fixed/default choices and switching a paused legacy Quality
  job to Auto while retaining its saved encode and locked settings.
- Desktop build passed. The test build is in scratch, not installed or published.

Raw timing JSON, queues, source samples, media and logs remain under the local
scratch directory. The short source sample and the two numbered real collections
were not combined; numbered source sets were not processed by this sweep.

## Single-title follow-up

The [encoder documentation](../docs/ENCODERS.md#chunking-a-single-title) records
why chunking is separate work. HCEnc already enables its documented SMP mode;
its manual does not expose an encoder thread-count setting. It does expose
frame ranges. The existing lossless intermediate could feed parallel frame
ranges without another lossy preparation step, but independent two-pass chunks
change GOP boundaries and rate allocation. Output joining, frame/field counts,
VBV and continuous audio need dedicated validation before enabling that path.

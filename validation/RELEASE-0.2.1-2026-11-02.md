# 0.2.1 release correction

The first hosted 0.2.0 candidate passed the encoding/mux and DVD cases but failed
the short Blu-ray queue test: it requested 51.2 GB free for a few seconds of
content on a runner with 28.6 GB available. That candidate was not published.
Its pushed tag is retained unchanged.

The inherited workspace calculation reserved two full blank-disc capacities.
Blu-ray now uses the conservative packing projection: prepared/estimated media
plus menu/structure reserves per disc, retained BDMV and ISO copies of the
projected disc set, staging, safety headroom and output when it shares the volume.
An oversized/unpackable estimate retains the full-capacity fallback. DVD's
existing space calculation and all encoding/checkpoint identities are unchanged.

Regression tests show the same short content has the same reservation on BD-25
and BD-50 and needs less than 3 GiB, while large prepared media still reserves
both authoring copies and the final output, including multi-disc sets.
All-package tests/vet and engine/
packing race checks pass. The real-tool DVD/Blu-ray suite is rerun for this
correction; the hosted tagged build must pass before publication.

This release also includes the feature and packaging validation recorded in
[0.2.0 preparation](RELEASE-0.2.0-2026-11-02.md). All generated files and queues
use isolated local scratch storage; the user's GUI queue is untouched.

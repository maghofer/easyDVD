# Automatic HCEnc setup validation

Completed: 2026-10-31, Windows 11 x64. Candidate: `0.1.0-rc.2`.
This updates the HCEnc download behavior recorded for rc.1 in
[the public-package report](PUBLIC-PACKAGE-2026-10-31.md).

## Behavior

- HCEnc comes directly from the author's original ZIP at
  `http://hank315.nl/files/HC_028/HC028_21-12-2015.zip` (2,228,924 bytes).
  The server returned 403 to Go's default user-agent, but accepted the named
  easyDVD user-agent. No proxy or certificate checks were bypassed.
- The legacy site uses HTTP. Installation requires the pinned archive SHA-256
  `ed392df990120cd5430454bec68555dd444f1193f930e021c42b1871f5e245a7`
  and encoder SHA-256
  `be55e5811c3641b364ff1e12e758b76895e61008e5759c2c14acc3ec4ca4f371`.
  Unexpected redirect hosts, schemes and credentials are rejected.
- The GUI starts setup after FFmpeg discovery or successful FFmpeg setup when
  HCEnc is missing and the support DLLs are present. Progress and cancellation
  remain available. Success closes automatic setup; a verified cache avoids
  subsequent downloads. Existing encoders and explicit HCEnc overrides are kept.
- Failure leaves Quality setup available for retry; closing setup permits
  Balanced work. Missing HCEnc is retried on a subsequent launch. FFmpeg remains
  an explicitly selected or downloaded dependency. CLI HCEnc setup remains
  `-download-hcenc`.
- The private cache retains the encoder, copyright notice, changelog and manual.
  No HCEnc binary/archive is bundled or republished. Source-built support DLLs,
  encoding settings, installer behavior and user-data preservation are unchanged.

## Completed checks

- All Go unit tests and `go vet ./...`; source audit and whitespace checks.
- Tests cover publisher-style user-agent rejection, both pinned hashes, cache
  reuse without HTTP requests, corrupt-cache replacement without overwriting,
  extraction restrictions, cancellation, download limits, redirect restrictions,
  unavailable-server cleanup, automatic launch without a download click,
  duplicate-start prevention, close-to-cancel and existing/custom runtime handling.
- The actual packaged GUI opened with isolated application/configuration caches
  and downloaded HCEnc without a click. Its installed encoder matched the pinned
  hash. Packaged CLI Quality tool checks reported no errors using that same cache
  and the packaged source-built DLLs.
- The packaged CLI then encoded a one-second PAL fixture using that downloaded
  encoder and authored a 1,708,032-byte DVD ISO with an isolated queue/work/output.
- Public packaging verified runtime/source records and built the rc.2 installer,
  portable ZIP and matching sources ZIP. No release, upload or tag was created.

## Limits

The native Windows screen-inspection helper timed out, so the automatic dialog's
visual appearance was not confirmed in this run. The actual startup download,
tool validation and Quality encode were confirmed independently.

The installer implementation is unchanged from the completed rc.1 lifecycle
checks. Its full install/upgrade/uninstall cycle was not repeated while the user's
production rc.1 application remained open. Clean-machine Windows 10/11 testing,
signing and broader user testing remain release checks.

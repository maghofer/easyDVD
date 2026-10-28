# First GitHub release preparation

The validated [preview.8](SOURCE-BUILT-TOOLS.md) replaces the earlier external-tools workaround with a source-built
portable helper bundle. The current recipe is scripts/toolchain/README.md;
DVDStyler is no longer required by the normal package.

Follow-up: the source/license inventory has now been reviewed in
[THIRD-PARTY-REVIEW.md](THIRD-PARTY-REVIEW.md). An external-tools coworker package
avoids redistributing the tool binaries with unresolved source/build provenance.
The hosted manual workflow now produces that variant. The bundled variant's
review remains incomplete.

Validated on 2026-10-28. Candidate: `0.1.0-preview.7`.

## Source and privacy audit

- No Git history or remote existed at the start of this audit. Nothing was pushed
  or published during preparation.
- Publishable source passes `scripts/audit-source.ps1`. No personal profile paths,
  workstation scratch paths, share paths, private keys or common token patterns
  were found. This is a pattern-based audit, not a guarantee against every secret.
- Workstation instructions and private background experiments are retained locally
  and ignored by Git, as are generated builds, media, tools and logo explorations.
- Public validation summaries omit local paths, hardware identification and private
  asset details. Remaining drive paths in documentation/tests are generic examples;
  application defaults derive from the current user's profile.
- Package reports whitelist portable module metadata, replace executable paths with
  package-relative paths and remove the build directory from build-info headers.
  Runtime documentation is selected from publishable Git files, excluding private notes.
- Runtime binaries were checked for this workstation's profile and scratch paths;
  none were found. An upstream TOML parser includes a generic Windows home-directory
  example in its error text; it is unrelated to this workstation or app defaults.
- The application source ZIP excludes local instructions, private background notes,
  tools, outputs and media. Runtime and source bundles build successfully.

## New behavior and verification

- VERSION drives the app title, Settings, CLI output, Windows version resource and
  package label. CLI output and Windows FileVersion/ProductVersion were verified.
- Gold disc/e artwork has an alpha background. The ICO has seven sizes from 16 to
  256 pixels. The icon extracted from the actual executable was visually checked.
- Settings shows a manual update button and current version. Its initial rendered
  layout was checked; opening a release page is unavailable before a successful check.
- Update-client tests cover semantic ordering, preview-to-stable updates, equal/ahead
  versions, missing releases, malformed/oversized responses, drafts/prereleases,
  GitHub errors, cancellation and invalid configuration. Response-supplied browser
  URLs are ignored. Unit tests, vet and race tests for the update client pass.
- Real integration passes PAL/NTSC pipelines, menu asset recovery, 36-title/four-page
  authoring and 99-title/99-page authoring using pinned media tools.
- GitHub workflow files provide validation on push/PR and separate manually triggered
  preview packaging. Neither publishes a release. Hosted runs remain untested until
  the repository is pushed to GitHub; local Windows equivalents passed.

## Remaining release work

The corresponding-source/license review for bundled external tools and runtime
DLLs is still pending. Clean Windows VM and physical-player checks, full-capacity
DVD-9/layer-break checks and other outstanding validation in RESULTS.md remain
open. These are existing limitations, not completed checks. See docs/RELEASING.md
for the source push, version/tag and deliberate release procedure.

No stable GitHub release exists for exercising an actual update end-to-end yet;
the API behavior was tested with local controlled HTTP responses.

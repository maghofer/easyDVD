# Release preparation

## How GitHub fits together

- A **push** saves commits to GitHub. The Test and build workflow runs tests and
  builds on pushes and pull requests; it does not create a release.
- **Actions → Build preview packages → Run workflow** manually builds downloadable
  portable ZIPs and SHA-256 files. Download them from the completed run's Artifacts section.
  These artifacts expire after 14 days. This workflow never publishes a release.
- A **tag**, such as `v0.1.0`, names a particular commit. A **release** is the
  public download page attached to a tag, with notes and permanent release assets.
  Creating a release is a separate, deliberate step in **Releases → Draft a new release**.

The intended repository is `maghofer/easyDVD`, configured in `RELEASE_REPOSITORY`.
Change that file if the actual repository name differs. No GitHub token or private
account information is embedded in the application.

## Version numbers and updates

`VERSION` is the source of the default application/package version. Keep it in
SemVer form without a `v` prefix: `0.1.0-preview.8`, then `0.1.0` for the first
stable release. Bump it when preparing a new release, not on every push. Existing
preview packages through preview.6 predate the embedded version system.
Use `0.1.1` for fixes, `0.2.0` for the next feature release during initial development.

Scripted builds embed this version in the GUI title, Settings, CLI `-version`,
Windows executable properties and package names. `-Version` overrides it for a
local candidate; public tags should always match the committed `VERSION`.
The manual workflow rejects a selected tag that does not match `v` plus `VERSION`.
Direct `go build` reads the same VERSION but does not add the Windows icon/resource;
use `scripts/build.ps1` for distributable executables.

The update button uses GitHub's public `/releases/latest` API with a 15-second
timeout. It compares semantic versions, including preview-to-stable progression.
It checks only when clicked, ignores prereleases and drafts, and handles missing
releases, offline errors and rate limits without changing app settings. GitHub
receives the normal request IP and easyDVD version in its User-Agent. Opening the
release page requires a second click. Nothing is installed by the checker.

## First push

1. Create an empty `easyDVD` repository under the `maghofer` GitHub account.
   Leave GitHub's initial README/license options unchecked; this repository has them.
2. Run `./scripts/audit-source.ps1`, then review `git status --short`.
   Local instructions, private asset experiments, tools, media and builds are ignored.
3. Commit the reviewed source and add the remote:

   ```powershell
   git add .
   git commit -m "Prepare easyDVD preview"
   git remote add origin https://github.com/maghofer/easyDVD.git
   git push -u origin main
   ```

4. Open Actions and inspect Test and build. To obtain a package, run Build preview
   packages manually. The workflow file must first be on the default branch.

The current hosted preview bundles three helper executables built from our pinned
source recipe. DVDStyler is not required. The matching sources ZIP includes the
helper/library source archives, local changes and build instructions automatically.
Use `-PublicRelease` to verify the corresponding-source material during packaging.
See [the helper recipe](../scripts/toolchain/README.md) and
[coworker setup](COWORKER-TEST.md).
After validation, commit the release version, tag that commit, and draft a GitHub
release for that tag. Attach the matching runtime ZIP, sources ZIP and hashes,
describe known limitations, and mark preview versions as **pre-release**. Preview
pages can be shared directly; stable-only update checks will not announce them.
Do not repoint a published tag or replace its assets with a different build.

GitHub references: [manual workflows](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow),
[workflow triggers](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow),
[latest release API](https://docs.github.com/en/rest/releases/releases#get-the-latest-release).

## Local preview package

```powershell
./scripts/bootstrap.ps1
./scripts/build.ps1 -Test -Integration
./scripts/package.ps1 -PublicRelease
```

This creates two ZIPs in `dist`: a small runtime package containing the GUI, CLI,
benchmark, three self-contained DVD helper programs, notices and manifests; and
a separate matching sources package containing application/Go sources and the
helper toolchain's source archives, patches, configuration and build scripts.
FFmpeg/FFprobe are excluded from the default runtime package. First run finds
a compatible external installation or offers an explicit private download.
No system-wide installation or PATH change is required. Packaging refuses to
overwrite existing output directories. Close the app before rebuilding its exe.

The authoring toolchain is built once into a reusable local/CI cache. A valid
cache is verified and reused on subsequent app builds and pushes. To rebuild the
helpers explicitly, run `scripts/build-authoring.ps1` with an isolated scratch
directory. Change the toolchain ID when updating released source/build inputs.

For a local offline validation candidate:

```powershell
./scripts/package.ps1 -IncludeFFmpeg
```

That produces an `-offline` runtime package which also includes the pinned FFmpeg
pair and its notices. Public offline packaging is blocked until the separate
FFmpeg/dependency source review is completed. The normal package's explicit
FFmpeg download/location setup remains available. Both variants include our
three DVD helper executables; their non-system libraries are linked into them.

## Before a public binary release

- Keep the matching sources ZIP with the runtime release. The helper build
  supplies the original source archives, local changes, notices and build recipe;
  packaging verifies their recorded hashes. FFmpeg offline bundles require their
  own complete source/build review before public distribution.
- Pass the clean-machine exercise in `VALIDATION.md` on Windows 10 and 11 x64.
  Removing PATH dependencies on this development machine is useful evidence,
  but is not a substitute for a clean VM.
- Test burned DVD-5 and DVD-9 media in representative standalone players,
  including menu return, Play All and aspect-ratio switching.
- Review actual film-scan output and complete the optional encoder comparison.
- Record the results, known limitations, tag, source archive, build information
  and published ZIP hashes. Code signing is optional for a preview and is not
  configured by this project.

```powershell
./scripts/package.ps1 -PublicRelease
```

The public-release switch uses the source folder generated by the helper build.
A maintainer can select another folder with `-CorrespondingSources` when needed.
`review.json` must declare `status: complete` and
map every exact binary name/hash to hash-verified source archives, license files,
build instructions and patch status (`none` or `included`). Paths are relative
to the supplied directory. Merely pointing at an empty directory is rejected.
`verify-toolchain-sources.ps1` checks this structure and its referenced files;
it cannot certify unsupported claims. The historical DVDStyler-derived candidate
manifest remains incomplete and is not used by this package. There is no automatic
publishing step.

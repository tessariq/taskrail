# Releasing

Checklist for cutting a tagged Taskrail release and shipping it to Homebrew and
WinGet.

Publishing a GitHub Release is automated; so is the WinGet PR (via GoReleaser),
but the Homebrew tap is not. Skipping the manual tap step leaves `brew
install`/`brew upgrade` pinned to the previous version even though the GitHub
Release is live — follow every step below.

## Source Of Truth

- `.github/workflows/release.yml` — tag-triggered release workflow; passes `WINGET_TOKEN` to GoReleaser
- `.goreleaser.yaml` — build/archive/checksum config, the `winget` publisher block, and the note that Homebrew is hand-maintained
- `CHANGELOG.md` — release notes source (`## v<version>` section)
- `scripts/check-changelog-version.sh`, `scripts/changelog-release-notes.sh` — release-note guards
- `tessariq/homebrew-tap` — `Formula/taskrail.rb`, bumped by hand each release
- `Tessariq/winget-pkgs` — fork of `microsoft/winget-pkgs`; GoReleaser pushes the manifest here and opens the upstream PR

## Automated: GitHub Release

1. Land the `## v<version>` section in `CHANGELOG.md` (move entries out of `## Unreleased`).
   The workflow refuses to publish if this section is missing or its notes are empty.
2. Tag and push:
   ```sh
   git tag v<version>
   git push origin v<version>
   ```
3. Confirm the `Release` workflow is green and the GitHub Release has every asset:
   ```sh
   gh release view v<version> --json assets --jq '.assets[].name'
   # expect: checksums.txt
   #       + taskrail_<version>_{darwin,linux}_{amd64,arm64}.tar.gz
   #       + taskrail_<version>_windows_{amd64,arm64}.zip
   ```

## Automated: WinGet (Windows)

GoReleaser's built-in `winget` publisher packages the Windows `.zip`
(`InstallerType: zip`, `NestedInstallerType: portable` — Taskrail is a single
binary, no MSI/NSIS) as the `Tessariq.Taskrail` manifest and opens a PR against
`microsoft/winget-pkgs`. It runs as part of the same tag-triggered release; no
separate step is needed once the prerequisites below are in place.

### One-time prerequisites

- **Fork:** `Tessariq/winget-pkgs` must exist as a fork of
  `microsoft/winget-pkgs`. GoReleaser pushes the version branch here and opens
  the upstream PR from it. (Provisioned by T-078.)
- **Secret:** repo secret `WINGET_TOKEN` — a **classic** PAT with `public_repo`
  scope, owned by an account that can push to the fork. The default
  `GITHUB_TOKEN` cannot open a cross-repository PR, so GoReleaser fails the
  winget step without it.

> **If the fork or `WINGET_TOKEN` isn't provisioned when you tag:** GoReleaser
> publishes the GitHub Release *first*, then runs the winget publisher, so a
> missing/invalid token fails only the winget leg — the `Release` workflow job
> goes **red** (log shows a winget git-push / PR-open auth error) even though the
> **GitHub Release and its assets are already live and valid**. Provision the
> prerequisites, then re-run the failed job to complete only the winget PR; do
> not re-tag. Until T-078 lands the fork and secret, expect this red-job window.

### First submission (bootstrap)

Chosen path: **let GoReleaser open the first PR** on the first tagged release —
no separate manual bootstrap. A brand-new package's initial manifest faces
longer, stricter moderation than version bumps, so expect the first PR to sit in
review longer. If the first automated PR stalls or the identifier needs manual
seeding, the fallback is a one-time manual submission with `wingetcreate` before
re-running the release; subsequent releases use the automated path either way.

### Latency and verification

The GitHub Release is live immediately, but `winget install Tessariq.Taskrail`
only works **after** Microsoft moderators review and merge the PR — typically
hours to a few days, longer for the first submission. Track the PR under
`Tessariq/winget-pkgs` / `microsoft/winget-pkgs`; this is a release-time outcome,
not a CI gate.

```sh
winget install Tessariq.Taskrail   # once the upstream PR is merged
taskrail version                   # -> v<version>
```

## Manual: Homebrew Tap

Not automated — GoReleaser's `brews` is deprecated and its `homebrew_casks`
replacement is macOS-only, which would drop the Linuxbrew support Taskrail's
linux builds rely on. Bump the formula by hand:

1. Fetch the checksums the release published:
   ```sh
   gh release download v<version> --repo tessariq/taskrail --pattern checksums.txt -O checksums.txt
   ```
2. In `tessariq/homebrew-tap`, edit `Formula/taskrail.rb`:
   - `version "<version>"`
   - all four asset URLs → `v<version>` / `taskrail_<version>_...`
   - all four `sha256` values from `checksums.txt` (darwin+linux × amd64+arm64)
3. Commit and push the tap:
   ```sh
   git commit -am "Update taskrail formula to v<version>"
   git push
   ```

## Verify

```sh
brew update && brew upgrade taskrail   # or: brew reinstall taskrail
taskrail version                       # -> v<version>
```

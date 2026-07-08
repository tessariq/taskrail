# Releasing

Checklist for cutting a tagged Taskrail release and shipping it to Homebrew.

Publishing a GitHub Release is automated; the Homebrew tap is not. Skipping the
manual tap step leaves `brew install`/`brew upgrade` pinned to the previous
version even though the GitHub Release is live — follow every step below.

## Source Of Truth

- `.github/workflows/release.yml` — tag-triggered release workflow
- `.goreleaser.yaml` — build/archive/checksum config and the note that Homebrew is hand-maintained
- `CHANGELOG.md` — release notes source (`## v<version>` section)
- `scripts/check-changelog-version.sh`, `scripts/changelog-release-notes.sh` — release-note guards
- `tessariq/homebrew-tap` — `Formula/taskrail.rb`, bumped by hand each release

## Automated: GitHub Release

1. Land the `## v<version>` section in `CHANGELOG.md` (move entries out of `## Unreleased`).
   The workflow refuses to publish if this section is missing or its notes are empty.
2. Tag and push:
   ```sh
   git tag v<version>
   git push origin v<version>
   ```
3. Confirm the `Release` workflow is green and the GitHub Release has all five assets:
   ```sh
   gh release view v<version> --json assets --jq '.assets[].name'
   # expect: checksums.txt + taskrail_<version>_{darwin,linux}_{amd64,arm64}.tar.gz
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

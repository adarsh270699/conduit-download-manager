# Releasing Conduit

Releases are built and published by GitHub Actions from a version tag. Nobody
uploads release files by hand. This page covers how versions are numbered,
where the number lives, and the exact steps for a release, a release
candidate, and a hotfix.

## Versioning

Conduit follows [Semantic Versioning](https://semver.org/):

| Change | Bump | Example |
| :--- | :--- | :--- |
| Breaking change to the local API, the config file, or the extension protocol; dropping a platform | MAJOR | 1.4.2 → 2.0.0 |
| New feature, new setting, new endpoint | MINOR | 1.4.2 → 1.5.0 |
| Bug fix only | PATCH | 1.4.2 → 1.4.3 |
| Release candidate for testing | prerelease suffix | 1.5.0-rc.1, 1.5.0-rc.2, then 1.5.0 |

### One source of truth

`daemon/version/VERSION` holds the version, for example `1.3.0` or
`1.3.0-rc.1`. Everything else follows it:

| Where | How it gets the version |
| :--- | :--- |
| Daemon, desktop app, `/api/health`, `--version` | `conduit/daemon/version` embeds the file. Builds add the git commit with `-ldflags`. |
| Windows installer | Build scripts pass `/DVERSION` and `/DVERSION_QUAD` to `makensis`. |
| Ubuntu `.deb` | Build scripts export `VERSION` for `nfpm`. A prerelease becomes `1.3.0~rc.1`. |
| `desktop/wails.json` `productVersion` | Copy of the numeric part, written by the release tool. |
| `extension/manifest.json` `version` | Copy of the numeric part, written by the release tool. Browsers accept only dotted numbers. |
| Git tag | `v` + VERSION. The release workflow refuses a tag that does not match. |

Never edit a version by hand. `go run ./tools/release bump` changes all of
them, and a unit test (`TestRepositoryVersionsAgree`) plus the CI `Checks` job
fail if any copy drifts.

### Changelog

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Add a line under `## [Unreleased]` in the same pull request as the change.
The bump moves those lines into the new version's section, and that section
becomes the GitHub Release notes. A bump with an empty Unreleased section is
refused.

## Branches

- `main` is always releasable. Every change lands through a pull request that
  passes CI.
- Releases are tags on `main`.
- A hotfix for an older line uses a `release/X.Y` branch cut from the
  `vX.Y.Z` tag. The release workflow accepts tags on `main` or `release/*`
  only.

## Cutting a release

1. **Prepare the bump** on a branch:

   ```bash
   git switch -c release/prepare-1.3.0
   cd daemon
   go run ./tools/release bump 1.3.0
   cd ..
   git diff            # VERSION, wails.json, manifest.json, CHANGELOG.md
   git commit -am "Release 1.3.0"
   ```

2. **Open a pull request** and merge it once CI is green.

3. **Tag the merge commit** on `main`:

   ```bash
   git switch main && git pull
   git tag -a v1.3.0 -m "Conduit 1.3.0"
   git push origin v1.3.0
   ```

4. **Watch the Release workflow.** It:
   - checks the tag against VERSION and CHANGELOG, and that the commit is on `main`;
   - builds and tests on Ubuntu 24.04 and Windows (unit, integration, desktop, `.deb` install, installer install, launch, and uninstall, and upgrade when an earlier release exists);
   - writes `SHA256SUMS.txt` and signed build provenance for every file;
   - creates a draft release, uploads everything, then publishes it as the latest release.

   If the `release` environment has required reviewers, the publish step waits
   for an approval.

5. **Check the release page** and install the published setup on a clean
   machine.

### Release candidates

Bump to `1.3.0-rc.1`, merge, tag `v1.3.0-rc.1`. The workflow publishes a
GitHub **prerelease**, which is never shown as the latest release. Fix,
bump to `-rc.2`, and repeat. When it is ready, bump to `1.3.0` (the
changelog section of the last candidate can be reused by moving its lines
back under Unreleased) and tag `v1.3.0`. [REVIEW REQUIRED] The release tool does not automate this step. Confirm the procedure before the first release candidate.

### Hotfix on an older line

```bash
git switch -c release/1.2 v1.2.0
# cherry-pick or commit the fix, with a CHANGELOG line
cd daemon && go run ./tools/release bump 1.2.1 && cd ..
git commit -am "Release 1.2.1"
git push origin release/1.2
git tag -a v1.2.1 -m "Conduit 1.2.1" && git push origin v1.2.1
```

Then bring the fix and the changelog section to `main` in a normal pull request.

### If a release run fails

- **Before Publish:** nothing is public. Fix the cause on `main`. If the
  version itself was wrong, delete the tag (`git push --delete origin v1.3.0`,
  `git tag -d v1.3.0`), merge the fix, and tag again.
- **During Publish:** re-run the failed jobs. The step uploads with
  `--clobber` and finishes the existing draft.
- **A published release is broken:** do not move or reuse its tag. Ship the fix
  as the next PATCH version.

## Local dry run

The staging scripts are what the release workflow runs, so they reproduce a
release locally without publishing anything:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare_release_windows.ps1
```

```bash
bash scripts/prepare_release_linux.sh
```

Files land in `dist/release/`. Do not upload them by hand.

`scripts/test_installer_windows.ps1` installs, launches, upgrades, and
uninstalls the built setup in the current user profile. It refuses to run when
Conduit is already installed (`-Force` overrides), so use a VM.
`scripts/test_deb.sh` does the same for the `.deb` with `sudo apt`.

## One-time repository setup

- **Settings → Environments → New environment → `release`.** Add yourself as a
  required reviewer to approve each publish.
- **Settings → Rules → Rulesets:** protect `main` (require pull requests and
  the `CI` checks) and protect tags matching `v*` (block deletion and updates).
- **Settings → Actions → General:** keep the default read-only `GITHUB_TOKEN`.
  The workflows ask for write access only in the publish job.

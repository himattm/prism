# Releasing a New Version of Prism

## Steps

### 1. Merge all feature PRs to main

Ensure all PRs for the release are merged and `main` is up to date.

### 2. Create a version bump PR

```bash
git checkout main && git pull
git checkout -b himattm/bump-X.Y.Z
```

Edit the single version source of truth:

```
internal/version/version.go
```

Update `const Version = "X.Y.Z"` to the new version number.

**Versioning convention:**
- **Major (X):** Breaking changes
- **Minor (Y):** New features (e.g., new plugins, new config options)
- **Patch (Z):** Bug fixes and small improvements

Commit and push:

```bash
git add internal/version/version.go
git commit -m "Bump version to X.Y.Z"
git push -u origin himattm/bump-X.Y.Z
gh pr create --title "Bump version to X.Y.Z" --body "Release X.Y.Z"
```

### 3. Merge the version bump PR

```bash
gh pr merge --squash --delete-branch
```

### 4. Release happens automatically

Once the version bump lands on `main`, CI handles everything:

1. **auto-tag.yml** — Detects the change to `internal/version/version.go`, creates a `vX.Y.Z` git tag, and triggers the release workflow
2. **release.yml** — Builds cross-platform binaries (darwin-arm64, darwin-amd64, linux-amd64, linux-arm64) and creates a GitHub release with auto-generated release notes

### 5. Verify

```bash
gh release view vX.Y.Z
```

Confirm the release has all 4 platform binaries attached.

### 6. Update local install (optional)

Users on the Max plan with auto-update enabled will get the new version automatically. To update manually:

```bash
prism update
```

For development, use the dev scripts:

```bash
# Install from local source (creates backup)
bash scripts/dev-install.sh

# Restore to latest release (cleans up backups)
bash scripts/dev-restore.sh
```

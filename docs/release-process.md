# Keep Core Release Process

## Automated Releases

Keep Core now supports fully automated releases through GitHub Actions. When you push a version tag, the system automatically:

1. Builds multi-platform binaries
2. Runs tests to ensure quality
3. Creates a GitHub release with artifacts
4. Generates release notes

## Creating a Release

### 1. Prepare the Release

Ensure you're on the main branch with all changes merged:

```bash
git checkout main
git pull origin main
```

### 2. Create and Push a Version Tag

```bash
# For a new patch release
git tag v2.1.1

# For a new minor release
git tag v2.2.0

# For a pre-release
git tag v2.2.0-rc.1

# Push the tag to trigger the release
git push origin v2.1.1
```

### 3. Monitor the Release

1. Go to the [Actions tab](../../actions) in GitHub
2. Watch the "Release" workflow complete
3. Check the [Releases page](../../releases) for the new release

## Release Artifacts

Each release automatically includes:

- **Linux AMD64 binary**: `keep-client-mainnet-{version}-linux-amd64.tar.gz`
- **macOS AMD64 binary**: `keep-client-mainnet-{version}-darwin-amd64.tar.gz`
- **Checksums**: `.md5` and `.sha256` files for verification

## Version Numbering

Follow [Semantic Versioning](https://semver.org/):

- **Patch** (`v2.1.1`): Bug fixes, security patches
- **Minor** (`v2.2.0`): New features, backwards compatible
- **Major** (`v3.0.0`): Breaking changes
- **Pre-release** (`v2.2.0-rc.1`): Release candidates, alpha/beta versions

## Pre-releases

Tags containing hyphens (e.g., `v2.2.0-rc.1`, `v2.2.0-alpha.1`) are automatically marked as pre-releases.

## Manual Release (Legacy)

If automatic releases fail, you can still create releases manually:

1. Use `workflow_dispatch` on the client workflow
2. Download artifacts from the workflow run
3. Create a GitHub release manually
4. Upload the downloaded artifacts

## Troubleshooting

### Release Workflow Fails

1. Check the Actions logs for specific errors
2. Ensure the tag follows the `v*` pattern
3. Verify tests are passing on the main branch

### Missing Artifacts

1. Check if the Docker build completed successfully
2. Verify the `output-bins` target in the Dockerfile
3. Ensure artifact paths match the workflow configuration

## Configuration

The release process is configured in:

- `.github/workflows/release.yml` - Main release automation
- `Makefile` - Build configuration and binary naming
- `Dockerfile` - Multi-stage build for binaries
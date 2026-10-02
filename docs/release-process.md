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
- **Sigstore bundles**: a `<artifact>.sigstore.json` keyless signature next to each tarball
- **Image digests**: `keep-client-{version}-image-digests.txt`, published with the release

The signed release build restores no shared Actions cache. A shared cache
could contain layers produced by an earlier, different run, so the provenance
attestation over the build could no longer describe what the build consumed.

## Artifact Verification

Release artifacts are signed keylessly with [Sigstore](https://www.sigstore.dev/)
during the release workflow run that builds them. No signing key is ever
provisioned: on GitHub Actions `cosign` exchanges the workflow's OIDC
identity into each signature. The certificate identity the signature carries
is the signing workflow's path plus the ref that triggered the run; it
proves *which* workflow and *which* tag ref produced the artifact, not one
specific execution. To bind a specific approved run, an operator verifies the
immutable tag commit and the attested build-provenance (the attested
source revision and run metadata) in addition to the identity and issuer:

- **OIDC issuer** - `https://token.actions.githubusercontent.com`
- **Certificate identity** - the signing workflow's path plus the triggering
  ref: `https://github.com/<owner>/<repo>/.github/workflows/release.yml@<ref>`

The `<ref>` for a tag release is the tag itself (`refs/tags/v2.6.0`), which
is the release's own trigger ref. The full identity the signature carries is
printed in the workflow run's log and written into the release notes, so an
operator can copy it from the release rather than reconstruct it.

### Verify a tarball

Download the tarball and its Sigstore bundle from the release, then:

```bash
cosign verify-blob \
  --bundle keep-client-mainnet-v2.6.0-linux-amd64.tar.gz.sigstore.json \
  --certificate-identity="https://github.com/threshold-network/keep-core/.github/workflows/release.yml@refs/tags/v2.6.0" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  keep-client-mainnet-v2.6.0-linux-amd64.tar.gz
```

A passing run prints the matching certificate identity and issuer. Both are
also checkable against the public transparency log with `cosign
verify-blob ... --trusted-root` or `sigstore bundle` tooling.

### Verify and pull a Docker image

Images are published by digest. The immutable digests are recorded in the
release's `keep-client-{version}-image-digests.txt` file and in the release
notes. Pull by digest, never by mutable tag:

```bash
pull_digest="$(grep -F 'keep-client:v2.6.0 ' keep-client-v2.6.0-image-digests.txt | awk '{print $2}')"
docker pull "thresholdnetwork/keep-client@${pull_digest}"

cosign verify \
  "thresholdnetwork/keep-client@${pull_digest}" \
  --certificate-identity="https://github.com/threshold-network/keep-core/.github/workflows/release.yml@refs/tags/v2.6.0" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

The pushed image also carries a SLSA build-provenance attestation (attached
with `provenance: mode=max`), which operators can inspect with `docker buildx
imagetools inspect` or pull the attestation by digest.

## Rehearsal Dry Run

There are two ways to run a dry run that builds, signs and verifies
everything but performs no live mutations (no Docker Hub push, no GitHub
release, no registry signing; only local artifacts, blob signatures and
build-provenance attestations are produced):

1. **Tag name rule (hard)**: any tag whose name contains `-rehearsal`
   forces a dry run in every job, whatever the `RELEASE_DRY_RUN` repository
   variable says. Pushing a tag such as `v0.0.0-rehearsal.1` is therefore
   always safe: it can never publish a release or push an image.
2. **Repository variable**: setting the Actions variable `RELEASE_DRY_RUN`
   to `true` forces a dry run for any tag.

Before a real release, push a disposable tag. The pipeline still builds
every artifact, signs and verifies the tarballs and a locally saved image,
and records GitHub attestations. It does not push an image to Docker Hub or
create a GitHub release. The saved image's SHA256 is not a registry manifest
digest; its digest file stays on the runner.

A dry run verifies local blob signing and provenance but cannot exercise a
registry-digest signature, registry attachment, or GitHub release upload.
These must be checked on a real tag push after operator approval.

To switch the repository between the two modes, set or clear the
`RELEASE_DRY_RUN` repository variable (Actions > Variables). Push a
disposable tag while it is `true`, or use a tag whose name contains
`-rehearsal`. Before pushing a distinct real release tag, unset the variable
and make sure the tag name does not contain `-rehearsal`; do not reuse a
rehearsal tag.

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
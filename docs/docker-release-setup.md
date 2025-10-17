# Docker Release Setup with Admin Gating

This document describes the setup required for automated Docker image publishing with admin approval gating.

## Overview

The release workflow now includes a gated Docker publishing step that requires approval from the `threshold-network/release-admin` team before Docker images are published to Docker Hub.

## Setup Requirements

### 1. GitHub Environment Protection

Set up the `keep-production` environment with protection rules:

1. Go to **Repository Settings** → **Environments**
2. Click **New environment** and name it `keep-production`
3. Configure protection rules:
   - ✅ **Required reviewers**: `threshold-network/release-admin`
   - ✅ **Prevent self-review**: Enabled
   - ✅ **Wait timer**: 0 minutes

### 2. Repository Secrets

Add the following secrets in **Repository Settings** → **Secrets and variables** → **Actions**:

| Secret Name | Description | Example |
|-------------|-------------|---------|
| `DOCKERHUB_USERNAME` | Docker Hub username | `keepnetwork` |
| `DOCKERHUB_TOKEN` | Docker Hub access token (not password) | `dckr_pat_...` |

#### Creating Docker Hub Token

**Important**: This must be created from the `keepnetwork` Docker Hub organization account, not a personal account.

1. Log in to [Docker Hub](https://hub.docker.com/) with the `keepnetwork` organization account
2. Go to **Account Settings** → **Security** → **Personal Access Tokens**
3. Click **Generate New Token**
4. Configure the token:
   - **Token description**: `threshold-keep-core-releases`
   - **Access permissions**: **Read, Write, Delete**
   - **Scope**: All repositories in `keepnetwork` organization (or specific to `keepnetwork/keep-client`)
5. Click **Generate**
6. **Copy the token immediately** (it won't be shown again)
7. Add it as `DOCKERHUB_TOKEN` secret in GitHub repository settings

**Note**: Personal Access Tokens from Docker Hub work similar to GitHub tokens - they're generated from your account settings and can have specific permissions and scopes. They're more secure than using your account password.

### 3. Team Permissions

Ensure the `threshold-network/release-admin` team has appropriate permissions:
- Repository access: **Write** or higher
- Can approve environment deployments

## Workflow Behavior

### Release Process

1. **Tag Creation**: Push a version tag (e.g., `v2.2.1`)
2. **Binary Build**: Workflow builds binaries and creates GitHub release
3. **Docker Gating**: Docker job starts and **waits for release-admin approval**
4. **Release Admin Approval**: Release admin team member approves the deployment
5. **Docker Publishing**: Images are built and pushed to Docker Hub

### Published Images

Upon approval, the following images are published:
- `keepnetwork/keep-client:latest`
- `keepnetwork/keep-client:v2.2.1` (version tag)
- `keepnetwork/keep-client:mainnet`

### Release Admin Approval Process

Release admin team members will receive notifications and can approve via:
- GitHub web interface (Environments tab)
- GitHub mobile app notifications
- Email notifications (if configured)

## Security Benefits

- ✅ **Manual verification** of production releases
- ✅ **Audit trail** of who approved each release
- ✅ **Separate credentials** for Docker Hub publishing
- ✅ **Environment isolation** between dev and production
- ✅ **Team-based approval** (not individual-based)

## Troubleshooting

### Docker Push Fails
- Verify `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets are set correctly
- Check Docker Hub token permissions include write access
- Ensure token hasn't expired

### Release Admin Can't Approve
- Verify user is member of `threshold-network/release-admin` team
- Check repository permissions for the release-admin team
- Confirm environment protection rules are configured correctly

### Workflow Stuck Waiting
- Check if release admin approval is pending in repository's **Environments** tab
- Verify the `keep-production` environment exists and has protection rules
- Review GitHub Actions logs for specific error messages

## Migration Notes

This setup maintains backward compatibility:
- Existing binary releases continue to work unchanged
- Docker publishing is additive (doesn't break existing processes)
- Manual Docker publishing still possible if needed
# Building keep-core for x86_64 Architecture

This guide explains how to build keep-core for x86_64 (amd64) architecture on Linux and macOS.

## Prerequisites

- Go 1.24 or later
- Make
- Node.js and npm (for contract artifacts)
- Git

## Quick Build

### For Linux x86_64

```bash
# Set environment variables
export GOOS=linux
export GOARCH=amd64

# Build using Makefile
make build
```

Or directly with Go:

```bash
GOOS=linux GOARCH=amd64 go build -o keep-client .
```

### For macOS x86_64 (Intel Macs)

```bash
# Set environment variables
export GOOS=darwin
export GOARCH=amd64

# Build using Makefile
make build
```

Or directly with Go:

```bash
GOOS=darwin GOARCH=amd64 go build -o keep-client .
```

## Full Build Process

The complete build process includes downloading contract artifacts, generating code, and building the binary.

### 1. Build with Contract Artifacts

Choose your environment (development, sepolia, mainnet, or local):

```bash
# For development environment
make development

# For Sepolia testnet
make sepolia

# For mainnet
make mainnet

# For local contracts
make local
```

This will:
- Download contract artifacts from NPM
- Generate Go code from contracts
- Build the client binary

### 2. Cross-Compile for Linux x86_64

To build specifically for Linux x86_64:

```bash
# Set version and revision (optional)
export version=$(git describe --tags --match "v[0-9]*" HEAD)
export revision=$(git rev-parse --short HEAD)

# Build for Linux x86_64
GOOS=linux GOARCH=amd64 make build \
    version=$version \
    revision=$revision
```

### 3. Build Multiple Platforms

The Makefile includes a `build_multi` target that builds for multiple platforms:

```bash
# Build for both Linux and macOS x86_64
make build_multi
```

This creates binaries in `out/bin/` directory:
- `keep-client-<environment>-<version>-linux-amd64`
- `keep-client-<environment>-<version>-darwin-amd64`

### 4. Create Release Packages

To build release packages with checksums:

```bash
make release
```

This creates:
- `out/bin/keep-client-<environment>-<version>-linux-amd64.tar.gz`
- `out/bin/keep-client-<environment>-<version>-darwin-amd64.tar.gz`
- MD5 and SHA256 checksum files

## Docker Build

### Using Docker Buildx (Recommended)

The project includes a build script that uses Docker:

```bash
./scripts/build.sh
```

This builds:
- Linux x86_64 binary in `out/bin/`
- Docker image tagged as `thresholdnetwork/keep-client:latest`

### Manual Docker Build

```bash
# Build Linux x86_64 binary
docker buildx build \
    --platform linux/amd64 \
    --output type=local,dest=./out/bin/ \
    --target=output-bins \
    --build-arg ENVIRONMENT=sepolia \
    --build-arg VERSION=$(git describe --tags --match "v[0-9]*" HEAD) \
    --build-arg REVISION=$(git rev-parse --short HEAD) \
    .

# Build Docker image
docker buildx build \
    --platform=linux/amd64 \
    --target runtime-docker \
    --tag thresholdnetwork/keep-client:latest \
    --build-arg ENVIRONMENT=sepolia \
    --build-arg VERSION=$(git describe --tags --match "v[0-9]*" HEAD) \
    --build-arg REVISION=$(git rev-parse --short HEAD) \
    .
```

## Architecture Notes

- **x86_64** and **amd64** are the same architecture in Go
- Use `GOARCH=amd64` for x86_64 builds
- The Makefile uses `amd64` as the architecture identifier
- Default platforms in Makefile: `linux/amd64` and `darwin/amd64`

## Build Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary for current platform |
| `make build_multi` | Build binaries for multiple platforms (Linux and macOS x86_64) |
| `make release` | Build binaries and create release packages with checksums |
| `make development` | Full build with development contract artifacts |
| `make sepolia` | Full build with Sepolia testnet contract artifacts |
| `make mainnet` | Full build with mainnet contract artifacts |
| `make local` | Full build with local contract artifacts |

## Troubleshooting

### Missing Contract Artifacts

If you get errors about missing contract artifacts:

```bash
# Download artifacts for your environment
make get_artifacts environment=sepolia
```

### Code Generation Issues

If generated code is out of date:

```bash
# Regenerate all code
make generate
```

### Cross-Compilation Issues

If cross-compilation fails, ensure:
- Go toolchain supports the target platform
- CGO is disabled (if not needed): `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`
- All dependencies are available for the target platform

## Verification

After building, verify the binary:

```bash
# Check binary architecture
file keep-client

# Expected output for Linux x86_64:
# keep-client: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), ...

# Expected output for macOS x86_64:
# keep-client: Mach-O 64-bit x86_64 executable, ...
```

## Example: Complete Build for Linux x86_64

```bash
# 1. Clone repository
git clone https://github.com/keep-network/keep-core.git
cd keep-core

# 2. Download dependencies
go mod download

# 3. Build with Sepolia contracts
make sepolia

# 4. Verify binary
file keep-client
./keep-client --version
```

## Related Documentation

- `README.adoc` - General project information
- `CONTRIBUTING.adoc` - Contribution guidelines
- `Makefile` - Build system reference

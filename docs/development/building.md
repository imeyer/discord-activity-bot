# Build Instructions

Complete guide for building the Discord Activity Bot using Bazel or Go directly.

## Prerequisites

### Required Tools
- **Go 1.24+** - Primary development language
- **Bazelisk** - Recommended build tool (manages Bazel versions automatically)
- **Git** - Version control and build stamping

### Optional Tools  
- **Docker/Podman** - Container builds
- **PostgreSQL 16+** - Local database for testing
- **TimescaleDB** - Time-series database extension

## Bazel Build (Recommended)

### Installing Bazelisk

**macOS:**
```bash
brew install bazelisk
```

**Linux:**
```bash
# Download latest release
curl -LO "https://github.com/bazelbuild/bazelisk/releases/latest/download/bazelisk-linux-amd64"
chmod +x bazelisk-linux-amd64
sudo mv bazelisk-linux-amd64 /usr/local/bin/bazelisk
```

**Windows:**
```powershell
# Using Chocolatey
choco install bazelisk

# Or download from GitHub releases
```

### Build Commands

**Basic Build:**
```bash
# Build for current platform
bazelisk build //cmd/discord-activity-bot:discord-activity-bot

# Build with version stamping (recommended)
bazelisk build //cmd/discord-activity-bot:discord-activity-bot --stamp

# Copy binary to current directory
cp bazel-bin/cmd/discord-activity-bot/discord-activity-bot_/discord-activity-bot .
```

**Cross-Platform Builds:**
```bash
# Build for Linux (container deployment)
bazelisk build //cmd/discord-activity-bot:discord-activity-bot \
  --stamp \
  --platforms=@rules_go//go/toolchain:linux_amd64

# Build for Windows
bazelisk build //cmd/discord-activity-bot:discord-activity-bot \
  --stamp \
  --platforms=@rules_go//go/toolchain:windows_amd64
```

**Makefile Shortcuts:**
```bash
# Build with version injection using Bazel
make build-bazel

# Build for Linux using Bazel  
make build-bazel-linux

# Show version information that would be built
make version
```

### Version Injection

Bazel builds automatically inject version information via build stamping:

**Version Sources:**
- **Git Tags**: `git describe --tags --exact-match HEAD` (preferred)
- **Git Describe**: `git describe --tags --always --dirty` (fallback)
- **Development**: `"dev"` (when not in git repository)

**Build Variables Injected:**
- `main.version` - Version string from git
- `main.buildDate` - ISO 8601 build timestamp  
- `main.gitCommit` - Full git commit hash

**Workspace Status Script**: [`tools/workspace_status.sh`](../../tools/workspace_status.sh)  
**Build Configuration**: [`cmd/discord-activity-bot/BUILD.bazel:19`](../../cmd/discord-activity-bot/BUILD.bazel#L19)

**Example Version Output:**
```bash
./discord-activity-bot --version
# Output:
# Discord Activity Bot
# Version:    v1.0.0
# Built:      2024-01-15T10:30:45Z
# Git Commit: abc123def456...
```

### Bazel Configuration

**Build Settings**: [`.bazelrc`](../../.bazelrc)
```bash
# Go specific settings
build --@rules_go//go/config:pure

# Workspace status script for build stamping  
build --workspace_status_command=tools/workspace_status.sh

# Platform-specific builds
build:linux --platforms=@rules_go//go/toolchain:linux_amd64
build:darwin --platforms=@rules_go//go/toolchain:darwin_amd64

# Release build optimization
build:release --compilation_mode=opt
build:release --strip=always
```

### Caching and Performance

**Local Caching:**
- Bazel automatically caches build artifacts
- Incremental builds are very fast
- Clean builds cache dependencies aggressively

**Remote Caching** (optional):
```bash
# Configure remote cache (example)
bazelisk build //... --remote_cache=grpc://build-cache.example.com
```

**Build Performance:**
- **Cold Build**: ~2-3 minutes (downloading dependencies)
- **Incremental Build**: ~10-30 seconds  
- **No-Change Build**: ~2-5 seconds

## Go Build (Alternative)

### Direct Go Builds

**Basic Build:**
```bash
go build -o discord-activity-bot ./cmd/discord-activity-bot
```

**Build with Version Injection:**
```bash
# Manual version variables
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")

# Build with ldflags
go build \
  -ldflags="-X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT} -s -w" \
  -o discord-activity-bot \
  ./cmd/discord-activity-bot
```

**Makefile Shortcuts:**
```bash
# Build with automatic version detection
make build

# Build for Linux  
make build-linux

# Show version information
make version
```

### Cross-Compilation with Go

**Linux Binary:**
```bash
GOOS=linux GOARCH=amd64 go build \
  -ldflags="-X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT} -s -w" \
  -o discord-activity-bot-linux \
  ./cmd/discord-activity-bot
```

**Windows Binary:**
```bash
GOOS=windows GOARCH=amd64 go build \
  -ldflags="-X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT} -s -w" \
  -o discord-activity-bot.exe \
  ./cmd/discord-activity-bot
```

**All Platforms:**
```bash
# Build matrix for multiple platforms
for GOOS in linux darwin windows; do
  for GOARCH in amd64 arm64; do
    echo "Building for $GOOS/$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build \
      -ldflags="-X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT} -s -w" \
      -o discord-activity-bot-$GOOS-$GOARCH \
      ./cmd/discord-activity-bot
  done
done
```

## Container Builds

### Building Container Images

**Using Pre-built Binary:**
```bash
# Build binary first (Linux target)
make build-bazel-linux
# or: make build-linux

# Build container image
docker build -f Containerfile -t discord-activity-bot .
```

**Multi-stage Build** (alternative approach):
```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o discord-activity-bot ./cmd/discord-activity-bot

# Runtime stage  
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/discord-activity-bot /usr/local/bin/discord-activity-bot
ENTRYPOINT ["/usr/local/bin/discord-activity-bot"]
```

### Container Configuration

**Base Image**: `gcr.io/distroless/static-debian12:nonroot`  
**User**: `nonroot:nonroot` (UID 65532)  
**Entrypoint**: `/usr/local/bin/discord-activity-bot`

**Container Labels:**
```dockerfile
LABEL org.opencontainers.image.title="Discord Activity Bot"
LABEL org.opencontainers.image.description="A Discord bot for tracking server activity"  
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.source="https://github.com/imeyer/discord-activity-bot"
```

**Security Features:**
- **Distroless Base**: Minimal attack surface (no shell, package manager)
- **Non-root User**: Runs as unprivileged user
- **Static Binary**: No dynamic dependencies
- **Read-only Filesystem**: Compatible with read-only root

## Development Builds

### Debug Builds

**Go Debug Build:**
```bash
go build -gcflags="all=-N -l" -o discord-activity-bot-debug ./cmd/discord-activity-bot
```

**Bazel Debug Build:**
```bash
bazelisk build //cmd/discord-activity-bot:discord-activity-bot \
  --compilation_mode=dbg \
  --strip=never
```

### Development Workflow

**Live Reload** (requires `entr`):
```bash
# Install entr on macOS
brew install entr

# Auto-restart on file changes
make dev
# or: find . -name "*.go" | entr -r go run ./cmd/discord-activity-bot
```

**Quick Development Cycle:**
```bash
# 1. Make code changes
# 2. Run tests
make test-bazel  # or: make test

# 3. Build and test locally
make build-bazel
./discord-activity-bot --help

# 4. Test with real Discord (requires token)
export DISCORD_TOKEN="your-dev-token"
export DATABASE_URL="postgres://localhost/discord_activity_dev"
./discord-activity-bot
```

## Build Troubleshooting

### Common Bazel Issues

**Cache Corruption:**
```bash
# Clean Bazel cache
bazelisk clean --expunge

# Rebuild dependencies
bazelisk sync
```

**Platform Issues:**
```bash
# Force platform specification
bazelisk build //cmd/discord-activity-bot:discord-activity-bot \
  --platforms=@rules_go//go/toolchain:linux_amd64 \
  --incompatible_enable_cc_toolchain_resolution
```

**Dependency Issues:**
```bash
# Update Go dependencies
go mod tidy

# Update Bazel dependencies  
bazelisk run //:gazelle-update-repos
```

### Common Go Issues

**Module Issues:**
```bash
# Clean module cache
go clean -modcache

# Verify dependencies
go mod verify

# Download dependencies  
go mod download
```

**Version Detection:**
```bash
# Test version script manually
./tools/workspace_status.sh

# Verify git status
git status
git describe --tags --always --dirty
```

### Build Performance

**Parallel Builds:**
```bash
# Bazel (automatic parallelization)
bazelisk build //... --jobs=auto

# Go (manual parallelization)  
go build -p $(nproc) ./cmd/discord-activity-bot
```

**Build Time Optimization:**
- Use Bazel for faster incremental builds
- Enable remote caching for team development
- Use `make build-bazel` for consistent versioning
- Keep dependency updates minimal during development

**Source Code References:**
- **Makefile**: [`Makefile:36`](../../Makefile#L36) - Build targets and version injection
- **Bazel BUILD**: [`cmd/discord-activity-bot/BUILD.bazel`](../../cmd/discord-activity-bot/BUILD.bazel) - Binary configuration
- **Workspace Status**: [`tools/workspace_status.sh`](../../tools/workspace_status.sh) - Version extraction script
- **Container Config**: [`Containerfile`](../../Containerfile) - Container build configuration
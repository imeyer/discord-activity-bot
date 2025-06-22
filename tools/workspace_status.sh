#!/bin/bash
# Workspace status script for Bazel build stamping

set -euo pipefail

# Try to get version from git tag, fallback to git describe, then to "dev"
if git describe --tags --exact-match HEAD >/dev/null 2>&1; then
  VERSION=$(git describe --tags --exact-match HEAD)
elif git describe --tags --always --dirty >/dev/null 2>&1; then
  VERSION=$(git describe --tags --always --dirty)
else
  VERSION="dev"
fi

# Get git commit
if git rev-parse HEAD >/dev/null 2>&1; then
  GIT_COMMIT=$(git rev-parse HEAD)
else
  GIT_COMMIT="unknown"
fi

# Build timestamp in ISO 8601 format
BUILD_TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Output stable build variables (these trigger rebuilds when changed)
echo "STABLE_BUILD_VERSION $VERSION"
echo "STABLE_BUILD_SCM_REVISION $GIT_COMMIT"

# Output volatile build variables (these don't trigger rebuilds)
echo "BUILD_TIMESTAMP $BUILD_TIMESTAMP"
echo "BUILD_USER $(whoami)"
echo "BUILD_HOST $(hostname)"
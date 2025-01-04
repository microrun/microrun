#!/bin/bash
set -euxo pipefail

# Create a named volume for the build directory if it doesn't exist to cache poky and build state
docker volume create kas-build-vol

# Set up the build directory and permissions
docker run --rm \
    -v kas-build-vol:/build \
    --user root \
    --entrypoint sh \
    ghcr.io/siemens/kas/kas:4.6 \
    -c 'mkdir -p /build/kas && chown 30000:30000 /build/kas'

# Run kas with:
# - Current directory mounted as /src (read-only)
# - Build volume mounted at /build
# - Copy files from /src to /work with correct ownership during copy to avoid host/guest permission issues
docker run --rm \
    -it \
    -v "$(pwd):/src:ro" \
    -v kas-build-vol:/build \
    -e KAS_BUILD_DIR=/build/kas \
    ghcr.io/siemens/kas/kas:4.6 \
    sh -c 'cp -a --no-preserve=ownership /src/. /builder && exec /container-entrypoint "$@"' \
    -- "$@"
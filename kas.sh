#!/bin/bash
set -euxo pipefail

# Create a named volume for the build directory if it doesn't exist
docker volume create kas-build-vol

# Set up the build directory and permissions
docker run --rm \
    -v kas-build-vol:/build \
    --user root \
    --entrypoint sh \
    ghcr.io/siemens/kas/kas:4.6 \
    -c 'mkdir -p /build/kas && chown -R 30000:30000 /build/kas'

# Run kas with:
# - Current directory mounted as /work
# - Build volume mounted at /build
# - Work directory set to /work
# - Build directory set to /build
docker run --rm \
    -v "$(pwd):/work" \
    -v kas-build-vol:/build \
    -w /work \
    -e KAS_WORK_DIR=/work \
    -e KAS_BUILD_DIR=/build/kas \
    -it \
    ghcr.io/siemens/kas/kas:4.6 \
    "$@"
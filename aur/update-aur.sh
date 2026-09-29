#!/bin/bash

# This script updates the PKGBUILD with the latest version and SHA256 sum.
# It is intended to be run during the GitHub Action release workflow.

set -eo pipefail

VERSION=$1
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    exit 1
fi

# Remove 'v' prefix if present
VERSION=${VERSION#v}

# Find the project root directory to allow running from anywhere
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PKGBUILD="${PROJECT_ROOT}/aur/PKGBUILD"

if [ ! -f "$PKGBUILD" ]; then
    echo "Error: PKGBUILD not found at $PKGBUILD"
    exit 1
fi

# Download the source tarball to calculate the checksum
SOURCE_URL="https://github.com/foonly/foondot/archive/refs/tags/v${VERSION}.tar.gz"
TEMP_FILE=$(mktemp)

echo "Downloading ${SOURCE_URL}..."
if ! curl -sSLf "$SOURCE_URL" -o "$TEMP_FILE"; then
    echo "Error: Failed to download source from $SOURCE_URL"
    rm -f "$TEMP_FILE"
    exit 1
fi

SHA256=$(sha256sum "$TEMP_FILE" | awk '{ print $1 }')
rm -f "$TEMP_FILE"

if [ -z "$SHA256" ]; then
    echo "Error: Failed to calculate SHA256 sum"
    exit 1
fi

echo "Updating PKGBUILD to version ${VERSION} with SHA256 ${SHA256}"

# Update pkgver
sed -i "s/^pkgver=.*/pkgver=${VERSION}/" "$PKGBUILD"
# Reset pkgrel to 1 for new versions
sed -i "s/^pkgrel=.*/pkgrel=1/" "$PKGBUILD"
# Update sha256sums
sed -i "s/^sha256sums=('.*')/sha256sums=('${SHA256}')/" "$PKGBUILD"

echo "PKGBUILD updated successfully."

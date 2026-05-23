#!/bin/bash
set -e

# Configuration
ROCKY_VERSION=${ROCKY_VERSION:-9}
ROCKY_ISO=${ROCKY_ISO:-"Rocky-${ROCKY_VERSION}-latest-x86_64-minimal.iso"}
CUSTOM_ISO=${CUSTOM_ISO:-"rocky-nodemanager.iso"}
KS_FILE=${KS_FILE:-"images/rocky/ks.cfg"}
BUILD_DIR=${BUILD_DIR:-"build"}

echo "Preparing Rocky Linux image build..."

# Check for base ISO
if [ ! -f "$ROCKY_ISO" ]; then
    echo "Base ISO $ROCKY_ISO not found."
    echo "You can download it with:"
    echo "curl -L -O https://download.rockylinux.org/pub/rocky/${ROCKY_VERSION}/isos/x86_64/${ROCKY_ISO}"
    exit 1
fi

mkdir -p "$BUILD_DIR"

# Check for mkksiso
if ! command -v mkksiso &> /dev/null; then
    echo "Error: mkksiso not found. Please install the 'lorax' package."
    echo "On Fedora/RHEL/Rocky: sudo dnf install lorax"
    exit 1
fi

echo "Building custom ISO with kickstart: $KS_FILE"
mkksiso --ks "$KS_FILE" "$ROCKY_ISO" "$BUILD_DIR/$CUSTOM_ISO"

echo "Success! Custom ISO created at $BUILD_DIR/$CUSTOM_ISO"

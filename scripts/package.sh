#!/bin/bash
set -e

VERSION=${1:-$(git describe --tags --always --dirty)}
PROJECT="tama"
DIST_DIR="dist"
ARCHIVES_DIR="dist/archives"

echo "Packaging ${PROJECT} ${VERSION}..."

# Create archives directory
mkdir -p ${ARCHIVES_DIR}

# Package each binary
for BINARY in ${DIST_DIR}/${PROJECT}-${VERSION}-*; do
    # Skip if not a file or already an archive
    [ -f "$BINARY" ] || continue
    [[ "$BINARY" == *.tar.gz ]] && continue
    [[ "$BINARY" == *.zip ]] && continue

    FILENAME=$(basename "$BINARY")

    # Extract platform from filename
    if [[ $FILENAME =~ windows ]]; then
        # Create zip for Windows
        ARCHIVE="${ARCHIVES_DIR}/${FILENAME%.exe}.zip"
        echo "Creating ${ARCHIVE}..."
        (cd ${DIST_DIR} && zip -q "../${ARCHIVE}" "${FILENAME}")
    else
        # Create tar.gz for Unix
        ARCHIVE="${ARCHIVES_DIR}/${FILENAME}.tar.gz"
        echo "Creating ${ARCHIVE}..."
        tar -czf "${ARCHIVE}" -C ${DIST_DIR} "${FILENAME}"
    fi

    echo "  ✓ $(basename ${ARCHIVE})"
done

echo ""
echo "Archives created in ${ARCHIVES_DIR}/"
ls -lh ${ARCHIVES_DIR}/

#!/bin/bash
set -e

VERSION=${1}

if [ -z "$VERSION" ]; then
    echo "Usage: ./scripts/release-all.sh v0.1.0"
    exit 1
fi

echo "======================================"
echo "  Full Release Process for ${VERSION}"
echo "======================================"
echo ""

# Run tests first
echo "Step 1/6: Running tests..."
go test ./...
echo "✓ Tests passed"
echo ""

# Build binaries
echo "Step 2/6: Building binaries..."
./scripts/build.sh ${VERSION}
echo ""

# Package archives
echo "Step 3/6: Creating archives..."
./scripts/package.sh ${VERSION}
echo ""

# Generate checksums
echo "Step 4/6: Generating checksums..."
./scripts/checksums.sh
echo ""

# Preview release notes
echo "Step 5/6: Generating release notes..."
echo ""
echo "======================================"
./scripts/release-notes.sh ${VERSION}
echo "======================================"
echo ""
read -p "Continue with GitHub release? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Release cancelled."
    exit 0
fi

# Create GitHub release
echo "Step 6/6: Creating GitHub release..."
./scripts/release.sh ${VERSION}

echo ""
echo "======================================"
echo "  Release complete! 🎉"
echo "======================================"

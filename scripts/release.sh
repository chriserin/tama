#!/bin/bash
set -e

VERSION=${1}
ARCHIVES_DIR="dist/archives"

if [ -z "$VERSION" ]; then
    echo "Usage: ./scripts/release.sh v0.1.0"
    exit 1
fi

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "Error: GitHub CLI (gh) is not installed"
    echo "Install from: https://cli.github.com/"
    exit 1
fi

echo "Creating GitHub release ${VERSION}..."

# Check if tag exists
if ! git rev-parse ${VERSION} >/dev/null 2>&1; then
    echo "Tag ${VERSION} does not exist. Creating..."
    git tag -a ${VERSION} -m "Release ${VERSION}"

    read -p "Push tag to GitHub? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git push origin ${VERSION}
    fi
fi

# Generate release notes from conventional commits
echo "Generating release notes..."
NOTES=$(./scripts/release-notes.sh ${VERSION})

# Create release
echo "Creating release on GitHub..."
gh release create ${VERSION} \
    --title "Release ${VERSION}" \
    --notes "$NOTES" \
    ${ARCHIVES_DIR}/*.tar.gz \
    ${ARCHIVES_DIR}/*.zip \
    ${ARCHIVES_DIR}/checksums.txt

echo ""
echo "✓ Release ${VERSION} created successfully!"
echo "View at: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases/tag/${VERSION}"

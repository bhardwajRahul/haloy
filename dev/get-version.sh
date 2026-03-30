#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR/.."

if [ -n "${HALOY_VERSION:-}" ]; then
    echo "$HALOY_VERSION"
    exit 0
fi

version=$(git -C "$REPO_ROOT" describe --tags --dirty --always --match 'v*' 2>/dev/null || true)
if [ -z "$version" ]; then
    version="dev"
fi

if [ -n "${HALOY_VERSION_SUFFIX:-}" ]; then
    version="${version}${HALOY_VERSION_SUFFIX}"
fi

echo "$version"

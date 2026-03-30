#!/usr/bin/env bash

set -e

CLI_BINARY_NAME=haloy
HOSTNAME=""
VERSION_OVERRIDE=""
VERSION_SUFFIX="-dev"

while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            if [ -z "${2:-}" ]; then
                echo "Missing value for --version"
                exit 1
            fi
            VERSION_OVERRIDE=$2
            shift 2
            ;;
        --version=*)
            VERSION_OVERRIDE=${1#*=}
            shift
            ;;
        --version-suffix)
            if [ -z "${2:-}" ]; then
                echo "Missing value for --version-suffix"
                exit 1
            fi
            VERSION_SUFFIX=$2
            shift 2
            ;;
        --version-suffix=*)
            VERSION_SUFFIX=${1#*=}
            shift
            ;;
        --no-dev-suffix)
            VERSION_SUFFIX=""
            shift
            ;;
        -*)
            echo "Unknown option: $1"
            exit 1
            ;;
        *)
            HOSTNAME=$1
            shift
            ;;
    esac
done

if [ -z "$HOSTNAME" ]; then
    echo "Usage: $0 [--version <value>] [--version-suffix <suffix>] [--no-dev-suffix] <host|user@host>"
    echo ""
    echo "This script builds and deploys the haloy CLI."
    echo ""
    echo "Options:"
    echo "  --version <value>         Override the embedded haloy version completely"
    echo "  --version-suffix <value>  Append a custom suffix to the detected version"
    echo "  --no-dev-suffix           Use the detected version without the default -dev suffix"
    exit 1
fi

# Use the current username from the shell unless a remote user is specified.
DEFAULT_USERNAME=$(whoami)
TARGET_HOST=${HOSTNAME##*@}
if [[ "$HOSTNAME" == *"@"* ]]; then
    SSH_TARGET=$HOSTNAME
else
    SSH_TARGET="${DEFAULT_USERNAME}@${HOSTNAME}"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
version=$(HALOY_VERSION="$VERSION_OVERRIDE" HALOY_VERSION_SUFFIX="$VERSION_SUFFIX" "$SCRIPT_DIR/get-version.sh")
echo "Building version: $version"

# Detect target platform
if [ "$TARGET_HOST" = "localhost" ] || [ "$TARGET_HOST" = "127.0.0.1" ]; then
    # Local deployment - detect current platform
    OS=$(uname -s)
    ARCH=$(uname -m)

    case "$OS" in
        "Darwin")
            GOOS="darwin"
            ;;
        "Linux")
            GOOS="linux"
            ;;
        *)
            echo "Unsupported OS: $OS"
            exit 1
            ;;
    esac

    case "$ARCH" in
        "x86_64")
            GOARCH="amd64"
            ;;
        "arm64")
            GOARCH="arm64"
            ;;
        "aarch64")
            GOARCH="arm64"
            ;;
        *)
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    echo "Building for local platform: $GOOS/$GOARCH"
else
    # Remote deployment - assume Linux amd64
    GOOS="linux"
    GOARCH="amd64"
    echo "Building for remote platform: $GOOS/$GOARCH"
fi

# Build the CLI binary using detected/default platform
# Using same flags as production: -s -w strips debug symbols, -trimpath for reproducible builds
BUILD_DIR=$(mktemp -d)
CLI_BUILD_PATH="$BUILD_DIR/$CLI_BINARY_NAME"

(
    cd "$REPO_ROOT"
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -trimpath -ldflags="-s -w -X 'github.com/haloydev/haloy/internal/constants.Version=$version'" -o "$CLI_BUILD_PATH" ./cmd/haloy
)

# Support localhost: If HOSTNAME is localhost (or 127.0.0.1), use local commands instead of SSH/SCP.
if [ "$TARGET_HOST" = "localhost" ] || [ "$TARGET_HOST" = "127.0.0.1" ]; then
    echo "Using local deployment for ${HOSTNAME}"

    LOCAL_BIN_DIR="$HOME/.local/bin"
    mkdir -p "$LOCAL_BIN_DIR"
    cp "$CLI_BUILD_PATH" "$LOCAL_BIN_DIR/$CLI_BINARY_NAME"

    # Make binary executable
    chmod +x "$LOCAL_BIN_DIR/$CLI_BINARY_NAME"
else
    ssh "$SSH_TARGET" "mkdir -p \$HOME/.local/bin"
    scp "$CLI_BUILD_PATH" "$SSH_TARGET":~/.local/bin/$CLI_BINARY_NAME
fi

rm -rf "$BUILD_DIR"

echo "Successfully built and deployed haloy CLI for $GOOS/$GOARCH."

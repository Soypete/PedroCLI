#!/bin/sh
# PedroCLI installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/Soypete/PedroCLI/main/install.sh | sh

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Detect OS and architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux*)
        OS="Linux"
        ;;
    Darwin*)
        OS="Darwin"
        ;;
    *)
        echo "${RED}Unsupported operating system: $OS${NC}"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        ARCH="x86_64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "${RED}Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

# Determine latest version
LATEST_VERSION=$(curl -sL https://api.github.com/repos/Soypete/PedroCLI/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo "${RED}Failed to determine latest version${NC}"
    exit 1
fi

echo "${GREEN}Installing PedroCLI ${LATEST_VERSION}${NC}"

# Download URL
DOWNLOAD_URL="https://github.com/Soypete/PedroCLI/releases/download/${LATEST_VERSION}/pedrocli_${OS}_${ARCH}.tar.gz"

echo "Downloading from: $DOWNLOAD_URL"

# Create temp directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Download and extract
if command -v curl > /dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o pedrocli.tar.gz
elif command -v wget > /dev/null 2>&1; then
    wget -q "$DOWNLOAD_URL" -O pedrocli.tar.gz
else
    echo "${RED}Neither curl nor wget found. Please install one of them.${NC}"
    exit 1
fi

tar -xzf pedrocli.tar.gz

# Determine installation directory
if [ "$(id -u)" = "0" ]; then
    # Running as root
    INSTALL_DIR="/usr/local/bin"
else
    # Running as user
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

# Install binaries
mv pedrocli "$INSTALL_DIR/"
mv pedrocli-server "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/pedrocli"
chmod +x "$INSTALL_DIR/pedrocli-server"

# Install example configs
CONFIG_DIR="$HOME/.pedrocli"
mkdir -p "$CONFIG_DIR"

if [ -f ".pedroceli.example.ollama.json" ]; then
    cp .pedroceli.example.ollama.json "$CONFIG_DIR/"
fi

if [ -f ".pedroceli.example.llamacpp.json" ]; then
    cp .pedroceli.example.llamacpp.json "$CONFIG_DIR/"
fi

# Cleanup
cd -
rm -rf "$TMP_DIR"

echo "${GREEN}✅ PedroCLI installed successfully!${NC}"
echo ""
echo "Binaries installed to: $INSTALL_DIR"
echo "Example configs copied to: $CONFIG_DIR"
echo ""

# Check if directory is in PATH
case ":$PATH:" in
    *":$INSTALL_DIR:"*)
        ;;
    *)
        echo "${YELLOW}⚠️  $INSTALL_DIR is not in your PATH${NC}"
        echo "Add it to your PATH by adding this to your shell profile:"
        echo ""
        echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
        echo ""
        ;;
esac

echo "Next steps:"
echo "1. Install Ollama: ${GREEN}curl -fsSL https://ollama.com/install.sh | sh${NC}"
echo "2. Pull a model: ${GREEN}ollama pull qwen2.5-coder:32b${NC}"
echo "3. Create config: ${GREEN}cp $CONFIG_DIR/.pedroceli.example.ollama.json ~/.pedroceli.json${NC}"
echo "4. Edit config to set your project path"
echo "5. Run: ${GREEN}pedrocli build -description \"your feature\"${NC}"
echo ""
echo "For help: ${GREEN}pedrocli help${NC}"
echo "Documentation: ${GREEN}https://github.com/Soypete/PedroCLI${NC}"

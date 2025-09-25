#!/bin/bash

# Quick development script for Tree-sitter Craft
# Use this for rapid grammar development and testing

set -e

echo "🚀 Tree-sitter Craft Development Build"
echo "======================================"

# Set up Docker/Podman for WASM builds (only if --wasm flag is used)
if [ "$1" = "--wasm" ]; then
    source ./setup-docker.sh
    if ! setup_docker; then
        echo "❌ Cannot build WASM without a container runtime"
        echo "🔄 Continuing with parser generation and tests only..."
        set -- # Clear arguments to skip WASM build
    fi
fi

# 1. Generate parser
echo "📝 Generating parser..."
npx tree-sitter generate

# 2. Run tests only (skip WASM build for speed)
echo "🧪 Running tests..."
if npx tree-sitter test; then
    echo "✅ Tests passed!"
else
    echo "❌ Tests failed"
    exit 1
fi

# 3. Optional: Quick WASM build and copy if requested
if [ "$1" = "--wasm" ]; then
    echo "🐳 Building WASM..."
    PATH=".:$PATH" npx tree-sitter build --wasm
    
    if [ -f "tree-sitter-craft.wasm" ]; then
        cp tree-sitter-craft.wasm ../vscode-extension/resources/
        echo "✅ WASM copied to VS Code extension resources"
    fi
fi

echo ""
echo "🎉 Development build complete!"
if [ "$1" != "--wasm" ]; then
    echo "💡 Use './dev.sh --wasm' to also build and copy WASM"
fi
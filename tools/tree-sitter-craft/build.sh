#!/bin/bash

# Tree-sitter Craft Build Script
# This script builds the grammar and copies the WASM file to the VS Code extension

set -e  # Exit on any error

echo "🔨 Building Tree-sitter Craft Grammar"
echo "======================================"

# Check if we're in the right directory
if [ ! -f "grammar.js" ]; then
    echo "❌ Error: grammar.js not found. Run this script from the tree-sitter-craft directory."
    exit 1
fi

# Set up Docker/Podman for WASM builds
source ./setup-docker.sh
if ! setup_docker; then
    echo "❌ Cannot proceed without a container runtime"
    exit 1
fi

# 1. Generate parser from grammar
echo "📝 Generating parser from grammar.js..."
npx tree-sitter generate

# 2. Build WASM using Docker/Podman
echo "🐳 Building WASM binary..."
PATH=".:$PATH" npx tree-sitter build --wasm

# 3. Verify WASM file was created
if [ ! -f "tree-sitter-craft.wasm" ]; then
    echo "❌ Error: WASM build failed - tree-sitter-craft.wasm not found"
    exit 1
fi

echo "✅ WASM build successful: tree-sitter-craft.wasm ($(du -h tree-sitter-craft.wasm | cut -f1))"

# 4. Copy WASM to VS Code extension resources directory
EXTENSION_RESOURCES="../vscode-extension/resources/"
if [ -d "$EXTENSION_RESOURCES" ]; then
    echo "📦 Copying WASM to VS Code extension resources..."
    cp tree-sitter-craft.wasm "$EXTENSION_RESOURCES/"
    echo "✅ Copied to: $EXTENSION_RESOURCES/tree-sitter-craft.wasm"
else
    echo "⚠️  Warning: VS Code extension resources directory not found at $EXTENSION_RESOURCES"
    echo "   You may need to manually copy tree-sitter-craft.wasm to the correct location"
fi

# 5. Test the parser
echo "🧪 Running parser tests..."
if npx tree-sitter test; then
    echo "✅ All tests passed!"
else
    echo "❌ Some tests failed - check the output above"
    exit 1
fi

echo ""
echo "🎉 Build complete!"
echo "   - Grammar generated: src/parser.c, src/grammar.json, src/node-types.json"
echo "   - WASM binary: tree-sitter-craft.wasm"
echo "   - Copied to VS Code extension"
echo "   - All tests passed"
echo ""
echo "💡 Next steps:"
echo "   - Rebuild VS Code extension: cd ../vscode-extension && pnpm run compile"
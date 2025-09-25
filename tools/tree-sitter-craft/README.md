# Tree-sitter Craft Grammar

Tree-sitter grammar for the Craft DSL (Domain Specific Language) used in the VS Code extension.

## 📁 Project Structure

### Version Control (✅ Include in Git)
- `grammar.js` - **Main grammar definition** (source of truth)
- `package.json` - Dependencies and build scripts
- `test/` - Test cases for grammar validation
- `build.sh` - Main build script
- `build-force.sh` - Build script (ignores test failures)
- `dev.sh` - Development build script
- `setup-docker.sh` - Docker/Podman detection script
- `README.md` - Documentation
- `.gitignore` - Excludes generated files

### Generated Files (❌ Do NOT version)
- `src/parser.c` - Generated C parser
- `src/grammar.json` - Generated grammar metadata
- `src/node-types.json` - Generated type definitions
- `tree-sitter-craft.wasm` - Generated WASM binary
- `docker` - Generated Podman wrapper
- `build/` - Build artifacts
- `node_modules/` - Dependencies

## 🚀 Quick Start

### For Grammar Development
```bash
# Quick iteration: generate + test (no WASM)
npm run dev

# With WASM build and copy to VS Code extension
npm run dev-wasm
```

### For Production Build
```bash
# Full build with WASM + copy to VS Code extension
npm run build-wasm
```

## 🔧 Development Workflow

### 1. Edit Grammar
Edit `grammar.js` to modify the language syntax.

### 2. Test Changes
```bash
# Quick test without WASM build
./dev.sh

# Test with WASM build and copy
./dev.sh --wasm
```

### 3. Add/Update Tests
Add test cases in `test/corpus/` directory.

### 4. Full Build
```bash
# Production build with all outputs
./build.sh
```

### 5. Update VS Code Extension
```bash
# Compile the updated extension
cd .. && pnpm run compile

# Test the formatter
node test-craft-formatter-final.js
```

## 📋 Build Scripts Explained

### `./build.sh` (Full Production Build)
1. Generates parser from `grammar.js`
2. Builds WASM binary using Docker/Podman
3. Copies WASM to VS Code extension
4. Runs all tests
5. Reports build status

### `./dev.sh` (Development Build)
1. Generates parser from `grammar.js`
2. Runs tests (no WASM build for speed)
3. Use `./dev.sh --wasm` to also build WASM

### Available npm Scripts
- `npm run dev` - Quick development build (no WASM)
- `npm run dev-wasm` - Development build with WASM
- `npm run build-wasm` - Full production build
- `npm run build-force` - Production build ignoring test failures
- `npm run test` - Run grammar tests only
- `npm run setup` - Test Docker/Podman detection and setup

## 🔄 File Flow

```
grammar.js (source)
    ↓ tree-sitter generate
src/parser.c (generated)
src/grammar.json (generated)
src/node-types.json (generated)
    ↓ tree-sitter build-wasm
tree-sitter-craft.wasm (generated)
    ↓ copy script
../vscode-extension/resources/tree-sitter-craft.wasm (VS Code extension)
```

## 📦 Dependencies

- `tree-sitter-cli` - Grammar generation and WASM building
- **Docker or Podman** - Required for WASM compilation (auto-detected)
- Node.js - For npm scripts and VS Code extension

### Container Runtime Setup
The build scripts automatically detect and set up Docker/Podman:
- Searches for `docker` in PATH using `which`
- Searches for `podman` in PATH using `which`  
- Checks common installation paths:
  - `/opt/podman/bin/podman` (macOS common location)
  - `/usr/local/bin/podman`
  - `/usr/bin/podman`
  - `$HOME/.local/bin/podman`
- Automatically creates `./docker` wrapper for podman if needed

Run `npm run setup` to test container runtime detection.

## 🧪 Testing

Tests are located in `test/corpus/` and follow Tree-sitter test format:

```
================
Test Name
================

input_code_here

---

(expected_ast_structure)
```

Run tests with:
```bash
npm run test
# or
tree-sitter test
```

## 🚨 Important Notes

### When you modify grammar.js:
1. **Always run tests first**: `./dev.sh`
2. **Build WASM for VS Code**: `./dev.sh --wasm`
3. **Recompile extension**: `cd .. && pnpm run compile`
4. **Test formatter**: `node test-craft-formatter-final.js`

### Git Workflow:
- Only commit `grammar.js`, `package.json`, tests, and build scripts
- **Never commit** generated files (`src/parser.c`, `*.wasm`, etc.)
- The `.gitignore` file handles this automatically

### 6 Months From Now:
1. `cd tools/vscode-extension/tree-sitter-craft`
2. `./build.sh` (full build)
3. `cd .. && pnpm run compile` (rebuild extension)
4. Done! 🎉
# Tree-sitter Craft Grammar

Tree-sitter grammar for the Craft DSL (Domain Specific Language). Available as both an npm package with Node.js bindings and WASM for browser/VS Code extension use.

## 📁 Project Structure

### Version Control (✅ Include in Git)
- `grammar.js` - **Main grammar definition** (source of truth)
- `package.json` - NPM package configuration and dependencies
- `binding.gyp` - Node.js native addon build configuration
- `bindings/node/` - Node.js binding source code
- `index.js` - Main package entry point
- `index.d.ts` - TypeScript definitions
- `test/` - Test cases for grammar validation
- `build.sh` - Main build script (WASM + Node.js)
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
- `build/` - Node.js native addon build artifacts
- `*.node` - Compiled native addon binaries
- `*.dylib` - Compiled dynamic libraries
- `docker` - Generated Podman wrapper
- `node_modules/` - Dependencies

## 🚀 Quick Start

### Using as NPM Package (Node.js Projects)

```bash
# Install from local directory
npm install /path/to/tree-sitter-craft

# Or from npm registry (if published)
npm install tree-sitter-craft
```

```javascript
const Parser = require('tree-sitter');
const Craft = require('tree-sitter-craft');

const parser = new Parser();
parser.setLanguage(Craft.language);

const sourceCode = `
use_case "User Registration" {
    when User creates account
        AuthService validates email format
        AuthService asks UserDatabase to store user data
}`;

const tree = parser.parse(sourceCode);
console.log('Parsed successfully:', tree.rootNode.type);
```

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
- `npm run build` - Build Node.js native addon (for npm package)
- `npm run build-wasm` - Full production build (WASM + Node.js)
- `npm run build-force` - Production build ignoring test failures
- `npm run test` - Run grammar tests only
- `npm run setup` - Test Docker/Podman detection and setup
- `npm install` - Install dependencies and build native addon

## 🔄 File Flow

### WASM Build (for VS Code Extension)
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

### Node.js Package Build
```
grammar.js (source)
    ↓ tree-sitter generate
src/parser.c (generated)
src/node-types.json (generated)
    ↓ node-gyp rebuild
bindings/node/binding.cc + src/parser.c
    ↓ compile to native addon
build/Release/tree_sitter_craft_binding.node
    ↓ node-gyp-build
index.js exports { language, nodeTypeInfo }
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

## 🔄 Generating New Versions

### When updating the grammar:

1. **Edit grammar.js** - Make your changes to the grammar definition
2. **Test changes**: `npm run dev` (quick iteration)
3. **Build all outputs**: `npm run build` (Node.js addon + WASM)
4. **Update VS Code extension**: 
   ```bash
   cd ../vscode-extension
   pnpm run compile
   ```
5. **Test in projects** - Test both npm package and VS Code extension
6. **Version bump**: `npm version patch|minor|major`
7. **Publish** (if publishing to npm): `npm publish`

### 6 Months From Now:
1. `cd tools/tree-sitter-craft`
2. `npm run build` (full build - Node.js + WASM)
3. `cd ../vscode-extension && pnpm run compile` (rebuild extension)
4. Done! 🎉

### What happens when generating a new version:
- `tree-sitter generate` creates new parser.c from grammar.js
- `node-gyp rebuild` compiles new Node.js native addon
- `tree-sitter build-wasm` creates new WASM binary  
- VS Code extension gets updated WASM file
- Both npm package and VS Code extension are ready to use
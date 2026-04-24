# LSP Distribution — Decision Log

> **Status:** Decisions finalised 2026-04-23.
> **Supersedes:** D2 and D3 in `lsp-migration-plan.md`; the S2 distribution-tracer description in `lsp-migration-tracer-bullets.md`.
> **Key change from original plan:** Multi-arch per-platform VSIX (6 VSIXs, each bundling a binary) is **replaced** by a single universal VSIX that downloads the correct binary at runtime (rust-analyzer style).

---

## Decision summary

| # | Decision | Chosen | Rationale |
|---|----------|--------|-----------|
| D-1 | Distribution mechanism | **Download on activation** (rust-analyzer style) | Single universal VSIX eliminates 6-VSIX CI matrix; no binary bloated in VSIX; architecture-specific binary downloaded transparently at runtime |
| D-2 | Binary resolution order | **1. `craft.lsp.executablePath` setting; 2. managed download** | User override escape hatch for offline/contributor scenarios; managed fallback for standard users |
| D-3 | Version coupling | **Strict: extension version == binary version** | Rust-analyzer approach; prevents client/server protocol mismatch; extension downloads from the exact matching GitHub Release tag |
| D-4 | Syntax highlighting | **TextMate grammar + LSP Semantic Tokens; delete Tree-sitter from VSCode client** | Industry standard (gopls, rust-analyzer, tsc); tree-sitter kept only in `tree-sitter-craft/` for Neovim/Helix/Zed/GitHub |
| D-5 | Diagram preview | **Keep HTTP `craft server` forever** | Binary blobs (PNG, SVG, PDF) are wrong over LSP; HTTP is the right tool; `previewCommon.ts` axios calls remain unchanged |
| D-6 | Download trigger | **Lazy — on first `.craft` file open** | Don't consume bandwidth for users not actively editing `.craft` files; matches rust-analyzer behaviour |
| D-7 | Download UX | **`vscode.window.withProgress` notification with version + byte progress** | A silent 5–10s download on slow connections looks like a broken extension; progress bar sets correct expectations |
| D-8 | Binary storage | **`context.globalStorageUri` with version-keyed subdirectory** | One shared copy per machine across all workspaces; path pattern: `craft-lsp/v0.1.0/<platform>/craft` |
| D-9 | Integrity check | **SHA256 checksum via GoReleaser's `checksums.txt`** | HTTPS alone doesn't catch partial/corrupt downloads; Node.js built-in `crypto` — ~10 lines |
| D-10 | Asset name translation | **TypeScript mapping table in `BinaryManager`** | GoReleaser standard names (`linux_amd64`) differ from Node.js names (`linux x64`); GoReleaser naming kept for Homebrew/ecosystem compatibility |
| D-11 | Download failure UX | **Graceful degrade: show error with "Retry" and "Set Binary Path" buttons; TextMate grammar stays alive** | TextMate coloring remains active; user can immediately fall back to `craft.lsp.executablePath` without hunting for the setting |
| D-12 | Release pipeline order | **`craft` tagged first → GoReleaser publishes binaries; then `craft-vscode-extension` tagged with same version** | Extension CI smoke-tests the download before publishing; sequencing rule documented in `AGENT.md` |
| D-13 | VSIX structure | **Single universal VSIX; CI smoke-tests the download before publishing** | No per-platform matrix; CI downloads `linux_amd64` binary, runs `craft lsp --stdio` handshake, then packages and publishes |
| D-14 | Binary auto-update | **Version-keyed path triggers fresh download automatically on extension update; old version directories deleted on startup** | ~8 MB per binary; silent accumulation across versions wastes disk; cleanup is a simple directory scan |
| D-15 | OpenVSX | **Publish alongside Marketplace from v0.1** | `OVSX_PAT` already added; Cursor (primary dogfood client) requires OpenVSX; one extra line in `release.yml` |
| D-16 | macOS Gatekeeper | **Path C: automated `xattr -dr com.apple.quarantine` immediately post-download (S2–S10); replaced by proper `codesign` + `notarytool` signing in S11b** | Rust-analyzer uses proper signing; we automate the workaround for dogfooding until Apple Developer enrollment is complete |
| D-17 | Code structure | **Dedicated `client/src/lsp/binaryManager.ts` module with single public function `resolveBinary(context): Promise<string>`** | All download/storage/checksum/quarantine complexity hidden behind one interface; `extension.ts` stays clean; testable in isolation |
| D-18 | New VS Code setting | **`craft.lsp.executablePath`** | Distinct from `craft.server.*` (HTTP diagram server); leaves room for future `craft.lsp.*` settings; unambiguous |

---

## What changes vs the original plan

### What is removed

| Original plan item | Status |
|---|---|
| D2: `.goreleaser.yml` builds 6 targets | **Unchanged** — GoReleaser config is not modified |
| D3: "download matching binaries, package 6 per-platform VSIXs, publish" | **Replaced** — single universal VSIX; no per-platform packaging |
| S2 distribution tracer: "per-platform VSIXs produced by CI" | **Replaced** — see updated S2 description below |
| `craftBinaryPath()` in `extension.ts` | **Deleted** — replaced by `BinaryManager.resolveBinary()` |

### What is added / changed

| Item | Change |
|---|---|
| `client/src/lsp/binaryManager.ts` | New module — all distribution logic lives here |
| `craft.lsp.executablePath` setting | Added to `package.json` `contributes.configuration` |
| `release.yml` | Add `npx ovsx publish` step; add CI smoke-test step; remove any per-platform VSIX logic |
| S2 distribution tracer | See updated description below |

---

## Updated S2 — Distribution tracer (replaces original)

**Goal:** prove the single-VSIX distribution pipeline works end-to-end before any LSP feature exists.

**What the VSIX contains:**
- `dist/client.js` — TypeScript extension (BinaryManager + LSP client spawn)
- `syntaxes/craft.tmLanguage.json` — TextMate grammar
- `resources/`, `language-configuration.json`, icons
- **No binary. Zero.**

**`BinaryManager.resolveBinary(context)` internal logic:**
```
1. Check craft.lsp.executablePath setting → if set and file exists, return it immediately.
2. Build expected path: globalStorageUri/craft-lsp/v{extensionVersion}/{platform}/craft
3. If binary exists at that path → return it (cache hit).
4. Show withProgress notification "Downloading Craft language server v{version}..."
5. Download checksums.txt from github.com/tcarcao/craft/releases/download/v{version}/checksums.txt
6. Download craft_{version}_{os}_{arch}.tar.gz (or .zip on Windows) from the same release
7. Verify SHA256 against checksums.txt → if mismatch, delete and throw.
8. Extract binary from archive to the path from step 2.
9. chmod +x (Linux/macOS).
10. On macOS: xattr -dr com.apple.quarantine <path>   ← automated Gatekeeper workaround until S11b
11. Delete any craft-lsp/v{oldVersion}/ directories (cleanup).
12. Return the path.
On any failure at steps 4–11: show error notification with "Retry" + "Set Binary Path" buttons.
```

**Platform → GoReleaser asset name mapping table (in `BinaryManager`):**
```typescript
const PLATFORM_MAP: Record<string, { os: string; arch: string; ext: string }> = {
  'darwin-arm64':   { os: 'darwin',  arch: 'arm64',  ext: 'tar.gz' },
  'darwin-x64':     { os: 'darwin',  arch: 'amd64',  ext: 'tar.gz' },
  'linux-arm64':    { os: 'linux',   arch: 'arm64',  ext: 'tar.gz' },
  'linux-x64':      { os: 'linux',   arch: 'amd64',  ext: 'tar.gz' },
  'win32-x64':      { os: 'windows', arch: 'amd64',  ext: 'zip'    },
};
// key: `${os.platform()}-${os.arch() === 'x64' ? 'x64' : os.arch()}`
```

**Updated `release.yml` delta from current:**
```yaml
# NEW: smoke-test the craft binary download before packaging
- name: Smoke-test craft binary download
  run: |
    VERSION=${GITHUB_REF#refs/tags/v}
    ASSET="craft_${VERSION}_linux_amd64.tar.gz"
    RELEASE_URL="https://github.com/tcarcao/craft/releases/download/v${VERSION}"
    curl -fL "${RELEASE_URL}/${ASSET}" -o craft.tar.gz
    EXPECTED=$(curl -fL "${RELEASE_URL}/checksums.txt" | grep "${ASSET}" | awk '{print $1}')
    ACTUAL=$(sha256sum craft.tar.gz | awk '{print $1}')
    [ "$EXPECTED" = "$ACTUAL" ] || (echo "Checksum mismatch" && exit 1)
    tar xzf craft.tar.gz && chmod +x craft
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":null,"capabilities":{}}}' \
      | timeout 5 ./craft lsp --stdio | grep -q '"id":1' || (echo "LSP handshake failed" && exit 1)

# UNCHANGED: single VSIX package
- name: Package extension
  run: npx @vscode/vsce package

# UNCHANGED: Marketplace publish
- name: Publish to VS Code Marketplace
  run: npx @vscode/vsce publish -p ${{ secrets.DEV_AZURE_PERSONAL_TOKEN }}

# NEW: OpenVSX publish
- name: Publish to OpenVSX
  run: npx ovsx publish -p ${{ secrets.OVSX_PAT }}
```

---

## VS Code Web path (documented, deferred to post-v0.1)

If VS Code Web support is ever needed:
- Go compiles to WASM via `GOOS=wasip1 GOARCH=wasm`
- `BinaryManager` detects `vscode.env.uiKind === vscode.UIKind.Web` and downloads `craft.wasm` instead
- WASM binary runs inside a Web Worker using `@vscode/wasm-wasi`
- `resolveBinary()` interface is unchanged — callers are unaffected

This path does not block v0.1.

---

## Sequencing rule (for `AGENT.md`)

> **Release sequencing — mandatory order:**
> 1. Tag `craft` with `vX.Y.Z` → wait for GoReleaser CI to complete and all GitHub Release assets to appear (the 5 tarballs + `checksums.txt`).
> 2. Only then tag `craft-vscode-extension` with the same `vX.Y.Z`.
>
> **Never** tag the extension before the Go binary GitHub Release is fully published. The extension CI downloads the `linux_amd64` binary and runs an LSP handshake as a smoke test — it will fail if the Release assets are missing.

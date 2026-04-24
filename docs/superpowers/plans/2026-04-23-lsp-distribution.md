# LSP Distribution (S2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the old per-platform VSIX approach with a single universal VSIX that downloads the correct `craft` binary at runtime (rust-analyzer style), while removing tree-sitter from the VSCode client.

**Architecture:** A new `BinaryManager` module (`client/src/lsp/binaryManager.ts`) owns all download/storage/checksum/quarantine logic behind a single `resolveBinary(context)` function. `extension.ts` calls it instead of the deleted `craftBinaryPath()`. CI drops the 6-platform VSIX matrix; the release pipeline gains a binary smoke-test and an OpenVSX publish step.

**Tech Stack:** TypeScript, Node.js built-ins (`node:https`, `node:fs/promises`, `node:crypto`, `node:child_process`), VS Code Extension API (`vscode.window.withProgress`, `context.globalStorageUri`), Jest + ts-jest ESM, `@vscode/vsce`, `ovsx`, GitHub Actions.

---

## File map

| Action | Path | Responsibility |
|--------|------|----------------|
| CREATE | `client/src/lsp/binaryManager.ts` | All download/storage/checksum/quarantine logic |
| CREATE | `client/src/lsp/binaryManager.test.ts` | Unit tests for BinaryManager |
| CREATE | `__mocks__/vscode.ts` | Manual jest mock for the `vscode` module |
| MODIFY | `jest.config.js` | Add `vscode` moduleNameMapper |
| MODIFY | `client/src/extension.ts` | Delete `craftBinaryPath()`, call `resolveBinary(context)` |
| MODIFY | `package.json` | Add `craft.lsp.executablePath` setting |
| MODIFY | `client/package.json` | Remove `web-tree-sitter` dependency |
| MODIFY | `.github/workflows/ci.yml` | Remove 6-platform matrix; add single-VSIX build job |
| MODIFY | `.github/workflows/release.yml` | Add smoke-test step; add OpenVSX publish step |
| DELETE | `resources/tree-sitter-craft.wasm` | No longer used in VSCode client |
| DELETE | `resources/queries/` | No longer used in VSCode client |

---

## Task 1: Jest vscode mock + moduleNameMapper

The `vscode` module is a VS Code runtime global — it doesn't exist in Node.js. Tests will blow up without a mock. We set that up before writing any extension code.

**Files:**
- Create: `__mocks__/vscode.ts`
- Modify: `jest.config.js`

- [ ] **Step 1: Create the vscode manual mock**

Create `__mocks__/vscode.ts` at the repo root (`craft-vscode-extension/__mocks__/vscode.ts`):

```typescript
export const window = {
  withProgress: jest.fn(),
  showErrorMessage: jest.fn(),
  setStatusBarMessage: jest.fn(),
};

export const workspace = {
  getConfiguration: jest.fn().mockReturnValue({
    get: jest.fn().mockReturnValue(''),
  }),
};

export const commands = {
  executeCommand: jest.fn(),
};

export const ProgressLocation = {
  Notification: 15,
  SourceControl: 1,
  Window: 10,
};

export const Uri = {
  file: jest.fn((p: string) => ({ fsPath: p })),
};
```

- [ ] **Step 2: Add vscode to jest.config.js moduleNameMapper**

In `jest.config.js`, update the `moduleNameMapper` object:

```js
moduleNameMapper: {
  '^vscode$': '<rootDir>/__mocks__/vscode.ts',
  '^(\\.{1,2}/.*)\\.js$': '$1',
},
```

- [ ] **Step 3: Verify the mock resolves**

Create a temporary smoke-test `client/src/lsp/binaryManager.test.ts` (just the scaffold — content is filled in Task 4):

```typescript
import { window } from 'vscode';

describe('vscode mock', () => {
  it('window mock is available', () => {
    expect(window.showErrorMessage).toBeDefined();
  });
});
```

- [ ] **Step 4: Run and confirm it passes**

```
npm test
```

Expected: `PASS client/src/lsp/binaryManager.test.ts` with 1 passing test.

- [ ] **Step 5: Commit**

```bash
git add __mocks__/vscode.ts jest.config.js client/src/lsp/binaryManager.test.ts
git commit -m "test: add vscode jest mock and moduleNameMapper"
```

---

## Task 2: Remove tree-sitter from VSCode client

Per D-4: tree-sitter stays only in `tree-sitter-craft/` for Neovim/Helix/Zed/GitHub. The VSCode extension uses TextMate grammar + LSP Semantic Tokens.

**Files:**
- Modify: `client/package.json`
- Delete: `resources/tree-sitter-craft.wasm`
- Delete: `resources/queries/` (entire directory)

- [ ] **Step 1: Remove web-tree-sitter from client/package.json**

In `client/package.json`, remove this line from `"dependencies"`:

```json
"web-tree-sitter": "^0.25.10"
```

The `dependencies` block should then contain only:

```json
"dependencies": {
  "@vscode/codicons": "^0.0.40",
  "axios": "^1.0.0",
  "react": "^18.3.1",
  "react-dom": "^18.3.1",
  "vscode-languageclient": "^9.0.1"
},
```

- [ ] **Step 2: Update the lock file**

```bash
npm install
```

Expected: `package-lock.json` updated, no errors.

- [ ] **Step 3: Delete tree-sitter resources**

```bash
rm resources/tree-sitter-craft.wasm
rm -rf resources/queries
```

- [ ] **Step 4: Verify the extension still bundles**

```bash
npm run bundle
```

Expected: completes without errors referencing tree-sitter or wasm.

- [ ] **Step 5: Commit**

```bash
git add client/package.json package-lock.json
git rm resources/tree-sitter-craft.wasm
git rm -r resources/queries
git commit -m "feat(D-4): remove tree-sitter from VSCode client"
```

---

## Task 3: Add craft.lsp.executablePath setting

Per D-18: distinct from `craft.server.*`; user escape hatch for offline/contributor scenarios.

**Files:**
- Modify: `package.json` (the root package.json at `craft-vscode-extension/package.json`)

- [ ] **Step 1: Add the setting to contributes.configuration.properties**

In `package.json`, inside `"contributes" > "configuration" > "properties"`, add after the `craft.logging.level` block:

```json
"craft.lsp.executablePath": {
  "type": "string",
  "default": "",
  "description": "Path to the craft language server binary. When set, the managed download is skipped and this binary is used directly. Leave empty to use the automatically downloaded binary."
}
```

- [ ] **Step 2: Verify package.json is valid JSON**

```bash
node -e "require('./package.json'); console.log('valid')"
```

Expected: `valid`

- [ ] **Step 3: Commit**

```bash
git add package.json
git commit -m "feat(D-18): add craft.lsp.executablePath setting"
```

---

## Task 4: BinaryManager — PLATFORM_MAP and unsupported platform

The platform map (D-10) translates Node.js platform/arch strings to GoReleaser asset naming. An unsupported platform must throw early with a clear message.

**Files:**
- Modify: `client/src/lsp/binaryManager.test.ts`
- Modify: `client/src/lsp/binaryManager.ts` (create if not present — Task 1 created a stub)

- [ ] **Step 1: Replace test stub with platform map tests**

Replace the full contents of `client/src/lsp/binaryManager.test.ts`:

```typescript
import type { ExtensionContext } from 'vscode';
import { PLATFORM_MAP, resolveBinary } from './binaryManager.js';

const mockContext = {
  extension: { packageJSON: { version: '0.1.0' } },
  globalStorageUri: { fsPath: '/fake/storage' },
} as unknown as ExtensionContext;

describe('PLATFORM_MAP', () => {
  it('contains entries for all 5 supported targets', () => {
    expect(Object.keys(PLATFORM_MAP)).toHaveLength(5);
  });

  it.each([
    ['darwin-arm64', { os: 'darwin',  arch: 'arm64', ext: 'tar.gz' }],
    ['darwin-x64',   { os: 'darwin',  arch: 'amd64', ext: 'tar.gz' }],
    ['linux-arm64',  { os: 'linux',   arch: 'arm64', ext: 'tar.gz' }],
    ['linux-x64',    { os: 'linux',   arch: 'amd64', ext: 'tar.gz' }],
    ['win32-x64',    { os: 'windows', arch: 'amd64', ext: 'zip'    }],
  ])('maps %s correctly', (key, expected) => {
    expect(PLATFORM_MAP[key]).toEqual(expected);
  });
});

describe('resolveBinary — unsupported platform', () => {
  it('throws when platform is not in PLATFORM_MAP', async () => {
    const deps = {
      existsSync: () => false,
      platform: () => 'freebsd',
      arch: () => 'x64',
    };
    await expect(resolveBinary(mockContext, deps)).rejects.toThrow(
      'Unsupported platform: freebsd-x64'
    );
  });
});
```

- [ ] **Step 2: Run — expect failure (module not found)**

```
npm test
```

Expected: `FAIL` — `Cannot find module './binaryManager.js'`

- [ ] **Step 3: Create binaryManager.ts with PLATFORM_MAP and skeleton**

Create `client/src/lsp/binaryManager.ts`:

```typescript
import * as https from 'node:https';
import * as fs from 'node:fs/promises';
import * as fsSync from 'node:fs';
import * as path from 'node:path';
import * as os from 'node:os';
import * as crypto from 'node:crypto';
import { execFile as execFileCb } from 'node:child_process';
import { promisify } from 'node:util';
import { ProgressLocation, commands, window, workspace } from 'vscode';
import type { ExtensionContext } from 'vscode';

const execFileAsync = promisify(execFileCb);

export const PLATFORM_MAP: Record<string, { os: string; arch: string; ext: string }> = {
  'darwin-arm64': { os: 'darwin',  arch: 'arm64', ext: 'tar.gz' },
  'darwin-x64':   { os: 'darwin',  arch: 'amd64', ext: 'tar.gz' },
  'linux-arm64':  { os: 'linux',   arch: 'arm64', ext: 'tar.gz' },
  'linux-x64':    { os: 'linux',   arch: 'amd64', ext: 'tar.gz' },
  'win32-x64':    { os: 'windows', arch: 'amd64', ext: 'zip'    },
};

export type BinaryManagerDeps = {
  existsSync?: (p: string) => boolean;
  downloadString?: (url: string) => Promise<string>;
  downloadFile?: (url: string, dest: string, onProgress: (r: number, t: number) => void) => Promise<void>;
  sha256File?: (p: string) => Promise<string>;
  execFile?: (cmd: string, args: string[]) => Promise<{ stdout: string; stderr: string }>;
  platform?: () => string;
  arch?: () => string;
  tmpdir?: () => string;
  mkdtemp?: (prefix: string) => Promise<string>;
  mkdir?: (p: string, opts?: { recursive?: boolean }) => Promise<string | undefined>;
  chmod?: (p: string, mode: number) => Promise<void>;
  readdir?: (p: string) => Promise<string[]>;
  rm?: (p: string, opts?: { recursive?: boolean; force?: boolean }) => Promise<void>;
};

export async function resolveBinary(context: ExtensionContext, deps: BinaryManagerDeps = {}): Promise<string> {
  const {
    existsSync = fsSync.existsSync,
    downloadString = _downloadString,
    downloadFile = _downloadFile,
    sha256File = _sha256File,
    execFile = execFileAsync,
    platform = () => os.platform() as string,
    arch = () => os.arch(),
    tmpdir = () => os.tmpdir(),
    mkdtemp = (prefix) => fs.mkdtemp(prefix),
    mkdir = (p, opts) => fs.mkdir(p, opts),
    chmod = (p, mode) => fs.chmod(p, mode),
    readdir = (p) => fs.readdir(p),
    rm = (p, opts) => fs.rm(p, opts),
  } = deps;

  // Step 1: User-configured binary path
  const config = workspace.getConfiguration('craft');
  const execPath = config.get<string>('lsp.executablePath', '');
  if (execPath && existsSync(execPath)) {
    return execPath;
  }

  // Step 2: Build expected path
  const version = context.extension.packageJSON.version as string;
  const resolvedArch = arch() === 'x64' ? 'x64' : arch();
  const platformKey = `${platform()}-${resolvedArch}`;
  const entry = PLATFORM_MAP[platformKey];
  if (!entry) {
    throw new Error(`Unsupported platform: ${platformKey}`);
  }

  const binaryName = platform() === 'win32' ? 'craft.exe' : 'craft';
  const binaryPath = path.join(
    context.globalStorageUri.fsPath,
    'craft-lsp',
    `v${version}`,
    platformKey,
    binaryName
  );

  // Step 3: Cache hit
  if (existsSync(binaryPath)) {
    return binaryPath;
  }

  // Steps 4–11: Download with progress
  try {
    await window.withProgress(
      {
        location: ProgressLocation.Notification,
        title: `Downloading Craft language server v${version}...`,
        cancellable: false,
      },
      async (progress) => {
        await _performDownload({
          context, version, entry, platformKey, binaryName, binaryPath,
          progress, downloadString, downloadFile, sha256File, execFile,
          tmpdir, mkdtemp, mkdir, chmod, readdir, rm,
          platformStr: platform(),
        });
      }
    );
  } catch (err) {
    // Step 12: Graceful degrade — error with Retry + Set Binary Path
    const choice = await window.showErrorMessage(
      `Failed to download Craft language server: ${(err as Error).message}`,
      'Retry',
      'Set Binary Path'
    );
    if (choice === 'Retry') {
      return resolveBinary(context, deps);
    }
    if (choice === 'Set Binary Path') {
      await commands.executeCommand('workbench.action.openSettings', 'craft.lsp.executablePath');
    }
    throw err;
  }

  return binaryPath;
}

type DownloadParams = {
  context: ExtensionContext;
  version: string;
  entry: { os: string; arch: string; ext: string };
  platformKey: string;
  binaryName: string;
  binaryPath: string;
  progress: { report(v: { message?: string; increment?: number }): void };
  platformStr: string;
  downloadString: (url: string) => Promise<string>;
  downloadFile: (url: string, dest: string, onProgress: (r: number, t: number) => void) => Promise<void>;
  sha256File: (p: string) => Promise<string>;
  execFile: (cmd: string, args: string[]) => Promise<{ stdout: string; stderr: string }>;
  tmpdir: () => string;
  mkdtemp: (prefix: string) => Promise<string>;
  mkdir: (p: string, opts?: { recursive?: boolean }) => Promise<string | undefined>;
  chmod: (p: string, mode: number) => Promise<void>;
  readdir: (p: string) => Promise<string[]>;
  rm: (p: string, opts?: { recursive?: boolean; force?: boolean }) => Promise<void>;
};

async function _performDownload(p: DownloadParams): Promise<void> {
  const { version, entry, binaryName, binaryPath } = p;
  const baseUrl = `https://github.com/tcarcao/craft/releases/download/v${version}`;
  const archiveName = `craft_${version}_${entry.os}_${entry.arch}.${entry.ext}`;

  // Step 5: Download checksums.txt
  p.progress.report({ message: 'fetching checksums...' });
  const checksums = await p.downloadString(`${baseUrl}/checksums.txt`);

  const expectedHash = checksums
    .split('\n')
    .map((l) => l.trim())
    .find((l) => l.endsWith(archiveName))
    ?.split(/\s+/)[0];

  if (!expectedHash) {
    throw new Error(`No checksum entry found for ${archiveName}`);
  }

  // Step 6: Download archive to temp dir
  const tmpDir = await p.mkdtemp(path.join(p.tmpdir(), 'craft-lsp-'));
  const archivePath = path.join(tmpDir, archiveName);

  try {
    p.progress.report({ message: 'downloading...' });
    await p.downloadFile(`${baseUrl}/${archiveName}`, archivePath, (received, total) => {
      if (total > 0) {
        p.progress.report({ message: `${Math.round((received / total) * 100)}%` });
      }
    });

    // Step 7: Verify SHA256
    const actualHash = await p.sha256File(archivePath);
    if (actualHash !== expectedHash) {
      throw new Error(
        `Checksum mismatch for ${archiveName}: expected ${expectedHash}, got ${actualHash}`
      );
    }

    // Step 8: Extract binary
    const binaryDir = path.dirname(binaryPath);
    await p.mkdir(binaryDir, { recursive: true });

    if (entry.ext === 'tar.gz') {
      await p.execFile('tar', ['xzf', archivePath, '-C', binaryDir, binaryName]);
    } else {
      await p.execFile('powershell', [
        '-Command',
        `Expand-Archive -Path "${archivePath}" -DestinationPath "${binaryDir}" -Force`,
      ]);
    }

    // Step 9: chmod +x (Linux/macOS)
    if (p.platformStr !== 'win32') {
      await p.chmod(binaryPath, 0o755);
    }

    // Step 10: macOS Gatekeeper workaround
    if (p.platformStr === 'darwin') {
      await p.execFile('xattr', ['-dr', 'com.apple.quarantine', binaryPath]);
    }

    // Step 11: Delete old version directories
    const lspDir = path.join(p.context.globalStorageUri.fsPath, 'craft-lsp');
    const currentVersionDir = `v${version}`;
    try {
      const entries = await p.readdir(lspDir);
      await Promise.all(
        entries
          .filter((e) => e !== currentVersionDir)
          .map((e) => p.rm(path.join(lspDir, e), { recursive: true, force: true }))
      );
    } catch {
      // lspDir may not exist on first download — that's fine
    }
  } finally {
    await p.rm(tmpDir, { recursive: true, force: true }).catch(() => {});
  }
}

export async function _sha256File(filePath: string): Promise<string> {
  const content = await fs.readFile(filePath);
  return crypto.createHash('sha256').update(content).digest('hex');
}

export function _downloadString(url: string): Promise<string> {
  return new Promise((resolve, reject) => {
    https
      .get(url, { headers: { 'User-Agent': 'craft-vscode-extension' } }, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          _downloadString(res.headers.location!).then(resolve, reject);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`HTTP ${res.statusCode} for ${url}`));
          return;
        }
        const chunks: Buffer[] = [];
        res.on('data', (c: Buffer) => chunks.push(c));
        res.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
        res.on('error', reject);
      })
      .on('error', reject);
  });
}

export function _downloadFile(
  url: string,
  dest: string,
  onProgress: (received: number, total: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    https
      .get(url, { headers: { 'User-Agent': 'craft-vscode-extension' } }, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          _downloadFile(res.headers.location!, dest, onProgress).then(resolve, reject);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`HTTP ${res.statusCode} for ${url}`));
          return;
        }
        const total = parseInt(res.headers['content-length'] ?? '0', 10);
        let received = 0;
        const ws = fsSync.createWriteStream(dest);
        res.on('data', (chunk: Buffer) => {
          received += chunk.length;
          onProgress(received, total);
        });
        res.pipe(ws);
        ws.on('finish', resolve);
        ws.on('error', reject);
        res.on('error', reject);
      })
      .on('error', reject);
  });
}
```

- [ ] **Step 4: Run tests — expect PLATFORM_MAP and unsupported platform tests to pass**

```
npm test
```

Expected: 7 passing tests (5 platform map entries + 1 length + 1 unsupported platform).

- [ ] **Step 5: Commit**

```bash
git add client/src/lsp/binaryManager.ts client/src/lsp/binaryManager.test.ts
git commit -m "feat: add BinaryManager with PLATFORM_MAP"
```

---

## Task 5: BinaryManager — executablePath override and cache hit

**Files:**
- Modify: `client/src/lsp/binaryManager.test.ts`

- [ ] **Step 1: Add tests for the two short-circuit paths**

Append to `describe` blocks in `binaryManager.test.ts`:

```typescript
describe('resolveBinary — executablePath setting override', () => {
  it('returns the configured path when it exists', async () => {
    const deps: BinaryManagerDeps = {
      existsSync: (p) => p === '/custom/craft',
      platform: () => 'linux',
      arch: () => 'x64',
    };
    // workspace.getConfiguration('craft').get mock must return '/custom/craft'
    const { workspace } = await import('vscode');
    (workspace.getConfiguration as jest.Mock).mockReturnValue({
      get: jest.fn().mockReturnValue('/custom/craft'),
    });

    const result = await resolveBinary(mockContext, deps);
    expect(result).toBe('/custom/craft');
  });

  it('falls through to download when executablePath is set but file does not exist', async () => {
    const { workspace } = await import('vscode');
    (workspace.getConfiguration as jest.Mock).mockReturnValue({
      get: jest.fn().mockReturnValue('/nonexistent/craft'),
    });

    const deps: BinaryManagerDeps = {
      existsSync: () => false,           // setting path does not exist
      platform: () => 'linux',
      arch: () => 'x64',
      downloadString: jest.fn().mockRejectedValue(new Error('download skipped in test')),
      mkdtemp: jest.fn(),
    };
    const { window } = await import('vscode');
    (window.withProgress as jest.Mock).mockResolvedValue(undefined);
    (window.showErrorMessage as jest.Mock).mockResolvedValue(undefined);

    await expect(resolveBinary(mockContext, deps)).rejects.toThrow();
  });
});

describe('resolveBinary — cache hit', () => {
  it('returns cached path when binary already exists at expected location', async () => {
    const { workspace } = await import('vscode');
    (workspace.getConfiguration as jest.Mock).mockReturnValue({
      get: jest.fn().mockReturnValue(''),
    });

    const expectedPath = '/fake/storage/craft-lsp/v0.1.0/linux-x64/craft';
    const deps: BinaryManagerDeps = {
      existsSync: (p) => p === expectedPath,
      platform: () => 'linux',
      arch: () => 'x64',
    };

    const result = await resolveBinary(mockContext, deps);
    expect(result).toBe(expectedPath);
  });
});
```

Also add the import at the top of the test file:

```typescript
import type { BinaryManagerDeps } from './binaryManager.js';
```

- [ ] **Step 2: Run — expect the new tests to pass**

```
npm test
```

Expected: all previous tests still pass + 3 new tests pass (1 override hit, 1 override miss, 1 cache hit).

- [ ] **Step 3: Commit**

```bash
git add client/src/lsp/binaryManager.test.ts
git commit -m "test: executablePath override and cache hit paths"
```

---

## Task 6: BinaryManager — SHA256 helper

**Files:**
- Modify: `client/src/lsp/binaryManager.test.ts`

- [ ] **Step 1: Add _sha256File test**

Append to `binaryManager.test.ts`:

```typescript
import { _sha256File } from './binaryManager.js';
import * as fsPromises from 'node:fs/promises';

describe('_sha256File', () => {
  it('computes the correct SHA256 hex digest', async () => {
    const content = Buffer.from('hello craft');
    jest.spyOn(fsPromises, 'readFile').mockResolvedValue(content as any);

    const result = await _sha256File('/any/path');

    const expected = require('node:crypto')
      .createHash('sha256')
      .update(content)
      .digest('hex');

    expect(result).toBe(expected);
  });
});
```

- [ ] **Step 2: Run — expect the test to pass** (the implementation is already in `binaryManager.ts`)

```
npm test
```

Expected: all tests pass including the new SHA256 test.

- [ ] **Step 3: Commit**

```bash
git add client/src/lsp/binaryManager.test.ts
git commit -m "test: _sha256File helper"
```

---

## Task 7: BinaryManager — full download happy path

This test exercises the complete download flow: checksums fetch → archive download → SHA256 verify → extract → chmod → quarantine workaround → old-version cleanup.

**Files:**
- Modify: `client/src/lsp/binaryManager.test.ts`

- [ ] **Step 1: Add the happy-path test**

Append to `binaryManager.test.ts`:

```typescript
describe('resolveBinary — full download happy path', () => {
  beforeEach(() => {
    const { workspace, window } = require('vscode');
    (workspace.getConfiguration as jest.Mock).mockReturnValue({
      get: jest.fn().mockReturnValue(''),
    });
    // withProgress calls the task function immediately
    (window.withProgress as jest.Mock).mockImplementation(
      (_opts: unknown, task: (progress: { report: jest.Mock }) => Promise<void>) =>
        task({ report: jest.fn() })
    );
  });

  it('downloads, verifies, extracts, and returns binary path on linux-x64', async () => {
    const archiveName = 'craft_0.1.0_linux_amd64.tar.gz';
    const checksumLine = `abc123def456  ${archiveName}`;

    const mockDownloadString = jest.fn().mockResolvedValue(checksumLine + '\n');
    const mockDownloadFile = jest.fn().mockResolvedValue(undefined);
    const mockSha256File = jest.fn().mockResolvedValue('abc123def456');
    const mockExecFile = jest.fn().mockResolvedValue({ stdout: '', stderr: '' });
    const mockMkdtemp = jest.fn().mockResolvedValue('/tmp/craft-lsp-xyz');
    const mockMkdir = jest.fn().mockResolvedValue(undefined);
    const mockChmod = jest.fn().mockResolvedValue(undefined);
    const mockReaddir = jest.fn().mockResolvedValue(['v0.1.0']); // only current version
    const mockRm = jest.fn().mockResolvedValue(undefined);

    const deps: BinaryManagerDeps = {
      existsSync: () => false,
      platform: () => 'linux',
      arch: () => 'x64',
      tmpdir: () => '/tmp',
      downloadString: mockDownloadString,
      downloadFile: mockDownloadFile,
      sha256File: mockSha256File,
      execFile: mockExecFile,
      mkdtemp: mockMkdtemp,
      mkdir: mockMkdir,
      chmod: mockChmod,
      readdir: mockReaddir,
      rm: mockRm,
    };

    const result = await resolveBinary(mockContext, deps);

    expect(result).toBe('/fake/storage/craft-lsp/v0.1.0/linux-x64/craft');

    // checksums URL
    expect(mockDownloadString).toHaveBeenCalledWith(
      'https://github.com/tcarcao/craft/releases/download/v0.1.0/checksums.txt'
    );

    // archive URL
    expect(mockDownloadFile).toHaveBeenCalledWith(
      'https://github.com/tcarcao/craft/releases/download/v0.1.0/craft_0.1.0_linux_amd64.tar.gz',
      '/tmp/craft-lsp-xyz/craft_0.1.0_linux_amd64.tar.gz',
      expect.any(Function)
    );

    // tar extraction
    expect(mockExecFile).toHaveBeenCalledWith('tar', [
      'xzf',
      '/tmp/craft-lsp-xyz/craft_0.1.0_linux_amd64.tar.gz',
      '-C',
      '/fake/storage/craft-lsp/v0.1.0/linux-x64',
      'craft',
    ]);

    // chmod
    expect(mockChmod).toHaveBeenCalledWith(
      '/fake/storage/craft-lsp/v0.1.0/linux-x64/craft',
      0o755
    );

    // tmp cleanup
    expect(mockRm).toHaveBeenCalledWith('/tmp/craft-lsp-xyz', { recursive: true, force: true });
  });

  it('runs xattr quarantine removal on darwin', async () => {
    const archiveName = 'craft_0.1.0_darwin_arm64.tar.gz';
    const checksumLine = `deadbeef  ${archiveName}`;

    const mockExecFile = jest.fn().mockResolvedValue({ stdout: '', stderr: '' });

    const deps: BinaryManagerDeps = {
      existsSync: () => false,
      platform: () => 'darwin',
      arch: () => 'arm64',
      tmpdir: () => '/tmp',
      downloadString: jest.fn().mockResolvedValue(checksumLine + '\n'),
      downloadFile: jest.fn().mockResolvedValue(undefined),
      sha256File: jest.fn().mockResolvedValue('deadbeef'),
      execFile: mockExecFile,
      mkdtemp: jest.fn().mockResolvedValue('/tmp/craft-lsp-abc'),
      mkdir: jest.fn().mockResolvedValue(undefined),
      chmod: jest.fn().mockResolvedValue(undefined),
      readdir: jest.fn().mockResolvedValue(['v0.1.0']),
      rm: jest.fn().mockResolvedValue(undefined),
    };

    await resolveBinary(mockContext, deps);

    expect(mockExecFile).toHaveBeenCalledWith('xattr', [
      '-dr',
      'com.apple.quarantine',
      '/fake/storage/craft-lsp/v0.1.0/darwin-arm64/craft',
    ]);
  });
});
```

- [ ] **Step 2: Run — expect the new tests to pass**

```
npm test
```

Expected: all tests pass including the two happy-path tests.

- [ ] **Step 3: Commit**

```bash
git add client/src/lsp/binaryManager.test.ts
git commit -m "test: full download happy path (linux + darwin quarantine)"
```

---

## Task 8: BinaryManager — error handling and old-version cleanup

**Files:**
- Modify: `client/src/lsp/binaryManager.test.ts`

- [ ] **Step 1: Add error-handling tests**

Append to `binaryManager.test.ts`:

```typescript
describe('resolveBinary — checksum mismatch', () => {
  it('throws when SHA256 does not match checksums.txt', async () => {
    const { workspace, window } = require('vscode');
    (workspace.getConfiguration as jest.Mock).mockReturnValue({
      get: jest.fn().mockReturnValue(''),
    });
    (window.withProgress as jest.Mock).mockImplementation(
      (_opts: unknown, task: (p: { report: jest.Mock }) => Promise<void>) =>
        task({ report: jest.fn() })
    );
    (window.showErrorMessage as jest.Mock).mockResolvedValue(undefined);

    const archiveName = 'craft_0.1.0_linux_amd64.tar.gz';
    const deps: BinaryManagerDeps = {
      existsSync: () => false,
      platform: () => 'linux',
      arch: () => 'x64',
      tmpdir: () => '/tmp',
      downloadString: jest.fn().mockResolvedValue(`expectedhash  ${archiveName}\n`),
      downloadFile: jest.fn().mockResolvedValue(undefined),
      sha256File: jest.fn().mockResolvedValue('actuallydifferenthash'),
      execFile: jest.fn().mockResolvedValue({ stdout: '', stderr: '' }),
      mkdtemp: jest.fn().mockResolvedValue('/tmp/craft-lsp-xyz'),
      mkdir: jest.fn().mockResolvedValue(undefined),
      chmod: jest.fn().mockResolvedValue(undefined),
      readdir: jest.fn().mockResolvedValue([]),
      rm: jest.fn().mockResolvedValue(undefined),
    };

    await expect(resolveBinary(mockContext, deps)).rejects.toThrow('Checksum mismatch');
  });
});

describe('resolveBinary — old-version cleanup', () => {
  it('removes old version directories but keeps the current one', async () => {
    const { workspace, window } = require('vscode');
    (workspace.getConfiguration as jest.Mock).mockReturnValue({
      get: jest.fn().mockReturnValue(''),
    });
    (window.withProgress as jest.Mock).mockImplementation(
      (_opts: unknown, task: (p: { report: jest.Mock }) => Promise<void>) =>
        task({ report: jest.fn() })
    );

    const archiveName = 'craft_0.1.0_linux_amd64.tar.gz';
    const mockRm = jest.fn().mockResolvedValue(undefined);
    const mockReaddir = jest.fn().mockResolvedValue(['v0.0.8', 'v0.0.9', 'v0.1.0']);

    const deps: BinaryManagerDeps = {
      existsSync: () => false,
      platform: () => 'linux',
      arch: () => 'x64',
      tmpdir: () => '/tmp',
      downloadString: jest.fn().mockResolvedValue(`abc123  ${archiveName}\n`),
      downloadFile: jest.fn().mockResolvedValue(undefined),
      sha256File: jest.fn().mockResolvedValue('abc123'),
      execFile: jest.fn().mockResolvedValue({ stdout: '', stderr: '' }),
      mkdtemp: jest.fn().mockResolvedValue('/tmp/craft-lsp-xyz'),
      mkdir: jest.fn().mockResolvedValue(undefined),
      chmod: jest.fn().mockResolvedValue(undefined),
      readdir: mockReaddir,
      rm: mockRm,
    };

    await resolveBinary(mockContext, deps);

    // Should remove v0.0.8 and v0.0.9 but NOT v0.1.0
    expect(mockRm).toHaveBeenCalledWith(
      '/fake/storage/craft-lsp/v0.0.8',
      { recursive: true, force: true }
    );
    expect(mockRm).toHaveBeenCalledWith(
      '/fake/storage/craft-lsp/v0.0.9',
      { recursive: true, force: true }
    );
    expect(mockRm).not.toHaveBeenCalledWith(
      '/fake/storage/craft-lsp/v0.1.0',
      expect.anything()
    );
  });
});

describe('resolveBinary — download failure UX', () => {
  it('shows error with Retry + Set Binary Path and re-calls resolveBinary on Retry', async () => {
    const { workspace, window } = require('vscode');
    (workspace.getConfiguration as jest.Mock).mockReturnValue({
      get: jest.fn().mockReturnValue(''),
    });

    let callCount = 0;
    (window.withProgress as jest.Mock).mockImplementation(
      (_opts: unknown, task: (p: { report: jest.Mock }) => Promise<void>) => {
        callCount++;
        if (callCount === 1) {
          return task({ report: jest.fn() }); // fails on first call
        }
        return Promise.resolve(); // succeeds on retry (won't reach here — test just checks retry path)
      }
    );

    const deps: BinaryManagerDeps = {
      existsSync: (p) => {
        // Return true on the second resolveBinary call (simulating successful retry finding cache)
        return callCount > 1 && p.includes('/fake/storage');
      },
      platform: () => 'linux',
      arch: () => 'x64',
      downloadString: jest.fn().mockRejectedValue(new Error('network error')),
      mkdtemp: jest.fn(),
      rm: jest.fn().mockResolvedValue(undefined),
    };

    // User clicks Retry
    (window.showErrorMessage as jest.Mock).mockResolvedValueOnce('Retry');

    // On retry, existsSync returns true so it returns from cache — no second download
    await expect(resolveBinary(mockContext, deps)).resolves.toBe(
      '/fake/storage/craft-lsp/v0.1.0/linux-x64/craft'
    );

    expect(window.showErrorMessage).toHaveBeenCalledWith(
      expect.stringContaining('Failed to download'),
      'Retry',
      'Set Binary Path'
    );
  });
});
```

- [ ] **Step 2: Run — expect all new tests to pass**

```
npm test
```

Expected: all tests pass.

- [ ] **Step 3: Commit**

```bash
git add client/src/lsp/binaryManager.test.ts
git commit -m "test: checksum mismatch, old-version cleanup, download failure UX"
```

---

## Task 9: Update extension.ts — delete craftBinaryPath, use resolveBinary

**Files:**
- Modify: `client/src/extension.ts`

- [ ] **Step 1: Remove craftBinaryPath and update startLanguageServer**

In `client/src/extension.ts`, remove lines 1–2 of the import block (`import * as os from 'os';` — it's only used by `craftBinaryPath`). Then:

1. Add the new import at the top:

```typescript
import { resolveBinary } from './lsp/binaryManager.js';
```

2. Delete the entire `craftBinaryPath` function (lines 35–54 of the current file).

3. Replace the `startLanguageServer` function body with:

```typescript
async function startLanguageServer(context: ExtensionContext) {
    const binary = await resolveBinary(context);
    Logger.info(`Craft LSP binary: ${binary}`);

    const serverOptions: ServerOptions = {
        run: {
            command: binary,
            args: ['lsp', '--stdio'],
            transport: TransportKind.stdio
        },
        debug: {
            command: binary,
            args: ['lsp', '--stdio', '--log-file', path.join(os.tmpdir(), 'craft-lsp-debug.log')],
            transport: TransportKind.stdio
        }
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [{ scheme: 'file', language: 'craft' }],
        synchronize: {
            fileEvents: workspace.createFileSystemWatcher('**/*.craft')
        },
        initializationOptions: {
            logLevel: workspace.getConfiguration('craft').get('logging.level', 'warn')
        }
    };

    client = new LanguageClient(
        'craftLanguageServer',
        'Craft Language Server',
        serverOptions,
        clientOptions
    );

    await client.start();
    Logger.info('Craft language server: connected');
    window.setStatusBarMessage('Craft language server: connected', 3000);
}
```

Note: keep `import * as os from 'os'` — it is still used by the `debug` server options for `os.tmpdir()`.

The final `extension.ts` import block should look like:

```typescript
import * as os from 'os';
import * as path from 'path';
import { workspace, ExtensionContext, window } from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';
import { registerPreviewCommands, cleanUpPreviewCommands } from './commands/index.js';
import { DomainsViewProvider } from './providers/domainsViewProvider.js';
import { DomainsViewService } from './services/domainsViewService.js';
import { ServicesViewProvider } from './providers/servicesViewProvider.js';
import { DslExtractService } from './services/dslExtractService.js';
import { ServicesViewService } from './services/servicesViewService.js';
import { Logger } from './utils/Logger.js';
import { resolveBinary } from './lsp/binaryManager.js';
```

- [ ] **Step 2: Type-check**

```bash
cd client && npx tsc --noEmit
```

Expected: exits 0 with no errors.

- [ ] **Step 3: Bundle**

```bash
cd .. && npm run bundle
```

Expected: `dist/client.js` produced with no errors.

- [ ] **Step 4: Commit**

```bash
git add client/src/extension.ts
git commit -m "feat: replace craftBinaryPath with BinaryManager.resolveBinary"
```

---

## Task 10: Update ci.yml — remove 6-platform matrix

The `build-vsix` job in `ci.yml` checks out the Go repo, cross-compiles 6 binaries, and packages 6 per-platform VSIXs. This entire job is replaced with a single universal-VSIX build that contains no binary.

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Replace the build-vsix job**

Replace the entire `build-vsix` job (lines 29–122 in the current file) with:

```yaml
  build-vsix:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Bundle extension
        run: npm run bundle

      - name: Package universal VSIX
        run: npx @vscode/vsce package --no-dependencies

      - name: Upload VSIX artifact
        uses: actions/upload-artifact@v4
        with:
          name: vsix-universal
          path: '*.vsix'
          retention-days: 30
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: replace 6-platform matrix with single universal VSIX build"
```

---

## Task 11: Update release.yml — smoke-test + OpenVSX

The release workflow needs two additions:
1. A binary smoke-test (download `linux_amd64` binary from the craft GitHub Release, verify SHA256, run `craft lsp --stdio` handshake) that gates packaging.
2. An OpenVSX publish step after the VS Code Marketplace publish.

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Add smoke-test step between "Build extension" and "Package extension"**

In the `create-release` job, after the `Build extension` step (currently line 69) and before the `Package extension` step (line 72), insert:

```yaml
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
          echo "Smoke-test passed"
```

- [ ] **Step 2: Add OpenVSX publish step after the Marketplace publish step**

After the existing `Publish to VS Code Marketplace` step (currently the last step), append:

```yaml
      - name: Publish to OpenVSX
        run: npx ovsx publish -p ${{ secrets.OVSX_PAT }}
        env:
          OVSX_PAT: ${{ secrets.OVSX_PAT }}
```

- [ ] **Step 3: Verify YAML is valid**

```bash
node -e "require('js-yaml').load(require('fs').readFileSync('.github/workflows/release.yml', 'utf8')); console.log('valid')" 2>/dev/null || python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/release.yml')); print('valid')"
```

Expected: `valid`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add binary smoke-test and OpenVSX publish to release workflow"
```

---

## Self-review

### Spec coverage check

| Decision | Covered by |
|----------|-----------|
| D-1 Download on activation | Task 4 — `resolveBinary` downloads if not cached |
| D-2 Binary resolution order (setting → managed) | Task 5 — setting override test |
| D-3 Strict version coupling | Task 4 — `context.extension.packageJSON.version` used as path key |
| D-4 TextMate + delete tree-sitter from VSCode | Task 2 — removes wasm, queries, web-tree-sitter dep |
| D-5 Diagram preview stays HTTP | Not touched — `previewCommon.ts` unchanged ✓ |
| D-6 Lazy download on first .craft file open | VS Code activates extension on language; `activate()` triggers `startLanguageServer` ✓ |
| D-7 withProgress notification | Task 4 implementation — `window.withProgress` with title and byte progress |
| D-8 Binary storage path pattern | Task 4 — `globalStorageUri/craft-lsp/v{version}/{platform}/craft` |
| D-9 SHA256 integrity via checksums.txt | Task 6 + Task 8 (mismatch test) |
| D-10 Asset name translation table | Task 4 — PLATFORM_MAP |
| D-11 Graceful degrade with Retry + Set Binary Path | Task 8 — download failure UX test |
| D-12 Release pipeline order | Task 11 smoke-test ensures craft binary Release exists before packaging |
| D-13 Single universal VSIX | Tasks 10, 11 — no per-platform packaging |
| D-14 Old version cleanup | Task 8 — cleanup test |
| D-15 OpenVSX | Task 11 — `npx ovsx publish` step |
| D-16 macOS Gatekeeper xattr | Task 7 — darwin quarantine test |
| D-17 `BinaryManager` module with `resolveBinary` | Task 4 |
| D-18 `craft.lsp.executablePath` setting | Task 3 |

All 18 decisions covered. No gaps.

### Placeholder scan

No TBDs, TODOs, or vague steps found.

### Type consistency check

- `BinaryManagerDeps` type defined in Task 4, imported in Task 5 (`import type { BinaryManagerDeps }`) — consistent.
- `resolveBinary(context, deps)` signature used identically across all tasks — consistent.
- `_sha256File`, `_downloadString`, `_downloadFile` exported and used as defaults in `deps` — consistent.
- `mockContext.extension.packageJSON.version = '0.1.0'` used in all task tests — consistent with the path assertions (all contain `v0.1.0`).

# Installation

## Prerequisites

- **VSCode** 1.75.0 or later
- **Node.js** 16+ (optional, for diagram server)

## Install VSCode Extension

### Option 1: VSCode Marketplace

1. Open VSCode
2. Press `Ctrl+Shift+X` (or `Cmd+Shift+X` on Mac) to open Extensions
3. Search for "**Craft**" or visit the [marketplace page](https://marketplace.visualstudio.com/items?itemName=tcarcao.craft-arch-diagrams)
4. Click **Install**

### Option 2: Command Line

```bash
code --install-extension craft
```

### Option 3: Manual Installation

1. Download the `.vsix` file from [GitHub Releases](https://github.com/tcarcao/craft-vscode-extension/releases)
2. Open VSCode
3. Go to Extensions (`Ctrl+Shift+X`)
4. Click the `...` menu → "Install from VSIX..."
5. Select the downloaded file

## Verify Installation

1. Create a new file: `test.craft`
2. Type:
   ```craft
   use_case "Test" {
     when user does something
   }
   ```
3. You should see syntax highlighting

## Install Diagram Server (Optional)

To generate diagrams, you need to run the Craft diagram server.

::: tip
If you just want to write Craft code with syntax highlighting, you can skip this step.
:::

### Using Docker (Recommended)

Run the latest Craft server directly from Docker Hub — no build needed:

```bash
docker run --rm -p 8080:8080 tiagocarcao/craft:latest
```

The server will start on `http://localhost:8080`. It is an HTTP API with no web page; `GET /health` answers `{"status":"ok"}`.

### Server settings

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | Port the server listens on |
| `CRAFT_BASE_PATH` | none (root) | Path prefix for every endpoint, for an ingress that routes by path and keeps the prefix. With `/craft`, the server answers `POST /craft/preview/c4` and `GET /craft/health`; point the extension's `craft.server.url` at `https://<host>/craft` |
| `CRAFT_LOG_FORMAT` | `text` | `json` writes one JSON object per line, for log collectors |
| `CRAFT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. Health checks log at `debug` only |
| `CRAFT_SOURCES` | none | Git sources file, see below |
| `CRAFT_CACHE` | a temporary folder | Where the server keeps its git source clones |

### Git sources (optional)

The server can also render models kept in git, by name, for clients such as the [Confluence macro](https://github.com/tcarcao/craft-confluence). List them in a JSON file and pass its path in `CRAFT_SOURCES`:

```bash
docker run --rm -p 8080:8080 \
  -e CRAFT_SOURCES=/config/sources.json -e CRAFT_CACHE=/cache \
  -e GITLAB_TOKEN=glpat-xxxxxxxx \
  -v "$PWD/sources.json:/config/sources.json:ro" -v craft-cache:/cache \
  tiagocarcao/craft:latest
```

The file format and the `/sources` endpoints are described in the [craft-confluence server guide](https://github.com/tcarcao/craft-confluence/blob/main/docs/server.md).

### From Source

```bash
# Clone the repository
git clone https://github.com/tcarcao/craft.git
cd craft

# Generate ANTLR parser (pulls tiagocarcao/antlr4-craft:4.13.2 from Docker Hub automatically)
make generate-grammar

# Build and run the server container
make docker-build && make docker-run
```

The server will start on `http://localhost:8080`.

## Configure Extension

Open VSCode settings (`Ctrl+,`) and search for "craft":

```json
{
  "craft.server.url": "http://localhost:8080",
  "craft.server.timeout": 30000,
  "craft.logging.level": "warn"
}
```

## Test Diagram Generation

1. Create a file `example.craft`
2. Add some content:
   ```craft
   services {
     UserService {
       contexts: Authentication
     }
   }
   ```
3. Press `Ctrl+Shift+C` to preview C4 diagram
4. If configured correctly, you'll see the generated diagram

::: warning
Make sure the diagram server is running before trying to preview diagrams.
:::

## Next Steps

- [Quick Start Tutorial](/guide/quickstart) to write your first use case
- [Extension Features](/extension/features) to learn all the capabilities
- [Examples](/examples/ecommerce) to see complete applications

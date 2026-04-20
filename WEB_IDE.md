# Craft Web IDE

The Craft project now includes a browser-based VS Code environment with the Craft extension pre-installed, allowing you to model and visualize your domain architecture directly in your browser.

## Overview

The web IDE integration consists of:
- **OpenVSCode Server**: Browser-based VS Code instance
- **Craft Extension**: Pre-installed with syntax highlighting and live preview
- **Go Backend**: Diagram generation API and reverse proxy

## Quick Start

### Using Docker Compose (Recommended)

1. **Build the services:**
   ```bash
   make compose-build
   ```

2. **Start the services:**
   ```bash
   # Foreground (see logs in terminal)
   make compose-up

   # Or background (detached mode)
   make compose-up-detached
   ```

3. **Access the IDE:**
   - Web IDE: http://localhost:8080/ide
   - Direct IDE access: http://localhost:3000
   - Server API: http://localhost:8080

4. **Stop the services:**
   ```bash
   make compose-down
   ```

### Architecture

```
┌─────────────────────────────────────────────────┐
│  Browser                                        │
├─────────────────────────────────────────────────┤
│  http://localhost:8080      → Web UI & API     │
│  http://localhost:8080/ide  → VS Code IDE       │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│  craft-server:8080 (Go)                         │
│  - Serves web UI                                │
│  - Provides diagram generation API              │
│  - Reverse proxies /ide/* to craft-ide:3000     │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│  craft-ide:3000 (OpenVSCode Server)             │
│  - Browser-based VS Code                        │
│  - Craft extension pre-installed                │
│  - Example .craft files in workspace            │
└─────────────────────────────────────────────────┘
```

## Features

### Pre-installed Craft Extension

The IDE comes with the Craft VS Code extension pre-installed, providing:

- **Syntax Highlighting**: Full syntax support for `.craft` files
- **Language Server**: Real-time validation and error checking
- **Live Preview Commands**:
  - `Ctrl+Shift+C` (or `Cmd+Shift+C`): Preview C4 diagram
  - `Ctrl+Shift+M` (or `Cmd+Shift+M`): Preview context map
  - `Ctrl+Shift+S` (or `Cmd+Shift+S`): Preview sequence diagram
  - `Ctrl+Shift+D` (or `Cmd+Shift+D`): Preview domain diagram

### Pre-configured Workspace

The workspace includes:
- Example `.craft` files in the `examples/` directory
- Pre-configured settings for Craft extension
- Welcome README with getting started guide
- Connection to the diagram generation server

### Persistent Workspace

Your workspace data is persisted using Docker volumes, so your files and changes are preserved between container restarts.

## Available Commands

### Docker Compose Commands

| Command | Description |
|---------|-------------|
| `make compose-build` | Build all services (server + IDE) |
| `make compose-up` | Start all services in foreground |
| `make compose-up-detached` | Start all services in background |
| `make compose-down` | Stop all services |
| `make compose-logs` | View logs from all services |
| `make compose-restart` | Restart all services |

### Traditional Docker Commands

You can still run just the server without the IDE:

| Command | Description |
|---------|-------------|
| `make docker-build` | Build server Docker image |
| `make docker-run` | Run server container |
| `make docker-clean` | Remove server image |

## Configuration

### Environment Variables

The services can be configured via environment variables in [docker-compose.yml](docker-compose.yml):

**craft-server:**
- `IDE_URL`: URL of the IDE service (default: `http://craft-ide:3000`)

**craft-ide:**
- `CRAFT_SERVER_URL`: URL of the Craft server (default: `http://craft-server:8080`)

### Workspace Settings

The IDE workspace settings are defined in [build/package/workspace-settings.json](build/package/workspace-settings.json):

```json
{
  "craft.serverUrl": "http://craft-server:8080",
  "craft.enableLivePreview": true,
  "files.associations": {
    "*.craft": "craft"
  }
}
```

## Troubleshooting

### IDE not accessible at /ide

1. Check if both services are running:
   ```bash
   podman-compose ps
   ```

2. Check logs for errors:
   ```bash
   make compose-logs
   ```

3. Verify the reverse proxy is working by accessing the IDE directly at http://localhost:3000

### Extension not installed

The extension is downloaded from GitHub releases during the Docker build. If the extension is missing:

1. Check if the `.vsix` file URL in [Dockerfile.openvscode](build/package/Dockerfile.openvscode) is correct
2. Verify you have internet access during the build
3. Rebuild the services:
   ```bash
   make compose-down
   make compose-build
   make compose-up-detached
   ```

### Diagrams not generating

1. Ensure the craft-server service is healthy:
   ```bash
   podman-compose logs craft-server
   ```

2. Check if PlantUML and Graphviz are installed in the server container
3. Verify the workspace settings have the correct `craft.serverUrl`

## Development

### Using Different Extension Versions

**By default**, the build automatically downloads the **latest** Craft extension release from GitHub. The version is detected dynamically, so you don't need to know the exact filename.

To use a **specific version**, set the `CRAFT_VERSION` environment variable:

**Option 1: Environment variable**
```bash
# Build with a specific version
CRAFT_VERSION=v0.1.0 make compose-build
```

**Option 2: Export variable**
```bash
# Set for your session
export CRAFT_VERSION=v0.1.0
make compose-build
```

**Option 3: Modify docker-compose.yml**
```yaml
craft-ide:
  build:
    args:
      CRAFT_VERSION: v0.1.0  # Pin to specific version
```

The Dockerfile automatically fetches the correct `.vsix` file from the GitHub release, regardless of the filename!

### Modifying Workspace Settings

Edit [build/package/workspace-settings.json](build/package/workspace-settings.json) and rebuild:

```bash
make compose-down
make compose-build
make compose-up-detached
```

### Adding More Extensions

You can add more VS Code extensions by modifying the Dockerfile:

```dockerfile
# Install additional extensions
RUN ${OPENVSCODE} --install-extension ms-vscode.vscode-typescript-next
RUN ${OPENVSCODE} --install-extension dbaeumer.vscode-eslint
```

## Next Steps

- Explore the example files in the `examples/` directory
- Create your own `.craft` files
- Use the preview commands to generate diagrams
- Check out the [main README](README.md) for more information about Craft

## Links

- [Craft Main Repository](https://github.com/tcarcao/craft)
- [Craft VS Code Extension](https://github.com/tcarcao/craft-vscode-extension)
- [OpenVSCode Server](https://github.com/gitpod-io/openvscode-server)

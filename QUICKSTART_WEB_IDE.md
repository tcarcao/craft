# Quick Start: Craft Web IDE

Get started with the Craft Web IDE in under 5 minutes!

## Prerequisites

- **Podman** or **Docker** installed
- **podman-compose** or **docker-compose** installed
- Internet connection (for downloading the Craft extension)

## Step 1: Build the Services

```bash
make compose-build
```

This will:
- Build the Craft server with diagram generation capabilities
- Build the OpenVSCode Server with the Craft extension pre-installed
- **Automatically download the latest** Craft VS Code extension from GitHub

**Note:** First build may take 5-10 minutes.

**Optional: Use a specific extension version**
```bash
# Build with a specific version instead of latest
CRAFT_VERSION=v0.1.0 make compose-build
```

## Step 2: Start the Services

```bash
make compose-up-detached
```

You should see:
```
✅ Services started!
   - Server: http://localhost:8080
   - Web IDE: http://localhost:3000
```

## Step 3: Open the Web IDE

Open your browser and navigate to:
```
http://localhost:3000
```

You'll see a full VS Code environment in your browser!

## Step 4: Try the Examples

1. In the IDE, open the file explorer (click the folder icon on the left)
2. Navigate to `examples/` directory
3. Open `user-management.craft` or `banking-system.craft`
4. Press `Ctrl+Shift+C` (or `Cmd+Shift+C` on Mac) to preview the C4 diagram

## Step 5: Create Your First Craft File

1. Create a new file: `my-domain.craft`
2. Start modeling your domain:

```craft
service "MyService" {
    domain "Orders" {
        usecase "Place Order" {
            trigger external "Customer"
            action sync "Validate Order"
            action async "Process Payment"
        }
    }
}
```

3. Press `Ctrl+Shift+C` to see your architecture diagram!

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+Shift+C` (Cmd+Shift+C) | Preview C4 diagram |
| `Ctrl+Shift+M` (Cmd+Shift+M) | Preview context map |
| `Ctrl+Shift+S` (Cmd+Shift+S) | Preview sequence diagram |
| `Ctrl+Shift+D` (Cmd+Shift+D) | Preview domain diagram |

## Managing the Services

**View logs:**
```bash
make compose-logs
```

**Stop services:**
```bash
make compose-down
```

**Restart services:**
```bash
make compose-restart
```

## Troubleshooting

**IDE not loading?**
- Check if services are running: `podman-compose ps`
- View logs: `make compose-logs`
- Try accessing directly: http://localhost:3000

**Extension not working?**
- Check the extension is installed: Look for "Craft" in the extensions panel
- Rebuild services: `make compose-down && make compose-build && make compose-up-detached`

**Diagrams not generating?**
- Check server logs: `podman-compose logs craft-server`
- Verify server is healthy: http://localhost:8080

## Next Steps

- Read [WEB_IDE.md](WEB_IDE.md) for detailed documentation
- Check out more examples in the `examples/` directory
- Visit the [Craft repository](https://github.com/tcarcao/craft) for language documentation

## Getting Help

- [GitHub Issues](https://github.com/tcarcao/craft/issues)
- [VS Code Extension Issues](https://github.com/tcarcao/craft-vscode-extension/issues)

Happy modeling! 🚀

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

```bash
docker run -p 8080:8080 tcarcao/craft-server
```

### From Source

```bash
# Clone the repository
git clone https://github.com/tcarcao/craft.git
cd craft

# Install dependencies
npm install

# Start the server
npm start
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
       domains: Authentication
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

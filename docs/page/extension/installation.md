# Extension Installation

Get started with the Craft VSCode Extension in minutes. This guide covers installation from the marketplace, command line, and manual installation from GitHub releases.

## Prerequisites

- **Visual Studio Code**: Version 1.75.0 or later
- **Craft Diagram Server**: Required for diagram generation features
  - See the [Installation Guide](/guide/installation) for server setup
  - Server should run on `http://localhost:8080` by default

## Install from VS Code Marketplace (Recommended)

![VS Code Extensions marketplace showing Craft Arch Diagrams extension with Install button](/images/extension/marketplace-install.png)

1. Open Visual Studio Code
2. Press `Ctrl+Shift+X` (Windows/Linux) or `Cmd+Shift+X` (Mac) to open the Extensions view
3. Search for **"Craft Arch Diagrams"**
4. Click the **Install** button
5. Wait for installation to complete
6. The extension activates automatically when you open a `.craft` file

## Install from Command Line

For automated or scripted installations:

```bash
code --install-extension tcarcao.craft-arch-diagrams
```

This is useful for:
- Setting up development environments
- Installing across multiple machines
- Containerized development setups

## Install from GitHub Releases

For pre-release versions or offline installation:

1. Visit the [Releases page](https://github.com/tcarcao/craft-vscode-extension/releases)
2. Download the latest `.vsix` file
3. In VS Code, press `Ctrl+Shift+P` (Windows/Linux) or `Cmd+Shift+P` (Mac)
4. Type "Extensions: Install from VSIX..." and select it
5. Choose the downloaded `.vsix` file
6. Click "Install" when prompted

## Verify Installation

![VS Code with Craft file and language](/images/extension/empty-craft-file.png)

After installation, verify everything works:

### Step 1: Create a Test File

Create a new file named `test.craft` with the following content:

```craft
actors {
    user Business_User
}

domain User {
    Authentication
}

use_case "User Login" {
  when Business_User authenticates
    Authentication validates credentials
    Authentication returns access token
}
```

![Basic Craft code with syntax highlighting](/images/extension/basic-syntax-highlighting.png)

### Step 2: Check Syntax Highlighting

You should immediately see:
- Keywords like `actors`, `domain`, `use_case` highlighted in purple/bold
- Domain names like `Authentication` in blue/bold
- Action verbs like `validates`, `returns` in orange/green
- Strings in colored text

If you see plain text instead:
1. Check the language mode in the status bar (bottom-right)
2. Click it and select "Craft Language" if not already selected
3. Reload VS Code if necessary: `Ctrl+Shift+P` → "Developer: Reload Window"

### Step 3: Test Auto-completion

Position your cursor on a new line and start typing:

- Type `use` → Should suggest `use_case` with a snippet template
- Type `dom` → Should suggest `domain`
- Press `Ctrl+Space` → Should show all available keywords

**[SCREENSHOT NEEDED: Auto-completion in action]**
*Show auto-completion dropdown appearing while typing*

### Step 4: Verify Sidebar Views

1. Click the Craft icon in the Activity Bar (left sidebar)
2. You should see two panels:
   - **Domain Manager**: Showing domains from your file
   - **Services**: Showing services (if defined)

If the sidebar doesn't appear:
1. Make sure you have a `.craft` file open
2. Try saving the file to trigger a refresh
3. Check View → Open View → Craft Domain Manager

### Step 5: Test Diagram Generation (Optional)

::: warning Server Required
This step requires the Craft diagram server running. If you haven't set it up yet, skip this step and refer to the [Installation Guide](/guide/installation).
:::

1. With `test.craft` open, press `Ctrl+Shift+D` (Windows/Linux) or `Cmd+Shift+D` (Mac)
2. A diagram preview panel should open beside your code
3. If you get a connection error, verify the server is running at `http://localhost:8080`

## Language Mode Indicator

**[SCREENSHOT NEEDED: Language mode selector]**
*Show the language mode indicator in status bar and the language selection dropdown*

The status bar (bottom-right) should show **"Craft Language"**. If it shows something else:

1. Click the language indicator
2. Select "Craft Language" from the dropdown
3. The extension will activate for that file

VS Code remembers your choice for `.craft` files, so you only need to do this once.

## Troubleshooting Installation

### Extension Not Activating

**Symptoms:** No syntax highlighting, no sidebar views, plain text display

**Solutions:**
1. Check that the file has a `.craft` extension
2. Verify the language mode is set to "Craft Language"
3. Reload VS Code: `Ctrl+Shift+P` → "Developer: Reload Window"
4. Disable and re-enable the extension: Extensions view → Craft → Disable → Enable

### Sidebar Views Not Appearing

**Symptoms:** Can't see Domain Manager or Services view

**Solutions:**
1. Click the Craft icon in the Activity Bar (left sidebar)
2. Go to View → Open View → Search for "Craft"
3. Ensure you have a `.craft` file open
4. Save your file to trigger a refresh

### Auto-completion Not Working

**Symptoms:** No suggestions when typing

**Solutions:**
1. Check that semantic highlighting is enabled
2. Look for "Craft Language Server" in Output panel (View → Output → Craft Language Server)
3. Manually trigger with `Ctrl+Space`
4. Reload the window if issues persist

### Diagram Commands Not Found

**Symptoms:** Can't find "Craft: Preview C4 Diagram" in Command Palette

**Solutions:**
1. Ensure the extension is installed and enabled
2. Open a `.craft` file to activate the extension
3. Check Extensions view to verify installation
4. Reinstall the extension if necessary

## Updating the Extension

The extension updates automatically when new versions are released on the VS Code Marketplace:

1. VS Code checks for updates periodically
2. When an update is available, you'll see a notification
3. Click "Update" to install the new version
4. Reload VS Code if prompted

To manually check for updates:
1. Go to Extensions view (`Ctrl+Shift+X`)
2. Click the "..." menu at the top
3. Select "Check for Extension Updates"

## Uninstalling

To remove the extension:

1. Open Extensions view (`Ctrl+Shift+X`)
2. Search for "Craft Arch Diagrams"
3. Click the gear icon → "Uninstall"
4. Reload VS Code when prompted

## Next Steps

Now that you have the extension installed:

- [Explore all features](/extension/features) - Learn about syntax highlighting, sidebar views, and diagram generation
- [Learn keyboard shortcuts](/extension/commands) - Master the command palette and shortcuts
- [Configure settings](/extension/configuration) - Customize server URL, timeouts, and more

::: tip First Time Users
Start with the [Quick Start Guide](/guide/quickstart) to learn the Craft language basics and create your first architecture diagram!
:::

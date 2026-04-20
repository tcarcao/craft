# Craft VSCode Extension - User Guide & Tutorial

## Table of Contents
1. [What is the Craft VSCode Extension?](#what-is-the-craft-vscode-extension)
2. [Installation](#installation)
3. [Getting Started](#getting-started)
4. [Core Features](#core-features)
5. [Advanced Features](#advanced-features)
6. [Configuration](#configuration)
7. [Tips & Best Practices](#tips--best-practices)
8. [Troubleshooting](#troubleshooting)

---

## What is the Craft VSCode Extension?

The Craft VSCode Extension is a comprehensive development tool for working with the Craft architecture modeling language. It provides an integrated development environment within Visual Studio Code for creating, editing, and visualizing software architectures using the Craft DSL (Domain-Specific Language).

### Key Capabilities

- **Syntax Highlighting**: Advanced Tree-sitter-based semantic highlighting that makes your Craft code more readable
- **Language Server Support**: Real-time validation, auto-completion, and code formatting
- **Interactive Visualizations**: Browse and explore your architecture through interactive tree views
- **Selective Diagram Generation**: Choose exactly which domains, services, and use cases to include in your diagrams
- **Configurable Diagrams**: Control diagram appearance with multiple visualization modes and options
- **Live Diagram Preview**: Generate and preview C4 diagrams, context maps, and domain diagrams
- **Intelligent Navigation**: Jump between domains, services, and use cases with ease

---

## Installation

### From VS Code Marketplace (Recommended)

**[SCREENSHOT: VS Code Extensions marketplace showing the Craft extension]**
*Screenshot description: Show the Extensions view (Ctrl+Shift+X) with "Craft Arch Diagrams" search results, highlighting the Install button*

1. Open Visual Studio Code
2. Press `Ctrl+Shift+X` (Windows/Linux) or `Cmd+Shift+X` (Mac) to open the Extensions view
3. Search for **"Craft Arch Diagrams"**
4. Click the **Install** button

**Alternative - Command Line:**
```bash
code --install-extension tcarcao.craft-arch-diagrams
```

### From GitHub Releases

1. Visit the [Releases page](https://github.com/tcarcao/craft-vscode-extension/releases)
2. Download the latest `.vsix` file
3. In VS Code, press `Ctrl+Shift+P` (Windows/Linux) or `Cmd+Shift+P` (Mac)
4. Type "Extensions: Install from VSIX..." and select it
5. Choose the downloaded `.vsix` file

---

## Getting Started

### Creating Your First Craft File

1. Create a new file with the `.craft` extension (e.g., `my-architecture.craft`)
2. Start writing your architecture definition using the Craft DSL

**[SCREENSHOT: Empty .craft file in VS Code]**
*Screenshot description: Show VS Code with a new file named "example.craft" open, with the Craft language mode indicator visible in the status bar*

### Basic Example

Here's a simple Craft architecture to get you started:

```craft
TODO
```

**[SCREENSHOT: Basic Craft code with syntax highlighting]**
*Screenshot description: Show the above example code with full syntax highlighting active, demonstrating different colors for keywords, service names, domain names, etc.*

---

## Core Features

### 1. Syntax Highlighting

The extension provides sophisticated semantic syntax highlighting powered by Tree-sitter. Different elements of your Craft DSL are highlighted with distinct colors:

- **Keywords** (purple, bold): `service`, `domain`, `arch`, `use_case`, `actors`, etc.
- **Service names** (blue, bold): Service identifiers
- **Domain names** (blue, bold): Domain identifiers
- **Properties** (red, bold): `domains`, `language`, `data-stores`, etc.
- **Action verbs** (orange/green, bold): `asks`, `notifies`, `listens`, `returns`
- **Strings** (various colors): Use case names, event strings, regular strings
- **Comments** (green, italic): Code comments

**[SCREENSHOT: Complex Craft file with full syntax highlighting]**
*Screenshot description: Show a more complex Craft file with multiple services, domains, use cases, and actors, demonstrating the full range of syntax highlighting*

### 2. Language Server Features

#### Auto-completion

As you type, the extension provides context-aware suggestions for:
- Keywords and properties
- Service and domain names
- DSL structure

**[SCREENSHOT: Auto-completion in action]**
*Screenshot description: Show the auto-completion dropdown appearing while typing, with relevant Craft keywords or identifiers*

Press `Ctrl+Space` to manually trigger auto-completion at any time.

#### Real-time Validation

The language server validates your Craft code as you type, highlighting errors and warnings.

**[SCREENSHOT: Validation errors highlighted]**
*Screenshot description: Show a Craft file with syntax errors underlined in red, with an error tooltip visible*

#### Code Formatting

Right-click in your `.craft` file and select **"Format Document"** or press `Shift+Alt+F` (Windows/Linux) or `Shift+Option+F` (Mac) to automatically format your code.

**[SCREENSHOT: Before and after formatting]**
*Screenshot description: Split view showing unformatted Craft code on the left and the same code formatted on the right*

### 3. Domain Manager Sidebar

The Domain Manager provides an interactive tree view of all domains in your workspace with powerful selection capabilities.

**[SCREENSHOT: Domain Manager sidebar view]**
*Screenshot description: Show the Craft icon in the Activity Bar and the Domain Manager panel open, displaying a hierarchical tree of domains with checkboxes*

#### Opening the Domain Manager

1. Click the Craft icon in the Activity Bar (left sidebar)
2. The Domain Manager panel will open showing all available domains

#### View Modes

Toggle between two view modes using the buttons at the top:

- **Current File**: Shows only domains from the currently active `.craft` file
- **Workspace**: Shows all domains from all `.craft` files in your workspace

**[SCREENSHOT: View mode toggle buttons]**
*Screenshot description: Close-up of the "Current File" and "Workspace" toggle buttons, with one highlighted as active*

#### Selecting Elements for Diagrams

The Domain Manager uses a hierarchical checkbox system:

**Domain Level:**
- Click a domain's checkbox to select/deselect all subdomains and use cases within it
- A checkmark (✓) means all items are selected
- A filled square (▣) means some items are selected (partial selection)

**Subdomain Level:**
- Click a subdomain's checkbox to select/deselect all use cases within it
- Subdomain selection automatically updates the parent domain's state

**Use Case Level:**
- Click individual use cases to include them in your diagram
- Use case selections cascade up to update subdomain and domain states

**[SCREENSHOT: Hierarchical selection in action]**
*Screenshot description: Show a domain tree with some domains fully selected (✓), some partially selected (▣), and some unselected (○)*

#### Quick Selection Toolbar

Use the toolbar buttons for rapid selection:

- **Select All** (checklist icon): Select all domains, subdomains, and use cases
- **Clear Selection** (clear icon): Deselect everything
- **Select Current File** (file icon): Select only items from the active file

**[SCREENSHOT: Toolbar buttons highlighted]**
*Screenshot description: Show the toolbar with all three selection buttons clearly visible*

#### Selection Counter

The bottom of the header shows your current selection:
- Number of use cases selected
- Number of subdomains selected
- Number of domains selected

**[SCREENSHOT: Selection counter display]**
*Screenshot description: Show the selection counter showing something like "12 use cases • 5 subdomains • 2 domains"*

#### Diagram Options

Click the **gear icon** to expand diagram configuration options:

**Mode Options:**
- **Detailed**: Shows complete domain diagram with all use cases and their interactions
- **Architecture**: Shows only subdomain-to-subdomain connections without use case details

**[SCREENSHOT: Domain diagram options panel expanded]**
*Screenshot description: Show the expanded options panel with "Detailed" and "Architecture" mode buttons*

#### Generating Domain Diagrams

1. Select the domains, subdomains, or use cases you want to visualize
2. Configure the diagram mode (Detailed or Architecture)
3. Click the **Preview** button (eye icon) in the toolbar
4. The diagram will open in a new panel beside your code

**[SCREENSHOT: Domain diagram preview]**
*Screenshot description: Show a generated domain diagram displayed in a webview panel next to the Craft code*

### 4. Services View

The Services view provides visualization of your service architecture with granular control over what appears in C4 diagrams.

**[SCREENSHOT: Services view in sidebar]**
*Screenshot description: Show the Services panel below the Domain Manager, displaying a hierarchical tree of service groups and services*

#### Service Hierarchy

Services are organized in a three-level hierarchy:

1. **Service Groups** (e.g., "Core Services", "External Services")
2. **Services** (individual microservices)
3. **Subdomains** (domain boundaries within services)
4. **Use Cases** (specific functionality)

**[SCREENSHOT: Expanded service hierarchy]**
*Screenshot description: Show a fully expanded service tree with all four levels visible*

#### Selecting Services

Similar to the Domain Manager, use checkboxes to select:
- Entire service groups
- Individual services
- Specific subdomains
- Individual use cases

The selection state cascades up and down the hierarchy automatically.

#### Service Focus Buttons

Each service and subdomain has a **focus button** (◉/◎):

- **Focused (◉)**: Service will be shown as an internal component in C4 diagrams (detailed view)
- **Unfocused (◎)**: Service will be shown as an external system (simplified view)

**[SCREENSHOT: Focus buttons on services]**
*Screenshot description: Close-up showing services with both focused and unfocused states*

This allows you to control the level of detail shown for each service in your diagrams.

#### C4 Diagram Configuration

Click the **gear icon** to access advanced C4 diagram options:

**Mode Options:**
- **Transparent**: Shows direct service-to-service connections
- **Boundaries**: Shows connections at domain boundary level

**Database Options:**
- **Show**: Include database components in the diagram
- **Hide**: Exclude database components

**Focus Layer:**
- **Business**: Focus on business logic and domain services
- **Presentation**: Focus on UI and presentation components
- **Composition**: Focus on service composition and integration patterns

**Infrastructure:**
- **Show**: Include infrastructure components (message queues, caches, etc.)
- **Hide**: Exclude infrastructure components

**[SCREENSHOT: Services diagram options panel]**
*Screenshot description: Show the expanded options panel with all configuration options visible*

#### Generating Service/C4 Diagrams

1. Select the services and use cases you want to visualize
2. Set focus levels for each service (internal vs external)
3. Configure diagram options (mode, database visibility, focus layer, infrastructure)
4. Click the **Preview** button (eye icon)
5. The C4 diagram will open showing only your selected elements with your configured settings

**[SCREENSHOT: C4 diagram with custom configuration]**
*Screenshot description: Show a C4 diagram with some services shown in detail and others as external systems*

### 5. Command Palette Operations

The extension provides several commands accessible via the Command Palette (`Ctrl+Shift+P` or `Cmd+Shift+P`):

#### C4 Diagram Commands

**Command:** `Craft: Preview C4 Diagram`
**Keyboard Shortcut:** `Ctrl+Shift+C` (Windows/Linux) or `Cmd+Shift+C` (Mac)

Generates a C4 diagram from your entire Craft file with default settings.

**Command:** `Craft: Preview Selected C4 Diagram`

Generates a C4 diagram from only the selected text in your editor. Select a portion of your Craft code and run this command to preview just that section.

**[SCREENSHOT: Selected text being previewed]**
*Screenshot description: Show Craft code with a selection highlighted on the left, and the resulting partial C4 diagram on the right*

#### Domain Diagram Commands

**Command:** `Craft: Preview Domain Diagram`
**Keyboard Shortcut:** `Ctrl+Shift+D` (Windows/Linux) or `Cmd+Shift+D` (Mac)

Generates a domain-focused diagram from your entire Craft file.

**Command:** `Craft: Preview Domain Diagram from selection`

Generates a domain diagram from only the selected code.

### 6. Downloading Diagrams

From any preview panel, you can download diagrams in multiple formats:

**[SCREENSHOT: Download buttons in preview panel]**
*Screenshot description: Show the preview panel with download buttons for PNG, SVG, PDF, and PlantUML*

1. Click one of the download format buttons:
   - **PNG**: Raster image format (good for presentations, documents)
   - **SVG**: Vector format, scalable without quality loss (good for web, print)
   - **PDF**: Portable document format (good for sharing, printing)
   - **PlantUML**: Source code format (allows further editing in PlantUML tools)

2. Choose a save location (or use the configured default path)
3. The diagram will be saved with your selected configuration preserved

**Note:** Downloaded diagrams include all the settings you configured (boundaries mode, database visibility, focus levels, etc.)

---

## Advanced Features

### Working with Multiple Files

When working on large architectures split across multiple `.craft` files:

1. Use the **Workspace** view mode to see all domains and services
2. Elements from non-active files appear in a lighter color
3. Use **Select Current File** to quickly filter to just the active file
4. Generate diagrams combining elements from multiple files

**[SCREENSHOT: Workspace view with multi-file indicators]**
*Screenshot description: Show the workspace view with items from the current file in normal color and items from other files in gray*

### Cross-References

The Domain Manager shows when subdomains are involved in multiple use cases:

- A link indicator (🔗) shows the number of cross-references
- Expand the subdomain to see where it's referenced
- References show whether the subdomain is an entry point or just involved

**[SCREENSHOT: Cross-references expanded]**
*Screenshot description: Show a subdomain with its "Also Involved In" section expanded, showing reference links to other use cases*

### Partial Diagram Generation

Create focused diagrams by:

1. **Text Selection**: Select specific services or domains in your code and use "Preview Selected" commands
2. **Checkbox Selection**: Use the sidebar views to select only the elements you want
3. **Focus Levels**: Use focus buttons to control detail level per service

**[SCREENSHOT: Comparison of full vs partial diagram]**
*Screenshot description: Side-by-side showing a full architecture diagram and a focused partial diagram with only 2-3 services*

### Real-time Updates

The sidebar views automatically refresh when you:
- Save your `.craft` files
- Switch between files
- Edit domain or service definitions

Preview panels remain open and can be refreshed by re-running the preview command.

---

## Configuration

### Opening Settings

**Command:** `Craft: Open Settings`

Or navigate to: **File → Preferences → Settings** and search for "Craft"

**[SCREENSHOT: Craft settings panel]**
*Screenshot description: Show the VS Code settings UI filtered to "Craft", displaying all available configuration options*

### Available Settings

#### Server URL
**Setting:** `craft.server.url`
**Default:** `http://localhost:8080`

The URL of your Craft server for diagram generation. Update this if your Craft server runs on a different host or port.

```json
{
    "craft.server.url": "http://localhost:8080"
}
```

#### Server Timeout
**Setting:** `craft.server.timeout`
**Default:** `30000` (30 seconds)
**Range:** 5000-120000 milliseconds

Maximum time to wait for diagram generation before timeout. Increase for very large architectures.

```json
{
    "craft.server.timeout": 60000
}
```

#### Download Path
**Setting:** `craft.downloadPath`
**Default:** `""` (prompt each time)

Set a default directory for downloading diagrams. Leave empty to be prompted for location each time.

```json
{
    "craft.downloadPath": "/Users/yourname/Documents/diagrams"
}
```

#### Logging Level
**Setting:** `craft.logging.level`
**Default:** `warn`
**Options:** `off`, `error`, `warn`, `info`, `debug`, `trace`

Control the verbosity of extension logging. Use `debug` or `trace` for troubleshooting.

```json
{
    "craft.logging.level": "debug"
}
```

---

## Tips & Best Practices

### 1. Use Keyboard Shortcuts

Memorize these shortcuts to boost your productivity:
- `Ctrl+Shift+C`: Preview C4 diagram
- `Ctrl+Shift+D`: Preview domain diagram
- `Ctrl+Space`: Trigger auto-completion
- `Shift+Alt+F`: Format document

### 2. Organize Large Architectures

For large systems:
- Split your architecture across multiple `.craft` files by domain or service group
- Use the Domain Manager to select specific subsets for focused diagrams
- Use the Architecture mode for high-level overviews
- Use the Detailed mode for deep-dives into specific domains

### 3. Leverage Selection Hierarchy

Work efficiently with the hierarchical selection:
- Click at the domain level to quickly select everything
- Use partial selections to include specific use cases across multiple subdomains
- Combine checkbox selection with focus buttons for precise control

### 4. Configure Diagrams Before Generating

Before clicking Preview:
- Choose the right mode (Transparent vs Boundaries, Detailed vs Architecture)
- Set focus levels on services (internal vs external view)
- Configure visibility options (databases, infrastructure)
- This saves time compared to regenerating diagrams

### 5. Use Selection-based Previews for Documentation

When creating documentation:
- Select related use cases or services
- Generate focused diagrams showing just those elements
- Download in SVG format for scalable documentation
- Use PlantUML format if you need to make manual adjustments

### 6. Experiment with View Modes

Try different combinations:
- **Boundaries mode + Hide databases** = Clean domain-level view
- **Transparent mode + Business focus** = Detailed service interactions
- **Architecture mode** = Executive-level overview
- **Detailed mode with specific use cases selected** = Feature documentation

---

## Troubleshooting

### Syntax Highlighting Not Working

**Symptoms:** Your `.craft` file appears as plain text without colors

**Solutions:**
1. Check the language mode in the status bar (bottom-right) - it should show "Craft Language"
2. If it shows something else, click it and select "Craft Language" from the dropdown
3. Reload VS Code: Press `Ctrl+Shift+P` and run "Developer: Reload Window"

**[SCREENSHOT: Language mode selector]**
*Screenshot description: Show the language mode indicator in the status bar and the language selection dropdown*

### Diagrams Not Generating

**Symptoms:** Preview command runs but no diagram appears, or you see an error message

**Solutions:**
1. **Check server connection:** Ensure your Craft server is running at the configured URL
   - Test by opening `http://localhost:8080` (or your configured URL) in a browser
2. **Verify server URL:** Open Craft settings and confirm `craft.server.url` is correct
3. **Check for DSL errors:** Review your Craft code for syntax errors (red underlines)
4. **Increase timeout:** For large architectures, increase `craft.server.timeout` in settings
5. **Check logs:** Set `craft.logging.level` to `debug` and check the Output panel (View → Output → Craft Language Server)

**[SCREENSHOT: Output panel with logs]**
*Screenshot description: Show the VS Code Output panel with "Craft Language Server" selected, displaying debug logs*

### Selection Not Working in Sidebar

**Symptoms:** Checkboxes don't respond or selections get lost

**Solutions:**
1. Click the **Refresh** button in the view toolbar
2. Save your `.craft` file (selections are preserved across refreshes)
3. Close and reopen the sidebar panel
4. Reload the window if the issue persists

### Auto-completion Not Working

**Solutions:**
1. Ensure semantic highlighting is enabled: **File → Preferences → Settings** → search for "semantic highlighting"
2. Check that the language server started: Look for "Craft Language Server" in the Output panel
3. Reload the window: `Ctrl+Shift+P` → "Developer: Reload Window"
4. Try manually triggering with `Ctrl+Space`

### Preview Panel Shows Outdated Diagram

**Solutions:**
1. Make sure you've saved your `.craft` file
2. Re-run the preview command to regenerate
3. Check if you have multiple preview panels open - close old ones
4. Verify your changes are valid Craft DSL (check for errors)

### Domain/Service Trees Empty

**Symptoms:** Sidebar views show "No domains found" or "No services found"

**Solutions:**
1. Ensure you have a `.craft` file open and it contains valid `arch` and `services` blocks
2. Check the view mode - switch between "Current File" and "Workspace"
3. Save your file to trigger a refresh
4. Click the Refresh button in the toolbar
5. Check the Output panel for parsing errors

### Downloaded Diagrams Don't Match Preview

**Symptoms:** Downloaded diagram looks different from preview

**Solutions:**
1. Verify your diagram options are set correctly before downloading
2. Check that the same elements are selected in the sidebar
3. Ensure focus buttons are in the desired state
4. Try regenerating the preview before downloading

### Extension Conflicts

If you have other language extensions installed that might conflict:
1. Temporarily disable other extensions
2. Reload VS Code
3. Test if the Craft extension works correctly
4. Re-enable extensions one at a time to identify conflicts

### Performance Issues with Large Files

**Symptoms:** Extension is slow with very large `.craft` files

**Solutions:**
1. Split large files into smaller, domain-focused files
2. Use "Current File" view mode instead of "Workspace"
3. Close preview panels when not needed
4. Reduce logging level to `warn` or `error`
5. Increase server timeout for complex diagram generation

---

## Additional Resources

- **GitHub Repository:** [https://github.com/tcarcao/craft-vscode-extension](https://github.com/tcarcao/craft-vscode-extension)
- **Report Issues:** [https://github.com/tcarcao/craft-vscode-extension/issues](https://github.com/tcarcao/craft-vscode-extension/issues)
- **Main Craft Project:** *(Add link to main Craft documentation if available)*

---

## Feature Summary Table

| Feature | Access Method | Description |
|---------|--------------|-------------|
| **C4 Diagram Preview** | `Ctrl+Shift+C` or Command Palette | Generate C4 architecture diagram |
| **Domain Diagram Preview** | `Ctrl+Shift+D` or Command Palette | Generate domain-focused diagram |
| **Selected C4 Preview** | Command Palette | Generate C4 diagram from selected code |
| **Selected Domain Preview** | Command Palette | Generate domain diagram from selected code |
| **Hierarchical Selection** | Domain Manager sidebar | Select domains, subdomains, and use cases via checkboxes |
| **Service Focus Control** | Services view sidebar | Mark services as internal (◉) or external (◎) |
| **Diagram Mode Selection** | Domain Manager gear icon | Choose Detailed or Architecture view |
| **C4 Boundaries Mode** | Services view gear icon | Toggle Transparent or Boundaries mode |
| **Database Visibility** | Services view gear icon | Show or hide database components |
| **Focus Layer** | Services view gear icon | Select Business, Presentation, or Composition focus |
| **Infrastructure Toggle** | Services view gear icon | Show or hide infrastructure components |
| **View Mode Toggle** | Sidebar header | Switch between Current File and Workspace views |
| **Quick Select All** | Toolbar checklist icon | Select all items in current view |
| **Quick Clear Selection** | Toolbar clear icon | Deselect all items |
| **Select Current File** | Toolbar file icon | Select only items from active file |
| **Format Document** | `Shift+Alt+F` | Auto-format Craft DSL code |
| **Auto-completion** | `Ctrl+Space` | Trigger code completion suggestions |
| **Download PNG/SVG/PDF** | Preview panel buttons | Download diagram in various formats |
| **Export PlantUML** | Preview panel button | Export diagram source code |
| **Refresh Views** | Toolbar refresh icon | Manually refresh domain/service trees |
| **Open Settings** | Command: "Craft: Open Settings" | Access extension configuration |

---

## Workflow Examples

### Creating Architecture Documentation

1. Open your main architecture `.craft` file
2. In the Domain Manager, select "Workspace" to see all domains
3. Click "Select All" to include everything
4. Toggle to "Architecture" mode for a high-level view
5. Click Preview to generate an overview diagram
6. Download as SVG for your documentation
7. Select specific domains for detailed subsystem diagrams
8. Switch to "Detailed" mode and regenerate for each subsystem

### Reviewing a Feature Implementation

1. Open the `.craft` file containing the feature
2. Use text selection to highlight the relevant use cases
3. Run "Craft: Preview Domain Diagram from selection"
4. Review the use case flows in the generated diagram
5. Switch to Services view
6. Select the services involved in the feature
7. Set focus levels to show internal implementation details
8. Generate a C4 diagram with "Transparent" mode to see service interactions

### Presenting to Stakeholders

1. For executives: Use Architecture mode with Boundaries to show domain relationships
2. For architects: Use Detailed mode with selected critical use cases
3. For developers: Use Transparent mode with Business focus and infrastructure shown
4. For each audience, configure and download the appropriate diagram format

---

This comprehensive guide should help you make the most of the Craft VSCode Extension. Happy architecting!

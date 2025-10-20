# Commands & Shortcuts

Master the Craft VSCode Extension with these commands and shortcuts. This guide covers all available commands, keyboard shortcuts, and practical workflow examples.

## Quick Reference

### Keyboard Shortcuts

| Shortcut | Command | Description |
|----------|---------|-------------|
| `Ctrl+Shift+C` | Preview C4 Diagram | Generate C4 architecture diagram from entire file |
| `Ctrl+Shift+D` | Preview Domain Diagram | Generate domain diagram from entire file |
| `Shift+Alt+F` | Format Document | Auto-format your `.craft` file |
| `Ctrl+Space` | Trigger Suggest | Show auto-completion suggestions |
| `Ctrl+Shift+P` | Command Palette | Access all Craft commands |

::: tip Mac Users
Replace `Ctrl` with `Cmd` and `Alt` with `Option` in all shortcuts above.
:::

## Diagram Commands

### Full File Diagram Generation

**Command:** `Craft: Preview C4 Diagram`
**Shortcut:** `Ctrl+Shift+C` (Windows/Linux) or `Cmd+Shift+C` (Mac)

Generates a C4 architecture diagram from your entire `.craft` file with default settings.

**When to use:**
- Getting an overview of your complete architecture
- Reviewing all services and their interactions
- Generating documentation diagrams
- Presenting the full system architecture

**Example workflow:**
1. Open your main architecture file (e.g., `architecture.craft`)
2. Press `Ctrl+Shift+C`
3. The C4 diagram opens in a preview panel beside your code
4. Use the download buttons to export in your preferred format

---

**Command:** `Craft: Preview Domain Diagram`
**Shortcut:** `Ctrl+Shift+D` (Windows/Linux) or `Cmd+Shift+D` (Mac)

Generates a domain-focused diagram from your entire Craft file, showing domain relationships and use case flows.

**When to use:**
- Understanding domain boundaries and interactions
- Reviewing business logic flow
- Creating domain-driven design documentation
- Analyzing use case dependencies

**Example workflow:**
1. Open a file with domain definitions and use cases
2. Press `Ctrl+Shift+D`
3. Review the domain interactions in the preview
4. Export as SVG for scalable documentation

### Selection-Based Diagram Generation

**Command:** `Craft: Preview Selected C4 Diagram`

Generates a C4 diagram from only the selected text in your editor.

**When to use:**
- Focusing on a specific subsystem or feature
- Creating targeted documentation for a single service
- Isolating a portion of a large architecture
- Demonstrating specific service interactions

**Example workflow:**

**[SCREENSHOT NEEDED: Selected code being previewed]**
*Show highlighted Craft code selection on left and resulting partial C4 diagram on right*

1. Open your architecture file
2. Select the services or domains you want to visualize (click and drag to highlight)
3. Open Command Palette (`Ctrl+Shift+P`)
4. Type "Craft: Preview Selected C4"
5. The diagram shows only your selected elements

**Example selection:**
```craft
// Select just this service definition
services {
    UserService {
        domains: Authentication, Profile
        data-stores: user_db
        language: golang
    }
}

use_case "User Registration" {
    when Business_User creates Account
        Authentication validates email format
        Profile creates user profile
}
```

---

**Command:** `Craft: Preview Domain Diagram from selection`

Generates a domain diagram from only the selected code.

**When to use:**
- Analyzing specific use case flows
- Creating feature-specific diagrams
- Documenting individual user journeys
- Isolating complex domain interactions

**Example workflow:**
1. Select one or more related use cases
2. Run "Craft: Preview Domain Diagram from selection" from Command Palette
3. Review the focused domain diagram
4. Export for feature documentation

## Sidebar View Commands

### Domain Manager Commands

**Command:** `Craft: Refresh Domains`

Manually refreshes the domain tree view.

**When to use:**
- After making changes to domain definitions
- When switching between files
- If the tree view appears out of sync

**How to access:**
- Command Palette: Type "Craft: Refresh Domains"
- Click the refresh icon in the Domain Manager toolbar

---

**Command:** `Craft: Select All Domains`

Selects all domains, subdomains, and use cases in the current view.

**When to use:**
- Generating a complete architecture diagram using sidebar selections
- Starting with everything selected, then deselecting specific items
- Quickly including all elements in your visualization

**How to access:**
- Command Palette: Type "Craft: Select All Domains"
- Click the "Select All" icon (checklist) in the Domain Manager toolbar

---

**Command:** `Craft: Select Current File Domains`

Selects only domains from the currently active `.craft` file.

**When to use:**
- Working with multi-file projects
- Filtering out domains from other files
- Focusing on the current file's architecture

**How to access:**
- Command Palette: Type "Craft: Select Current File"
- Click the "Select Current File" icon (file) in the Domain Manager toolbar

### Services View Commands

**Command:** `Craft: Refresh Services`

Manually refreshes the services tree view.

**When to use:**
- After modifying service definitions
- When technology stack or data stores change
- If the tree view doesn't reflect recent edits

**How to access:**
- Command Palette: Type "Craft: Refresh Services"
- Click the refresh icon in the Services view toolbar

## Configuration Commands

**Command:** `Craft: Open Settings`

Opens the VS Code settings filtered to Craft extension configuration.

**When to use:**
- Configuring server URL
- Adjusting timeout values
- Setting default download paths
- Changing logging levels

**How to access:**
- Command Palette: Type "Craft: Open Settings"
- Or navigate to File → Preferences → Settings and search "Craft"

## Using Commands

### Via Command Palette

**[SCREENSHOT NEEDED: Command Palette with Craft commands]**
*Show Command Palette (`Ctrl+Shift+P`) with "Craft" typed and command suggestions visible*

The Command Palette provides access to all Craft commands:

1. Press `Ctrl+Shift+P` (Windows/Linux) or `Cmd+Shift+P` (Mac)
2. Type "Craft" to filter to Craft commands
3. Use arrow keys to navigate
4. Press Enter to execute the selected command

**Tip:** You don't need to type the full command name. Type just a few letters:
- "cpc4" → Craft: Preview C4 Diagram
- "cpd" → Craft: Preview Domain Diagram
- "crs" → Craft: Refresh Services

### Via Keyboard Shortcuts

The fastest way to generate diagrams:

- **`Ctrl+Shift+C`**: Instantly generate C4 diagram
- **`Ctrl+Shift+D`**: Instantly generate domain diagram

These shortcuts work when you have a `.craft` file active in the editor.

### Via Context Menu

**[SCREENSHOT NEEDED: Right-click context menu]**
*Show context menu appearing when right-clicking in a .craft file with Craft-specific options*

Right-click anywhere in a `.craft` file to access:
- "Preview C4 Diagram"
- "Preview Domain Diagram"
- "Format Document"

This is useful when you prefer mouse-based workflows.

### Via Sidebar Icons

**[SCREENSHOT NEEDED: Sidebar toolbar icons]**
*Show Domain Manager and Services view toolbars with icons highlighted and labeled*

The Domain Manager and Services view provide toolbar icons for:
- **Preview** (eye icon): Generate diagram from selected elements
- **Select All** (checklist icon): Select all items
- **Clear Selection** (clear icon): Deselect all items
- **Select Current File** (file icon): Select items from active file
- **Refresh** (refresh icon): Reload the tree view
- **Options** (gear icon): Configure diagram settings

## Workflow Examples

### Workflow 1: Creating Architecture Documentation

**Goal:** Generate complete architecture documentation with overview and detailed diagrams.

1. **Overview Diagram:**
   - Open main `architecture.craft` file
   - In Services view, click "Select All"
   - Toggle to "Boundaries" mode in diagram options
   - Click Preview to generate high-level C4 diagram
   - Download as SVG

2. **Subsystem Details:**
   - In Domain Manager, select specific domains (e.g., "User Management")
   - Toggle to "Detailed" mode
   - Click Preview to generate detailed domain diagram
   - Download as PDF

3. **Repeat** for each major subsystem

**Result:** Complete documentation package with overview and detailed diagrams.

---

### Workflow 2: Reviewing a Feature Implementation

**Goal:** Understand how a specific feature is implemented across services and domains.

1. **Locate Use Cases:**
   - Open the file containing the feature
   - Expand Domain Manager to find related use cases

2. **Generate Focused Diagram:**
   - In the editor, select the relevant use cases (click and drag)
   - Run "Craft: Preview Domain Diagram from selection" from Command Palette
   - Review the use case flows

3. **Service View:**
   - Switch to Services view
   - Select only the services involved in the feature
   - Set focus levels: Internal (◉) for main services, External (◎) for dependencies
   - Generate C4 diagram with "Transparent" mode

**Result:** Clear understanding of feature implementation and service dependencies.

---

### Workflow 3: Presenting to Different Audiences

**Goal:** Create tailored diagrams for executives, architects, and developers.

**For Executives:**
- Use Architecture mode with Boundaries
- Hide databases and infrastructure
- Select only top-level domains
- Export as PDF

**For Architects:**
- Use Detailed mode with selected critical use cases
- Show boundaries and databases
- Include all relevant services
- Export as SVG

**For Developers:**
- Use Transparent mode with Business focus
- Show infrastructure components
- Focus on specific services (set to Internal)
- Include data stores
- Export as PlantUML for editing

---

### Workflow 4: Working with Large Multi-File Projects

**Goal:** Navigate and visualize a complex architecture split across multiple files.

1. **Switch to Workspace view** in both Domain Manager and Services view

2. **Filter by Current File:**
   - Navigate to a specific `.craft` file
   - Click "Select Current File" in Domain Manager
   - Generate diagram showing only that file's elements

3. **Cross-File Diagrams:**
   - Use checkboxes to select domains from multiple files
   - Elements from different files appear with file indicators
   - Generate combined diagram showing cross-file interactions

4. **Refresh Views:**
   - After editing multiple files, click refresh icons to ensure trees are up to date

**Result:** Effective navigation and visualization of large, distributed architectures.

## Tips & Best Practices

### Memorize Core Shortcuts

For daily use, memorize these three:
- `Ctrl+Shift+C` - C4 diagram
- `Ctrl+Shift+D` - Domain diagram
- `Shift+Alt+F` - Format document

### Use Selection for Focus

When files are large:
- Select specific sections and use "Preview Selected" commands
- Faster than configuring sidebar selections for one-time views
- Perfect for quick reviews

### Leverage Sidebar for Repeated Use

When you need to generate similar diagrams repeatedly:
- Configure selections in sidebar views
- Adjust diagram options (mode, visibility)
- Click Preview to regenerate with same settings

### Combine with Format Document

Before generating diagrams for documentation:
1. Format your code with `Shift+Alt+F`
2. Review for any formatting issues
3. Generate diagrams from clean, consistent code

### Use Refresh When Needed

Sidebar views auto-refresh on file save, but if they seem out of sync:
- Click the refresh icon
- Or run Refresh commands from Command Palette

## Next Steps

- [Explore all features](/extension/features) - Deep dive into sidebar views and diagram options
- [Configure settings](/extension/configuration) - Customize server URL, timeouts, paths

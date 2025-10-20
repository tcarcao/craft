# Extension Features

The Craft VSCode Extension transforms Visual Studio Code into a powerful IDE for architecture modeling. This page details all the features available to help you create, visualize, and manage your software architecture.

## Syntax Highlighting


![Complex .craft file with full syntax highlighting](/images/extension/complex-syntax-highlighting.png)

The extension provides sophisticated semantic syntax highlighting powered by Tree-sitter, with 40+ distinct token types that make your Craft code highly readable:

- **Keywords** (purple, bold): `service`, `domain`, `arch`, `use_case`, `actors`, `when`
- **Service names** (blue, bold): Service identifiers like `UserService`, `CommsService`
- **Domain names** (blue, bold): Domain identifiers like `Authentication`, `Profile`
- **Properties** (red, bold): `domains`, `language`, `data-stores`, `framework`
- **Action verbs** (orange/green, bold): `asks`, `notifies`, `listens`, `returns`, `creates`
- **Strings** (various colors): Use case names, event strings, regular quoted text
- **Comments** (green, italic): Line and block comments

The highlighting adapts in real-time as you type, making it easy to spot syntax issues and understand code structure at a glance.

## Language Server Features

### Auto-completion

**[SCREENSHOT NEEDED: Auto-completion dropdown in action]**
*Show the auto-completion popup with Craft keywords and domain names*

The language server provides intelligent, context-aware code completion:

- **Keyword suggestions**: Type to get suggestions for `actors`, `services`, `use_case`, `domain`, `arch`
- **Template snippets**: Automatically insert complete code blocks with placeholders
- **Domain and service references**: Auto-complete existing domain and service names
- **DSL structure guidance**: Suggest valid syntax in current context

Press `Ctrl+Space` (Windows/Linux) or `Cmd+Space` (Mac) to manually trigger auto-completion at any time.

### Real-time Validation

![Validation errors highlighted in code](/images/extension/validation-errors.png)

As you type, the language server validates your Craft code and highlights errors:

```craft
use_case "Test" {
  when user does something
    Domain asks  // ❌ Error: incomplete action
}
```

Errors appear as red squiggly underlines, and warnings as yellow underlines. Hover over any issue to see a detailed error message.

### Code Formatting

**[SCREENSHOT NEEDED: Before and after formatting comparison]**
*Split view showing unformatted code on left and formatted code on right*

Keep your architecture definitions clean and consistent:

- Press `Shift+Alt+F` (Windows/Linux) or `Shift+Option+F` (Mac)
- Right-click and select "Format Document"
- Automatically formats indentation, spacing, and alignment

## Domain Manager Sidebar

![Domain Manager panel with hierarchical tree](/images/extension/domain-manager-panel.png)

The Domain Manager provides a powerful tree view for exploring and selecting domains in your architecture.

### View Modes

Toggle between two view modes using the buttons at the top:

- **Current File**: Shows only domains from the currently active `.craft` file
- **Workspace**: Shows all domains from all `.craft` files in your workspace

**[SCREENSHOT NEEDED: View mode toggle buttons]**
*Close-up of "Current File" and "Workspace" toggle buttons with one highlighted*

### Hierarchical Selection

The Domain Manager uses a sophisticated checkbox system for selecting elements:

**Domain Level:**
- Click a domain checkbox to select/deselect all subdomains and use cases within it
- ✓ (checkmark) = all items selected
- ▣ (filled square) = partial selection (some items selected)
- ○ (empty) = nothing selected

**Subdomain Level:**
- Select/deselect all use cases within a subdomain
- Selection automatically updates parent domain state

**Use Case Level:**
- Select individual use cases to include in diagrams
- Selections cascade up to update subdomain and domain states

**[SCREENSHOT NEEDED: Hierarchical selection states]**
*Show domains with full selection (✓), partial selection (▣), and no selection (○)*

### Quick Selection Toolbar

**[SCREENSHOT NEEDED: Toolbar buttons highlighted]**
*Show toolbar with all three selection buttons visible and labeled*

Rapid selection controls:

- **Select All** (checklist icon): Select all domains, subdomains, and use cases
- **Clear Selection** (clear icon): Deselect everything
- **Select Current File** (file icon): Select only items from the active file

### Selection Counter

**[SCREENSHOT NEEDED: Selection counter display]**
*Show counter displaying "12 use cases • 5 subdomains • 2 domains"*

The header displays your current selection count, helping you track what will be included in your diagram:
- Number of use cases selected
- Number of subdomains selected
- Number of domains selected

### Diagram Options

**[SCREENSHOT NEEDED: Domain diagram options panel expanded]**
*Show expanded options with "Detailed" and "Architecture" mode buttons*

Click the **gear icon** to configure diagram visualization:

**Mode Options:**
- **Detailed**: Shows complete domain diagram with all use cases and their interactions
- **Architecture**: Shows only subdomain-to-subdomain connections without use case details

### Generating Domain Diagrams

![Generated domain diagram in preview panel](/images/extension/domain-diagram-preview.png)

To generate a domain diagram:

1. Select the domains, subdomains, or use cases you want to visualize
2. Configure the diagram mode (Detailed or Architecture)
3. Click the **Preview** button (eye icon) in the toolbar
4. The diagram opens in a new panel beside your code

## Services View

![Services panel in sidebar](/images/extension/services-panel.png)

The Services view provides comprehensive control over service architecture visualization.

### Service Hierarchy

**[SCREENSHOT NEEDED: Fully expanded service tree]**
*Show all four levels: service groups, services, subdomains, use cases*

Services are organized in a four-level hierarchy:

1. **Service Groups**: Logical grouping (e.g., "Core Services", "External Services")
2. **Services**: Individual microservices or components
3. **Subdomains**: Domain boundaries within services
4. **Use Cases**: Specific functionality

### Service Selection

Similar to the Domain Manager, use checkboxes to select:
- Entire service groups
- Individual services
- Specific subdomains within services
- Individual use cases

Selection state cascades automatically up and down the hierarchy.

### Service Focus Control

**[SCREENSHOT NEEDED: Focus buttons on services]**
*Close-up showing services with both focused (◉) and unfocused (◎) states*

Each service and subdomain has a **focus button** that controls visualization detail:

- **Focused (◉)**: Service shown as internal component in C4 diagrams with full detail
- **Unfocused (◎)**: Service shown as external system with simplified view

This allows precise control over which services are shown in detail vs. abstracted as external dependencies.

### C4 Diagram Configuration

**[SCREENSHOT NEEDED: Services diagram options panel]**
*Show expanded options with all configuration toggles visible*

Click the **gear icon** to access advanced C4 diagram options:

**Mode Options:**
- **Transparent**: Shows direct service-to-service connections
- **Boundaries**: Shows connections at domain boundary level

**Database Options:**
- **Show**: Include database components in diagram
- **Hide**: Exclude database components

**Focus Layer:**
- **Business**: Focus on business logic and domain services
- **Presentation**: Focus on UI and presentation components
- **Composition**: Focus on service composition and integration patterns

**Infrastructure:**
- **Show**: Include infrastructure components (message queues, caches, etc.)
- **Hide**: Exclude infrastructure components

### Generating Service/C4 Diagrams

![C4 diagram](/images/extension/c4-diagram-mixed-focus.png)

To generate a C4 architecture diagram:

1. Select the services and use cases you want to visualize
2. Set focus levels for each service (internal vs external)
3. Configure diagram options (mode, database visibility, focus layer, infrastructure)
4. Click the **Preview** button (eye icon)
5. The C4 diagram opens showing your selected elements with configured settings

## Live Diagram Preview

**[SCREENSHOT NEEDED: Split view with code and live preview]**
*Show .craft file on left and generated diagram on right*

Generate architecture visualizations in real-time:

### Keyboard Shortcuts

- **Ctrl+Shift+C** (or `Cmd+Shift+C` on Mac): Preview C4 architecture diagram
- **Ctrl+Shift+D** (or `Cmd+Shift+D` on Mac): Preview domain diagram

### Command Palette

Press `Ctrl+Shift+P` (or `Cmd+Shift+P` on Mac) and search for:

- `Craft: Preview C4 Diagram` - Generate from entire file
- `Craft: Preview Selected C4 Diagram` - Generate from selected code
- `Craft: Preview Domain Diagram` - Generate domain diagram from file
- `Craft: Preview Domain Diagram from selection` - Generate from selection

### Selection-Based Previews

**[SCREENSHOT NEEDED: Selected code being previewed]**
*Show highlighted code selection on left and resulting partial diagram on right*

Create focused diagrams by selecting specific portions of your Craft code:

1. Highlight the services, domains, or use cases you want to visualize
2. Run "Preview Selected C4 Diagram" or "Preview Domain Diagram from selection"
3. Only the selected elements appear in the generated diagram

This is perfect for documentation, focusing on specific features, or isolating parts of large architectures.

## Downloading Diagrams

**[SCREENSHOT NEEDED: Download buttons in preview panel]**
*Show preview panel with PNG, SVG, PDF, and PlantUML download buttons*

Export diagrams in multiple formats directly from the preview panel:

- **PNG**: Raster format for presentations, documents, and sharing
- **SVG**: Scalable vector format for web, print, and editing
- **PDF**: Portable format for professional documentation
- **PlantUML**: Source code format for further editing in PlantUML tools

Downloaded diagrams preserve all your configuration settings (boundaries mode, database visibility, focus levels, etc.).

## Cross-References

**[SCREENSHOT NEEDED: Subdomain with cross-references expanded]**
*Show subdomain with "Also Involved In" section showing reference links*

The Domain Manager shows when subdomains participate in multiple use cases:

- Link indicator (🔗) displays the number of cross-references
- Expand to see where the subdomain is referenced
- View whether the subdomain is an entry point or supporting participant

This helps you understand coupling and dependencies across your architecture.

## Real-time Updates

The sidebar views automatically refresh when you:
- Save your `.craft` files
- Switch between open files
- Edit domain or service definitions

Preview panels can be refreshed by re-running the preview command to see the latest changes.

## Multi-file Support

**[SCREENSHOT NEEDED: Workspace view with multi-file indicators]**
*Show workspace view with current file items in normal color and other files in gray*

When working on large architectures split across multiple `.craft` files:

1. Use **Workspace** view mode to see all domains and services
2. Elements from non-active files appear in a lighter color
3. Use **Select Current File** to quickly filter to just the active file
4. Generate diagrams combining elements from multiple files

::: warning Server Required
All diagram generation features require the Craft diagram server running on `http://localhost:8080` (or your configured URL). See the [Installation Guide](/guide/installation) for server setup instructions.
:::

## Next Steps

- [Learn all available commands](/extension/commands)
- [Configure extension settings](/extension/configuration)
- [See example workflows](/extension/workflows)

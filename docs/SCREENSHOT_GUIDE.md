# Screenshot Guide for Documentation

This guide provides detailed instructions for taking screenshots for the Craft VSCode Extension documentation. Each screenshot is mapped to specific examples and shows exactly what should be captured.

## File Locations

- **Main example file**: `/examples/screenshot-examples.craft`
- **User management example**: `/examples/user-management.craft`
- **Simple example**: `/examples/simple.craft`

## Required Screenshots by Documentation Page

### Installation Page (`docs/page/extension/installation.md`)

#### 1. VS Code Extensions Marketplace
**Location:** Line 14-15
**File:** N/A (Marketplace view)
**Steps:**
1. Open VS Code
2. Press `Ctrl+Shift+X` to open Extensions view
3. Search for "Craft Arch Diagrams" in the search box
4. Capture the Extensions view showing:
   - Search box with "Craft Arch Diagrams"
   - Extension listing with Install button highlighted
   - Extension icon, name, and description visible
**Name:** `marketplace-install.png`

---

#### 2. Empty .craft File with Language Mode
**Location:** Line 50-51
**File:** Create new empty file named `example.craft`
**Steps:**
1. Create a new file: File → New File
2. Save as `example.craft`
3. Ensure status bar shows "Craft Language" in bottom-right
4. Capture the editor showing:
   - Empty file with `example.craft` in tab
   - Status bar with "Craft Language" indicator visible
   - Clean editor interface
**Name:** `empty-craft-file.png`

---

#### 3. Basic Craft Code with Syntax Highlighting
**Location:** Line 75-76
**File:** Use the code shown in installation.md (lines 59-73)
**Steps:**
1. Create `test.craft` with the example code from the doc
2. Ensure syntax highlighting is active
3. Capture showing:
   - Keywords (`actors`, `domain`, `use_case`) in purple/bold
   - Domain name (`Authentication`) in blue/bold
   - Action verbs (`authenticates`, `validates`, `returns`) in orange/green
   - Strings in colored text
   - Line numbers visible
**Name:** `basic-syntax-highlighting.png`

---

#### 4. Auto-completion in Action
**Location:** Line 99-100
**File:** `test.craft` or any `.craft` file
**Steps:**
1. Open a `.craft` file
2. On a new line, type `use` (don't complete it)
3. Trigger completion: `Ctrl+Space`
4. Capture showing:
   - Auto-completion dropdown visible
   - Suggestions including `use_case` with snippet icon
   - Code context visible
**Name:** `auto-completion.png`

---

#### 5. Language Mode Selector
**Location:** Line 126-127
**File:** Any `.craft` file
**Steps:**
1. Open a `.craft` file
2. Click the language mode indicator in status bar (bottom-right)
3. Capture showing:
   - Language selection dropdown open
   - "Craft Language" highlighted or selected
   - Other language options visible in list
**Name:** `language-mode-selector.png`

---

### Features Page (`docs/page/extension/features.md`)

#### 6. Complex File with Full Syntax Highlighting
**Location:** Line 7-8
**File:** `/examples/screenshot-examples.craft`
**Section:** Use Examples 1-4 (lines 1-120)
**Steps:**
1. Open `screenshot-examples.craft`
2. Scroll to show actors, arch, domains, and services sections
3. Capture showing:
   - Multiple syntax elements: keywords, domains, services, properties
   - Different colors for different token types
   - Comments in green/italic
   - Full visual variety
**Name:** `complex-syntax-highlighting.png`

---

#### 7. Validation Errors Highlighted
**Location:** Line 40-41
**File:** Create temporary file with intentional errors
**Steps:**
1. Create file with this code:
```craft
use_case "Test" {
  when user does something
    Domain asks  // Error: incomplete action
    Authentication returns
}
```
2. Save the file to trigger validation
3. Capture showing:
   - Red squiggly underline on error line
   - Error tooltip visible when hovering
   - Problem count in status bar
**Name:** `validation-errors.png`

---

#### 8. Before and After Formatting
**Location:** Line 56-57
**File:** Create unformatted code
**Steps:**
1. Create file with poorly formatted code:
```craft
use_case "Test"{when user does something
Authentication validates credentials
   Authentication returns token}
```
2. Take first screenshot (before)
3. Press `Shift+Alt+F` to format
4. Take second screenshot (after)
5. Combine both in side-by-side view
**Name:** `formatting-before-after.png`

---

#### 9. Domain Manager Panel with Hierarchical Tree
**Location:** Line 67-68
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open `screenshot-examples.craft`
2. Click Craft icon in Activity Bar (left sidebar)
3. Expand several domains in Domain Manager
4. Show checkboxes (some checked, some unchecked)
5. Capture showing:
   - Craft icon in Activity Bar
   - Domain Manager panel title
   - Hierarchical tree with domains, subdomains, use cases
   - Checkboxes visible
   - Toolbar icons at top
**Name:** `domain-manager-panel.png`

---

#### 10. View Mode Toggle Buttons
**Location:** Line 79-80
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Domain Manager
2. Focus on the header area
3. Capture close-up showing:
   - "Current File" and "Workspace" toggle buttons
   - One button highlighted as active
   - Clean, clear view of toggle UI
**Name:** `view-mode-toggle.png`

---

#### 11. Hierarchical Selection States
**Location:** Line 100-101
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Domain Manager
2. Set up different selection states:
   - Fully select one domain (all children selected) - shows ✓
   - Partially select another domain (some children selected) - shows ▣
   - Leave another domain unselected - shows ○
3. Capture showing all three states clearly
**Name:** `hierarchical-selection-states.png`

---

#### 12. Toolbar Buttons Highlighted
**Location:** Line 105-106
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Domain Manager
2. Capture toolbar showing:
   - Select All icon (checklist)
   - Clear Selection icon
   - Select Current File icon
   - Preview icon (eye)
   - Refresh icon
   - Options icon (gear)
3. Add labels or arrows to identify each button
**Name:** `toolbar-buttons.png`

---

#### 13. Selection Counter Display
**Location:** Line 116-117
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Domain Manager
2. Select various use cases to get numbers like "12 use cases • 5 subdomains • 2 domains"
3. Capture header area showing the counter clearly
**Name:** `selection-counter.png`

---

#### 14. Domain Diagram Options Panel
**Location:** Line 126-127
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Domain Manager
2. Click gear icon to expand options
3. Capture showing:
   - Expanded options panel
   - "Detailed" and "Architecture" mode buttons
   - Clear view of configuration options
**Name:** `domain-diagram-options.png`

---

#### 15. Generated Domain Diagram in Preview
**Location:** Line 137-138
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open `screenshot-examples.craft`
2. Press `Ctrl+Shift+D` to generate domain diagram
3. Capture showing:
   - Code on left side
   - Domain diagram preview panel on right side
   - Split view clearly visible
   - Diagram content readable
**Name:** `domain-diagram-preview.png`

---

#### 16. Services Panel in Sidebar
**Location:** Line 149-150
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open `screenshot-examples.craft`
2. Ensure Services panel is visible in sidebar
3. Capture showing:
   - Services panel below Domain Manager
   - Service hierarchy visible
   - Service names and checkboxes
**Name:** `services-panel.png`

---

#### 17. Fully Expanded Service Tree
**Location:** Line 156-157
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Services panel
2. Expand all levels: service groups → services → subdomains → use cases
3. Capture showing all four levels clearly
**Name:** `service-tree-expanded.png`

---

#### 18. Focus Buttons on Services
**Location:** Line 178-179
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Services panel
2. Ensure some services show ◉ (focused) and others ◎ (unfocused)
3. Capture close-up showing the focus button states
**Name:** `service-focus-buttons.png`

---

#### 19. Services Diagram Options Panel
**Location:** Line 190-191
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Services panel
2. Click gear icon to expand options
3. Capture showing:
   - Mode options (Transparent/Boundaries)
   - Database toggle (Show/Hide)
   - Focus layer (Business/Presentation/Composition)
   - Infrastructure toggle (Show/Hide)
**Name:** `services-diagram-options.png`

---

#### 20. C4 Diagram with Mixed Focus Levels
**Location:** Line 214-215
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open `screenshot-examples.craft`
2. In Services view, set some services to Internal (◉) and others to External (◎)
3. Press `Ctrl+Shift+C` to generate C4 diagram
4. Capture showing C4 diagram with clearly different detail levels
**Name:** `c4-diagram-mixed-focus.png`

---

#### 21. Split View with Code and Live Preview
**Location:** Line 227-228
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open `screenshot-examples.craft`
2. Generate any diagram (C4 or domain)
3. Capture full window showing:
   - Code editor on left
   - Live diagram preview on right
   - Both clearly visible and readable
**Name:** `split-view-preview.png`

---

#### 22. Selected Code Being Previewed
**Location:** Line 248-249
**File:** `/examples/screenshot-examples.craft`
**Section:** Example 9 - Password Reset (lines 214-235)
**Steps:**
1. Open `screenshot-examples.craft`
2. Select the "Password Reset" use case (lines 214-235)
3. Run "Craft: Preview Selected C4 Diagram" or "Craft: Preview Domain Diagram from selection"
4. Capture showing:
   - Selected code highlighted on left
   - Resulting partial diagram on right
   - Clear indication of what's selected
**Name:** `selected-code-preview.png`

---

#### 23. Download Buttons in Preview Panel
**Location:** Line 261-262
**File:** Any diagram preview
**Steps:**
1. Generate any diagram preview
2. Capture the preview panel header/footer showing:
   - PNG download button
   - SVG download button
   - PDF download button
   - PlantUML download button
**Name:** `download-buttons.png`

---

#### 24. Subdomain with Cross-References
**Location:** Line 275-276
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open Domain Manager
2. Find a subdomain that appears in multiple use cases (e.g., EmailNotifier)
3. Expand the subdomain to show "Also Involved In" section
4. Capture showing cross-reference links
**Name:** `cross-references-expanded.png`

---

#### 25. Workspace View with Multi-File Indicators
**Location:** Line 298-299
**File:** Multiple `.craft` files in workspace
**Steps:**
1. Have multiple `.craft` files open in workspace
2. Switch Domain Manager to "Workspace" view
3. Capture showing:
   - Items from current file in normal color
   - Items from other files in lighter/gray color
   - File name indicators if visible
**Name:** `workspace-multi-file.png`

---

### Commands Page (`docs/page/extension/commands.md`)

#### 26. Command Palette with Craft Commands
**Location:** Line 202-203
**File:** Any `.craft` file
**Steps:**
1. Press `Ctrl+Shift+P` to open Command Palette
2. Type "Craft"
3. Capture showing:
   - Command Palette open
   - "Craft" typed in search
   - List of Craft commands visible
   - Command descriptions visible
**Name:** `command-palette-craft.png`

---

#### 27. Right-Click Context Menu
**Location:** Line 228-229
**File:** Any `.craft` file
**Steps:**
1. Right-click in editor with `.craft` file open
2. Capture showing:
   - Context menu open
   - Craft-specific options visible:
     - "Preview C4 Diagram"
     - "Preview Domain Diagram"
     - "Format Document"
**Name:** `context-menu.png`

---

#### 28. Sidebar Toolbar Icons
**Location:** Line 240-241
**File:** `/examples/screenshot-examples.craft`
**Steps:**
1. Open both Domain Manager and Services panels
2. Capture close-up of toolbars showing all icons
3. Add labels identifying each icon
**Name:** `sidebar-toolbar-icons.png`

---

### Troubleshooting Page (`docs/page/extension/troubleshooting.md`)

#### 29. Error Message - Server Unreachable
**Location:** Line 64-65
**File:** Any `.craft` file (with server stopped)
**Steps:**
1. Stop the Craft diagram server
2. Try to generate a diagram (press `Ctrl+Shift+C`)
3. Capture the error toast notification showing connection failure
**Name:** `error-server-unreachable.png`

---

#### 30. Empty Sidebar View
**Location:** Line 144-145
**File:** Empty or invalid `.craft` file
**Steps:**
1. Open file with no valid domains
2. Capture Domain Manager showing "No domains found" message
**Name:** `empty-sidebar-view.png`

---

#### 31. Output Panel with Language Server Logs
**Location:** Line 252-253
**File:** N/A
**Steps:**
1. Set logging to debug: Settings → "craft.logging.level" → "debug"
2. View → Output
3. Select "Craft Language Server" from dropdown
4. Capture showing:
   - Output panel open
   - "Craft Language Server" selected
   - Debug logs visible
**Name:** `output-panel-logs.png`

---

## Screenshot Standards

### Technical Requirements
- **Format**: PNG
- **Resolution**: Minimum 1920x1080 (use Retina/2x if available)
- **DPI**: 144 or higher
- **Color**: sRGB color space

### Composition Guidelines
- **Clean workspace**: Close unnecessary panels/windows
- **Focus**: Highlight or annotate key UI elements when needed
- **Readable text**: Ensure code and UI text is clearly readable
- **Consistent theme**: Use the same VS Code theme for all screenshots (recommend: Dark+ or Light+)
- **No personal info**: Remove any personal file paths, usernames, etc.

### File Naming Convention
All screenshot files should be placed in: `/docs/page/public/images/extension/`

Use kebab-case naming that matches the name listed above:
- ✅ `marketplace-install.png`
- ✅ `domain-manager-panel.png`
- ❌ `DomainManager.png`
- ❌ `screenshot_1.png`

### Annotation Guidelines
When adding arrows, boxes, or labels:
- Use a consistent color (recommend: red or yellow for highlights)
- Keep annotations minimal and clear
- Use sans-serif font for labels
- Don't obscure important content

## Priority Screenshots

If you need to prioritize, take these screenshots first:

**Highest Priority:**
1. Marketplace install (#1)
2. Basic syntax highlighting (#3)
3. Domain Manager panel (#9)
4. Services panel (#16)
5. Split view preview (#21)
6. C4 diagram (#20)

**Medium Priority:**
7. Auto-completion (#4)
8. Hierarchical selection (#11)
9. Command palette (#26)
10. Selected code preview (#22)

**Lower Priority:**
All remaining screenshots

## Testing Your Screenshots

Before considering screenshots complete:
1. View at actual size on a standard monitor
2. Verify all text is readable
3. Check that highlighted elements are clear
4. Ensure no personal/sensitive information is visible
5. Confirm file names match this guide

## Questions or Issues?

If you have questions about any screenshot or need clarification:
1. Check the corresponding documentation page for context
2. Review the example file sections referenced
3. Look at the "Steps" section for detailed instructions

# Extension Features

## Syntax Highlighting

Tree-sitter based semantic highlighting with 40+ token types:

- Keywords: `use_case`, `when`, `services`, `domain`
- Actions: `asks`, `notifies`, `listens`, `returns`
- Entities: domain names, service names
- Strings: quoted text and event names

## Real-time Diagnostics

Syntax errors are highlighted as you type:

```craft
use_case "Test" {
  when user does something
    Domain asks  // ❌ Error: incomplete action
}
```

## Code Completion

Type and get suggestions:

- `actors` → snippet with template
- `services` → snippet with template
- `use_case` → snippet with template
- `domain` → snippet with template

## Live Diagram Preview

Press keyboard shortcuts to generate diagrams:

- **Ctrl+Shift+C** - C4 architecture diagram
- **Ctrl+Shift+D** - Domain diagram

::: warning
Requires diagram server running on `http://localhost:8080`
:::

## Domain Explorer

Sidebar view showing:
- All domains in workspace
- Subdomains
- Related use cases
- Click to navigate

## Service Explorer

Sidebar view showing:
- All services
- Technology stack
- Deployment strategy
- Associated domains

## Document Formatting

Press `Shift+Alt+F` to format your `.craft` file.

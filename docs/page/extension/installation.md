# Extension Installation

## Prerequisites

- VSCode 1.75.0 or later

## Install from Marketplace

1. Open VSCode
2. Press `Ctrl+Shift+X` (Extensions)
3. Search for "**Craft**"
4. Click **Install**

## Install from Command Line

```bash
code --install-extension craft
```

## Verify Installation

Create a file `test.craft`:

```craft
use_case "Test" {
  when user does something
    Domain handles it
}
```

You should see syntax highlighting!

## Next Steps

- [Learn about features](/extension/features)
- [See all commands](/extension/commands)
- [Configure the extension](/extension/configuration)

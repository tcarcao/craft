# Craft Language Extension

This extension provides comprehensive language support for Craft DSL files including syntax highlighting, language server features, and interactive domain/service visualization.

## Features

- **Syntax Highlighting**: Tree-sitter based semantic highlighting for Craft DSL
- **Language Server**: Real-time validation, completion, and formatting
- **Domain Visualization**: Interactive tree views for domains and use cases
- **Service Visualization**: Explore services, boundaries, and architectural relationships
- **Live Preview**: Generate C4, context map, sequence, and domain diagrams
- **Real-time Updates**: Views update automatically as you edit DSL files

## Structure

```
.
├── client/                 # Language Client & UI
│   ├── src/
│   │   ├── extension.ts    # Main extension entry point
│   │   ├── providers/      # Webview providers for domain/service views
│   │   ├── services/       # DSL extraction and processing services
│   │   ├── webview/        # React components for tree views
│   │   └── TreeSitterHighlightProvider.ts
├── server/                 # Language Server
│   └── src/
│       └── server.ts       # Language server implementation
├── resources/              # Tree-sitter grammar and queries
├── dist/                   # Bundled extension files
└── package.json           # Extension manifest and build scripts
```

## Installation & Packaging

### For Users
1. Download the latest `.vsix` from releases
2. Install: `code --install-extension craft-0.0.1.vsix`

### For Developers - Building from Source

#### Prerequisites
```bash
# Install dependencies
npm install
```

#### Development Workflow
```bash
# Development mode (watch files, rebuild on changes)
npm run dev

# Debug in VS Code
# Press F5 to launch Extension Development Host
# Uses debug configuration for sourcemaps
```

#### Production Packaging
```bash
# Build optimized bundle
npm run bundle

# Package extension
npx @vscode/vsce package

# Install locally
code --install-extension craft-0.0.1.vsix
```

## Build Scripts

- `npm run bundle` - Production build (minified, optimized)
- `npm run dev` - Development build with watch mode  
- `npm run debug` - Debug build with sourcemaps
- `npm run bundle-client` - Bundle main extension code
- `npm run bundle-server` - Bundle language server
- `npm run bundle-webviews` - Bundle React webview components
- `npm run copy-static-assets` - Copy CSS and icon assets

## Extension Architecture

### Bundled Structure (`dist/`)
- `client.js` - Main extension bundle (~711KB)
- `server.js` - Language server bundle (~1.4MB) 
- `webview/domains.js` - Domain tree React app (~158KB)
- `webview/services.js` - Services tree React app (~161KB)
- `styles/treeStyles.css` - UI styles
- `@vscode/codicons/` - VS Code icons
- `tree-sitter-XXXXX.wasm` - Tree-sitter grammar (~206KB)

### Key Components
- **TreeSitterHighlightProvider**: Semantic syntax highlighting
- **DomainsViewProvider**: Interactive domain exploration
- **ServicesViewProvider**: Service architecture visualization  
- **Language Server**: Validation, completion, formatting via tree-sitter

## Development

### Debug Configuration
The extension includes two debug configurations:

1. **Launch Client (TypeScript)** - Debug uncompiled TypeScript source
2. **Launch Client (Bundled)** - Debug bundled production code

### File Watching
```bash
# Watch and rebuild everything
npm run dev

# Watch individual components
npm run dev-client      # Main extension
npm run dev-server      # Language server  
npm run dev-webviews    # React webviews
```

### Testing
1. Press F5 in VS Code to launch Extension Development Host
2. Open a `.craft` file to test:
   - Syntax highlighting
   - Domain/Service tree views (sidebar)
   - Preview commands (Ctrl+Shift+C/M/S/D)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and test thoroughly
4. Run `npm run bundle` to ensure production build works
5. Submit a pull request

## Publishing

```bash
# Build and package
npm run bundle
npx @vscode/vsce package

# Publish to marketplace (requires publisher account)
npx @vscode/vsce publish
```
# Craft Documentation

Official documentation for the Craft language and VSCode extension.

## Quick Start

### Development

```bash
# Install dependencies
npm install

# Start dev server
npm run docs:dev
```

Visit `http://localhost:5173`

### Build

```bash
# Build for production
npm run docs:build

# Preview production build
npm run docs:preview
```

## Documentation Structure

- **guide/** - Getting started and tutorials
- **language/** - Complete language reference
- **extension/** - VSCode extension documentation
- **examples/** - Full working examples

## Contributing

1. Edit markdown files in their respective folders
2. Test locally with `npm run docs:dev`
3. Build and verify with `npm run docs:build`
4. Submit a pull request

## Tech Stack

- **VitePress** - Fast static site generator
- **Vue 3** - Framework
- **Custom CSS** - Craft syntax highlighting
- **GitHub Actions** - Automated deployment

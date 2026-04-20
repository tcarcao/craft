# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Craft is a domain-specific language for modeling business use cases and domain interactions with powerful visualization capabilities for domain-driven design and C4 architecture diagrams. The project is currently in v2.0 development, representing a transformation from static architecture description to dynamic use case modeling.

## Core Architecture

### Language Components
- **Grammar**: ANTLR4 grammar located in `tools/antlr-grammar/Craft.g4` defines the DSL syntax
- **Parser**: Go parser implementation in `pkg/parser/` and `internal/parser/` generated from ANTLR grammar
- **Visualizer**: Diagram generation engine in `internal/visualizer/` that produces C4, context map, sequence, and domain diagrams
- **Processor**: Core processing logic in `internal/processor/` that orchestrates parsing and visualization

### Project Structure
- `cmd/craft/` - CLI tool for batch processing DSL files into diagrams
- `cmd/server/` - HTTP server providing web interface and API endpoints for real-time diagram generation
- `internal/` - Core business logic (parser, visualizer, processor)
- `pkg/` - Public APIs and shared components
- `tools/antlr-grammar/` - ANTLR4 grammar definition
- `tools/vscode-extension/` - VS Code extension with language server, syntax highlighting, and live preview
- `examples/` - DSL example files demonstrating language features

## Essential Commands

### Grammar Generation (Required before building)
```bash
# Pull ANTLR builder image and generate parser code
make docker-pull-antlr-image generate-grammar
```

### Building and Running
```bash
# Build Docker image
make docker-build

# Run web server (http://localhost:8080)
make docker-run

# Run tests
make test
```

### VS Code Extension Development
```bash
cd tools/vscode-extension
pnpm install
pnpm run compile    # Compile TypeScript
pnpm run watch      # Watch mode for development
```

## DSL Language Features

The Craft language supports:
- **Services**: Group domains into deployable units with technology stack specifications
- **Use Cases**: Model business scenarios through triggers (external, events, listeners, scheduled) and actions (sync, async, internal)
- **Exposures**: Define external access points and their relationships
- **Domains**: Core business domains that interact within use cases

## Development Dependencies

### External Tools Required
- **plantuml**: For generating PNG diagrams from PlantUML files
- **dot (Graphviz)**: For generating PNG diagrams from DOT files
- Both are required by the CLI tool (`cmd/craft/main.go`) for diagram generation

### Container Requirements
- **Docker or Podman**: The Makefile auto-detects the available runtime
- **ANTLR4 Docker Image**: Pulled from `tiagocarcao/antlr4-craft:4.13.2` on Docker Hub (no local build needed)

## VS Code Extension Integration

The extension provides:
- Syntax highlighting for `.craft` files
- Language server with real-time validation
- Domain and services tree views
- Live diagram preview commands:
  - `Ctrl+Shift+C`: Preview C4 diagram
  - `Ctrl+Shift+M`: Preview context map
  - `Ctrl+Shift+S`: Preview sequence diagram
  - `Ctrl+Shift+D`: Preview domain diagram

## Key Technical Notes

- Grammar changes require regeneration via `make generate-grammar`
- The web server at `:8080` provides API endpoints for VS Code extension integration
- All diagram generation happens server-side and returns base64-encoded PNGs
- The parser supports multi-file DSL projects with cross-file domain references
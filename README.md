# ArchDSL

A Domain Specific Language for describing software architecture, with support for generating various diagram types.

## Overview

ArchDSL allows you to describe software systems using a simple, readable syntax focused on:
- System boundaries and bounded contexts
- Components, services, and aggregates
- Domain-driven design concepts

## Grammar

The DSL is built using ANTLR4 and supports:
- System definitions
- Bounded context declarations  
- Aggregate, component, and service definitions

## Building

### Prerequisites
- Go 1.22+
- Java (for ANTLR code generation)

### Build Steps
```bash
# Generate ANTLR parser code
make generate

# Build the project
make build

# Run tests
make test
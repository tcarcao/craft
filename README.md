# ArchDSL

A Domain Specific Language for describing software architecture, with support for generating various diagram types.

## Overview

ArchDSL allows you to describe software systems using a simple, readable syntax focused on:
- System boundaries and bounded contexts
- Components, services, and aggregates
- Domain-driven design concepts
- Service interactions and flows

## Features

- **ANTLR-based grammar** for precise parsing
- **Technology annotations** (go, java, python, nodejs, php)
- **Platform specifications** (eks, lambda, sqs, sns, dynamodb, redis)
- **DDD patterns** (upstream/downstream relationships with ACL, OHS, Conformist patterns)
- **Service flows** with argument passing
- **Diagram generation** (coming soon)

## Grammar

The DSL supports:
- System and bounded context definitions
- Aggregate, component, and service declarations
- Technology and platform annotations
- Relationship patterns between contexts
- Service interaction flows

## Building

### Prerequisites
- Go 1.22+
- Java (for ANTLR code generation)
- PlantUML (for diagram generation)
- Graphviz (for context maps)

### Build Steps
```bash
# Generate ANTLR parser code
make generate

# Build the project
make build

# Run tests
make test
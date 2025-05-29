# ArchDSL

A domain-specific language for modeling use cases, domains, and services with automatic diagram generation.


## v2 Development (In Progress)
We are transitioning ArchDSL from static architecture description to dynamic use case modeling. This will enable:
- Business-driven domain modeling
- Event-driven architecture support
- Better alignment with Domain-Driven Design
- Support for complex business scenario modeling

See `docs/v2-research.md` for detailed research and direction.

Current v1 documentation remains below...

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
make docker-build-antlr-image generate-grammar

# Build the project
make docker-build

# Run the project
make docker-run

# Run tests
make test
```
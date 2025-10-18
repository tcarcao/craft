# Introduction

## What is Craft?

Craft is a **domain-specific language (DSL)** for modeling business use cases and domain interactions. It combines Domain-Driven Design principles with executable architecture models that generate visual diagrams automatically.

## Key Features

### 🎯 Domain-Driven Design
Model your business domains, subdomains, and bounded contexts using clear, declarative syntax.

### 📊 Automatic Diagram Generation
Generate C4 architecture diagrams, domain flow diagrams, and sequence diagrams from your code.

### ⚡ Event-Driven Architecture
Define asynchronous events, domain listeners, and event-driven communication patterns.

### 🔧 Full IDE Support
VSCode extension with syntax highlighting, code completion, real-time diagnostics, and live preview.

### 🎨 Use Case Modeling
Describe business scenarios through triggers and actions in natural, readable language.

### 🚀 Microservices Architecture
Define services with technology stacks, deployment strategies, and data stores.

## Language Overview

Craft allows you to define:

- **Domains** - Core business domains and subdomains
- **Actors** - Users, systems, and services that interact with your system
- **Services** - Deployable units with technology specifications
- **Use Cases** - Business scenarios with triggers and domain actions
- **Architecture** - Component flows and system design
- **Exposures** - External access points and API gateways

## Simple Example

```craft
// Define actors
actors {
  user Customer
  service OrderService
}

// Define domains
domain Order {
  OrderManagement
  OrderTracking
}

// Define a use case
use_case "Place Order" {
  when Customer creates order
    OrderManagement validates items
    OrderManagement notifies "Order Created"
}
```

## Why Use Craft?

### Traditional Approach
- Architecture docs in separate tools (Visio, Lucidchart, etc.)
- Manual diagram updates when code changes
- Documentation drift and inconsistency
- Hard to keep architecture and code in sync

### With Craft
- Architecture as code, living with your source
- Diagrams generated automatically
- Single source of truth
- Easy to review in pull requests
- Versioned alongside your code

## Use Cases

Craft is ideal for:

- **System Design** - Model new systems before implementation
- **Documentation** - Document existing architectures
- **Microservices** - Define service boundaries and interactions
- **Event-Driven Systems** - Model async communication patterns
- **Domain-Driven Design** - Express bounded contexts and domain models
- **Team Communication** - Shared language between developers, architects, and stakeholders

## Next Steps

- [Install Craft](/guide/installation) to get started
- [Quick Start Tutorial](/guide/quickstart) to write your first use case
- [Language Reference](/language/overview) for complete syntax guide
- [Examples](/examples/ecommerce) to see real-world applications

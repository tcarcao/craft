---
layout: home

hero:
  name: Craft
  text: Domain-Driven Architecture Language
  tagline: Model business use cases and domain interactions with powerful visualization capabilities
  actions:
    - theme: brand
      text: Get Started
      link: /guide/introduction
    - theme: alt
      text: View Examples
      link: /examples/ecommerce
    - theme: alt
      text: GitHub
      link: https://github.com/tcarcao/craft-vscode-extension

features:
  - icon: 🎯
    title: Domain-Driven Design
    details: Model your business domains, subdomains, and their interactions using clear, declarative syntax
  - icon: 📊
    title: Visual Diagrams
    details: Generate C4, domain flow, and sequence diagrams automatically from your code
  - icon: ⚡
    title: Event-Driven Architecture
    details: Define asynchronous events, listeners, and event-driven flows with ease
  - icon: 🔧
    title: VSCode Integration
    details: Full IDE support with syntax highlighting, live preview, and interactive explorers
  - icon: 🎨
    title: Use Case Modeling
    details: Describe business scenarios through triggers and actions in natural language
  - icon: 🚀
    title: Microservices Ready
    details: Define services with deployment strategies, tech stacks, and data stores
---

## Quick Example

```craft
// Define your services
services {
  OrderService {
    domains: Order, Payment
    language: nodejs
    data-stores: order_db
    deployment: canary(50% -> staging, 100% -> production)
  }
}

// Model business use cases
use_case "Order Placement" {
  when Customer places order
    Order validates product availability
    Order asks Inventory to reserve items
    Order creates payment request
    Order notifies "Order Created"

  when Payment listens "Order Created"
    Payment processes transaction
    Payment notifies "Payment Processed"
}
```

## Why Craft?

**Traditional architecture documentation** is often disconnected from code, quickly becomes outdated, and requires specialized tools.

**Craft provides** a simple DSL that lives with your code, generates diagrams automatically, and makes architecture visible and maintainable.

## Get Started in 3 Steps

1. **Install the VSCode extension** from the marketplace
2. **Create a `.craft` file** and start modeling your domains
3. **Press `Ctrl+Shift+C`** to preview your architecture diagram

[Read the full guide →](/guide/introduction)

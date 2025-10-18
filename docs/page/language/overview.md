# Language Overview

Craft is a declarative DSL for modeling business architectures. This reference covers all language constructs.

## File Extension

Craft files use the `.craft` extension.

## Comments

```craft
// Single-line comment

/*
  Multi-line
  comment
*/
```

## Top-Level Constructs

A Craft file can contain any combination of these blocks:

- **Actors** - Define users, systems, and services
- **Domains** - Define business domains and subdomains
- **Services** - Define deployable services with tech stacks
- **Use Cases** - Model business scenarios and flows
- **Architecture** - Define component flows and system design
- **Exposures** - Define external access points

## Basic Syntax Rules

### Identifiers

```craft
// Valid identifiers
UserService
user_profile
API-Gateway
auth_service_v2
```

**Rules:**
- Start with letter or underscore
- Can contain letters, numbers, underscores, hyphens, dots
- Case-sensitive

### Strings

```craft
"User Registration"
"Order Placed"
"Welcome to the system"
```

**Rules:**
- Double-quoted
- Cannot span multiple lines
- Use `\"` to escape quotes

### Lists

Comma-separated, with optional trailing comma:

```craft
domains: Authentication, Profile, Order
domains: Authentication, Profile, Order,
```

## Minimal Example

```craft
actors {
  user Customer
}

domain Order {
  OrderManagement
}

use_case "Place Order" {
  when Customer places order
    OrderManagement creates order
}
```

## Language Philosophy

### Declarative
Describe **what** the system does, not **how** it does it.

### Natural Language
Use readable phrases like `Customer places order` instead of `customer_place_order()`.

### Event-Driven
First-class support for async events and domain listeners.

### Domain-Focused
Center your models around business domains, not technical components.

## Quick Reference

| Construct | Keyword | Purpose |
|-----------|---------|---------|
| Actors | `actors`, `actor` | Define system actors |
| Domains | `domains`, `domain` | Define business domains |
| Services | `services`, `service` | Define deployable services |
| Use Cases | `use_case` | Model business scenarios |
| Architecture | `arch` | Define component flows |
| Exposures | `exposure` | Define API access |

## Next Steps

- [Actors](/language/actors) - Define users, systems, and services
- [Domains](/language/domains) - Organize business capabilities
- [Services](/language/services) - Group domains into deployables
- [Use Cases](/language/use-cases) - Model business logic
- [Architecture](/language/architecture) - Define system components
- [Exposures](/language/exposures) - Control external access

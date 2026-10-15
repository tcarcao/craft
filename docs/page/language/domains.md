# Domains

Domains represent core business capabilities in your system. They are the building blocks of domain-driven design in Craft.

## Single Domain

Define one domain at a time:

```craft
domain Authentication {
  Login
  Registration
  PasswordReset
}
```

**Syntax:**
```craft
domain <Name> {
  <bounded context>*
}
```

## Multiple Domains

Define multiple domains in one block:

```craft
domains {
  Authentication {
    Login
    Registration
    PasswordReset
  }

  Profile {
    UserProfile
    Settings
    Preferences
  }

  Order {
    OrderManagement
    OrderTracking
    OrderHistory
  }
}
```

**Syntax:**
```craft
domains {
  <Name> {
    <bounded context>*
  }*
}
```

## bounded contexts

bounded contexts are capabilities within a domain:

```craft
domain Payment {
  PaymentProcessing    // Core capability
  PaymentValidation    // Supporting capability
  RefundManagement     // Supporting capability
  PaymentHistory       // Reporting capability
}
```

::: tip
bounded contexts help you organize related capabilities and identify bounded contexts.
:::

## Display names (2.27)

A bounded context id is the right identity for the model, but a reader may not recognise it. Give it a display name once, in the domain block, as a quoted string after the name on the same line:

```craft
domains {
  billing {
    nmf_monetization  "nMF, the money engine"
    portfoliovalue    "portfolio value"
    invoicearchive
  }
}
```

Use cases keep using the id (`nmf_monetization asks …`). Diagrams show the display name and keep the id as the diagram identifier:

```plantuml
participant "nMF, the money engine" as nmf_monetization
```

Mermaid writes `participant nmf_monetization as nMF, the money engine`. Flow, architecture and C4 nodes and event queues show the display name too, and the surface table heading reads `nMF, the money engine (nmf_monetization)`. Hover and completion in the editor show it.

Rules:
- The string must start on the name's line. A string on its own line is a syntax error.
- Display names are declared in domain blocks only, not in a service's `contexts:` list.
- Two contexts or actors with the same display name get a `craft/lint/duplicate-display-name` warning; `""` gets `craft/lint/empty-display-name` and is ignored.
- A context or actor declared again with another display name gets a `craft/lint/conflicting-display-name` warning. Diagrams draw the first declaration, in sorted file order.
- A file without display names renders exactly as before.

## Domain Naming

### Use Business Terms

✅ Good:
```craft
domain Order {
  OrderManagement
  Fulfillment
}
```

❌ Bad:
```craft
domain OrderService {
  OrderAPI
  OrderDB
}
```

### Be Specific

✅ Good:
```craft
domain Payment {
  CreditCardProcessing
  RefundManagement
}
```

❌ Bad:
```craft
domain Payment {
  Process
  Manage
}
```

## Example: E-commerce Domains

```craft
domains {
  // Customer-facing capabilities
  Catalog {
    ProductBrowsing
    ProductSearch
    Recommendations
  }

  Order {
    ShoppingCart
    Checkout
    OrderTracking
  }

  Payment {
    PaymentProcessing
    PaymentMethods
    Refunds
  }

  // Internal capabilities
  Inventory {
    StockManagement
    Warehouse
    Replenishment
  }

  Shipping {
    ShippingCalculation
    LabelGeneration
    TrackingIntegration
  }

  // Supporting capabilities
  Notification {
    EmailNotification
    SMSNotification
    PushNotification
  }

  Analytics {
    UserBehavior
    SalesMetrics
    InventoryMetrics
  }
}
```

## Using Domains in Use Cases

Domains are the actors in your use case actions:

```craft
domains {
  Order {
    OrderManagement
  }
  Payment {
    PaymentProcessing
  }
  Inventory {
    StockManagement
  }
}

use_case "Place Order" {
  when Customer places order
    // Domains performing actions
    OrderManagement validates items
    OrderManagement asks Inventory to reserve stock
    OrderManagement asks Payment to create payment
}
```

## Domain-Driven Design Concepts

### Core Domain

Your main business differentiator:

```craft
domain Recommendation {  // Core domain for e-commerce
  PersonalizedRecommendations
  MLModel
  UserPreferences
}
```

### Supporting Domains

Necessary but not differentiating:

```craft
domain Notification {  // Supporting domain
  EmailSending
  SMSDelivery
}
```

### Generic Domains

Could be replaced by third-party solutions:

```craft
domain Authentication {  // Could use Auth0, Okta, etc.
  Login
  Registration
}
```

## Best Practices

### Keep Domains Cohesive

Each domain should have a single, clear responsibility.

✅ Good:
```craft
domain Order {
  OrderCreation
  OrderModification
  OrderCancellation
}
```

❌ Bad:
```craft
domain OrderAndShipping {
  OrderCreation
  ShippingLabel
  TrackingNumber
}
```

### Limit bounded contexts

Keep 3-7 bounded contexts per domain for clarity.

✅ Good:
```craft
domain Payment {
  PaymentProcessing
  RefundProcessing
  PaymentValidation
  PaymentMethods
}
```

❌ Bad:
```craft
domain Payment {
  // 20+ bounded contexts...
}
```

### Use Consistent Naming

Pick a naming pattern and stick with it:

```craft
domains {
  Order {
    OrderManagement      // -Management suffix
    OrderTracking        // -ing suffix
    OrderHistory         // Noun suffix
  }
}
```

## Domain Relationships

Domains interact through use case actions:

```craft
domains {
  Order { OrderManagement }
  Payment { PaymentProcessing }
  Inventory { StockManagement }
}

use_case "Order Flow" {
  when Customer places order
    // Order → Inventory (synchronous)
    OrderManagement asks StockManagement to reserve items

    // Order → Payment (synchronous)
    OrderManagement asks PaymentProcessing to charge customer

    // Order → Everyone (asynchronous)
    OrderManagement notifies order.OrderPlaced

  when StockManagement listens order.OrderPlaced
    StockManagement updates inventory
}
```

## Domain Relationships (Strategic View)

The use-case interactions above are the **communication view** — who calls whom. Craft also has a **strategic view**, [`context_map`](/language/context-map), for classifying how the bounded contexts declared here relate using DDD patterns (customer/supplier, conformist, anticorruption layer, and others):

```craft
context_map Order {
  OrderManagement open_host_service OrderTracking
}
```

## Next Steps

- Learn about [services](/language/services) to group domains
- Model interactions with [use cases](/language/use-cases)
- Classify relationships with the [context map](/language/context-map)
- Define [actors](/language/actors) that interact with domains
- See [complete examples](/examples/ecommerce)

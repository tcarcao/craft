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
  <Subdomain>*
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
    <Subdomain>*
  }*
}
```

## Subdomains

Subdomains are capabilities within a domain:

```craft
domain Payment {
  PaymentProcessing    // Core capability
  PaymentValidation    // Supporting capability
  RefundManagement     // Supporting capability
  PaymentHistory       // Reporting capability
}
```

::: tip
Subdomains help you organize related capabilities and identify bounded contexts.
:::

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

### Limit Subdomains

Keep 3-7 subdomains per domain for clarity.

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
  // 20+ subdomains...
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
    OrderManagement notifies "Order Placed"

  when StockManagement listens "Order Placed"
    StockManagement updates inventory
}
```

## Next Steps

- Learn about [services](/language/services) to group domains
- Model interactions with [use cases](/language/use-cases)
- Define [actors](/language/actors) that interact with domains
- See [complete examples](/examples/ecommerce)

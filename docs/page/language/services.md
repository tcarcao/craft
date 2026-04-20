# Services

Services group related domains into deployable units with technology specifications and deployment strategies.

## Basic Syntax

```craft
service UserService {
  contexts: Authentication, Profile
  language: nodejs
  data-stores: user_db
  deployment: rolling
}
```

## Multiple Services Block

```craft
services {
  UserService {
    contexts: Authentication, Profile
    language: nodejs
    data-stores: user_db, cache
    deployment: rolling
  }

  OrderService {
    contexts: Order, Payment
    language: java
    data-stores: order_db, payment_db
    deployment: canary(50% -> staging, 100% -> production)
  }
}
```

## Service Properties

### domains
Comma-separated list of domains this service handles:

```craft
contexts: Authentication, Profile, Settings
```

### language
Programming language or platform:

```craft
language: nodejs
language: java
language: python
language: golang
language: rust
```

### data-stores
Databases and storage systems:

```craft
data-stores: postgres_db, redis_cache, s3_bucket
```

### deployment
Deployment strategy:

```craft
// Simple
deployment: rolling
deployment: blue_green

// With routing rules
deployment: canary(50% -> staging, 100% -> production)
```

## Deployment Strategies

### Rolling Deployment
Sequential instance updates:

```craft
deployment: rolling
```

### Blue-Green Deployment
Parallel environment switching:

```craft
deployment: blue_green
```

### Canary Deployment
Gradual rollout with traffic routing:

```craft
deployment: canary(
  10% -> canary-production,
  50% -> staging-production,
  100% -> production
)
```

## Complete Example

```craft
services {
  APIGateway {
    contexts: Routing, Authentication
    language: nodejs
    data-stores: gateway_cache
    deployment: rolling
  }

  UserService {
    contexts: Profile, Settings, Preferences
    language: golang
    data-stores: user_db, user_cache
    deployment: canary(50% -> staging, 100% -> production)
  }

  OrderService {
    contexts: Order, Cart, Checkout
    language: java
    data-stores: order_db, order_event_store
    deployment: blue_green
  }

  InventoryService {
    contexts: Inventory, Warehouse
    language: rust
    data-stores: inventory_db
    deployment: rolling
  }
}
```

## Best Practices

### Group Related Domains
```craft
✅ UserService {
  contexts: Authentication, Profile, Settings
}

❌ MixedService {
  contexts: Authentication, Order, Inventory
}
```

### Choose Appropriate Deployment Strategy
- **rolling**: Safe, standard deployments
- **blue_green**: Zero-downtime critical services
- **canary**: Gradual rollout for risky changes

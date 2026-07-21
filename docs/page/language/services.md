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

### catalog_ref
The service's stable identifier in your organization's service catalog:

```craft
catalog_ref: subscriptions-api
```

### repo
The source repository reference:

```craft
repo: olxeu/realestate/subscriptions
```

## Code Anchors

`catalog_ref:` and `repo:` are **code anchors** — they bind a `service` block to
the real thing it describes. Both are optional; a service that declares neither
behaves exactly as it always has.

The governing principle is **bind by identity, never by location**. Anchors name
*what* a service is, not *where* its files sit — there are no file paths and no
line numbers in either value, because those rot on the first refactor.

- **`catalog_ref:`** is an *immutable identity anchor*: the token your service
  catalog uses to identify this service, stable across renames. The language
  deliberately does not name the catalog vendor — which catalog resolves the
  anchor is deployment configuration, not part of the grammar, so the catalog can
  be swapped without a DSL migration. (It is `catalog_ref`, not `catalog_slug`:
  "slug" is reserved in this vocabulary for *mutable* human-readable names.)
- **`repo:`** is a repository slug, not a checkout path.

Each anchor may appear **at most once per service**; a repeat is a
`craft/sema/duplicate-service-anchor` error.

Craft validates *shape* only — that a declared anchor is present and
well-formed. Resolving an anchor against a real catalog or a real repository is
the consuming system's job, not the language's.

```craft
services {
  SubscriptionsApi {
    contexts: Subscriptions
    catalog_ref: subscriptions-api
    repo: olxeu/realestate/subscriptions
  }
}
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

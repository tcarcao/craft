# Stack and Exposure Modeling

## Context
Current DSL effectively models services, domains, and use cases but lacks:
- Gateway/composition layer representation
- Different access patterns for various audiences  
- Infrastructure concerns for C4 diagram generation
- Component orchestration and request flows

## Decision

### Add `arch` blocks for component inventory
```
arch production {
  presentation:
    cdn[cloudflare] > Web[nextjs]
    cdn[cloudflare] > Mobile[flutter]
    Developer Portal[nextjs]
  
  gateway:
    cdn[cloudflare] > L7LoadBalancer > GraphQL[gqlgen]
    API GW > Partner API[authn:oauth2]
}
```

### Add `exposure` blocks for access patterns
```
customer-facing exposure {
  to: Web, Mobile
  through: GraphQL
}

partner exposure {
  to: Developer Portal, external
  through: Partner API
}
```

### Enhance services with deployment strategies
```
services {
  WalletService: {
    domains: Wallet, WalletItemPurchase
    data-stores: postgres, redis
    deployment: canary(90% -> stable, 10% -> experimental)
  }
}
```

## Key Principles
- **`arch`** = what exists (component inventory)
- **`exposure`** = how it's accessed (business access patterns)
- **`>`** operator for request flow
- Multiple `arch` blocks for different environments
- Services own routing details (future enhancement)

## Consequences

### Positive
- Clear C4 diagram generation with proper gateway representation
- Business-aligned exposure patterns
- Environment-specific architecture modeling
- Maintains domain-focused approach

### Negative
- Additional complexity in DSL
- Need to update grammar and tooling
- Learning curve for existing users

## Future Evolutions
- Infrastructure modeling (`infrastructure:` section)
- Conditional gateway patterns
- Service-level routing details
- External system integration
- Cross-cutting concerns (monitoring, caching)

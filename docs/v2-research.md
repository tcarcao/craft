# ArchDSL v2 Research: Use Case Modeling Approach

## Current v1 Limitations
- Architecture descriptions are static, don't capture business logic flow
- Hard to express domain interactions and business rules
- Limited support for event-driven architectures
- Difficult to model complex business scenarios

## Proposed v2 Direction: Use Case Modeling
- Focus on business use cases as primary modeling unit
- Capture domain interactions through scenarios
- Support event-driven and synchronous domain communications
- Better alignment with Domain-Driven Design principles

## Initial Grammar Ideas

```
use_case "User Registration" {
    when user submits registration
        authentication validates credentials
        profile creates user record
        notification sends welcome email
}
```

## Research References
- Domain-Driven Design (Eric Evans)
- Use Case Driven Object Modeling (Doug Rosenberg)
- Event Storming methodology


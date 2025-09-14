# Use Case Modeling Analysis

## Why Pivot from Architecture DSL?

Current v1 Craft focuses on static architecture:
- System definitions
- Component relationships  
- Data flow descriptions

But real systems are driven by business use cases:
- What happens when a user performs an action?
- How do different domains collaborate?
- What events trigger business processes?

## Proposed Grammar Evolution

### Phase 1: Basic Use Cases

```
use_case "name" {
    when <trigger>
        <actions>
}
```

### Future: Advanced Scenarios
- Multiple trigger types
- Domain collaboration patterns
- Event-driven flows
- Error handling scenarios


# Actors

Actors represent entities that interact with your system. Six kinds are known: `user`, `system`, `service`, and (since 2.24) `scheduler`, `tool` and `infra`. Any other kind is still accepted and is drawn as a person.

## Actor Types

### User
Human users of your system:

```craft
actor user Customer
actor user Admin
actor user SupportAgent
```

### System
External systems:

```craft
actor system PaymentGateway
actor system EmailService
actor system SMSProvider
```

### Service
Internal services (actors from other bounded contexts):

```craft
actor service AnalyticsService
actor service AuditService
```

### Scheduler, Tool, Infra (2.24)
Initiators that are not people:

```craft
actors {
  scheduler Ticker          // a clock that starts work: cron, an in-process ticker
  tool OperatorTool         // a CLI, script or client a person runs; it makes the calls
  infra Pipeline            // platform machinery: pod start, probes, CI pipelines, monitors
}
```

They sit outside the system, like `system`, so a trigger from one draws an arrow. Diagrams draw them as stereotyped boundaries, not persons:

| Kind | PlantUML | Mermaid sequence | C4 |
|---|---|---|---|
| `user` (and any unknown kind) | `actor` | `actor` | `Person`, "External user" |
| `system` | `boundary` | `actor` | `System_Ext`, "External system" |
| `service` | `control` | `actor` | `System_Ext`, "External service" |
| `scheduler` | `boundary … <<scheduler>>` | `participant … (scheduler)` | `System_Ext`, "Scheduler" |
| `tool` | `boundary … <<tool>>` | `participant … (tool)` | `System_Ext`, "Operator tool" |
| `infra` | `boundary … <<infra>>` | `participant … (infra)` | `System_Ext`, "Platform infrastructure" |

A `when cron` or `when every` trigger starts from the actor named `CRON`, whatever its kind, so `scheduler CRON` works.

::: warning Needs craft 2.24
Craft 2.23 and earlier read `scheduler CRON` (or any kind outside `user`/`system`/`service`) as an actor named after the kind, so a second `scheduler …` line was a duplicate-name error. Use the new kinds only with craft 2.24 or later.
:::

## Single Actor Definition

```craft
actor user Customer
actor system PaymentGateway
actor service NotificationService
```

## Multiple Actors Block

```craft
actors {
  user Customer
  user Admin
  system PaymentGateway
  system EmailService
  service AnalyticsService
}
```

## Display names (2.27)

An actor may carry a display name, a quoted string after its name on the same line. Diagrams and hovers show it; use cases keep the name:

```craft
actors {
  user Seller        "the seller"
  scheduler CRON     "the hourly clock"
}

actor system AdCore "OLX ad-core"
```

PlantUML draws `actor "the seller" as Seller`. See [Domains](./domains.md#display-names-2-27) for the rules, which are the same for bounded contexts.

## Using Actors in Use Cases

Actors appear in external triggers:

```craft
actors {
  user Customer
  user Admin
  system EmailService
}

use_case "Order Management" {
  // User actor triggers
  when Customer places order
    Order creates order record

  // Admin actor triggers
  when Admin approves order
    Order updates status

  // System actor involvement
  when Order listens order.OrderConfirmed
    Order asks EmailService to send confirmation
}
```

## Best Practices

### Name by Role, Not Person
✅ `user Admin`
❌ `user JohnDoe`

### Use Specific Names
✅ `system PaymentGateway`
❌ `system External`

### Group Related Actors
```craft
actors {
  // Human users
  user Customer
  user Admin

  // External systems
  system PaymentGateway
  system ShippingProvider

  // Internal services
  service AnalyticsService
}
```

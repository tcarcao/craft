# Actors

Actors represent entities that interact with your system. There are three types: users, systems, and services.

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
  when Order listens "Order Confirmed"
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

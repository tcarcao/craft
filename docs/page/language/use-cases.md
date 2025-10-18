# Use Cases

Use cases model business scenarios through triggers and domain actions. They are the core of Craft's dynamic modeling approach.

## Syntax

```craft
use_case "<name>" {
  when <trigger>
    <action>*

  when <trigger>
    <action>*
}
```

## Basic Example

```craft
use_case "Order Placement" {
  when Customer places order
    Order validates items
    Order creates order record
    Order notifies "Order Created"
}
```

## Triggers

Triggers define what starts a scenario. There are four types:

### External Triggers

Initiated by actors:

```craft
when user submits registration
when admin approves order
when customer places order
when system sends notification
```

**Syntax:**
```craft
when <actor> <verb> [connector] <phrase>
```

**Connector words** (optional): `a`, `an`, `the`, `to`, `from`, `in`, `on`, `at`, `for`, `with`, `by`

### Event Triggers

Initiated by events:

```craft
when "Order Placed"
when "User Registered"
when "Payment Processed"
```

**Syntax:**
```craft
when "<event name>"
```

::: tip
Use past tense for event names: "Order Placed" not "Place Order"
:::

### Domain Listener Triggers

Domains reacting to events:

```craft
when Payment listens "Order Created"
when Notification listens "User Registered"
when Inventory listens "Order Cancelled"
```

**Syntax:**
```craft
when <domain> listens "<event name>"
```

### CRON Triggers

Scheduled tasks:

```craft
when CRON runs daily cleanup
when CRON executes hourly sync
```

**Syntax:**
```craft
when CRON [phrase]
```

## Actions

Actions describe what domains do. There are four types:

### Synchronous Actions

Direct domain-to-domain communication:

```craft
Order asks Inventory to reserve items
Authentication asks Database to verify credentials
Payment asks Gateway for transaction status
```

**Syntax:**
```craft
<domain> asks <domain> [connector] <phrase>
```

**Use when:** One domain needs an immediate response from another.

### Asynchronous Actions

Publish events:

```craft
Order notifies "Order Created"
Payment notifies "Payment Processed"
Profile notifies "User Updated"
```

**Syntax:**
```craft
<domain> notifies "<event name>"
```

**Use when:** Other domains might want to react, but the publisher doesn't need a response.

### Internal Actions

Domain internal operations:

```craft
Order validates items
Profile creates user record
Authentication generates token
Inventory updates stock levels
```

**Syntax:**
```craft
<domain> <verb> [connector] <phrase>
```

**Use when:** A domain does something internally without calling other domains.

### Return Actions

Return responses:

```craft
Database returns user record to Authentication
Payment returns confirmation
API returns error message to Client
Gateway returns transaction result to Payment
```

**Syntax:**
```craft
<domain> returns [connector] <phrase> [to <domain>]
```

**Use when:** A domain returns data, especially in response to an `asks` action.

## Complete Example

```craft
use_case "Order Processing" {
  // External trigger: customer action
  when Customer places order
    Order validates product availability
    Order asks Inventory to reserve items
    Order calculates total amount
    Order asks Payment to create payment request
    Order notifies "Order Created"

  // Domain listener: Payment reacts to Order Created
  when Payment listens "Order Created"
    Payment asks PaymentGateway to process transaction
    PaymentGateway returns transaction result to Payment
    Payment updates payment status
    Payment notifies "Payment Processed"

  // Domain listener: Notification reacts to Payment
  when Notification listens "Payment Processed"
    Notification asks EmailService to send confirmation
    Notification asks SMSService to send notification

  // Domain listener: Inventory reacts to Payment
  when Inventory listens "Payment Processed"
    Inventory confirms reservation
    Inventory updates stock levels
}
```

## Event-Driven Pattern

```craft
use_case "User Registration" {
  when user submits registration
    Authentication validates email format
    Authentication asks Database to check uniqueness
    Profile creates initial profile
    Authentication notifies "User Registered"

  when Profile listens "User Registered"
    Profile asks Database to store profile
    Profile notifies "Profile Created"

  when Notification listens "User Registered"
    Notification asks EmailService to send welcome email

  when Analytics listens "User Registered"
    Analytics records registration event
    Analytics updates metrics
}
```

## Request-Response Pattern

```craft
use_case "Get User Profile" {
  when Customer requests profile
    API validates authentication token
    API asks Profile for user data
    Profile asks Database to fetch profile
    Database returns profile data to Profile
    Profile returns formatted profile to API
    API returns profile to Customer
}
```

## Error Handling Pattern

```craft
use_case "Process Payment" {
  when Customer submits payment
    Payment validates payment details
    Payment asks Gateway to charge card
    Gateway returns transaction result to Payment

  when Payment listens "Transaction Failed"
    Payment creates retry attempt
    Payment notifies "Payment Failed"

  when Order listens "Payment Failed"
    Order cancels order
    Order asks Inventory to release reservation
    Order notifies "Order Cancelled"
}
```

## Best Practices

### Use Past Tense for Events

✅ Good:
```craft
Order notifies "Order Created"
Payment notifies "Payment Processed"
```

❌ Bad:
```craft
Order notifies "Create Order"
Payment notifies "Process Payment"
```

### Be Specific with Actions

✅ Good:
```craft
Authentication validates email format
Order calculates total amount
```

❌ Bad:
```craft
Authentication validates
Order calculates
```

### Use Domain Names, Not Service Names

✅ Good:
```craft
Order asks Inventory to reserve items
```

❌ Bad:
```craft
OrderService asks InventoryService to reserve items
```

### Keep Scenarios Focused

Each `when` block should represent a cohesive scenario.

✅ Good:
```craft
use_case "Order Processing" {
  when Customer places order
    // 5-10 related actions

  when Payment listens "Order Created"
    // 3-5 related actions
}
```

❌ Bad:
```craft
use_case "Everything" {
  when Customer places order
    // 50+ unrelated actions
}
```

## Common Patterns

### Saga Pattern

```craft
use_case "Distributed Transaction" {
  when user initiates order
    Order creates order
    Order notifies "Order Started"

  when Payment listens "Order Started"
    Payment charges customer
    Payment notifies "Payment Completed"

  when Inventory listens "Payment Completed"
    Inventory ships items
    Inventory notifies "Shipment Sent"

  when Order listens "Payment Failed"
    Order cancels order
    Order notifies "Order Cancelled"
}
```

### CQRS Pattern

```craft
use_case "Order Management" {
  // Command side
  when user creates order
    OrderCommand validates order
    OrderCommand stores order
    OrderCommand notifies "Order Created"

  // Query side
  when OrderQuery listens "Order Created"
    OrderQuery updates read model
    OrderQuery indexes order data
}
```

## Next Steps

- See [complete examples](/examples/ecommerce) with multiple use cases
- Learn about [services](/language/services) to organize domains
- Understand [domains](/language/domains) to structure your model

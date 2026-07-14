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
    Order notifies order.OrderCreated
}
```

::: tip
`order.OrderCreated` is a **typed event ref** — a dotted qualified id. The older quoted-string form (`notifies "Order Created"`) still parses but is deprecated; see [Deprecated: Quoted Event Strings](#deprecated-quoted-event-strings) below.
:::

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
when order.OrderPlaced
when auth.UserRegistered
when payment.PaymentProcessed
```

**Syntax:**
```craft
when <event_ref>
```

An `<event_ref>` is a dotted qualified id — the FQ Avro record name / OpenAPI `operationId` the event corresponds to in code. No quotes, no `kind:` prefix, no `/`.

::: tip
Use past tense for event names: `order.OrderPlaced` not `order.PlaceOrder`
:::

### Domain Listener Triggers

Domains reacting to events:

```craft
when Payment listens order.OrderCreated
when Notification listens auth.UserRegistered
when Inventory listens order.OrderCancelled
```

**Syntax:**
```craft
when <domain> listens <event_ref>
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

The target can also be a **node slug** for a cross-context/typed reference: `Subscriptions asks bc:re/billing for a fresh charge attempt`. See [Node Slugs](/language/overview) — a bare domain name like `Inventory` is still the normal short form.

The trailing `<phrase>` accepts special characters unquoted (`! & * / # ? +`), e.g. `Subscriptions asks bc:re/billing for a fresh charge attempt (1! & 2!)` — no need to quote punctuation.

**Use when:** One domain needs an immediate response from another.

### Asynchronous Actions

Publish events:

```craft
Order notifies order.OrderCreated
Payment notifies payment.PaymentProcessed
Profile notifies profile.UserUpdated
```

**Syntax:**
```craft
<domain> notifies <event_ref>
```

**Use when:** Other domains might want to react, but the publisher doesn't need a response.

::: tip
`notifies "Order Created"` (quoted string) still parses but is **deprecated** — `craft validate` emits a `craft/lint/deprecated-string-ref` warning and points at the typed-ref form.
:::

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
Database returns to Authentication the user record
Payment returns confirmation
API returns to Client the error message
Gateway returns to Payment the transaction result
```

**Syntax:**
```craft
<domain> returns [to <domain>] [connector] <phrase>
```

**Use when:** A domain returns data, especially in response to an `asks` action.

## Deprecated: Quoted Event Strings

Older `.craft` files (and older docs) write event names as quoted strings:

```craft
Order notifies "Order Created"
when Payment listens "Order Created"
```

This form still **parses** — `craft validate` won't reject it — but it's **deprecated**: every quoted event string emits a `craft/lint/deprecated-string-ref` warning pointing at the typed-ref replacement. Prefer the typed-ref form (`notifies order.OrderCreated`) in all new or extended files; only leave quoted strings where you're intentionally not migrating an existing file.

## Complete Example

```craft
use_case "Order Processing" {
  // External trigger: customer action
  when Customer places order
    Order validates product availability
    Order asks Inventory to reserve items
    Order calculates total amount
    Order asks Payment to create payment request
    Order notifies order.OrderCreated

  // Domain listener: Payment reacts to Order Created
  when Payment listens order.OrderCreated
    Payment asks PaymentGateway to process transaction
    PaymentGateway returns to Payment the transaction result
    Payment updates payment status
    Payment notifies payment.PaymentProcessed

  // Domain listener: Notification reacts to Payment
  when Notification listens payment.PaymentProcessed
    Notification asks EmailService to send confirmation
    Notification asks SMSService to send notification

  // Domain listener: Inventory reacts to Payment
  when Inventory listens payment.PaymentProcessed
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
    Authentication notifies auth.UserRegistered

  when Profile listens auth.UserRegistered
    Profile asks Database to store profile
    Profile notifies profile.ProfileCreated

  when Notification listens auth.UserRegistered
    Notification asks EmailService to send welcome email

  when Analytics listens auth.UserRegistered
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
    Database returns to Profile the profile data
    Profile returns to API the formatted profile
    API returns to Customer the profile
}
```

## Error Handling Pattern

```craft
use_case "Process Payment" {
  when Customer submits payment
    Payment validates payment details
    Payment asks Gateway to charge card
    Gateway returns to Payment the transaction result

  when Payment listens payment.TransactionFailed
    Payment creates retry attempt
    Payment notifies payment.PaymentFailed

  when Order listens payment.PaymentFailed
    Order cancels order
    Order asks Inventory to release reservation
    Order notifies order.OrderCancelled
}
```

## Best Practices

### Use Past Tense for Events

✅ Good:
```craft
Order notifies order.OrderCreated
Payment notifies payment.PaymentProcessed
```

❌ Bad:
```craft
Order notifies order.CreateOrder
Payment notifies payment.ProcessPayment
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

  when Payment listens order.OrderCreated
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
    Order notifies order.OrderStarted

  when Payment listens order.OrderStarted
    Payment charges customer
    Payment notifies payment.PaymentCompleted

  when Inventory listens payment.PaymentCompleted
    Inventory ships items
    Inventory notifies inventory.ShipmentSent

  when Order listens payment.PaymentFailed
    Order cancels order
    Order notifies order.OrderCancelled
}
```

### CQRS Pattern

```craft
use_case "Order Management" {
  // Command side
  when user creates order
    OrderCommand validates order
    OrderCommand stores order
    OrderCommand notifies order.OrderCreated

  // Query side
  when OrderQuery listens order.OrderCreated
    OrderQuery updates read model
    OrderQuery indexes order data
}
```

## Next Steps

- See [complete examples](/examples/ecommerce) with multiple use cases
- Learn about [services](/language/services) to organize domains
- Understand [domains](/language/domains) to structure your model

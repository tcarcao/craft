# Exposures

Define external access points and API gateways.

## Basic Syntax

```craft
exposure APIName {
  to: Actor1, Actor2
  of: Domain1, Domain2
  through: Gateway1, Gateway2
}
```

## Properties

### to
Who can access this exposure:

```craft
to: Customer, Partner, Admin
```

### of
Which domains are exposed:

```craft
of: Order, Product, Inventory
```

### through
Which gateways are used:

```craft
through: APIGateway, LoadBalancer
```

## Example

```craft
exposure PublicAPI {
  to: Customer
  of: Catalog, Order, Payment
  through: APIGateway, LoadBalancer
}

exposure PartnerAPI {
  to: Partner
  of: Inventory, Order
  through: PartnerGateway
}

exposure AdminAPI {
  to: Admin
  of: Catalog, Order, Payment, Inventory, Analytics
  through: AdminGateway
}
```

## Use Case

Exposures help document:
- Who has access to what
- Which domains are public vs internal
- Gateway routing configuration

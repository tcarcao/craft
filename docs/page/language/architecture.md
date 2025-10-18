# Architecture

Define system architecture with component flows and modifiers.

## Basic Syntax

```craft
arch SystemName {
  presentation:
    Component1 > Component2

  gateway:
    Component3 > Component4
}
```

## Component Flows

Chain components with `>`:

```craft
arch WebApp {
  presentation:
    Browser > CDN > LoadBalancer

  gateway:
    LoadBalancer > APIGateway > Backend
}
```

## Component Modifiers

Add attributes in brackets:

```craft
arch API {
  gateway:
    LoadBalancer[replicas: 3, ssl: true]
    APIGateway[port: 8080, timeout: 30s]
    Database[type: postgres, replicas: 2]
}
```

## Complete Example

```craft
arch EcommerceSystem {
  presentation:
    WebApp > CDN[provider: cloudflare]
    MobileApp > LoadBalancer[ssl: true]

  gateway:
    LoadBalancer > APIGateway[framework: express, port: 8080]
    APIGateway > CatalogService[replicas: 3]
    APIGateway > OrderService[replicas: 5]
    OrderService > Database[type: postgres, replicas: 2]
    OrderService > Cache[type: redis, ttl: 3600]
}
```

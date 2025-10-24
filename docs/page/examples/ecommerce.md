# E-commerce System

A complete e-commerce system with order processing, payment handling, and inventory management.

## Complete Code

```craft
// Define system actors
actors {
  user Customer
  user Admin
  system PaymentGateway
  system ShippingProvider
  system EmailService
}

// Define business domains
domains {
  Catalog {
    ProductManagement
    CategoryManagement
    Search
  }

  Order {
    OrderManagement
    OrderTracking
    ShoppingCart
  }

  Payment {
    PaymentProcessing
    PaymentValidation
    RefundManagement
  }

  Inventory {
    StockManagement
    Warehouse
    Replenishment
  }

  Shipping {
    ShippingCalculation
    LabelGeneration
    TrackingIntegration
  }

  Notification {
    EmailNotification
    OrderNotification
  }
}

// Define microservices
services {
  CatalogService {
    domains: Catalog
    language: python
    data-stores: product_db, search_index
    deployment: rolling
  }

  OrderService {
    domains: Order, Payment
    language: nodejs
    data-stores: order_db, payment_db
    deployment: canary(50% -> staging, 100% -> production)
  }

  InventoryService {
    domains: Inventory
    language: java
    data-stores: inventory_db
    deployment: blue_green
  }

  ShippingService {
    domains: Shipping
    language: golang
    data-stores: shipping_db
    deployment: rolling
  }

  NotificationService {
    domains: Notification
    language: nodejs
    data-stores: notification_queue
    deployment: rolling
  }
}

// Use case: Browse and search products
use_case "Product Browsing" {
  when Customer searches for products
    Search processes search query
    Search asks ProductManagement for matching products
    ProductManagement returns to Search the product list
    Search returns to Customer the ranked results
}

// Use case: Complete order flow
use_case "Order Placement" {
  when Customer adds items to cart
    ShoppingCart validates product availability
    ShoppingCart calculates subtotal

  when Customer proceeds to checkout
    OrderManagement validates cart items
    OrderManagement asks StockManagement to check availability
    StockManagement returns to OrderManagement the availability status 
    OrderManagement asks ShippingCalculation for shipping cost
    ShippingCalculation returns to OrderManagement the shipping cost 
    OrderManagement calculates total amount
    OrderManagement creates order

  when Customer submits payment
    PaymentProcessing validates payment details
    PaymentProcessing asks PaymentGateway to process transaction
    PaymentGateway returns to PaymentProcessing the transaction result
    PaymentProcessing updates payment status
    PaymentProcessing notifies "Payment Processed"

  when OrderManagement listens "Payment Processed"
    OrderManagement confirms order
    OrderManagement asks StockManagement to reserve items
    OrderManagement notifies "Order Confirmed"

  when StockManagement listens "Order Confirmed"
    StockManagement updates inventory levels
    StockManagement notifies "Inventory Updated"

  when ShippingService listens "Order Confirmed"
    LabelGeneration creates shipping label
    LabelGeneration asks ShippingProvider to register shipment
    ShippingProvider returns to LabelGeneration the tracking number
    LabelGeneration notifies "Shipment Ready"

  when EmailNotification listens "Order Confirmed"
    EmailNotification prepares order confirmation
    EmailNotification asks EmailService to send email
}

// Use case: Order cancellation
use_case "Order Cancellation" {
  when Customer cancels order
    OrderManagement validates cancellation eligibility
    OrderManagement updates order status
    OrderManagement notifies "Order Cancelled"

  when PaymentProcessing listens "Order Cancelled"
    PaymentProcessing initiates refund
    PaymentProcessing asks PaymentGateway to refund transaction
    PaymentGateway returns to PaymentProcessing the refund confirmation
    PaymentProcessing notifies "Refund Processed"

  when StockManagement listens "Order Cancelled"
    StockManagement releases reserved inventory
    StockManagement updates stock levels

  when EmailNotification listens "Refund Processed"
    EmailNotification asks EmailService to send refund confirmation
}

// Use case: Inventory replenishment
use_case "Stock Replenishment" {
  when CRON runs daily inventory check
    Replenishment analyzes stock levels
    Replenishment identifies low stock items
    Replenishment notifies "Low Stock Alert"

  when Admin approves replenishment
    Replenishment creates purchase order
    Replenishment notifies "Replenishment Ordered"

  when Warehouse receives stock
    Warehouse updates inventory levels
    Warehouse notifies "Stock Updated"

  when ProductManagement listens "Stock Updated"
    ProductManagement marks products as available
}

// Architecture definition
arch EcommerceSystem {
  presentation:
    WebApp > CDN
    MobileApp > LoadBalancer

  gateway:
    LoadBalancer > APIGateway[ssl: true, rate_limit: 1000]
    APIGateway > CatalogService
    APIGateway > OrderService
    APIGateway > InventoryService
    APIGateway > ShippingService
}

// External API exposure
exposure PublicAPI {
  to: Customer
  of: Catalog, Order, Payment
  through: APIGateway, LoadBalancer
}

exposure AdminAPI {
  to: Admin
  of: Catalog, Inventory, Order
  through: APIGateway
}
```

## Key Features Demonstrated

### 1. Event-Driven Architecture
- Order placement triggers multiple domain reactions
- Async communication via events (e.g., "Order Confirmed")
- Decoupled domains listening to events

### 2. Microservices Boundaries
- Clear service boundaries by domain
- Different technology stacks per service
- Different deployment strategies

### 3. Domain-Driven Design
- Bounded contexts (Catalog, Order, Payment, etc.)
- Domain interactions through events
- Core domains vs supporting domains

### 4. Synchronous Communication
- `asks` pattern for request-response
- `returns` pattern for explicit responses
- Clear data flow

### 5. CRON Scheduling
- Automated inventory checks
- Scheduled background tasks

## Deployment Strategy

The example shows three deployment strategies:

- **Rolling**: Gradual instance updates (Catalog, Shipping, Notification)
- **Canary**: Percentage-based rollout (Order - 50% staging, then 100% production)
- **Blue-Green**: Parallel environment switching (Inventory)

## Generated Diagrams

When you preview this file (`Ctrl+Shift+C`), you'll see:

- **C4 Diagram**: Shows all services and their relationships
- **Domain Diagram**: Shows domain interactions and events
- **Sequence Diagram**: Shows the order flow step-by-step

## Extending This Example

Try adding:

1. **Wishlist feature**
   ```craft
   domain Wishlist {
     WishlistManagement
   }
   ```

2. **Product reviews**
   ```craft
   use_case "Submit Review" {
     when Customer submits review
       ReviewManagement validates review
       ReviewManagement notifies "Review Submitted"
   }
   ```

3. **Promotions and discounts**
   ```craft
   domain Promotion {
     DiscountManagement
     CouponValidation
   }
   ```

## Related Examples

- [Banking System](/examples/banking) - More complex event flows
- [User Management](/examples/user-management) - Authentication patterns

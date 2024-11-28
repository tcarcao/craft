// Main e-commerce architecture
system ECommerceSystem {
    // Core order management
    bounded context Orders {
        // Domain model
        aggregate Order
        aggregate Cart
        
        // Core services and components
        component OrderValidator using go
        component CartManager using go
        service OrderProcessor using go on eks
        service CartService using go on eks
        service OrderStore using go on dynamodb
        service OrderQueue using go on sqs
        
        // Domain events
        event OrderCreated
        event OrderValidated
        event OrderShipped

        // Relationships with other contexts
        upstream to Payments as ohs
        upstream to Shipping as acl
    }

    // Inventory management
    bounded context Inventory {
        aggregate Stock
        aggregate Warehouse

        component StockValidator using java
        component InventoryManager using java
        service StockService using java on eks
        service WarehouseService using java on eks
        service StockStore using java on dynamodb

        event StockUpdated
        event StockAllocated

        downstream to Orders as conformist
    }
}

// Payment handling system
system PaymentSystem {
    bounded context Payments {
        aggregate Payment
        aggregate Refund

        component PaymentProcessor using java
        component RefundHandler using java
        service PaymentService using java on eks
        service PaymentStore using java on dynamodb
        service PaymentQueue using java on sqs

        event PaymentProcessed
        event RefundInitiated

        downstream to Orders as conformist
    }
}

// Shipping and logistics
system LogisticsSystem {
    bounded context Shipping {
        aggregate Shipment
        aggregate DeliveryRoute

        component RouteOptimizer using go
        component ShipmentTracker using go
        service ShippingService using go on eks
        service TrackingService using go on eks
        service LocationStore using go on dynamodb

        event ShipmentCreated
        event ShipmentDelivered

        downstream to Orders as conformist
    }
}

// Service interactions
Orders.ProcessOrder(Order) -> Payments.ProcessPayment()
Payments.PaymentService(PaymentProcessed) -> Orders.OrderQueue()
Orders.OrderProcessor(OrderValidated) -> Shipping.CreateShipment()
Orders.CartManager(Cart) -> Inventory.CheckStock()
Shipping.ShipmentTracker(ShipmentCreated) -> Orders.OrderQueue()

system OrderSystem {
   bounded context Orders {
       aggregate Order
       component OrderProcessor
       service OrderService
       event OrderCreated

       upstream to Payments as ohs
   }
}

system PaymentSystem {
   bounded context Payments {
       aggregate Payment
       component PaymentProcessor
       service PaymentService
       event PaymentProcessed
   }
}

Orders.ProcessOrder(Customer) -> Payments.CreatePayment()
Payments.PublishEvent("PaymentProcessed")
Orders.ConsumeEvent("PaymentProcessed")
system OrderSystem {
   bounded context Orders {
       aggregate Order
       component OrderProcessor
       service OrderService
   }
}

system PaymentSystem {
   bounded context Payments {
       aggregate Payment
       component PaymentProcessor
       service PaymentService
   }
}
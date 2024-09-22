system OrderSystem {
   bounded context Orders {
       aggregate Order
       component OrderProcessor using go
       service OrderService using go on eks
       event OrderCreated

       upstream to Payments as ohs
   }
}

system PaymentSystem {
   bounded context Payments {
       aggregate Payment
       component PaymentProcessor using java
       service PaymentService using java on lambda
       event PaymentProcessed
   }
}

Orders.ProcessOrder(Customer) -> Payments.CreatePayment()
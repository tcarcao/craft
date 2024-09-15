system OrderSystem {
   bounded context Orders {
       aggregate Order
       component OrderProcessor
       service OrderService
   }
}
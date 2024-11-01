// Microservices platform example
system PlatformSystem {
    bounded context ApiGateway {
        component RequestRouter using go
        service GatewayService using go on eks
        service AuthService using go on eks
        service RateLimiter using go on redis
        
        event RequestReceived
        event RequestRouted

        upstream to UserManagement as acl
        upstream to OrderProcessing as acl
    }

    bounded context UserManagement {
        aggregate User
        aggregate Profile
        
        component UserValidator using java
        service UserService using java on eks
        service ProfileService using java on eks
        service UserStore using java on dynamodb
        
        event UserCreated
        event ProfileUpdated

        downstream to ApiGateway as conformist
    }

    bounded context OrderProcessing {
        aggregate Order
        aggregate Payment
        
        component OrderProcessor using python
        service OrderService using python on lambda
        service PaymentService using python on lambda
        service OrderQueue using python on sqs
        
        event OrderCreated
        event PaymentProcessed

        downstream to ApiGateway as conformist
        upstream to NotificationService as ohs
    }

    bounded context NotificationService {
        component NotificationDispatcher using nodejs
        service EmailService using nodejs on lambda
        service SMSService using nodejs on lambda
        service PushService using nodejs on sns
        
        event NotificationSent

        downstream to OrderProcessing as conformist
    }
}

// Platform interactions
ApiGateway.RouteRequest(UserId) -> UserManagement.ValidateUser()
ApiGateway.RouteRequest(OrderRequest) -> OrderProcessing.CreateOrder()
OrderProcessing.ProcessOrder(Order) -> NotificationService.SendConfirmation()
UserManagement.CreateUser(User) -> NotificationService.SendWelcome()
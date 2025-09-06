arch {
    presentation: 
        WebApp[framework:react, ssl]
        MobileApp
        
    gateway:
        LoadBalancer[ssl:true] > APIGateway[type:nginx]
}

exposure default {
    to: Business_User
    through: APIGateway
}

services {
    UserService: {
        domains: Authentication, Profile
        data-stores: user_db
        language: golang
    },
    CommsService: {
        domains: Notifier
    }
}

use_case "User Registration" {
    when Business_User creates Account
        Authentication validates email format
        Authentication asks Database to check email uniqueness
        Profile creates user profile
        Authentication notifies "User Registered"

    when Profile listens "User Registered"
        Profile asks Database to store profile data
        Profile asks Notifier to send welcome email
}
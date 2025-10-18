# User Management

Authentication and profile management system.

```craft
actors {
  user Customer
  user Admin
  system EmailService
  system SMSService
}

domains {
  Authentication {
    Login
    Registration
    PasswordReset
    TwoFactorAuth
  }

  Profile {
    UserProfile
    Settings
    Preferences
  }

  Authorization {
    RoleManagement
    PermissionManagement
  }
}

services {
  AuthService {
    domains: Authentication, Authorization
    language: nodejs
    data-stores: auth_db, token_cache
    deployment: rolling
  }

  ProfileService {
    domains: Profile
    language: python
    data-stores: profile_db
    deployment: rolling
  }
}

use_case "User Registration" {
  when Customer submits registration
    Registration validates email format
    Registration validates password strength
    Registration asks Database to check email uniqueness
    Database returns uniqueness result to Registration
    Registration creates user account
    Registration notifies "User Registered"

  when Profile listens "User Registered"
    UserProfile creates default profile
    UserProfile notifies "Profile Created"

  when Authentication listens "User Registered"
    Authentication generates verification token
    Authentication asks EmailService to send verification email
}

use_case "User Login" {
  when Customer submits credentials
    Login validates credentials format
    Login asks Database to verify credentials
    Database returns user data to Login
    Login generates session token
    Login notifies "Login Successful"

  when TwoFactorAuth listens "Login Successful"
    TwoFactorAuth generates OTP code
    TwoFactorAuth asks SMSService to send code

  when Customer submits OTP
    TwoFactorAuth validates code
    TwoFactorAuth updates session status
    TwoFactorAuth notifies "2FA Verified"
}

use_case "Password Reset" {
  when Customer requests password reset
    PasswordReset validates email exists
    PasswordReset generates reset token
    PasswordReset asks EmailService to send reset link
    PasswordReset notifies "Reset Token Sent"

  when Customer submits new password
    PasswordReset validates reset token
    PasswordReset validates password strength
    PasswordReset updates password hash
    PasswordReset invalidates reset token
    PasswordReset notifies "Password Changed"

  when Authentication listens "Password Changed"
    Authentication invalidates all user sessions
    Authentication asks EmailService to send confirmation
}
```

## Key Patterns

- **Multi-step authentication (2FA)**
- **Token-based password reset**
- **Event-driven profile creation**
- **Session management**

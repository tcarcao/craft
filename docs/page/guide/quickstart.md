# Quick Start

Let's build your first Craft architecture in 5 minutes! We'll model a simple user registration system.

## Step 1: Create Your First File

Create a new file called `user-registration.craft`.

## Step 2: Define Actors

Start by defining who interacts with your system:

```craft
actors {
  user Customer
  system EmailService
}
```

**What this means:**
- `user Customer` - A human user named "Customer"
- `system EmailService` - An external system for sending emails

## Step 3: Define Domains

Define your business domains:

```craft
domains {
  Authentication {
    Registration
    Login
  }
  Profile {
    UserProfile
    Settings
  }
}
```

**What this means:**
- `Authentication` domain with `Registration` and `Login` subdomains
- `Profile` domain with `UserProfile` and `Settings` subdomains

## Step 4: Define Services

Group domains into deployable services:

```craft
services {
  UserService {
    domains: Authentication, Profile
    language: nodejs
    data-stores: user_db
  }
}
```

**What this means:**
- `UserService` handles both Authentication and Profile domains
- Built with Node.js
- Uses a database called `user_db`

## Step 5: Model a Use Case

Now the interesting part - model the registration flow:

```craft
use_case "User Registration" {
  when Customer submits registration
    Authentication validates email format
    Authentication asks Database to check email uniqueness
    Profile creates user profile
    Authentication notifies "User Registered"

  when Profile listens "User Registered"
    Profile asks EmailService to send welcome email
}
```

**What this means:**
1. When a customer submits registration:
   - Authentication validates the email format
   - Authentication checks with the database if email is unique
   - Profile creates a user profile
   - Authentication publishes a "User Registered" event

2. When Profile hears the "User Registered" event:
   - Profile asks EmailService to send a welcome email

## Complete Example

Here's your complete first Craft file:

```craft
// Define actors
actors {
  user Customer
  system EmailService
}

// Define domains
domains {
  Authentication {
    Registration
    Login
  }
  Profile {
    UserProfile
    Settings
  }
}

// Define services
services {
  UserService {
    domains: Authentication, Profile
    language: nodejs
    data-stores: user_db
  }
}

// Model the registration use case
use_case "User Registration" {
  when Customer submits registration
    Authentication validates email format
    Authentication asks Database to check email uniqueness
    Profile creates user profile
    Authentication notifies "User Registered"

  when Profile listens "User Registered"
    Profile asks EmailService to send welcome email
}
```

## Step 6: Preview Your Architecture

Press `Ctrl+Shift+C` (or `Cmd+Shift+C` on Mac) to generate a C4 diagram!

::: tip
Make sure you have the [diagram server running](/guide/installation#install-diagram-server-optional) to preview diagrams.
:::

## Understanding the Output

Your diagram will show:
- Services and their boundaries
- Domain interactions
- Event flows
- External system dependencies

## Key Concepts

### Triggers
Use cases start with triggers:
- `when Customer does something` - External trigger from an actor
- `when "Event Name"` - Event trigger
- `when Domain listens "Event"` - Domain listening to an event

### Actions
Four types of actions:
1. **Sync** - `Domain asks Domain to do something`
2. **Async** - `Domain notifies "Event Name"`
3. **Internal** - `Domain does something`
4. **Return** - `Domain returns result to Domain`

## Next Steps

Now that you've created your first Craft file:

- Explore [Language Reference](/language/overview) for complete syntax
- Check out [more examples](/examples/ecommerce) for complex scenarios
- Learn about [all extension features](/extension/features)
- Understand [use case modeling](/language/use-cases) in depth

## Exercise

Try adding a password reset use case:

<details>
<summary>Show solution</summary>

```craft
use_case "Password Reset" {
  when Customer requests password reset
    Authentication validates email exists
    Authentication generates reset token
    Authentication asks EmailService to send reset link
    Authentication notifies "Reset Token Generated"

  when Customer submits new password
    Authentication validates reset token
    Authentication updates password
    Authentication notifies "Password Changed"
}
```

</details>

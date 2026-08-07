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
- `Authentication` domain with `Registration` and `Login` bounded contexts
- `Profile` domain with `UserProfile` and `Settings` bounded contexts

## Step 4: Define Services

Group domains into deployable services:

```craft
services {
  UserService {
    contexts: Authentication, Profile
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
    Authentication notifies auth.UserRegistered

  when Profile listens auth.UserRegistered
    Profile asks EmailService to send welcome email
}
```

**What this means:**
1. When a customer submits registration:
   - Authentication validates the email format
   - Authentication checks with the database if email is unique
   - Profile creates a user profile
   - Authentication publishes an `auth.UserRegistered` event

2. When Profile hears the `auth.UserRegistered` event:
   - Profile asks EmailService to send a welcome email

::: tip
`auth.UserRegistered` is a **typed event ref** — a dotted qualified id, not a quoted string. The old quoted form (`notifies "User Registered"`) still parses but is deprecated; typed refs are the recommended syntax.
:::

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
    contexts: Authentication, Profile
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
    Authentication notifies auth.UserRegistered

  when Profile listens auth.UserRegistered
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
- `when module.EventName` - Event trigger (typed event ref, a dotted qualified id)
- `when Domain listens module.EventName` - Domain listening to an event

### Actions
Four types of actions:
1. **Sync** - `Domain asks Domain to do something` (target can also be domain-qualified, like `re/billing`; a `kind:` prefix such as `bc:re/billing` is not accepted)
2. **Async** - `Domain notifies module.EventName`
3. **Internal** - `Domain does something`
4. **Return** - `Domain returns to Domain the result`

Any action can end with a bracketed operation annotation describing the wire call it makes, e.g. `Domain asks Domain to do something [POST /v1/charges]`. See [Operation Annotations](/language/use-cases#operation-annotations) for the full syntax.

::: tip
The old quoted-string form (`when "Event Name"`, `notifies "Event Name"`) still parses but is deprecated in favor of typed event refs.
:::

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
    Authentication notifies auth.ResetTokenGenerated

  when Customer submits new password
    Authentication validates reset token
    Authentication updates password
    Authentication notifies auth.PasswordChanged
}
```

</details>

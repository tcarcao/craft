import { Parser } from './parser/CraftParser.js';

describe('extractArchitecturalBlocks', () => {
  let parser: Parser;

  beforeEach(() => {
    parser = new Parser();
  });

  const userManagementDSL = `actors {
    user Business_User
    system CronA
    service Database
}

actor user Customer_Support

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

domain User {
    Authentication
    Profile
}

services {
    UserService {
        domains: Authentication, Profile
        data-stores: user_db
        language: golang
    }
    CommsService {
        domains: Notifier
    }
}

// this is a comment
use_case "User Registration" {
    when Business_User creates Account
        Authentication validates email format
        Authentication returns hello
        Authentication asks Database to check email uniqueness
        Profile creates user profile
        Authentication notifies "User Registered"

    when Profile listens "User Registered"
        Profile asks Database to store profile data
        Profile asks Notifier to send welcome email
}`;

  const ecommerceDSL = `services {
  WalletService {
    domains: Wallet, WalletItemPurchase, WalletItemAddition,
    data-stores: posgres, redis
    language: golang
  }
  "Order Service" {
    domains: OrderManagement, OrderFulfilment
    data-stores: posgres
  }
}

use_case "Purchase VAS to Wallet" {
  when Business_User purchases VAS to wallet
    WalletItemPurchase asks Wallet to initiate a VAS addition
    Wallet notifies "VAS Addition Request Approved"
}`;

  test('should extract only architectural blocks from user management DSL', () => {
    const result = parser.extractArchitecturalBlocks(userManagementDSL);
    
    // Should contain actors, arch, exposure, and domain blocks
    expect(result).toContain('actors {');
    expect(result).toContain('user Business_User');
    expect(result).toContain('actor user Customer_Support');
    expect(result).toContain('arch {');
    expect(result).toContain('presentation:');
    expect(result).toContain('exposure default {');
    expect(result).toContain('to: Business_User');
    expect(result).toContain('domain User {');
    expect(result).toContain('Authentication');
    expect(result).toContain('Profile');
    
    // Should NOT contain services or use cases
    expect(result).not.toContain('UserService');
    expect(result).not.toContain('CommsService');
    expect(result).not.toContain('use_case "User Registration"');
  });

  test('should return empty string for DSL with no architectural blocks', () => {
    const result = parser.extractArchitecturalBlocks(ecommerceDSL);
    
    // E-commerce DSL has no actors, arch, or exposure blocks
    expect(result.trim()).toBe('');
  });

  test('should handle DSL with only partial architectural blocks', () => {
    const partialDSL = `actors {
    user TestUser
}

services {
    TestService {
        domains: Test
    }
}`;

    const result = parser.extractArchitecturalBlocks(partialDSL);
    
    // Should contain actors but not services
    expect(result).toContain('actors {');
    expect(result).toContain('user TestUser');
    expect(result).not.toContain('TestService');
    expect(result).not.toContain('services {');
  });

  test('should extract individual actor definitions', () => {
    const actorOnlyDSL = `actor user TestUser
actor system TestSystem

services {
    TestService {
        domains: Test
    }
}`;

    const result = parser.extractArchitecturalBlocks(actorOnlyDSL);
    
    // Should contain individual actor definitions
    expect(result).toContain('actor user TestUser');
    expect(result).toContain('actor system TestSystem');
    
    // Should NOT contain services
    expect(result).not.toContain('TestService');
    expect(result).not.toContain('services {');
  });

  test('should handle DSL with only arch block', () => {
    const archOnlyDSL = `arch production {
    presentation:
        Frontend
    gateway:
        Gateway
}

services {
    TestService {
        domains: Test
    }
}`;

    const result = parser.extractArchitecturalBlocks(archOnlyDSL);
    
    // Should contain arch block
    expect(result).toContain('arch production {');
    expect(result).toContain('presentation:');
    expect(result).toContain('Frontend');
    
    // Should NOT contain services
    expect(result).not.toContain('TestService');
  });

  test('should handle DSL with only exposure block', () => {
    const exposureOnlyDSL = `exposure api {
    to: ExternalUser
    through: Gateway
}

services {
    TestService {
        domains: Test
    }
}`;

    const result = parser.extractArchitecturalBlocks(exposureOnlyDSL);
    
    // Should contain exposure block
    expect(result).toContain('exposure api {');
    expect(result).toContain('to: ExternalUser');
    expect(result).toContain('through: Gateway');
    
    // Should NOT contain services
    expect(result).not.toContain('TestService');
  });

  test('should handle empty DSL', () => {
    const result = parser.extractArchitecturalBlocks('');
    expect(result.trim()).toBe('');
  });

  test('should handle DSL with only comments', () => {
    const commentOnlyDSL = `// This is a comment
// Another comment`;

    const result = parser.extractArchitecturalBlocks(commentOnlyDSL);
    expect(result.trim()).toBe('');
  });
});
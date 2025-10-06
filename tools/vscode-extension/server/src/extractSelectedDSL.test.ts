import { Parser } from './parser/CraftParser.js';
import { BlockRange } from '../../shared/lib/types/domain-extraction.js';

describe('extractSelectedDSL', () => {
  let parser: Parser;

  beforeEach(() => {
    parser = new Parser();
  });

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
  service-re-go-vas { 
    domains: VASScheduling
    data-stores: posgres, dynamodb
  }
}

use_case "Purchase VAS to Wallet" {
  when Business_User purchases VAS to wallet
    WalletItemPurchase asks Wallet to initiate a VAS addition
    Wallet notifies "VAS Addition Request Approved"

  when WalletItemPurchase listens "VAS Addition Request Approved"
    WalletItemPurchase asks OrderManagement to create an Order
    OrderManagement notifies "Order Created"
    OrderManagement marks the Order as payment deferred
    OrderManagement notifies "Order payment deferred"

  when OrderFulfilment listens "Order payment deferred"
    OrderFulfilment asks WalletItemAddition to fulfil the addition request
    WalletItemAddition asks Wallet to add VAS
    Wallet notifies "VAS added"
      
  when OrderFulfilment listens "VAS added"
    OrderFulfilment asks OrderManagement to mark the Order as fulfiled
    OrderManagement notifies "Order completed"
}

use_case "VAS expires" {
  when CRON identifies a VAS expiring
    Wallet removes VAS
    Wallet notifies "VAS Expired"
}`;

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
}

use_case "C2C user purchases" {
    when Customer decides to purchase a VAS
        Offering  returns available offering

    when Customer picks what to purchase
        Customer asks Order to create an order
        Order creates a draft Order
        Order returns the url of the payment provider

    when  Customer pays of the order
        Customer asks PaymentProvider to pay for the order
        PaymentProvider notifies "PaymentCompleted"
    
    when  Order listens  "PaymentCompleted"
        Order marks the Order as Paid
}`;

  test('should extract only WalletService (lines 2-6)', () => {
    const ranges: BlockRange[] = [{
      startLine: 2,
      endLine: 6,
      fileUri: 'e-commerce.craft'
    }];

    const result = parser.extractSelectedDSL(ecommerceDSL, ranges);
    
    // Expected behavior: should contain WalletService but not the other services
    expect(result).toContain('WalletService');
    expect(result).not.toContain('Order Service');
    expect(result).not.toContain('service-re-go-vas');
  });

  test('should extract only Order Service (lines 7-10)', () => {
    const ranges: BlockRange[] = [{
      startLine: 7,
      endLine: 10,
      fileUri: 'e-commerce.craft'
    }];

    const result = parser.extractSelectedDSL(ecommerceDSL, ranges);
    
    // Expected behavior: should contain Order Service but not the others
    expect(result).not.toContain('WalletService');
    expect(result).toContain('Order Service');
    expect(result).not.toContain('service-re-go-vas');
  });

  test('should handle exact line boundaries (lines 3-4)', () => {
    const ranges: BlockRange[] = [{
      startLine: 3,
      endLine: 4,
      fileUri: 'e-commerce.craft'
    }];

    const result = parser.extractSelectedDSL(ecommerceDSL, ranges);
    
    // The result should not be empty and should be focused on the selected content
    expect(result.trim()).not.toBe('');
  });

  test('should extract VAS expires use case (lines 37-41)', () => {
    const ranges: BlockRange[] = [{
      startLine: 37,
      endLine: 41,
      fileUri: 'e-commerce.craft'
    }];

    const result = parser.extractSelectedDSL(ecommerceDSL, ranges);
    
    // Should contain "VAS expires" use case but not "Purchase VAS to Wallet"
    expect(result).toContain('VAS expires');
    expect(result).not.toContain('Purchase VAS to Wallet');
  });

  // Tests for user management DSL components
  describe('User Management DSL Components', () => {
    test('should extract actors block with architectural context (lines 1-5)', () => {
      const ranges: BlockRange[] = [{
        startLine: 1,
        endLine: 5,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      // Should contain the selected actors block
      expect(result).toContain('actors {');
      expect(result).toContain('user Business_User');
      expect(result).toContain('system CronA');
      expect(result).toContain('service Database');
      
      // Should also contain architectural context (arch, exposure) as they're always included
      expect(result).toContain('arch {');
      expect(result).toContain('exposure default {');
      
      // Should NOT contain services or use cases as they weren't selected
      expect(result).not.toContain('UserService');
      expect(result).not.toContain('use_case "User Registration"');
    });

    test('should extract single actor definition with architectural context (line 7)', () => {
      const ranges: BlockRange[] = [{
        startLine: 7,
        endLine: 7,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      // Should contain the selected actor
      expect(result).toContain('Customer_Support');
      
      // Should also contain architectural context (actors block, arch, exposure) as they're always included
      expect(result).toContain('actors {');
      expect(result).toContain('arch {');
      expect(result).toContain('exposure default {');
      
      // Should NOT contain services or use cases as they weren't selected
      expect(result).not.toContain('UserService');
      expect(result).not.toContain('use_case "User Registration"');
    });

    test('should extract arch block with full architectural context (lines 9-16)', () => {
      const ranges: BlockRange[] = [{
        startLine: 9,
        endLine: 16,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      // Should contain the selected arch block
      expect(result).toContain('arch {');
      expect(result).toContain('presentation:');
      expect(result).toContain('WebApp[framework:react, ssl]');
      expect(result).toContain('gateway:');
      expect(result).toContain('LoadBalancer[ssl:true] > APIGateway[type:nginx]');
      
      // Should also contain other architectural context (actors, exposure) as they're always included
      expect(result).toContain('actors {');
      expect(result).toContain('exposure default {');
      
      // Should NOT contain services or use cases as they weren't selected  
      expect(result).not.toContain('UserService');
      expect(result).not.toContain('use_case "User Registration"');
    });

    test('should extract exposure block with full architectural context (lines 18-21)', () => {
      const ranges: BlockRange[] = [{
        startLine: 18,
        endLine: 21,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      // Should contain the selected exposure block
      expect(result).toContain('exposure default {');
      expect(result).toContain('to: Business_User');
      expect(result).toContain('through: APIGateway');
      
      // Should also contain other architectural context (actors, arch) as they're always included
      expect(result).toContain('actors {');
      expect(result).toContain('arch {');
      
      // Should NOT contain services or use cases as they weren't selected
      expect(result).not.toContain('UserService');
      expect(result).not.toContain('use_case "User Registration"');
    });

    test('should extract domain definition with architectural context (lines 23-26)', () => {
      const ranges: BlockRange[] = [{
        startLine: 23,
        endLine: 26,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      // Should contain the selected domain
      expect(result).toContain('domain User {');
      expect(result).toContain('Authentication');
      expect(result).toContain('Profile');
      
      // Should also contain architectural context (actors, arch, exposure) as they're always included
      expect(result).toContain('actors {');
      expect(result).toContain('arch {');
      expect(result).toContain('exposure default {');
      
      // Should NOT contain services or use cases as they weren't selected
      expect(result).not.toContain('UserService');
      expect(result).not.toContain('use_case "User Registration"');
    });

    test('should extract specific service with architectural context (lines 29-33)', () => {
      const ranges: BlockRange[] = [{
        startLine: 29,
        endLine: 33,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      // Should contain the selected service
      expect(result).toContain('UserService {');
      expect(result).toContain('domains: Authentication, Profile');
      expect(result).toContain('data-stores: user_db');
      expect(result).toContain('language: golang');
      
      // Should NOT contain other services
      expect(result).not.toContain('CommsService');
      
      // Should contain architectural context (actors, arch, exposure) as they're always included
      expect(result).toContain('actors {');
      expect(result).toContain('arch {');
      expect(result).toContain('exposure default {');
      
      // Should NOT contain use cases as they weren't selected
      expect(result).not.toContain('use_case "User Registration"');
    });

    test('should extract User Registration use case (lines 40-51)', () => {
      const ranges: BlockRange[] = [{
        startLine: 40,
        endLine: 51,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      expect(result).toContain('use_case "User Registration" {');
      expect(result).toContain('when Business_User creates Account');
      expect(result).toContain('Authentication validates email format');
      expect(result).toContain('when Profile listens "User Registered"');
      expect(result).not.toContain('C2C user purchases');
    });

    test('should extract specific scenario from use case (lines 48-51)', () => {
      const ranges: BlockRange[] = [{
        startLine: 48,
        endLine: 51,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      expect(result).toContain('when Profile listens "User Registered"');
      expect(result).toContain('Profile asks Database to store profile data');
      expect(result).toContain('Profile asks Notifier to send welcome email');
      expect(result).not.toContain('when Business_User creates Account');
    });
  });

  // Tests for mixed selections and edge cases
  describe('Mixed Selections and Edge Cases', () => {
    test('should handle comment lines (line 39 in user management)', () => {
      const ranges: BlockRange[] = [{
        startLine: 39,
        endLine: 39,
        fileUri: 'user-management.craft'
      }];

      const result = parser.extractSelectedDSL(userManagementDSL, ranges);
      
      // Should extract the comment or handle it appropriately
      expect(result.trim()).not.toBe('');
    });

    test('should extract multiple non-contiguous services', () => {
      const ranges: BlockRange[] = [
        {
          startLine: 2,
          endLine: 6,
          fileUri: 'e-commerce.craft'
        },
        {
          startLine: 11,
          endLine: 14,
          fileUri: 'e-commerce.craft'
        }
      ];

      const result = parser.extractSelectedDSL(ecommerceDSL, ranges);
      
      expect(result).toContain('WalletService');
      expect(result).toContain('service-re-go-vas');
      expect(result).not.toContain('Order Service');
    });

    test('should handle single line selections within services', () => {
      const ranges: BlockRange[] = [{
        startLine: 4,
        endLine: 4,
        fileUri: 'e-commerce.craft'
      }];

      const result = parser.extractSelectedDSL(ecommerceDSL, ranges);
      
      expect(result).toContain('data-stores: posgres, redis');
      // Should still include the service context
      expect(result).toContain('WalletService');
    });

    test('should extract partial use case scenario based on selected nodes', () => {
      // Based on debug output, the parser is selecting nodes that correspond to OrderFulfilment scenario
      const ranges: BlockRange[] = [{
        startLine: 28,
        endLine: 30,
        fileUri: 'e-commerce.craft'
      }];

      const result = parser.extractSelectedDSL(ecommerceDSL, ranges);
      
      // The parser is actually selecting the OrderFulfilment scenario based on the debug output
      // This is the correct behavior - it finds the scenario that contains the selected lines
      expect(result).toContain('when OrderFulfilment listens "Order payment deferred"');
      expect(result).toContain('OrderFulfilment asks WalletItemAddition to fulfil the addition request');
      expect(result).toContain('WalletItemAddition asks Wallet to add VAS');
      
      // Should include the use case context
      expect(result).toContain('use_case "Purchase VAS to Wallet"');
    });
  });
});
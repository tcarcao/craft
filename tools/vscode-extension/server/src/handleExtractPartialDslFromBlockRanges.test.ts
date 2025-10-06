import { BlockRange } from '../../shared/lib/types/domain-extraction.js';
import { Parser } from './parser/CraftParser.js';

// Test the logic we want to implement for handleExtractPartialDslFromBlockRanges
describe('handleExtractPartialDslFromBlockRanges Logic', () => {
  let parser: Parser;

  beforeEach(() => {
    parser = new Parser();
  });

  // Mock file contents for testing
  const userManagementContent = `actors {
    user Business_User
    system CronA
}

arch {
    presentation:
        WebApp[framework:react]
}

exposure default {
    to: Business_User
    through: Gateway
}

services {
    UserService {
        domains: Authentication
    }
}

use_case "User Registration" {
    when Business_User creates Account
        Authentication validates email
}`;

  const ecommerceContent = `services {
  OrderService {
    domains: Orders
  }
  PaymentService {
    domains: Payments  
  }
}

use_case "Place Order" {
  when Customer places order
    Orders creates order
}`;

  const architecturalOnlyContent = `actors {
    user Customer
}

arch {
    presentation:
        MobileApp
}`;

  test('should extract selected ranges plus architectural blocks from file with selections', () => {
    // Arrange - simulating a file with specific selections
    const ranges: BlockRange[] = [
      {
        startLine: 15,
        endLine: 19,
        fileUri: 'user-management.craft'
      }
    ];

    // Act - Extract DSL from file WITH selections
    const result = parser.extractSelectedDSL(userManagementContent, ranges);

    // Assert
    // Should contain selected UserService plus architectural blocks
    expect(result).toContain('UserService {');
    expect(result).toContain('domains: Authentication');
    expect(result).toContain('actors {');
    expect(result).toContain('arch {');
    expect(result).toContain('exposure default {');
    
    // Should NOT contain the use case (not selected)
    expect(result).not.toContain('use_case "User Registration"');
  });

  test('should extract only architectural blocks from file without selections', () => {
    // Act - Extract architectural blocks only (no ranges)
    const result = parser.extractArchitecturalBlocks(userManagementContent);

    // Assert
    // Should contain architectural blocks
    expect(result).toContain('actors {');
    expect(result).toContain('arch {');
    expect(result).toContain('exposure default {');
    
    // Should NOT contain services or use cases
    expect(result).not.toContain('UserService');
    expect(result).not.toContain('use_case "User Registration"');
  });

  test('should return empty string for file with no architectural blocks', () => {
    // Act - Extract architectural blocks from file with no architectural content
    const result = parser.extractArchitecturalBlocks(ecommerceContent);

    // Assert
    // E-commerce DSL has no actors, arch, or exposure blocks
    expect(result.trim()).toBe('');
  });

  test('should demonstrate the logic for multi-file processing', () => {
    // This test demonstrates how the handleExtractPartialDslFromBlockRanges should work
    
    // Simulate file with selections: get selected ranges + architectural blocks
    const rangesForUserMgmt: BlockRange[] = [{
      startLine: 15,
      endLine: 19,
      fileUri: 'user-management.craft'
    }];
    const userMgmtResult = parser.extractSelectedDSL(userManagementContent, rangesForUserMgmt);
    
    // Simulate file without selections: get only architectural blocks
    const ecommerceResult = parser.extractArchitecturalBlocks(ecommerceContent);
    
    // Combine results
    const combinedResult = [userMgmtResult, ecommerceResult].filter(r => r.trim()).join('\n\n');
    
    // Assert combined behavior
    expect(combinedResult).toContain('UserService {'); // selected from user-management
    expect(combinedResult).toContain('actors {');      // architectural from user-management
    expect(combinedResult).not.toContain('OrderService'); // not selected from e-commerce
    expect(combinedResult).not.toContain('use_case "User Registration"'); // not selected
  });
});
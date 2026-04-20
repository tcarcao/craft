# Grammar Fix Summary: Return Action Syntax Change

## Problem Solved

### Original Issues:
1. ❌ **Scheduled VAS Test Failing**: The word "when" appearing in phrases caused parser errors
   ```craft
   VASMgmt applies a VAS and stores details until when its valid
                                                     ^^^^ Error: keyword conflict
   ```

2. ❌ **Return Action Ambiguity**: The word "to" could be both a phrase word and a keyword separator
   ```craft
   BankGateway returns payment result to PaymentService
                                      ^^ Ambiguous: part of phrase or separator?
   ```

## Solution Implemented

### Grammar Changes

Changed the `return_action` syntax to eliminate ambiguity by moving `'to'` immediately after `'returns'`:

**Before**:
```antlr
return_action: domain 'returns' connector_word? phrase 'to' domain
            | domain 'returns' connector_word? phrase;
```

**After**:
```antlr
return_action: domain 'returns' 'to' domain connector_word? phrase
            | domain 'returns' connector_word? phrase;
```

Additionally, reordered action alternatives to try `return_action` before `internal_action`:

```antlr
action: async_action NEWLINE+
      | sync_action NEWLINE+
      | return_action NEWLINE+      // Moved before internal_action
      | internal_action NEWLINE+;
```

### DSL Syntax Change

**Old Syntax**:
```craft
BankGateway returns payment result to PaymentService
PaymentService returns confirmation status
```

**New Syntax**:
```craft
BankGateway returns to PaymentService payment result
BankGateway returns to PaymentService the payment result  # with optional connector
PaymentService returns confirmation status
```

### Key Benefits

1. ✅ **No Ambiguity**: The `'to'` keyword position is now unambiguous - it comes immediately after `'returns'`
2. ✅ **Phrases Can Contain Anything**: Words like "to", "when", "use_case" can appear freely in phrases
3. ✅ **Clearer Intent**: You immediately see who is returning to whom
4. ✅ **Natural English**: "returns to X the result" is grammatically correct
5. ✅ **Simpler Grammar**: No need for complex phrase_word exclusion rules

## Files Modified

### 1. Grammar File
- **File**: `tools/antlr-grammar/Craft.g4`
- **Changes**:
  - Updated `return_action` rule to put `'to' domain` before `phrase`
  - Reordered `action` alternatives to prioritize `return_action`
  - Kept `phrase_word` with 'when' support for scheduled VAS case

### 2. Example Files (4 occurrences updated)
- `examples/return_action_test.craft`
- `examples/return_flow_test.craft`

### 3. Test Files (4 test functions updated)
- `internal/parser/usecases_visitor_test.go`:
  - `TestParser_ReturnAction`
  - `TestParser_ReturnActionWithConnector`
  - (Other return tests)

### 4. Parser Code
- **File**: `internal/parser/usecases_visitor.go`
- **Status**: No changes needed - visitor already handles elements in any order

## Test Results

### All Tests Passing ✅

```bash
$ go test ./internal/parser -v
```

**Key Tests**:
- ✅ `TestParser_ScheduledVASApplied` - "when" in phrases works
- ✅ `TestParser_ReturnAction` - new syntax parses correctly
- ✅ `TestParser_ReturnActionWithConnector` - connector after target domain works
- ✅ `TestParser_ReturnActionCallStack` - return flow tracking works
- ✅ `TestParser_ReturnActionDD` - domain diagram generation works
- ✅ All 71 parser tests pass

## Examples

### Example 1: Return with Target Domain
```craft
use_case "Payment Processing" {
    when User submits payment request
        PaymentService asks BankGateway to process payment
        BankGateway returns to PaymentService payment result
        PaymentService returns confirmation status
}
```

**Parsed as**:
- Action 1: `BankGateway` returns to `PaymentService` the phrase "payment result"
- Action 2: `PaymentService` returns the phrase "confirmation status" (no target)

### Example 2: Return with Connector
```craft
use_case "Data Retrieval" {
    when User requests data
        DataService returns to User the user information
}
```

**Parsed as**:
- Domain: `DataService`
- Target: `User`
- Connector: `the`
- Phrase: `user information`

### Example 3: Scheduled VAS (Original Issue Fixed)
```craft
use_case "Scheduled VAS Applied" {
    when VASMgmt identifies a scheduled VAS to apply
        VASMgmt applies a VAS and stores details until when its valid
}
```

**Parsed as**:
- Trigger: `VASMgmt identifies` with phrase "scheduled VAS to apply"
- Action: `VASMgmt applies` with phrase "VAS and stores details until when its valid"
- ✅ "when" in phrase is now allowed

## Migration Guide

### For Existing `.craft` Files

Find and replace pattern:
```
returns <phrase> to <domain>
```

Replace with:
```
returns to <domain> <phrase>
```

### Examples of Migration

**Before** → **After**:
```craft
# Before
Authentication returns access token to User
Payment returns receipt to Customer
Service returns error message to Caller

# After
Authentication returns to User access token
Payment returns to Customer receipt
Service returns to Caller error message
```

## Technical Details

### Why Reordering Action Alternatives Was Necessary

ANTLR tries alternatives in order. Since `internal_action` is defined as:
```antlr
internal_action: domain verb connector_word? phrase;
```

And `verb` can be any `identifier` (including 'returns'), the parser would match `returns` as an internal_action before checking return_action.

By putting `return_action` first in the alternatives list, we ensure it's checked before `internal_action`, allowing the more specific rule to match.

### Phrase Word Rules

The final `phrase_word` rule allows keywords in phrases:

```antlr
phrase_word: identifier
           | connector_word
           | 'when'
           | 'use_case'  // Optional, can be removed if not needed
           ;
```

This enables natural language phrases like:
- "until when its valid"
- "to process the request"
- "for the user to verify"

## Conclusion

✅ **Problem Solved**: Both parser bugs fixed with a simple, elegant grammar change
✅ **Tests Passing**: All 71 parser tests pass
✅ **Migration Impact**: Minimal - only 8 occurrences in examples and tests needed updating
✅ **Better Design**: The new syntax is clearer and more grammatically correct

The key insight was recognizing that keyword ambiguity should be resolved by **syntax structure** (keyword position) rather than by excluding words from phrases. This makes the grammar simpler and the DSL more flexible.

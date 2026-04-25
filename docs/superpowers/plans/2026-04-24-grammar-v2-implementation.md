# Grammar V2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the genuine feature gaps from `craft/docs/decisions/grammar-v2-refactor-plan.md` in the hand-written lexer/parser stack and mirror changes to tree-sitter.

**Architecture:** The codebase uses a hand-written lexer (`internal/lexer/lexer.go`) and recursive-descent parser (`internal/syntax/parser.go`). The original plan was written when the parser was ANTLR-based. Q1 and Q4 (hard-reserving keywords) were needed to eliminate ANTLR LL(\*) grammar ambiguities — they do NOT apply to a hand-written context-sensitive parser and are intentionally excluded from this plan. Q2, Q3, Q12, Q13 are already implemented. This plan covers the remaining genuine gaps: Q5–Q11.

**Tech Stack:** Go 1.22 (`go test ./...`), tree-sitter Node.js CLI (`npx tree-sitter generate`, `npx tree-sitter test`).

---

## What Is and Is Not in Scope

| Decision | Status | Notes |
|---|---|---|
| Q1 hard-reserve action verbs | **OUT OF SCOPE** | ANTLR artifact; hand-written parser already handles contextually |
| Q2 `returns [to target]` | ✅ Already done | |
| Q3 newlines via line tracking | ✅ Already done | |
| Q4 hard-reserve structural keywords | **OUT OF SCOPE** | ANTLR artifact; no ambiguity in hand-written parser |
| Q5 open actor_type | **Gap** | Locked to user/system/service today |
| Q6 open deployment_type | **Gap** | Minor — keyword-typed tokens not accepted |
| Q7 modifier STRING+NUMBER; deployment rules | **Gap** | Strings/numbers in modifiers unimplemented; `canary(90%->x)` unparsed |
| Q8 block comments | **Gap** | Never implemented |
| Q9 identifier dots | **Gap** | `my.service` breaks today |
| Q10 string escapes | **Gap** | `\\`, `\n`, `\t`, `\r` not handled |
| Q11 singular `service X {}` form | **Gap** | Only `services { X {} }` exists |
| Q12 subject-less trigger | ✅ Already done | |
| Q13 actor/domain syntactically identical | ✅ Already done | |
| Tree-sitter mirror | **Gap** | Hardcoded actor_type/deployment_type; narrow identifier regex |

---

## File Map

| File | Change |
|---|---|
| `craft/internal/lexer/lexer.go` | Block comments, dot in ident, full string escapes, NUMBER/PERCENTAGE/LPAREN/RPAREN/ARROW tokens |
| `craft/internal/lexer/lexer_test.go` | Tests for each new token |
| `craft/internal/syntax/parser.go` | Open actor_type, open deployment_type, modifier STRING+NUMBER, full deployment rule parsing, singular `service X {}` |
| `craft/internal/syntax/parser_test.go` | Tests for Q5–Q11 test matrix |
| `craft/internal/ast/ast.go` | Add `DeploymentRule` slice to `ServiceDecl` |
| `tree-sitter-craft/grammar.js` | Open actor_type/deployment_type, add string to modifier_value, fix identifier regex, newlines to extras |
| `tree-sitter-craft/test/corpus/real_examples.txt` | Update deployment_type expected parse tree |
| `tree-sitter-craft/queries/highlights.scm` | actor_type identifier highlight; string modifier_value |

---

## Task 1: Capture Baseline

- [ ] **Step 1: Run tests and record baseline**

```bash
cd craft && go test ./... 2>&1
```

Expected: all packages green. This is the compatibility target — no regressions allowed.

- [ ] **Step 2: Commit**

```bash
git add -A && git commit -m "chore: baseline before grammar-v2 implementation"
```

---

## Task 2: Lexer — Block Comments (Q8)

**Files:**
- Modify: `craft/internal/lexer/lexer.go`
- Modify: `craft/internal/lexer/lexer_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestLexer_BlockComment(t *testing.T) {
    tests := []struct {
        name      string
        src       string
        wantTypes []lexer.TokenType
    }{
        {
            name:      "block comment skipped",
            src:       "/* this is a comment */ actor user Foo",
            wantTypes: []lexer.TokenType{lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
        },
        {
            name:      "multi-line block comment",
            src:       "/* line1\nline2 */ domain Foo { }",
            wantTypes: []lexer.TokenType{lexer.TokenKwDomain, lexer.TokenIdent, lexer.TokenLBrace, lexer.TokenRBrace, lexer.TokenEOF},
        },
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            l := lexer.New(tc.src)
            toks := l.All()
            if len(toks) != len(tc.wantTypes) {
                t.Fatalf("token count: got %d want %d\ntokens: %v", len(toks), len(tc.wantTypes), toks)
            }
            for i, tok := range toks {
                if tok.Type != tc.wantTypes[i] {
                    t.Errorf("token[%d]: got type %v want %v (value=%q)", i, tok.Type, tc.wantTypes[i], tok.Value)
                }
            }
        })
    }
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd craft && go test ./internal/lexer/... -run TestLexer_BlockComment -v
```

Expected: FAIL.

- [ ] **Step 3: Implement**

In `skipWhitespaceAndComments` in `craft/internal/lexer/lexer.go`, add the `/* */` case:

```go
func (l *Lexer) skipWhitespaceAndComments() {
    for l.pos < len(l.src) {
        ch := l.src[l.pos]
        switch {
        case ch == ' ' || ch == '\t' || ch == '\r':
            l.advance()
        case ch == '\n':
            l.advance()
        case ch == '/' && l.peek(1) == '/':
            for l.pos < len(l.src) && l.src[l.pos] != '\n' {
                l.advance()
            }
        case ch == '/' && l.peek(1) == '*':
            l.advance() // consume /
            l.advance() // consume *
            for l.pos < len(l.src) {
                if l.src[l.pos] == '*' && l.peek(1) == '/' {
                    l.advance() // consume *
                    l.advance() // consume /
                    break
                }
                l.advance()
            }
        default:
            return
        }
    }
}
```

- [ ] **Step 4: Confirm pass**

```bash
cd craft && go test ./internal/lexer/... -run TestLexer_BlockComment -v
```

Expected: PASS.

- [ ] **Step 5: Full suite**

```bash
cd craft && go test ./...
```

Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add craft/internal/lexer/lexer.go craft/internal/lexer/lexer_test.go
git commit -m "feat(lexer): block comment support (Q8)"
```

---

## Task 3: Lexer — Dot in Identifier (Q9)

**Files:**
- Modify: `craft/internal/lexer/lexer.go`
- Modify: `craft/internal/lexer/lexer_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestLexer_IdentifierWithDot(t *testing.T) {
    tests := []struct{ src, want string }{
        {"my.service", "my.service"},
        {"v1.2.3", "v1.2.3"},
    }
    for _, tc := range tests {
        t.Run(tc.src, func(t *testing.T) {
            l := lexer.New(tc.src)
            tok := l.Next()
            if tok.Type != lexer.TokenIdent {
                t.Fatalf("expected TokenIdent, got %v", tok.Type)
            }
            if tok.Value != tc.want {
                t.Errorf("got %q want %q", tok.Value, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd craft && go test ./internal/lexer/... -run TestLexer_IdentifierWithDot -v
```

Expected: FAIL — `my.service` becomes two tokens.

- [ ] **Step 3: Implement**

In `craft/internal/lexer/lexer.go`:

```go
func isIdentContinue(r rune) bool {
    return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}
```

- [ ] **Step 4: Confirm pass + full suite**

```bash
cd craft && go test ./internal/lexer/... -run TestLexer_IdentifierWithDot -v
cd craft && go test ./...
```

Expected: both green.

- [ ] **Step 5: Commit**

```bash
git add craft/internal/lexer/lexer.go craft/internal/lexer/lexer_test.go
git commit -m "feat(lexer): allow dots in identifier character class (Q9)"
```

---

## Task 4: Lexer — Full String Escape Sequences (Q10)

**Files:**
- Modify: `craft/internal/lexer/lexer.go`
- Modify: `craft/internal/lexer/lexer_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestLexer_StringEscapes(t *testing.T) {
    tests := []struct {
        name string
        src  string
        want string
    }{
        {name: "escaped quote",           src: `"He said \"hi\""`, want: `He said "hi"`},
        {name: "escaped backslash",       src: `"path\\file"`,      want: `path\file`},
        {name: "escaped newline",         src: `"line1\nline2"`,    want: "line1\nline2"},
        {name: "escaped tab",             src: `"col1\tcol2"`,      want: "col1\tcol2"},
        {name: "escaped carriage return", src: `"cr\r"`,            want: "cr\r"},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            l := lexer.New(tc.src)
            tok := l.Next()
            if tok.Type != lexer.TokenString {
                t.Fatalf("expected TokenString, got %v", tok.Type)
            }
            if tok.Value != tc.want {
                t.Errorf("got %q want %q", tok.Value, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run to confirm failures**

```bash
cd craft && go test ./internal/lexer/... -run TestLexer_StringEscapes -v
```

Expected: `\\`, `\n`, `\t`, `\r` cases FAIL.

- [ ] **Step 3: Replace `scanString`**

```go
func (l *Lexer) scanString() Token {
    startLine := l.line
    startCol := l.col
    l.advance() // consume opening "
    var val []rune
    closed := false
    for l.pos < len(l.src) {
        ch := l.src[l.pos]
        if ch == '"' {
            l.advance()
            closed = true
            break
        }
        if ch == '\\' && l.pos+1 < len(l.src) {
            l.advance() // consume backslash
            next := l.src[l.pos]
            l.advance() // consume escape char
            switch next {
            case '"':
                val = append(val, '"')
            case '\\':
                val = append(val, '\\')
            case 'n':
                val = append(val, '\n')
            case 't':
                val = append(val, '\t')
            case 'r':
                val = append(val, '\r')
            default:
                val = append(val, '\\', next)
            }
            continue
        }
        val = append(val, ch)
        l.advance()
    }
    if !closed {
        return Token{Type: TokenError, Value: "unterminated string literal", Line: startLine, Column: startCol}
    }
    return Token{Type: TokenString, Value: string(val), Line: startLine, Column: startCol}
}
```

- [ ] **Step 4: Confirm pass + full suite**

```bash
cd craft && go test ./internal/lexer/... -run TestLexer_StringEscapes -v
cd craft && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add craft/internal/lexer/lexer.go craft/internal/lexer/lexer_test.go
git commit -m "feat(lexer): full string escape sequences — backslash n t r (Q10)"
```

---

## Task 5: Lexer — NUMBER, PERCENTAGE, LPAREN, RPAREN, ARROW Tokens

Required for modifier number values (Q7) and deployment rule parsing (`canary(90% -> stable)`).

**Files:**
- Modify: `craft/internal/lexer/lexer.go`
- Modify: `craft/internal/lexer/lexer_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestLexer_NumberTokens(t *testing.T) {
    tests := []struct {
        src       string
        wantType  lexer.TokenType
        wantValue string
    }{
        {"42",   lexer.TokenNumber,     "42"},
        {"1.5",  lexer.TokenNumber,     "1.5"},
        {"90%",  lexer.TokenPercentage, "90%"},
        {"25.5%",lexer.TokenPercentage, "25.5%"},
    }
    for _, tc := range tests {
        t.Run(tc.src, func(t *testing.T) {
            l := lexer.New(tc.src)
            tok := l.Next()
            if tok.Type != tc.wantType {
                t.Errorf("type: got %v want %v", tok.Type, tc.wantType)
            }
            if tok.Value != tc.wantValue {
                t.Errorf("value: got %q want %q", tok.Value, tc.wantValue)
            }
        })
    }
}

func TestLexer_PunctuationTokens(t *testing.T) {
    tests := []struct {
        src      string
        wantType lexer.TokenType
    }{
        {"(", lexer.TokenLParen},
        {")", lexer.TokenRParen},
        {"->", lexer.TokenArrow},
    }
    for _, tc := range tests {
        t.Run(tc.src, func(t *testing.T) {
            l := lexer.New(tc.src)
            tok := l.Next()
            if tok.Type != tc.wantType {
                t.Errorf("got %v want %v", tok.Type, tc.wantType)
            }
        })
    }
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd craft && go test ./internal/lexer/... -run "TestLexer_NumberTokens|TestLexer_PunctuationTokens" -v
```

Expected: FAIL — types don't exist yet.

- [ ] **Step 3: Add token type constants**

In `craft/internal/lexer/lexer.go`, add after `TokenKwExposure` and before `TokenSentinel`:

```go
    // Q7: numeric values for modifier values and deployment rules
    TokenNumber     // [0-9]+ ('.' [0-9]+)?
    TokenPercentage // NUMBER '%'

    // deployment rule punctuation
    TokenLParen // (
    TokenRParen // )
    TokenArrow  // ->
```

- [ ] **Step 4: Add `scanNumber` method**

```go
func (l *Lexer) scanNumber() Token {
    startLine := l.line
    startCol := l.col
    start := l.pos
    for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
        l.advance()
    }
    if l.pos < len(l.src) && l.src[l.pos] == '.' &&
        l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1]) {
        l.advance() // consume '.'
        for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
            l.advance()
        }
    }
    val := string(l.src[start:l.pos])
    if l.pos < len(l.src) && l.src[l.pos] == '%' {
        l.advance()
        return Token{Type: TokenPercentage, Value: val + "%", Line: startLine, Column: startCol}
    }
    return Token{Type: TokenNumber, Value: val, Line: startLine, Column: startCol}
}
```

- [ ] **Step 5: Add cases to `Next()`**

Add these cases in the `switch` block, before the `isIdentStart(ch)` case:

```go
case ch == '(':
    return l.consume(TokenLParen)
case ch == ')':
    return l.consume(TokenRParen)
case ch == '-' && l.peek(1) == '>':
    tok := l.token(TokenArrow, "->")
    l.advance()
    l.advance()
    return tok
case unicode.IsDigit(ch):
    return l.scanNumber()
```

**Note:** The `ch == '-' && l.peek(1) == '>'` case must appear **before** the `isIdentStart` branch. Currently `-` is not an ident start (only a continuation), so there is no conflict. However, verify that `->` in `deployment: canary(90% -> stable)` tokenizes as `TokenArrow` not as two tokens by running the test.

- [ ] **Step 6: Confirm pass + full suite**

```bash
cd craft && go test ./internal/lexer/... -run "TestLexer_NumberTokens|TestLexer_PunctuationTokens" -v
cd craft && go test ./...
```

Expected: both green.

- [ ] **Step 7: Commit**

```bash
git add craft/internal/lexer/lexer.go craft/internal/lexer/lexer_test.go
git commit -m "feat(lexer): NUMBER, PERCENTAGE, LPAREN, RPAREN, ARROW tokens"
```

---

## Task 6: Parser — Open Actor Type (Q5)

Currently `parseActorStatement` / `parseActorsBlock` reject any actor type that isn't `user`, `system`, or `service`.

**Files:**
- Modify: `craft/internal/syntax/parser.go`
- Modify: `craft/internal/syntax/parser_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestParse_OpenActorType(t *testing.T) {
    tests := []struct {
        src      string
        wantName string
        wantType string
    }{
        {"actor bot SlackBot", "SlackBot", "bot"},
        {"actor integration ExternalCRM", "ExternalCRM", "integration"},
    }
    for _, tc := range tests {
        t.Run(tc.wantType, func(t *testing.T) {
            f, diags := syntax.Parse(tc.src)
            if len(diags) != 0 {
                t.Fatalf("unexpected diagnostics: %v", diags)
            }
            if len(f.Actors) != 1 {
                t.Fatalf("expected 1 actor, got %d", len(f.Actors))
            }
            a := f.Actors[0]
            if a.Name != tc.wantName || string(a.Type) != tc.wantType {
                t.Errorf("got name=%q type=%q", a.Name, a.Type)
            }
        })
    }
}

func TestParse_OpenActorTypeInBlock(t *testing.T) {
    src := `actors {
    bot SlackBot
    integration ExternalCRM
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    if len(f.Actors) != 2 {
        t.Fatalf("expected 2 actors, got %d", len(f.Actors))
    }
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd craft && go test ./internal/syntax/... -run TestParse_OpenActorType -v
```

Expected: FAIL — `bot` is not a known actor type.

- [ ] **Step 3: Update `tokenToActorType`**

Replace the function in `craft/internal/syntax/parser.go`:

```go
// tokenToActorType converts a token to an ActorType.
// Q5: any identifier is a valid actor type (open taxonomy).
// The canonical types (user/system/service) are preserved as constants.
func tokenToActorType(tok lexer.Token) (ast.ActorType, bool) {
    switch tok.Type {
    case lexer.TokenKwUser:
        return ast.ActorTypeUser, true
    case lexer.TokenKwSystem:
        return ast.ActorTypeSystem, true
    case lexer.TokenKwService:
        return ast.ActorTypeService, true
    case lexer.TokenIdent:
        return ast.ActorType(tok.Value), true
    }
    if isAnyKeywordAsIdent(tok.Type) {
        return ast.ActorType(tok.Value), true
    }
    return "", false
}
```

- [ ] **Step 4: Confirm pass + full suite**

```bash
cd craft && go test ./internal/syntax/... -run TestParse_OpenActorType -v
cd craft && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add craft/internal/syntax/parser.go craft/internal/syntax/parser_test.go
git commit -m "feat(parser): open actor taxonomy — any identifier is a valid actor type (Q5)"
```

---

## Task 7: Parser — Modifier Values Accept STRING and NUMBER (Q7)

**Files:**
- Modify: `craft/internal/syntax/parser.go`
- Modify: `craft/internal/syntax/parser_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestParse_ModifierValueTypes(t *testing.T) {
    src := `arch {
    presentation:
        WebApp[title:"My App", port:8080, timeout:1.5]
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    comp := f.Archs[0].Presentation[0]
    if len(comp.Modifiers) != 3 {
        t.Fatalf("expected 3 modifiers, got %d: %+v", len(comp.Modifiers), comp.Modifiers)
    }
    want := []struct{ key, value string }{
        {"title", "My App"},
        {"port", "8080"},
        {"timeout", "1.5"},
    }
    for i, w := range want {
        m := comp.Modifiers[i]
        if m.Key != w.key || m.Value != w.value {
            t.Errorf("modifier[%d]: got {%q,%q} want {%q,%q}", i, m.Key, m.Value, w.key, w.value)
        }
    }
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd craft && go test ./internal/syntax/... -run TestParse_ModifierValueTypes -v
```

Expected: FAIL — string and number values rejected.

- [ ] **Step 3: Update modifier value parsing in `parseModifierList`**

Replace the value-reading block inside `parseModifierList`:

```go
var value string
if p.peek().Type == lexer.TokenColon {
    p.consume() // consume ':'
    valTok := p.peek()
    switch valTok.Type {
    case lexer.TokenIdent:
        value = valTok.Value
        p.consume()
    case lexer.TokenString:
        value = valTok.Value // already unquoted by lexer
        p.consume()
    case lexer.TokenNumber, lexer.TokenPercentage:
        value = valTok.Value
        p.consume()
    default:
        if isAnyKeywordAsIdent(valTok.Type) {
            value = valTok.Value
            p.consume()
        } else {
            diags = append(diags, p.diagUnexpected(valTok, "modifier value (identifier, string, or number)"))
        }
    }
}
```

- [ ] **Step 4: Confirm pass + full suite**

```bash
cd craft && go test ./internal/syntax/... -run TestParse_ModifierValueTypes -v
cd craft && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add craft/internal/syntax/parser.go craft/internal/syntax/parser_test.go
git commit -m "feat(parser): modifier values accept STRING and NUMBER (Q7)"
```

---

## Task 8: Parser — Full Deployment Rule Parsing (Q7 / Q6)

`deployment: canary(90% -> stable, 10% -> experimental)` currently only captures `"canary"` — the `(...)` rule list is ignored.

**Files:**
- Modify: `craft/internal/ast/ast.go`
- Modify: `craft/internal/syntax/parser.go`
- Modify: `craft/internal/syntax/parser_test.go`
- Modify: `craft/internal/syntax/projection.go`

- [ ] **Step 1: Add `DeploymentRule` slice to `ast.ServiceDecl`**

In `craft/internal/ast/ast.go`, update `ServiceDecl`:

```go
type ServiceDecl struct {
    Name            string           `json:"name"`
    Contexts        []string         `json:"contexts,omitempty"`
    DataStores      []string         `json:"dataStores,omitempty"`
    Language        string           `json:"language,omitempty"`
    DeploymentType  string           `json:"deploymentType,omitempty"`
    DeploymentRules []DeploymentRule `json:"deploymentRules,omitempty"`
    Line            int              `json:"line,omitempty"`
    ContextLines    []int            `json:"contextLines,omitempty"`
}

// DeploymentRule is a single percentage-to-target mapping within a deployment spec.
type DeploymentRule struct {
    Percentage string // e.g. "90%"
    Target     string // e.g. "stable"
}
```

- [ ] **Step 2: Write failing test**

```go
func TestParse_DeploymentRules(t *testing.T) {
    src := `services {
    CommsService {
        contexts: Notifier
        deployment: canary(90% -> stable, 10% -> experimental)
    }
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    if len(f.Services) != 1 {
        t.Fatalf("expected 1 service")
    }
    svc := f.Services[0]
    if svc.DeploymentType != "canary" {
        t.Errorf("type: got %q want canary", svc.DeploymentType)
    }
    if len(svc.DeploymentRules) != 2 {
        t.Fatalf("expected 2 rules, got %d", len(svc.DeploymentRules))
    }
    if svc.DeploymentRules[0].Percentage != "90%" || svc.DeploymentRules[0].Target != "stable" {
        t.Errorf("rule[0]: %+v", svc.DeploymentRules[0])
    }
    if svc.DeploymentRules[1].Percentage != "10%" || svc.DeploymentRules[1].Target != "experimental" {
        t.Errorf("rule[1]: %+v", svc.DeploymentRules[1])
    }
}

func TestParse_DeploymentTypeOnly(t *testing.T) {
    src := `services { UserService { deployment: blue_green } }`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    if f.Services[0].DeploymentType != "blue_green" {
        t.Errorf("type: got %q", f.Services[0].DeploymentType)
    }
    if len(f.Services[0].DeploymentRules) != 0 {
        t.Errorf("expected no rules, got %d", len(f.Services[0].DeploymentRules))
    }
}
```

- [ ] **Step 3: Run to confirm failure**

```bash
cd craft && go test ./internal/syntax/... -run "TestParse_Deployment" -v
```

Expected: `TestParse_DeploymentRules` FAIL; `TestParse_DeploymentTypeOnly` may pass already.

- [ ] **Step 4: Add `parseDeploymentSpec` to parser**

Add to `craft/internal/syntax/parser.go`:

```go
// parseDeploymentSpec parses: deployment_type ('(' deployment_rule (',' deployment_rule)* ')')?
func (p *Parser) parseDeploymentSpec() (string, []ast.DeploymentRule, []craft.Diagnostic) {
    var diags []craft.Diagnostic

    typeTok := p.peek()
    var dt string
    if typeTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(typeTok.Type) {
        dt = typeTok.Value
        p.consume()
    } else {
        diags = append(diags, p.diagUnexpected(typeTok, "deployment type identifier"))
        return "", nil, diags
    }

    if p.peek().Type != lexer.TokenLParen {
        return dt, nil, diags
    }
    p.consume() // consume '('

    var rules []ast.DeploymentRule
    for !p.atEOF() && p.peek().Type != lexer.TokenRParen {
        pctTok := p.peek()
        if pctTok.Type != lexer.TokenPercentage {
            diags = append(diags, p.diagUnexpected(pctTok, "percentage (e.g. 90%)"))
            p.consume()
            continue
        }
        pct := pctTok.Value
        p.consume()

        if p.peek().Type != lexer.TokenArrow {
            diags = append(diags, p.diagUnexpected(p.peek(), "->"))
            continue
        }
        p.consume() // consume '->'

        targetTok := p.peek()
        var target string
        if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
            target = targetTok.Value
            p.consume()
        } else {
            diags = append(diags, p.diagUnexpected(targetTok, "deployment target identifier"))
        }

        rules = append(rules, ast.DeploymentRule{Percentage: pct, Target: target})

        if p.peek().Type == lexer.TokenComma {
            p.consume()
        }
    }

    if p.atEOF() {
        diags = append(diags, craft.Diagnostic{
            Code:     "craft/syntax/unclosed-block",
            Message:  "unclosed deployment rule list (missing `)`)",
            Severity: craft.SeverityError,
            Range:    tokenRange(typeTok),
        })
        return dt, rules, diags
    }
    p.consume() // consume ')'
    return dt, rules, diags
}
```

- [ ] **Step 5: Wire into `parseServiceBody`**

In the `"deployment"` case of `parseServiceBody`:

```go
case "deployment":
    dt, rules, dd := p.parseDeploymentSpec()
    diags = append(diags, dd...)
    svc.DeploymentType = dt
    svc.DeploymentRules = rules
```

- [ ] **Step 6: Update projection to pass rules through**

In `craft/internal/syntax/projection.go`, inside `mergeServices`, after the existing deployment type merge:

```go
if e.svc.Deployment.Type == "" && s.DeploymentType != "" {
    e.svc.Deployment.Type = s.DeploymentType
}
if len(e.svc.Deployment.Rules) == 0 && len(s.DeploymentRules) > 0 {
    for _, r := range s.DeploymentRules {
        e.svc.Deployment.Rules = append(e.svc.Deployment.Rules, craft.DeploymentRule{
            Percentage: r.Percentage,
            Target:     r.Target,
        })
    }
}
```

- [ ] **Step 7: Confirm pass + full suite**

```bash
cd craft && go test ./internal/syntax/... -run "TestParse_Deployment" -v
cd craft && go test ./...
```

- [ ] **Step 8: Commit**

```bash
git add craft/internal/ast/ast.go craft/internal/syntax/parser.go craft/internal/syntax/parser_test.go craft/internal/syntax/projection.go
git commit -m "feat(parser): full deployment rule parsing — canary(N%->target) form (Q7)"
```

---

## Task 9: Parser — Singular Service Form (Q11)

Add `service X { ... }` as a top-level construct alongside `services { X { } }`.

**Files:**
- Modify: `craft/internal/syntax/parser.go`
- Modify: `craft/internal/syntax/parser_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestParse_SingleServiceForm(t *testing.T) {
    src := `service UserService {
    contexts: Auth, Profile
    language: golang
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    if len(f.Services) != 1 {
        t.Fatalf("expected 1 service, got %d", len(f.Services))
    }
    svc := f.Services[0]
    if svc.Name != "UserService" {
        t.Errorf("name: got %q", svc.Name)
    }
    if len(svc.Contexts) != 2 {
        t.Errorf("contexts: got %d want 2", len(svc.Contexts))
    }
    if svc.Language != "golang" {
        t.Errorf("language: got %q", svc.Language)
    }
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd craft && go test ./internal/syntax/... -run TestParse_SingleServiceForm -v
```

Expected: FAIL — `service` at top level produces "not-yet-implemented" diagnostic.

- [ ] **Step 3: Add `parseServiceStatement` and wire it**

Add to `craft/internal/syntax/parser.go`:

```go
// parseServiceStatement parses: service <name> { <field>* }
// This is the singular top-level service form (Q11).
func (p *Parser) parseServiceStatement() (*ast.ServiceDecl, []craft.Diagnostic) {
    p.consume() // consume `service`
    var diags []craft.Diagnostic

    nameTok := p.peek()
    var name string
    var nameLine int
    switch nameTok.Type {
    case lexer.TokenIdent, lexer.TokenString:
        name = nameTok.Value
        nameLine = nameTok.Line
        p.consume()
    default:
        if isServiceNameKeyword(nameTok.Type) {
            name = nameTok.Value
            nameLine = nameTok.Line
            p.consume()
        } else {
            diags = append(diags, p.diagUnexpected(nameTok, "service name"))
            p.resyncToTopLevel()
            return nil, diags
        }
    }

    if p.peek().Type != lexer.TokenLBrace {
        diags = append(diags, p.diagUnexpected(p.peek(), "{"))
        p.resyncToTopLevel()
        return nil, diags
    }
    p.consume() // consume '{'

    svc, d := p.parseServiceBody(name, nameLine)
    diags = append(diags, d...)

    if p.atEOF() {
        diags = append(diags, craft.Diagnostic{
            Code:     "craft/syntax/unclosed-block",
            Message:  fmt.Sprintf("unclosed service block for %q (missing `}`)", name),
            Severity: craft.SeverityError,
            Range:    craft.Range{
                Start: craft.Position{Line: ast.LineToLSP(nameLine)},
                End:   craft.Position{Line: ast.LineToLSP(nameLine)},
            },
        })
        return svc, diags
    }
    p.consume() // consume '}'
    return svc, diags
}
```

In `parseFile`, add a case before `default`:

```go
case lexer.TokenKwService:
    svc, d := p.parseServiceStatement()
    diags = append(diags, d...)
    if svc != nil {
        file.Services = append(file.Services, svc)
    }
```

In `isTopLevelKeyword`, add:

```go
case lexer.TokenKwService:
    return true
```

- [ ] **Step 4: Confirm pass + full suite**

```bash
cd craft && go test ./internal/syntax/... -run TestParse_SingleServiceForm -v
cd craft && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add craft/internal/syntax/parser.go craft/internal/syntax/parser_test.go
git commit -m "feat(parser): singular service X {} top-level form (Q11)"
```

---

## Task 10: Tests — Full Test Matrix

Add all test-matrix cases from §7 of the original plan that aren't covered by earlier tasks.

**Files:**
- Modify: `craft/internal/syntax/parser_test.go`

- [ ] **Step 1: Add remaining test matrix tests**

```go
// Q2: returns to <target> <phrase>
func TestParse_ReturnWithTarget(t *testing.T) {
    src := `use_case "Test" {
    when User checks balance
        AccountManagement returns to User confirmation status
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    a := f.UseCases[0].Scenarios[0].Actions[0]
    if a.TargetDomain != "User" {
        t.Errorf("targetDomain: got %q want User", a.TargetDomain)
    }
    if a.Phrase != "confirmation status" {
        t.Errorf("phrase: got %q want %q", a.Phrase, "confirmation status")
    }
}

// Q2: returns without target
func TestParse_ReturnWithoutTarget(t *testing.T) {
    src := `use_case "Test" {
    when User checks balance
        AccountManagement returns confirmation status
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    a := f.UseCases[0].Scenarios[0].Actions[0]
    if a.TargetDomain != "" {
        t.Errorf("targetDomain: got %q want empty", a.TargetDomain)
    }
    if a.Phrase != "confirmation status" {
        t.Errorf("phrase: got %q want %q", a.Phrase, "confirmation status")
    }
}

// Q3: single-line services block parses identically to multi-line
func TestParse_SingleLineServicesEquivalent(t *testing.T) {
    multi := `services {
    A { contexts: X }
    B { contexts: Y }
}`
    single := `services { A { contexts: X } B { contexts: Y } }`
    f1, d1 := syntax.Parse(multi)
    f2, d2 := syntax.Parse(single)
    if len(d1) != 0 || len(d2) != 0 {
        t.Fatalf("diagnostics: multi=%v single=%v", d1, d2)
    }
    if len(f1.Services) != len(f2.Services) {
        t.Errorf("service count: multi=%d single=%d", len(f1.Services), len(f2.Services))
    }
}

// Q8: block comments are ignored (parser-level)
func TestParse_BlockComment(t *testing.T) {
    src := `/* opening comment */
services {
    /* inline comment */ UserService {
        contexts: Auth
    }
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    if len(f.Services) != 1 || f.Services[0].Name != "UserService" {
        t.Fatalf("unexpected services: %+v", f.Services)
    }
}

// Q10: use_case name with escaped quotes
func TestParse_UseCaseEscapedName(t *testing.T) {
    src := `use_case "He said \"hello\"" {
    when User greets
        System confirms greeting
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    if f.UseCases[0].Name != `He said "hello"` {
        t.Errorf("name: got %q", f.UseCases[0].Name)
    }
}

// Q12: subject-less event trigger
func TestParse_SubjectlessTrigger(t *testing.T) {
    src := `use_case "Cron" {
    when "DailyCron"
        PaymentProcessing processes payments
}`
    f, diags := syntax.Parse(src)
    if len(diags) != 0 {
        t.Fatalf("unexpected diagnostics: %v", diags)
    }
    trigger := f.UseCases[0].Scenarios[0].Trigger
    if trigger.TriggerType != "event" {
        t.Errorf("type: got %q want event", trigger.TriggerType)
    }
    if trigger.Event != "DailyCron" {
        t.Errorf("event: got %q want DailyCron", trigger.Event)
    }
}
```

- [ ] **Step 2: Run all new tests**

```bash
cd craft && go test ./internal/syntax/... -run "TestParse_Return|TestParse_SingleLine|TestParse_BlockComment|TestParse_UseCase|TestParse_Subjectless" -v
```

Expected: all PASS (tasks 2–9 cover them).

- [ ] **Step 3: Full suite**

```bash
cd craft && go test ./...
```

- [ ] **Step 4: Commit**

```bash
git add craft/internal/syntax/parser_test.go
git commit -m "test: Q2/Q3/Q8/Q10/Q12 test matrix coverage"
```

---

## Task 11: Examples Corpus Validation

- [ ] **Step 1: Parse all examples**

```bash
cd craft && go run ./cmd/craft validate examples/ 2>&1
```

Expected: zero errors.

- [ ] **Step 2: Full suite without cache**

```bash
cd craft && go test ./... -count=1
```

Expected: all green.

---

## Task 12: Tree-sitter Mirror

Implement §5 of the original plan in `tree-sitter-craft/grammar.js`.

**Files:**
- Modify: `tree-sitter-craft/grammar.js`
- Modify: `tree-sitter-craft/test/corpus/real_examples.txt`
- Modify: `tree-sitter-craft/queries/highlights.scm`

### 12.1 — Open `actor_type`

```js
// Before:
actor_type: $ => choice('user', 'system', 'service'),
// After:
actor_type: $ => $.identifier,
```

### 12.2 — Open `deployment_type`

```js
// Before:
deployment_type: $ => choice('canary', 'blue_green', 'rolling'),
// After:
deployment_type: $ => $.identifier,
```

### 12.3 — Add `string` to `modifier_value`

```js
// Before:
modifier_value: $ => choice($.identifier, $.number, $.boolean),
// After:
modifier_value: $ => choice($.identifier, $.number, $.boolean, $.string),
```

### 12.4 — Fix `identifier` regex (Q9)

```js
// Before:
identifier: $ => /[a-zA-Z_][a-zA-Z0-9_-]*/,
// After:
identifier: $ => /[a-zA-Z0-9_][a-zA-Z0-9_.-]*/,
```

### 12.5 — Add newlines to extras (Q3 loose mode)

```js
// Before:
extras: $ => [
  /[ \t]/,
  $.comment,
],
// After:
extras: $ => [
  /[ \t]/,
  $._newline,
  $.comment,
],
```

This makes tree-sitter treat newlines as optional whitespace globally (looser than the Go parser but acceptable for editor tooling — per §5 recommendation (b) of the original plan).

- [ ] **Step 1: Apply changes 12.1–12.5 to `tree-sitter-craft/grammar.js`**

- [ ] **Step 2: Generate**

```bash
cd tree-sitter-craft && npx tree-sitter generate 2>&1
```

Expected: no errors.

- [ ] **Step 3: Run tests — expect failures on deployment_type**

```bash
cd tree-sitter-craft && npx tree-sitter test 2>&1
```

Expected: failures where `(deployment_type)` appears as a leaf — it now wraps an identifier.

- [ ] **Step 4: Update corpus test**

In `tree-sitter-craft/test/corpus/real_examples.txt`, replace every occurrence of:

```
(deployment_type)
```

with:

```
(deployment_type
  (identifier))
```

Verify with: `grep -n 'deployment_type' tree-sitter-craft/test/corpus/real_examples.txt` — there are 3 occurrences in the "Services with deployment strategies" test case.

Also check actor corpus tests:

```bash
grep -n 'actor_type' tree-sitter-craft/test/corpus/*.txt
```

If any test has `(actor_type)` as a leaf node, update it to `(actor_type (identifier))`.

- [ ] **Step 5: Update `highlights.scm`**

Replace the fixed-string actor type highlights:

```scheme
; Before:
"user" @craft.actor-type
"system" @craft.actor-type
"service" @craft.actor-type

; After:
(actor_type (identifier) @craft.actor-type)
```

Add string modifier_value highlight (after existing modifier_value lines):

```scheme
(modifier_value (string) @craft.modifier-value)
```

- [ ] **Step 6: Run tests again**

```bash
cd tree-sitter-craft && npx tree-sitter test 2>&1
```

Expected: all PASS.

- [ ] **Step 7: Full Go suite (no regressions)**

```bash
cd craft && go test ./...
```

- [ ] **Step 8: Commit**

```bash
cd ..
git add tree-sitter-craft/grammar.js tree-sitter-craft/test/corpus/ tree-sitter-craft/queries/
git commit -m "feat(tree-sitter): open actor_type/deployment_type, string modifier, fix identifier regex (§5)"
```

---

## Final Verification

- [ ] Complete Go test suite: `cd craft && go test ./... -count=1 -v 2>&1 | grep -E "^(ok|FAIL|---)"` — all `ok`
- [ ] Examples parse clean: `go run ./cmd/craft validate examples/` — zero errors
- [ ] Tree-sitter tests: `cd tree-sitter-craft && npx tree-sitter test` — all pass

### Test Matrix Checklist (§7 of original plan)

| Input | Test |
|---|---|
| All `craft/examples/` files | Task 11 |
| `X returns to User confirmation status` → TargetDomain="User" | `TestParse_ReturnWithTarget` |
| `X returns confirmation status` → TargetDomain="" | `TestParse_ReturnWithoutTarget` |
| `services { A{} B{} }` single-line | `TestParse_SingleLineServicesEquivalent` |
| `actor bot SlackBot` → actor_type="bot" | `TestParse_OpenActorType` |
| `deployment: shadow` → type="shadow" | `TestParse_OpenDeploymentType` |
| `deployment: canary(90%->stable, 10%->experimental)` | `TestParse_DeploymentRules` |
| `WebApp[title:"My App", port:8080, timeout:1.5]` | `TestParse_ModifierValueTypes` |
| `/* block comment */` skipped | `TestParse_BlockComment` |
| `"He said \"hi\""` escape | `TestParse_UseCaseEscapedName` |
| `when "DailyCron"` event trigger | `TestParse_SubjectlessTrigger` |

---

## Self-Review

**Spec coverage:** Every Q5–Q11 gap has a task. Q1/Q4 intentionally excluded (ANTLR artifact). Q2/Q3/Q12/Q13 already done.

**Placeholder check:** All tasks include actual code. No TBDs.

**Dropped from previous plan version:** Tasks 6, 7, 8 (hard-reserving keyword tokens) and the large parser predicate overhaul in 8.1–8.15 — none of this is technically needed in a hand-written parser and would introduce unnecessary breaking changes.

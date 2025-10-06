import { CharStream, CommonTokenStream, ParserRuleContext } from "antlr4ng";
import { CraftLexer } from "./generated/CraftLexer";
import { CraftParser } from "./generated/CraftParser";
import { CompletionItem, CompletionItemKind, Position } from 'vscode-languageserver/node.js';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { Logger } from '../utils/Logger.js';

export interface CompletionContext {
  ruleStack: string[];
  currentRule?: string;
  parentRule?: string;
  tokensBefore: string[];
  isAtStartOfLine: boolean;
  indentLevel: number;
}

export class CraftCompletionProvider {
  
  getCompletions(document: TextDocument, position: Position): CompletionItem[] {
    try {
      const context = this.analyzeCompletionContext(document, position);
      return this.getCompletionsForContext(context);
    } catch (error) {
      Logger.error('Error getting completions:', error);
      return [];
    }
  }

  private analyzeCompletionContext(document: TextDocument, position: Position): CompletionContext {
    const text = document.getText();
    const lines = text.split('\n');
    const currentLine = lines[position.line] || '';
    const textBeforeCursor = currentLine.substring(0, position.character);
    
    // Create a modified text that includes a placeholder token at cursor position
    // This helps ANTLR understand the current parsing context
    const textWithPlaceholder = this.insertPlaceholderAtPosition(text, position);
    
    try {
      // Parse the text to get the current context
      const lexer = new CraftLexer(CharStream.fromString(textWithPlaceholder));
      const tokens = new CommonTokenStream(lexer);
      const parser = new CraftParser(tokens);
      
      // Remove error listeners for better performance
      parser.removeErrorListeners();
      lexer.removeErrorListeners();
      
      const tree = parser.dsl();
      
      // Find the context at cursor position
      const ruleStack = this.findRuleStackAtPosition(tree, position, textWithPlaceholder);
      
      return {
        ruleStack,
        currentRule: ruleStack[ruleStack.length - 1],
        parentRule: ruleStack[ruleStack.length - 2],
        tokensBefore: textBeforeCursor.trim().split(/\s+/).filter(t => t.length > 0),
        isAtStartOfLine: textBeforeCursor.trim().length === 0,
        indentLevel: this.getIndentLevel(currentLine)
      };
    } catch (error) {
      // Fallback to simple text-based analysis
      return this.getFallbackContext(text, position);
    }
  }

  private insertPlaceholderAtPosition(text: string, position: Position): string {
    const lines = text.split('\n');
    const currentLine = lines[position.line] || '';
    const beforeCursor = currentLine.substring(0, position.character);
    const afterCursor = currentLine.substring(position.character);
    
    // Insert a placeholder identifier that won't break parsing
    const modifiedLine = beforeCursor + '__COMPLETION_PLACEHOLDER__' + afterCursor;
    lines[position.line] = modifiedLine;
    
    return lines.join('\n');
  }

  private findRuleStackAtPosition(tree: ParserRuleContext, position: Position, text: string): string[] {
    const lines = text.split('\n');
    let currentLine = 0;
    let currentChar = 0;
    
    // Convert position to absolute character offset
    for (let i = 0; i < position.line && i < lines.length; i++) {
      currentChar += lines[i].length + 1; // +1 for newline
    }
    currentChar += position.character;
    
    const ruleStack: string[] = [];
    this.traverseTreeForPosition(tree, currentChar, ruleStack);
    
    return ruleStack;
  }

  private traverseTreeForPosition(ctx: ParserRuleContext, targetPos: number, ruleStack: string[]): boolean {
    if (!ctx) return false;
    
    const start = ctx.start?.start || 0;
    const stop = ctx.stop?.stop || 0;
    
    // Check if target position is within this context
    if (targetPos >= start && targetPos <= stop) {
      ruleStack.push(this.getRuleName(ctx));
      
      // Check children
      for (let i = 0; i < ctx.getChildCount(); i++) {
        const child = ctx.getChild(i);
        if (child instanceof ParserRuleContext) {
          if (this.traverseTreeForPosition(child, targetPos, ruleStack)) {
            return true;
          }
        }
      }
      return true;
    }
    
    return false;
  }

  private getRuleName(ctx: ParserRuleContext): string {
    const className = ctx.constructor.name;
    // Convert "ServiceBlockContext" to "service_block"
    return className.replace('Context', '').replace(/([A-Z])/g, '_$1').toLowerCase().substring(1);
  }

  private getFallbackContext(text: string, position: Position): CompletionContext {
    const lines = text.split('\n');
    const currentLine = lines[position.line] || '';
    const textBeforeCursor = currentLine.substring(0, position.character);
    
    // Simple rule detection based on surrounding text
    const ruleStack: string[] = [];
    
    // Look for block context by scanning backwards
    for (let i = position.line; i >= 0; i--) {
      const line = lines[i].trim();
      if (line.startsWith('services {')) {
        ruleStack.push('services_def');
        break;
      } else if (line.startsWith('arch {')) {
        ruleStack.push('arch');
        break;
      } else if (line.startsWith('domain ')) {
        ruleStack.push('domain_block');
        break;
      } else if (line.startsWith('use_case ')) {
        ruleStack.push('use_case');
        break;
      } else if (line.startsWith('exposure ')) {
        ruleStack.push('exposure');
        break;
      }
    }
    
    return {
      ruleStack,
      currentRule: ruleStack[ruleStack.length - 1],
      parentRule: ruleStack[ruleStack.length - 2],
      tokensBefore: textBeforeCursor.trim().split(/\s+/).filter(t => t.length > 0),
      isAtStartOfLine: textBeforeCursor.trim().length === 0,
      indentLevel: this.getIndentLevel(currentLine)
    };
  }

  private getIndentLevel(line: string): number {
    const match = line.match(/^(\s*)/);
    return match ? Math.floor(match[1].length / 4) : 0;
  }

  private getCompletionsForContext(context: CompletionContext): CompletionItem[] {
    const { currentRule, parentRule, tokensBefore, isAtStartOfLine } = context;
    
    // Top-level completions
    if (!currentRule || currentRule === 'dsl') {
      return this.getTopLevelCompletions();
    }
    
    // Architecture block completions
    if (currentRule === 'arch' || parentRule === 'arch') {
      return this.getArchCompletions();
    }
    
    // Services block completions
    if (currentRule === 'services_def' || parentRule === 'services_def') {
      if (currentRule === 'service_block' || parentRule === 'service_block') {
        return this.getServicePropertyCompletions(tokensBefore);
      }
      return this.getServicesBlockCompletions();
    }
    
    // Use case completions
    if (currentRule === 'use_case' || parentRule === 'use_case') {
      return this.getUseCaseCompletions();
    }
    
    // Domain completions
    if (currentRule === 'domain_block' || parentRule === 'domain_block') {
      return this.getDomainCompletions();
    }
    
    // Exposure completions
    if (currentRule === 'exposure' || parentRule === 'exposure') {
      return this.getExposureCompletions();
    }
    
    // Context-specific completions based on tokens before cursor
    const lastToken = tokensBefore[tokensBefore.length - 1];
    
    if (lastToken === 'language:' || (tokensBefore.includes('language:') && !tokensBefore.slice(-1)[0]?.includes(','))) {
      return this.getLanguageCompletions();
    }
    
    if (lastToken === 'domains:' || (tokensBefore.includes('domains:') && !this.isListComplete(tokensBefore))) {
      return this.getDomainNameCompletions();
    }
    
    return [];
  }

  private isListComplete(tokens: string[]): boolean {
    // Simple heuristic: if the last token is not a comma and doesn't end with comma, list might be complete
    const lastToken = tokens[tokens.length - 1];
    return !!(lastToken && !lastToken.endsWith(','));
  }

  private getTopLevelCompletions(): CompletionItem[] {
    return [
      {
        label: 'arch',
        kind: CompletionItemKind.Keyword,
        detail: 'Architecture block',
        insertText: 'arch {\n    presentation:\n        \n    \n    gateway:\n        \n}',
      },
      {
        label: 'services',
        kind: CompletionItemKind.Keyword,
        detail: 'Services block',
        insertText: 'services {\n    ServiceName {\n        domains: \n        language: \n    }\n}',
      },
      {
        label: 'use_case',
        kind: CompletionItemKind.Keyword,
        detail: 'Use case definition',
        insertText: 'use_case "Use Case Name" {\n    when Actor does action\n        Domain performs action\n}',
      },
      {
        label: 'domain',
        kind: CompletionItemKind.Keyword,
        detail: 'Domain definition',
        insertText: 'domain DomainName {\n    SubDomain1\n    SubDomain2\n}',
      },
      {
        label: 'exposure',
        kind: CompletionItemKind.Keyword,
        detail: 'Exposure definition',
        insertText: 'exposure default {\n    to: Business_User\n    through: APIGateway\n}',
      },
    ];
  }

  private getArchCompletions(): CompletionItem[] {
    return [
      {
        label: 'presentation:',
        kind: CompletionItemKind.Property,
        detail: 'Presentation layer components',
        insertText: 'presentation:\n    WebApp[framework:react, ssl]\n    MobileApp',
      },
      {
        label: 'gateway:',
        kind: CompletionItemKind.Property,
        detail: 'Gateway components',
        insertText: 'gateway:\n    LoadBalancer[ssl:true] > APIGateway[type:nginx]',
      },
    ];
  }

  private getServicesBlockCompletions(): CompletionItem[] {
    return [
      {
        label: 'ServiceName',
        kind: CompletionItemKind.Class,
        detail: 'Service definition',
        insertText: 'ServiceName {\n    domains: \n    language: \n}',
      },
    ];
  }

  private getServicePropertyCompletions(tokensBefore: string[]): CompletionItem[] {
    const completions: CompletionItem[] = [];
    const existingProperties = new Set(tokensBefore.map(t => t.replace(':', '')));
    
    if (!existingProperties.has('domains')) {
      completions.push({
        label: 'domains:',
        kind: CompletionItemKind.Property,
        detail: 'Service domains',
        insertText: 'domains: ',
      });
    }
    
    if (!existingProperties.has('language')) {
      completions.push({
        label: 'language:',
        kind: CompletionItemKind.Property,
        detail: 'Programming language',
        insertText: 'language: ',
      });
    }
    
    if (!existingProperties.has('data-stores')) {
      completions.push({
        label: 'data-stores:',
        kind: CompletionItemKind.Property,
        detail: 'Data storage components',
        insertText: 'data-stores: ',
      });
    }
    
    return completions;
  }

  private getUseCaseCompletions(): CompletionItem[] {
    return [
      {
        label: 'when',
        kind: CompletionItemKind.Keyword,
        detail: 'When clause',
        insertText: 'when Actor does action\n    Domain performs action',
      },
    ];
  }

  private getDomainCompletions(): CompletionItem[] {
    return [
      {
        label: 'SubDomain',
        kind: CompletionItemKind.Class,
        detail: 'Subdomain name',
        insertText: 'SubDomain',
      },
    ];
  }

  private getExposureCompletions(): CompletionItem[] {
    return [
      {
        label: 'to:',
        kind: CompletionItemKind.Property,
        detail: 'Exposure target',
        insertText: 'to: ',
      },
      {
        label: 'through:',
        kind: CompletionItemKind.Property,
        detail: 'Exposure pathway',
        insertText: 'through: ',
      },
    ];
  }

  private getLanguageCompletions(): CompletionItem[] {
    return [
      { label: 'golang', kind: CompletionItemKind.Value, detail: 'Go programming language' },
      { label: 'java', kind: CompletionItemKind.Value, detail: 'Java programming language' },
      { label: 'python', kind: CompletionItemKind.Value, detail: 'Python programming language' },
      { label: 'javascript', kind: CompletionItemKind.Value, detail: 'JavaScript programming language' },
      { label: 'typescript', kind: CompletionItemKind.Value, detail: 'TypeScript programming language' },
      { label: 'csharp', kind: CompletionItemKind.Value, detail: 'C# programming language' },
      { label: 'rust', kind: CompletionItemKind.Value, detail: 'Rust programming language' },
    ];
  }

  private getDomainNameCompletions(): CompletionItem[] {
    // These could be extracted from the current document's domain definitions
    return [
      { label: 'Authentication', kind: CompletionItemKind.Reference, detail: 'Authentication domain' },
      { label: 'Authorization', kind: CompletionItemKind.Reference, detail: 'Authorization domain' },
      { label: 'UserManagement', kind: CompletionItemKind.Reference, detail: 'User Management domain' },
      { label: 'PaymentProcessing', kind: CompletionItemKind.Reference, detail: 'Payment Processing domain' },
      { label: 'OrderManagement', kind: CompletionItemKind.Reference, detail: 'Order Management domain' },
      { label: 'InventoryManagement', kind: CompletionItemKind.Reference, detail: 'Inventory Management domain' },
      { label: 'Notification', kind: CompletionItemKind.Reference, detail: 'Notification domain' },
    ];
  }
}
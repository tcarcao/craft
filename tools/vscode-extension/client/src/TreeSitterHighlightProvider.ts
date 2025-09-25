import * as vscode from 'vscode';
import * as path from 'path';

// Import web-tree-sitter (WASM-based for distribution)
const TreeSitter = require('web-tree-sitter');

/**
 * Tree-sitter based syntax highlighting provider for Craft DSL
 * Provides semantic highlighting using the parsed AST
 */
export class TreeSitterHighlightProvider implements vscode.DocumentSemanticTokensProvider {
  private parser: any = null;
  private language: any = null;
  private initializationPromise: Promise<void>;

  // Define valid semantic token types (no dots allowed)
  private readonly tokenTypes = [
    'craft-flow-keyword',        // use_case, when
    'craft-services-keyword',    // services
    'craft-arch-keyword',        // arch
    'craft-exposure-keyword',    // exposure
    'craft-domain-keyword',      // domain
    'craft-domains-property',    // domains
    'craft-language-property',   // language
    'craft-data-stores-property', // data-stores
    'craft-to-property',         // to
    'craft-through-property',    // through
    'craft-presentation-section', // presentation
    'craft-gateway-section',     // gateway
    'craft-asks-verb',           // asks
    'craft-notifies-verb',       // notifies
    'craft-listens-verb',        // listens
    'craft-service-name',        // Service names
    'craft-domain-name',         // Domain definition names
    'craft-domain-list',         // Domain list values
    'craft-data-store-name',     // Data store names
    'craft-subdomain-name',      // Subdomain names
    'craft-component-name',      // Component names
    'craft-exposure-name',       // Exposure names
    'craft-language-value',      // Language values
    'craft-actor-name',          // Actor names (Business_User)
    'craft-regular-verb',        // Regular verbs (creates, validates)
    'craft-phrase-word',         // Words in phrases (email, format, etc.)
    'craft-modifier-key',        // Modifier keys (framework, ssl, type)
    'craft-modifier-value',      // Modifier values (react, true, nginx)
    'craft-flow-arrow',          // Flow arrows (>)
    'craft-usecase-string',      // Use case names
    'craft-event-string',        // Event strings
    'craft-regular-string',      // Regular strings
    'craft-comment',             // Comments
    'craft-braces',              // { }
    'craft-colon',               // :
    'craft-comma'                // ,
  ];

  private readonly tokenModifiers = [
    'declaration',   // When declaring something
    'definition',    // When defining a block
    'readonly',      // For constants
    'static',        // For keywords
    'deprecated',    // For deprecated syntax
    'modification'   // For property assignments
  ];

  public readonly legend = new vscode.SemanticTokensLegend(this.tokenTypes, this.tokenModifiers);

  constructor() {
    this.initializationPromise = this.initializeParser();
  }

  private async initializeParser(): Promise<void> {
    try {
      console.log('🔄 Initializing Tree-sitter for Craft highlighting...');
      
      // Use the same pattern as the formatter that works
      const { Parser } = TreeSitter;
      
      if (typeof Parser.init === 'function') {
        await Parser.init();
        console.log('✅ Tree-sitter WASM runtime initialized');
        
        // Load the Craft WASM language from extension resources
        const wasmPath = path.join(__dirname, '../../resources/tree-sitter-craft.wasm');
        console.log(`📁 Loading WASM from: ${wasmPath}`);
        
        this.language = await TreeSitter.Language.load(wasmPath);
        console.log('✅ Craft language loaded for highlighting');
        
        // Create parser and set language
        this.parser = new TreeSitter.Parser();
        this.parser.setLanguage(this.language);
        console.log('✅ Tree-sitter Craft highlighter ready');
      } else {
        throw new Error('Parser.init method not found');
      }
      
    } catch (error) {
      console.error('❌ Failed to initialize Tree-sitter highlighter:', error);
      console.error('Stack trace:', error instanceof Error ? error.stack : 'No stack trace');
      console.log('Tree-sitter highlighting will be disabled');
    }
  }

  async provideDocumentSemanticTokens(
    document: vscode.TextDocument,
    token: vscode.CancellationToken
  ): Promise<vscode.SemanticTokens> {
    // Wait for initialization to complete
    await this.initializationPromise;
    
    if (!this.parser || !this.language) {
      console.warn('Tree-sitter parser not initialized - highlighting disabled');
      return new vscode.SemanticTokens(new Uint32Array(0));
    }

    try {
      const text = document.getText();
      console.log(`🔍 Parsing document: ${document.fileName}, length: ${text.length}`);
      
      const tree = this.parser.parse(text);
      console.log(`📊 Parse tree root: ${tree.rootNode.type}, children: ${tree.rootNode.children?.length || 0}`);
      
      const tokens = this.extractSemanticTokens(tree.rootNode, document);
      
      console.log(`🎨 Generated ${tokens.length} semantic tokens for ${document.fileName}`);
      if (tokens.length > 0) {
        console.log('First few tokens:', tokens.slice(0, 3).map(t => ({
          line: t.line,
          char: t.startChar,
          length: t.length,
          type: this.tokenTypes[t.tokenType],
          modifiers: t.tokenModifiers
        })));
      }
      
      return new vscode.SemanticTokens(this.encodeTokens(tokens));
    } catch (error) {
      console.error('Error providing semantic tokens:', error);
      return new vscode.SemanticTokens(new Uint32Array(0));
    }
  }

  private extractSemanticTokens(node: any, document: vscode.TextDocument): Array<{
    line: number;
    startChar: number;
    length: number;
    tokenType: number;
    tokenModifiers: number;
  }> {
    const tokens: Array<{
      line: number;
      startChar: number;
      length: number;
      tokenType: number;
      tokenModifiers: number;
    }> = [];

    this.traverseNode(node, document, tokens);
    
    // Sort tokens by position
    tokens.sort((a, b) => {
      if (a.line !== b.line) return a.line - b.line;
      return a.startChar - b.startChar;
    });

    // Remove overlapping tokens
    return this.removeOverlappingTokens(tokens);
  }

  private traverseNode(
    node: any, 
    document: vscode.TextDocument, 
    tokens: Array<{
      line: number;
      startChar: number;
      length: number;
      tokenType: number;
      tokenModifiers: number;
    }>
  ): void {
    // DEBUG: Disabled for production
    // if (node.type === 'arch_continuation') {
    //   console.log(`🔍 TRAVERSE: ${node.type} | Children: ${node.children?.length || 0}`);
    // }

    // Get node position and validate bounds
    const startPos = document.positionAt(node.startIndex);
    const endPos = document.positionAt(node.endIndex);
    const lineText = document.lineAt(startPos.line).text;
    
    // Ensure we don't exceed line length
    const maxChar = lineText.length;
    const length = Math.min(node.endIndex - node.startIndex, maxChar - startPos.character);
    
    // Skip bounds checking for structural wrapper nodes that don't represent tokens
    const isStructuralNode = node.type === 'arch_continuation' || 
                             node.type === 'scenario_continuation' ||
                             node.type === 'arch_section' ||
                             node.type === 'presentation_section' ||
                             node.type === 'gateway_section' ||
                             node.type === 'arch_component_list' ||
                             node.type === 'source_file';
    
    // Skip invalid or zero-length tokens (but allow structural nodes)
    if (!isStructuralNode && (length <= 0 || startPos.character >= maxChar)) {
      return;
    }

    // Determine token type and modifiers based on node type and context
    const tokenInfo = this.getTokenInfo(node);
    
    if (tokenInfo && !this.isChildOfHigherPriorityNode(node)) {
      tokens.push({
        line: startPos.line,
        startChar: startPos.character,
        length: length,
        tokenType: tokenInfo.type,
        tokenModifiers: tokenInfo.modifiers
      });
    }

    // Only process children if this node doesn't provide tokens itself
    // This prevents overlapping tokens
    if (node.children && (!tokenInfo || this.shouldProcessChildren(node))) {
      for (const child of node.children) {
        this.traverseNode(child, document, tokens);
      }
    }
  }


  private getTokenInfo(node: any): { type: number; modifiers: number } | null {
    const nodeType = node.type;
    const parent = node.parent;
    
    // DEBUG: Disabled for production
    // console.log(`🔍 TOKEN: "${node.text}" (${nodeType}) | Parent: ${parent?.type}`);
    
    // GRANULAR MAPPING USING VALID TOKEN TYPE NAMES
    switch (nodeType) {
      
      // === BLOCK KEYWORDS ===
      case 'services':
        return { type: this.tokenTypes.indexOf('craft-services-keyword'), modifiers: 0 };
      case 'arch':
        return { type: this.tokenTypes.indexOf('craft-arch-keyword'), modifiers: 0 };
      case 'exposure':
        return { type: this.tokenTypes.indexOf('craft-exposure-keyword'), modifiers: 0 };
      case 'domain':
        return { type: this.tokenTypes.indexOf('craft-domain-keyword'), modifiers: 0 };
      case 'use_case':
      case 'when':
        return { type: this.tokenTypes.indexOf('craft-flow-keyword'), modifiers: 0 };

      // === PROPERTY KEYWORDS ===
      case 'domains':
        return { type: this.tokenTypes.indexOf('craft-domains-property'), modifiers: 0 };
      case 'language':
        return { type: this.tokenTypes.indexOf('craft-language-property'), modifiers: 0 };
      case 'data-stores':
        return { type: this.tokenTypes.indexOf('craft-data-stores-property'), modifiers: 0 };
      case 'to':
        return { type: this.tokenTypes.indexOf('craft-to-property'), modifiers: 0 };
      case 'through':
        return { type: this.tokenTypes.indexOf('craft-through-property'), modifiers: 0 };

      // === SECTION KEYWORDS ===
      case 'presentation':
        return { type: this.tokenTypes.indexOf('craft-presentation-section'), modifiers: 0 };
      case 'gateway':
        return { type: this.tokenTypes.indexOf('craft-gateway-section'), modifiers: 0 };

      // === ACTION VERBS ===
      case 'asks':
        return { type: this.tokenTypes.indexOf('craft-asks-verb'), modifiers: 0 };
      case 'notifies':
        return { type: this.tokenTypes.indexOf('craft-notifies-verb'), modifiers: 0 };
      case 'listens':
        return { type: this.tokenTypes.indexOf('craft-listens-verb'), modifiers: 0 };
      case 'creates':
      case 'validates':
      case 'updates':
      case 'deletes':
        return { type: this.tokenTypes.indexOf('craft-regular-verb'), modifiers: 0 };

      // === STRINGS - CONTEXT DEPENDENT ===
      case 'string':
        if (parent?.type === 'use_case_block') {
          return { type: this.tokenTypes.indexOf('craft-usecase-string'), modifiers: 0 };
        }
        if (parent?.type === 'async_action' || parent?.type === 'domain_listener') {
          return { type: this.tokenTypes.indexOf('craft-event-string'), modifiers: 0 };
        }
        return { type: this.tokenTypes.indexOf('craft-regular-string'), modifiers: 0 };

      // === IDENTIFIERS - CONTEXT SPECIFIC WITH FULL GRANULARITY ===
      case 'identifier':
        if (!parent) return null;
        
        // === HYBRID APPROACH - DIRECT PARENT DETECTION ===
        
        // Component names (hybrid) → craft-component-name
        if (parent.type === 'component_name') {
          return { type: this.tokenTypes.indexOf('craft-component-name'), modifiers: 0 };
        }
        
        // Modifier keys (hybrid) → craft-modifier-key
        if (parent.type === 'modifier_key') {
          return { type: this.tokenTypes.indexOf('craft-modifier-key'), modifiers: 0 };
        }
        
        // Modifier values (hybrid) → craft-modifier-value
        if (parent.type === 'modifier_value') {
          return { type: this.tokenTypes.indexOf('craft-modifier-value'), modifiers: 0 };
        }
        
        // Action subjects (hybrid) → craft-service-name (service/domain names in actions)
        if (parent.type === 'action_subject') {
          return { type: this.tokenTypes.indexOf('craft-service-name'), modifiers: 0 };
        }
        
        // Action verbs (hybrid) → craft-regular-verb
        if (parent.type === 'action_verb') {
          return { type: this.tokenTypes.indexOf('craft-regular-verb'), modifiers: 0 };
        }
        
        // Action targets (hybrid) → craft-service-name
        if (parent.type === 'action_target') {
          return { type: this.tokenTypes.indexOf('craft-service-name'), modifiers: 0 };
        }
        
        // Trigger actors (hybrid) → craft-actor-name
        if (parent.type === 'trigger_actor') {
          return { type: this.tokenTypes.indexOf('craft-actor-name'), modifiers: 0 };
        }
        
        // Trigger verbs (hybrid) → craft-regular-verb
        if (parent.type === 'trigger_verb') {
          return { type: this.tokenTypes.indexOf('craft-regular-verb'), modifiers: 0 };
        }
        
        // === FALLBACK TO EXISTING LOGIC ===
        
        // Service names → craft-service-name → entity.name.type.service.domain-dsl
        if (parent.type === 'service_definition') {
          return { type: this.tokenTypes.indexOf('craft-service-name'), modifiers: 0 };
        }
        
        // Domain definition names → craft-domain-name → entity.name.type.domain-name.domain-dsl
        if (parent.type === 'domain_block') {
          return { type: this.tokenTypes.indexOf('craft-domain-name'), modifiers: 0 };
        }
        
        // Exposure names → craft-exposure-name → entity.name.type.exposure.domain-dsl
        if (parent.type === 'exposure_block') {
          return { type: this.tokenTypes.indexOf('craft-exposure-name'), modifiers: 0 };
        }
        
        // Subdomain names → craft-subdomain-name → entity.name.type.subdomain.domain-dsl
        if (parent.type === 'subdomain') {
          return { type: this.tokenTypes.indexOf('craft-subdomain-name'), modifiers: 0 };
        }
        
        // Domain list values → craft-domain-list → entity.name.type.domain.list.domain-dsl
        // Data store list values → craft-data-store-name → entity.name.type.datastore.domain-dsl
        // Exposure property values → craft-actor-name for 'to', craft-component-name for 'through'
        if (parent.type === 'identifier_list') {
          const grandParent = parent.parent;
          if (grandParent?.type === 'domains_property') {
            return { type: this.tokenTypes.indexOf('craft-domain-list'), modifiers: 0 };
          }
          if (grandParent?.type === 'data_stores_property') {
            return { type: this.tokenTypes.indexOf('craft-data-store-name'), modifiers: 0 };
          }
          if (grandParent?.type === 'to_property') {
            return { type: this.tokenTypes.indexOf('craft-actor-name'), modifiers: 0 };
          }
          if (grandParent?.type === 'through_property') {
            return { type: this.tokenTypes.indexOf('craft-component-name'), modifiers: 0 };
          }
        }
        
        // Language values → craft-language-value → entity.name.type.language.domain-dsl
        if (parent.type === 'language_property') {
          return { type: this.tokenTypes.indexOf('craft-language-value'), modifiers: 0 };
        }
        
        // External trigger context - distinguish between actor and verb
        if (parent.type === 'external_trigger') {
          // Use position-based comparison instead of object reference
          const identifiers = parent.children?.filter((child: any) => child.type === 'identifier') || [];
          const currentIndex = identifiers.findIndex((child: any) => 
            child.startIndex === node.startIndex && child.endIndex === node.endIndex
          );
          // console.log(`🔍 TRIGGER DEBUG: "${node.text}" | CurrentIndex: ${currentIndex}`);
          
          if (currentIndex === 0) {
            // First identifier = Actor name → craft-actor-name → entity.name.class.actor.domain-dsl
            return { type: this.tokenTypes.indexOf('craft-actor-name'), modifiers: 0 };
          } else {
            // Subsequent identifiers = Verb → craft-regular-verb → entity.name.function.verb.domain-dsl
            return { type: this.tokenTypes.indexOf('craft-regular-verb'), modifiers: 0 };
          }
        }
        
        // Phrase context - words in action phrases like "email format", "user input"
        if (parent.type === 'phrase') {
          return { type: this.tokenTypes.indexOf('craft-phrase-word'), modifiers: 0 };
        }
        
        // Domain listener context - domain that listens to events
        if (parent.type === 'domain_listener') {
          return { type: this.tokenTypes.indexOf('craft-service-name'), modifiers: 0 };
        }
        
        // Modifier context - modifier keys and values
        if (parent.type === 'modifier') {
          const modifierChildren = parent.children?.filter((child: any) => child.type === 'identifier' || child.type === 'number' || child.type === 'boolean') || [];
          const currentIndex = modifierChildren.findIndex((child: any) => 
            child.startIndex === node.startIndex && child.endIndex === node.endIndex
          );
          
          if (currentIndex === 0) {
            // First identifier = Modifier key (framework, ssl, type)
            return { type: this.tokenTypes.indexOf('craft-modifier-key'), modifiers: 0 };
          } else {
            // Second identifier/number/boolean = Modifier value (react, true, nginx)
            return { type: this.tokenTypes.indexOf('craft-modifier-value'), modifiers: 0 };
          }
        }
        
        // Action context - service names and domain references (multiple action types)
        if (parent.type === 'internal_action' || parent.type === 'sync_action' || parent.type === 'async_action') {
          // In actions like "UserService validates user input data" or "Authentication validates email"
          // We need to distinguish between service names (first identifier) and verbs (second identifier)
          const identifiers = parent.children?.filter((child: any) => child.type === 'identifier') || [];
          const currentIndex = identifiers.findIndex((child: any) => 
            child.startIndex === node.startIndex && child.endIndex === node.endIndex
          );
          console.log(`🔍 ACTION DEBUG: "${node.text}" | Parent: ${parent.type} | Action identifiers: [${identifiers.map((id: any) => id.text).join(', ')}] | CurrentIndex: ${currentIndex}`);
          
          if (currentIndex === 0) {
            // First identifier = Service/Domain name → craft-service-name or craft-domain-name
            // For now, let's use service name color (we can refine this later)
            return { type: this.tokenTypes.indexOf('craft-service-name'), modifiers: 0 };
          } else if (currentIndex === 1) {
            // Second identifier = Verb → craft-regular-verb
            return { type: this.tokenTypes.indexOf('craft-regular-verb'), modifiers: 0 };
          }
          // Note: Other identifiers in actions will be handled by phrase context above
        }
        
        return null; // Don't color unrecognized identifiers

      // === COMMENTS ===
      case 'comment':
        return { type: this.tokenTypes.indexOf('craft-comment'), modifiers: 0 };

      // === MODIFIER VALUES (NON-IDENTIFIERS) ===
      case 'number':
      case 'boolean':
        // Hybrid approach - direct parent detection
        if (parent?.type === 'modifier_value') {
          return { type: this.tokenTypes.indexOf('craft-modifier-value'), modifiers: 0 };
        }
        // Fallback to existing logic
        if (parent?.type === 'modifier') {
          return { type: this.tokenTypes.indexOf('craft-modifier-value'), modifiers: 0 };
        }
        return null;

      // === PUNCTUATION ===
      case '{':
      case '}':
        return { type: this.tokenTypes.indexOf('craft-braces'), modifiers: 0 };
      case ':':
        return { type: this.tokenTypes.indexOf('craft-colon'), modifiers: 0 };
      case ',':
        return { type: this.tokenTypes.indexOf('craft-comma'), modifiers: 0 };
      case '>':
        return { type: this.tokenTypes.indexOf('craft-flow-arrow'), modifiers: 0 };

      default:
        return null;
    }
  }


  private encodeTokens(tokens: Array<{
    line: number;
    startChar: number;
    length: number;
    tokenType: number;
    tokenModifiers: number;
  }>): Uint32Array {
    const data: number[] = [];
    let prevLine = 0;
    let prevChar = 0;

    for (const token of tokens) {
      const deltaLine = token.line - prevLine;
      const deltaChar = deltaLine === 0 ? token.startChar - prevChar : token.startChar;

      data.push(deltaLine);
      data.push(deltaChar);
      data.push(token.length);
      data.push(token.tokenType);
      data.push(token.tokenModifiers);

      prevLine = token.line;
      prevChar = token.startChar;
    }

    return new Uint32Array(data);
  }

  private isChildOfHigherPriorityNode(_node: any): boolean {
    return false;
  }

  private shouldProcessChildren(_node: any): boolean {
    return true;
  }

  private removeOverlappingTokens(tokens: Array<{
    line: number;
    startChar: number;
    length: number;
    tokenType: number;
    tokenModifiers: number;
  }>): Array<{
    line: number;
    startChar: number;
    length: number;
    tokenType: number;
    tokenModifiers: number;
  }> {
    if (tokens.length === 0) return tokens;

    const filtered: typeof tokens = [];
    let lastToken = tokens[0];
    filtered.push(lastToken);

    for (let i = 1; i < tokens.length; i++) {
      const current = tokens[i];
      const lastEnd = lastToken.startChar + lastToken.length;

      if (current.line === lastToken.line && current.startChar < lastEnd) {
        if (current.length < lastToken.length) {
          filtered[filtered.length - 1] = current;
          lastToken = current;
        }
      } else {
        filtered.push(current);
        lastToken = current;
      }
    }

    return filtered;
  }
}
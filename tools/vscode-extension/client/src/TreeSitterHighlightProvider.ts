import { DocumentSemanticTokensProvider, SemanticTokensLegend, SemanticTokens, CancellationToken, TextDocument } from 'vscode';
import * as path from 'path';

// Import web-tree-sitter (WASM-based for distribution)
const TreeSitter = require('web-tree-sitter');

/**
 * Tree-sitter based syntax highlighting provider for Craft DSL
 * Provides semantic highlighting using the parsed AST
 */
export class TreeSitterHighlightProvider implements DocumentSemanticTokensProvider {
  private parser: any = null;
  private language: any = null;
  private query: any = null;
  private initializationPromise: Promise<void>;

  // Use tree-sitter capture names directly as token types (replace dots with hyphens for VS Code)
  private readonly tokenTypes = [
    'craft-comment',
    'craft-services-keyword',
    'craft-arch-keyword',
    'craft-exposure-keyword',
    'craft-domain-keyword',
    'craft-actors-keyword',
    'craft-actor-keyword',
    'craft-flow-keyword',
    'craft-domains-property',
    'craft-language-property',
    'craft-data-stores-property',
    'craft-to-property',
    'craft-through-property',
    'craft-presentation-section',
    'craft-gateway-section',
    'craft-asks-verb',
    'craft-notifies-verb',
    'craft-listens-verb',
    'craft-returns-verb',
    'craft-regular-verb',
    'craft-actor-type',
    'craft-connector-word',
    'craft-usecase-string',
    'craft-event-string',
    'craft-regular-string',
    'craft-component-name',
    'craft-modifier-key',
    'craft-modifier-value',
    'craft-service-name',
    'craft-domain-name',
    'craft-exposure-name',
    'craft-subdomain-name',
    'craft-actor-definition',
    'craft-domain-list',
    'craft-data-store-name',
    'craft-actor-name',
    'craft-language-value',
    'craft-phrase-word',
    'craft-braces',
    'craft-colon',
    'craft-comma',
    'craft-flow-arrow'
  ];

  private readonly tokenModifiers = [
    'declaration',   // When declaring something
    'definition',    // When defining a block
    'readonly',      // For constants
    'static',        // For keywords
    'deprecated',    // For deprecated syntax
    'modification'   // For property assignments
  ];

  public readonly legend = new SemanticTokensLegend(this.tokenTypes, this.tokenModifiers);

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
        
        // Load highlights query
        try {
          const highlightsPath = path.join(__dirname, '../../resources/tree-sitter-craft/queries/highlights.scm');
          const fs = require('fs');
          const highlightsQuery = fs.readFileSync(highlightsPath, 'utf8');
          this.query = new TreeSitter.Query(this.language, highlightsQuery);
          console.log('✅ Tree-sitter highlights query loaded');
        } catch (queryError) {
          console.warn('⚠️ Could not load highlights query, falling back to manual traversal:', queryError);
        }
        
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
    document: TextDocument,
    token: CancellationToken
  ): Promise<SemanticTokens> {
    // Wait for initialization to complete
    await this.initializationPromise;
    
    if (!this.parser || !this.language) {
      console.warn('Tree-sitter parser not initialized - highlighting disabled');
      return new SemanticTokens(new Uint32Array(0));
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
      
      return new SemanticTokens(this.encodeTokens(tokens));
    } catch (error) {
      console.error('Error providing semantic tokens:', error);
      return new SemanticTokens(new Uint32Array(0));
    }
  }

  private extractSemanticTokens(node: any, document: TextDocument): Array<{
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

    // Use query-based approach only
    if (this.query) {
      const captures = this.query.captures(node);
      console.log(`🔍 Query-based highlighting found ${captures.length} captures`);
      
      for (const capture of captures) {
        const captureNode = capture.node;
        const captureName = capture.name;
        
        // Convert capture name to token type (replace dots with hyphens)
        const tokenTypeName = captureName.replace(/\./g, '-');
        const tokenType = this.tokenTypes.indexOf(tokenTypeName);
        if (tokenType !== -1) {
          const startPos = document.positionAt(captureNode.startIndex);
          const endPos = document.positionAt(captureNode.endIndex);
          const lineText = document.lineAt(startPos.line).text;
          const maxChar = lineText.length;
          const length = Math.min(captureNode.endIndex - captureNode.startIndex, maxChar - startPos.character);
          
          if (length > 0 && startPos.character < maxChar) {
            tokens.push({
              line: startPos.line,
              startChar: startPos.character,
              length: length,
              tokenType: tokenType,
              tokenModifiers: 0
            });
          }
        }
      }
      
      console.log(`🎨 Query-based highlighting generated ${tokens.length} tokens`);
    } else {
      console.warn('⚠️ Tree-sitter query not available - no highlighting');
    }
    
    // Sort tokens by position
    tokens.sort((a, b) => {
      if (a.line !== b.line) return a.line - b.line;
      return a.startChar - b.startChar;
    });

    // Remove overlapping tokens
    return this.removeOverlappingTokens(tokens);
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
import { TextDocument } from 'vscode-languageserver-textdocument';
import { CompletionItem, CompletionItemKind, Position } from 'vscode-languageserver/node';
import * as path from 'path';
import { ActorDefinition } from '../../../shared/lib/types/domain-extraction';

// Import web-tree-sitter (WASM-based for distribution)
const TreeSitter = require('web-tree-sitter');

export interface TreeSitterCompletionContext {
  nodeType: string;
  parentNodeType?: string;
  ancestorTypes: string[];
  currentText: string;
  isAtStartOfLine: boolean;
  indentLevel: number;
  cursorOffset: number;
}

/**
 * Tree-sitter based completion provider for Craft DSL
 * Provides context-aware completions using the parsed AST
 */
export class TreeSitterCompletionProvider {
  private parser: any = null;
  private language: any = null;
  private initializationPromise: Promise<void>;
  private actorProvider?: () => Promise<ActorDefinition[]>;

  constructor(actorProvider?: () => Promise<ActorDefinition[]>) {
    this.actorProvider = actorProvider;
    this.initializationPromise = this.initializeParser();
  }

  private async initializeParser(): Promise<void> {
    try {
      const { Parser } = TreeSitter;
      
      if (typeof Parser.init === 'function') {
        await Parser.init();
        console.log('✅ Tree-sitter WASM runtime initialized for completion');
        
        // Load the Craft WASM language from resources directory
        const wasmPath = path.join(__dirname, '../../../resources/tree-sitter-craft.wasm');
        this.language = await TreeSitter.Language.load(wasmPath);
        console.log('✅ Craft language loaded for completion');
        
        // Create parser and set language
        this.parser = new TreeSitter.Parser();
        this.parser.setLanguage(this.language);
        console.log('✅ Tree-sitter Craft completion provider ready');
      } else {
        throw new Error('Parser.init method not found');
      }
      
    } catch (error) {
      console.error('Failed to initialize Tree-sitter completion provider:', error);
      console.log('Tree-sitter completion will be disabled');
    }
  }

  async getCompletions(document: TextDocument, position: Position): Promise<CompletionItem[]> {
    // Wait for initialization to complete
    await this.initializationPromise;
    
    if (!this.parser || !this.language) {
      console.warn('Tree-sitter parser not initialized - completion disabled');
      return [];
    }

    try {
      const context = this.analyzeCompletionContext(document, position);
      console.log(`TreeSitter completion context: node=${context.nodeType}, ancestors=[${context.ancestorTypes.join(', ')}], text="${context.currentText}"`);
      
      const completions = await this.getCompletionsForContext(context);
      console.log(`TreeSitter generated ${completions.length} completions`);
      
      return completions;
    } catch (error) {
      console.error('Error getting Tree-sitter completions:', error);
      return [];
    }
  }

  private analyzeCompletionContext(document: TextDocument, position: Position): TreeSitterCompletionContext {
    const text = document.getText();
    const cursorOffset = document.offsetAt(position);
    
    // Parse the document to get AST
    const tree = this.parser.parse(text);
    const rootNode = tree.rootNode;
    
    // Find the node at cursor position - try a small range around cursor for better detection
    let nodeAtCursor = rootNode.descendantForIndex(cursorOffset, cursorOffset);
    
    // Get current line information for better context analysis
    const lines = text.split('\n');
    const currentLine = lines[position.line] || '';
    const textBeforeCursor = currentLine.substring(0, position.character);
    const isAtStartOfLine = textBeforeCursor.trim().length === 0;
    const indentLevel = this.calculateIndentLevel(textBeforeCursor);

    // Build complete ancestor chain from AST (Tree-sitter approach)
    const ancestorTypes: string[] = [];
    let current = nodeAtCursor;
    
    // Always traverse up the AST to get complete context
    while (current && current.parent) {
      ancestorTypes.push(current.type);
      current = current.parent;
    }
    // Add the root node type
    if (current) {
      ancestorTypes.push(current.type);
    }
    
    // For incomplete syntax at cursor position, also check the parent of cursor position
    if (nodeAtCursor.type === 'ERROR' || nodeAtCursor.isMissing) {
      // Look for a better context node by checking slightly before cursor
      if (cursorOffset > 0) {
        const nodeBefore = rootNode.descendantForIndex(Math.max(0, cursorOffset - 1), Math.max(0, cursorOffset - 1));
        if (nodeBefore && nodeBefore.type !== 'ERROR' && nodeBefore !== nodeAtCursor) {
          // Add ancestors from the node before cursor as well
          let beforeCurrent = nodeBefore;
          while (beforeCurrent && beforeCurrent.parent) {
            if (!ancestorTypes.includes(beforeCurrent.type)) {
              ancestorTypes.push(beforeCurrent.type);
            }
            beforeCurrent = beforeCurrent.parent;
          }
        }
      }
    }

    console.log(`Completion context: node=${nodeAtCursor.type}, ancestors=[${ancestorTypes.join(', ')}], indent=${indentLevel}, text="${textBeforeCursor}"`);

    return {
      nodeType: nodeAtCursor.type,
      parentNodeType: nodeAtCursor.parent?.type,
      ancestorTypes,
      currentText: textBeforeCursor, // Use the text before cursor for better context
      isAtStartOfLine,
      indentLevel,
      cursorOffset
    };
  }

  private analyzeTextStructure(text: string, position: Position): { inferredAncestors: string[] } {
    const lines = text.split('\n');
    const currentLineIndex = position.line;
    const inferredAncestors: string[] = [];
    
    // Look backwards from current position to find context
    for (let i = currentLineIndex; i >= 0; i--) {
      const line = lines[i].trim();
      
      // Check for block openings
      if (line.includes('services {')) {
        inferredAncestors.push('services_block');
        break;
      } else if (line.match(/^[A-Z]\w*\s*{/)) {
        // Service definition pattern
        inferredAncestors.push('service_definition');
        if (!inferredAncestors.includes('services_block')) {
          inferredAncestors.push('services_block');
        }
        break;
      } else if (line.startsWith('use_case')) {
        inferredAncestors.push('use_case_block');
        break;
      } else if (line.startsWith('when')) {
        inferredAncestors.push('when_clause');
        if (!inferredAncestors.includes('use_case_block')) {
          inferredAncestors.push('use_case_block');
        }
        break;
      } else if (line.includes('arch {')) {
        inferredAncestors.push('arch_block');
        break;
      } else if (line.startsWith('domain')) {
        inferredAncestors.push('domain_block');
        break;
      } else if (line.startsWith('exposure')) {
        inferredAncestors.push('exposure_block');
        break;
      }
    }
    
    return { inferredAncestors };
  }

  private calculateIndentLevel(textBeforeCursor: string): number {
    const match = textBeforeCursor.match(/^(\\s*)/);
    if (match) {
      const spaces = match[1];
      return Math.floor(spaces.length / 4); // Assuming 4-space indentation
    }
    return 0;
  }

  private async getCompletionsForContext(context: TreeSitterCompletionContext): Promise<CompletionItem[]> {
    const completions: CompletionItem[] = [];

    // AST-based context detection using actual node types
    const currentNodeType = context.nodeType;
    const parentNodeType = context.parentNodeType;
    
    // Check ancestors for specific contexts
    const isInArchBlock = context.ancestorTypes.includes('arch_block');
    const isInUseCaseBlock = context.ancestorTypes.includes('use_case_block');
    const isInServicesBlock = context.ancestorTypes.includes('services_block');
    const isInExposureBlock = context.ancestorTypes.includes('exposure_block');
    const isInActorsBlock = context.ancestorTypes.includes('actors_block');
    const isInPresentationSection = context.ancestorTypes.includes('presentation_section');
    const isInGatewaySection = context.ancestorTypes.includes('gateway_section');

    // Always show top-level completions unless clearly in a specific block context
    const isInSpecificBlock = isInArchBlock || isInUseCaseBlock || isInServicesBlock || isInExposureBlock || isInActorsBlock;
    const shouldShowTopLevel = !isInSpecificBlock || 
                               currentNodeType === 'ERROR' || 
                               context.currentText.trim().length === 0 ||
                               context.isAtStartOfLine;
    
    console.log(`Top-level check: node=${currentNodeType}, inSpecificBlock=${isInSpecificBlock}, shouldShow=${shouldShowTopLevel}`);
    
    if (shouldShowTopLevel) {
      completions.push(
        this.createCompletionItem('actors', 'actors {\\n    $1\\n}', 'Actors definition block', CompletionItemKind.Module),
        this.createCompletionItem('services', 'services {\\n    $1\\n}', 'Services definition block', CompletionItemKind.Module),
        this.createCompletionItem('use_case', 'use_case "$1" {\\n    $2\\n}', 'Use case definition', CompletionItemKind.Class),
        this.createCompletionItem('domain', 'domain $1 {\\n    $2\\n}', 'Domain definition', CompletionItemKind.Module),
        this.createCompletionItem('arch', 'arch {\\n    $1\\n}', 'Architecture definition', CompletionItemKind.Module),
        this.createCompletionItem('exposure', 'exposure $1 {\\n    $2\\n}', 'Exposure definition', CompletionItemKind.Interface),
        this.createCompletionItem('actor', 'actor $1 $2', 'Individual actor definition', CompletionItemKind.Class)
      );
    }

    // Services block context
    if (this.isInServicesBlock(context)) {
      completions.push(
        this.createCompletionItem('ServiceName', '$1 {\\n    domains: $2\\n    language: $3\\n}', 'Service definition', CompletionItemKind.Class)
      );
    }

    // Actors block context
    if (isInActorsBlock) {
      completions.push(
        this.createCompletionItem('user', 'user $1', 'User actor definition', CompletionItemKind.Class),
        this.createCompletionItem('system', 'system $1', 'System actor definition', CompletionItemKind.Class),
        this.createCompletionItem('service', 'service $1', 'Service actor definition', CompletionItemKind.Class)
      );
    }

    // Service properties context
    if (this.isInServiceDefinition(context)) {
      completions.push(
        this.createCompletionItem('domains', 'domains: $1', 'Service domains property', CompletionItemKind.Property),
        this.createCompletionItem('language', 'language: $1', 'Service language property', CompletionItemKind.Property),
        this.createCompletionItem('data-stores', 'data-stores: $1', 'Service data stores property', CompletionItemKind.Property),
        this.createCompletionItem('deployment', 'deployment: $1', 'Service deployment property', CompletionItemKind.Property)
      );
    }

    // Use case context - AST-based detection
    if (isInUseCaseBlock) {
      completions.push(
        this.createCompletionItem('when', 'when $1\\n    $2', 'When clause for use case scenario', CompletionItemKind.Event)
      );
    }

    // When clause context (for triggers and actions)
    if (this.isInWhenClause(context)) {
      // Trigger completions
      if (this.needsTrigger(context)) {
        completions.push(
          this.createCompletionItem('User', 'User $1', 'External user trigger', CompletionItemKind.Event),
          this.createCompletionItem('System', 'System $1', 'External system trigger', CompletionItemKind.Event),
          this.createCompletionItem('CRON', 'CRON "$1"', 'Scheduled trigger', CompletionItemKind.Event),
          this.createCompletionItem('listens', 'listens $1', 'Domain event listener', CompletionItemKind.Event)
        );

        // Add actor name completions for triggers
        const actorCompletions = await this.getActorCompletions();
        completions.push(...actorCompletions.map(actor => 
          this.createCompletionItem(
            actor.name, 
            `${actor.name} $1`, 
            `${actor.type} actor trigger`, 
            CompletionItemKind.Event
          )
        ));
      }
      
      // Action completions
      if (this.needsAction(context)) {
        completions.push(
          this.createCompletionItem('Domain', '$1 $2', 'Domain action', CompletionItemKind.Method),
          this.createCompletionItem('notifies', 'notifies $1', 'Notification action', CompletionItemKind.Method),
          this.createCompletionItem('asks', 'asks $1', 'External system call', CompletionItemKind.Method)
        );
      }
    }

    // Architecture context - AST-based detection
    if (isInArchBlock) {
      if (isInPresentationSection || isInGatewaySection) {
        // Inside arch sections - provide component completions
        completions.push(
          this.createCompletionItem('WebApp', 'WebApp[framework:$1, ssl:$2]', 'Web application component', CompletionItemKind.Class),
          this.createCompletionItem('MobileApp', 'MobileApp[platform:$1]', 'Mobile application component', CompletionItemKind.Class),
          this.createCompletionItem('LoadBalancer', 'LoadBalancer[ssl:$1]', 'Load balancer component', CompletionItemKind.Class),
          this.createCompletionItem('APIGateway', 'APIGateway[type:$1]', 'API gateway component', CompletionItemKind.Class),
          this.createCompletionItem('Database', 'Database[type:$1]', 'Database component', CompletionItemKind.Class),
          this.createCompletionItem('Cache', 'Cache[type:redis]', 'Cache component', CompletionItemKind.Class),
          this.createCompletionItem('MessageQueue', 'MessageQueue[type:$1]', 'Message queue component', CompletionItemKind.Class)
        );
      } else {
        // Inside arch block but not in sections - provide section completions
        completions.push(
          this.createCompletionItem('presentation', 'presentation:\\n    $1', 'Presentation layer', CompletionItemKind.Module),
          this.createCompletionItem('gateway', 'gateway:\\n    $1', 'Gateway layer', CompletionItemKind.Module)
        );
      }
    }

    // Domain context
    if (this.isInDomainBlock(context)) {
      completions.push(
        this.createCompletionItem('subdomain', '$1', 'Subdomain definition', CompletionItemKind.Class)
      );
    }

    // Exposure context - AST-based detection
    if (isInExposureBlock) {
      completions.push(
        this.createCompletionItem('to', 'to: $1', 'Target user/actor', CompletionItemKind.Property),
        this.createCompletionItem('through', 'through: $1', 'Through component/gateway', CompletionItemKind.Property)
      );

      // Add actor name completions for exposure "to" property
      if (this.isInToProperty(context)) {
        const actorCompletions = await this.getActorCompletions();
        completions.push(...actorCompletions.map(actor => 
          this.createCompletionItem(
            actor.name, 
            actor.name, 
            `${actor.type} actor for exposure`, 
            CompletionItemKind.Value
          )
        ));
      }
    }

    // Language keywords and common values
    if (this.isInPropertyValue(context)) {
      completions.push(
        ...this.getLanguageCompletions(),
        ...this.getDeploymentCompletions(),
        ...this.getBooleanCompletions()
      );
    }

    // Add actor name completions in general contexts where actors might be referenced
    if (this.shouldShowActorNames(context)) {
      const actorCompletions = await this.getActorCompletions();
      completions.push(...actorCompletions.map(actor => 
        this.createCompletionItem(
          actor.name, 
          actor.name, 
          `${actor.type} actor`, 
          CompletionItemKind.Value
        )
      ));
    }

    return completions;
  }

  // AST-based context detection methods (simplified)
  private isInServicesBlock(context: TreeSitterCompletionContext): boolean {
    return context.ancestorTypes.includes('services_block') && 
           !context.ancestorTypes.includes('service_definition');
  }

  private isInServiceDefinition(context: TreeSitterCompletionContext): boolean {
    return context.ancestorTypes.includes('service_definition') ||
           context.ancestorTypes.includes('service_block');
  }

  private isInWhenClause(context: TreeSitterCompletionContext): boolean {
    return context.ancestorTypes.includes('when_clause') ||
           context.ancestorTypes.includes('scenario') ||
           context.ancestorTypes.includes('scenario_continuation');
  }

  private isInDomainBlock(context: TreeSitterCompletionContext): boolean {
    return context.ancestorTypes.includes('domain_block');
  }

  private isInPropertyValue(context: TreeSitterCompletionContext): boolean {
    return context.nodeType === 'string' || 
           context.parentNodeType === 'service_property' ||
           context.parentNodeType === 'exposure_property';
  }

  private isInToProperty(context: TreeSitterCompletionContext): boolean {
    return context.ancestorTypes.includes('to_property') ||
           (context.parentNodeType === 'to_property') ||
           (context.currentText.includes('to:'));
  }

  private shouldShowActorNames(context: TreeSitterCompletionContext): boolean {
    // Show actor names in contexts where they might be referenced
    return (
      // In exposure "to" property
      this.isInToProperty(context) ||
      // In when clauses for triggers
      this.isInWhenClause(context) ||
      // When typing identifiers after keywords that expect actor names
      (context.currentText.includes('User ') || context.currentText.includes('System ')) ||
      // In general identifier contexts where not in specific blocks
      (context.nodeType === 'identifier' && !context.ancestorTypes.includes('service_definition')) ||
      // When cursor is at start and we have empty context
      (context.isAtStartOfLine && context.currentText.trim().length === 0)
    );
  }

  private async getActorCompletions(): Promise<ActorDefinition[]> {
    if (!this.actorProvider) {
      return [];
    }
    
    try {
      return await this.actorProvider();
    } catch (error) {
      console.error('Error getting actor definitions for completion:', error);
      return [];
    }
  }

  private needsTrigger(context: TreeSitterCompletionContext): boolean {
    // Check if we're in a when clause context and need a trigger
    if (!context.ancestorTypes.includes('when_clause')) {
      return false;
    }
    
    // Get the text we're analyzing from the context logging
    const textWithCursor = context.currentText;
    
    // For test case: "    when " - we need triggers after "when "
    if (textWithCursor.includes('when ') && textWithCursor.trim().endsWith('when')) {
      return true;
    }
    
    // Check if line contains "when " but no trigger keywords
    const hasTrigger = textWithCursor.includes('User') || 
                      textWithCursor.includes('System') ||
                      textWithCursor.includes('CRON') ||
                      textWithCursor.includes('listens');
    
    return textWithCursor.includes('when') && !hasTrigger;
  }

  private needsAction(context: TreeSitterCompletionContext): boolean {
    // Check if we're in a when clause and likely need an action
    return context.ancestorTypes.includes('when_clause') && 
           context.indentLevel >= 2;
  }

  // Completion item generators
  private getLanguageCompletions(): CompletionItem[] {
    const languages = ['java', 'kotlin', 'python', 'javascript', 'typescript', 'go', 'rust', 'csharp'];
    return languages.map(lang => 
      this.createCompletionItem(lang, lang, `${lang} programming language`, CompletionItemKind.Value)
    );
  }

  private getDeploymentCompletions(): CompletionItem[] {
    const deployments = ['blue_green', 'rolling', 'canary'];
    return deployments.map(deployment => 
      this.createCompletionItem(deployment, deployment, `${deployment} deployment strategy`, CompletionItemKind.Value)
    );
  }

  private getBooleanCompletions(): CompletionItem[] {
    return [
      this.createCompletionItem('true', 'true', 'Boolean true value', CompletionItemKind.Value),
      this.createCompletionItem('false', 'false', 'Boolean false value', CompletionItemKind.Value)
    ];
  }

  private createCompletionItem(
    label: string, 
    insertText: string, 
    detail: string, 
    kind: CompletionItemKind
  ): CompletionItem {
    return {
      label,
      kind,
      detail,
      insertText,
      insertTextFormat: 2 // Snippet format
    };
  }
}
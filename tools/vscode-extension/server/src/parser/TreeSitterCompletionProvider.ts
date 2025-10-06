import { TextDocument } from 'vscode-languageserver-textdocument';
import { CompletionItem, CompletionItemKind, Position } from 'vscode-languageserver/node.js';
import Parser from 'tree-sitter';
import Craft from 'tree-sitter-craft';
import { Logger } from '../utils/Logger.js';

/**
 * Simple Tree-sitter based completion provider for Craft DSL
 * Only provides root-level completions to avoid interference
 */
export class TreeSitterCompletionProvider {
  private parser: any = null;
  private initializationPromise: Promise<void>;

  constructor() {
    this.initializationPromise = this.initializeParser();
  }

  private async initializeParser(): Promise<void> {
    try {
      // Use native Node.js tree-sitter with tree-sitter-craft npm package
      this.parser = new Parser();
      this.parser.setLanguage(Craft);
      
      Logger.info('✅ TreeSitterCompletionProvider Native tree-sitter Craft completion provider ready');
      Logger.info('✅ TreeSitterCompletionProvider Using native Node.js performance instead of WASM');
      
    } catch (error) {
      Logger.error('Failed to initialize native Tree-sitter completion provider:', error);
      Logger.warn('Tree-sitter completion will be disabled');
    }
  }

  async getCompletions(document: TextDocument, position: Position): Promise<CompletionItem[]> {
    // Wait for initialization to complete
    await this.initializationPromise;
    
    if (!this.parser) {
      Logger.warn('Tree-sitter parser not initialized - completion disabled');
      return [];
    }

    try {
      // Simple root-level check only
      const text = document.getText();
      const lines = text.split('\n');
      const currentLine = lines[position.line] || '';
      const textBeforeCursor = currentLine.substring(0, position.character);
      
      // Only suggest at document root when line is empty or starts with whitespace
      if (textBeforeCursor.trim().length === 0) {
        Logger.debug('TreeSitter: Providing root-level completions');
        return this.getRootCompletions();
      }
      
      // No completions in other contexts for now
      return [];
    } catch (error) {
      Logger.error('Error getting Tree-sitter completions:', error);
      return [];
    }
  }

  private getRootCompletions(): CompletionItem[] {
    return [
      {
        label: 'actors',
        kind: CompletionItemKind.Module,
        detail: 'Actors definition block',
        insertText: 'actors {\n    $1\n}',
        insertTextFormat: 2 // Snippet format
      },
      {
        label: 'services',
        kind: CompletionItemKind.Module,
        detail: 'Services definition block',
        insertText: 'services {\n    $1\n}',
        insertTextFormat: 2
      },
      {
        label: 'use_case',
        kind: CompletionItemKind.Class,
        detail: 'Use case definition',
        insertText: 'use_case "$1" {\n    $2\n}',
        insertTextFormat: 2
      },
      {
        label: 'domain',
        kind: CompletionItemKind.Module,
        detail: 'Domain definition',
        insertText: 'domain $1 {\n    $2\n}',
        insertTextFormat: 2
      },
      {
        label: 'arch',
        kind: CompletionItemKind.Module,
        detail: 'Architecture definition',
        insertText: 'arch {\n    $1\n}',
        insertTextFormat: 2
      },
      {
        label: 'exposure',
        kind: CompletionItemKind.Interface,
        detail: 'Exposure definition',
        insertText: 'exposure $1 {\n    $2\n}',
        insertTextFormat: 2
      }
    ];
  }
}
// server/src/TreeSitterDiagnosticProvider.ts
import {
    Diagnostic,
    DiagnosticSeverity,
    Position,
    Range
} from 'vscode-languageserver/node.js';
import { TextDocument } from 'vscode-languageserver-textdocument';

// Import native tree-sitter with tree-sitter-craft npm package
import Parser from 'tree-sitter';
import Craft from 'tree-sitter-craft';
import { Logger } from './utils/Logger.js';

export class TreeSitterDiagnosticProvider {
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
            
            Logger.info('Native Tree Sitter diagnostic provider initialized successfully');
            Logger.info('Using native Node.js performance instead of WASM');
        } catch (error) {
            Logger.error('Failed to initialize native Tree Sitter diagnostic provider:', error);
        }
    }

    public async getDiagnostics(document: TextDocument): Promise<Diagnostic[]> {
        // Ensure parser is initialized
        await this.initializationPromise;
        
        if (!this.parser) {
            return [];
        }

        const text = document.getText();
        const tree = this.parser.parse(text);
        const diagnostics: Diagnostic[] = [];

        // Find all ERROR and MISSING nodes
        this.findErrors(tree.rootNode, diagnostics, document);

        return diagnostics;
    }

    private findErrors(node: any, diagnostics: Diagnostic[], document: TextDocument): void {
        // Check if this node is an error
        if (node.type === 'ERROR') {
            const range = Range.create(
                document.positionAt(node.startIndex),
                document.positionAt(node.endIndex)
            );

            let message = 'Syntax error';
            
            // Provide context-specific error messages
            if (node.parent) {
                message = this.getContextualErrorMessage(node);
            }

            diagnostics.push(Diagnostic.create(
                range,
                message,
                DiagnosticSeverity.Error,
                undefined,
                'tree-sitter-craft'
            ));
        }

        // Check for missing nodes
        if (node.isMissing) {
            const range = Range.create(
                document.positionAt(node.startIndex),
                document.positionAt(node.startIndex)
            );

            const message = `Missing ${node.type}`;

            diagnostics.push(Diagnostic.create(
                range,
                message,
                DiagnosticSeverity.Error,
                undefined,
                'tree-sitter-craft'
            ));
        }

        // Recursively check children
        for (let i = 0; i < node.childCount; i++) {
            this.findErrors(node.child(i), diagnostics, document);
        }
    }

    private getContextualErrorMessage(errorNode: any): string {
        const parent = errorNode.parent;
        
        if (!parent) {
            return 'Syntax error';
        }

        switch (parent.type) {
            case 'arch_block':
                return 'Invalid architecture syntax. Expected "presentation:" or "gateway:" section';
            
            case 'use_case_block':
                return 'Invalid use case syntax. Expected "when" clause';
            
            case 'services_block':
                return 'Invalid services syntax. Expected service definition';
            
            case 'service_definition':
            case 'service_block':
                return 'Invalid service property. Expected: domains, language, data-stores, or deployment';
            
            case 'arch_component_list':
                return 'Invalid component syntax. Expected component name or component flow (with >)';
            
            case 'component_modifiers':
                return 'Invalid component modifier. Expected format: [key:value, key:value]';
            
            case 'modifier_list':
                return 'Invalid modifier format. Expected: key:value';
            
            case 'when_clause':
                return 'Invalid trigger syntax. Expected actor-verb-object or domain listens "event"';
            
            case 'external_trigger':
                return 'Invalid external trigger. Expected format: Actor verb [object]';
            
            case 'domain_listener':
                return 'Invalid domain listener. Expected format: Domain listens "Event"';
            
            case 'sync_action':
                return 'Invalid sync action. Expected format: Domain asks Domain [to] do something';
            
            case 'async_action':
                return 'Invalid async action. Expected format: Domain notifies "Event"';
            
            case 'internal_action':
                return 'Invalid internal action. Expected format: Domain verb phrase';
            
            default:
                return `Syntax error in ${parent.type}`;
        }
    }
}
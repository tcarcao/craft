// server/src/DomainExtractor.ts
import { TextDocument } from 'vscode-languageserver-textdocument';
import { Parser } from './CraftParser.js';
import { DslContext} from './generated/CraftParser.js';
import { ExtractionResult } from '../../../shared/lib/types/domain-extraction.js';
import { DomainVisitor } from './DomainVisitor.js';
import { Logger } from '../utils/Logger.js';


export class DomainExtractor {
    private parser: Parser;

    constructor() {
        this.parser = new Parser();
    }

	extractFromDocument(document: TextDocument): ExtractionResult {
		const text = document.getText();
		return this.extractFromText(text, document.uri);
	}

    extractFromText(text: string, fileUri: string): ExtractionResult {
        const result = this.parser.parse(text);

        if (!result.success || !result.tree) {
            Logger.warn('Parse failed, returning empty result');
            return {
                domains: [],
                useCases: [],
                fileResults: [],
                serviceDefinitions: [],
                domainDefinitions: [],
                actorDefinitions: []
            };
        }

        return this.extractFromParseTree(result.tree, fileUri);
    }

    private extractFromParseTree(tree: DslContext, fileUri: string): ExtractionResult {
        const visitor = new DomainVisitor();
        
        visitor.visit(tree);

        visitor.useCases.forEach(useCase => useCase.blockRange.fileUri = fileUri);
        visitor.serviceDefinitions.forEach(sd => sd.blockRange.fileUri = fileUri);
        visitor.domainDefinitions.forEach(dd => dd.blockRange.fileUri = fileUri);
        visitor.actorDefinitions.forEach(ad => ad.blockRange.fileUri = fileUri);

        // Build domain -> use cases mapping
        const domainUseCases: Record<string, string[]> = {};
        visitor.useCases.forEach(useCase => {
            if (useCase.entryPointSubDomain) {
                if (!domainUseCases[useCase.entryPointSubDomain]) {
                    domainUseCases[useCase.entryPointSubDomain] = [];
                }
                domainUseCases[useCase.entryPointSubDomain].push(useCase.name);
            }
        });

        const result: ExtractionResult = {
            domains: Array.from(visitor.domains).sort(),
            useCases: visitor.useCases,
            fileResults: [],
            serviceDefinitions: visitor.serviceDefinitions,
            domainDefinitions: visitor.domainDefinitions,
            actorDefinitions: visitor.actorDefinitions
        };

        return result;
    }
}
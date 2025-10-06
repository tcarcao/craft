// src/parser/CraftParser.ts
import { CharStream, CommonTokenStream } from "antlr4ng";
import { CraftLexer } from "./generated/CraftLexer.js";
import { CraftParser } from "./generated/CraftParser.js";
import { CustomErrorListener } from './ErrorListener.js';
import { BlockRange } from '../../../shared/lib/types/domain-extraction.js';
import { extractMinimalSubtree } from './DSLExtractor.js';
import { Logger } from '../utils/Logger.js';

export class Parser {
    parse(input: string) {
        try {
            const [lexer, parser] = this.initializeParser(input);

            // Remove default error listeners
            parser.removeErrorListeners();
            lexer.removeErrorListeners();

            // Add custom error listener
            const errorListener = new CustomErrorListener();
            parser.addErrorListener(errorListener);
            lexer.addErrorListener(errorListener);

            // Parse the input - replace 'dsl' with your grammar's start rule
            const tree = parser.dsl();

            return {
                success: errorListener.errors.length === 0,
                errors: errorListener.errors,
                tree: tree
            };
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } catch (e: any) {
            Logger.error('Parser error:', e);
            return {
                success: false,
                errors: [e.message || 'Unknown parser error'],
                tree: null
            };
        }
    }

    extractSelectedDSL(
        originalText: string,
        selectedRanges: BlockRange[]
    ): string {
        try {
            const [lexer, parser] = this.initializeParser(originalText);

            // Remove default error listeners
            parser.removeErrorListeners();
            lexer.removeErrorListeners();

            // Add custom error listener
            const errorListener = new CustomErrorListener();
            parser.addErrorListener(errorListener);
            lexer.addErrorListener(errorListener);

            const ast = parser.dsl();

            return extractMinimalSubtree(ast, selectedRanges, originalText);
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } catch (e: any) {
            Logger.error('Parser error:', e);
            return '';
        }
    }

    extractArchitecturalBlocks(originalText: string): string {
        try {
            const [lexer, parser] = this.initializeParser(originalText);

            // Remove default error listeners
            parser.removeErrorListeners();
            lexer.removeErrorListeners();

            // Add custom error listener
            const errorListener = new CustomErrorListener();
            parser.addErrorListener(errorListener);
            lexer.addErrorListener(errorListener);

            const ast = parser.dsl();

            // Extract only architectural blocks (no specific ranges)
            return extractMinimalSubtree(ast, [], originalText);
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } catch (e: any) {
            Logger.error('Parser error:', e);
            return '';
        }
    }

    private initializeParser(input: string): [CraftLexer, CraftParser] {
        const inputStream = CharStream.fromString(input);

        // Create lexer
        const lexer = new CraftLexer(inputStream);

        // Create token stream
        const tokenStream = new CommonTokenStream(lexer);

        // Create parser
        const parser = new CraftParser(tokenStream);

        return [lexer, parser];
    }

}
// src/parser/ArchDSLParser.ts
import { CharStream, CommonTokenStream } from "antlr4ng";
import { ArchDSLLexer } from "./generated/ArchDSLLexer";
import { ArchDSLParser } from "./generated/ArchDSLParser";
import { CustomErrorListener } from './ErrorListener';

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
            console.error('Parser error:', e);
            return {
                success: false,
                errors: [e.message || 'Unknown parser error'],
                tree: null
            };
        }
    }

    private initializeParser(input: string): [ArchDSLLexer, ArchDSLParser] {
        const inputStream = CharStream.fromString(input);

        // Create lexer
        const lexer = new ArchDSLLexer(inputStream);

        // Create token stream
        const tokenStream = new CommonTokenStream(lexer);

        // Create parser
        const parser = new ArchDSLParser(tokenStream);

        return [lexer, parser];
    }

}
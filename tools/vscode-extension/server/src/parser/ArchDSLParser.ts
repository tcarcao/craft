// src/parser/Parser.ts
import { CustomErrorListener } from './ErrorListener';
import { InputStream, CommonTokenStream } from 'antlr4';

// Need to require the generated files since they're JavaScript
// eslint-disable-next-line @typescript-eslint/no-require-imports
const ArchDSLLexer = require('./generated/ArchDSLLexer.js');
// eslint-disable-next-line @typescript-eslint/no-require-imports
const ArchDSLParser = require('./generated/ArchDSLParser.js');

export class Parser {
    parse(input: string) {
        const chars = new InputStream(input);
        const lexer = new ArchDSLLexer.default(chars);
        const tokens = new CommonTokenStream(lexer);
        const parser = new ArchDSLParser.default(tokens);
        
        // Create and add error listener
        const errorListener = new CustomErrorListener();
        parser.removeErrorListeners();
        parser.addErrorListener(errorListener);

        try {
            const tree = parser.architecture();
            return {
                success: errorListener.errors.length === 0,
                errors: errorListener.errors,
                tree: tree
            };
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } catch (e: any) {
            return {
                success: false,
                errors: [...errorListener.errors, e.message],
                tree: null
            };
        }
    }
}
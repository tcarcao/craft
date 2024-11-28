import { Token } from 'antlr4';
import { ErrorListener, RecognitionException, Recognizer } from 'antlr4';

export class CustomErrorListener implements ErrorListener<Token> {
    errors: string[] = [];

    syntaxError(
        recognizer: Recognizer<Token>,
        offendingSymbol: Token | null,
        line: number,
        column: number,
        msg: string,
        e: RecognitionException | undefined
    ): void {
        this.errors.push(`Line ${line}:${column} ${msg}`);
    }
}
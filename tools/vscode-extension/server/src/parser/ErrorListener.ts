import { BaseErrorListener, RecognitionException, Recognizer, Token, ATNSimulator } from "antlr4ng";


export class CustomErrorListener extends BaseErrorListener {
    errors: string[] = [];

    public override syntaxError<S extends Token, T extends ATNSimulator>(_recognizer: Recognizer<T>,
        _offendingSymbol: S | null, line: number, column: number, msg: string,
        _e: RecognitionException | null): void {
        this.errors.push(`Line ${line}:${column} ${msg}`);
    }
}

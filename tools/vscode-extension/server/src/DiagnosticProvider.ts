// server/src/DiagnosticProvider.ts
import {
    Diagnostic,
    DiagnosticSeverity,
    Position,
    Range
} from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { Parser } from './parser/ArchDSLParser';

export class DiagnosticProvider {
    private parser: Parser;

    constructor() {
        this.parser = new Parser();
    }

    public getDiagnostics(document: TextDocument): Diagnostic[] {
        const text = document.getText();
        const result = this.parser.parse(text);

        const diagnostics: Diagnostic[] = [];

        console.log(result.errors);

        if (!result.success) {
            result.errors.forEach(error => {
                const match = error.match(/Line (\d+):(\d+) (.*)/);
                if (match) {
                    const line = parseInt(match[1]) - 1;
                    const column = parseInt(match[2]);
                    const message = match[3];

                    const range = Range.create(
                        Position.create(line, column),
                        Position.create(line, column + 1)
                    );

                    diagnostics.push(Diagnostic.create(
                        range,
                        message,
                        DiagnosticSeverity.Error
                    ));
                }
            });
        }

        return diagnostics;
    }
    /*
    const diagnostics: Diagnostic[] = validationResult.errors.map(error => ({
          severity: DiagnosticSeverity.Error,
          range: {
              start: document.positionAt(error.startIndex),
              end: document.positionAt(error.endIndex)
          },
          message: error.message,
          source: 'c4-dsl'
      }));
    */
}
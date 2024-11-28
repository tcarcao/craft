// Server (server.ts)
import {
  createConnection,
  TextDocuments,
  Diagnostic,
  DiagnosticSeverity,
  ProposedFeatures,
  InitializeParams,
  TextDocumentSyncKind,
  InitializeResult,
} from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument'
import { DiagnosticProvider } from './DiagnosticProvider';

// Create connection and documents manager
const connection = createConnection(ProposedFeatures.all);
const documents: TextDocuments<TextDocument> = new TextDocuments(TextDocument);
let diagnosticProvider: DiagnosticProvider;

connection.onInitialize((params: InitializeParams) => {
  diagnosticProvider = new DiagnosticProvider();

  const result: InitializeResult = {
      capabilities: {
          textDocumentSync: TextDocumentSyncKind.Incremental,
          // Enable other capabilities as needed
      }
  };
  return result;
});

// Validate document on changes
documents.onDidChangeContent(change => {
  validateDocument(change.document);
});

async function validateDocument(document: TextDocument): Promise<void> {
  try {
      const diagnostics = diagnosticProvider.getDiagnostics(document);
      
      // Send diagnostics to VS Code
      connection.sendDiagnostics({ uri: document.uri, diagnostics });
  } catch (error: any) {
      connection.console.error(`Error validating document: ${error.message}`);
  }
}

// Start the language server
documents.listen(connection);
connection.listen();
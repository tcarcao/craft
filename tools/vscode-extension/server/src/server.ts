// Server (server.ts)
import {
  createConnection,
  TextDocuments,
  // Diagnostic,
  // DiagnosticSeverity,
  ProposedFeatures,
  InitializeParams,
  TextDocumentSyncKind,
  InitializeResult,
  ExecuteCommandParams,
} from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { DiagnosticProvider } from './DiagnosticProvider';
import { DomainExtractor } from './parser/DomainExtractor';
import { WorkspaceParser } from './services/documentsProcessor';
import {
  FileResult,
  UseCaseInfo,
  ExtractionResult,
  ServerCommands,
} from '../../shared/lib/types/domain-extraction';

// Create connection and documents manager
const connection = createConnection(ProposedFeatures.all);
const documents = new TextDocuments<TextDocument>(TextDocument);
let diagnosticProvider: DiagnosticProvider;
let domainExtractor: DomainExtractor;
const workspaceParser = new WorkspaceParser(documents);


connection.onInitialize((params: InitializeParams) => {
  if (params.workspaceFolders) {
    workspaceParser.setWorkspaceFolders(params.workspaceFolders);
  }

  diagnosticProvider = new DiagnosticProvider();
  domainExtractor = new DomainExtractor();

  const result: InitializeResult = {
    capabilities: {
      textDocumentSync: TextDocumentSyncKind.Incremental,
      executeCommandProvider: {
        commands: [
          ServerCommands.EXTRACT_DOMAINS_FROM_CURRENT,
          ServerCommands.EXTRACT_DOMAINS_FROM_WORKSPACE,
        ]
      }
      // Enable other capabilities as needed
    }
  };
  return result;
});

// Handle custom commands
connection.onExecuteCommand((params: ExecuteCommandParams) => {
  switch (params.command) {
    case ServerCommands.EXTRACT_DOMAINS_FROM_CURRENT:
      return handleExtractDomains(params.arguments);
    case ServerCommands.EXTRACT_DOMAINS_FROM_WORKSPACE:
      return handleExtractAllDomainsFromWorkspace(params.arguments, workspaceParser);
    default:
      return { error: 'Unknown command' };
  }
});

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function handleExtractDomains(args: any[] | undefined): Promise<ExtractionResult> {
  if (!args || args.length === 0) {
    return {
      domains: [],
      useCases: [],
      fileResults: [],
      error: 'No document URI provided'
    };
  }

  const documentUri = args[0];
  const document = documents.get(documentUri);

  if (!document) {
    return {
      domains: [],
      useCases: [],
      fileResults: [],
      error: 'Document not found'
    };
  }

  try {
    const result = domainExtractor.extractFromDocument(document);
    return result;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } catch (error: any) {
    return {
      domains: [],
      useCases: [],
      fileResults: [],
      error: error.message
    };
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function handleExtractAllDomainsFromWorkspace(_args: any[] | undefined, workspaceParser: WorkspaceParser): Promise<ExtractionResult> {
  try {

    const processedDocuments = await workspaceParser.processAllDocuments(
      (content, info) => {
        const extraction = domainExtractor.extractFromText(content, info.uri);
        return {
          uri: info.uri,
          fileName: info.uri.split('/').pop() || 'unkown',
          ...extraction
        };
        //   ({
        //   lineCount: content.split('\n').length,
        //   charCount: content.length,
        //   hasExports: content.includes('export'),
        //   filePath: info.filePath
        // })
      },
      {
        include: ['**/*.dsl'],
        exclude: ['**/node_modules/**'],
        concurrency: 5
      }
    );
    const fileResults: FileResult[] = processedDocuments.map(processedDocument => {
      const extraction = processedDocument.result;

      return {
        ...extraction
      };
    });

    // Combine all results
    const combined = combineExtractionResults(fileResults);
    return {
      ...combined,
      fileResults
    };
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } catch (error: any) {
    return {
      domains: [],
      useCases: [],
      fileResults: [],
      error: error.message
    };
  }
}

function combineExtractionResults(results: FileResult[]): Omit<ExtractionResult, 'fileResults'> {
  const allDomains = new Set<string>();
  const allUseCases: UseCaseInfo[] = [];

  results.forEach(result => {
    if (result.domains) {
      result.domains.forEach((domain: string) => allDomains.add(domain));
    }

    if (result.useCases) {
      result.useCases.forEach(useCase => allUseCases.push(useCase));
    }
  });

  return {
    domains: Array.from(allDomains).sort(),
    useCases: allUseCases,
  };
}

// Validate document on changes
documents.onDidChangeContent(change => {
  validateDocument(change.document);
});

async function validateDocument(document: TextDocument): Promise<void> {
  try {
    const diagnostics = diagnosticProvider.getDiagnostics(document);

    // Send diagnostics to VS Code
    connection.sendDiagnostics({ uri: document.uri, diagnostics });
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } catch (error: any) {
    connection.console.error(`Error validating document: ${error.message}`);
  }
}

// Start the language server
documents.listen(connection);
connection.listen();
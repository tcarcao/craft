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
  ServiceDefinition,
  DomainDefinition,
  BlockRange
} from '../../shared/lib/types/domain-extraction';
import { Parser } from './parser/ArchDSLParser';

// Create connection and documents manager
const connection = createConnection(ProposedFeatures.all);
const documents = new TextDocuments<TextDocument>(TextDocument);
let diagnosticProvider: DiagnosticProvider;
let domainExtractor: DomainExtractor;
const workspaceParser = new WorkspaceParser(documents);
const parser = new Parser();


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
          ServerCommands.EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES,
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
    case ServerCommands.EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES:
      return handleExtractPartialDslFromBlockRanges(params.arguments, workspaceParser);
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
      serviceDefinitions: [],
      domainDefinitions: [],
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
      serviceDefinitions: [],
      domainDefinitions: [],
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
      serviceDefinitions: [],
      domainDefinitions: [],
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
      serviceDefinitions: [],
      domainDefinitions: [],
      error: error.message
    };
  }
}

function combineExtractionResults(results: FileResult[]): Omit<ExtractionResult, 'fileResults'> {
  const allDomains = new Set<string>();
  const allUseCases: UseCaseInfo[] = [];
  const allServiceDefinitions: ServiceDefinition[] = [];
  const allDomainDefinitions: DomainDefinition[] = [];

  results.forEach(result => {
    if (result.domains) {
      result.domains.forEach((domain: string) => allDomains.add(domain));
    }

    if (result.useCases) {
      result.useCases.forEach(useCase => allUseCases.push(useCase));
    }

    if (result.serviceDefinitions) {
      result.serviceDefinitions.forEach(sd => allServiceDefinitions.push(sd));
    }

    if (result.domainDefinitions) {
      result.domainDefinitions.forEach(dd => {
        // Check if domain already exists and merge if necessary
        const existingIndex = allDomainDefinitions.findIndex(existing => existing.name === dd.name);
        if (existingIndex !== -1) {
          // Merge subdomains
          const existing = allDomainDefinitions[existingIndex];
          const mergedSubDomains = Array.from(new Set([...existing.subDomains, ...dd.subDomains]));
          allDomainDefinitions[existingIndex] = {
            ...existing,
            subDomains: mergedSubDomains
          };
        } else {
          // Add new domain definition
          allDomainDefinitions.push(dd);
        }
      });
    }
  });

  return {
    domains: Array.from(allDomains).sort(),
    useCases: allUseCases,
    serviceDefinitions: allServiceDefinitions,
    domainDefinitions: allDomainDefinitions,
  };
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unused-vars
async function handleExtractPartialDslFromBlockRanges(args: any[] | undefined, workspaceParser: WorkspaceParser): Promise<string> {
  if (!args || args.length === 0) {
    return '';
  }

  const ranges: BlockRange[] = args[0];
  const combinedParts: string[] = [];
  
  // Group ranges by file to minimize file reads
  const rangesByFile = ranges.reduce((acc, range) => {
    if (!acc[range.fileUri]) {
      acc[range.fileUri] = [];
    }
    acc[range.fileUri].push(range);
    return acc;
  }, {} as Record<string, BlockRange[]>);


  await workspaceParser.processAllDocuments(
      (content, info) => {
        const fileRanges = rangesByFile[info.uri] || [];
        
        // Sort ranges by line number for this file
        fileRanges.sort((a, b) => a.startLine - b.startLine);

        const extractedDSL: string = parser.extractSelectedDSL(content, fileRanges);
        combinedParts.push(extractedDSL);
      },
      {
        include: ['**/*.dsl'],
        exclude: ['**/node_modules/**'],
        concurrency: 5
      }
  );
  
  return combinedParts.join('\n\n');
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
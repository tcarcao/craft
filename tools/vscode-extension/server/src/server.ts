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
  DocumentFormattingParams,
  TextEdit,
  CompletionParams,
  CompletionItem,
  CompletionItemKind,
} from 'vscode-languageserver/node.js';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { TreeSitterDiagnosticProvider } from './TreeSitterDiagnosticProvider.js';
import { DomainExtractor } from './parser/DomainExtractor.js';
import { WorkspaceParser } from './services/documentsProcessor.js';
import {
  FileResult,
  UseCaseInfo,
  ExtractionResult,
  ServerCommands,
  ServiceDefinition,
  DomainDefinition,
  ActorDefinition,
  BlockRange
} from '../../shared/lib/types/domain-extraction.js';
import { Parser } from './parser/CraftParser.js';
import { TreeSitterCompletionProvider } from './parser/TreeSitterCompletionProvider.js';
import { TreeSitterFormatterProvider } from './parser/TreeSitterFormatterProvider.js';
import { Logger } from './utils/Logger.js';

// Create connection and documents manager
console.log('SERVER: Starting Craft Language Server...');
console.log('SERVER: Environment info:');
console.log('SERVER: __dirname:', __dirname);
console.log('SERVER: process.cwd():', process.cwd());
console.log('SERVER: Node version:', process.version);
console.log('SERVER: Process argv:', process.argv.join(' '));

let connection: any;
let documents: any;
try {
  connection = createConnection(ProposedFeatures.all);
  console.log('SERVER: Connection created');
  documents = new TextDocuments<TextDocument>(TextDocument);
  console.log('SERVER: Document manager created');
} catch (error) {
  console.error('SERVER: Error creating connection or documents:', error);
  console.error('SERVER: Error stack:', error.stack);
  process.exit(1);
}
let treeSitterDiagnosticProvider: TreeSitterDiagnosticProvider;
let domainExtractor: DomainExtractor;
console.log('SERVER: Creating core components...');
let workspaceParser: WorkspaceParser;
let parser: Parser;
let treeSitterCompletionProvider: TreeSitterCompletionProvider;
let treeSitterFormatter: TreeSitterFormatterProvider;

try {
  workspaceParser = new WorkspaceParser(documents);
  console.log('SERVER: WorkspaceParser created');
  parser = new Parser();
  console.log('SERVER: Parser created');

  // Tree-sitter providers for language features
  treeSitterCompletionProvider = new TreeSitterCompletionProvider();
  console.log('SERVER: TreeSitterCompletionProvider created');
  treeSitterFormatter = new TreeSitterFormatterProvider();
  console.log('SERVER: TreeSitterFormatterProvider created');
} catch (error) {
  console.error('SERVER: Error creating core components:', error);
  process.exit(1);
}


connection.onInitialize((params: InitializeParams) => {
  console.log('SERVER: onInitialize called');
  if (params.workspaceFolders) {
    console.log('SERVER: Setting workspace folders:', params.workspaceFolders.length);
    workspaceParser.setWorkspaceFolders(params.workspaceFolders);
  }

  console.log('SERVER: Creating providers...');
  try {
    treeSitterDiagnosticProvider = new TreeSitterDiagnosticProvider();
    console.log('SERVER: TreeSitterDiagnosticProvider created');
    domainExtractor = new DomainExtractor();
    console.log('SERVER: DomainExtractor created');
    console.log('SERVER: All providers created successfully');
  } catch (error) {
    console.error('SERVER: Error creating providers:', error);
    throw error;
  }
  
  // Initialize logger with default level
  const logLevel = params.initializationOptions?.logLevel || 'warn';
  Logger.setLevel(logLevel);

  const result: InitializeResult = {
    capabilities: {
      textDocumentSync: TextDocumentSyncKind.Incremental,
      executeCommandProvider: {
        commands: [
          ServerCommands.EXTRACT_DOMAINS_FROM_CURRENT,
          ServerCommands.EXTRACT_DOMAINS_FROM_WORKSPACE,
          ServerCommands.EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES,
        ]
      },
      documentFormattingProvider: true,
      completionProvider: {
        triggerCharacters: [' ', ':', '{', '\n'],
        resolveProvider: true,
        allCommitCharacters: ['{', '}', ' ', '\n']
      }
      // Enable other capabilities as needed
    }
  };
  return result;
});

// Handle configuration changes
connection.onDidChangeConfiguration(change => {
  if (change.settings.craft?.logging?.level) {
    Logger.setLevel(change.settings.craft.logging.level);
    Logger.info('Log level updated to:', change.settings.craft.logging.level);
  }
});

// Handle custom commands
connection.onExecuteCommand((params: ExecuteCommandParams) => {
  Logger.debugServerRequest(params.command, params.arguments);
  
  switch (params.command) {
    case ServerCommands.EXTRACT_DOMAINS_FROM_CURRENT:
      return handleExtractDomains(params.arguments);
    case ServerCommands.EXTRACT_DOMAINS_FROM_WORKSPACE:
      return handleExtractAllDomainsFromWorkspace(params.arguments, workspaceParser);
    case ServerCommands.EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES:
      return handleExtractPartialDslFromBlockRanges(params.arguments, workspaceParser);
    default:
      Logger.warn('Unknown command:', params.command);
      return { error: 'Unknown command' };
  }
});

// Handle document formatting
connection.onDocumentFormatting(async (params: DocumentFormattingParams): Promise<TextEdit[]> => {
  const document = documents.get(params.textDocument.uri);
  if (!document) {
    return [];
  }

  try {
    const formattedContent = await formatCraftDocument(document.getText());
    return [{
      range: {
        start: { line: 0, character: 0 },
        end: { line: document.lineCount, character: 0 }
      },
      newText: formattedContent
    }];
  } catch (error) {
    Logger.error('Error formatting document:', error);
    return [];
  }
});

// Handle autocomplete
connection.onCompletion(async (params: CompletionParams): Promise<CompletionItem[]> => {
  const document = documents.get(params.textDocument.uri);
  if (!document) {
    Logger.debug('No document found for completion');
    return [];
  }

  Logger.debug(`Completion requested at ${params.position.line}:${params.position.character}, context: ${JSON.stringify(params.context)}`);

  try {
    // Use only Tree-sitter completion provider
    const completions = await treeSitterCompletionProvider.getCompletions(document, params.position);
    Logger.debug(`Tree-sitter completions: ${completions.length} items`);
    
    // If no completions from tree-sitter, provide basic fallback
    if (completions.length === 0) {
      Logger.debug('Providing fallback completions');
      return [
        {
          label: 'actors',
          kind: CompletionItemKind.Module,
          detail: 'Actors definition block',
          insertText: 'actors {\n    $1\n}',
          insertTextFormat: 2
        },
        {
          label: 'services',
          kind: CompletionItemKind.Module,
          detail: 'Services definition block',
          insertText: 'services {\n    $1\n}',
          insertTextFormat: 2
        },
        {
          label: 'use_case',
          kind: CompletionItemKind.Class,
          detail: 'Use case definition',
          insertText: 'use_case "$1" {\n    $2\n}',
          insertTextFormat: 2
        }
      ];
    }
    
    return completions;
  } catch (error) {
    Logger.error('Error providing completions:', error);
    return [];
  }
});

// Handle completion resolve for additional details
connection.onCompletionResolve((item: CompletionItem): CompletionItem => {
  // Add any additional information to completion items if needed
  return item;
});

async function formatCraftDocument(content: string): Promise<string> {
  try {
    // Use Tree-sitter formatter
    const document = TextDocument.create('temp://format.craft', 'craft', 1, content);
    const edits = await treeSitterFormatter.formatDocument(document);
    
    if (edits.length > 0) {
      return edits[0].newText;
    }
    
    return content; // No changes needed
  } catch (error) {
    Logger.error('Error in formatting:', error);
    return content; // Return original content on any error
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function handleExtractDomains(args: any[] | undefined): Promise<ExtractionResult> {
  if (!args || args.length === 0) {
    return {
      domains: [],
      useCases: [],
      fileResults: [],
      serviceDefinitions: [],
      domainDefinitions: [],
      actorDefinitions: [],
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
      actorDefinitions: [],
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
      actorDefinitions: [],
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
        include: ['**/*.craft'],
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
      actorDefinitions: [],
      error: error.message
    };
  }
}

function combineExtractionResults(results: FileResult[]): Omit<ExtractionResult, 'fileResults'> {
  const allDomains = new Set<string>();
  const allUseCases: UseCaseInfo[] = [];
  const allServiceDefinitions: ServiceDefinition[] = [];
  const allDomainDefinitions: DomainDefinition[] = [];
  const allActorDefinitions: ActorDefinition[] = [];

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

    if (result.actorDefinitions) {
      result.actorDefinitions.forEach(ad => {
        // Check if actor already exists - avoid duplicates across files
        const existingIndex = allActorDefinitions.findIndex(existing => existing.name === ad.name);
        if (existingIndex === -1) {
          // Actor doesn't exist, add it
          allActorDefinitions.push(ad);
        }
        // Note: If actor exists, we could merge or overwrite, but for now just keep the first one
      });
    }
  });

  return {
    domains: Array.from(allDomains).sort(),
    useCases: allUseCases,
    serviceDefinitions: allServiceDefinitions,
    domainDefinitions: allDomainDefinitions,
    actorDefinitions: allActorDefinitions,
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
        
        if (fileRanges.length > 0) {
          // File has specific selections - extract selected ranges + architectural blocks
          fileRanges.sort((a, b) => a.startLine - b.startLine);
          const extractedDSL: string = parser.extractSelectedDSL(content, fileRanges);
          if (extractedDSL.trim()) {
            combinedParts.push(extractedDSL);
          }
        } else {
          // File has no selections - extract only architectural blocks (actors, arch, exposure)
          const architecturalDSL: string = parser.extractArchitecturalBlocks(content);
          if (architecturalDSL.trim()) {
            combinedParts.push(architecturalDSL);
          }
        }
      },
      {
        include: ['**/*.craft'],
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
    const treeSitterDiagnostics = await treeSitterDiagnosticProvider.getDiagnostics(document);

    // Send diagnostics to VS Code
    connection.sendDiagnostics({ uri: document.uri, diagnostics: treeSitterDiagnostics });
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } catch (error: any) {
    Logger.error('Error validating document:', error.message);
  }
}

// Start the language server
console.log('SERVER: Setting up document listeners...');
documents.listen(connection);
console.log('SERVER: Starting connection listener...');
connection.listen();
console.log('SERVER: Language server started and listening');
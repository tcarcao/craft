export interface BlockRange {
  startLine: number;
  endLine: number;
  fileUri: string;
}

export interface UseCaseInfo {
    name: string;
    entryPointSubDomain: string | null;
    allDomains: string[];
    scenarios: string[];
    blockRange: BlockRange;
}

// Individual file result with metadata
export interface FileResult {
    domains: string[];
    useCases: UseCaseInfo[];
    uri: string;
    fileName: string;
}

// Workspace extraction result (combines all files)
export interface ExtractionResult {
    // Combined data from all files
    domains: string[];
    useCases: UseCaseInfo[];
    
    // Individual file results
    fileResults: FileResult[];
    
    // Error handling
    error?: string;
}

// Command request/response types
export const ServerCommands = {
  EXTRACT_DOMAINS_FROM_CURRENT: 'archdsl.extractDomains',
  EXTRACT_DOMAINS_FROM_WORKSPACE: 'archdsl.extractAllDomainsFromWorkspace',
} as const;

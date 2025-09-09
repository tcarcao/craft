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
export interface ServiceDefinition {
    name: string;
    domains: string[];
    parentDomain?: string;
    dataStores?: string[];
    language?: string;
    blockRange: BlockRange;
}

export interface DomainDefinition {
    name: string;
    subDomains: string[];
    blockRange: BlockRange;
}

export interface FileResult {
    domains: string[];
    useCases: UseCaseInfo[];
    serviceDefinitions: ServiceDefinition[];
    domainDefinitions: DomainDefinition[];
    uri: string;
    fileName: string;
}

// Workspace extraction result (combines all files)
export interface ExtractionResult {
    // Combined data from all files
    domains: string[];
    useCases: UseCaseInfo[];
    serviceDefinitions: ServiceDefinition[];
    domainDefinitions: DomainDefinition[];
    
    // Individual file results
    fileResults: FileResult[];
    
    // Error handling
    error?: string;
}

// Command request/response types
export const ServerCommands = {
  EXTRACT_DOMAINS_FROM_CURRENT: 'archdsl.extractDomains',
  EXTRACT_DOMAINS_FROM_WORKSPACE: 'archdsl.extractAllDomainsFromWorkspace',
  EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES: 'archdsl.extractDslFromBlockRanges'
} as const;

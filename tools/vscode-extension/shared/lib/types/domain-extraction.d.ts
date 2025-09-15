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
export interface ExtractionResult {
    domains: string[];
    useCases: UseCaseInfo[];
    serviceDefinitions: ServiceDefinition[];
    domainDefinitions: DomainDefinition[];
    fileResults: FileResult[];
    error?: string;
}
export declare const ServerCommands: {
    readonly EXTRACT_DOMAINS_FROM_CURRENT: "craft.extractDomains";
    readonly EXTRACT_DOMAINS_FROM_WORKSPACE: "craft.extractAllDomainsFromWorkspace";
    readonly EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES: "craft.extractDslFromBlockRanges";
};

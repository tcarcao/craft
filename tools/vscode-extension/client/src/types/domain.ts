import { BlockRange } from '../../../shared/lib/types/domain-extraction';

export interface DSLDiscoveryOptions {
    currentFile?: string;
    includeDataStores?: boolean;
    includeScenarios?: boolean;
}

export interface DSLDiscoveryResult {
    domains: Domain[];
    serviceGroups: ServiceGroup[];
}

const defaultDomain = 'Unknown';

export const DomainC = {
    DefaultDomain: defaultDomain,
    EmptyDomain: emptyDomain(),
    GenerateDomainId: generateDomainId,
    GenerateSubDomainId: generateSubDomainId,
    GenerateUseCaseId: generateUseCaseId,
    GenerateServiceId: generateServiceId,
}

export interface Domain {
    id: string;
    name: string;
    description: string;
    expanded: boolean;
    selected: boolean;
    partiallySelected: boolean;
    inCurrentFile: boolean;
    subDomains: SubDomain[];
    selectedUseCases: number;
    totalUseCases: number;
    selectedSubDomains: number;
}

function emptyDomain() {
    const emptyDomain: Domain = {
        id: generateDomainId(defaultDomain),
        name: defaultDomain,
        description: '',
        expanded: false,
        selected: false,
        partiallySelected: false,
        inCurrentFile: false,
        subDomains: [],
        selectedUseCases: 0,
        totalUseCases: 0,
        selectedSubDomains: 0
    }
    return emptyDomain;
}

function generateDomainId(domainName: string): string {
    return `domain-${domainName.toLowerCase().replace(/[^a-z0-9]/g, '-')}`;
}

export interface SubDomain {
    id: string;
    name: string;
    description: string;
    expanded: boolean;
    showReferences: boolean;
    selected: boolean;
    partiallySelected: boolean;
    focused: boolean; // New property for C4 focus mode
    inCurrentFile: boolean;
    useCases: UseCase[];
    referencedIn: UseCaseReference[];
    selectedUseCases: number;
    totalUseCases: number;
}

function generateSubDomainId(domainName: string, subDomainName: string): string {
    const cleanSubDomainName = subDomainName.toLowerCase().replace(/[^a-z0-9]/g, '-');
    return `${generateDomainId(domainName)}-subdomain-${cleanSubDomainName}`;
}

export interface UseCaseReference {
    useCaseId: string;
    useCaseName: string;
    domainName: string;
    blockRange: BlockRange;
    role: 'entry-point' | 'involved';
}

export interface UseCase {
    id: string;
    name: string;
    description: string;
    selected: boolean;
    fileName: string;
    blockRange: BlockRange;
    scenarios: string[];
    entryPointSubDomain: string;
    involvedSubDomains: string[];
}

function generateUseCaseId(domainName: string, subDomainName: string, useCaseName: string): string {
    const cleanUseCaseName = useCaseName.toLowerCase().replace(/[^a-z0-9]/g, '-');
    return `${generateSubDomainId(domainName, subDomainName)}-uc-${cleanUseCaseName}`;
}

export interface DomainTreeState {
    currentFileDomains: Map<string, Domain>;
    workspaceDomains: Map<string, Domain>;
    expandedNodes: Set<string>;
    selectedNodes: Set<string>;
    viewMode: 'current' | 'workspace';
    currentFile?: string;
    isLoading: boolean;
}

export interface Service {
    id: string;
    name: string;
    description?: string;
    domain: Domain;
    subDomains: SubDomain[];
    dependencies: string[];
    tags?: string[];
    selected: boolean;
    partiallySelected: boolean;
    focused: boolean; // New property for C4 focus mode
    inCurrentFile: boolean;
    expanded: boolean;
    blockRange: BlockRange;
}

function generateServiceId(domainName: string, subDomainName: string, serviceName: string): string {
    const cleanServiceName = serviceName.toLowerCase().replace(/[^a-z0-9]/g, '-');
    return `${generateSubDomainId(domainName, subDomainName)}-s-${cleanServiceName}`;
}

export interface ServiceGroup {
    name: string;
    services: Service[];
    expanded: boolean;
    selected: boolean;
    partiallySelected: boolean;
    inCurrentFile: boolean;
}

export interface ServiceTreeState {
    currentFileServiceGroups: Map<string, ServiceGroup>;
    workspaceServiceGroups: Map<string, ServiceGroup>;
    viewMode: 'current' | 'workspace';
    boundariesMode: 'transparent' | 'boundaries';
    showDatabases: boolean;
    optionsExpanded: boolean;
    expandedNodes: Set<string>;
    selectedNodes: Set<string>;
    currentFile?: string;
    isLoading: boolean;
}
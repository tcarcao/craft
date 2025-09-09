// src/services/domainService.ts
import { Uri } from 'vscode';
import { LanguageClient } from 'vscode-languageclient/node';
import {
    UseCaseInfo,
    FileResult,
    ExtractionResult,
    ServerCommands,
    ServiceDefinition
} from '../../../shared/lib/types/domain-extraction';
import { Domain, DomainC, DSLDiscoveryOptions, DSLDiscoveryResult, Service, ServiceGroup, SubDomain, UseCase, UseCaseReference } from '../types/domain';

export class DslExtractService {
    constructor(private readonly languageClient: LanguageClient) { }

    async discoverDSL(options: DSLDiscoveryOptions = {}): Promise<DSLDiscoveryResult> {
        try {
            // Get domains from current file if specified
            let currentFileResult: ExtractionResult | null = null;
            if (options.currentFile) {
                const currentFileUri = Uri.file(options.currentFile).toString();
                currentFileResult = await this.languageClient.sendRequest('workspace/executeCommand', {
                    command: ServerCommands.EXTRACT_DOMAINS_FROM_CURRENT,
                    arguments: [currentFileUri]
                });
            }

            // Get domains from entire workspace
            const workspaceResult: ExtractionResult = await this.languageClient.sendRequest('workspace/executeCommand', {
                command: ServerCommands.EXTRACT_DOMAINS_FROM_WORKSPACE,
                arguments: []
            });

            console.log('workspaceResult', workspaceResult);

            // Convert the results to Domain structure
            const domains = this.convertToDomainStructure(workspaceResult, currentFileResult);
            const serviceGroups = this.convertToServiceGroups(workspaceResult, currentFileResult, domains);

            return { domains, serviceGroups };
        } catch (error) {
            console.error('Error discovering domains:', error);
            throw error;
        }
    }

    private convertToDomainStructure(
        workspaceResult: ExtractionResult,
        currentFileResult: ExtractionResult | null
    ): Domain[] {
        console.log('convertToDomainStructure, workspaceResult', workspaceResult);
        if (workspaceResult.error) {
            throw new Error(workspaceResult.error);
        }

        // Group sub-domains by their parent domain
        const domainGroups = new Map<string, string[]>();
        const currentFileUriSet = currentFileResult ? new Set(currentFileResult.domains || []) : new Set();

        // Process each discovered domain as a sub-domain
        workspaceResult.domains.forEach((subDomainName: string) => {
            // Get parent domain from parsing results, fallback to Unknown
            const parentDomain = this.getParentDomainFromResults(workspaceResult, subDomainName);

            if (!domainGroups.has(parentDomain)) {
                domainGroups.set(parentDomain, []);
            }
            domainGroups.get(parentDomain)!.push(subDomainName);
        });

        const domains: Domain[] = [];

        // Create a top-level domain for each group
        domainGroups.forEach((subDomainNames, parentDomainName) => {
            const domain: Domain = {
                id: DomainC.GenerateDomainId(parentDomainName),
                name: parentDomainName,
                description: `Domain: ${parentDomainName}`,
                expanded: parentDomainName === DomainC.DefaultDomain, // Auto-expand Unknown
                selected: false,
                partiallySelected: false,
                inCurrentFile: subDomainNames.some(name => currentFileUriSet.has(name)),
                subDomains: [],
                selectedUseCases: 0,
                totalUseCases: 0,
                selectedSubDomains: 0,
            };

            // Create sub-domains (parsed domains become sub-domains)
            subDomainNames.forEach((subDomainName: string) => {
                const subDomain: SubDomain = {
                    id: DomainC.GenerateSubDomainId(parentDomainName, subDomainName),
                    name: subDomainName,
                    description: `Sub-domain: ${subDomainName}`,
                    expanded: false,
                    showReferences: false,
                    selected: false,
                    partiallySelected: false,
                    focused: true, // Default to focused (show as internal in C4)
                    inCurrentFile: currentFileUriSet.has(subDomainName),
                    useCases: [],
                    selectedUseCases: 0,
                    referencedIn: [],
                    totalUseCases: 0,
                };

                // Get all use cases for this sub-domain from all files
                const useCasesForSubDomain = this.getUseCasesForSubDomain(workspaceResult, parentDomainName, subDomainName);

                useCasesForSubDomain.forEach(useCase => {
                    subDomain.useCases.push(useCase);
                });

                const useCasesWhereSubDomainIsInvolved = this.getUseCasesWhereSubDomainIsInvolved(workspaceResult, parentDomainName, subDomainName);

                useCasesWhereSubDomainIsInvolved.forEach(useCase => {
                    subDomain.referencedIn.push(useCase);
                });

                domain.subDomains.push(subDomain);
            });

            domains.push(domain);
        });

        return domains.sort((a, b) => {
            // Sort so that "Unknown" comes last, others alphabetically
            if (a.name === DomainC.DefaultDomain && b.name !== DomainC.DefaultDomain) { return 1; }
            if (b.name === DomainC.DefaultDomain && a.name !== DomainC.DefaultDomain) { return -1; }
            return a.name.localeCompare(b.name);
        });
    }

    private getParentDomainFromResults(
        workspaceResult: ExtractionResult,
        subDomainName: string
    ): string {
        // FIRST PRIORITY: Check domain definitions - this is the new functionality
        if (workspaceResult.domainDefinitions) {
            for (const domainDef of workspaceResult.domainDefinitions) {
                if (domainDef.subDomains.includes(subDomainName)) {
                    return domainDef.name;
                }
            }
        }

        // Check file-level domain definitions as well
        if (workspaceResult.fileResults) {
            for (const fileResult of workspaceResult.fileResults) {
                if (fileResult.domainDefinitions) {
                    for (const domainDef of fileResult.domainDefinitions) {
                        if (domainDef.subDomains.includes(subDomainName)) {
                            return domainDef.name;
                        }
                    }
                }
            }
        }

        // FALLBACK: Check individual file results for domain hierarchy in service definitions
        if (workspaceResult.fileResults) {
            for (const fileResult of workspaceResult.fileResults) {
                // Check if this sub-domain is defined with a parent in service definitions
                if (fileResult.serviceDefinitions) {
                    for (const service of fileResult.serviceDefinitions) {
                        if (service.domains && service.domains.includes(subDomainName) && service.parentDomain) {
                            return service.parentDomain;
                        }
                    }
                }
            }
        }

        return DomainC.DefaultDomain;
    }

    private getUseCasesForSubDomain(
        workspaceResult: ExtractionResult,
        parentDomainName: string,
        subDomainName: string
    ): UseCase[] {
        const useCases: UseCase[] = [];

        if (workspaceResult.fileResults) {
            workspaceResult.fileResults.forEach((fileResult: FileResult) => {
                if (fileResult.useCases) {
                    fileResult.useCases.forEach((useCaseInfo: UseCaseInfo) => {
                        // Include use case if this sub-domain is the primary domain or involved
                        if (useCaseInfo.entryPointSubDomain === subDomainName) {
                            useCases.push({
                                id: DomainC.GenerateUseCaseId(parentDomainName, subDomainName, useCaseInfo.name),
                                name: useCaseInfo.name,
                                description: this.generateUseCaseDescription(useCaseInfo, fileResult.fileName),
                                selected: false,
                                fileName: fileResult.fileName,
                                blockRange: useCaseInfo.blockRange,
                                scenarios: useCaseInfo.scenarios || [],
                                involvedSubDomains: useCaseInfo.allDomains || [subDomainName],
                                entryPointSubDomain: subDomainName
                            });
                        }
                    });
                }
            });
        }

        return useCases;
    }

    private getUseCasesWhereSubDomainIsInvolved(
        workspaceResult: ExtractionResult, parentDomainName: string, subDomainName: string): UseCaseReference[] {
        const useCases: UseCaseReference[] = [];

        if (workspaceResult.fileResults) {
            workspaceResult.fileResults.forEach((fileResult: FileResult) => {
                if (fileResult.useCases) {
                    fileResult.useCases.forEach((useCaseInfo: UseCaseInfo) => {

                        if (useCaseInfo.entryPointSubDomain !== subDomainName && useCaseInfo.allDomains && useCaseInfo.allDomains.includes(subDomainName)) {

                            useCases.push({
                                useCaseId: DomainC.GenerateUseCaseId(parentDomainName, subDomainName, useCaseInfo.name),
                                useCaseName: useCaseInfo.name,
                                domainName: useCaseInfo.entryPointSubDomain,
                                blockRange: useCaseInfo.blockRange,
                                role: 'involved'
                            });
                        }
                    });
                }
            });
        }

        return useCases;
    }

    private generateUseCaseDescription(useCaseInfo: UseCaseInfo, fileName: string): string {
        let description = "";

        if (useCaseInfo.allDomains && useCaseInfo.allDomains.length > 1) {
            description += `\nInvolved Domains: ${useCaseInfo.allDomains.join(', ')}`;
        }

        if (fileName) {
            description += `\nFile: ${fileName}`;
        }

        return description;
    }

    convertToServiceGroups(
workspaceResult: ExtractionResult, currentFileResult: ExtractionResult | null, domains: Domain[],
    ): ServiceGroup[] {

        const serviceDefinitions = workspaceResult.serviceDefinitions;
        const currentFileUriSet = currentFileResult ? new Set(currentFileResult.serviceDefinitions.map(s => s.name) || []) : new Set();

        // Group services by parentDomain (using domain definitions or explicit parentDomain)
        const groupedServices = serviceDefinitions.reduce((groups, service) => {
            // First try explicit parentDomain, then check domain definitions for service domains
            let parentDomain = service.parentDomain;
            if (!parentDomain && service.domains && service.domains.length > 0) {
                // Find parent domain from domain definitions for any of the service's domains
                for (const serviceDomain of service.domains) {
                    parentDomain = this.getParentDomainFromResults(workspaceResult, serviceDomain);
                    if (parentDomain !== DomainC.DefaultDomain) {
                        break; // Use the first non-Unknown parent domain found
                    }
                }
            }
            parentDomain = parentDomain || DomainC.DefaultDomain;
            const groupName = parentDomain;
            const domain = domains.find(d => d.name === parentDomain) || DomainC.EmptyDomain;
            const subDomains = domain.subDomains.filter(sd => service.domains.some(otherSd => otherSd === sd.name));

            if (!groups[groupName]) {
                groups[groupName] = [];
            }

            const serviceAsService: Service = {
                id: DomainC.GenerateServiceId(groupName, service.domains[0] || "", service.name),
                name: service.name,
                domain: domain,
                subDomains: subDomains,
                // dataStores: service.dataStores,
                // language: service.language,
                dependencies: [],
                blockRange: service.blockRange,
                selected: false,
                partiallySelected: false,
                focused: true, // Default to focused (show as internal in C4)
                inCurrentFile: currentFileUriSet.has(service.name),
                expanded: false
            };

            groups[groupName].push(serviceAsService);
            return groups;
        }, {} as Record<string, Service[]>);

        // Convert grouped services to ServiceGroup array
        return Object.entries(groupedServices).map(([groupName, services]) => ({
            name: groupName,
            services: services,
            expanded: false,
            selected: false,
            partiallySelected: false,
            inCurrentFile: services.map(s => s.name).some(name => currentFileUriSet.has(name))
        }));
    }
}
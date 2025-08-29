import { Uri, WebviewViewProvider, WebviewView, WebviewViewResolveContext, CancellationToken, TextDocument, window, workspace, commands } from 'vscode';
import { ServicesViewService } from '../services/servicesViewService';
import { DslExtractService } from '../services/dslExtractService';
import { ServicesViewHtmlGenerator } from '../ui/servicesViewHtmlGenerator';
import { ServiceTreeState, ServiceGroup, Service, UseCase } from '../types/domain';
import { LanguageClient } from 'vscode-languageclient/node';
import { ServerCommands } from '../../../shared/lib/types/domain-extraction';

export class ServicesViewProvider implements WebviewViewProvider {
    public static readonly viewType = 'dslServicesView';
    
    private _view?: WebviewView;
    private _state: ServiceTreeState = {
        serviceGroups: new Map(),
        viewMode: 'current',
        expandedNodes: new Set(),
        selectedNodes: new Set(),
        currentFile: undefined,
        isLoading: false,
    };

    private _isInitialized = false;
    private _refreshTimeout?: NodeJS.Timeout;

    constructor(
        private readonly languageClient: LanguageClient,
        private readonly _extensionUri: Uri,
        private readonly _extractService: DslExtractService,
        private readonly _serviceTreeService: ServicesViewService,
        private readonly _htmlGenerator: ServicesViewHtmlGenerator
    ) {
        // Listen for active editor changes
        window.onDidChangeActiveTextEditor(() => {
            this.updateCurrentFile();
            this.deferredRefresh();
        });

        // Listen for file saves
        workspace.onDidSaveTextDocument((document) => {
            if (this.isArchDSLDocument(document)) {
                this.deferredRefresh();
            }
        });

        // Listen for file opens/closes
        workspace.onDidOpenTextDocument((document) => {
            if (this.isArchDSLDocument(document)) {
                this.deferredRefresh();
            }
        });

        workspace.onDidCloseTextDocument((document) => {
            if (this.isArchDSLDocument(document)) {
                this.deferredRefresh();
            }
        });
    }

    public resolveWebviewView(
        webviewView: WebviewView,
        _context: WebviewViewResolveContext,
        _token: CancellationToken,
    ) {
        this._view = webviewView;

        webviewView.webview.options = {
            enableScripts: true,
            localResourceRoots: [this._extensionUri]
        };

        // Show loading state initially
        this._state.isLoading = true;
        this.updateWebview();

        this.updateCurrentFile();

        // Defer the initial load to give the language server time to discover documents
        this.deferredRefresh(1000); // Wait 1 second on initial load
        
        // Load domains asynchronously
        this.refreshServices().then(() => {
            this._state.isLoading = false;
            this.updateWebview();
        });

        // Handle messages from the webview
        webviewView.webview.onDidReceiveMessage(async (data) => {
            switch (data.type) {
                case 'toggleServiceGroup':
                    this.handleToggleServiceGroup(data.groupId);
                    break;
                case 'toggleService':
                    this.handleToggleService(data.serviceGroupId, data.serviceId);
                    break;
                case 'toggleSubDomain':
                    this.handleToggleSubDomain(data.serviceGroupId, data.serviceId, data.subDomainId);
                    break;
                case 'toggleUseCase':
                    this.handleToggleUseCase(data.serviceGroupId, data.serviceId, data.subDomainId, data.useCaseId);
                    break;
                case 'toggleGroupExpansion':
                    this.handleToggleGroupExpansion(data.groupId);
                    break;
                case 'toggleServiceExpansion':
                    this.handleToggleServiceExpansion(data.groupId, data.serviceId);
                    break;
                case 'toggleSubDomainExpansion':
                    this.handleToggleSubDomainExpansion(data.groupId, data.serviceId, data.subDomainId);
                    break;
                case 'setViewMode':
                    this.handleSetViewMode(data.mode);
                    break;
                case 'selectAll':
                    this.handleSelectAll();
                    break;
                case 'selectNone':
                    this.handleSelectNone();
                    break;
                case 'preview':
                    this.handlePreview();
                    break;
                case 'refresh':
                    this.handleRefresh();
                    break;
            }
        });
    }

    private updateCurrentFile() {
        const activeEditor = window.activeTextEditor;
        const previousFile = this._state.currentFile;
        
        if (activeEditor && this.isArchDSLDocument(activeEditor.document)) {
            this._state.currentFile = activeEditor.document.fileName;
            console.log('Current file updated to:', this._state.currentFile);
            
            // If file changed and we're in current file mode, refresh
            if (this._isInitialized && 
                this._state.currentFile !== previousFile && 
                this._state.viewMode === 'current') {
                this.deferredRefresh();
            }
        } else {
            this._state.currentFile = undefined;
            
            // If we lost the current file and we're in current file mode, refresh
            if (this._isInitialized && 
                previousFile && 
                this._state.viewMode === 'current') {
                this.deferredRefresh();
            }
        }
    }

    private isArchDSLDocument(document: TextDocument): boolean {
        return document.languageId === 'archdsl' || 
               document.fileName.endsWith('.dsl');
    }

    private deferredRefresh(delay = 300) {
        // Clear existing timeout
        if (this._refreshTimeout) {
            clearTimeout(this._refreshTimeout);
        }

        // Set new timeout
        this._refreshTimeout = setTimeout(() => {
            this.refreshServices();
        }, delay);
    }

    private async refreshServices() {
        try {
            const { serviceGroups } = await this._extractService.discoverDSL({ currentFile: this._state.currentFile });

            serviceGroups.forEach(serviceGroup => {
                const existingServiceGroup = this._state.serviceGroups.get(serviceGroup.name);
                if (existingServiceGroup) {
                    serviceGroup.expanded = existingServiceGroup.expanded;
                    serviceGroup.services.forEach(service => {
                        const existingService = existingServiceGroup.services.find(s => s.id === service.id);
                        if (existingService) {
                            service.selected = existingService.selected;
                        }
                    });
                    this._serviceTreeService.updateServiceGroupSelection(serviceGroup);
                }

                this._state.serviceGroups.set(serviceGroup.name, serviceGroup);
            });

            const currentSubGroups = new Set(serviceGroups.map(d => d.name));
            for (const [groupName] of this._state.serviceGroups) {
                if (!currentSubGroups.has(groupName)) {
                    this._state.serviceGroups.delete(groupName);
                }
            }

            // Only update webview if not in loading state (to avoid double updates)
            if (!this._state.isLoading) {
                this.updateWebview();
            }
        } catch (error) {
            console.error('Error refreshing domains:', error);
            window.showErrorMessage(`Failed to refresh domains: ${error}`);
        }
    }

    private handleToggleServiceGroup(groupId: string) {
        const serviceGroup = this._state.serviceGroups.get(groupId);
        if (serviceGroup) {
            this._serviceTreeService.toggleServiceGroupSelection(serviceGroup);
            this.updateWebview();
        }
    }

    private handleToggleService(serviceGroupId: string, serviceId: string) {
        const serviceGroup = this._state.serviceGroups.get(serviceGroupId);
        if (serviceGroup) {
            this._serviceTreeService.toggleServiceSelection(serviceGroup, serviceId);
            this.updateWebview();
        }
    }

    private handleToggleSubDomain(serviceGroupId: string, serviceId: string, subDomainId: string) {
        const serviceGroup = this._state.serviceGroups.get(serviceGroupId);
        if (serviceGroup) {
            const service = serviceGroup.services.find(s => s.id === serviceId);

            if (service) {
                this._serviceTreeService.toggleSubDomainSelection(serviceGroup, service, subDomainId);
                this.updateWebview();
            }
        }
    }

    private handleToggleUseCase(serviceGroupId: string, serviceId: string, subDomainId: string, useCaseId: string) {
        const serviceGroup = this._state.serviceGroups.get(serviceGroupId);
        if (serviceGroup) {
            const service = serviceGroup.services.find(s => s.id === serviceId);

            if (service) {
                const subDomain = service.subDomains.find(sd => sd.id === subDomainId);

                if (subDomain) {
                    this._serviceTreeService.toggleUseCaseSelection(serviceGroup, subDomain, useCaseId);
                    this.updateWebview();
                }
            }
        }
    }

    private handleToggleGroupExpansion(groupId: string) {
        const serviceGroup = this._state.serviceGroups.get(groupId);
        if (serviceGroup) {
            serviceGroup.expanded = !serviceGroup.expanded;
            this.updateWebview();
        }
    }

    private handleToggleServiceExpansion(groupId: string, serviceId: string) {
        const serviceGroup = this._state.serviceGroups.get(groupId);
        if (serviceGroup) {
            const service = serviceGroup.services.find(s => s.id === serviceId);
            if (service) {
                service.expanded = !service.expanded;
                this.updateWebview();
            }
        }
    }

    private handleToggleSubDomainExpansion(groupId: string, serviceId: string, subDomainId: string) {
        const serviceGroup = this._state.serviceGroups.get(groupId);
        if (serviceGroup) {
            const service = serviceGroup.services.find(s => s.id === serviceId);
            if (service) {
                const subDomain = service.subDomains.find(sd => sd.id === subDomainId);
                subDomain.expanded = !subDomain.expanded;
                this.updateWebview();
            }
        }
    }

    private handleSetViewMode(mode: 'current' | 'workspace') {
        this._state.viewMode = mode;
        this.updateWebview();
    }

    // private handleSetGroupBy(groupBy: 'type' | 'domain') {
    //     this._groupBy = groupBy;
    //     this.updateServiceGroups();
    //     this.updateWebview();
    // }

    private handleSelectAll() {
        const groupServices = Array.from(this._state.serviceGroups.values());
        this._serviceTreeService.selectAll(groupServices, this._state.viewMode === 'current');
        this.updateWebview();
    }

    private handleSelectNone() {
        const groupServices = Array.from(this._state.serviceGroups.values());
        this._serviceTreeService.selectNone(groupServices);
        this.updateWebview();
    }

    private async handlePreview() {
        console.log('handle preview here we go');
        const selectedServices = this.getSelectedServices();
        const selectedUseCases = this.getSelectedUseCases();
        const blockRanges = [];
        selectedServices.forEach(s => blockRanges.push(s.blockRange));
        selectedUseCases.forEach(uc => blockRanges.push(uc.blockRange));
        const partialDsl: string = await this.languageClient.sendRequest('workspace/executeCommand', {
            command: ServerCommands.EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES,
            arguments: [blockRanges]
        });
        console.log(partialDsl);
        commands.executeCommand('archdsl.previewPartialDSL', partialDsl, "C4");
    }

    private async handleRefresh() {
        this._state.isLoading = true;
        this.updateWebview();
        await this.refreshServices();
        this._state.isLoading = false;
        this.updateWebview();
    }

    private updateWebview() {
        if (!this._view) {
            return;
        }

        if (this._state.isLoading) {
            this._view.webview.html = this._htmlGenerator.generateLoadingHtml();
            return;
        }

        const serviceGroups = Array.from(this._state.serviceGroups.values());
        const visibleDomains = this._state.viewMode === 'current' 
            ? serviceGroups.filter(d => d.inCurrentFile)
            : serviceGroups;

        // Calculate selection counts
        const selectedCount = this.calculateSelectionCounts(visibleDomains);
        const totalCount = this.calculateTotalCounts(visibleDomains);

        this._view.webview.html = this._htmlGenerator.generateTreeHtml(
            visibleDomains,
            this._state.viewMode,
            selectedCount,
            totalCount
        );
    }

    private calculateSelectionCounts(serviceGroups: ServiceGroup[]) {
        let selectedServiceGroups = 0;
        let selectedServices = 0;

        serviceGroups.forEach(serviceGroup => {
            if (serviceGroup.selected) {
                selectedServiceGroups++;
            }
            
            serviceGroup.services.forEach(service => {
                if (service.selected) {
                    selectedServices++;
                }
            });
        });

        return { serviceGroups: selectedServiceGroups, services: selectedServices };
    }

    private calculateTotalCounts(serviceGroups: ServiceGroup[]) {
        const totalServiceGroups = serviceGroups.length;
        let totalServices = 0;

        serviceGroups.forEach(domain => {
            totalServices += domain.services.length;
        });

        return { serviceGroups: totalServiceGroups, services: totalServices };
    }

    private getSelectedServices(): Service[] {
        return Array.from(this._state.serviceGroups.values())
            .filter(serviceGroup => serviceGroup.selected || serviceGroup.partiallySelected)
            .flatMap(serviceGroup => serviceGroup.services)
            .filter(service => service.selected || service.partiallySelected);
    }

    private getSelectedUseCases(): UseCase[] {
        return Array.from(this._state.serviceGroups.values())
            .filter(serviceGroup => serviceGroup.selected || serviceGroup.partiallySelected)
            .flatMap(serviceGroup => serviceGroup.services)
            .filter(service => service.selected || service.partiallySelected)
            .flatMap(service => service.subDomains)
            .filter(subDomain => subDomain.selected || subDomain.partiallySelected)
            .flatMap(subDomain => subDomain.useCases)
            .filter(useCase => useCase.selected);
    }
}
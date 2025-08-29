// src/providers/domainTreeViewProvider.ts

import { Uri, WebviewViewProvider, WebviewView, WebviewViewResolveContext, CancellationToken, TextDocument, window, workspace, commands } from 'vscode';
import { DomainsViewService } from '../services/domainsViewService';
import { DomainsViewHtmlGenerator } from '../ui/domainsViewHtmlGenerator';
import { DslExtractService } from '../services/dslExtractService';
import { Domain, DomainTreeState, UseCase } from '../types/domain';
import { LanguageClient } from 'vscode-languageclient/node';
import { ServerCommands } from '../../../shared/lib/types/domain-extraction';

export class DomainsViewProvider implements WebviewViewProvider {
    public static readonly viewType = 'dslDomainView';

    private _view?: WebviewView;
    private _state: DomainTreeState = {
        domains: new Map(),
        expandedNodes: new Set(),
        selectedNodes: new Set(),
        viewMode: 'current',
        currentFile: undefined,
        isLoading: false
    };

    private _isInitialized = false;
    private _refreshTimeout?: NodeJS.Timeout;

    constructor(
        private readonly languageClient: LanguageClient,
        private readonly _extensionUri: Uri,
        private readonly _extractService: DslExtractService,
        private readonly _domainService: DomainsViewService,
        private readonly _htmlGenerator: DomainsViewHtmlGenerator
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
        this.refreshDomains().then(() => {
            this._state.isLoading = false;
            this.updateWebview();
        });

        // Handle messages from the webview
        webviewView.webview.onDidReceiveMessage(async (data) => {
            switch (data.type) {
                case 'toggleDomain':
                    await this.handleToggleDomain(data.domainId);
                    break;
                case 'toggleSubDomain':
                    await this.handleToggleSubDomain(data.domainId, data.subDomainId);
                    break;
                case 'toggleUseCase':
                    await this.handleToggleUseCase(data.domainId, data.subDomainId, data.useCaseId);
                    break;
                case 'toggleExpansion':
                    await this.handleToggleExpansion(data.domainId);
                    break;
                case 'toggleSubDomainExpansion':
                    await this.handleToggleSubDomainExpansion(data.domainId, data.subDomainId);
                    break;
                case 'setViewMode':
                    await this.handleSetViewMode(data.mode);
                    break;
                case 'selectAll':
                    await this.handleSelectAll();
                    break;
                case 'selectNone':
                    await this.handleSelectNone();
                    break;
                case 'selectCurrentFileOnly':
                    await this.handleSelectCurrentFileOnly();
                    break;
                case 'preview':
                    this.handlePreview();
                    break;
                case 'refresh':
                    await this.handleRefresh();
                    break;
                case 'toggleReferences':
                    this.handleToggleReferences(data.domainId, data.subDomainId);
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
            this.refreshDomains();
        }, delay);
    }

    private async refreshDomains() {
        try {
            const { domains } = await this._extractService.discoverDSL({ currentFile: this._state.currentFile });
            domains.forEach(domain => this._domainService.updateDomainCounts(domain));

            // Preserve existing expansion and selection states
            domains.forEach(domain => {
                const existingDomain = this._state.domains.get(domain.id);
                if (existingDomain) {
                    // Preserve domain expansion state
                    domain.expanded = existingDomain.expanded;

                    // Preserve subdomain states
                    domain.subDomains.forEach(subDomain => {
                        const existingSubDomain = existingDomain.subDomains.find(sd => sd.id === subDomain.id);
                        if (existingSubDomain) {
                            subDomain.expanded = existingSubDomain.expanded;

                            // Preserve use case selection states
                            subDomain.useCases.forEach(useCase => {
                                const existingUseCase = existingSubDomain.useCases.find(uc => uc.id === useCase.id);
                                if (existingUseCase) {
                                    useCase.selected = existingUseCase.selected;
                                }
                            });
                        }
                    });

                    // Recalculate selection states
                    this._domainService.updateDomainCounts(domain);
                }

                this._state.domains.set(domain.id, domain);
            });

            // Remove domains that no longer exist
            const currentDomainIds = new Set(domains.map(d => d.id));
            for (const [domainId] of this._state.domains) {
                if (!currentDomainIds.has(domainId)) {
                    this._state.domains.delete(domainId);
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

    private async handleToggleDomain(domainId: string) {
        const domain = this._state.domains.get(domainId);
        if (domain) {
            const newSelectedState = !domain.selected && !domain.partiallySelected;
            this._domainService.toggleDomainSelection(domain, newSelectedState);
            this.updateWebview();
        }
    }

    private async handleToggleSubDomain(domainId: string, subDomainId: string) {
        const domain = this._state.domains.get(domainId);
        if (domain) {
            const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
            if (subDomain) {
                const newSelectedState = !subDomain.selected && !subDomain.partiallySelected;
                this._domainService.toggleSubDomainSelection(domain, subDomainId, newSelectedState);
                this.updateWebview();
            }
        }
    }

    private async handleToggleUseCase(domainId: string, subDomainId: string, useCaseId: string) {
        const domain = this._state.domains.get(domainId);
        if (domain) {
            this._domainService.toggleUseCaseSelection(domain, subDomainId, useCaseId);
            this.updateWebview();
        }
    }

    private async handleToggleExpansion(domainId: string) {
        const domain = this._state.domains.get(domainId);
        if (domain) {
            domain.expanded = !domain.expanded;
            this.updateWebview();
        }
    }

    private async handleToggleSubDomainExpansion(domainId: string, subDomainId: string) {
        const domain = this._state.domains.get(domainId);
        if (domain) {
            const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
            if (subDomain) {
                subDomain.expanded = !subDomain.expanded;
                this.updateWebview();
            }
        }
    }

    private async handleToggleReferences(domainId: string, subDomainId: string) {
        const domain = this._state.domains.get(domainId);
        if (domain) {
            const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
            if (subDomain) {
                subDomain.showReferences = !subDomain.showReferences;
                this.updateWebview();
            }
        }
    }

    private async handleSetViewMode(mode: 'current' | 'workspace') {
        this._state.viewMode = mode;
        this.updateWebview();
    }

    private async handleSelectAll() {
        const domains = Array.from(this._state.domains.values());
        this._domainService.selectAll(domains, this._state.viewMode === 'current');
        this.updateWebview();
    }

    private async handleSelectNone() {
        const domains = Array.from(this._state.domains.values());
        this._domainService.selectNone(domains);
        this.updateWebview();
    }

    private async handleSelectCurrentFileOnly() {
        const domains = Array.from(this._state.domains.values());
        this._domainService.selectNone(domains);
        this._domainService.selectAll(domains.filter(d => d.inCurrentFile), false);
        this.updateWebview();
    }

    private async handlePreview() {
        console.log('handle preview here we go');
        const selectedUseCases = this.getSelectedUseCases();
        console.log('selectedUseCases', selectedUseCases);
        const blockRanges = selectedUseCases.map(d => d.blockRange);
        const partialDsl: string = await this.languageClient.sendRequest('workspace/executeCommand', {
            command: ServerCommands.EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES,
            arguments: [blockRanges]
        });
        console.log(partialDsl);
        commands.executeCommand('archdsl.previewPartialDSL', partialDsl, "Domain");
    }

    private async handleRefresh() {
        this._state.isLoading = true;
        this.updateWebview();
        await this.refreshDomains();
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

        const domains = Array.from(this._state.domains.values());
        const visibleDomains = this._state.viewMode === 'current'
            ? domains.filter(d => d.inCurrentFile)
            : domains;

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

    private calculateSelectionCounts(domains: Domain[]) {
        let selectedDomains = 0;
        let selectedSubDomains = 0;
        let selectedUseCases = 0;

        domains.forEach(domain => {
            if (domain.selected) {
                selectedDomains++;
            }

            domain.subDomains.forEach(subDomain => {
                if (subDomain.selected) {
                    selectedSubDomains++;
                }

                selectedUseCases += subDomain.useCases.filter(uc => uc.selected).length;
            });
        });

        return { domains: selectedDomains, subDomains: selectedSubDomains, useCases: selectedUseCases };
    }

    private calculateTotalCounts(domains: Domain[]) {
        const totalDomains = domains.length;
        let totalSubDomains = 0;
        let totalUseCases = 0;

        domains.forEach(domain => {
            totalSubDomains += domain.subDomains.length;
            totalUseCases += domain.totalUseCases;
        });

        return { domains: totalDomains, subDomains: totalSubDomains, useCases: totalUseCases };
    }

    private getSelectedUseCases(): UseCase[] {
        return Array.from(this._state.domains.values())
            .filter(domain => domain.selected || domain.partiallySelected)
            .flatMap(domain => domain.subDomains)
            .flatMap(subDomain => subDomain.useCases)
            .filter(useCase => useCase.selected);
    }
}
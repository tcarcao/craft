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
        currentFileDomains: new Map(),
        workspaceDomains: new Map(),
        expandedNodes: new Set(),
        selectedNodes: new Set(),
        viewMode: 'current',
        currentFile: undefined,
        isLoading: false
    };

    // Helper method to get the appropriate domain map based on view mode
    private getDomainsMap(): Map<string, Domain> {
        return this._state.viewMode === 'current' 
            ? this._state.currentFileDomains 
            : this._state.workspaceDomains;
    }

    // Helper method to get both domain maps for dual updates
    private getBothDomainMaps(): Map<string, Domain>[] {
        return [this._state.currentFileDomains, this._state.workspaceDomains];
    }

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
            const shouldRefresh = this.updateCurrentFile();
            if (shouldRefresh) {
                this.deferredRefresh();
            }
        });

        // Listen for file content changes (real-time)
        workspace.onDidChangeTextDocument((changeEvent) => {
            if (this.isCraftDocument(changeEvent.document)) {
                // Only refresh if content is parseable to avoid flickering during invalid intermediate states
                this.deferredRefreshWithValidation(changeEvent.document);
            }
        });

        // Listen for file saves
        workspace.onDidSaveTextDocument((document) => {
            if (this.isCraftDocument(document)) {
                this.deferredRefresh();
            }
        });

        // Listen for file opens/closes
        workspace.onDidOpenTextDocument((document) => {
            if (this.isCraftDocument(document)) {
                this.deferredRefresh();
            }
        });

        workspace.onDidCloseTextDocument((document) => {
            if (this.isCraftDocument(document)) {
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
            this._isInitialized = true; // Mark as initialized after first load
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

    private updateCurrentFile(): boolean {
        const activeEditor = window.activeTextEditor;
        const previousFile = this._state.currentFile;

        if (activeEditor && this.isCraftDocument(activeEditor.document)) {
            // We switched to a DSL file - update current file
            this._state.currentFile = activeEditor.document.fileName;
            console.log('Current file updated to:', this._state.currentFile);

            // Only refresh if file actually changed and we're in current file mode
            if (this._isInitialized &&
                this._state.currentFile !== previousFile &&
                this._state.viewMode === 'current') {
                console.log('File changed from', previousFile, 'to', this._state.currentFile, '- refresh needed');
                return true; // Refresh needed
            } else {
                console.log('No refresh needed - same file or not in current mode');
                return false; // No refresh needed
            }
        } else {
            // We switched to a non-DSL file or panel (like preview)
            // Keep the current file state - don't set it to undefined
            // This maintains the trees showing the last DSL file's content
            console.log('Switched to non-DSL file/panel, maintaining current file state:', this._state.currentFile);

            // Don't refresh or clear the trees when switching to non-DSL files
            return false; // No refresh needed
        }
    }

    private isCraftDocument(document: TextDocument): boolean {
        return document.languageId === 'craft' ||
            document.fileName.endsWith('.craft');
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

    private async deferredRefreshWithValidation(document: TextDocument, delay = 300) {
        // Clear existing timeout
        if (this._refreshTimeout) {
            clearTimeout(this._refreshTimeout);
        }

        // Set new timeout with validation
        this._refreshTimeout = setTimeout(async () => {
            try {
                // Quick validation check - try to parse the content
                const content = document.getText();
                if (content.trim().length === 0) {
                    return; // Don't refresh on empty content
                }

                // Attempt to parse the DSL to see if it's valid
                // We'll use the language server to validate the content
                await this.languageClient.sendRequest('workspace/executeCommand', {
                    command: 'craft.validateDocument',
                    arguments: [document.uri.toString()]
                });

                // If validation succeeds, proceed with refresh
                this.refreshDomains();
            } catch (error) {
                // If validation fails, don't refresh to avoid flickering
                console.log('Skipping refresh due to invalid DSL content during editing');
            }
        }, delay);
    }

    private async refreshDomains() {
        try {
            const { domains } = await this._extractService.discoverDSL({ currentFile: this._state.currentFile });
            domains.forEach(domain => this._domainService.updateDomainCounts(domain));

            // Update both current file and workspace domains with preserved states
            // Create deep copies to avoid shared references
            const currentDomains = domains.filter(d => d.inCurrentFile).map(d => this.deepCopyDomainCurrentFile(d));
            const workspaceDomains = domains.map(d => this.deepCopyDomainWorkspace(d));

            // Preserve existing expansion and selection states for current file domains
            currentDomains.forEach(domain => {
                const existingDomain = this._state.currentFileDomains.get(domain.id);
                if (existingDomain) {
                    this.preserveDomainStates(domain, existingDomain);
                }
                this._state.currentFileDomains.set(domain.id, domain);
            });

            // Preserve existing expansion and selection states for workspace domains
            workspaceDomains.forEach(domain => {
                const existingDomain = this._state.workspaceDomains.get(domain.id);
                if (existingDomain) {
                    this.preserveDomainStates(domain, existingDomain);
                }
                this._state.workspaceDomains.set(domain.id, domain);
            });

            // Remove domains that no longer exist from current file
            const currentDomainIds = new Set(currentDomains.map(d => d.id));
            for (const [domainId] of this._state.currentFileDomains) {
                if (!currentDomainIds.has(domainId)) {
                    this._state.currentFileDomains.delete(domainId);
                }
            }

            // Remove domains that no longer exist from workspace
            const workspaceDomainIds = new Set(workspaceDomains.map(d => d.id));
            for (const [domainId] of this._state.workspaceDomains) {
                if (!workspaceDomainIds.has(domainId)) {
                    this._state.workspaceDomains.delete(domainId);
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
        // Get the target selection state from the current view
        const currentViewDomain = this.getDomainsMap().get(domainId);
        if (!currentViewDomain) return;
        
        const newSelectedState = !currentViewDomain.selected && !currentViewDomain.partiallySelected;
        
        // Determine which subdomains to update based on current view mode
        const subDomainsToUpdate = this._state.viewMode === 'current' 
            ? currentViewDomain.subDomains.filter(sd => sd.inCurrentFile) // Only current file subdomains
            : currentViewDomain.subDomains; // All subdomains in workspace mode
            
        // Apply updates to both maps
        this.updateSelectionInBothMaps(domainId, (domain) => {
            subDomainsToUpdate.forEach(targetSubDomain => {
                const subDomain = domain.subDomains.find(sd => sd.id === targetSubDomain.id);
                if (subDomain) {
                    subDomain.selected = newSelectedState;
                    subDomain.useCases.forEach(useCase => useCase.selected = newSelectedState);
                }
            });
        });
        
        this.updateWebview();
    }

    private async handleToggleSubDomain(domainId: string, subDomainId: string) {
        // Find the subdomain in current view to get target state
        const currentViewDomain = this.getDomainsMap().get(domainId);
        const currentViewSubDomain = currentViewDomain?.subDomains.find(sd => sd.id === subDomainId);
        if (!currentViewSubDomain) return;
        
        const newSelectedState = !currentViewSubDomain.selected && !currentViewSubDomain.partiallySelected;
        
        // Apply updates to both maps
        this.updateSelectionInBothMaps(domainId, (domain) => {
            const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
            if (subDomain) {
                subDomain.selected = newSelectedState;
                subDomain.useCases.forEach(useCase => useCase.selected = newSelectedState);
            }
        });
        
        this.updateWebview();
    }

    private async handleToggleUseCase(domainId: string, subDomainId: string, useCaseId: string) {
        // Find the use case in current view to get target state
        const currentViewDomain = this.getDomainsMap().get(domainId);
        const currentViewSubDomain = currentViewDomain?.subDomains.find(sd => sd.id === subDomainId);
        const currentViewUseCase = currentViewSubDomain?.useCases.find(uc => uc.id === useCaseId);
        if (!currentViewUseCase) return;
        
        const newSelectedState = !currentViewUseCase.selected;
        
        // Apply updates to both maps
        this.updateSelectionInBothMaps(domainId, (domain) => {
            const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
            const useCase = subDomain?.useCases.find(uc => uc.id === useCaseId);
            if (useCase) {
                useCase.selected = newSelectedState;
            }
        });
        
        this.updateWebview();
    }

    private async handleToggleExpansion(domainId: string) {
        // Update expansion state in both maps to keep them in sync
        this.getBothDomainMaps().forEach(domainMap => {
            const domain = domainMap.get(domainId);
            if (domain) {
                domain.expanded = !domain.expanded;
            }
        });
        this.updateWebview();
    }

    private async handleToggleSubDomainExpansion(domainId: string, subDomainId: string) {
        // Update expansion state in both maps to keep them in sync
        this.getBothDomainMaps().forEach(domainMap => {
            const domain = domainMap.get(domainId);
            if (domain) {
                const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
                if (subDomain) {
                    subDomain.expanded = !subDomain.expanded;
                }
            }
        });
        this.updateWebview();
    }

    private async handleToggleReferences(domainId: string, subDomainId: string) {
        // Update references state in both maps to keep them in sync
        this.getBothDomainMaps().forEach(domainMap => {
            const domain = domainMap.get(domainId);
            if (domain) {
                const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
                if (subDomain) {
                    subDomain.showReferences = !subDomain.showReferences;
                }
            }
        });
        this.updateWebview();
    }

    private async handleSetViewMode(mode: 'current' | 'workspace') {
        this._state.viewMode = mode;
        this.updateWebview();
    }

    private async handleSelectAll() {
        const domains = Array.from(this.getDomainsMap().values());
        this._domainService.selectAll(domains, this._state.viewMode === 'current');
        this.updateWebview();
    }

    private async handleSelectNone() {
        // Update selection in both maps to keep them in sync
        this.getBothDomainMaps().forEach(domainMap => {
            const domains = Array.from(domainMap.values());
            this._domainService.selectNone(domains);
        });
        this.updateWebview();
    }

    private async handleSelectCurrentFileOnly() {
        // Clear all selections in both maps first
        this.getBothDomainMaps().forEach(domainMap => {
            const domains = Array.from(domainMap.values());
            this._domainService.selectNone(domains);
        });
        
        // Then select only current file domains in both maps
        this.getBothDomainMaps().forEach(domainMap => {
            const domains = Array.from(domainMap.values());
            this._domainService.selectAll(domains.filter(d => d.inCurrentFile), false);
        });
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
        commands.executeCommand('craft.previewPartialDSL', partialDsl, "Domain");
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
            const cssUri = this._view.webview.asWebviewUri(
                Uri.joinPath(this._extensionUri, 'client', 'src', 'ui', 'styles', 'treeStyles.css')
            );
            this._view.webview.html = this._htmlGenerator.generateLoadingHtml(cssUri.toString());
            return;
        }

        const domains = Array.from(this.getDomainsMap().values());
        const visibleDomains = this._state.viewMode === 'current'
            ? domains.filter(d => d.inCurrentFile)
            : domains;

        // Filter children (subdomains) based on current file mode
        const filteredDomains = this.filterDomainChildren(visibleDomains);

        // Calculate selection counts
        const selectedCount = this.calculateSelectionCounts(filteredDomains);
        const totalCount = this.calculateTotalCounts(filteredDomains);

        const codiconsUri = this._view.webview.asWebviewUri(
            Uri.joinPath(this._extensionUri, 'client', 'node_modules', '@vscode/codicons', 'dist', 'codicon.css')
        );
        
        const cssUri = this._view.webview.asWebviewUri(
            Uri.joinPath(this._extensionUri, 'client', 'src', 'ui', 'styles', 'treeStyles.css')
        );
        
        this._view.webview.html = this._htmlGenerator.generateTreeHtml(
            filteredDomains,
            this._state.viewMode,
            selectedCount,
            totalCount,
            codiconsUri.toString(),
            cssUri.toString()
        );
    }

    private filterDomainChildren(domains: Domain[]): Domain[] {
        if (this._state.viewMode === 'workspace') {
            // In workspace mode, show all children (HTML generator will apply grey styling)
            return domains;
        }

        // In current mode, filter out children not in current file
        return domains.map(domain => ({
            ...domain,
            subDomains: domain.subDomains.filter(subDomain => subDomain.inCurrentFile)
        }));
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
        return Array.from(this.getDomainsMap().values())
            .filter(domain => domain.selected || domain.partiallySelected)
            .flatMap(domain => domain.subDomains)
            .flatMap(subDomain => subDomain.useCases)
            .filter(useCase => useCase.selected);
    }

    // Helper method to preserve domain states
    private preserveDomainStates(domain: Domain, existingDomain: Domain) {
        // Preserve domain expansion state
        domain.expanded = existingDomain.expanded;

        // Preserve subdomain states
        domain.subDomains.forEach(subDomain => {
            const existingSubDomain = existingDomain.subDomains.find(sd => sd.id === subDomain.id);
            if (existingSubDomain) {
                subDomain.expanded = existingSubDomain.expanded;
                subDomain.showReferences = existingSubDomain.showReferences;

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

    // Generic helper method for selection updates
    private updateSelectionInBothMaps(
        domainId: string,
        updateFn: (domain: Domain) => void
    ) {
        // 1) Update in current file map
        const currentFileDomain = this._state.currentFileDomains.get(domainId);
        if (currentFileDomain) {
            updateFn(currentFileDomain);
            this._domainService.updateDomainCounts(currentFileDomain);
        }
        
        // 2) Update in workspace map  
        const workspaceDomain = this._state.workspaceDomains.get(domainId);
        if (workspaceDomain) {
            updateFn(workspaceDomain);
            this._domainService.updateDomainCounts(workspaceDomain);
        }
    }

    private deepCopyDomainCurrentFile(domain: Domain): Domain {
        return this.deepCopyDomain(domain, true);
    }

    private deepCopyDomainWorkspace(domain: Domain): Domain {
        return this.deepCopyDomain(domain, false);
    }

    // Helper method to create deep copies of domains to avoid shared references
    private deepCopyDomain(domain: Domain, inCurrentFileFilter: boolean): Domain {
        return {
            ...domain,
            subDomains: domain.subDomains.filter(sd => inCurrentFileFilter === true ? sd.inCurrentFile : true).map(subDomain => ({
                ...subDomain,
                useCases: subDomain.useCases.map(useCase => ({ ...useCase })),
                referencedIn: subDomain.referencedIn.map(ref => ({ ...ref }))
            }))
        };
    }
}
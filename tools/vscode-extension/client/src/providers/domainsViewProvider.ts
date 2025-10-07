import { Uri, WebviewViewProvider, WebviewView, WebviewViewResolveContext, CancellationToken, TextDocument, window, workspace, commands, ExtensionContext } from 'vscode';
import { DomainsViewService } from '../services/domainsViewService';
import { DslExtractService } from '../services/dslExtractService';
import { Domain, DomainTreeState, UseCase, ServiceGroup, Service } from '../types/domain';
import { LanguageClient } from 'vscode-languageclient/node';
import { ServerCommands, BlockRange } from '../../../shared/lib/types/domain-extraction';
import { WebviewMessages, ProviderMessages, SelectionActions } from '../types/messages';
import { Logger } from '../utils/Logger';

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
    
    // Store service groups for service block range lookup
    private _serviceGroups: ServiceGroup[] = [];

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

    private getWebviewContent(): string {
        if (!this._view) return '';

        const scriptUri = this._view.webview.asWebviewUri(
            Uri.joinPath(this._extensionUri, 'dist', 'webview', 'domains.js')
        );

        const codiconsUri = this._view.webview.asWebviewUri(
            Uri.joinPath(this._extensionUri, 'dist', '@vscode/codicons', 'dist', 'codicon.css')
        );

        const stylesUri = this._view.webview.asWebviewUri(
            Uri.joinPath(this._extensionUri, 'dist', 'styles', 'treeStyles.css')
        );

        // Generate a nonce for security
        const nonce = this.getNonce();

        return `<!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${this._view.webview.cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}'; font-src ${this._view.webview.cspSource};">
            <link href="${codiconsUri}" rel="stylesheet">
            <link href="${stylesUri}" rel="stylesheet">
            <title>Domains View</title>
        </head>
        <body>
            <div id="root">Loading...</div>
            <script nonce="${nonce}" src="${scriptUri}"></script>
        </body>
        </html>`;
    }

    private getNonce(): string {
        let text = '';
        const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
        for (let i = 0; i < 32; i++) {
            text += possible.charAt(Math.floor(Math.random() * possible.length));
        }
        return text;
    }

    private _isInitialized = false;
    private _refreshTimeout?: NodeJS.Timeout;

    constructor(
        private readonly languageClient: LanguageClient,
        private readonly _extensionUri: Uri,
        private readonly _extractService: DslExtractService,
        private readonly _domainService: DomainsViewService,
        private readonly _context: ExtensionContext
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
            localResourceRoots: [
                Uri.joinPath(this._extensionUri, 'dist')
            ]
        };

        // Set up the React webview HTML
        webviewView.webview.html = this.getWebviewContent();

        this.updateCurrentFile();

        // Defer the initial load to give the language server time to discover documents
        this.deferredRefresh(1000); // Wait 1 second on initial load
        
        // Load domains asynchronously
        this.refreshDomains().then(() => {
            this._isInitialized = true; // Mark as initialized after first load
            this.sendInitialData();
        });

        // Handle messages from the webview
        webviewView.webview.onDidReceiveMessage(async (data) => {
            switch (data.type) {
                case WebviewMessages.PREVIEW:
                    this.handlePreview(data.selectedDomains, data.selectedUseCases, data.diagramMode);
                    break;
                case WebviewMessages.REFRESH:
                    this.handleRefresh();
                    break;
                case WebviewMessages.SET_VIEW_MODE:
                    this.handleSetViewMode(data.viewMode);
                    break;
                case WebviewMessages.READY:
                    this.sendInitialData();
                    break;
                case WebviewMessages.SHOW_INFORMATION:
                    window.showInformationMessage(data.message);
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
            Logger.debug('Current file updated to:', this._state.currentFile);
            
            // Only refresh if file actually changed and we're in current file mode
            if (this._isInitialized && 
                this._state.currentFile !== previousFile && 
                this._state.viewMode === 'current') {
                Logger.debug('File changed from', previousFile, 'to', this._state.currentFile, '- refresh needed');
                return true; // Refresh needed
            } else {
                Logger.debug('No refresh needed - same file or not in current mode');
                return false; // No refresh needed
            }
        } else {
            // We switched to a non-DSL file or panel (like preview)
            // Keep the current file state - don't set it to undefined
            // This maintains the trees showing the last DSL file's content
            Logger.debug('Switched to non-DSL file/panel, maintaining current file state:', this._state.currentFile);
            
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
                Logger.warn('Skipping refresh due to invalid DSL content during editing');
            }
        }, delay);
    }

    private async refreshDomains() {
        try {
            const { domains, serviceGroups } = await this._extractService.discoverDSL({ currentFile: this._state.currentFile });
            
            // Store service groups for service block range lookup
            this._serviceGroups = serviceGroups;

            // Update both current file and workspace domains with preserved states
            // Create deep copies to avoid shared references
            const currentDomains = domains.filter(d => d.inCurrentFile).map(d => this.deepCopyDomainCurrentFile(d));
            const workspaceDomains = domains.map(d => this.deepCopyDomainWorkspace(d));

            // Preserve existing states for current file domains
            currentDomains.forEach(domain => {
                const existingDomain = this._state.currentFileDomains.get(domain.name);
                if (existingDomain) {
                    this.preserveDomainStates(domain, existingDomain);
                }
                this._state.currentFileDomains.set(domain.name, domain);
            });

            // Preserve existing states for workspace domains
            workspaceDomains.forEach(domain => {
                const existingDomain = this._state.workspaceDomains.get(domain.name);
                if (existingDomain) {
                    this.preserveDomainStates(domain, existingDomain);
                }
                this._state.workspaceDomains.set(domain.name, domain);
            });

            // Remove domains that no longer exist from current file
            const currentDomainIds = new Set(currentDomains.map(d => d.name));
            for (const [domainName] of this._state.currentFileDomains) {
                if (!currentDomainIds.has(domainName)) {
                    this._state.currentFileDomains.delete(domainName);
                }
            }

            // Remove domains that no longer exist from workspace
            const workspaceDomainIds = new Set(workspaceDomains.map(d => d.name));
            for (const [domainName] of this._state.workspaceDomains) {
                if (!workspaceDomainIds.has(domainName)) {
                    this._state.workspaceDomains.delete(domainName);
                }
            }

            // Only update webview if already initialized (to avoid double updates)
            if (this._isInitialized) {
                this.sendDataRefresh();
            }
        } catch (error) {
            Logger.error('Error refreshing domains:', error);
            window.showErrorMessage(`Failed to refresh domains: ${error}`);
        }
    }

    private async handlePreview(selectedDomains: Domain[], selectedUseCases: UseCase[], diagramMode: string = 'detailed') {       
        // Collect use case block ranges
        const blockRanges = selectedUseCases.map(uc => uc.blockRange);
        
        // Collect service block ranges for selected subdomains
        const serviceBlockRanges = this.getServiceBlockRangesForSubDomains(selectedDomains);
        blockRanges.push(...serviceBlockRanges);
        
        const partialDsl: string = await this.languageClient.sendRequest('workspace/executeCommand', {
            command: ServerCommands.EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES,
            arguments: [blockRanges]
        });
        Logger.debug('Partial DSL extracted:', partialDsl);
        
        // Choose diagram type based on mode
        const diagramType = diagramMode === 'architecture' ? 'Architecture' : 'Domain';
        commands.executeCommand('craft.previewPartialDSL', partialDsl, diagramType);
    }

    private getServiceBlockRangesForSubDomains(selectedDomains: Domain[]): BlockRange[] {
        const serviceBlockRanges: BlockRange[] = [];
        const processedServices = new Set<string>(); // Avoid duplicate services
        
        // Get all selected subdomain names
        const selectedSubDomainNames = new Set<string>();
        selectedDomains.forEach(domain => {
            domain.subDomains.forEach(subDomain => {
                if (subDomain.selected) {
                    selectedSubDomainNames.add(subDomain.name);
                }
            });
        });
        
        // Find services that contain the selected subdomains
        this._serviceGroups.forEach(serviceGroup => {
            serviceGroup.services.forEach(service => {
                // Check if this service contains any of the selected subdomains
                const hasSelectedSubDomain = service.subDomains.some(subDomain => 
                    selectedSubDomainNames.has(subDomain.name)
                );
                
                // If this service contains selected subdomains and we haven't processed it yet
                if (hasSelectedSubDomain && !processedServices.has(service.id)) {
                    serviceBlockRanges.push(service.blockRange);
                    processedServices.add(service.id);
                }
            });
        });
        
        return serviceBlockRanges;
    }

    private async handleRefresh() {
        await this.refreshDomains();
        this.sendDataRefresh();
    }

    private async handleSetViewMode(viewMode: 'current' | 'workspace') {
        this._state.viewMode = viewMode;
        await this.refreshDomains();
        this.sendDataRefresh();
    }

    private sendDataRefresh() {
        if (!this._view) return;

        const domains = Array.from(this.getDomainsMap().values());
        const visibleDomains = this._state.viewMode === 'current' 
            ? domains.filter(d => d.inCurrentFile)
            : domains;

        const filteredDomains = this.ensureValidDomains(this.filterDomainsChildren(visibleDomains));

        this._view.webview.postMessage({
            type: ProviderMessages.DATA_REFRESH,
            data: {
                domains: filteredDomains,
                currentFile: this._state.currentFile,
                viewMode: this._state.viewMode
            }
        });
    }

    public sendSelectionCommand(action: 'selectAll' | 'selectNone' | 'selectCurrentFile') {
        if (!this._view) return;
        this._view.webview.postMessage({
            type: ProviderMessages.SELECTION_COMMAND,
            action: action
        });
    }

    public sendRefreshCommand() {
        if (!this._view) return;
        this._view.webview.postMessage({
            type: ProviderMessages.REFRESH_COMMAND
        });
    }

    public sendPreviewCommand() {
        if (!this._view) return;
        this._view.webview.postMessage({
            type: ProviderMessages.PREVIEW_COMMAND
        });
    }


    public sendToggleOptionsCommand() {
        if (!this._view) return;
        this._view.webview.postMessage({
            type: ProviderMessages.TOGGLE_OPTIONS_COMMAND
        });
    }

    private sendInitialData() {
        if (!this._view) return;

        const domains = Array.from(this.getDomainsMap().values());
        const visibleDomains = this._state.viewMode === 'current' 
            ? domains.filter(d => d.inCurrentFile)
            : domains;

        const filteredDomains = this.ensureValidDomains(this.filterDomainsChildren(visibleDomains));

        this._view.webview.postMessage({
            type: ProviderMessages.INITIAL_DATA,
            data: {
                domains: filteredDomains,
                currentFile: this._state.currentFile,
                viewMode: this._state.viewMode
            }
        });
    }

    private ensureValidDomains(domains: Domain[]): Domain[] {
        if (!domains) return [];
        
        return domains.map(domain => ({
            ...domain,
            subDomains: (domain.subDomains || []).map(subDomain => ({
                ...subDomain,
                useCases: subDomain.useCases || []
            }))
        }));
    }

    private filterDomainsChildren(domains: Domain[]): Domain[] {
        if (this._state.viewMode === 'workspace') {
            // In workspace mode, show all children
            return domains;
        }

        // In current mode, filter out children not in current file
        return domains.map(domain => ({
            ...domain,
            subDomains: domain.subDomains
                .filter(subDomain => subDomain.inCurrentFile)
                .map(subDomain => ({
                    ...subDomain,
                    useCases: subDomain.useCases
                }))
        }));
    }

    // Helper method to preserve domain states  
    private preserveDomainStates(domain: Domain, existingDomain: Domain) {
        // Only preserve basic structural states, not selection since React manages those
        domain.expanded = existingDomain.expanded;
        domain.subDomains.forEach(subDomain => {
            const existingSubDomain = existingDomain.subDomains.find(sd => sd.id === subDomain.id);
            if (existingSubDomain) {
                subDomain.expanded = existingSubDomain.expanded;
                subDomain.showReferences = existingSubDomain.showReferences;
            }
        });
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
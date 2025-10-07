import { Uri, WebviewViewProvider, WebviewView, WebviewViewResolveContext, CancellationToken, TextDocument, window, workspace, commands, ExtensionContext } from 'vscode';
import { ServicesViewService } from '../services/servicesViewService';
import { DslExtractService } from '../services/dslExtractService';
import { ServiceTreeState, ServiceGroup, Service, UseCase, SubDomain } from '../types/domain';
import { LanguageClient } from 'vscode-languageclient/node';
import { ServerCommands } from '../../../shared/lib/types/domain-extraction';
import { WebviewMessages, ProviderMessages, SelectionActions } from '../types/messages';
import { Logger } from '../utils/Logger';

export class ServicesViewProvider implements WebviewViewProvider {
    public static readonly viewType = 'dslServicesView';
    
    private _view?: WebviewView;
    private _state: ServiceTreeState = {
        currentFileServiceGroups: new Map(),
        workspaceServiceGroups: new Map(),
        viewMode: 'current',
        boundariesMode: 'boundaries',
        showDatabases: true,
        optionsExpanded: false,
        expandedNodes: new Set(),
        selectedNodes: new Set(),
        currentFile: undefined,
        isLoading: false,
    };

    // Helper method to get the appropriate service groups map based on view mode
    private getServiceGroupsMap(): Map<string, ServiceGroup> {
        return this._state.viewMode === 'current' 
            ? this._state.currentFileServiceGroups 
            : this._state.workspaceServiceGroups;
    }

    // Helper method to get both service groups maps for dual updates
    private getBothServiceGroupsMaps(): Map<string, ServiceGroup>[] {
        return [this._state.currentFileServiceGroups, this._state.workspaceServiceGroups];
    }

    private getWebviewContent(): string {
        if (!this._view) return '';

        const scriptUri = this._view.webview.asWebviewUri(
            Uri.joinPath(this._extensionUri, 'dist', 'webview', 'services.js')
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
            <title>Services View</title>
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
        private readonly _serviceTreeService: ServicesViewService,
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

        // Show loading state initially
        this._state.isLoading = true;

        this.updateCurrentFile();

        // Defer the initial load to give the language server time to discover documents
        this.deferredRefresh(1000); // Wait 1 second on initial load
        
        // Load domains asynchronously
        this.refreshServices().then(() => {
            this._isInitialized = true; // Mark as initialized after first load
            this.sendInitialData();
        });

        // Handle messages from the webview
        webviewView.webview.onDidReceiveMessage(async (data) => {
            switch (data.type) {
                case WebviewMessages.PREVIEW:
                    this.handlePreview(data.selectedServices, data.selectedUseCases, data.focusInfo);
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
            this.refreshServices();
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
                this.refreshServices();
            } catch (error) {
                // If validation fails, don't refresh to avoid flickering
                Logger.warn('Skipping refresh due to invalid DSL content during editing');
            }
        }, delay);
    }

    private async refreshServices() {
        try {
            const { serviceGroups } = await this._extractService.discoverDSL({ currentFile: this._state.currentFile });

            // Update both current file and workspace service groups with preserved states
            // Create deep copies to avoid shared references
            const currentServiceGroups = serviceGroups.filter(sg => sg.inCurrentFile).map(sg => this.deepCopyServiceGroupCurrentFile(sg));
            const workspaceServiceGroups = serviceGroups.map(sg => this.deepCopyServiceGroupWorkspace(sg));

            // Preserve existing states for current file service groups
            currentServiceGroups.forEach(serviceGroup => {
                const existingServiceGroup = this._state.currentFileServiceGroups.get(serviceGroup.name);
                if (existingServiceGroup) {
                    this.preserveServiceGroupStates(serviceGroup, existingServiceGroup);
                }
                this._state.currentFileServiceGroups.set(serviceGroup.name, serviceGroup);
            });

            // Preserve existing states for workspace service groups
            workspaceServiceGroups.forEach(serviceGroup => {
                const existingServiceGroup = this._state.workspaceServiceGroups.get(serviceGroup.name);
                if (existingServiceGroup) {
                    this.preserveServiceGroupStates(serviceGroup, existingServiceGroup);
                }
                this._state.workspaceServiceGroups.set(serviceGroup.name, serviceGroup);
            });

            // Remove service groups that no longer exist from current file
            const currentGroupIds = new Set(currentServiceGroups.map(sg => sg.name));
            for (const [groupName] of this._state.currentFileServiceGroups) {
                if (!currentGroupIds.has(groupName)) {
                    this._state.currentFileServiceGroups.delete(groupName);
                }
            }

            // Remove service groups that no longer exist from workspace
            const workspaceGroupIds = new Set(workspaceServiceGroups.map(sg => sg.name));
            for (const [groupName] of this._state.workspaceServiceGroups) {
                if (!workspaceGroupIds.has(groupName)) {
                    this._state.workspaceServiceGroups.delete(groupName);
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


    private async handlePreview(selectedServices: any[], selectedUseCases: any[], focusInfo: any) {
        const blockRanges = [];
        selectedServices.forEach(s => blockRanges.push(s.blockRange));
        selectedUseCases.forEach(uc => blockRanges.push(uc.blockRange));
        
        const partialDsl: string = await this.languageClient.sendRequest('workspace/executeCommand', {
            command: ServerCommands.EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES,
            arguments: [blockRanges]
        });
        Logger.debug('Partial DSL extracted:', partialDsl);
        
        commands.executeCommand('craft.previewC4PartialDSL', partialDsl, focusInfo);
    }


    private async handleRefresh() {
        await this.refreshServices();
        this.sendDataRefresh();
    }

    private async handleSetViewMode(viewMode: 'current' | 'workspace') {
        this._state.viewMode = viewMode;
        await this.refreshServices();
        this.sendDataRefresh();
    }


    private sendDataRefresh() {
        if (!this._view) return;

        const serviceGroups = Array.from(this.getServiceGroupsMap().values());
        const visibleGroups = this._state.viewMode === 'current' 
            ? serviceGroups.filter(d => d.inCurrentFile)
            : serviceGroups;

        const filteredGroups = this.filterServiceGroupChildren(visibleGroups);

        this._view.webview.postMessage({
            type: ProviderMessages.DATA_REFRESH,
            data: {
                serviceGroups: filteredGroups,
                currentFile: this._state.currentFile,
                viewMode: this._state.viewMode
            }
        });
    }

    public sendSelectionCommand(action: 'selectAll' | 'selectNone') {
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

        const serviceGroups = Array.from(this.getServiceGroupsMap().values());
        const visibleGroups = this._state.viewMode === 'current' 
            ? serviceGroups.filter(d => d.inCurrentFile)
            : serviceGroups;

        const filteredGroups = this.filterServiceGroupChildren(visibleGroups);

        this._view.webview.postMessage({
            type: ProviderMessages.INITIAL_DATA,
            data: {
                serviceGroups: filteredGroups,
                currentFile: this._state.currentFile,
                viewMode: this._state.viewMode
            }
        });
    }

    private filterServiceGroupChildren(serviceGroups: ServiceGroup[]): ServiceGroup[] {
        if (this._state.viewMode === 'workspace') {
            // In workspace mode, show all children (HTML generator will apply grey styling)
            return serviceGroups;
        }

        // In current mode, filter out children not in current file
        return serviceGroups.map(group => ({
            ...group,
            services: group.services
                .filter(service => service.inCurrentFile)
                .map(service => ({
                    ...service,
                    subDomains: service.subDomains.filter(subDomain => subDomain.inCurrentFile)
                }))
        }));
    }


    // Helper method to preserve service group states  
    private preserveServiceGroupStates(serviceGroup: ServiceGroup, existingServiceGroup: ServiceGroup) {
        // Only preserve basic structural states, not selection/focus since React manages those
        serviceGroup.expanded = existingServiceGroup.expanded;
        serviceGroup.services.forEach(service => {
            const existingService = existingServiceGroup.services.find(s => s.id === service.id);
            if (existingService) {
                service.expanded = existingService.expanded;
                
                service.subDomains.forEach(subDomain => {
                    const existingSubDomain = existingService.subDomains.find(sd => sd.id === subDomain.id);
                    if (existingSubDomain) {
                        subDomain.expanded = existingSubDomain.expanded;
                    }
                });
            }
        });
    }

    private deepCopyServiceGroupCurrentFile(serviceGroup: ServiceGroup): ServiceGroup {
        return this.deepCopyServiceGroup(serviceGroup, true);
    }

    private deepCopyServiceGroupWorkspace(serviceGroup: ServiceGroup): ServiceGroup {
        return this.deepCopyServiceGroup(serviceGroup, false);
    }

    // Helper method to create deep copies of service groups to avoid shared references
    private deepCopyServiceGroup(serviceGroup: ServiceGroup, inCurrentFileFilter: boolean): ServiceGroup {
        return {
            ...serviceGroup,
            services: serviceGroup.services.filter(service => inCurrentFileFilter === true ? service.inCurrentFile : true).map(service => ({
                ...service,
                domain: { ...service.domain },
                subDomains: service.subDomains.map(subDomain => ({
                    ...subDomain,
                    useCases: subDomain.useCases.map(useCase => ({ ...useCase })),
                    referencedIn: subDomain.referencedIn.map(ref => ({ ...ref }))
                }))
            }))
        };
    }
}
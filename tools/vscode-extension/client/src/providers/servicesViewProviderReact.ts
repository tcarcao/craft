import { Uri, WebviewView, WebviewViewProvider, ExtensionContext } from 'vscode';
import { LanguageClient } from 'vscode-languageclient/node';
import { ServiceTreeState, ServiceGroup } from '../types/domain';
import { ServicesViewService } from '../services/servicesViewService';
import { DslExtractService } from '../services/dslExtractService';
import { Logger } from '../utils/Logger';

export class ServicesViewProviderReact implements WebviewViewProvider {
    public static readonly viewType = 'dslServicesViewReact';
    
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

    constructor(
        private readonly languageClient: LanguageClient,
        private readonly _extensionUri: Uri,
        private readonly _extractService: DslExtractService,
        private readonly _serviceTreeService: ServicesViewService,
        private readonly _context: ExtensionContext
    ) {}

    public resolveWebviewView(webviewView: WebviewView): void {
        this._view = webviewView;

        webviewView.webview.options = {
            enableScripts: true,
            localResourceRoots: [
                Uri.joinPath(this._extensionUri, 'dist')
            ]
        };

        // Set up the webview HTML
        webviewView.webview.html = this.getWebviewContent();

        // Handle messages from the webview
        webviewView.webview.onDidReceiveMessage(data => {
            this.handleMessage(data);
        });

        // Initialize the view
        this.initialize();
    }

    private getWebviewContent(): string {
        if (!this._view) return '';

        const scriptUri = this._view.webview.asWebviewUri(
            Uri.joinPath(this._extensionUri, 'dist', 'webview', 'services.js')
        );

        const codiconsUri = this._view.webview.asWebviewUri(
            Uri.joinPath(this._extensionUri, 'dist', '@vscode/codicons', 'dist', 'codicon.css')
        );

        // Generate a nonce for security
        const nonce = this.getNonce();

        return `<!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${this._view.webview.cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}';">
            <link href="${codiconsUri}" rel="stylesheet">
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

    private handleMessage(data: any): void {
        switch (data.type) {
            case 'ready':
                this.sendInitialState();
                break;
            case 'setDatabaseVisibility':
                this._state.showDatabases = data.show;
                this.sendStateUpdate('showDatabases', data.show);
                break;
            case 'toggleDiagramOptions':
                this._state.optionsExpanded = !this._state.optionsExpanded;
                this.sendStateUpdate('optionsExpanded', this._state.optionsExpanded);
                break;
            case 'setViewMode':
                this._state.viewMode = data.mode;
                this.sendFullState();
                break;
            case 'setBoundariesMode':
                this._state.boundariesMode = data.mode;
                this.sendStateUpdate('boundariesMode', data.mode);
                break;
            case 'refresh':
                this.initialize();
                break;
            case 'preview':
                // TODO: Implement preview functionality
                break;
            case 'toggleServiceGroup':
                this.handleToggleServiceGroup(data.groupId);
                break;
            case 'toggleService':
                this.handleToggleService(data.serviceGroupId, data.serviceId);
                break;
        }
    }

    private sendInitialState(): void {
        this.sendFullState();
    }

    private sendFullState(): void {
        const visibleGroups = this.getVisibleServiceGroups();
        
        if (this._view) {
            this._view.webview.postMessage({
                type: 'updateState',
                data: {
                    serviceGroups: Array.from(visibleGroups),
                    viewMode: this._state.viewMode,
                    boundariesMode: this._state.boundariesMode,
                    showDatabases: this._state.showDatabases,
                    optionsExpanded: this._state.optionsExpanded,
                    isLoading: false
                }
            });
        }
    }

    private sendStateUpdate(key: string, value: any): void {
        if (this._view) {
            this._view.webview.postMessage({
                type: 'stateUpdate',
                updateType: key,
                data: { [key]: value }
            });
        }
    }

    private getVisibleServiceGroups(): ServiceGroup[] {
        const serviceGroupsMap = this._state.viewMode === 'current' 
            ? this._state.currentFileServiceGroups 
            : this._state.workspaceServiceGroups;
        
        return Array.from(serviceGroupsMap.values());
    }

    private handleToggleServiceGroup(groupId: string): void {
        const serviceGroupsMap = this.getServiceGroupsMap();
        const group = serviceGroupsMap.get(groupId);
        if (group) {
            group.expanded = !group.expanded;
            this.sendFullState();
        }
    }

    private handleToggleService(serviceGroupId: string, serviceId: string): void {
        const serviceGroupsMap = this.getServiceGroupsMap();
        const group = serviceGroupsMap.get(serviceGroupId);
        const service = group?.services.find(s => s.id === serviceId);
        
        if (service) {
            service.selected = !service.selected;
            // Update group selection state
            this.updateGroupSelectionState(group!);
            this.sendFullState();
        }
    }

    private getServiceGroupsMap(): Map<string, ServiceGroup> {
        return this._state.viewMode === 'current' 
            ? this._state.currentFileServiceGroups 
            : this._state.workspaceServiceGroups;
    }

    private updateGroupSelectionState(group: ServiceGroup): void {
        const selectedServices = group.services.filter(s => s.selected);
        group.selected = selectedServices.length === group.services.length;
        group.partiallySelected = selectedServices.length > 0 && selectedServices.length < group.services.length;
    }

    private async initialize(): Promise<void> {
        this._state.isLoading = true;
        
        try {
            // Get data using the same method as the original provider
            const { serviceGroups } = await this._extractService.discoverDSL({ currentFile: this._state.currentFile });
            
            // Update state maps - simplified version for now
            this._state.workspaceServiceGroups.clear();
            this._state.currentFileServiceGroups.clear();
            
            serviceGroups.forEach(group => {
                this._state.workspaceServiceGroups.set(group.name, group);
                if (group.inCurrentFile) {
                    this._state.currentFileServiceGroups.set(group.name, group);
                }
            });

            this.sendFullState();
        } catch (error) {
            Logger.error('Failed to initialize services view:', error);
            this._state.isLoading = false;
            this.sendFullState();
        }
    }
}
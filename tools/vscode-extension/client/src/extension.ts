// client/src/extension.ts
import * as path from 'path';
import { workspace, ExtensionContext, window, languages } from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';
import { registerPreviewCommands, cleanUpPreviewCommands } from './commands';
import { DomainsViewProvider } from './providers/domainsViewProvider';
import { DomainsViewService } from './services/domainsViewService';
import { ServicesViewProvider } from './providers/servicesViewProvider';
import { DslExtractService } from './services/dslExtractService';
import { ServicesViewService } from './services/servicesViewService';
import { TreeSitterHighlightProvider } from './TreeSitterHighlightProvider';

let domainTreeProvider: DomainsViewProvider;
let serviceTreeProvider: ServicesViewProvider;
let client: LanguageClient;

export function activate(context: ExtensionContext) {
    startLanguageServer(context);
    registerDomainView(context, client);
    registerPreviewCommands(context);
    registerTreeSitterHighlighting(context);
}

function registerTreeSitterHighlighting(context: ExtensionContext) {
    console.log('🔄 Registering Tree-sitter semantic highlighting for Craft...');
    
    // Register Tree-sitter semantic highlighting for Craft files
    const highlightProvider = new TreeSitterHighlightProvider();
    
    const disposable = languages.registerDocumentSemanticTokensProvider(
        { language: 'craft' },
        highlightProvider,
        highlightProvider.legend
    );
    
    context.subscriptions.push(disposable);
    console.log('✅ Tree-sitter syntax highlighting registered for Craft language');
    console.log('📋 Token legend:', highlightProvider.legend);
}

function startLanguageServer(context: ExtensionContext) {
    const serverModule = context.asAbsolutePath(
		path.join('server', 'out', 'server.js')
	);

    const serverOptions: ServerOptions = {
        run: { module: serverModule, transport: TransportKind.ipc },
        debug: {
            module: serverModule,
            transport: TransportKind.ipc,
            options: { execArgv: ['--nolazy', '--inspect=6009'] }
        }
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [{ scheme: 'file', language: 'craft' }],
        synchronize: {
            // Notify the server about file changes to '.clientrc files contained in the workspace
            fileEvents: workspace.createFileSystemWatcher('**/.clientrc')
        }
    };

    client = new LanguageClient(
        'craftLanguageServer',
        'Craft Language Server',
        serverOptions,
        clientOptions
    );

    client.start();
}

function registerDomainView(context: ExtensionContext, client: LanguageClient) {
    // Initialize services
    const extractService = new DslExtractService(client);
    const domainService = new DomainsViewService();
    const serviceTreeService = new ServicesViewService();
    
    // Register the Domain Tree view provider
    domainTreeProvider = new DomainsViewProvider(
        client,
        context.extensionUri,
        extractService,
        domainService,
        context
    );

    serviceTreeProvider = new ServicesViewProvider(
        client,
        context.extensionUri,
        extractService,
        serviceTreeService,
        context
    );
    
    context.subscriptions.push(
        window.registerWebviewViewProvider(
            DomainsViewProvider.viewType, 
            domainTreeProvider
        ),
        window.registerWebviewViewProvider(
            ServicesViewProvider.viewType, 
            serviceTreeProvider
        ),
    );
}

export function deactivate(): Thenable<void> | undefined {
    cleanUpPreviewCommands();

	if (!client) {
		return undefined;
	}
	return client.stop();
}
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
import { Logger } from './utils/Logger';

let domainTreeProvider: DomainsViewProvider;
let serviceTreeProvider: ServicesViewProvider;
let client: LanguageClient;

export function activate(context: ExtensionContext) {
    registerTreeSitterHighlighting(context);
    startLanguageServer(context).then(() => {
        registerDomainView(context, client);
    });
}

function registerTreeSitterHighlighting(context: ExtensionContext) {
    Logger.info('🔄 Registering Tree-sitter semantic highlighting for Craft...');
    
    // Register Tree-sitter semantic highlighting for Craft files
    const highlightProvider = new TreeSitterHighlightProvider();
    
    const disposable = languages.registerDocumentSemanticTokensProvider(
        { language: 'craft' },
        highlightProvider,
        highlightProvider.legend
    );
    
    context.subscriptions.push(disposable);
    Logger.info('✅ Tree-sitter syntax highlighting registered for Craft language');
    Logger.debug('📋 Token legend:', highlightProvider.legend);
}

async function startLanguageServer(context: ExtensionContext) {
    const serverModule = context.asAbsolutePath(
		path.join('dist', 'server.js')
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
            fileEvents: workspace.createFileSystemWatcher('**/.clientrc'),
            configurationSection: 'craft'
        },
        initializationOptions: {
            logLevel: workspace.getConfiguration('craft').get('logging.level', 'warn')
        }
    };

    client = new LanguageClient(
        'craftLanguageServer',
        'Craft Language Server',
        serverOptions,
        clientOptions
    );

    await client.start();
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
    
    // Register commands after providers are initialized
    registerPreviewCommands(context, domainTreeProvider, serviceTreeProvider);
}

export function deactivate(): Thenable<void> | undefined {
    cleanUpPreviewCommands();

	if (!client) {
		return undefined;
	}
	return client.stop();
}
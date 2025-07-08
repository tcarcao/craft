// client/src/extension.ts
import * as path from 'path';
import { workspace, ExtensionContext, window } from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';
import { registerPreviewCommands, cleanUpPreviewCommands } from './commands';
import { DomainsViewProvider } from './providers/domainsViewProvider';
import { DomainsViewService } from './services/domainsViewService';
import { DomainsViewHtmlGenerator } from './ui/domainsViewHtmlGenerator';
import { DslExtractService } from './services/dslExtractService';

let domainTreeProvider: DomainsViewProvider;
let client: LanguageClient;

export function activate(context: ExtensionContext) {
    startLanguageServer(context);
    registerDomainView(context, client);
    registerPreviewCommands(context);
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
        documentSelector: [{ scheme: 'file', language: 'archdsl' }],
        synchronize: {
            // Notify the server about file changes to '.clientrc files contained in the workspace
            fileEvents: workspace.createFileSystemWatcher('**/.clientrc')
        }
    };

    client = new LanguageClient(
        'archdslLanguageServer',
        'ArchDSL Language Server',
        serverOptions,
        clientOptions
    );

    client.start();
}

function registerDomainView(context: ExtensionContext, client: LanguageClient) {
    // Initialize services
    const extractService = new DslExtractService(client);
    const domainService = new DomainsViewService();
    const htmlGenerator = new DomainsViewHtmlGenerator();
    
    // Register the Domain Tree view provider
    domainTreeProvider = new DomainsViewProvider(
        client,
        context.extensionUri,
        extractService,
        domainService,
        htmlGenerator
    );
    
    context.subscriptions.push(
        window.registerWebviewViewProvider(
            DomainsViewProvider.viewType, 
            domainTreeProvider
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
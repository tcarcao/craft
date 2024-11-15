// client/src/extension.ts
import * as path from 'path';
import { workspace, ExtensionContext } from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';
import { registerPreviewCommands, cleanUpPreviewCommands } from './commands';

let client: LanguageClient;

export function activate(context: ExtensionContext) {
    // Setup server
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

    // Register commands
    registerPreviewCommands(context, client);

    client.start();
}

export function deactivate(): Thenable<void> | undefined {
    cleanUpPreviewCommands();

	if (!client) {
		return undefined;
	}
	return client.stop();
}
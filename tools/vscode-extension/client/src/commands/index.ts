// client/src/commands/index.ts
import { commands, ExtensionContext } from 'vscode';
import { LanguageClient } from 'vscode-languageclient/node';
import { handlePreviewC4, cleanUpPreviewC4 } from './previewC4';
import { handlePreviewContext } from './previewContext';
import { handlePreviewSequence } from './previewSequence';

export function registerPreviewCommands(context: ExtensionContext, client: LanguageClient) {
    context.subscriptions.push(
        commands.registerCommand('archdsl.previewC4', () => 
            handlePreviewC4(client)
        ),
        commands.registerCommand('archdsl.previewContext', () => 
            handlePreviewContext(client)
        ),
        commands.registerCommand('archdsl.previewSequence', () => 
            handlePreviewSequence(client)
        )
    );
}

export function cleanUpPreviewCommands() {
    cleanUpPreviewC4();
}
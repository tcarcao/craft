// client/src/commands/index.ts
import { commands, ExtensionContext } from 'vscode';
import { handlePreviewC4, handlePreviewSelectedC4, handlePreviewPartialC4, cleanUpPreviewC4 } from './previewC4';
import { handlePreviewContext } from './previewContext';
import { handlePreviewSequence } from './previewSequence';
import { handlePreviewDomain, handlePreviewDomainsFromSelection, handlePreviewPartialDomains, cleanUpPreviewDomain } from './previewDomain';

export function registerPreviewCommands(context: ExtensionContext) {
    context.subscriptions.push(
        commands.registerCommand('archdsl.previewC4', () =>
            handlePreviewC4()
        ),
        commands.registerCommand('archdsl.previewSelectedC4', () =>
            handlePreviewSelectedC4()
        ),
        commands.registerCommand('archdsl.previewContext', () =>
            handlePreviewContext()
        ),
        commands.registerCommand('archdsl.previewSequence', () =>
            handlePreviewSequence()
        ),
        commands.registerCommand('archdsl.previewDomain', () =>
            handlePreviewDomain()
        ),
        commands.registerCommand('archdsl.previewDomainsFromSelection', () =>
            handlePreviewDomainsFromSelection()
        ),
        commands.registerCommand('archdsl.previewPartialDSL', (partialDSL, diagramType) => {
            switch(diagramType) {
                case "C4":
                    handlePreviewPartialC4(partialDSL);
                    break;
                case "Domain":
                default:
                    handlePreviewPartialDomains(partialDSL);
                    break;
            }
        })
    );
}

export function cleanUpPreviewCommands() {
    cleanUpPreviewC4();
    cleanUpPreviewDomain();
}
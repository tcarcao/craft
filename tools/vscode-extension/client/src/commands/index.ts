// client/src/commands/index.ts
import { commands, ExtensionContext } from 'vscode';
import { handlePreviewC4, handlePreviewSelectedC4, handlePreviewPartialC4, cleanUpPreviewC4 } from './previewC4';
import { handlePreviewDomain, handlePreviewDomainsFromSelection, handlePreviewPartialDomains, cleanUpPreviewDomain } from './previewDomain';

export function registerPreviewCommands(context: ExtensionContext) {
    context.subscriptions.push(
        commands.registerCommand('craft.previewC4', () =>
            handlePreviewC4()
        ),
        commands.registerCommand('craft.previewSelectedC4', () =>
            handlePreviewSelectedC4()
        ),
        commands.registerCommand('craft.previewDomain', () =>
            handlePreviewDomain()
        ),
        commands.registerCommand('craft.previewDomainsFromSelection', () =>
            handlePreviewDomainsFromSelection()
        ),
        commands.registerCommand('craft.previewPartialDSL', (partialDSL, diagramType) => {
            switch(diagramType) {
                case "C4":
                    handlePreviewPartialC4(partialDSL);
                    break;
                case "Domain":
                default:
                    handlePreviewPartialDomains(partialDSL);
                    break;
            }
        }),
        commands.registerCommand('craft.previewPartialDSLWithFocus', (partialDSL, diagramType, focusInfo) => {
            switch(diagramType) {
                case "C4":
                    handlePreviewPartialC4(partialDSL, focusInfo);
                    break;
                case "Domain":
                default:
                    handlePreviewPartialDomains(partialDSL);
                    break;
            }
        }),
        commands.registerCommand('craft.openSettings', () => {
            commands.executeCommand('workbench.action.openSettings', 'craft.');
        })
    );
}

export function cleanUpPreviewCommands() {
    cleanUpPreviewC4();
    cleanUpPreviewDomain();
}
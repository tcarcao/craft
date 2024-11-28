import { window, ViewColumn, WebviewPanel } from 'vscode';
import { LanguageClient } from 'vscode-languageclient/node';
import { updatePreview } from './previewCommon';

let previewPanel: WebviewPanel | undefined;

export async function handlePreviewC4(_client: LanguageClient) {
    const activeEditor = window.activeTextEditor;
        if (!activeEditor) {
            window.showErrorMessage('No active editor');
            return;
        }

        // Create and show panel
        if (!previewPanel) {
            previewPanel = window.createWebviewPanel(
                'c4Preview',
                'C4 Preview',
                ViewColumn.Beside,
                {
                    enableScripts: true,
                    retainContextWhenHidden: true
                }
            );

            // Reset panel when disposed
            previewPanel.onDidDispose(() => {
                previewPanel = undefined;
            });
        }

        // Show the panel
        previewPanel.reveal(ViewColumn.Beside);

        // Update content
        updatePreview(previewPanel, activeEditor.document, "C4");
}

export function cleanUpPreviewC4() {
    if (previewPanel) {
        previewPanel.dispose();
    }
}

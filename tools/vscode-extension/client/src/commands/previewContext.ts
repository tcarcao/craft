import { window, ViewColumn, WebviewPanel } from 'vscode';
import { LanguageClient } from 'vscode-languageclient/node';
import { updatePreview } from './previewCommon';

let previewPanel: WebviewPanel | undefined;

export async function handlePreviewContext(_client: LanguageClient) {
    const activeEditor = window.activeTextEditor;
        if (!activeEditor) {
            window.showErrorMessage('No active editor');
            return;
        }

        // Create and show panel
        if (!previewPanel) {
            previewPanel = window.createWebviewPanel(
                'contextPreview',
                'Context Preview',
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
        updatePreview(previewPanel, activeEditor.document, "Context");
}

export function cleanUpPreviewContext() {
    if (previewPanel) {
        previewPanel.dispose();
    }
}

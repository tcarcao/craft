import { window, ViewColumn, WebviewPanel } from 'vscode';
import { updatePreview, handleDownload } from './previewCommon';

let previewPanel: WebviewPanel | undefined;

export async function handlePreviewContext() {
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

            // Handle messages from webview
            previewPanel.webview.onDidReceiveMessage(
                async (message) => {
                    if (message.command === 'download') {
                        await handleDownload(message);
                    }
                }
            );
        }

        // Show the panel
        previewPanel.reveal(ViewColumn.Beside);

        // Update content
        updatePreview(previewPanel, activeEditor.document.getText(), "Context");
}

export function cleanUpPreviewContext() {
    if (previewPanel) {
        previewPanel.dispose();
    }
}

import { window, ViewColumn, WebviewPanel } from 'vscode';
import { updatePreview } from './previewCommon';

let previewPanel: WebviewPanel | undefined;

export async function handlePreviewSequence() {
    const activeEditor = window.activeTextEditor;
        if (!activeEditor) {
            window.showErrorMessage('No active editor');
            return;
        }

        // Create and show panel
        if (!previewPanel) {
            previewPanel = window.createWebviewPanel(
                'sequencePreview',
                'Sequence Preview',
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
        updatePreview(previewPanel, activeEditor.document.getText(), "Sequence");
}

export function cleanUpPreviewSequence() {
    if (previewPanel) {
        previewPanel.dispose();
    }
}

import { window, ViewColumn, WebviewPanel } from 'vscode';
import { previewC4Diagram, handlePreviewError, handleDownload, C4PreviewOptions } from './previewCommon';
import { Logger } from '../utils/Logger';

const viewType = 'c4Preview';
const panelTitle = 'C4 Preview';

let previewPanel: WebviewPanel | undefined;

export async function handlePreviewC4() {
    const activeEditor = window.activeTextEditor;
    if (!activeEditor) {
        window.showErrorMessage('No active editor');
        return;
    }

    createAndShowPreviewPanel();

    // Update content
    await updateC4Preview(previewPanel, activeEditor.document.getText());
}

export async function handlePreviewSelectedC4() {
    const activeEditor = window.activeTextEditor;
    if (!activeEditor) {
        window.showErrorMessage('No active editor');
        return;
    }

    if (activeEditor.selection.isEmpty) {
        window.showWarningMessage('No text selected. Please select some DSL code to preview.');
        return;
    }

    createAndShowPreviewPanel();

    // Update content
    const selectedText = activeEditor.document.getText(activeEditor.selection);
    await updateC4Preview(previewPanel, selectedText);
}

export async function handlePreviewPartialC4(text: string, focusInfo?: C4PreviewOptions) {
    createAndShowPreviewPanel();
    await updateC4Preview(previewPanel, text, focusInfo);
}


async function updateC4Preview(
    previewPanel: WebviewPanel | undefined,
    text: string,
    options?: C4PreviewOptions
): Promise<void> {
    if (!previewPanel) {
        Logger.debug('Preview panel not available');
        return;
    }

    try {
        await previewC4Diagram(previewPanel, text, options);
    } catch (error: unknown) {
        handlePreviewError(error);
    }
}

function createAndShowPreviewPanel() {
    if (!previewPanel) {
        previewPanel = window.createWebviewPanel(
            viewType,
            panelTitle,
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
}

export function cleanUpPreviewC4() {
    if (previewPanel) {
        previewPanel.dispose();
    }
}

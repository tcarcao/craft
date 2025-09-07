import { window, ViewColumn, WebviewPanel } from 'vscode';
import { updatePreview } from './previewCommon';

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
    updatePreview(previewPanel, activeEditor.document.getText(), "C4");
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
    updatePreview(previewPanel, selectedText, "C4");
}

export async function handlePreviewPartialC4(text: string, focusInfo?: any) {
    createAndShowPreviewPanel();
    updatePreview(previewPanel, text, "C4", focusInfo);
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
    }

    // Show the panel
    previewPanel.reveal(ViewColumn.Beside);
}

export function cleanUpPreviewC4() {
    if (previewPanel) {
        previewPanel.dispose();
    }
}

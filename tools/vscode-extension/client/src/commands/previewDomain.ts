import { window, ViewColumn, WebviewPanel } from 'vscode';
import { updatePreview } from './previewCommon';

const viewType = 'domainPreview';
const panelTitle = 'Domain Preview';

let previewPanel: WebviewPanel | undefined;

export async function handlePreviewDomain() {
    const activeEditor = window.activeTextEditor;
    if (!activeEditor) {
        window.showErrorMessage('No active editor');
        return;
    }

    createAndShowPreviewPanel();

    // Update content
    updatePreview(previewPanel, activeEditor.document.getText(), "Domain");
}

export async function handlePreviewDomainsFromSelection() {
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
    updatePreview(previewPanel, selectedText, "Domain");
}

export async function handlePreviewPartialDomains(text: string) {
    createAndShowPreviewPanel();
    updatePreview(previewPanel, text, "Domain");
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

export function cleanUpPreviewDomain() {
    if (previewPanel) {
        previewPanel.dispose();
    }
}

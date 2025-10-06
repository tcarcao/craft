import { window, ViewColumn, WebviewPanel } from 'vscode';
import { previewDomainDiagram, handlePreviewError, handleDownload, DomainPreviewOptions } from './previewCommon';
import { Logger } from '../utils/Logger';

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
    await updateDomainPreview(previewPanel, activeEditor.document.getText());
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
    await updateDomainPreview(previewPanel, selectedText);
}

export async function handlePreviewPartialDomains(text: string) {
    createAndShowPreviewPanel();
    await updateDomainPreview(previewPanel, text, { domainMode: 'detailed' });
}

export async function handlePreviewPartialArchitecture(text: string) {
    createAndShowPreviewPanel();
    await updateDomainPreview(previewPanel, text, { domainMode: 'architecture' });
}

export async function updateDomainPreview(
    previewPanel: WebviewPanel | undefined,
    text: string,
    options?: DomainPreviewOptions
): Promise<void> {
    if (!previewPanel) {
        Logger.debug('Preview panel not available');
        return;
    }

    try {
        await previewDomainDiagram(previewPanel, text, options);
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

export function cleanUpPreviewDomain() {
    if (previewPanel) {
        previewPanel.dispose();
    }
}

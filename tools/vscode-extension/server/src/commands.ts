// client/src/commands.ts
import * as vscode from 'vscode';
import { LanguageClient } from 'vscode-languageclient/node';

export function registerPreviewCommands(context: vscode.ExtensionContext, client: LanguageClient) {
    // Register diagram preview command
    const previewCommand = vscode.commands.registerCommand('archdsl.preview', () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            vscode.window.showErrorMessage('No active editor found');
            return;
        }

        if (editor.document.languageId !== 'archdsl') {
            vscode.window.showErrorMessage('Active file is not an ArchDSL file');
            return;
        }

        // Create and show preview panel
        const panel = vscode.window.createWebviewPanel(
            'archdslPreview',
            'ArchDSL Preview',
            vscode.ViewColumn.Beside,
            {
                enableScripts: true,
                retainContextWhenHidden: true
            }
        );

        panel.webview.html = getPreviewContent(editor.document.getText());
    });

    context.subscriptions.push(previewCommand);
}

export function cleanUpPreviewCommands() {
    // Clean up any resources if needed
}

function getPreviewContent(dslContent: string): string {
    return `<!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>ArchDSL Preview</title>
        <style>
            body {
                font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
                margin: 20px;
                background-color: #f5f5f5;
            }
            .preview-container {
                background: white;
                border-radius: 8px;
                padding: 20px;
                box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            }
            .preview-content {
                font-family: 'Courier New', monospace;
                white-space: pre-wrap;
                background: #f8f8f8;
                padding: 15px;
                border-radius: 4px;
                border: 1px solid #ddd;
            }
            h1 {
                color: #333;
                border-bottom: 2px solid #007acc;
                padding-bottom: 10px;
            }
        </style>
    </head>
    <body>
        <div class="preview-container">
            <h1>ArchDSL Preview</h1>
            <div class="preview-content">${escapeHtml(dslContent)}</div>
        </div>
    </body>
    </html>`;
}

function escapeHtml(unsafe: string): string {
    return unsafe
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}
// eslint-disable-next-line @typescript-eslint/no-require-imports
const axios = require('axios');
import { window, WebviewPanel, workspace, Uri } from 'vscode';
import { getCraftConfig } from '../utils/config';
import * as fs from 'fs';
import * as path from 'path';

export async function updatePreview(previewPanel: WebviewPanel | undefined, text: string, documentType: string, focusInfo?: any) {
    if (!previewPanel) {
        console.log('not there');
        return;
    }

    try {
        // Get configuration settings
        const { serverUrl, timeout } = getCraftConfig();

        const requestBody: any = {
            DSL: text
        };
        
        // Add focus information if provided
        if (focusInfo) {
            requestBody.focusInfo = focusInfo;
            
            // Add boundaries mode if provided
            if (focusInfo.boundariesMode) {
                requestBody.boundariesMode = focusInfo.boundariesMode;
            }
        }
        
        const { data } = await axios.post(`${serverUrl}/preview/${documentType.toLowerCase()}`, requestBody, {
            headers: {
                'Content-Type': 'application/json'
            },
            timeout: timeout
        });

        const diagram = await data.data;

        // Update webview content
        previewPanel.webview.html = `
            <!DOCTYPE html>
            <html>
            <head>
                <meta charset="UTF-8">
                <meta name="viewport" content="width=device-width, initial-scale=1.0">
                <title>${documentType} Preview</title>
                <style>
                    body {
                        padding: 10px;
                        margin: 0;
                        font-family: var(--vscode-font-family);
                    }
                    .controls {
                        margin-bottom: 10px;
                        padding: 10px;
                        background: var(--vscode-editor-background);
                        border: 1px solid var(--vscode-panel-border);
                        border-radius: 3px;
                    }
                    .download-buttons {
                        display: flex;
                        gap: 8px;
                        align-items: center;
                    }
                    .download-btn {
                        background: var(--vscode-button-background);
                        color: var(--vscode-button-foreground);
                        border: none;
                        padding: 6px 12px;
                        border-radius: 2px;
                        cursor: pointer;
                        font-size: 13px;
                    }
                    .download-btn:hover {
                        background: var(--vscode-button-hoverBackground);
                    }
                    .download-label {
                        font-weight: bold;
                        margin-right: 10px;
                        color: var(--vscode-foreground);
                    }
                    .diagram-wrapper {
                        width: 100%;
                        overflow-x: auto;
                        border: 1px solid var(--vscode-panel-border);
                        border-radius: 3px;
                    }
                    .diagram-wrapper img {
                        max-width: none;
                        display: block;
                    }
                </style>
            </head>
            <body>
                <div class="controls">
                    <div class="download-buttons">
                        <span class="download-label">Download:</span>
                        <button class="download-btn" onclick="downloadDiagram('png')">PNG</button>
                        <button class="download-btn" onclick="downloadDiagram('svg')">SVG</button>
                        <button class="download-btn" onclick="downloadDiagram('pdf')">PDF</button>
                        <button class="download-btn" onclick="downloadDiagram('puml')">PlantUML</button>
                    </div>
                </div>
                <div class="diagram-wrapper">
                    <img src="data:image/png;base64,${diagram}" alt="${documentType} Diagram">
                </div>
                
                <script>
                    const vscode = acquireVsCodeApi();
                    
                    function downloadDiagram(format) {
                        vscode.postMessage({
                            command: 'download',
                            diagramType: '${documentType.toLowerCase()}',
                            format: format,
                            dsl: \`${text.replace(/`/g, '\\`').replace(/\${/g, '\\${')}\`,
                            focusInfo: ${JSON.stringify(focusInfo || null)}
                        });
                    }
                </script>
            </body>
            </html>`;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (error: any) {
        const { serverUrl } = getCraftConfig();
        
        let errorMessage = 'Failed to generate preview';
        
        if (error.code === 'ECONNREFUSED' || error.code === 'ENOTFOUND') {
            errorMessage = `Cannot connect to Craft server at ${serverUrl}. Please check if the server is running and the URL is correct in settings.`;
        } else if (error.code === 'ECONNABORTED') {
            errorMessage = `Request to Craft server timed out. You can increase the timeout in settings or check server performance.`;
        } else if (error.response) {
            errorMessage = `Server error (${error.response.status}): ${error.response.data?.message || error.message}`;
        } else {
            errorMessage = `${errorMessage}: ${error.message}`;
        }
        
        window.showErrorMessage(errorMessage);
    }
}

export async function handleDownload(message: any) {
    try {
        const { serverUrl, timeout } = getCraftConfig();
        
        const requestBody = {
            DSL: message.dsl,
            focusInfo: message.focusInfo,
            format: message.format,
            diagramType: message.diagramType,
            boundariesMode: message.focusInfo?.boundariesMode
        };

        // Make request to backend
        const response = await axios.post(`${serverUrl}/download`, requestBody, {
            headers: {
                'Content-Type': 'application/json'
            },
            timeout: timeout,
            responseType: 'arraybuffer'
        });

        // Get file extension based on format
        let extension = message.format;
        if (message.format === 'puml') {
            extension = 'puml';
        }

        // Generate filename
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5);
        const filename = `${message.diagramType}-diagram-${timestamp}.${extension}`;

        // Get download location from settings or show dialog
        const downloadPath = workspace.getConfiguration('craft').get<string>('downloadPath');
        
        let saveUri: Uri;
        if (downloadPath && fs.existsSync(downloadPath)) {
            saveUri = Uri.file(path.join(downloadPath, filename));
        } else {
            // Show save dialog
            const result = await window.showSaveDialog({
                defaultUri: Uri.file(filename),
                filters: {
                    'Diagram Files': [extension],
                    'All Files': ['*']
                }
            });
            
            if (!result) {
                return; // User cancelled
            }
            saveUri = result;
        }

        // Save file
        await workspace.fs.writeFile(saveUri, new Uint8Array(response.data));
        
        // Show success message
        const openAction = 'Open File';
        const result = await window.showInformationMessage(
            `Diagram saved to ${saveUri.fsPath}`,
            openAction
        );
        
        if (result === openAction) {
            await window.showTextDocument(saveUri);
        }

    } catch (error: any) {
        let errorMessage = 'Failed to download diagram';
        
        if (error.code === 'ECONNREFUSED' || error.code === 'ENOTFOUND') {
            const { serverUrl } = getCraftConfig();
            errorMessage = `Cannot connect to Craft server at ${serverUrl}. Please check if the server is running.`;
        } else if (error.response) {
            errorMessage = `Server error (${error.response.status}): ${error.response.data?.message || error.message}`;
        } else {
            errorMessage = `${errorMessage}: ${error.message}`;
        }
        
        window.showErrorMessage(errorMessage);
    }
}

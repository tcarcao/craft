// eslint-disable-next-line @typescript-eslint/no-require-imports
const axios = require('axios');
import { window, WebviewPanel, workspace, Uri } from 'vscode';
import { getCraftConfig } from '../utils/config';
import { 
    DomainPreviewRequest, 
    C4PreviewRequest, 
    DomainDownloadRequest, 
    C4DownloadRequest, 
    PreviewResponse, 
    DownloadMessage,
    DomainDownloadMessage,
    C4DownloadMessage,
    DiagramFormat,
    DomainMode,
    FocusInfo,
    DomainPreviewOptions,
    C4PreviewOptions,
    DomainDownloadOptions,
    C4DownloadOptions
} from '../types/api';

// Re-export types for external use
export type { DomainPreviewOptions, C4PreviewOptions, DomainDownloadOptions, C4DownloadOptions };
import * as fs from 'fs';
import * as path from 'path';

export async function previewDomainDiagram(
    previewPanel: WebviewPanel,
    text: string,
    options?: DomainPreviewOptions
): Promise<void> {
    const { serverUrl, timeout } = getCraftConfig();
    
    const requestBody: DomainPreviewRequest = {
        dsl: text,
        domainMode: options?.domainMode
    };
    
    const { data } = await axios.post(`${serverUrl}/preview/domain`, requestBody, {
        headers: {
            'Content-Type': 'application/json'
        },
        timeout: timeout
    });

    const response: PreviewResponse = data;
    if (response.success && response.data) {
        updateWebviewContent(previewPanel, 'Domain', response.data, text, { domainMode: options?.domainMode });
    } else {
        throw new Error(response.error || 'Failed to generate domain diagram');
    }
}

export async function previewC4Diagram(
    previewPanel: WebviewPanel,
    text: string,
    options?: C4PreviewOptions
): Promise<void> {
    const { serverUrl, timeout } = getCraftConfig();
    
    const requestBody: C4PreviewRequest = {
        dsl: text
    };
    
    if (options && (options.hasFocusedServices || options.hasFocusedSubDomains)) {
        requestBody.focusInfo = {
            focusedServiceNames: options.focusedServiceNames,
            focusedSubDomainNames: options.focusedSubDomainNames,
            hasFocusedServices: options.hasFocusedServices,
            hasFocusedSubDomains: options.hasFocusedSubDomains
        };
    }
    
    // Use explicit values from options, with reasonable defaults only if not provided
    requestBody.boundariesMode = options?.boundariesMode || 'boundaries';
    requestBody.showDatabases = options?.showDatabases ?? true;
    
    const { data } = await axios.post(`${serverUrl}/preview/c4`, requestBody, {
        headers: {
            'Content-Type': 'application/json'
        },
        timeout: timeout
    });

    const response: PreviewResponse = data;
    if (response.success && response.data) {
        updateWebviewContent(previewPanel, 'C4', response.data, text, { 
            focusInfo: requestBody.focusInfo, 
            boundariesMode: options?.boundariesMode, 
            showDatabases: options?.showDatabases 
        });
    } else {
        throw new Error(response.error || 'Failed to generate C4 diagram');
    }
}

// Helper function to update webview content
function updateWebviewContent(
    previewPanel: WebviewPanel,
    diagramType: string,
    diagram: string,
    text: string,
    options: { domainMode?: DomainMode; focusInfo?: FocusInfo; boundariesMode?: 'boundaries' | 'transparent'; showDatabases?: boolean }
): void {
    const { domainMode, focusInfo, boundariesMode, showDatabases } = options;

    // Update webview content
    previewPanel.webview.html = `
        <!DOCTYPE html>
        <html>
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <title>${diagramType} Preview</title>
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
                <img src="data:image/png;base64,${diagram}" alt="${diagramType} Diagram">
            </div>
            
            <script>
                const vscode = acquireVsCodeApi();
                
                function downloadDiagram(format) {
                    vscode.postMessage({
                        command: 'download',
                        diagramType: '${diagramType.toLowerCase()}',
                        format: format,
                        dsl: \`${text.replace(/`/g, '\\`').replace(/\${/g, '\\${')}\`,
                        focusInfo: ${JSON.stringify(focusInfo || null)},
                        domainMode: '${domainMode || 'detailed'}',
                        boundariesMode: '${boundariesMode || 'boundaries'}',
                        showDatabases: ${showDatabases ?? true}
                    });
                }
            </script>
        </body>
        </html>`;
}

// Helper function to handle preview errors
export function handlePreviewError(error: unknown): void {
    const { serverUrl } = getCraftConfig();
    
    let errorMessage = 'Failed to generate preview';
    
    if (error && typeof error === 'object' && 'code' in error) {
        const err = error as { code: string; message: string; response?: { status: number; data?: { message: string } } };
        
        if (err.code === 'ECONNREFUSED' || err.code === 'ENOTFOUND') {
            errorMessage = `Cannot connect to Craft server at ${serverUrl}. Please check if the server is running and the URL is correct in settings.`;
        } else if (err.code === 'ECONNABORTED') {
            errorMessage = `Request to Craft server timed out. You can increase the timeout in settings or check server performance.`;
        } else if (err.response) {
            errorMessage = `Server error (${err.response.status}): ${err.response.data?.message || err.message}`;
        } else {
            errorMessage = `${errorMessage}: ${err.message}`;
        }
    } else if (error instanceof Error) {
        errorMessage = `${errorMessage}: ${error.message}`;
    } else {
        errorMessage = `${errorMessage}: Unknown error`;
    }
    
    window.showErrorMessage(errorMessage);
}

// Type-safe download functions
export async function downloadDomainDiagram(
    dsl: string,
    options: DomainDownloadOptions
): Promise<void> {
    const { serverUrl, timeout } = getCraftConfig();
    
    const requestBody: DomainDownloadRequest = {
        dsl,
        domainMode: options.domainMode,
        format: options.format,
        filename: options.filename
    };
    
    const response = await axios.post(`${serverUrl}/download/domain`, requestBody, {
        headers: {
            'Content-Type': 'application/json'
        },
        timeout: timeout,
        responseType: 'arraybuffer'
    });

    await saveDownloadedFile(response.data, options.format, 'domain', options.filename);
}

export async function downloadC4Diagram(
    dsl: string,
    options: C4DownloadOptions
): Promise<void> {
    const { serverUrl, timeout } = getCraftConfig();
    
    const requestBody: C4DownloadRequest = {
        dsl,
        format: options.format,
        filename: options.filename
    };
    
    if (options.hasFocusedServices || options.hasFocusedSubDomains) {
        requestBody.focusInfo = {
            focusedServiceNames: options.focusedServiceNames,
            focusedSubDomainNames: options.focusedSubDomainNames,
            hasFocusedServices: options.hasFocusedServices,
            hasFocusedSubDomains: options.hasFocusedSubDomains
        };
    }
    
    // Use explicit values from options, with reasonable defaults only if not provided
    requestBody.boundariesMode = options.boundariesMode || 'boundaries';
    requestBody.showDatabases = options.showDatabases ?? true;
    
    const response = await axios.post(`${serverUrl}/download/c4`, requestBody, {
        headers: {
            'Content-Type': 'application/json'
        },
        timeout: timeout,
        responseType: 'arraybuffer'
    });

    await saveDownloadedFile(response.data, options.format, 'c4', options.filename);
}

// Helper function to save downloaded file
async function saveDownloadedFile(
    data: ArrayBuffer,
    format: DiagramFormat,
    diagramType: string,
    customFilename?: string
): Promise<void> {
    // Get file extension based on format
    const extension = format === 'puml' ? 'puml' : format;

    // Generate filename
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5);
    const filename = customFilename || `${diagramType}-diagram-${timestamp}.${extension}`;

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
    await workspace.fs.writeFile(saveUri, new Uint8Array(data));
    
    // Show success message
    const openAction = 'Open File';
    const result = await window.showInformationMessage(
        `Diagram saved to ${saveUri.fsPath}`,
        openAction
    );
    
    if (result === openAction) {
        await window.showTextDocument(saveUri);
    }
}

// Type-safe download handlers for each diagram type
export async function handleDomainDownload(message: DomainDownloadMessage): Promise<void> {
    try {
        const options: DomainDownloadOptions = {
            domainMode: message.domainMode || 'detailed',
            format: message.format
        };
        await downloadDomainDiagram(message.dsl, options);
    } catch (error: unknown) {
        handleDownloadError(error);
    }
}

export async function handleC4Download(message: C4DownloadMessage): Promise<void> {
    try {
        const options: C4DownloadOptions = {
            format: message.format,
            focusedServiceNames: message.focusInfo?.focusedServiceNames || [],
            focusedSubDomainNames: message.focusInfo?.focusedSubDomainNames || [],
            hasFocusedServices: message.focusInfo?.hasFocusedServices || false,
            hasFocusedSubDomains: message.focusInfo?.hasFocusedSubDomains || false,
            // Now we have access to these properties!
            boundariesMode: message.boundariesMode,
            showDatabases: message.showDatabases
        };
        await downloadC4Diagram(message.dsl, options);
    } catch (error: unknown) {
        handleDownloadError(error);
    }
}

// Legacy function for backward compatibility with webview messages
export async function handleDownload(message: DownloadMessage): Promise<void> {
    if (message.diagramType === 'domain') {
        await handleDomainDownload(message as DomainDownloadMessage);
    } else if (message.diagramType === 'c4') {
        await handleC4Download(message as C4DownloadMessage);
    } else {
        // This should never happen due to the discriminated union, but TypeScript requires it
        throw new Error(`Unsupported diagram type: ${(message as any).diagramType}`);
    }
}

// Helper function to handle download errors
function handleDownloadError(error: unknown): void {
    let errorMessage = 'Failed to download diagram';
    
    if (error && typeof error === 'object' && 'code' in error) {
        const err = error as { code: string; message: string; response?: { status: number; data?: { message: string } } };
        
        if (err.code === 'ECONNREFUSED' || err.code === 'ENOTFOUND') {
            const { serverUrl } = getCraftConfig();
            errorMessage = `Cannot connect to Craft server at ${serverUrl}. Please check if the server is running.`;
        } else if (err.response) {
            errorMessage = `Server error (${err.response.status}): ${err.response.data?.message || err.message}`;
        } else {
            errorMessage = `${errorMessage}: ${err.message}`;
        }
    } else if (error instanceof Error) {
        errorMessage = `${errorMessage}: ${error.message}`;
    } else {
        errorMessage = `${errorMessage}: Unknown error`;
    }
    
    window.showErrorMessage(errorMessage);
}

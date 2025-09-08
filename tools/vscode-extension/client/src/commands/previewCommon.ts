// eslint-disable-next-line @typescript-eslint/no-require-imports
const axios = require('axios');
import { window, WebviewPanel } from 'vscode';

export async function updatePreview(previewPanel: WebviewPanel | undefined, text: string, documentType: string, focusInfo?: any) {
    if (!previewPanel) {
        console.log('not there');
        return;
    }

    try {
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
        
        const { data } = await axios.post(`http://localhost:8080/preview/${documentType.toLowerCase()}`, requestBody, {
            headers: {
                'Content-Type': 'application/json'
            }
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
                        padding: 0;
                        margin: 0;
                    }
                    .diagram-wrapper {
                        width: 100%;
                        overflow-x: auto;
                        margin-bottom: 2rem;
                    }

                    .diagram-wrapper img {
                        max-width: none;  /* Allow image to maintain its natural size */
                    }
                </style>
            </head>
            <body>
                <div class="diagram-wrapper">
                    <img src="data:image/png;base64,${diagram}" alt="${documentType} Diagram">
                </div>
            </body>
            </html>`;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (error: any) {
        window.showErrorMessage(`Failed to generate preview: ${error.message}`);
    }
}

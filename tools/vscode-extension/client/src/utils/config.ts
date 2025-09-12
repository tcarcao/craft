// client/src/utils/config.ts
import { workspace } from 'vscode';

export interface ArchDSLConfig {
    serverUrl: string;
    timeout: number;
}

export function getArchDSLConfig(): ArchDSLConfig {
    const config = workspace.getConfiguration('archdsl.server');
    return {
        serverUrl: config.get<string>('url', 'http://localhost:8080'),
        timeout: config.get<number>('timeout', 30000)
    };
}

export function validateServerUrl(url: string): boolean {
    try {
        const parsedUrl = new URL(url);
        return parsedUrl.protocol === 'http:' || parsedUrl.protocol === 'https:';
    } catch {
        return false;
    }
}
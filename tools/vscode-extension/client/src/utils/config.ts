// client/src/utils/config.ts
import { workspace } from 'vscode';

export interface CraftConfig {
    serverUrl: string;
    timeout: number;
}

export function getCraftConfig(): CraftConfig {
    const config = workspace.getConfiguration('craft.server');
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
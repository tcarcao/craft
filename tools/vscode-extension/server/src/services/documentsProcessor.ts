import {
    // createConnection,
    TextDocuments,
    InitializeParams,
    WorkspaceFolder,
} from 'vscode-languageserver/node.js';
import { TextDocument } from 'vscode-languageserver-textdocument';
import * as fs from 'fs';
import * as path from 'path';
import { URI } from 'vscode-uri';
import { glob } from 'glob';
import { Logger } from '../utils/Logger.js';

interface DocumentInfo {
    uri: string;
    content: string;
    isOpen: boolean;
    languageId?: string;
    version?: number;
    filePath: string;
}

interface ProcessedDocument<T = unknown> {
    uri: string;
    filePath: string;
    isOpen: boolean;
    languageId?: string;
    version?: number;
    result: T;
    error?: string;
}

type DocumentProcessor<T = unknown> = (
    content: string,
    info: Omit<DocumentInfo, 'content'>
) => T | Promise<T>;

interface ParseAllDocumentsOptions {
    /** File patterns to include (e.g., ['**\/*.ts', '**\/*.js']) */
    include?: string[];
    /** File patterns to exclude (e.g., ['**\/node_modules/**', '**\/.git/**']) */
    exclude?: string[];
    /** Maximum file size in bytes to read (default: 1MB) */
    maxFileSize?: number;
    /** Whether to include binary files (default: false) */
    includeBinaryFiles?: boolean;
    /** Number of files to process concurrently (default: 10) */
    concurrency?: number;
    /** Whether to continue processing if one file fails (default: true) */
    continueOnError?: boolean;
}

interface StreamProcessOptions extends ParseAllDocumentsOptions {
    onProgress?: (processed: number, total: number, current: string) => void;
    onResult?: <T>(result: ProcessedDocument<T>) => void;
}

export class WorkspaceParser {
    private documents: TextDocuments<TextDocument>;
    private workspaceFolders: WorkspaceFolder[] = [];

    constructor(documents: TextDocuments<TextDocument>) {
        this.documents = documents;
    }

    public setWorkspaceFolders(folders: WorkspaceFolder[]) {
        this.workspaceFolders = folders;
    }

    /**
     * Process all documents with a custom processor function
     */
    public async processAllDocuments<T>(
        processor: DocumentProcessor<T>,
        options: ParseAllDocumentsOptions = {}
    ): Promise<ProcessedDocument<T>[]> {
        const {
            include = ['**/*'],
            exclude = ['**/node_modules/**', '**/.git/**', '**/.vscode/**', '**/dist/**', '**/build/**'],
            maxFileSize = 1024 * 1024,
            includeBinaryFiles = false,
            concurrency = 10,
            continueOnError = true
        } = options;

        const results: ProcessedDocument<T>[] = [];
        const processedUris = new Set<string>();

        // 1. Process open documents first (fast, in-memory)
        const openDocuments = this.documents.all();
        
        for (const doc of openDocuments) {
            try {
                const docInfo: Omit<DocumentInfo, 'content'> = {
                    uri: doc.uri,
                    isOpen: true,
                    languageId: doc.languageId,
                    version: doc.version,
                    filePath: URI.parse(doc.uri).fsPath
                };

                const result = await processor(doc.getText(), docInfo);
                
                results.push({
                    ...docInfo,
                    result
                });
                
                processedUris.add(doc.uri);
            } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                Logger.error(`Error processing open document ${doc.uri}:`, errorMsg);
                
                if (continueOnError) {
                    results.push({
                        uri: doc.uri,
                        filePath: URI.parse(doc.uri).fsPath,
                        isOpen: true,
                        languageId: doc.languageId,
                        version: doc.version,
                        result: null as T,
                        error: errorMsg
                    });
                } else {
                    throw error;
                }
            }
        }

        // 2. Get workspace files
        const workspaceFiles = await this.getWorkspaceFiles(include, exclude);
        const filesToProcess = workspaceFiles.filter(filePath => 
            !processedUris.has(URI.file(filePath).toString())
        );

        // 3. Process closed files with concurrency control
        await this.processBatch(filesToProcess, concurrency, async (filePath) => {
            const fileUri = URI.file(filePath).toString();
            
            try {
                // File size check
                const stats = await fs.promises.stat(filePath);
                if (stats.size > maxFileSize) {
                    if (continueOnError) {
                        results.push({
                            uri: fileUri,
                            filePath,
                            isOpen: false,
                            languageId: this.getLanguageId(filePath),
                            result: null as T,
                            error: `File too large (${stats.size} bytes)`
                        });
                        return;
                    } else {
                        throw new Error(`File too large: ${filePath} (${stats.size} bytes)`);
                    }
                }

                // Binary file check
                if (!includeBinaryFiles && await this.isBinaryFile(filePath)) {
                    return; // Skip binary files silently
                }

                // Read and process file
                const content = await fs.promises.readFile(filePath, 'utf8');
                
                const docInfo: Omit<DocumentInfo, 'content'> = {
                    uri: fileUri,
                    isOpen: false,
                    languageId: this.getLanguageId(filePath),
                    filePath
                };

                const result = await processor(content, docInfo);
                
                results.push({
                    ...docInfo,
                    result
                });

            } catch (error) {
                const errorMsg = error instanceof Error ? error.message : String(error);
                Logger.error(`Error processing file ${filePath}:`, errorMsg);
                
                if (continueOnError) {
                    results.push({
                        uri: fileUri,
                        filePath,
                        isOpen: false,
                        languageId: this.getLanguageId(filePath),
                        result: null as T,
                        error: errorMsg
                    });
                } else {
                    throw error;
                }
            }
        });

        return results;
    }

    /**
     * Parse all documents in the workspace (both open and closed) - Legacy method
     */
    public async parseAllDocuments(options: ParseAllDocumentsOptions = {}): Promise<DocumentInfo[]> {
        const results: DocumentInfo[] = [];
        
        const processedDocs = await this.processAllDocuments(
            (content, info) => ({ content, ...info }),
            options
        );

        for (const doc of processedDocs) {
            if (doc.error) {
                continue; // Skip errored documents in legacy mode
            }
            
            results.push({
                uri: doc.uri,
                content: doc.result.content,
                isOpen: doc.isOpen,
                languageId: doc.languageId,
                version: doc.version,
                filePath: doc.filePath
            });
        }

        return results;
    }

    /**
     * Parse documents with filtering and processing
     */
    public async parseDocumentsByLanguage(languageIds: string[]): Promise<DocumentInfo[]> {
        const allDocs = await this.parseAllDocuments();
        return allDocs.filter(doc => 
            doc.languageId && languageIds.includes(doc.languageId)
        );
    }

    /**
     * Process documents by language with custom processor
     */
    public async processDocumentsByLanguage<T>(
        languageIds: string[],
        processor: DocumentProcessor<T>,
        options: ParseAllDocumentsOptions = {}
    ): Promise<ProcessedDocument<T>[]> {
        const allDocs = await this.processAllDocuments(processor, options);
        return allDocs.filter(doc => 
            doc.languageId && languageIds.includes(doc.languageId)
        );
    }

    /**
     * Stream process documents one by one (memory efficient for large workspaces)
     */
    public async streamProcessDocuments<T>(
        processor: DocumentProcessor<T>,
        options: StreamProcessOptions = {}
    ): Promise<void> {
        const { onProgress, onResult, ...parseOptions } = options;
        
        // Process open documents first
        const openDocs = this.documents.all();
        let processed = 0;
        
        // Get total count for progress
        const workspaceFiles = await this.getWorkspaceFiles(
            parseOptions.include || ['**/*'],
            parseOptions.exclude || ['**/node_modules/**', '**/.git/**']
        );
        const total = openDocs.length + workspaceFiles.length;
        
        for (const doc of openDocs) {
            try {
                const docInfo: Omit<DocumentInfo, 'content'> = {
                    uri: doc.uri,
                    isOpen: true,
                    languageId: doc.languageId,
                    version: doc.version,
                    filePath: URI.parse(doc.uri).fsPath
                };

                onProgress?.(processed, total, doc.uri);
                const result = await processor(doc.getText(), docInfo);
                
                const processedDoc: ProcessedDocument<T> = {
                    ...docInfo,
                    result
                };
                
                onResult?.(processedDoc);
                processed++;
            } catch (error) {
                Logger.error(`Error processing ${doc.uri}:`, error);
            }
        }
        
        // Process workspace files
        const processedUris = new Set(openDocs.map(doc => doc.uri));
        
        for (const filePath of workspaceFiles) {
            const fileUri = URI.file(filePath).toString();
            
            if (processedUris.has(fileUri)) {
                processed++;
                continue;
            }
            
            try {
                onProgress?.(processed, total, fileUri);
                
                const content = await fs.promises.readFile(filePath, 'utf8');
                const docInfo: Omit<DocumentInfo, 'content'> = {
                    uri: fileUri,
                    isOpen: false,
                    languageId: this.getLanguageId(filePath),
                    filePath
                };

                const result = await processor(content, docInfo);
                
                const processedDoc: ProcessedDocument<T> = {
                    ...docInfo,
                    result
                };
                
                onResult?.(processedDoc);
            } catch (error) {
                Logger.error(`Error processing ${filePath}:`, error);
            }
            
            processed++;
        }
    }

    /**
     * Get document statistics with optional custom processing
     */
    public async getWorkspaceStats<T = unknown>(
        processor?: DocumentProcessor<T>
    ): Promise<{
        openDocuments: number;
        totalDocuments: number;
        languageBreakdown: Record<string, number>;
        totalLines: number;
        totalCharacters: number;
        processedResults?: ProcessedDocument<T>[];
        errors: number;
    }> {
        const languageBreakdown: Record<string, number> = {};
        let totalLines = 0;
        let totalCharacters = 0;
        let errors = 0;
        let processedResults: ProcessedDocument<T>[] | undefined;

        if (processor) {
            // Use custom processor
            processedResults = await this.processAllDocuments(processor, {
                continueOnError: true
            });
            
            for (const doc of processedResults) {
                if (doc.error) {
                    errors++;
                    continue;
                }
                
                const lang = doc.languageId || 'unknown';
                languageBreakdown[lang] = (languageBreakdown[lang] || 0) + 1;
            }
        } else {
            // Default processing - just get basic stats
            const allDocs = await this.processAllDocuments(
                (content) => ({
                    lines: content.split('\n').length,
                    characters: content.length
                }),
                { continueOnError: true }
            );

            for (const doc of allDocs) {
                if (doc.error) {
                    errors++;
                    continue;
                }
                
                const lang = doc.languageId || 'unknown';
                languageBreakdown[lang] = (languageBreakdown[lang] || 0) + 1;
                totalLines += doc.result.lines;
                totalCharacters += doc.result.characters;
            }
        }

        const totalDocuments = Object.values(languageBreakdown).reduce((a, b) => a + b, 0);
        const openDocuments = this.documents.all().length;

        return {
            openDocuments,
            totalDocuments,
            languageBreakdown,
            totalLines,
            totalCharacters,
            processedResults,
            errors
        };
    }

    /**
     * Process files in batches with concurrency control
     */
    private async processBatch<T>(
        items: T[],
        concurrency: number,
        processor: (item: T) => Promise<void>
    ): Promise<void> {
        for (let i = 0; i < items.length; i += concurrency) {
            const batch = items.slice(i, i + concurrency);
            const batchPromises = batch.map(processor);
            await Promise.all(batchPromises);
        }
    }

    /**
     * Get all files in workspace folders matching patterns
     */
    private async getWorkspaceFiles(include: string[], exclude: string[]): Promise<string[]> {
        const allFiles: string[] = [];

        for (const folder of this.workspaceFolders) {
            const folderPath = URI.parse(folder.uri).fsPath;
            
            try {
                for (const pattern of include) {
                    const files = await glob(pattern, {
                        cwd: folderPath,
                        absolute: true,
                        ignore: exclude,
                        nodir: true, // Only files, not directories
                    });
                    allFiles.push(...files);
                }
            } catch (error) {
                Logger.error(`Error scanning workspace folder ${folderPath}:`, error);
            }
        }

        // Remove duplicates
        return [...new Set(allFiles)];
    }

    /**
     * Simple binary file detection
     */
    private async isBinaryFile(filePath: string): Promise<boolean> {
        try {
            const buffer = await fs.promises.readFile(filePath, { flag: 'r' });
            const sample = buffer.slice(0, 512);
            
            // Check for null bytes (common in binary files)
			for (const elem of sample) {
				if (elem === 0) {
					return true;
				}
			}
            
            return false;
        } catch {
            return true; // Assume binary if can't read
        }
    }

    /**
     * Get language ID based on file extension
     */
    private getLanguageId(filePath: string): string {
        const ext = path.extname(filePath).toLowerCase();
        const languageMap: Record<string, string> = {
            '.ts': 'typescript',
            '.js': 'javascript',
            '.tsx': 'typescriptreact',
            '.jsx': 'javascriptreact',
            '.py': 'python',
            '.java': 'java',
            '.cpp': 'cpp',
            '.c': 'c',
            '.cs': 'csharp',
            '.php': 'php',
            '.rb': 'ruby',
            '.go': 'go',
            '.rs': 'rust',
            '.html': 'html',
            '.css': 'css',
            '.scss': 'scss',
            '.json': 'json',
            '.xml': 'xml',
            '.yaml': 'yaml',
            '.yml': 'yaml',
            '.md': 'markdown',
            '.txt': 'plaintext'
        };
        
        return languageMap[ext] || 'plaintext';
    }
}

// Usage example and setup function
export function setupWorkspaceParser(
    documents: TextDocuments<TextDocument>,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    connection: any
) {
    const parser = new WorkspaceParser(documents);

    connection.onInitialize((params: InitializeParams) => {
        // Set workspace folders
        if (params.workspaceFolders) {
            parser.setWorkspaceFolders(params.workspaceFolders);
        }

        return {
            capabilities: {
                // your server capabilities
            }
        };
    });

    // Custom LSP requests with processor support
    connection.onRequest('custom/processAllDocuments', async (params: {
        processor?: string; // Serialized processor function
        options?: ParseAllDocumentsOptions;
    }) => {
        if (params.processor) {
            // Note: In real implementation, you'd need to safely evaluate the processor
            // This is just a placeholder - serializing functions is complex
            Logger.warn('Custom processor from client not supported in this example');
        }
        return await parser.processAllDocuments(
            (content, info) => ({ length: content.length, ...info }),
            params.options
        );
    });

    connection.onRequest('custom/parseAllDocuments', async (options: ParseAllDocumentsOptions) => {
        return await parser.parseAllDocuments(options);
    });

    connection.onRequest('custom/processTypeScriptFiles', async () => {
        return await parser.processDocumentsByLanguage(
            ['typescript', 'typescriptreact'],
            (content, _info) => {
                // Example: Extract imports from TypeScript files
                const imports = content.match(/^import\s+.*?from\s+['"][^'"]+['"];?$/gm) || [];
                const exports = content.match(/^export\s+.*$/gm) || [];
                
                return {
                    importsCount: imports.length,
                    exportsCount: exports.length,
                    imports,
                    exports,
                    linesOfCode: content.split('\n').length
                };
            }
        );
    });

    connection.onRequest('custom/getWorkspaceStats', async () => {
        return await parser.getWorkspaceStats();
    });

    return parser;
}

// Example usage patterns
export const ExampleUsages = {
    // Basic processing
    basicExample: async (parser: WorkspaceParser) => {
        const results = await parser.processAllDocuments(
            (content, info) => ({
                lineCount: content.split('\n').length,
                charCount: content.length,
                hasExports: content.includes('export'),
                filePath: info.filePath
            }),
            {
                include: ['**/*.ts', '**/*.js'],
                exclude: ['**/node_modules/**'],
                concurrency: 5
            }
        );
        
        Logger.debug(`Processed ${results.length} files`);
        return results;
    },

    // Security scanning
    securityScan: async (parser: WorkspaceParser) => {
        return await parser.processDocumentsByLanguage(
            ['javascript', 'typescript'],
            (content, _info) => {
                const issues = [];
                if (content.includes('eval(')) {issues.push('eval usage');}
                if (content.includes('innerHTML =')) {issues.push('innerHTML assignment');}
                if (content.match(/document\.write\s*\(/)) {issues.push('document.write usage');}
                
                return {
                    securityIssues: issues,
                    riskLevel: issues.length > 2 ? 'high' : issues.length > 0 ? 'medium' : 'low'
                };
            }
        );
    },

    // Memory efficient streaming
    streamingExample: async (parser: WorkspaceParser) => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const results: any[] = [];
        
        await parser.streamProcessDocuments(
            (content, _info) => ({
                complexity: calculateComplexity(content),
                maintainabilityIndex: calculateMaintainability(content)
            }),
            {
                include: ['**/*.ts'],
                onProgress: (processed, total, current) => {
                    Logger.debug(`[${processed}/${total}] Processing: ${path.basename(current)}`);
                },
                onResult: (result) => {
                    results.push(result);
                    // if (result.result.complexity > 20) {
                    //     Logger.warn(`High complexity detected in ${result.uri}`);
                    // }
                }
            }
        );
        
        return results;
    }
};

// Helper functions for examples
function calculateComplexity(content: string): number {
    const cyclomaticKeywords = [
        'if', 'else', 'while', 'for', 'switch', 'case', 'catch', '&&', '||', '?'
    ];
    
    let complexity = 1; // Base complexity
    for (const keyword of cyclomaticKeywords) {
        const matches = content.match(new RegExp(`\\b${keyword}\\b`, 'g'));
        if (matches) {
            complexity += matches.length;
        }
    }
    
    return complexity;
}

function calculateMaintainability(content: string): number {
    const lines = content.split('\n').length;
    const complexity = calculateComplexity(content);
    
    // Simplified maintainability index calculation
    const maintainabilityIndex = Math.max(0, 171 - 5.2 * Math.log(lines) - 0.23 * complexity);
    return Math.round(maintainabilityIndex);
}
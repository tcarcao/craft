// API Types matching the server request/response structures

// Matches the server's FocusInfo struct exactly
export interface FocusInfo {
    focusedServiceNames: string[];
    focusedSubDomainNames: string[];
    hasFocusedServices: boolean;
    hasFocusedSubDomains: boolean;
}

// Client-side composite types that combine focus info with additional options
export interface DomainPreviewOptions {
    domainMode?: 'detailed' | 'architecture';
}

export interface C4PreviewOptions extends FocusInfo {
    boundariesMode?: 'boundaries' | 'transparent';
    showDatabases?: boolean;
}

export interface DomainDownloadOptions extends DomainPreviewOptions {
    format: 'png' | 'svg' | 'pdf' | 'puml';
    filename?: string;
}

export interface C4DownloadOptions extends C4PreviewOptions {
    format: 'png' | 'svg' | 'pdf' | 'puml';
    filename?: string;
}

// Server Request Types (for internal API communication)
export interface DomainPreviewRequest {
    dsl: string;
    domainMode?: 'detailed' | 'architecture';
}

export interface C4PreviewRequest {
    dsl: string;
    focusInfo?: FocusInfo;
    boundariesMode?: 'boundaries' | 'transparent';
    showDatabases?: boolean;
}

// Download Request Types
export interface DomainDownloadRequest {
    dsl: string;
    domainMode?: 'detailed' | 'architecture';
    format: 'png' | 'svg' | 'pdf' | 'puml';
    filename?: string;
}

export interface C4DownloadRequest {
    dsl: string;
    focusInfo?: FocusInfo;
    boundariesMode?: 'boundaries' | 'transparent';
    showDatabases?: boolean;
    format: 'png' | 'svg' | 'pdf' | 'puml';
    filename?: string;
}

// Response Types
export interface PreviewResponse {
    success: boolean;
    error?: string;
    data?: string; // base64 encoded diagram
}


// Separate download message types for each diagram type
export interface DomainDownloadMessage {
    command: 'download';
    diagramType: 'domain';
    format: 'png' | 'svg' | 'pdf' | 'puml';
    dsl: string;
    domainMode?: 'detailed' | 'architecture';
}

export interface C4DownloadMessage {
    command: 'download';
    diagramType: 'c4';
    format: 'png' | 'svg' | 'pdf' | 'puml';
    dsl: string;
    focusInfo?: FocusInfo;
    boundariesMode?: 'boundaries' | 'transparent';
    showDatabases?: boolean;
}

// Union type for backward compatibility
export type DownloadMessage = DomainDownloadMessage | C4DownloadMessage;

export type DiagramType = 'domain' | 'c4';
export type DiagramFormat = 'png' | 'svg' | 'pdf' | 'puml';
export type DomainMode = 'detailed' | 'architecture';
export type BoundariesMode = 'boundaries' | 'transparent';
export interface BlockRange {
    startLine: number;
    endLine: number;
    fileUri: string;
}
export interface UseCaseInfo {
    name: string;
    entryPointSubDomain: string | null;
    allDomains: string[];
    scenarios: string[];
    blockRange: BlockRange;
}
export interface ServiceDefinition {
    name: string;
    domains: string[];
    dataStores?: string[];
    language?: string;
    blockRange: BlockRange;
}
export interface DomainDefinition {
    name: string;
    subDomains: string[];
    blockRange: BlockRange;
}
export interface ActorDefinition {
    name: string;
    type: 'user' | 'system' | 'service';
    blockRange: BlockRange;
}
export interface FileResult {
    domains: string[];
    useCases: UseCaseInfo[];
    serviceDefinitions: ServiceDefinition[];
    domainDefinitions: DomainDefinition[];
    actorDefinitions: ActorDefinition[];
    uri: string;
    fileName: string;
}
export interface ExtractionResult {
    domains: string[];
    useCases: UseCaseInfo[];
    serviceDefinitions: ServiceDefinition[];
    domainDefinitions: DomainDefinition[];
    actorDefinitions: ActorDefinition[];
    fileResults: FileResult[];
    error?: string;
}
export declare const ServerCommands: {
    readonly EXTRACT_DOMAINS_FROM_CURRENT: "craft.extractDomains";
    readonly EXTRACT_DOMAINS_FROM_WORKSPACE: "craft.extractAllDomainsFromWorkspace";
    readonly EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES: "craft.extractDslFromBlockRanges";
};
export declare const WebviewMessages: {
    readonly READY: "ready";
    readonly REFRESH: "refresh";
    readonly SET_VIEW_MODE: "setViewMode";
    readonly SHOW_INFORMATION: "showInformation";
    readonly PREVIEW: "preview";
    readonly SELECTION_COMMAND: "selectionCommand";
    readonly REFRESH_COMMAND: "refreshCommand";
    readonly PREVIEW_COMMAND: "previewCommand";
    readonly TOGGLE_OPTIONS_COMMAND: "toggleOptionsCommand";
};
export declare const ProviderMessages: {
    readonly INITIAL_DATA: "initialData";
    readonly DATA_REFRESH: "dataRefresh";
    readonly SELECTION_COMMAND: "selectionCommand";
    readonly REFRESH_COMMAND: "refreshCommand";
    readonly PREVIEW_COMMAND: "previewCommand";
    readonly TOGGLE_OPTIONS_COMMAND: "toggleOptionsCommand";
};
export declare const SelectionActions: {
    readonly SELECT_ALL: "selectAll";
    readonly SELECT_NONE: "selectNone";
    readonly SELECT_CURRENT_FILE: "selectCurrentFile";
};
export interface WebviewMessage {
    type: string;
    [key: string]: any;
}
export interface ShowInformationMessage extends WebviewMessage {
    type: typeof WebviewMessages.SHOW_INFORMATION;
    message: string;
}
export interface SelectionCommandMessage extends WebviewMessage {
    type: typeof WebviewMessages.SELECTION_COMMAND;
    action: typeof SelectionActions.SELECT_ALL | typeof SelectionActions.SELECT_NONE | typeof SelectionActions.SELECT_CURRENT_FILE;
}
export interface SetViewModeMessage extends WebviewMessage {
    type: typeof WebviewMessages.SET_VIEW_MODE;
    viewMode: 'current' | 'workspace';
}

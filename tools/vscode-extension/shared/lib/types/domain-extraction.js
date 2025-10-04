"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.SelectionActions = exports.ProviderMessages = exports.WebviewMessages = exports.ServerCommands = void 0;
// Command request/response types
exports.ServerCommands = {
    EXTRACT_DOMAINS_FROM_CURRENT: 'craft.extractDomains',
    EXTRACT_DOMAINS_FROM_WORKSPACE: 'craft.extractAllDomainsFromWorkspace',
    EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES: 'craft.extractDslFromBlockRanges'
};
// Webview message types - from webview to provider
exports.WebviewMessages = {
    // Common messages
    READY: 'ready',
    REFRESH: 'refresh',
    SET_VIEW_MODE: 'setViewMode',
    SHOW_INFORMATION: 'showInformation',
    // Domain view messages
    PREVIEW: 'preview',
    // Selection commands (triggered by toolbar)
    SELECTION_COMMAND: 'selectionCommand',
    REFRESH_COMMAND: 'refreshCommand',
    PREVIEW_COMMAND: 'previewCommand',
    TOGGLE_OPTIONS_COMMAND: 'toggleOptionsCommand'
};
// Provider message types - from provider to webview
exports.ProviderMessages = {
    INITIAL_DATA: 'initialData',
    DATA_REFRESH: 'dataRefresh',
    SELECTION_COMMAND: 'selectionCommand',
    REFRESH_COMMAND: 'refreshCommand',
    PREVIEW_COMMAND: 'previewCommand',
    TOGGLE_OPTIONS_COMMAND: 'toggleOptionsCommand'
};
// Selection command actions
exports.SelectionActions = {
    SELECT_ALL: 'selectAll',
    SELECT_NONE: 'selectNone',
    SELECT_CURRENT_FILE: 'selectCurrentFile'
};

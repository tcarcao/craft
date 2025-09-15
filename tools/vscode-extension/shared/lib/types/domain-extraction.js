"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.ServerCommands = void 0;
// Command request/response types
exports.ServerCommands = {
    EXTRACT_DOMAINS_FROM_CURRENT: 'craft.extractDomains',
    EXTRACT_DOMAINS_FROM_WORKSPACE: 'craft.extractAllDomainsFromWorkspace',
    EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES: 'craft.extractDslFromBlockRanges'
};

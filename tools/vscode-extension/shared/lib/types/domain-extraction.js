"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.ServerCommands = void 0;
// Command request/response types
exports.ServerCommands = {
    EXTRACT_DOMAINS_FROM_CURRENT: 'archdsl.extractDomains',
    EXTRACT_DOMAINS_FROM_WORKSPACE: 'archdsl.extractAllDomainsFromWorkspace',
    EXTRACT_PARTIAL_DSL_FROM_BLOCK_RANGES: 'archdsl.extractDslFromBlockRanges'
};

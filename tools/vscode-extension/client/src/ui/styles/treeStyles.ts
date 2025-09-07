// src/ui/styles/domainTreeStyles.ts

// Style constants for reusability
const COLORS = {
    primary: 'var(--vscode-button-background)',
    primaryHover: 'var(--vscode-button-hoverBackground)',
    secondary: 'var(--vscode-button-secondaryBackground)',
    secondaryHover: 'var(--vscode-button-secondaryHoverBackground)',
    foreground: 'var(--vscode-foreground)',
    background: 'var(--vscode-sideBar-background)',
    border: 'var(--vscode-panel-border)',
    description: 'var(--vscode-descriptionForeground)',
    hover: 'var(--vscode-list-hoverBackground)',
    green: 'var(--vscode-charts-green)',
    orange: 'var(--vscode-charts-orange)',
    blue: 'var(--vscode-charts-blue)',
    badge: 'var(--vscode-badge-background)',
    badgeFg: 'var(--vscode-badge-foreground)',
    input: 'var(--vscode-input-background)',
    inputBorder: 'var(--vscode-input-border)',
    tooltip: 'var(--vscode-editorHoverWidget-background)',
    tooltipBorder: 'var(--vscode-editorHoverWidget-border)',
    toolbar: 'var(--vscode-toolbar-hoverBackground)',
    scrollBg: 'var(--vscode-scrollbarSlider-background)',
    scrollHover: 'var(--vscode-scrollbarSlider-hoverBackground)',
    error: 'var(--vscode-errorForeground)',
    warning: 'var(--vscode-inputValidation-warningBackground)',
    warningFg: 'var(--vscode-inputValidation-warningForeground)',
    info: 'var(--vscode-inputValidation-infoBackground)'
} as const;

const DIMENSIONS = {
    indentBase: '20px',
    indentUseCase: '30px',
    iconSize: '16px',
    borderRadius: '3px',
    maxTreeHeight: '500px',
    scrollbarWidth: '6px'
} as const;

export const domainTreeStyles = `
/* Base Styles */
body { 
    padding: 12px; 
    font-family: var(--vscode-font-family);
    font-size: var(--vscode-font-size);
    color: ${COLORS.foreground};
    margin: 0;
    background: ${COLORS.background};
}

/* Loading Animation */
@keyframes spin {
    to { transform: rotate(360deg); }
}

@keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.7; }
}

@keyframes fadeIn {
    from { opacity: 0; transform: translateY(-5px); }
    to { opacity: 1; transform: translateY(0); }
}

/* Loading Container */
.loading-container {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 200px;
    flex-direction: column;
}

.spinner {
    display: inline-block;
    width: ${DIMENSIONS.iconSize};
    height: ${DIMENSIONS.iconSize};
    border: 2px solid ${COLORS.description};
    border-radius: 50%;
    border-top-color: var(--vscode-progressBar-background);
    animation: spin 1s ease-in-out infinite;
    margin-bottom: 10px;
}

.loading-text {
    color: ${COLORS.description};
}

/* Header Section */
.header {
    margin-bottom: 16px;
    padding-bottom: 12px;
    border-bottom: 1px solid ${COLORS.border};
}

.header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
}

.title {
    font-size: 14px;
    font-weight: 600;
    margin: 0;
    margin-right: auto;
}

.header-btn {
    background: ${COLORS.primary};
    color: var(--vscode-button-foreground);
    border: none;
    padding: 4px 8px;
    cursor: pointer;
    border-radius: 2px;
    font-size: 11px;
    transition: background-color 0.1s;
    margin-left: 4px;
    width: 28px;
    height: 28px;
}

.header-btn:hover {
    background: ${COLORS.primaryHover};
}

/* View Mode Toggle */
.view-mode-toggle {
    display: flex;
    background: ${COLORS.input};
    border-radius: ${DIMENSIONS.borderRadius};
    overflow: hidden;
    border: 1px solid ${COLORS.inputBorder};
    margin-bottom: 12px;
}

.mode-btn {
    background: none;
    border: none;
    padding: 6px 12px;
    font-size: 11px;
    cursor: pointer;
    color: ${COLORS.foreground};
    transition: background-color 0.1s;
    flex: 1;
}

.mode-btn.active {
    background: ${COLORS.primary};
    color: var(--vscode-button-foreground);
}

.mode-btn:hover:not(.active) {
    background: ${COLORS.hover};
}

/* Selection Info */
.selection-info {
    font-size: 11px;
    color: ${COLORS.description};
}

.selection-summary {
    display: flex;
    align-items: center;
    gap: 6px;
}

.count-item {
    display: flex;
    align-items: center;
    gap: 2px;
}

.count-number {
    font-weight: 600;
    color: ${COLORS.foreground};
}

.count-separator {
    color: ${COLORS.description};
    opacity: 0.6;
}

/* Tree Container */
.tree-container {
    max-height: ${DIMENSIONS.maxTreeHeight};
    overflow-y: auto;
    margin-bottom: 12px;
}

.no-domains {
    text-align: center;
    color: ${COLORS.description};
    padding: 40px 20px;
    font-style: italic;
}

/* Tree Node Base */
.tree-node {
    position: relative;
    margin-bottom: 2px;
}

.tree-node.unavailable {
    opacity: 0.5;
}

.node-content {
    display: flex;
    align-items: center;
    padding: 6px 8px 6px 0px;
    cursor: pointer;
    border-radius: ${DIMENSIONS.borderRadius};
    transition: background-color 0.1s ease;
    user-select: none;
}

.node-content:hover {
    background-color: ${COLORS.hover};
}

/* Domain Nodes */
.domain-node .node-content {
    font-weight: 500;
}

/* Subdomain Nodes */
.subdomain-node {
    margin-left: ${DIMENSIONS.indentBase};
    border-left: 1px solid ${COLORS.border};
    padding-left: 8px;
}

/* Use Case Nodes */
.usecase-node {
    margin-left: ${DIMENSIONS.indentUseCase};
    border-left: 1px solid ${COLORS.border};
}

.usecase-node .node-content {
    margin-left: 8px;
}

/* Empty Subdomain Styles */
.empty-subdomain-node {
    opacity: 0.6;
}

.empty-subdomain-node .node-content {
    cursor: help;
}

.empty-subdomain-node .node-content:hover {
    background-color: ${COLORS.info};
}

.empty-subdomain {
    padding: 8px 12px;
    font-size: 10px;
    color: ${COLORS.description};
    font-style: italic;
    text-align: center;
    background: ${COLORS.info};
    border-radius: 2px;
    margin: 4px 0;
}

/* Expander Controls */
.expander {
    width: ${DIMENSIONS.iconSize};
    height: ${DIMENSIONS.iconSize};
    display: flex;
    align-items: center;
    justify-content: center;
    margin-right: 4px;
    font-size: 10px;
    color: ${COLORS.description};
    cursor: pointer;
    border-radius: 2px;
    transition: background-color 0.1s;
}

.expander:hover {
    background-color: ${COLORS.toolbar};
}

.expander-placeholder {
    width: ${DIMENSIONS.iconSize};
    margin-right: 4px;
}

/* Checkbox Styles */
.checkbox-container {
    margin-right: 8px;
}

.custom-checkbox {
    width: ${DIMENSIONS.iconSize};
    height: ${DIMENSIONS.iconSize};
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
}

.custom-checkbox .checkbox-symbol {
    font-size: 12px;
    font-weight: bold;
    transition: color 0.1s;
}

.custom-checkbox.checked .checkbox-symbol {
    color: ${COLORS.green};
}

.custom-checkbox.indeterminate .checkbox-symbol {
    color: ${COLORS.orange};
}

.custom-checkbox:not(.checked):not(.indeterminate) .checkbox-symbol {
    color: ${COLORS.description};
}

.custom-checkbox.disabled {
    cursor: not-allowed;
}

.custom-checkbox.disabled .checkbox-symbol {
    color: ${COLORS.error};
}

.custom-checkbox:hover .checkbox-symbol {
    opacity: 0.8;
}

/* Node Info */
.node-info {
    flex: 1;
    min-width: 0;
}

.node-header {
    display: flex;
    align-items: center;
    gap: 8px;
}

.node-name {
    word-break: break-word;
    flex: 1;
    align-items: center;
    min-width: 0;
}

.use-case-badge {
    background: ${COLORS.badge};
    color: ${COLORS.badgeFg};
    border-radius: 8px;
    padding: 1px 6px;
    font-size: 10px;
    font-weight: 500;
    white-space: nowrap;
    transition: transform 0.1s;
}

.use-case-badge:hover {
    transform: scale(1.05);
}

.use-case-badge.empty {
    background: ${COLORS.warning};
    color: ${COLORS.warningFg};
}

.node-meta {
    font-size: 10px;
    color: ${COLORS.description};
    margin-top: 2px;
}

.node-description {
    font-size: 10px;
    color: ${COLORS.description};
    margin-top: 2px;
    font-style: italic;
}

.node-children {
    margin-top: 2px;
    transition: all 0.2s ease-in-out;
}

/* Cross-Reference Styles */
.section-header {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 8px 0 4px 0;
    padding: 4px 8px;
    background: var(--vscode-editor-background);
    border-radius: 2px;
    font-size: 10px;
    font-weight: 600;
    color: ${COLORS.foreground};
}

.section-icon {
    font-size: 12px;
}

.section-title {
    flex: 1;
}

.toggle-refs-btn {
    background: ${COLORS.secondary};
    color: var(--vscode-button-secondaryForeground);
    border: none;
    padding: 2px 6px;
    border-radius: 2px;
    font-size: 9px;
    cursor: pointer;
    transition: all 0.1s;
}

.toggle-refs-btn:hover {
    background: ${COLORS.secondaryHover};
    transform: scale(1.05);
}

.references-list {
    border-left: 2px dotted ${COLORS.description};
    padding-left: 8px;
}

.reference-item {
    margin-bottom: 4px;
    padding: 4px 6px;
    border-radius: 2px;
    cursor: pointer;
    transition: all 0.1s ease;
    border: 1px solid transparent;
}

.reference-item:hover {
    background: ${COLORS.hover};
    border-color: ${COLORS.border};
    transform: translateX(2px);
}

.reference-item.entry-point {
    border-left: 3px solid ${COLORS.green};
}

.reference-item.involved {
    border-left: 3px solid ${COLORS.orange};
}

.ref-content {
    display: flex;
    align-items: center;
    gap: 6px;
}

.ref-role-icon {
    font-size: 10px;
    width: 12px;
    text-align: center;
}

.ref-info {
    flex: 1;
    min-width: 0;
}

.ref-usecase {
    font-size: 10px;
    font-weight: 500;
    margin-bottom: 1px;
    word-break: break-word;
}

.ref-domain {
    font-size: 9px;
    color: ${COLORS.description};
    opacity: 0.8;
}

.ref-role-badge {
    background: ${COLORS.secondary};
    color: var(--vscode-button-secondaryForeground);
    padding: 1px 4px;
    border-radius: 2px;
    font-size: 8px;
    font-weight: 500;
    text-transform: uppercase;
}

.ref-role-badge:hover {
    background: ${COLORS.secondaryHover};
}

.ref-indicator {
    display: inline-block;
    background: ${COLORS.info};
    border-radius: 6px;
    padding: 1px 4px;
    font-size: 8px;
    font-weight: 600;
    margin-left: 4px;
}

.entry-point-usecases {
    margin-bottom: 8px;
}

.entry-point-usecases .section-header {
    margin-left: ${DIMENSIONS.indentUseCase};
}

.cross-references {
    margin-left: ${DIMENSIONS.indentUseCase};
    
}

/* Tooltips */
.node-tooltip {
    display: none;
    position: absolute;
    top: 100%;
    left: 20px;
    background: ${COLORS.tooltip};
    border: 1px solid ${COLORS.tooltipBorder};
    border-radius: ${DIMENSIONS.borderRadius};
    padding: 8px;
    font-size: 11px;
    z-index: 1000;
    max-width: 250px;
    word-wrap: break-word;
    box-shadow: 0 2px 8px rgba(0,0,0,0.15);
    animation: fadeIn 0.2s ease;
}

.domain-node:hover .node-tooltip {
    display: block;
}

/* Quick Actions */
.quick-actions {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 16px;
    padding-top: 12px;
    border-top: 1px solid ${COLORS.border};
}

.action-group {
    display: flex;
    gap: 6px;
    align-items: center;
    flex-wrap: wrap;
}

.action-group label {
    font-size: 10px;
    color: ${COLORS.description};
    font-weight: 600;
    min-width: 60px;
    text-transform: uppercase;
}

.action-btn {
    background: ${COLORS.secondary};
    color: var(--vscode-button-secondaryForeground);
    border: none;
    padding: 6px 10px;
    cursor: pointer;
    border-radius: 2px;
    font-size: 11px;
    flex: 1;
    min-width: 0;
    transition: all 0.1s;
}

.action-btn:hover {
    background: ${COLORS.secondaryHover};
    transform: translateY(-1px);
}

.action-btn:active {
    transform: translateY(0);
}

/* Focus Button */
.focus-btn {
    background: transparent;
    border: 1px solid ${COLORS.border};
    color: ${COLORS.foreground};
    cursor: pointer;
    border-radius: 3px;
    font-size: 12px;
    padding: 2px 4px;
    margin-right: 4px;
    transition: all 0.2s ease;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 20px;
    height: 18px;
}

.focus-btn.focused {
    background: ${COLORS.blue};
    border-color: ${COLORS.blue};
    color: white;
}

.focus-btn.unfocused {
    background: transparent;
    border-color: ${COLORS.description};
    color: ${COLORS.description};
}

.focus-btn:hover {
    transform: scale(1.1);
    opacity: 0.8;
}

.node-actions {
    display: flex;
    align-items: center;
    gap: 4px;
}

/* Custom Scrollbar */
.tree-container::-webkit-scrollbar {
    width: ${DIMENSIONS.scrollbarWidth};
}

.tree-container::-webkit-scrollbar-track {
    background: ${COLORS.scrollBg};
}

.tree-container::-webkit-scrollbar-thumb {
    background: ${COLORS.scrollHover};
    border-radius: ${DIMENSIONS.borderRadius};
}

.tree-container::-webkit-scrollbar-thumb:hover {
    background: ${COLORS.description};
}

/* Services View Styles */
.controls-row {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
}

.group-by-toggle,
.view-mode-toggle {
    display: flex;
    background: ${COLORS.input};
    border-radius: ${DIMENSIONS.borderRadius};
    overflow: hidden;
    border: 1px solid ${COLORS.inputBorder};
}

.control-btn {
    background: none;
    border: none;
    padding: 4px 8px;
    font-size: 10px;
    cursor: pointer;
    color: ${COLORS.foreground};
    transition: background-color 0.1s;
}

.control-btn.active {
    background: ${COLORS.primary};
    color: var(--vscode-button-foreground);
}

.control-btn:hover:not(.active) {
    background: ${COLORS.hover};
}

.generate-btn {
    background: ${COLORS.primary};
    color: var(--vscode-button-foreground);
    border: none;
    padding: 4px 12px;
    cursor: pointer;
    border-radius: 2px;
    font-size: 11px;
    font-weight: 500;
    transition: all 0.1s;
}

.generate-btn:hover {
    background: ${COLORS.primaryHover};
    transform: translateY(-1px);
}

.services-container {
    max-height: 400px;
    overflow-y: auto;
    margin-bottom: 12px;
}

.service-group {
    margin-bottom: 8px;
}

.group-header {
    display: flex;
    align-items: center;
    padding: 8px;
    cursor: pointer;
    border-radius: ${DIMENSIONS.borderRadius};
    background: var(--vscode-list-activeSelectionBackground);
    transition: background-color 0.1s ease;
    font-weight: 500;
}

.group-header:hover {
    background: ${COLORS.hover};
}

.service-node {
    margin-bottom: 4px;
}

.service-content {
    display: flex;
    align-items: flex-start;
    padding: 6px 8px;
    cursor: pointer;
    border-radius: ${DIMENSIONS.borderRadius};
    transition: background-color 0.1s ease;
}

.service-content:hover {
    background-color: ${COLORS.hover};
}

.service-info {
    flex: 1;
    min-width: 0;
}

.service-header {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 2px;
}

.service-name {
    font-weight: 500;
    word-break: break-word;
}

.service-meta {
    font-size: 10px;
    color: ${COLORS.description};
    margin-bottom: 2px;
}

.service-description {
    font-size: 10px;
    color: ${COLORS.description};
    margin-bottom: 4px;
    font-style: italic;
}

.service-dependencies {
    font-size: 9px;
    color: ${COLORS.blue};
    margin-bottom: 4px;
}

.service-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 2px;
}

.tag {
    background: ${COLORS.secondary};
    color: var(--vscode-button-secondaryForeground);
    padding: 1px 4px;
    border-radius: 2px;
    font-size: 8px;
    font-weight: 500;
    transition: transform 0.1s;
}

.tag:hover {
    transform: scale(1.05);
}

.type-icon {
    font-size: 14px;
}

.type-icon.small {
    font-size: 12px;
}

.group-children {
    margin-left: 16px;
    border-left: 2px solid ${COLORS.border};
    padding-left: 8px;
    margin-top: 4px;
}

.service-count-badge {
    background: ${COLORS.badge};
    color: ${COLORS.badgeFg};
    border-radius: 8px;
    padding: 1px 6px;
    font-size: 10px;
    font-weight: 500;
}

.selection-count {
    font-weight: 500;
}

/* Responsive Design */
@media (max-width: 300px) {
    .header-row {
        flex-direction: column;
        gap: 8px;
    }
    
    .selection-summary {
        flex-direction: column;
        gap: 4px;
    }
    
    .quick-actions {
        flex-direction: column;
    }
}

/* Accessibility Improvements */
.custom-checkbox:focus,
.expander:focus,
.action-btn:focus,
.mode-btn:focus,
.header-btn:focus {
    outline: 2px solid var(--vscode-focusBorder);
    outline-offset: 2px;
}

/* High Contrast Mode Support */
@media (prefers-contrast: high) {
    .tree-node {
        border: 1px solid transparent;
    }
    
    .tree-node:hover {
        border-color: ${COLORS.foreground};
    }
    
    .custom-checkbox.checked .checkbox-symbol {
        color: var(--vscode-foreground);
    }
}

/* Reduced Motion Support */
@media (prefers-reduced-motion: reduce) {
    * {
        animation-duration: 0.01ms !important;
        animation-iteration-count: 1 !important;
        transition-duration: 0.01ms !important;
    }
}
`;

// Export individual style sections for modular usage
export const crossReferenceStyles = `
.section-header,
.references-list,
.reference-item,
.ref-content,
.ref-info,
.ref-usecase,
.ref-domain,
.ref-role-badge,
.ref-indicator,
.entry-point-usecases,
.cross-references { /* Styles included in main export */ }
`;

export const emptyStateStyles = `
.empty-subdomain,
.empty-subdomain-node { /* Styles included in main export */ }
`;

export const loadingStyles = `
.loading-container,
.spinner { /* Styles included in main export */ }
`;
// src/ui/htmlGenerator.ts

import { Domain, SubDomain, UseCase, UseCaseReference } from '../types/domain';
import { domainTreeStyles } from './styles/treeStyles';

export class DomainsViewHtmlGenerator {

    generateLoadingHtml(): string {
        return `<!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <title>DSL Domain Tree</title>
            <style>${domainTreeStyles}</style>
        </head>
        <body>
            <div class="loading-container">
                <div class="spinner"></div>
                <div class="loading-text">Loading domain tree...</div>
            </div>
        </body>
        </html>`;
    }

    generateTreeHtml(
        domains: Domain[],
        viewMode: 'current' | 'workspace',
        selectedCount: { domains: number, subDomains: number, useCases: number },
        totalCount: { domains: number, subDomains: number, useCases: number }
    ): string {
        // Don't filter here - the provider already filtered for current mode
        // For workspace mode, we'll show all domains but style non-current-file items in grey
        const visibleDomains = domains;

        return `<!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <title>DSL Domain Tree</title>
            <style>${domainTreeStyles}</style>
        </head>
        <body>
            ${this.generateHeader(viewMode, selectedCount, totalCount)}
            ${this.generateTreeContent(visibleDomains, viewMode)}
            ${this.generateQuickActions()}
            ${this.generateScript()}
        </body>
        </html>`;
    }

    private generateHeader(
        viewMode: 'current' | 'workspace',
        selectedCount: { domains: number, subDomains: number, useCases: number },
        _totalCount: { domains: number, subDomains: number, useCases: number }
    ): string {
        return `
        <div class="header">
            <div class="header-row">
                <h3 class="title">Domain Tree</h3>
                ${selectedCount.useCases > 0 ? '<button class="header-btn" onclick="preview()" title="Preview">👀</button>' : ''}
                <button class="header-btn" onclick="refresh()" title="Refresh domains">↻</button>
            </div>
            
            <div class="view-mode-toggle">
                <button class="mode-btn ${viewMode === 'current' ? 'active' : ''}" 
                        onclick="setViewMode('current')"
                        title="Show domains from current file only">Current File</button>
                <button class="mode-btn ${viewMode === 'workspace' ? 'active' : ''}" 
                        onclick="setViewMode('workspace')"
                        title="Show domains from entire workspace">Workspace</button>
            </div>
            
            <div class="selection-info">
                <div class="selection-summary">
                    <span class="count-item">
                        <span class="count-number">${selectedCount.useCases}</span>
                        <span class="count-label">use cases</span>
                    </span>
                    <span class="count-separator">•</span>
                    <span class="count-item">
                        <span class="count-number">${selectedCount.subDomains}</span>
                        <span class="count-label">subdomains</span>
                    </span>
                    <span class="count-separator">•</span>
                    <span class="count-item">
                        <span class="count-number">${selectedCount.domains}</span>
                        <span class="count-label">domains</span>
                    </span>
                </div>
            </div>
        </div>`;
    }

    private generateTreeContent(domains: Domain[], viewMode: 'current' | 'workspace' = 'workspace'): string {
        if (domains.length === 0) {
            return `<div class="tree-container">
                <div class="no-domains">No domains found</div>
            </div>`;
        }

        const treeItems = domains.map(domain => this.generateDomainNode(domain, viewMode)).join('');

        return `<div class="tree-container">${treeItems}</div>`;
    }

    private generateDomainNode(domain: Domain, viewMode: 'current' | 'workspace' = 'workspace'): string {
        const expanderIcon = domain.expanded ? '▼' : '▶';
        const checkboxClass = domain.selected ? 'checked' : (domain.partiallySelected ? 'indeterminate' : '');
        const checkboxSymbol = domain.selected ? '✓' : (domain.partiallySelected ? '▣' : '○');

        let subDomainsHtml = '';
        if (domain.expanded) {
            // Provider already filtered subdomains based on view mode, just render what we received
            subDomainsHtml = domain.subDomains.map(subDomain =>
                this.generateSubDomainNode(domain.id, subDomain, viewMode)
            ).join('');
        }

        // In workspace mode, apply grey styling to non-current-file items
        const greyClass = viewMode === 'workspace' && !domain.inCurrentFile ? 'non-current-file' : '';

        return `
        <div class="tree-node domain-node ${greyClass}" 
             data-id="${domain.id}"
             role="treeitem" 
             aria-expanded="${domain.expanded}">
            <div class="node-content" onclick="toggleDomain('${domain.id}')">
                <span class="expander" 
                      onclick="event.stopPropagation(); toggleExpansion('${domain.id}')"
                      title="${domain.expanded ? 'Collapse' : 'Expand'} domain"
                      role="button"
                      tabindex="0">${expanderIcon}</span>
                <div class="checkbox-container">
                    <div class="custom-checkbox ${checkboxClass}" 
                         title="Select/deselect domain"
                         role="checkbox"
                         aria-checked="${domain.selected ? 'true' : domain.partiallySelected ? 'mixed' : 'false'}">
                        <span class="checkbox-symbol">${checkboxSymbol}</span>
                    </div>
                </div>
                <div class="node-info">
                    <div class="node-header">
                        <span class="node-name">${domain.name}</span>
                        <span class="use-case-badge" 
                              title="${domain.selectedUseCases} of ${domain.totalUseCases} use cases selected">${domain.selectedUseCases}/${domain.totalUseCases}</span>
                    </div>
                    <div class="node-meta">
                        ${domain.subDomains.length} subdomain${domain.subDomains.length !== 1 ? 's' : ''}
                        ${/*domain. .files.length > 1 ? ` • ${domain.files.length} files` : ' • current file'*/''}
                    </div>
                </div>
            </div>
            <div class="node-children" ${!domain.expanded ? 'style="display: none;"' : ''} role="group">
                ${subDomainsHtml}
            </div>
            <div class="node-tooltip">
                <strong>${domain.name}</strong><br/>
                Subdomains: ${domain.subDomains.length}<br/>
                Use Cases: ${domain.totalUseCases}<br/>
                Files: ${/*domain.files.join(', ')*/''}
            </div>
        </div>`;
    }

    private generateSubDomainNode(domainId: string, subDomain: SubDomain, viewMode: 'current' | 'workspace' = 'workspace'): string {
        const expanderIcon = subDomain.expanded ? '▼' : '▶';
        const checkboxClass = subDomain.selected ? 'checked' : (subDomain.partiallySelected ? 'indeterminate' : '');
        const checkboxSymbol = subDomain.selected ? '✓' : (subDomain.partiallySelected ? '▣' : '○');
        const isEmpty = subDomain.useCases.length === 0;
        const hasReferences = subDomain.referencedIn && subDomain.referencedIn.length > 0;
        const isSelectable = !isEmpty || hasReferences; // Can select if has use cases OR references

        let contentHtml = '';
        if (subDomain.expanded) {
            // Entry Point use cases (where this subdomain is the entry point)
            if (!isEmpty) {
                contentHtml += `<div class="entry-point-usecases">
                    ${subDomain.useCases.map(useCase => 
                        this.generateUseCaseNode(domainId, subDomain.id, useCase)
                    ).join('')}
                </div>`;
            }

            // Cross-references (where this subdomain is involved but not entry point)
            if (hasReferences) {
                contentHtml += `<div class="cross-references">
                    <div class="section-header">
                        <span class="section-icon">🔗</span>
                        <span class="section-title">Also Involved In</span>
                        <button class="toggle-refs-btn" onclick="event.stopPropagation(); toggleReferences('${domainId}', '${subDomain.id}')">
                            ${subDomain.showReferences ? 'Hide' : 'Show'} (${subDomain.referencedIn.length})
                        </button>
                    </div>
                    ${subDomain.showReferences ? this.generateCrossReferences(subDomain.referencedIn) : ''}
                </div>`;
            }

            // Empty state - only show if no use cases AND no references
            if (isEmpty && !hasReferences) {
                contentHtml = '<div class="empty-subdomain">No use cases defined</div>';
            }
        }

        const selectedCount = subDomain.useCases.filter(uc => uc.selected).length;
        const clickHandler = isSelectable ? `onclick="toggleSubDomain('${domainId}', '${subDomain.id}')"` : '';
        const refIndicator = hasReferences ? ` <span class="ref-indicator" title="${subDomain.referencedIn.length} cross-references">🔗 ${subDomain.referencedIn.length}</span>` : '';

        // Determine checkbox symbol and class
        const finalCheckboxClass = checkboxClass;
        let finalCheckboxSymbol = '';
        
        if (isEmpty) {
            // finalCheckboxClass = 'disabled';
            finalCheckboxSymbol = '∅';
        } else {
            // finalCheckboxClass = checkboxClass;
            finalCheckboxSymbol = checkboxSymbol;
        }

        // In workspace mode, apply grey styling to non-current-file subdomains
        const subDomainGreyClass = viewMode === 'workspace' && !subDomain.inCurrentFile ? 'non-current-file' : '';

        return `
        <div class="tree-node subdomain-node ${!isSelectable ? 'empty-subdomain-node' : ''} ${subDomainGreyClass}" 
             data-id="${subDomain.id}"
             role="treeitem"
             aria-expanded="${subDomain.expanded}">
            <div class="node-content" ${clickHandler}>
                ${isSelectable ? `<span class="expander" 
                      onclick="event.stopPropagation(); toggleSubDomainExpansion('${domainId}', '${subDomain.id}')"
                      title="${subDomain.expanded ? 'Collapse' : 'Expand'} subdomain"
                      role="button"
                      tabindex="0">${expanderIcon}</span>` : '<span class="expander-placeholder"></span>'}
                <div class="checkbox-container">
                    <div class="custom-checkbox ${finalCheckboxClass}"
                         title="${!isSelectable ? 'No use cases or references to select' : 'Select/deselect subdomain'}"
                         role="checkbox"
                         aria-checked="${!isSelectable ? 'false' : (subDomain.selected ? 'true' : subDomain.partiallySelected ? 'mixed' : 'false')}">
                        <span class="checkbox-symbol">${finalCheckboxSymbol}</span>
                    </div>
                </div>
                <div class="node-info">
                    <div class="node-header">
                        <span class="node-name">${subDomain.name}${refIndicator}</span>
                        <span class="use-case-badge ${!isSelectable ? 'empty' : ''}"
                              title="${!isSelectable ? 'No use cases or references' : `${selectedCount} of ${subDomain.useCases.length} use cases selected`}">
                              ${!isSelectable ? '0' : `${selectedCount}/${subDomain.useCases.length}`}</span>
                    </div>
                </div>
            </div>
            <div class="node-children" ${!subDomain.expanded ? 'style="display: none;"' : ''} role="group">
                ${contentHtml}
            </div>
        </div>`;
    }

    private generateCrossReferences(references: UseCaseReference[]): string {
        return `
        <div class="references-list">
            ${references.map(ref => `
                <div class="reference-item ${ref.role}" onclick="navigateToUseCase('${ref.useCaseId}')">
                    <div class="ref-content">
                        <span class="ref-role-icon">${this.getRoleIcon(ref.role)}</span>
                        <div class="ref-info">
                            <div class="ref-usecase">${ref.useCaseName}</div>
                            <div class="ref-domain">${ref.domainName}</div>
                        </div>
                        <span class="ref-role-badge">${ref.role === 'entry-point' ? 'entry' : 'involved'}</span>
                    </div>
                </div>
            `).join('')}
        </div>`;
    }

    private getRoleIcon(role: 'entry-point' | 'involved'): string {
        const icons = {
            'entry-point': '🎯',
            'involved': '🔗'
        };
        return icons[role];
    }

    private generateUseCaseNode(domainId: string, subDomainId: string, useCase: UseCase): string {
        const checkboxSymbol = useCase.selected ? '✓' : '○';
        const checkboxClass = useCase.selected ? 'checked' : '';

        return `
        <div class="tree-node usecase-node" 
             data-id="${useCase.id}"
             role="treeitem">
            <div class="node-content" onclick="toggleUseCase('${domainId}', '${subDomainId}', '${useCase.id}')">
                <div class="checkbox-container">
                    <div class="custom-checkbox ${checkboxClass}"
                         title="Select/deselect use case"
                         role="checkbox"
                         aria-checked="${useCase.selected}">
                        <span class="checkbox-symbol">${checkboxSymbol}</span>
                    </div>
                </div>
                <div class="node-info">
                    <div class="node-header">
                        <span class="node-name">${useCase.name}</span>
                    </div>
                    ${useCase.description ? `<div class="node-description">${useCase.description}</div>` : ''}
                </div>
            </div>
        </div>`;
    }

    private generateQuickActions(): string {
        return `
        <div class="quick-actions">
            <button class="action-btn" onclick="selectAll()" title="Select all visible domains">
                Select All
            </button>
            <button class="action-btn" onclick="selectNone()" title="Deselect all domains">
                Select None
            </button>
            <button class="action-btn" onclick="selectCurrentFileOnly()" title="Select only domains from current file">
                Current File Only
            </button>
        </div>`;
    }

    private generateScript(): string {
        return `
        <script>
            const vscode = acquireVsCodeApi();
            
            // State management
            let isProcessing = false;
            
            // Helper function to prevent rapid clicks
            function debounce(func, delay = 100) {
                return function(...args) {
                    if (isProcessing) return;
                    isProcessing = true;
                    func.apply(this, args);
                    setTimeout(() => isProcessing = false, delay);
                };
            }
            
            // Domain selection handlers
            const toggleDomain = debounce((domainId) => {
                vscode.postMessage({ type: 'toggleDomain', domainId });
            });
            
            const toggleSubDomain = debounce((domainId, subDomainId) => {
                vscode.postMessage({ type: 'toggleSubDomain', domainId, subDomainId });
            });
            
            const toggleUseCase = debounce((domainId, subDomainId, useCaseId) => {
                vscode.postMessage({ type: 'toggleUseCase', domainId, subDomainId, useCaseId });
            });
            
            // Expansion handlers
            const toggleExpansion = debounce((domainId) => {
                vscode.postMessage({ type: 'toggleExpansion', domainId });
            });
            
            const toggleSubDomainExpansion = debounce((domainId, subDomainId) => {
                vscode.postMessage({ type: 'toggleSubDomainExpansion', domainId, subDomainId });
            });

            function toggleReferences(domainId, subDomainId) {
                vscode.postMessage({ type: 'toggleReferences', domainId, subDomainId });
            }
            
            // View mode handlers
            function setViewMode(mode) {
                vscode.postMessage({ type: 'setViewMode', mode });
            }
            
            // Quick action handlers
            const selectAll = debounce(() => {
                vscode.postMessage({ type: 'selectAll' });
            });
            
            const selectNone = debounce(() => {
                vscode.postMessage({ type: 'selectNone' });
            });
            
            const selectCurrentFileOnly = debounce(() => {
                vscode.postMessage({ type: 'selectCurrentFileOnly' });
            });

            function preview() {
                vscode.postMessage({ type: 'preview' });
            }
            
            function refresh() {
                vscode.postMessage({ type: 'refresh' });
            }
            
            // Additional utility functions
            function expandAll() {
                vscode.postMessage({ type: 'expandAll' });
            }
            
            function collapseAll() {
                vscode.postMessage({ type: 'collapseAll' });
            }
            
            // Keyboard shortcuts
            document.addEventListener('keydown', function(event) {
                // Don't trigger shortcuts if user is typing in an input
                if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA') {
                    return;
                }
                
                if (event.ctrlKey || event.metaKey) {
                    switch (event.key.toLowerCase()) {
                        case 'a':
                            event.preventDefault();
                            selectAll();
                            break;
                        case 'r':
                            event.preventDefault();
                            refresh();
                            break;
                        case 'e':
                            event.preventDefault();
                            expandAll();
                            break;
                        case 'c':
                            if (!event.shiftKey) { // Avoid conflict with copy
                                event.preventDefault();
                                collapseAll();
                            }
                            break;
                    }
                }
                
                // Space or Enter to toggle focused element
                if ((event.key === ' ' || event.key === 'Enter') && event.target.hasAttribute('role')) {
                    event.preventDefault();
                    event.target.click();
                }
            });
            
            // Enhanced accessibility: manage focus
            document.addEventListener('click', function(event) {
                const treeItem = event.target.closest('[role="treeitem"]');
                if (treeItem) {
                    // Remove focus from other items
                    document.querySelectorAll('[role="treeitem"]').forEach(item => {
                        item.removeAttribute('tabindex');
                    });
                    // Set focus to clicked item
                    treeItem.setAttribute('tabindex', '0');
                    treeItem.focus();
                }
            });
            
            // Initialize focus management
            document.addEventListener('DOMContentLoaded', function() {
                const firstTreeItem = document.querySelector('[role="treeitem"]');
                if (firstTreeItem) {
                    firstTreeItem.setAttribute('tabindex', '0');
                }
            });
            
            // Arrow key navigation for tree
            document.addEventListener('keydown', function(event) {
                if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.key)) {
                    return;
                }
                
                const focused = document.activeElement;
                const treeItems = Array.from(document.querySelectorAll('[role="treeitem"]'));
                const currentIndex = treeItems.indexOf(focused);
                
                if (currentIndex === -1) return;
                
                let newIndex = currentIndex;
                
                switch (event.key) {
                    case 'ArrowUp':
                        event.preventDefault();
                        newIndex = Math.max(0, currentIndex - 1);
                        break;
                    case 'ArrowDown':
                        event.preventDefault();
                        newIndex = Math.min(treeItems.length - 1, currentIndex + 1);
                        break;
                    case 'ArrowLeft':
                        event.preventDefault();
                        if (focused.getAttribute('aria-expanded') === 'true') {
                            const expander = focused.querySelector('.expander');
                            if (expander) expander.click();
                        }
                        break;
                    case 'ArrowRight':
                        event.preventDefault();
                        if (focused.getAttribute('aria-expanded') === 'false') {
                            const expander = focused.querySelector('.expander');
                            if (expander) expander.click();
                        }
                        break;
                }
                
                if (newIndex !== currentIndex && treeItems[newIndex]) {
                    focused.removeAttribute('tabindex');
                    treeItems[newIndex].setAttribute('tabindex', '0');
                    treeItems[newIndex].focus();
                }
            });
            
            // Debug helpers (remove in production)
            function logSelectionState() {
                vscode.postMessage({ type: 'debugLogState' });
            }
            
            function showHelp() {
                const helpMessage = \`Domain Tree Help:
                
Keyboard Shortcuts:
• Ctrl/Cmd + A: Select All
• Ctrl/Cmd + R: Refresh
• Ctrl/Cmd + E: Expand All
• Ctrl/Cmd + C: Collapse All
• Arrow Keys: Navigate tree
• Space/Enter: Toggle selection
• Left Arrow: Collapse node
• Right Arrow: Expand node

Mouse Actions:
• Click node name: Toggle selection
• Click arrow (▶/▼): Expand/collapse
• Hover domain: Show detailed tooltip
• Click badge: See selection count

View Modes:
• Current File: Show domains from active file
• Workspace: Show all domains in workspace

Selection States:
• ✓ Green: Fully selected
• ▣ Orange: Partially selected
• ○ Gray: Not selected\`;
                
                console.log(helpMessage);
                
                // Also show in VS Code
                vscode.postMessage({ 
                    type: 'showHelp', 
                    message: helpMessage 
                });
            }
            
            // Performance optimization: virtual scrolling for large trees
            let observer;
            function initVirtualScrolling() {
                if (observer) observer.disconnect();
                
                const treeContainer = document.querySelector('.tree-container');
                if (!treeContainer) return;
                
                observer = new IntersectionObserver((entries) => {
                    entries.forEach(entry => {
                        if (entry.isIntersecting) {
                            entry.target.classList.remove('virtual-hidden');
                        } else {
                            entry.target.classList.add('virtual-hidden');
                        }
                    });
                }, {
                    root: treeContainer,
                    rootMargin: '50px'
                });
                
                document.querySelectorAll('.tree-node').forEach(node => {
                    observer.observe(node);
                });
            }
            
            // Initialize virtual scrolling if tree is large
            if (document.querySelectorAll('.tree-node').length > 100) {
                initVirtualScrolling();
            }
            
            // Expose utility functions globally for console debugging
            window.domainTreeUtils = {
                selectAll,
                selectNone,
                expandAll,
                collapseAll,
                refresh,
                showHelp,
                logSelectionState
            };
            
            console.log('🌳 Domain Tree initialized. Type domainTreeUtils.showHelp() for help.');
        </script>`;
    }
}
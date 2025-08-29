
// src/ui/htmlGenerator.ts

// import { serviceTreeStyles } from './styles/serviceViewStyles';
import { Service, ServiceGroup, SubDomain, UseCase } from '../types/domain';
import { domainTreeStyles } from './styles/treeStyles';

export class ServicesViewHtmlGenerator {

	public generateLoadingHtml(): string {
		return `<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>DSL Services</title>
			<style>${domainTreeStyles}</style>
		</head>
		<body>
			<div class="loading-container">
				<div class="spinner"></div>
				<div class="loading-text">Loading services...</div>
			</div>
		</body>
		</html>`;
	}

	public generateTreeHtml(
		groups: ServiceGroup[],
		viewMode: 'current' | 'workspace',
		selectedCount: { serviceGroups: number, services: number },
		totalCount: { serviceGroups: number, services: number }
	): string {
		const visibleServiceGroups = viewMode === 'current'
			? groups.filter(sg => sg.inCurrentFile)
			: groups;


		return `<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>DSL Services</title>
			<style>${domainTreeStyles}</style>
		</head>
		<body>
			${this.generateHeader(viewMode, selectedCount, totalCount)}
			${this.generateTreeContent(visibleServiceGroups)}
			${this.generateQuickActions()}
			${this.generateScript()}
		</body>
		</html>`;
	}

	private generateHeader(
		viewMode: 'current' | 'workspace',
		selectedCount: { serviceGroups: number, services: number },
		totalCount: { serviceGroups: number, services: number }
	): string {
		return `
			<div class="header">
				<div class="header-row">
					<h3 class="title">Services</h3>
					${selectedCount.services > 0 ? '<button class="header-btn" onclick="preview()" title="Preview">👀</button>' : ''}
					<button class="header-btn" onclick="refresh()" title="Refresh services">↻</button>
				</div>
				
				<div class="view-mode-toggle">
					<button class="mode-btn ${viewMode === 'current' ? 'active' : ''}" 
							onclick="setViewMode('current')"
							title="Show services from current file only">Current File</button>
					<button class="mode-btn ${viewMode === 'workspace' ? 'active' : ''}" 
							onclick="setViewMode('workspace')"
							title="Show services from entire workspace">Workspace</button>
				</div>
				
				<div class="selection-info">
					<span class="selection-count">${selectedCount.services} of ${totalCount.services} services selected</span>
				</div>
			</div>
		`;
	}

	private generateTreeContent(serviceGroups: ServiceGroup[]): string {
		if (serviceGroups.length === 0) {
			return `<div class="tree-container">
					<div class="no-services">No services found</div>
				</div>`;
		}

		const treeItems = serviceGroups.map(group => this.generateServiceGroup(group)).join('');

		return `<div class="tree-container">${treeItems}</div>`;
	}

	private generateServiceGroup(group: ServiceGroup): string {
		const expanderIcon = group.expanded ? '▼' : '▶';
		const checkboxClass = group.selected ? 'checked' : (group.partiallySelected ? 'indeterminate' : '');
		const checkboxSymbol = group.selected ? '✓' : (group.partiallySelected ? '▣' : '○');

		let servicesHtml = '';
		if (group.expanded) {
			servicesHtml = group.services.map(service => this.generateServiceNode(group, service)).join('');
		}

		// const selectedCount = group.services.filter(s => s.selected).length;

		return `
        <div class="tree-node domain-node ${!group.inCurrentFile ? 'unavailable' : ''}" 
             data-id="${group.name}"
             role="treeitem" 
             aria-expanded="${group.expanded}">
            <div class="node-content" onclick="toggleServiceGroup('${group.name}')">
                <span class="expander" 
                      onclick="event.stopPropagation(); toggleGroupExpansion('${group.name}')"
                      title="${group.expanded ? 'Collapse' : 'Expand'} group"
                      role="button"
                      tabindex="0">${expanderIcon}</span>
                <div class="checkbox-container">
                    <div class="custom-checkbox ${checkboxClass}" 
                         title="Select/deselect group"
                         role="checkbox"
                         aria-checked="${group.selected ? 'true' : group.partiallySelected ? 'mixed' : 'false'}">
                        <span class="checkbox-symbol">${checkboxSymbol}</span>
                    </div>
                </div>
                <div class="node-info">
                    <div class="node-header">
                        <span class="node-name">${group.name}</span>
                        <span class="use-case-badge" 
                              title="TODO: ${0} of ${0} use cases selected">${0}/${0}</span>
                    </div>
                    <div class="node-meta">
                        ${group.services.length} service${group.services.length !== 1 ? 's' : ''}
                        ${/*domain. .files.length > 1 ? ` • ${domain.files.length} files` : ' • current file'*/''}
                    </div>
                </div>
            </div>
            <div class="node-children" ${!group.expanded ? 'style="display: none;"' : ''} role="group">
                ${servicesHtml}
            </div>
        </div>`;
	}

	private generateServiceNode(group: ServiceGroup, service: Service): string {
		const expanderIcon = service.expanded ? '▼' : '▶';
		const checkboxClass = service.selected ? 'checked' : (service.partiallySelected ? 'indeterminate' : '');
		const checkboxSymbol = service.selected ? '✓' : (service.partiallySelected ? '▣' : '○');
		const isEmpty = service.subDomains.length === 0;
		const isSelectable = !isEmpty;

		let contentHtml = '';
		if (service.expanded) {
			// Entry Point use cases (where this subdomain is the entry point)
			if (!isEmpty) {
				contentHtml += `<div class="entry-point-usecases">
						${service.subDomains.map(subDomain =>
					this.generateSubDomainNode(group.name, service.id, subDomain)
				).join('')}
					</div>`;
			}

			if (isEmpty) {
				contentHtml = '<div class="empty-subdomain">No sub-domains defined</div>';
			}
		}

		const selectedCount = service.subDomains.filter(sd => sd.selected).length;
		const clickHandler = isSelectable ? `onclick="toggleService('${group.name}', '${service.id}')"` : '';


		const a = `
        <div class="tree-node subdomain-node" 
             data-id="${service.id}"
             role="treeitem">
            <div class="node-content" onclick="toggleService('${group.name}', '${service.id}')">
                <div class="checkbox-container">
                    <div class="custom-checkbox ${checkboxClass}"
                         title="Select/deselect subdomain"
                         role="checkbox"
                         aria-checked="${service.selected ? 'true' : 'false'}">
                        <span class="checkbox-symbol">${checkboxSymbol}</span>
                    </div>
                </div>
                <div class="node-info">
                    <div class="node-header">
                        <span class="node-name">${service.name}</span>
                    </div>
					${service.description ? `<div class="service-description">${service.description}</div>` : ''}
					${service.dependencies.length > 0 ? `<div class="service-dependencies">Depends on: ${service.dependencies.join(', ')}</div>` : ''}
					${service.tags ? `<div class="service-tags">${service.tags.map(tag => `<span class="tag">${tag}</span>`).join('')}</div>` : ''}
                </div>
            </div>
        </div>`;

		return `
			<div class="tree-node subdomain-node ${!isSelectable ? 'empty-subdomain-node' : ''}" 
				 data-id="${service.id}"
				 role="treeitem"
				 aria-expanded="${service.expanded}">
				<div class="node-content" ${clickHandler}>
					${isSelectable ? `<span class="expander" 
						  onclick="event.stopPropagation(); toggleServiceExpansion('${group.name}', '${service.id}')"
						  title="${service.expanded ? 'Collapse' : 'Expand'} service"
						  role="button"
						  tabindex="0">${expanderIcon}</span>` : '<span class="expander-placeholder"></span>'}
					<div class="checkbox-container">
						<div class="custom-checkbox ${checkboxClass}"
							 title="Select/deselect service"
							 role="checkbox"
							 aria-checked="${service.selected ? 'true' : service.partiallySelected ? 'mixed' : 'false'}">
							<span class="checkbox-symbol">${checkboxSymbol}</span>
						</div>
					</div>
					<div class="node-info">
						<div class="node-header">
							<span class="node-name">${service.name}</span>
							<span class="use-case-badge ${!isSelectable ? 'empty' : ''}"
								  title="${!isSelectable ? 'No sub domains' : `${selectedCount} of ${service.subDomains.length} use cases selected`}">
								  ${!isSelectable ? '0' : `${selectedCount}/${service.subDomains.length}`}</span>
						</div>
					</div>
				</div>
				<div class="node-children" ${!service.expanded ? 'style="display: none;"' : ''} role="group">
					${contentHtml}
				</div>
			</div>`;
	}

	private generateSubDomainNode(groupId: string, serviceId: string, subDomain: SubDomain): string {
		const expanderIcon = subDomain.expanded ? '▼' : '▶';
		const checkboxClass = subDomain.selected ? 'checked' : (subDomain.partiallySelected ? 'indeterminate' : '');
		const checkboxSymbol = subDomain.selected ? '✓' : (subDomain.partiallySelected ? '▣' : '○');
		const isEmpty = subDomain.useCases.length === 0;

		let contentHtml = '';
		if (subDomain.expanded) {
			// Entry Point use cases (where this subdomain is the entry point)
			if (!isEmpty) {
				contentHtml += `<div class="entry-point-usecases">
						${subDomain.useCases.map(useCase =>
					this.generateUseCaseNode(groupId, serviceId, subDomain.id, useCase)
				).join('')}
					</div>`;
			}

			if (isEmpty) {
				contentHtml = '<div class="empty-subdomain">No use cases defined</div>';
			}
		}

		const selectedCount = subDomain.useCases.filter(uc => uc.selected).length;

		return `
			<div class="tree-node subdomain-node" 
				 data-id="${subDomain.id}"
				 role="treeitem"
				 aria-expanded="${subDomain.expanded}">
				<div class="node-content" onclick="toggleSubDomain('${groupId}', '${serviceId}', '${subDomain.id}')">
					${!isEmpty ? `<span class="expander" 
						  onclick="event.stopPropagation(); toggleSubDomainExpansion('${groupId}', '${serviceId}', '${subDomain.id}')"
						  title="${subDomain.expanded ? 'Collapse' : 'Expand'} subdomain"
						  role="button"
						  tabindex="0">${expanderIcon}</span>` : '<span class="expander-placeholder"></span>'}
					<div class="checkbox-container">
						<div class="custom-checkbox ${checkboxClass}"
							 title="Select/deselect subdomain"
							 role="checkbox"
							 aria-checked="${subDomain.selected ? 'true' : subDomain.partiallySelected ? 'mixed' : 'false'}">
							<span class="checkbox-symbol">${checkboxSymbol}</span>
						</div>
					</div>
					<div class="node-info">
						<div class="node-header">
							<span class="node-name">${subDomain.name}</span>
							<span class="use-case-badge "
								  title="${isEmpty ? 'No use cases' : `${selectedCount} of ${subDomain.useCases.length} use cases selected`}">
								  ${isEmpty ? `${subDomain.selected ? '1/1' : '0/1' }` : `${selectedCount}/${subDomain.useCases.length}`}</span>
						</div>
					</div>
				</div>
				<div class="node-children" ${!subDomain.expanded ? 'style="display: none;"' : ''} role="group">
					${contentHtml}
				</div>
			</div>`;
	}

	private generateUseCaseNode(groupId: string, serviceId: string, subDomainId: string, useCase: UseCase): string {
		const checkboxSymbol = useCase.selected ? '✓' : '○';
		const checkboxClass = useCase.selected ? 'checked' : '';

		return `
				<div class="tree-node usecase-node" 
					 data-id="${useCase.id}"
					 role="treeitem">
					<div class="node-content" onclick="toggleUseCase('${groupId}', '${serviceId}', '${subDomainId}', '${useCase.id}')">
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
				<button class="action-btn" onclick="selectAll()">Select All</button>
				<button class="action-btn" onclick="selectNone()">Select None</button>
				<button class="action-btn" onclick="selectByType('api')">APIs Only</button>
			</div>
		`;
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
			
			const toggleServiceGroup = debounce((groupId) => {
				vscode.postMessage({ type: 'toggleServiceGroup', groupId });
			});

			const toggleService = debounce((serviceGroupId, serviceId) => {
				vscode.postMessage({ type: 'toggleService', serviceGroupId, serviceId });
			});

			const toggleSubDomain = debounce((serviceGroupId, serviceId, subDomainId) => {
				vscode.postMessage({ type: 'toggleSubDomain', serviceGroupId, serviceId, subDomainId });
			});

			const toggleUseCase = debounce((serviceGroupId, serviceId, subDomainId, useCaseId) => {
				vscode.postMessage({ type: 'toggleUseCase', serviceGroupId, serviceId, subDomainId, useCaseId });
			});
			
			const toggleGroupExpansion = debounce((groupId) => {
				vscode.postMessage({ type: 'toggleGroupExpansion', groupId });
			});

            const toggleServiceExpansion = debounce((groupId, serviceId) => {
                vscode.postMessage({ type: 'toggleServiceExpansion', groupId, serviceId });
            });
            
            const toggleSubDomainExpansion = debounce((groupId, serviceId, subDomainId) => {
                vscode.postMessage({ type: 'toggleSubDomainExpansion', groupId, serviceId, subDomainId });
            });
			
			function setViewMode(mode) {
				vscode.postMessage({ type: 'setViewMode', mode });
			}
			
			function setGroupBy(groupBy) {
				vscode.postMessage({ type: 'setGroupBy', groupBy });
			}
			
			function selectAll() {
				vscode.postMessage({ type: 'selectAll' });
			}
			
			function selectNone() {
				vscode.postMessage({ type: 'selectNone' });
			}
			
			function selectByType(type) {
				vscode.postMessage({ type: 'selectByType', serviceType: type });
			}
			
			function preview() {
				vscode.postMessage({ type: 'preview' });
			}
			
			function refresh() {
				vscode.postMessage({ type: 'refresh' });
			}
			
			// Keyboard shortcuts
			document.addEventListener('keydown', function(event) {
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
						case 'd':
							event.preventDefault();
							generateDiagram();
							break;
						case 'g':
							event.preventDefault();
							// Toggle grouping mode
							const currentGroupBy = document.querySelector('.control-btn.active').textContent.includes('Type') ? 'domain' : 'type';
							setGroupBy(currentGroupBy);
							break;
					}
				}
			});
			
			console.log('🔧 Services view initialized. Ctrl+D to generate diagram.');
		</script>`;
	}
}
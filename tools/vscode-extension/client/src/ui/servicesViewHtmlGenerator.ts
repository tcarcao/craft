
// src/ui/htmlGenerator.ts

// import { serviceTreeStyles } from './styles/serviceViewStyles';
import { Service, ServiceGroup, SubDomain, UseCase } from '../types/domain';

export class ServicesViewHtmlGenerator {

	public generateLoadingHtml(cssUri: string): string {
		return `<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>DSL Services</title>
			<link href="${cssUri}" rel="stylesheet" />
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
		totalCount: { serviceGroups: number, services: number },
		boundariesMode: 'transparent' | 'boundaries' = 'boundaries',
		showDatabases: boolean = true,
		optionsExpanded: boolean = false,
		codiconsUri: string,
		cssUri: string
	): string {
		// Don't filter here - the provider already filtered for current mode
		// For workspace mode, we'll show all groups but style non-current-file items in grey
		const visibleServiceGroups = groups;


		return `<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>DSL Services</title>
			<link href="${codiconsUri}" rel="stylesheet" />
			<link href="${cssUri}" rel="stylesheet" />
		</head>
		<body>
			${this.generateHeader(viewMode, selectedCount, totalCount, boundariesMode, showDatabases, optionsExpanded)}
			${this.generateTreeContent(visibleServiceGroups, viewMode)}
			${this.generateQuickActions()}
			${this.generateScript()}
		</body>
		</html>`;
	}

	private generateHeader(
		viewMode: 'current' | 'workspace',
		selectedCount: { serviceGroups: number, services: number },
		totalCount: { serviceGroups: number, services: number },
		boundariesMode: 'transparent' | 'boundaries' = 'boundaries',
		showDatabases: boolean = true,
		optionsExpanded: boolean = false
	): string {
		return `
			<div class="header">
				<div class="header-row">
					<h3 class="title">Services</h3>
					<div class="header-actions">
						${selectedCount.services > 0 ? '<button class="header-btn" onclick="preview()" title="Preview"><i class="codicon codicon-preview"></i></button>' : ''}
						<button class="header-btn" onclick="refresh()" title="Refresh services"><i class="codicon codicon-refresh"></i></button>
					</div>
				</div>
				
				<div class="view-mode-toggle">
					<button class="mode-btn ${viewMode === 'current' ? 'active' : ''}" 
							onclick="setViewMode('current')"
							title="Show services from current file only">Current File</button>
					<button class="mode-btn ${viewMode === 'workspace' ? 'active' : ''}" 
							onclick="setViewMode('workspace')"
							title="Show services from entire workspace">Workspace</button>
				</div>
				
				<div class="diagram-options">
					<div class="options-header" onclick="toggleDiagramOptions()">
						<span class="options-title">Diagram Options</span>
						<span class="options-expander" id="diagram-options-expander">${optionsExpanded ? '▼' : '▶'}</span>
					</div>
					<div class="options-content" id="diagram-options-content" style="display: ${optionsExpanded ? 'block' : 'none'};">
						<div class="option-group">
							<label class="option-label">Mode:</label>
							<div class="option-toggle">
								<button class="option-btn ${boundariesMode === 'transparent' ? 'active' : ''}" 
										onclick="setBoundariesMode('transparent')"
										title="Show service-to-service connections">Transparent</button>
								<button class="option-btn ${boundariesMode === 'boundaries' ? 'active' : ''}" 
										onclick="setBoundariesMode('boundaries')"
										title="Show domain-to-domain connections">Boundaries</button>
							</div>
						</div>
						
						<div class="option-group">
							<label class="option-label">Database:</label>
							<div class="option-toggle">
								<button class="option-btn ${showDatabases ? 'active' : ''}" 
										onclick="setDatabaseVisibility(true)"
										title="Show databases in diagram">Show</button>
								<button class="option-btn ${!showDatabases ? 'active' : ''}" 
										onclick="setDatabaseVisibility(false)"
										title="Hide databases from diagram">Hide</button>
							</div>
						</div>
						
						<div class="option-group">
							<label class="option-label">Focus Layer:</label>
							<div class="option-toggle">
								<button class="option-btn active" 
										onclick="setFocusLayer('business')"
										title="Focus on business logic and domains">Business</button>
								<button class="option-btn" 
										onclick="setFocusLayer('presentation')"
										title="Focus on presentation and UI components">Presentation</button>
								<button class="option-btn" 
										onclick="setFocusLayer('composition')"
										title="Focus on service composition and integration">Composition</button>
							</div>
						</div>
						
						<div class="option-group">
							<label class="option-label">Infrastructure:</label>
							<div class="option-toggle">
								<button class="option-btn active" 
										onclick="setInfrastructureVisibility(true)"
										title="Show infrastructure components">Show</button>
								<button class="option-btn" 
										onclick="setInfrastructureVisibility(false)"
										title="Hide infrastructure components">Hide</button>
							</div>
						</div>
					</div>
				</div>
				
				<div class="selection-info">
					<span class="selection-count">${selectedCount.services} of ${totalCount.services} services selected</span>
				</div>
			</div>
		`;
	}

	private generateTreeContent(serviceGroups: ServiceGroup[], viewMode: 'current' | 'workspace' = 'workspace'): string {
		if (serviceGroups.length === 0) {
			return `<div class="tree-container">
					<div class="no-services">No services found</div>
				</div>`;
		}

		const treeItems = serviceGroups.map(group => this.generateServiceGroup(group, viewMode)).join('');

		return `<div class="tree-container">${treeItems}</div>`;
	}

	private generateServiceGroup(group: ServiceGroup, viewMode: 'current' | 'workspace' = 'workspace'): string {
		const expanderIcon = group.expanded ? '▼' : '▶';
		const checkboxClass = group.selected ? 'checked' : (group.partiallySelected ? 'indeterminate' : '');
		const checkboxSymbol = group.selected ? '✓' : (group.partiallySelected ? '▣' : '○');

		let servicesHtml = '';
		if (group.expanded) {
			servicesHtml = group.services.map(service => this.generateServiceNode(group, service, viewMode)).join('');
		}

		const selectedServices = group.services.filter(s => s.selected).length;
		const totalServices = group.services.length;
		
		// In workspace mode, apply grey styling to non-current-file items
		const greyClass = viewMode === 'workspace' && !group.inCurrentFile ? 'non-current-file' : '';

		return `
        <div class="tree-node domain-node ${greyClass}" 
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
                              title="${selectedServices} of ${totalServices} services selected">${selectedServices}/${totalServices}</span>
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

	private generateServiceNode(group: ServiceGroup, service: Service, viewMode: 'current' | 'workspace' = 'workspace'): string {
		const expanderIcon = service.expanded ? '▼' : '▶';
		const checkboxClass = service.selected ? 'checked' : (service.partiallySelected ? 'indeterminate' : '');
		const checkboxSymbol = service.selected ? '✓' : (service.partiallySelected ? '▣' : '○');
		const isEmpty = service.subDomains.length === 0;
		const isSelectable = !isEmpty;

		let contentHtml = '';
		if (service.expanded) {
			// Provider already filtered subdomains based on view mode, just render what we received
			if (service.subDomains.length > 0) {
				contentHtml += `<div class="entry-point-usecases">
						${service.subDomains.map(subDomain =>
					this.generateSubDomainNode(group.name, service.id, subDomain, viewMode)
				).join('')}
					</div>`;
			}

			if (isEmpty) {
				contentHtml = '<div class="empty-subdomain">No sub-domains defined</div>';
			}
		}

		const selectedCount = service.subDomains.filter(sd => sd.selected).length;
		const clickHandler = isSelectable ? `onclick="toggleService('${group.name}', '${service.id}')"` : '';

		// In workspace mode, apply grey styling to services in non-current-file groups
		const serviceGreyClass = viewMode === 'workspace' && !group.inCurrentFile ? 'non-current-file' : '';

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
			<div class="tree-node subdomain-node ${!isSelectable ? 'empty-subdomain-node' : ''} ${serviceGreyClass}" 
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
							<div class="node-actions">
								<button class="focus-btn ${service.focused ? 'focused' : 'unfocused'}" 
										onclick="event.stopPropagation(); toggleServiceFocus('${group.name}', '${service.id}')"
										title="${service.focused ? 'Remove focus (treat as external)' : 'Add focus (include in diagram)'}">
									${service.focused ? '◉' : '◎'}
								</button>
								<span class="use-case-badge ${!isSelectable ? 'empty' : ''}"
									  title="${!isSelectable ? 'No sub domains' : `${selectedCount} of ${service.subDomains.length} use cases selected`}">
									  ${!isSelectable ? '0' : `${selectedCount}/${service.subDomains.length}`}</span>
							</div>
						</div>
					</div>
				</div>
				<div class="node-children" ${!service.expanded ? 'style="display: none;"' : ''} role="group">
					${contentHtml}
				</div>
			</div>`;
	}

	private generateSubDomainNode(groupId: string, serviceId: string, subDomain: SubDomain, viewMode: 'current' | 'workspace' = 'workspace'): string {
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
		
		// In workspace mode, apply grey styling to non-current-file subdomains
		const subDomainGreyClass = viewMode === 'workspace' && !subDomain.inCurrentFile ? 'non-current-file' : '';

		return `
			<div class="tree-node subdomain-node ${subDomainGreyClass}" 
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
							<div class="node-actions">
								<button class="focus-btn ${subDomain.focused ? 'focused' : 'unfocused'}"
										onclick="event.stopPropagation(); toggleSubDomainFocus('${groupId}', '${serviceId}', '${subDomain.id}')"
										title="${subDomain.focused ? 'Click to unfocus (show as external in C4)' : 'Click to focus (show as internal in C4)'}">
									${subDomain.focused ? '◉' : '◎'}
								</button>
								<span class="use-case-badge "
									  title="${isEmpty ? 'No use cases' : `${selectedCount} of ${subDomain.useCases.length} use cases selected`}">
									  ${isEmpty ? `${subDomain.selected ? '1/1' : '0/1' }` : `${selectedCount}/${subDomain.useCases.length}`}</span>
							</div>
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
				<div class="action-group">
					<button class="action-btn" onclick="selectAll()">Select All</button>
					<button class="action-btn" onclick="selectNone()">Select None</button>
				</div>
				<div class="action-group">
					<button class="action-btn" onclick="focusAll()" title="Focus all services (show as internal)">◉ Focus All</button>
					<button class="action-btn" onclick="focusNone()" title="Unfocus all services (show as external)">◎ Unfocus All</button>
				</div>
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
			
			function setBoundariesMode(mode) {
				vscode.postMessage({ type: 'setBoundariesMode', mode });
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
			
			function focusAll() {
				vscode.postMessage({ type: 'focusAll' });
			}
			
			function focusNone() {
				vscode.postMessage({ type: 'focusNone' });
			}
			
			function toggleServiceFocus(serviceGroupId, serviceId) {
				vscode.postMessage({ type: 'toggleServiceFocus', serviceGroupId, serviceId });
			}

			function toggleSubDomainFocus(serviceGroupId, serviceId, subDomainId) {
				vscode.postMessage({ type: 'toggleSubDomainFocus', serviceGroupId, serviceId, subDomainId });
			}
			
			function preview() {
				vscode.postMessage({ type: 'preview' });
			}
			
			function refresh() {
				vscode.postMessage({ type: 'refresh' });
			}
			
			// Diagram Options Functions
			function toggleDiagramOptions() {
				vscode.postMessage({ type: 'toggleDiagramOptions' });
			}
			
			function setDatabaseVisibility(show) {
				console.log('Set database visibility:', show);
				vscode.postMessage({ type: 'setDatabaseVisibility', show });
				
				// Update button states
				updateOptionButtons('Database', show ? 'Show' : 'Hide');
			}
			
			function setFocusLayer(layer) {
				// TODO: Implement focus layer logic
				console.log('Set focus layer:', layer);
				vscode.postMessage({ type: 'setFocusLayer', layer });
				
				// Update button states
				updateOptionButtons('Focus Layer', layer);
			}
			
			function setInfrastructureVisibility(show) {
				// TODO: Implement infrastructure visibility logic
				console.log('Set infrastructure visibility:', show);
				vscode.postMessage({ type: 'setInfrastructureVisibility', show });
				
				// Update button states
				updateOptionButtons('Infrastructure', show ? 'Show' : 'Hide');
			}
			
			function updateOptionButtons(groupLabel, activeValue) {
				// Find the option group and update button states
				const groups = document.querySelectorAll('.option-group');
				groups.forEach(group => {
					const label = group.querySelector('.option-label');
					if (label && label.textContent.includes(groupLabel)) {
						const buttons = group.querySelectorAll('.option-btn');
						buttons.forEach(btn => {
							const isActive = btn.textContent.toLowerCase() === activeValue.toLowerCase();
							btn.classList.toggle('active', isActive);
						});
					}
				});
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
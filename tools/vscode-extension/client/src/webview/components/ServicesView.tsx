import React, { useState, useEffect } from 'react';
import { ServiceGroup, Service, SubDomain, UseCase } from '../../types/domain';
import { WebviewMessages, ProviderMessages, SelectionActions } from '../../types/messages';

interface ServicesViewProps {
  vscode: any;
}

interface ViewState {
  serviceGroups: ServiceGroup[];
  viewMode: 'current' | 'workspace';
  boundariesMode: 'transparent' | 'boundaries';
  showDatabases: boolean;
  optionsExpanded: boolean;
  focusLayer: 'business' | 'presentation' | 'composition';
  showInfrastructure: boolean;
  isLoading: boolean;
}

export const ServicesView: React.FC<ServicesViewProps> = ({ vscode }) => {
  const [state, setState] = useState<ViewState>({
    serviceGroups: [],
    viewMode: 'current',
    boundariesMode: 'boundaries',
    showDatabases: true,
    optionsExpanded: false,
    focusLayer: 'business',
    showInfrastructure: true,
    isLoading: true
  });

  // Helper function to merge new service data while preserving UI state
  const mergeServiceData = (newServiceGroups: ServiceGroup[], existingServiceGroups: ServiceGroup[]): ServiceGroup[] => {
    return newServiceGroups.map(newGroup => {
      const existingGroup = existingServiceGroups.find(g => g.name === newGroup.name);
      if (!existingGroup) {
        return newGroup; // New group, use as-is
      }

      return {
        ...newGroup,
        selected: existingGroup.selected !== undefined ? existingGroup.selected : newGroup.selected,
        partiallySelected: existingGroup.partiallySelected !== undefined ? existingGroup.partiallySelected : newGroup.partiallySelected,
        expanded: existingGroup.expanded !== undefined ? existingGroup.expanded : newGroup.expanded,
        services: newGroup.services.map(newService => {
          const existingService = existingGroup.services.find(s => s.id === newService.id);
          if (!existingService) {
            return newService; // New service, use as-is
          }

          return {
            ...newService,
            selected: existingService.selected !== undefined ? existingService.selected : newService.selected,
            partiallySelected: existingService.partiallySelected !== undefined ? existingService.partiallySelected : newService.partiallySelected,
            expanded: existingService.expanded !== undefined ? existingService.expanded : newService.expanded,
            subDomains: newService.subDomains.map(newSubDomain => {
              const existingSubDomain = existingService.subDomains.find(sd => sd.id === newSubDomain.id);
              if (!existingSubDomain) {
                return newSubDomain; // New subdomain, use as-is
              }

              return {
                ...newSubDomain,
                selected: existingSubDomain.selected !== undefined ? existingSubDomain.selected : newSubDomain.selected,
                partiallySelected: existingSubDomain.partiallySelected !== undefined ? existingSubDomain.partiallySelected : newSubDomain.partiallySelected,
                expanded: existingSubDomain.expanded !== undefined ? existingSubDomain.expanded : newSubDomain.expanded,
                useCases: newSubDomain.useCases.map(newUseCase => {
                  const existingUseCase = existingSubDomain.useCases.find(uc => uc.id === newUseCase.id);
                  return {
                    ...newUseCase,
                    selected: existingUseCase?.selected !== undefined ? existingUseCase.selected : newUseCase.selected
                  };
                })
              };
            })
          };
        })
      };
    });
  };

  // Listen for initial data from extension
  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      const message = event.data;
      
      switch (message.type) {
        case ProviderMessages.INITIAL_DATA:
          setState(prevState => ({
            ...prevState,
            ...message.data,
            isLoading: false
          }));
          break;
        case ProviderMessages.DATA_REFRESH:
          setState(prevState => {
            const mergedData = mergeServiceData(message.data.serviceGroups, prevState.serviceGroups);
            return {
              ...prevState,
              serviceGroups: mergedData,
              viewMode: message.data.viewMode || prevState.viewMode
            };
          });
          break;
        case ProviderMessages.SELECTION_COMMAND:
          switch (message.action) {
            case SelectionActions.SELECT_ALL:
              selectAll();
              break;
            case SelectionActions.SELECT_NONE:
              selectNone();
              break;
          }
          break;
        case ProviderMessages.REFRESH_COMMAND:
          refresh();
          break;
        case ProviderMessages.PREVIEW_COMMAND:
          // Check if any services are selected before previewing
          // const hasSelectedServices = state.serviceGroups.some(group => 
          //   group.services.some(service => service.selected || service.partiallySelected)
          // );
          // if (!hasSelectedServices) {
          //   vscode.postMessage({
          //     type: WebviewMessages.SHOW_INFORMATION,
          //     message: 'Please select at least one service to preview the services diagram.'
          //   });
          //   return;
          // }
          setState(currentState => {
            previewWithState(currentState);
            return currentState;
          });
          break;
        case ProviderMessages.TOGGLE_OPTIONS_COMMAND:
          toggleDiagramOptions();
          break;
      }
    };

    window.addEventListener('message', handleMessage);
    
    // Request initial data
    vscode.postMessage({ type: WebviewMessages.READY });

    return () => window.removeEventListener('message', handleMessage);
  }, [vscode]);



  // ===== REACT STATE MANAGEMENT (The React Way!) =====

  const toggleServiceGroup = (groupId: string) => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => {
        if (group.name === groupId) {
          const newSelected = !group.selected && !group.partiallySelected;
          return {
            ...group,
            selected: newSelected,
            partiallySelected: false,
            services: group.services.map(service => ({
              ...service,
              selected: newSelected,
              partiallySelected: false,
              subDomains: service.subDomains.map(subDomain => ({
                ...subDomain,
                selected: newSelected,
                partiallySelected: false,
                useCases: subDomain.useCases.map(useCase => ({
                  ...useCase,
                  selected: newSelected
                }))
              }))
            }))
          };
        }
        return group;
      })
    }));
  };

  const toggleService = (groupId: string, serviceId: string) => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => {
        if (group.name === groupId) {
          const updatedServices = group.services.map(service => {
            if (service.id === serviceId) {
              const newSelected = !service.selected && !service.partiallySelected;
              return {
                ...service,
                selected: newSelected,
                partiallySelected: false,
                subDomains: service.subDomains.map(subDomain => ({
                  ...subDomain,
                  selected: newSelected,
                  partiallySelected: false,
                  useCases: subDomain.useCases.map(useCase => ({
                    ...useCase,
                    selected: newSelected
                  }))
                }))
              };
            }
            return service;
          });
          
          // Update group selection state based on services
          const selectedServices = updatedServices.filter(s => s.selected).length;
          const totalServices = updatedServices.length;
          
          return {
            ...group,
            services: updatedServices,
            selected: selectedServices === totalServices,
            partiallySelected: selectedServices > 0 && selectedServices < totalServices
          };
        }
        return group;
      })
    }));
  };

  const toggleSubDomain = (groupId: string, serviceId: string, subDomainId: string) => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => {
        if (group.name === groupId) {
          const updatedServices = group.services.map(service => {
            if (service.id === serviceId) {
              const updatedSubDomains = service.subDomains.map(subDomain => {
                if (subDomain.id === subDomainId) {
                  const newSelected = !subDomain.selected && !subDomain.partiallySelected;
                  return {
                    ...subDomain,
                    selected: newSelected,
                    partiallySelected: false,
                    useCases: subDomain.useCases.map(useCase => ({
                      ...useCase,
                      selected: newSelected
                    }))
                  };
                }
                return subDomain;
              });
              
              // Update service selection state based on subdomains
              const selectedSubDomains = updatedSubDomains.filter(sd => sd.selected).length;
              const totalSubDomains = updatedSubDomains.length;
              
              return {
                ...service,
                subDomains: updatedSubDomains,
                selected: selectedSubDomains === totalSubDomains,
                partiallySelected: selectedSubDomains > 0 && selectedSubDomains < totalSubDomains
              };
            }
            return service;
          });
          
          // Update group selection state
          const selectedServices = updatedServices.filter(s => s.selected).length;
          const partialServices = updatedServices.filter(s => s.partiallySelected).length;
          const totalServices = updatedServices.length;
          
          return {
            ...group,
            services: updatedServices,
            selected: selectedServices === totalServices,
            partiallySelected: (selectedServices + partialServices) > 0 && selectedServices < totalServices
          };
        }
        return group;
      })
    }));
  };

  const toggleUseCase = (groupId: string, serviceId: string, subDomainId: string, useCaseId: string) => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => {
        if (group.name === groupId) {
          const updatedServices = group.services.map(service => {
            if (service.id === serviceId) {
              const updatedSubDomains = service.subDomains.map(subDomain => {
                if (subDomain.id === subDomainId) {
                  const updatedUseCases = subDomain.useCases.map(useCase => {
                    if (useCase.id === useCaseId) {
                      return { ...useCase, selected: !useCase.selected };
                    }
                    return useCase;
                  });
                  
                  // Update subdomain selection state based on use cases
                  const selectedUseCases = updatedUseCases.filter(uc => uc.selected).length;
                  const totalUseCases = updatedUseCases.length;
                  
                  return {
                    ...subDomain,
                    useCases: updatedUseCases,
                    selected: selectedUseCases === totalUseCases,
                    partiallySelected: selectedUseCases > 0 && selectedUseCases < totalUseCases
                  };
                }
                return subDomain;
              });
              
              // Update service selection state
              const selectedSubDomains = updatedSubDomains.filter(sd => sd.selected).length;
              const partialSubDomains = updatedSubDomains.filter(sd => sd.partiallySelected).length;
              const totalSubDomains = updatedSubDomains.length;
              
              return {
                ...service,
                subDomains: updatedSubDomains,
                selected: selectedSubDomains === totalSubDomains,
                partiallySelected: (selectedSubDomains + partialSubDomains) > 0 && selectedSubDomains < totalSubDomains
              };
            }
            return service;
          });
          
          // Update group selection state
          const selectedServices = updatedServices.filter(s => s.selected).length;
          const partialServices = updatedServices.filter(s => s.partiallySelected).length;
          const totalServices = updatedServices.length;
          
          return {
            ...group,
            services: updatedServices,
            selected: selectedServices === totalServices,
            partiallySelected: (selectedServices + partialServices) > 0 && selectedServices < totalServices
          };
        }
        return group;
      })
    }));
  };

  const toggleServiceExpansion = (groupId: string, serviceId: string) => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => {
        if (group.name === groupId) {
          return {
            ...group,
            services: group.services.map(service => {
              if (service.id === serviceId) {
                return { ...service, expanded: !service.expanded };
              }
              return service;
            })
          };
        }
        return group;
      })
    }));
  };

  const toggleSubDomainExpansion = (groupId: string, serviceId: string, subDomainId: string) => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => {
        if (group.name === groupId) {
          return {
            ...group,
            services: group.services.map(service => {
              if (service.id === serviceId) {
                return {
                  ...service,
                  subDomains: service.subDomains.map(subDomain => {
                    if (subDomain.id === subDomainId) {
                      return { ...subDomain, expanded: !subDomain.expanded };
                    }
                    return subDomain;
                  })
                };
              }
              return service;
            })
          };
        }
        return group;
      })
    }));
  };

  const toggleGroupExpansion = (groupId: string) => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => {
        if (group.name === groupId) {
          return { ...group, expanded: !group.expanded };
        }
        return group;
      })
    }));
  };

  const setViewMode = (mode: 'current' | 'workspace') => {
    setState(prev => ({ ...prev, viewMode: mode }));
    vscode.postMessage({ type: WebviewMessages.SET_VIEW_MODE, viewMode: mode });
  };

  const setBoundariesMode = (mode: 'transparent' | 'boundaries') => {
    setState(prev => ({ ...prev, boundariesMode: mode }));
  };

  const setDatabaseVisibility = (show: boolean) => {
    setState(prev => ({ ...prev, showDatabases: show }));
  };

  const toggleDiagramOptions = () => {
    setState(prev => ({ ...prev, optionsExpanded: !prev.optionsExpanded }));
  };

  const setFocusLayer = (layer: 'business' | 'presentation' | 'composition') => {
    setState(prev => ({ ...prev, focusLayer: layer }));
  };

  const setInfrastructureVisibility = (show: boolean) => {
    setState(prev => ({ ...prev, showInfrastructure: show }));
  };

  const selectAll = () => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => ({
        ...group,
        selected: true,
        partiallySelected: false,
        services: group.services.map(service => ({
          ...service,
          selected: true,
          partiallySelected: false,
          subDomains: service.subDomains.map(subDomain => ({
            ...subDomain,
            selected: true,
            partiallySelected: false,
            useCases: subDomain.useCases.map(useCase => ({
              ...useCase,
              selected: true
            }))
          }))
        }))
      }))
    }));
  };

  const selectNone = () => {
    setState(prev => ({
      ...prev,
      serviceGroups: prev.serviceGroups.map(group => ({
        ...group,
        selected: false,
        partiallySelected: false,
        services: group.services.map(service => ({
          ...service,
          selected: false,
          partiallySelected: false,
          subDomains: service.subDomains.map(subDomain => ({
            ...subDomain,
            selected: false,
            partiallySelected: false,
            useCases: subDomain.useCases.map(useCase => ({
              ...useCase,
              selected: false
            }))
          }))
        }))
      }))
    }));
  };

  const refresh = () => {
    setState(prev => ({ ...prev, isLoading: true }));
    vscode.postMessage({ type: WebviewMessages.REFRESH });
  };

  const previewWithState = (currentState: ViewState) => {    
    // Extract services and use cases for the preview
    const selectedServices: any[] = [];
    const selectedUseCases: any[] = [];
    const focusInfo = {
      boundariesMode: currentState.boundariesMode,
      showDatabases: currentState.showDatabases
    };
    
    currentState.serviceGroups.forEach(group => {
      group.services.forEach(service => {
        if (service.selected || service.partiallySelected) {
          selectedServices.push(service);
        }
        service.subDomains.forEach(subDomain => {
          subDomain.useCases.forEach(useCase => {
            if (useCase.selected) {
              selectedUseCases.push(useCase);
            }
          });
        });
      });
    });
    
    if (selectedServices.length === 0 && selectedUseCases.length === 0) {
      // Show a helpful message when nothing is selected
      vscode.postMessage({
        type: WebviewMessages.SHOW_INFORMATION,
        message: 'Please select at least one service or use case to preview the services diagram.'
      });
      return;
    }

    
    vscode.postMessage({ 
      type: WebviewMessages.PREVIEW, 
      selectedServices,
      selectedUseCases,
      focusInfo
    });
  };


  // ===== CALCULATED VALUES =====

  const selectedCount = {
    services: state.serviceGroups.reduce((acc, group) => 
      acc + group.services.filter(s => s.selected).length, 0),
    serviceGroups: state.serviceGroups.filter(g => g.selected).length
  };
  
  const totalCount = {
    services: state.serviceGroups.reduce((acc, group) => acc + group.services.length, 0),
    serviceGroups: state.serviceGroups.length
  };


  // ===== RENDER =====

  if (state.isLoading) {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
        <div className="loading-text">Loading services...</div>
      </div>
    );
  }


  return (
    <div className="services-view">
      {/* Header */}
      <div className="header">
        
        <div className="view-mode-toggle">
          <button 
            className={`mode-btn ${state.viewMode === 'current' ? 'active' : ''}`}
            onClick={() => setViewMode('current')}
            title="Show services from current file only"
          >
            Current File
          </button>
          <button 
            className={`mode-btn ${state.viewMode === 'workspace' ? 'active' : ''}`}
            onClick={() => setViewMode('workspace')}
            title="Show services from entire workspace"
          >
            Workspace
          </button>
        </div>
        
        {state.optionsExpanded && (
          <div className="diagram-options">
            <div className="options-content">
              <div className="option-group">
                <label className="option-label">Mode:</label>
                <div className="option-toggle">
                  <button 
                    className={`option-btn ${state.boundariesMode === 'transparent' ? 'active' : ''}`}
                    onClick={() => setBoundariesMode('transparent')}
                    title="Show service-to-service connections"
                  >
                    Transparent
                  </button>
                  <button 
                    className={`option-btn ${state.boundariesMode === 'boundaries' ? 'active' : ''}`}
                    onClick={() => setBoundariesMode('boundaries')}
                    title="Show domain-to-domain connections"
                  >
                    Boundaries
                  </button>
                </div>
              </div>
              
              <div className="option-group">
                <label className="option-label">Database:</label>
                <div className="option-toggle">
                  <button 
                    className={`option-btn ${state.showDatabases ? 'active' : ''}`}
                    onClick={() => setDatabaseVisibility(true)}
                    title="Show databases in diagram"
                  >
                    Show
                  </button>
                  <button 
                    className={`option-btn ${!state.showDatabases ? 'active' : ''}`}
                    onClick={() => setDatabaseVisibility(false)}
                    title="Hide databases from diagram"
                  >
                    Hide
                  </button>
                </div>
              </div>
              
              <div className="option-group">
                <label className="option-label">Focus:</label>
                <div className="option-toggle">
                  <button 
                    className={`option-btn ${state.focusLayer === 'business' ? 'active' : ''}`}
                    onClick={() => setFocusLayer('business')}
                    title="Focus on business logic and domains"
                  >
                    Business
                  </button>
                  <button 
                    className={`option-btn ${state.focusLayer === 'presentation' ? 'active' : ''}`}
                    onClick={() => setFocusLayer('presentation')}
                    title="Focus on presentation and UI components"
                  >
                    Presentation
                  </button>
                  <button 
                    className={`option-btn ${state.focusLayer === 'composition' ? 'active' : ''}`}
                    onClick={() => setFocusLayer('composition')}
                    title="Focus on service composition and integration"
                  >
                    Composition
                  </button>
                </div>
              </div>
              
              <div className="option-group">
                <label className="option-label">Infrastructure:</label>
                <div className="option-toggle">
                  <button 
                    className={`option-btn ${state.showInfrastructure ? 'active' : ''}`}
                    onClick={() => setInfrastructureVisibility(true)}
                    title="Show infrastructure components"
                  >
                    Show
                  </button>
                  <button 
                    className={`option-btn ${!state.showInfrastructure ? 'active' : ''}`}
                    onClick={() => setInfrastructureVisibility(false)}
                    title="Hide infrastructure components"
                  >
                    Hide
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
        
        <div className="selection-info">
          <span className="selection-count">{selectedCount.services} of {totalCount.services} services selected</span>
        </div>
      </div>

      {/* Tree Content */}
      {state.serviceGroups.length === 0 ? (
        <div className="tree-container">
          <div className="no-services">No services found</div>
        </div>
      ) : (
        <div className="tree-container">
          {state.serviceGroups.map(group => (
            <ServiceGroupNode 
              key={group.name} 
              group={group} 
              viewMode={state.viewMode}
              onToggleGroup={() => toggleServiceGroup(group.name)}
              onToggleService={(serviceId: string) => toggleService(group.name, serviceId)}
              onToggleSubDomain={(serviceId: string, subDomainId: string) => 
                toggleSubDomain(group.name, serviceId, subDomainId)}
              onToggleUseCase={(serviceId: string, subDomainId: string, useCaseId: string) => 
                toggleUseCase(group.name, serviceId, subDomainId, useCaseId)}
              onToggleServiceExpansion={(serviceId: string) => 
                toggleServiceExpansion(group.name, serviceId)}
              onToggleSubDomainExpansion={(serviceId: string, subDomainId: string) => 
                toggleSubDomainExpansion(group.name, serviceId, subDomainId)}
              onToggleGroupExpansion={() => toggleGroupExpansion(group.name)}
            />
          ))}
        </div>
      )}

    </div>
  );
};

// ===== TREE NODE COMPONENTS (Now Pure Components!) =====

interface ServiceGroupNodeProps {
  group: ServiceGroup;
  viewMode: 'current' | 'workspace';
  onToggleGroup: () => void;
  onToggleService: (serviceId: string) => void;
  onToggleSubDomain: (serviceId: string, subDomainId: string) => void;
  onToggleUseCase: (serviceId: string, subDomainId: string, useCaseId: string) => void;
  onToggleServiceExpansion: (serviceId: string) => void;
  onToggleSubDomainExpansion: (serviceId: string, subDomainId: string) => void;
  onToggleGroupExpansion: () => void;
}

const ServiceGroupNode: React.FC<ServiceGroupNodeProps> = ({ 
  group, 
  viewMode,
  onToggleGroup,
  onToggleService,
  onToggleSubDomain,
  onToggleUseCase,
  onToggleServiceExpansion,
  onToggleSubDomainExpansion,
  onToggleGroupExpansion
}) => {
  const selectedServices = group.services.filter(s => s.selected).length;
  const totalServices = group.services.length;
  const greyClass = viewMode === 'workspace' && !group.inCurrentFile ? 'non-current-file' : '';

  return (
    <div className={`tree-node service-group-node ${greyClass}`}>
      <div className="node-content" onClick={onToggleGroup}>
        <span 
          className="expander" 
          onClick={(e) => { e.stopPropagation(); onToggleGroupExpansion(); }}
        >
          {group.expanded ? '▼' : '▶'}
        </span>
        
        <div className="checkbox-container">
          <div 
            className={`custom-checkbox ${group.selected ? 'checked' : group.partiallySelected ? 'indeterminate' : ''}`}
            title="Select/deselect group"
            role="checkbox"
            aria-checked={group.selected ? 'true' : group.partiallySelected ? 'mixed' : 'false'}
          >
            <span className="checkbox-symbol">
              {group.selected ? '✓' : group.partiallySelected ? '▣' : '○'}
            </span>
          </div>
        </div>
        
        <div className="node-info">
          <div className="node-header">
            <span className="node-name">{group.name}</span>
            <span className="use-case-badge">
              {selectedServices}/{totalServices}
            </span>
          </div>
          <div className="node-meta">
            {group.services.length} service{group.services.length !== 1 ? 's' : ''}
          </div>
        </div>
      </div>
      
      <div className="node-children" style={{ display: group.expanded ? 'block' : 'none' }}>
        {group.expanded && group.services.map(service => (
          <ServiceNode 
            key={service.id}
            service={service}
            group={group}
            viewMode={viewMode}
            onToggleService={() => onToggleService(service.id)}
            onToggleSubDomain={(subDomainId: string) => onToggleSubDomain(service.id, subDomainId)}
            onToggleUseCase={(subDomainId: string, useCaseId: string) => 
              onToggleUseCase(service.id, subDomainId, useCaseId)}
            onToggleServiceExpansion={() => onToggleServiceExpansion(service.id)}
            onToggleSubDomainExpansion={(subDomainId: string) => 
              onToggleSubDomainExpansion(service.id, subDomainId)}
          />
        ))}
      </div>
    </div>
  );
};

interface ServiceNodeProps {
  service: Service;
  group: ServiceGroup;
  viewMode: 'current' | 'workspace';
  onToggleService: () => void;
  onToggleSubDomain: (subDomainId: string) => void;
  onToggleUseCase: (subDomainId: string, useCaseId: string) => void;
  onToggleServiceExpansion: () => void;
  onToggleSubDomainExpansion: (subDomainId: string) => void;
}

const ServiceNode: React.FC<ServiceNodeProps> = ({ 
  service, 
  group, 
  viewMode,
  onToggleService,
  onToggleSubDomain,
  onToggleUseCase,
  onToggleServiceExpansion,
  onToggleSubDomainExpansion
}) => {
  const isEmpty = service.subDomains.length === 0;
  const selectedCount = service.subDomains.filter(sd => sd.selected).length;
  const serviceGreyClass = viewMode === 'workspace' && !group.inCurrentFile ? 'non-current-file' : '';

  return (
    <div className={`tree-node subdomain-node ${!isEmpty ? '' : 'empty-subdomain-node'} ${serviceGreyClass}`}>
      <div className="node-content" onClick={onToggleService}>
        {!isEmpty ? (
          <span 
            className="expander" 
            onClick={(e) => { e.stopPropagation(); onToggleServiceExpansion(); }}
          >
            {service.expanded ? '▼' : '▶'}
          </span>
        ) : (
          <span className="expander-placeholder"></span>
        )}
        
        <div className="checkbox-container">
          <div 
            className={`custom-checkbox ${service.selected ? 'checked' : service.partiallySelected ? 'indeterminate' : ''}`}
            title="Select/deselect service"
            role="checkbox"
            aria-checked={service.selected ? 'true' : service.partiallySelected ? 'mixed' : 'false'}
          >
            <span className="checkbox-symbol">
              {service.selected ? '✓' : service.partiallySelected ? '▣' : '○'}
            </span>
          </div>
        </div>
        
        <div className="node-info">
          <div className="node-header">
            <span className="node-name">{service.name}</span>
            <div className="node-actions">
              <button 
                className={`focus-btn ${service.focused ? 'focused' : 'unfocused'}`}
                onClick={(e) => { e.stopPropagation(); /* toggle focus logic */ }}
                title={`${service.focused ? 'Remove focus (treat as external)' : 'Add focus (include in diagram)'}`}
              >
                {service.focused ? '◉' : '◎'}
              </button>
              <span 
                className={`use-case-badge ${!isEmpty ? '' : 'empty'}`}
                title={`${!isEmpty ? `${selectedCount} of ${service.subDomains.length} use cases selected` : 'No sub domains'}`}
              >
                {!isEmpty ? `${selectedCount}/${service.subDomains.length}` : '0'}
              </span>
            </div>
          </div>
        </div>
      </div>
      
      <div className="node-children" style={{ display: service.expanded ? 'block' : 'none' }}>
        {service.expanded && (
          <>
            {service.subDomains.length > 0 ? (
              <div className="entry-point-usecases">
                {service.subDomains.map(subDomain => (
                  <SubDomainNode 
                    key={subDomain.id}
                    subDomain={subDomain}
                    viewMode={viewMode}
                    onToggleSubDomain={() => onToggleSubDomain(subDomain.id)}
                    onToggleUseCase={(useCaseId: string) => onToggleUseCase(subDomain.id, useCaseId)}
                    onToggleSubDomainExpansion={() => onToggleSubDomainExpansion(subDomain.id)}
                  />
                ))}
              </div>
            ) : (
              <div className="empty-subdomain">No sub-domains defined</div>
            )}
          </>
        )}
      </div>
    </div>
  );
};

interface SubDomainNodeProps {
  subDomain: SubDomain;
  viewMode: 'current' | 'workspace';
  onToggleSubDomain: () => void;
  onToggleUseCase: (useCaseId: string) => void;
  onToggleSubDomainExpansion: () => void;
}

const SubDomainNode: React.FC<SubDomainNodeProps> = ({ 
  subDomain, 
  viewMode,
  onToggleSubDomain,
  onToggleUseCase,
  onToggleSubDomainExpansion
}) => {
  const isEmpty = subDomain.useCases.length === 0;
  const selectedCount = subDomain.useCases.filter(uc => uc.selected).length;
  const subDomainGreyClass = viewMode === 'workspace' && !subDomain.inCurrentFile ? 'non-current-file' : '';

  return (
    <div className={`tree-node subdomain-node ${subDomainGreyClass}`}>
      <div className="node-content" onClick={onToggleSubDomain}>
        {!isEmpty ? (
          <span 
            className="expander" 
            onClick={(e) => { e.stopPropagation(); onToggleSubDomainExpansion(); }}
          >
            {subDomain.expanded ? '▼' : '▶'}
          </span>
        ) : (
          <span className="expander-placeholder"></span>
        )}
        
        <div className="checkbox-container">
          <div 
            className={`custom-checkbox ${subDomain.selected ? 'checked' : subDomain.partiallySelected ? 'indeterminate' : ''}`}
            title="Select/deselect subdomain"
            role="checkbox"
            aria-checked={subDomain.selected ? 'true' : subDomain.partiallySelected ? 'mixed' : 'false'}
          >
            <span className="checkbox-symbol">
              {subDomain.selected ? '✓' : subDomain.partiallySelected ? '▣' : '○'}
            </span>
          </div>
        </div>
        
        <div className="node-info">
          <div className="node-header">
            <span className="node-name">{subDomain.name}</span>
            <div className="node-actions">
              <button 
                className={`focus-btn ${subDomain.focused ? 'focused' : 'unfocused'}`}
                onClick={(e) => { e.stopPropagation(); /* toggle focus logic */ }}
                title={`${subDomain.focused ? 'Click to unfocus (show as external in C4)' : 'Click to focus (show as internal in C4)'}`}
              >
                {subDomain.focused ? '◉' : '◎'}
              </button>
              <span 
                className="use-case-badge"
                title={`${isEmpty ? 'No use cases' : `${selectedCount} of ${subDomain.useCases.length} use cases selected`}`}
              >
                {isEmpty ? `${subDomain.selected ? '1/1' : '0/1'}` : `${selectedCount}/${subDomain.useCases.length}`}
              </span>
            </div>
          </div>
        </div>
      </div>
      
      <div className="node-children" style={{ display: subDomain.expanded ? 'block' : 'none' }}>
        {subDomain.expanded && (
          <>
            {!isEmpty ? (
              <div className="entry-point-usecases">
                {subDomain.useCases.map(useCase => (
                  <UseCaseNode 
                    key={useCase.id}
                    useCase={useCase}
                    onToggleUseCase={() => onToggleUseCase(useCase.id)}
                  />
                ))}
              </div>
            ) : (
              <div className="empty-subdomain">No use cases defined</div>
            )}
          </>
        )}
      </div>
    </div>
  );
};

interface UseCaseNodeProps {
  useCase: UseCase;
  onToggleUseCase: () => void;
}

const UseCaseNode: React.FC<UseCaseNodeProps> = ({ useCase, onToggleUseCase }) => {
  return (
    <div className="tree-node usecase-node">
      <div className="node-content" onClick={onToggleUseCase}>
        <div className="checkbox-container">
          <div 
            className={`custom-checkbox ${useCase.selected ? 'checked' : ''}`}
            title="Select/deselect use case"
            role="checkbox"
            aria-checked={useCase.selected ? 'true' : 'false'}
          >
            <span className="checkbox-symbol">{useCase.selected ? '✓' : '○'}</span>
          </div>
        </div>
        
        <div className="node-info">
          <div className="node-header">
            <span className="node-name">{useCase.name}</span>
          </div>
          {useCase.description && (
            <div className="node-description">{useCase.description}</div>
          )}
        </div>
      </div>
    </div>
  );
};
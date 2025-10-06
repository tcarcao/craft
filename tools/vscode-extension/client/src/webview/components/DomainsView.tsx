import React, { useState, useEffect } from 'react';
import { Domain, SubDomain, UseCase, UseCaseReference } from '../../types/domain';
import { WebviewMessages, ProviderMessages, SelectionActions } from '../../types/messages';

interface DomainsViewProps {
  vscode: any;
}

interface ViewState {
  domains: Domain[];
  viewMode: 'current' | 'workspace';
  diagramMode: 'detailed' | 'architecture';
  optionsExpanded: boolean;
  isLoading: boolean;
}

export const DomainsView: React.FC<DomainsViewProps> = ({ vscode }) => {
  const [state, setState] = useState<ViewState>({
    domains: [],
    viewMode: 'current',
    diagramMode: 'detailed',
    optionsExpanded: false,
    isLoading: true
  });

  // Helper function to merge new domain data while preserving UI state
  const mergeDomainData = (newDomains: Domain[], existingDomains: Domain[]): Domain[] => {
    return newDomains.map(newDomain => {
      const existingDomain = existingDomains.find(d => d.id === newDomain.id);
      if (!existingDomain) {
        return newDomain; // New domain, use as-is
      }

      return {
        ...newDomain,
        selected: existingDomain.selected !== undefined ? existingDomain.selected : newDomain.selected,
        partiallySelected: existingDomain.partiallySelected !== undefined ? existingDomain.partiallySelected : newDomain.partiallySelected,
        expanded: existingDomain.expanded !== undefined ? existingDomain.expanded : newDomain.expanded,
        subDomains: newDomain.subDomains.map(newSubDomain => {
          const existingSubDomain = existingDomain.subDomains.find(sd => sd.id === newSubDomain.id);
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
    });
  };

  // Listen for data from extension
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
          setState(prevState => ({
            ...prevState,
            domains: mergeDomainData(message.data.domains, prevState.domains),
            viewMode: message.data.viewMode || prevState.viewMode
          }));
          break;
        case ProviderMessages.SELECTION_COMMAND:
          switch (message.action) {
            case SelectionActions.SELECT_ALL:
              selectAll();
              break;
            case SelectionActions.SELECT_NONE:
              selectNone();
              break;
            case SelectionActions.SELECT_CURRENT_FILE:
              selectCurrentFileOnly();
              break;
          }
          break;
        case ProviderMessages.REFRESH_COMMAND:
          handleRefresh();
          break;
        case ProviderMessages.PREVIEW_COMMAND:
          setState(currentState => {
            handlePreviewWithState(currentState);
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


  // ===== REACT STATE MANAGEMENT =====

  const toggleDomain = (domainId: string) => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain => {
        if (domain.id === domainId) {
          const newSelected = !domain.selected && !domain.partiallySelected;
          return {
            ...domain,
            selected: newSelected,
            partiallySelected: false,
            subDomains: domain.subDomains.map(sd => ({
              ...sd,
              selected: newSelected,
              useCases: sd.useCases.map(uc => ({
                ...uc,
                selected: newSelected
              }))
            }))
          };
        }
        return domain;
      })
    }));
  };

  const toggleUseCase = (domainId: string, subDomainId: string, useCaseId: string) => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain => {
        if (domain.id === domainId) {
          const updatedDomain = {
            ...domain,
            subDomains: domain.subDomains.map(subDomain => {
              if (subDomain.id === subDomainId) {
                const updatedSubDomain = {
                  ...subDomain,
                  useCases: subDomain.useCases.map(useCase => 
                    useCase.id === useCaseId 
                      ? { ...useCase, selected: !useCase.selected }
                      : useCase
                  )
                };
                
                // Update subdomain selection state based on use cases
                const selectedUseCases = updatedSubDomain.useCases.filter(uc => uc.selected);
                const totalUseCases = updatedSubDomain.useCases.length;
                
                if (selectedUseCases.length === 0) {
                  updatedSubDomain.selected = false;
                  updatedSubDomain.partiallySelected = false;
                } else if (selectedUseCases.length === totalUseCases) {
                  updatedSubDomain.selected = true;
                  updatedSubDomain.partiallySelected = false;
                } else {
                  updatedSubDomain.selected = false;
                  updatedSubDomain.partiallySelected = true;
                }
                
                return updatedSubDomain;
              }
              return subDomain;
            })
          };
          
          // Update domain selection state based on subdomains
          const selectedSubDomains = updatedDomain.subDomains.filter(sd => sd.selected);
          const partialSubDomains = updatedDomain.subDomains.filter(sd => sd.partiallySelected);
          const totalSubDomains = updatedDomain.subDomains.length;
          
          if (selectedSubDomains.length === 0 && partialSubDomains.length === 0) {
            updatedDomain.selected = false;
            updatedDomain.partiallySelected = false;
          } else if (selectedSubDomains.length === totalSubDomains) {
            updatedDomain.selected = true;
            updatedDomain.partiallySelected = false;
          } else {
            updatedDomain.selected = false;
            updatedDomain.partiallySelected = true;
          }
          
          return updatedDomain;
        }
        return domain;
      })
    }));
  };

  const toggleSubDomain = (domainId: string, subDomainId: string) => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain => {
        if (domain.id === domainId) {
          const updatedDomain = {
            ...domain,
            subDomains: domain.subDomains.map(subDomain => {
              if (subDomain.id === subDomainId) {
                const newSelected = !subDomain.selected && !subDomain.partiallySelected;
                return {
                  ...subDomain,
                  selected: newSelected,
                  partiallySelected: false,
                  useCases: subDomain.useCases.map(uc => ({
                    ...uc,
                    selected: newSelected
                  }))
                };
              }
              return subDomain;
            })
          };
          
          // Update domain selection state based on subdomains
          const selectedSubDomains = updatedDomain.subDomains.filter(sd => sd.selected);
          const partialSubDomains = updatedDomain.subDomains.filter(sd => sd.partiallySelected);
          const totalSubDomains = updatedDomain.subDomains.length;
          
          if (selectedSubDomains.length === 0 && partialSubDomains.length === 0) {
            updatedDomain.selected = false;
            updatedDomain.partiallySelected = false;
          } else if (selectedSubDomains.length === totalSubDomains) {
            updatedDomain.selected = true;
            updatedDomain.partiallySelected = false;
          } else {
            updatedDomain.selected = false;
            updatedDomain.partiallySelected = true;
          }
          
          return updatedDomain;
        }
        return domain;
      })
    }));
  };

  const toggleDomainExpansion = (domainId: string) => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain =>
        domain.id === domainId 
          ? { ...domain, expanded: !domain.expanded }
          : domain
      )
    }));
  };

  const toggleSubDomainExpansion = (domainId: string, subDomainId: string) => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain => {
        if (domain.id === domainId) {
          return {
            ...domain,
            subDomains: domain.subDomains.map(subDomain =>
              subDomain.id === subDomainId 
                ? { ...subDomain, expanded: !subDomain.expanded }
                : subDomain
            )
          };
        }
        return domain;
      })
    }));
  };

  const setViewMode = (mode: 'current' | 'workspace') => {
    setState(prev => ({ ...prev, viewMode: mode }));
    vscode.postMessage({ type: WebviewMessages.SET_VIEW_MODE, viewMode: mode });
  };

  const setDiagramMode = (mode: 'detailed' | 'architecture') => {
    setState(prev => ({ ...prev, diagramMode: mode }));
  };

  const toggleDiagramOptions = () => {
    setState(prev => ({ ...prev, optionsExpanded: !prev.optionsExpanded }));
  };

  const selectAll = () => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain => ({
        ...domain,
        selected: true,
        partiallySelected: false,
        subDomains: domain.subDomains.map(sd => ({
          ...sd,
          selected: true,
          partiallySelected: false,
          useCases: sd.useCases.map(uc => ({ ...uc, selected: true }))
        }))
      }))
    }));
  };

  const selectNone = () => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain => ({
        ...domain,
        selected: false,
        partiallySelected: false,
        subDomains: domain.subDomains.map(sd => ({
          ...sd,
          selected: false,
          partiallySelected: false,
          useCases: sd.useCases.map(uc => ({ ...uc, selected: false }))
        }))
      }))
    }));
  };

  const selectCurrentFileOnly = () => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain => {
        const shouldSelect = domain.inCurrentFile;
        return {
          ...domain,
          selected: shouldSelect,
          partiallySelected: false,
          subDomains: domain.subDomains.map(sd => {
            const subShouldSelect = shouldSelect && sd.inCurrentFile;
            return {
              ...sd,
              selected: subShouldSelect,
              partiallySelected: false,
              useCases: sd.useCases.map(uc => ({ ...uc, selected: subShouldSelect }))
            };
          })
        };
      })
    }));
  };

  const toggleReferences = (domainId: string, subDomainId: string) => {
    setState(prev => ({
      ...prev,
      domains: prev.domains.map(domain => {
        if (domain.id === domainId) {
          return {
            ...domain,
            subDomains: domain.subDomains.map(subDomain =>
              subDomain.id === subDomainId 
                ? { ...subDomain, showReferences: !subDomain.showReferences }
                : subDomain
            )
          };
        }
        return domain;
      })
    }));
  };

  const handleRefresh = () => {
    vscode.postMessage({ type: WebviewMessages.REFRESH });
  };

  const handlePreviewWithState = (currentState: ViewState) => {
    const selectedItems = getSelectedItemsFromState(currentState);
    
    // if (selectedItems.useCases.length === 0) {
    //   // Show a helpful message when no use cases are selected
    //   vscode.postMessage({
    //     type: WebviewMessages.SHOW_INFORMATION,
    //     message: 'Please select at least one use case to preview the domain diagram.'
    //   });
    //   return;
    // }
    
    vscode.postMessage({ 
      type: WebviewMessages.PREVIEW, 
      selectedDomains: selectedItems.domains,
      selectedUseCases: selectedItems.useCases,
      diagramMode: currentState.diagramMode
    });
  };

  const getSelectedItemsFromState = (currentState: ViewState) => {
    const selectedDomains: Domain[] = [];
    const selectedUseCases: UseCase[] = [];
    
    currentState.domains.forEach(domain => {
      if (domain.selected || domain.partiallySelected) {
        selectedDomains.push(domain);
      }
      domain.subDomains.forEach(subDomain => {
        subDomain.useCases.forEach(useCase => {
          if (useCase.selected) {
            selectedUseCases.push(useCase);
          }
        });
      });
    });
    
    return { domains: selectedDomains, useCases: selectedUseCases };
  };


  // ===== CALCULATED VALUES =====

  const selectedCount = {
    domains: state.domains.filter(d => d.selected).length,
    subDomains: state.domains.reduce((acc, domain) => 
      acc + domain.subDomains.filter(sd => sd.selected).length, 0),
    useCases: state.domains.reduce((acc, domain) => 
      acc + domain.subDomains.reduce((subAcc, subDomain) => 
        subAcc + subDomain.useCases.filter(uc => uc.selected).length, 0), 0)
  };


  // ===== RENDER =====

  if (state.isLoading) {
    return (
      <div className="loading-container">
        <div className="loading-spinner"></div>
        <div>Loading domains...</div>
      </div>
    );
  }

  return (
    <div className="domains-view">
      {/* Header */}
      <div className="header">

        <div className="view-mode-toggle">
          <button 
            className={`mode-btn ${state.viewMode === 'current' ? 'active' : ''}`}
            onClick={() => setViewMode('current')}
            title="Show domains from current file only"
          >
            Current File
          </button>
          <button 
            className={`mode-btn ${state.viewMode === 'workspace' ? 'active' : ''}`}
            onClick={() => setViewMode('workspace')}
            title="Show domains from entire workspace"
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
                    className={`option-btn ${state.diagramMode === 'detailed' ? 'active' : ''}`}
                    onClick={() => setDiagramMode('detailed')}
                    title="Show detailed domain diagram with use cases"
                  >
                    Detailed
                  </button>
                  <button 
                    className={`option-btn ${state.diagramMode === 'architecture' ? 'active' : ''}`}
                    onClick={() => setDiagramMode('architecture')}
                    title="Show architecture view - subdomain connections only"
                  >
                    Architecture
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
        
        <div className="selection-info">
          <div className="selection-summary">
            <span className="count-item">
              <span className="count-number">{selectedCount.useCases}</span>
              <span className="count-label">use cases</span>
            </span>
            <span className="count-separator">•</span>
            <span className="count-item">
              <span className="count-number">{selectedCount.subDomains}</span>
              <span className="count-label">subdomains</span>
            </span>
            <span className="count-separator">•</span>
            <span className="count-item">
              <span className="count-number">{selectedCount.domains}</span>
              <span className="count-label">domains</span>
            </span>
          </div>
        </div>
      </div>

      {/* Domains list */}
      <div className="tree-container">
        {state.domains.length === 0 ? (
          <div className="no-domains">No domains found</div>
        ) : (
          state.domains.map(domain => (
            <DomainItem 
              key={domain.id}
              domain={domain}
              onToggleDomain={() => toggleDomain(domain.id)}
              onToggleSubDomain={(subDomainId) => toggleSubDomain(domain.id, subDomainId)}
              onToggleUseCase={(subDomainId, useCaseId) => toggleUseCase(domain.id, subDomainId, useCaseId)}
              onToggleExpansion={() => toggleDomainExpansion(domain.id)}
              onToggleSubDomainExpansion={(subDomainId) => toggleSubDomainExpansion(domain.id, subDomainId)}
              onToggleReferences={(subDomainId) => toggleReferences(domain.id, subDomainId)}
              viewMode={state.viewMode}
            />
          ))
        )}
      </div>

    </div>
  );
};

interface DomainItemProps {
  domain: Domain;
  onToggleDomain: () => void;
  onToggleSubDomain: (subDomainId: string) => void;
  onToggleUseCase: (subDomainId: string, useCaseId: string) => void;
  onToggleExpansion: () => void;
  onToggleSubDomainExpansion: (subDomainId: string) => void;
  onToggleReferences: (subDomainId: string) => void;
  viewMode: 'current' | 'workspace';
}

const DomainItem: React.FC<DomainItemProps> = ({
  domain,
  onToggleDomain,
  onToggleSubDomain,
  onToggleUseCase,
  onToggleExpansion,
  onToggleSubDomainExpansion,
  onToggleReferences,
  viewMode
}) => {
  const domainClasses = [
    'tree-node',
    'domain-node',
    domain.selected ? 'selected' : '',
    domain.partiallySelected ? 'partially-selected' : '',
    !domain.inCurrentFile && viewMode === 'workspace' ? 'non-current-file' : ''
  ].filter(Boolean).join(' ');

  // Calculate actual counters from current state
  const totalUseCases = domain.subDomains.reduce((total, sd) => total + sd.useCases.length, 0);
  const selectedUseCases = domain.subDomains.reduce((total, sd) => 
    total + sd.useCases.filter(uc => uc.selected).length, 0);

  return (
    <div className={domainClasses}>
      <div className="node-content" onClick={onToggleDomain}>
        <span 
          className="expander" 
          onClick={(e) => { e.stopPropagation(); onToggleExpansion(); }}
          title={domain.expanded ? 'Collapse' : 'Expand'}
          role="button"
          tabIndex={0}
        >
          {domain.expanded ? '▼' : '▶'}
        </span>
        
        <div className="checkbox-container">
          <div 
            className={`custom-checkbox ${domain.selected ? 'checked' : domain.partiallySelected ? 'indeterminate' : ''}`}
            title="Select/deselect domain"
            role="checkbox"
            aria-checked={domain.selected ? 'true' : domain.partiallySelected ? 'mixed' : 'false'}
          >
            <span className="checkbox-symbol">
              {domain.selected ? '✓' : domain.partiallySelected ? '▣' : '○'}
            </span>
          </div>
        </div>
        
        <div className="node-info">
          <div className="node-header">
            <span className="node-name">{domain.name}</span>
            <span className="use-case-badge" title={`${selectedUseCases} of ${totalUseCases} use cases selected`}>
              {selectedUseCases}/{totalUseCases}
            </span>
          </div>
          <div className="node-meta">
            {domain.subDomains.length} subdomain{domain.subDomains.length !== 1 ? 's' : ''}
          </div>
        </div>
      </div>

      <div className="node-children" style={{ display: domain.expanded ? 'block' : 'none' }} role="group">
        {domain.subDomains.map(subDomain => (
          <SubDomainItem
            key={subDomain.id}
            subDomain={subDomain}
            onToggleSubDomain={() => onToggleSubDomain(subDomain.id)}
            onToggleUseCase={(useCaseId) => onToggleUseCase(subDomain.id, useCaseId)}
            onToggleExpansion={() => onToggleSubDomainExpansion(subDomain.id)}
            onToggleReferences={() => onToggleReferences(subDomain.id)}
            viewMode={viewMode}
          />
        ))}
      </div>
    </div>
  );
};

interface SubDomainItemProps {
  subDomain: SubDomain;
  onToggleSubDomain: () => void;
  onToggleUseCase: (useCaseId: string) => void;
  onToggleExpansion: () => void;
  onToggleReferences: () => void;
  viewMode: 'current' | 'workspace';
}

const SubDomainItem: React.FC<SubDomainItemProps> = ({
  subDomain,
  onToggleSubDomain,
  onToggleUseCase,
  onToggleExpansion,
  onToggleReferences,
  viewMode
}) => {
  const isEmpty = subDomain.useCases.length === 0;
  const hasReferences = subDomain.referencedIn && subDomain.referencedIn.length > 0;
  const isSelectable = !isEmpty || hasReferences;
  
  const selectedCount = subDomain.useCases.filter(uc => uc.selected).length;
  const refIndicator = hasReferences ? (
    <span className="ref-indicator" title={`${subDomain.referencedIn.length} cross-references`}>
      🔗 {subDomain.referencedIn.length}
    </span>
  ) : null;

  const subDomainClasses = [
    'tree-node',
    'subdomain-node',
    !isSelectable ? 'empty-subdomain-node' : '',
    subDomain.selected ? 'selected' : '',
    subDomain.partiallySelected ? 'partially-selected' : '',
    !subDomain.inCurrentFile && viewMode === 'workspace' ? 'non-current-file' : ''
  ].filter(Boolean).join(' ');
  
  const checkboxSymbol = isEmpty ? '∅' : (subDomain.selected ? '✓' : subDomain.partiallySelected ? '▣' : '○');
  const checkboxClass = subDomain.selected ? 'checked' : (subDomain.partiallySelected ? 'indeterminate' : '');
  const clickHandler = isSelectable ? onToggleSubDomain : undefined;

  return (
    <div className={subDomainClasses}>
      <div className="node-content" onClick={clickHandler}>
        {isSelectable ? (
          <span 
            className="expander" 
            onClick={(e) => { e.stopPropagation(); onToggleExpansion(); }}
            title={subDomain.expanded ? 'Collapse' : 'Expand'}
            role="button"
            tabIndex={0}
          >
            {subDomain.expanded ? '▼' : '▶'}
          </span>
        ) : (
          <span className="expander-placeholder"></span>
        )}
        
        <div className="checkbox-container">
          <div 
            className={`custom-checkbox ${checkboxClass}`}
            title={!isSelectable ? 'No use cases or references to select' : 'Select/deselect subdomain'}
            role="checkbox"
            aria-checked={!isSelectable ? 'false' : (subDomain.selected ? 'true' : subDomain.partiallySelected ? 'mixed' : 'false')}
          >
            <span className="checkbox-symbol">
              {checkboxSymbol}
            </span>
          </div>
        </div>
        
        <div className="node-info">
          <div className="node-header">
            <span className="node-name">{subDomain.name}{refIndicator}</span>
            <span className={`use-case-badge ${!isSelectable ? 'empty' : ''}`}
                  title={!isSelectable ? 'No use cases or references' : `${selectedCount} of ${subDomain.useCases.length} use cases selected`}>
              {!isSelectable ? '0' : `${selectedCount}/${subDomain.useCases.length}`}
            </span>
          </div>
        </div>
      </div>

      <div className="node-children" style={{ display: subDomain.expanded ? 'block' : 'none' }} role="group">
        {subDomain.expanded && (
          <>
            {/* Entry Point use cases */}
            {!isEmpty && (
              <div className="entry-point-usecases">
                {subDomain.useCases.map(useCase => (
                  <UseCaseItem
                    key={useCase.id}
                    useCase={useCase}
                    onToggle={() => onToggleUseCase(useCase.id)}
                  />
                ))}
              </div>
            )}
            
            {/* Cross-references */}
            {hasReferences && (
              <div className="cross-references">
                <div className="section-header">
                  <span className="section-icon">🔗</span>
                  <span className="section-title">Also Involved In</span>
                  <button 
                    className="toggle-refs-btn" 
                    onClick={(e) => { e.stopPropagation(); onToggleReferences(); }}
                  >
                    {subDomain.showReferences ? 'Hide' : 'Show'} ({subDomain.referencedIn.length})
                  </button>
                </div>
                {subDomain.showReferences && (
                  <CrossReferencesList references={subDomain.referencedIn} />
                )}
              </div>
            )}
            
            {/* Empty state */}
            {isEmpty && !hasReferences && (
              <div className="empty-subdomain">No use cases defined</div>
            )}
          </>
        )}
      </div>
    </div>
  );
};

interface UseCaseItemProps {
  useCase: UseCase;
  onToggle: () => void;
}

const UseCaseItem: React.FC<UseCaseItemProps> = ({ useCase, onToggle }) => {
  const useCaseClasses = [
    'tree-node',
    'usecase-node',
    useCase.selected ? 'selected' : ''
  ].filter(Boolean).join(' ');

  return (
    <div className={useCaseClasses}>
      <div className="node-content" onClick={onToggle}>
        <div className="checkbox-container">
          <div 
            className={`custom-checkbox ${useCase.selected ? 'checked' : ''}`}
            title="Select/deselect use case"
            role="checkbox"
            aria-checked={useCase.selected ? 'true' : 'false'}
          >
            <span className="checkbox-symbol">
              {useCase.selected ? '✓' : '○'}
            </span>
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

interface CrossReferencesListProps {
  references: UseCaseReference[];
}

const CrossReferencesList: React.FC<CrossReferencesListProps> = ({ references }) => {
  const getRoleIcon = (role: 'entry-point' | 'involved'): string => {
    const icons = {
      'entry-point': '🎯',
      'involved': '🔗'
    };
    return icons[role];
  };

  const navigateToUseCase = (_useCaseId: string) => {
    // TODO: Implement navigation to use case
  };

  return (
    <div className="references-list">
      {references.map(ref => (
        <div 
          key={ref.useCaseId}
          className={`reference-item ${ref.role}`} 
          onClick={() => navigateToUseCase(ref.useCaseId)}
        >
          <div className="ref-content">
            <span className="ref-role-icon">{getRoleIcon(ref.role)}</span>
            <div className="ref-info">
              <div className="ref-usecase">{ref.useCaseName}</div>
              <div className="ref-domain">{ref.domainName}</div>
            </div>
            <span className="ref-role-badge">
              {ref.role === 'entry-point' ? 'entry' : 'involved'}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
};
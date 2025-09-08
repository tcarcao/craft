import { Service, ServiceGroup, SubDomain } from '../types/domain';

export class ServicesViewService {
    constructor() { }

    updateServiceGroupSelectionForCurrentFile(serviceGroup: ServiceGroup, currentFileOnly: boolean) {
        if (currentFileOnly) {
            // Filter to only services in current file for selection calculation
            this.updateServiceGroupSelectionWithFilter(serviceGroup, (service) => service.inCurrentFile);
        } else {
            this.updateServiceGroupSelection(serviceGroup);
        }
    }

    private updateServiceGroupSelectionWithFilter(serviceGroup: ServiceGroup, serviceFilter: (service: Service) => boolean) {
        const relevantServices = serviceGroup.services.filter(serviceFilter);
        
        relevantServices.forEach(service => {           
            const relevantSubDomains = service.subDomains.filter(sd => serviceFilter(service) && sd.inCurrentFile);
            
            relevantSubDomains.forEach(subDomain => {
                if (subDomain.useCases.length === 0) {
                    return;
                }

                const selectedInSubDomain = subDomain.useCases.filter(uc => uc.selected).length;

                if (selectedInSubDomain === 0) {
                    subDomain.selected = false;
                    subDomain.partiallySelected = false;
                } else if (selectedInSubDomain === subDomain.useCases.length) {
                    subDomain.selected = true;
                    subDomain.partiallySelected = false;
                } else {
                    subDomain.selected = false;
                    subDomain.partiallySelected = true;
                }
            });

            if (relevantSubDomains.length === 0) {
                return;
            }

            const selectedInService = relevantSubDomains.filter(sd => sd.selected).length;
            const partiallySelectedSubDomains = relevantSubDomains.filter(sd => sd.partiallySelected).length;

            if (selectedInService === relevantSubDomains.length && relevantSubDomains.length > 0) {
                service.selected = true;
                service.partiallySelected = false;
            } else if (selectedInService > 0 || partiallySelectedSubDomains > 0) {
                service.selected = false;
                service.partiallySelected = true;
            } else {
                service.selected = false;
                service.partiallySelected = false;
            }
        });

        // Calculate group selection based only on relevant services
        const selectedCount = relevantServices.filter(s => s.selected).length;
        const partiallySelectedServices = relevantServices.filter(s => s.partiallySelected).length;

        if (selectedCount === relevantServices.length && relevantServices.length > 0) {
            serviceGroup.selected = true;
            serviceGroup.partiallySelected = false;
        } else if (selectedCount > 0 || partiallySelectedServices > 0) {
            serviceGroup.selected = false;
            serviceGroup.partiallySelected = true;
        } else {
            serviceGroup.selected = false;
            serviceGroup.partiallySelected = false;
        }
    }

    updateServiceGroupSelection(serviceGroup: ServiceGroup) {
        serviceGroup.services.forEach(service => {           
            
            service.subDomains.forEach(subDomain => {
                if (subDomain.useCases.length === 0) {
                    return;
                }

                const selectedInSubDomain = subDomain.useCases.filter(uc => uc.selected).length;

                if (selectedInSubDomain === 0) {
                    subDomain.selected = false;
                    subDomain.partiallySelected = false;
                } else if (selectedInSubDomain === subDomain.useCases.length) {
                    subDomain.selected = true;
                    subDomain.partiallySelected = false;
                } else {
                    subDomain.selected = false;
                    subDomain.partiallySelected = true;
                }
            })

            if (service.subDomains.length === 0) {
                return;
            }

            const selectedInService = service.subDomains.filter(sd => sd.selected).length;
            const partiallySelectedSubDomains= service.subDomains.filter(sd => sd.partiallySelected).length;

            if (selectedInService === service.subDomains.length && service.subDomains.length > 0) {
                service.selected = true;
                service.partiallySelected = false;
            } else if (selectedInService > 0 || partiallySelectedSubDomains > 0) {
                service.selected = false;
                service.partiallySelected = true;
            } else {
                service.selected = false;
                service.partiallySelected = false;
            }
        });

        const selectedCount = serviceGroup.services.filter(s => s.selected).length;
        const partiallySelectedServices = serviceGroup.services.filter(s => s.partiallySelected).length;

        if (selectedCount === serviceGroup.services.length && serviceGroup.services.length > 0) {
            serviceGroup.selected = true;
            serviceGroup.partiallySelected = false;
        } else if (selectedCount > 0 || partiallySelectedServices > 0) {
            serviceGroup.selected = false;
            serviceGroup.partiallySelected = true;
        } else {
            serviceGroup.selected = false;
            serviceGroup.partiallySelected = false;
        }
    }

    toggleServiceGroupSelection(serviceGroup: ServiceGroup, currentFileOnly: boolean = false) {
        const newSelectedState = !serviceGroup.selected && !serviceGroup.partiallySelected;
        this.toggleServiceGroupSelectionWith(serviceGroup, newSelectedState, currentFileOnly);
    }

    toggleServiceGroupSelectionWith(serviceGroup: ServiceGroup, selectedState: boolean, currentFileOnly: boolean = false) {
        const servicesToUpdate = currentFileOnly 
            ? serviceGroup.services.filter(s => s.inCurrentFile)
            : serviceGroup.services;

        servicesToUpdate.forEach(service => {
            service.selected = selectedState;
            const subDomainsToUpdate = currentFileOnly 
                ? service.subDomains.filter(sd => sd.inCurrentFile)
                : service.subDomains;

            subDomainsToUpdate.forEach(sd => {
                sd.selected = selectedState;
                sd.useCases.forEach(uc => uc.selected = selectedState)
            });
        });
        this.updateServiceGroupSelectionForCurrentFile(serviceGroup, currentFileOnly);
    }

    toggleServiceSelection(serviceGroup: ServiceGroup, serviceId: string, currentFileOnly: boolean = false) {
        const service = serviceGroup.services.find(s => s.id === serviceId);

        if (service) {
            const newSelectedState = !service.selected && !service.partiallySelected;
            service.selected = newSelectedState;
            
            const subDomainsToUpdate = currentFileOnly 
                ? service.subDomains.filter(sd => sd.inCurrentFile)
                : service.subDomains;

            subDomainsToUpdate.forEach(sd => { 
                sd.selected = newSelectedState;
                sd.useCases.forEach(uc => uc.selected = newSelectedState);
            });
        } 
        this.updateServiceGroupSelectionForCurrentFile(serviceGroup, currentFileOnly);  
    }

    toggleSubDomainSelection(serviceGroup: ServiceGroup, service: Service, subDomainId: string, currentFileOnly: boolean = false) {
        const subDomain = service.subDomains.find(sd => sd.id === subDomainId);

        if (subDomain) {
            const newSelectedState = !subDomain.selected && !subDomain.partiallySelected;
            subDomain.selected = newSelectedState;  
            subDomain.useCases.forEach(uc => uc.selected = newSelectedState);
        } 
        this.updateServiceGroupSelectionForCurrentFile(serviceGroup, currentFileOnly);  
    }

    toggleUseCaseSelection(serviceGroup: ServiceGroup, subDomain: SubDomain, useCaseId: string, currentFileOnly: boolean = false) {
        const useCase = subDomain.useCases.find(uc => uc.id === useCaseId);

        if (useCase) {
            const newSelectedState = !useCase.selected;
            useCase.selected = newSelectedState;  
        } 
        this.updateServiceGroupSelectionForCurrentFile(serviceGroup, currentFileOnly);  
    }

    public selectAll(serviceGroups: ServiceGroup[], currentFileOnly: boolean): void {
        serviceGroups.forEach(serviceGroup => {
            if (!currentFileOnly || serviceGroup.inCurrentFile) {
                this.toggleServiceGroupSelectionWith(serviceGroup, true);
            }
        });
    }

    public selectNone(serviceGroups: ServiceGroup[]): void {
        serviceGroups.forEach(serviceGroup => {
            this.toggleServiceGroupSelectionWith(serviceGroup, false);
        });
    }
}
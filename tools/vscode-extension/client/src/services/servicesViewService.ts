import { Service, ServiceGroup, SubDomain } from '../types/domain';

export class ServicesViewService {
    constructor() { }

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

    toggleServiceGroupSelection(serviceGroup: ServiceGroup) {
        const newSelectedState = !serviceGroup.selected && !serviceGroup.partiallySelected;
        this.toggleServiceGroupSelectionWith(serviceGroup, newSelectedState);
    }

    toggleServiceGroupSelectionWith(serviceGroup: ServiceGroup, selectedState: boolean) {
        serviceGroup.services.forEach(service => {
            service.selected = selectedState;
            service.subDomains.forEach(sd => {
                sd.selected = selectedState;
                sd.useCases.forEach(uc => uc.selected = selectedState)
            });
        });
        this.updateServiceGroupSelection(serviceGroup);
    }

    toggleServiceSelection(serviceGroup: ServiceGroup, serviceId: string) {
        const service = serviceGroup.services.find(s => s.id === serviceId);

        if (service) {
            const newSelectedState = !service.selected && !service.partiallySelected;
            service.selected = newSelectedState;
            service.subDomains.forEach(sd => { 
                sd.selected = newSelectedState;
                sd.useCases.forEach(uc => uc.selected = newSelectedState);
            });
        } 
        this.updateServiceGroupSelection(serviceGroup);  
    }

    toggleSubDomainSelection(serviceGroup: ServiceGroup, service: Service, subDomainId: string) {
        const subDomain = service.subDomains.find(sd => sd.id === subDomainId);

        if (subDomain) {
            const newSelectedState = !subDomain.selected && !subDomain.partiallySelected;
            subDomain.selected = newSelectedState;  
            subDomain.useCases.forEach(uc => uc.selected = newSelectedState);
        } 
        this.updateServiceGroupSelection(serviceGroup);  
    }

    toggleUseCaseSelection(serviceGroup: ServiceGroup, subDomain: SubDomain, useCaseId: string) {
        const useCase = subDomain.useCases.find(uc => uc.id === useCaseId);

        if (useCase) {
            const newSelectedState = !useCase.selected;
            useCase.selected = newSelectedState;  
        } 
        this.updateServiceGroupSelection(serviceGroup);  
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
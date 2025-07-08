import { Domain } from '../types/domain';

export class DomainsViewService {

    constructor() { }

    /**
     * Update domain counts and selection states based on subdomain/usecase selections
     */
    updateDomainCounts(domain: Domain): void {
        let totalUseCases = 0;
        let selectedUseCases = 0;
        let selectedSubDomains = 0;

        domain.subDomains.forEach(subDomain => {
            totalUseCases += subDomain.useCases.length;
            
            const selectedInSubDomain = subDomain.useCases.filter(uc => uc.selected).length;
            selectedUseCases += selectedInSubDomain;

            // Update subdomain selection state
            if (subDomain.useCases.length === 0) {
                // Empty subdomain - not selectable
                subDomain.selected = false;
                subDomain.partiallySelected = false;
            } else if (selectedInSubDomain === 0) {
                subDomain.selected = false;
                subDomain.partiallySelected = false;
            } else if (selectedInSubDomain === subDomain.useCases.length) {
                subDomain.selected = true;
                subDomain.partiallySelected = false;
                selectedSubDomains++;
            } else {
                subDomain.selected = false;
                subDomain.partiallySelected = true;
            }
        });

        domain.totalUseCases = totalUseCases;
        domain.selectedUseCases = selectedUseCases;
        domain.selectedSubDomains = selectedSubDomains;

        // Update domain selection state
        const nonEmptySubDomains = domain.subDomains.filter(sd => sd.useCases.length > 0);
        const selectedNonEmptySubDomains = nonEmptySubDomains.filter(sd => sd.selected).length;
        const partiallySelectedNonEmptySubDomains = nonEmptySubDomains.filter(sd => sd.partiallySelected).length;

        if (selectedNonEmptySubDomains === nonEmptySubDomains.length && nonEmptySubDomains.length > 0) {
            domain.selected = true;
            domain.partiallySelected = false;
        } else if (selectedNonEmptySubDomains > 0 || partiallySelectedNonEmptySubDomains > 0) {
            domain.selected = false;
            domain.partiallySelected = true;
        } else {
            domain.selected = false;
            domain.partiallySelected = false;
        }
    }

    public toggleDomainSelection(domain: Domain, selected: boolean): void {
        domain.subDomains.forEach(subDomain => {
            subDomain.selected = selected;
            subDomain.useCases.forEach(useCase => {
                useCase.selected = selected;
            });
        });
        this.updateDomainCounts(domain);
    }

    public toggleSubDomainSelection(domain: Domain, subDomainId: string, selected: boolean): void {
        const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
        if (subDomain && subDomain.useCases.length > 0) {
            subDomain.selected = selected;
            subDomain.useCases.forEach(useCase => {
                useCase.selected = selected;
                this.updateDomainCounts(domain);
            });
        }
    }

    public toggleUseCaseSelection(domain: Domain, subDomainId: string, useCaseId: string): void {
        const subDomain = domain.subDomains.find(sd => sd.id === subDomainId);
        if (subDomain) {
            const useCase = subDomain.useCases.find(uc => uc.id === useCaseId);
            if (useCase) {
                useCase.selected = !useCase.selected;
                this.updateDomainCounts(domain);
            }
        }
    }

    public selectAll(domains: Domain[], currentFileOnly: boolean): void {
        domains.forEach(domain => {
            if (!currentFileOnly || domain.inCurrentFile) {
                this.toggleDomainSelection(domain, true);
            }
        });
    }

    public selectNone(domains: Domain[]): void {
        domains.forEach(domain => {
            this.toggleDomainSelection(domain, false);
        });
    }
}
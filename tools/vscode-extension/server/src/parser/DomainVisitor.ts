// server/src/DomainExtractor.ts
import { ArchDSLVisitor } from './generated/ArchDSLVisitor';
import { 
	DslContext, 
	ServicesContext, 
	Service_definitionContext, 
	Use_caseContext,
	Domain_listContext,
	Domain_refContext,
	DatastoreContext,
	Service_nameContext,
	// Service_propertiesContext,
	Service_propertyContext,
	Sync_actionContext,
	Async_actionContext,
	Internal_actionContext,
	DomainContext,
	StringContext
} from './generated/ArchDSLParser';
import { ServiceDefinition, UseCaseInfo } from '../../../shared/lib/types/domain-extraction';


export class DomainVisitor extends ArchDSLVisitor<void> {
	public domains = new Set<string>();
	public useCases: UseCaseInfo[] = [];
	public serviceDefinitions: ServiceDefinition[] = [];
	
	// Current use case being processed
	private currentUseCase: UseCaseInfo | null = null;
	private currentUseCaseDomains = new Set<string>();
	
	// Current lists being collected
	private currentDomainRefList: string[] = [];
	private currentDatastoreList: string[] = [];
	private isInDomainList = false;

	// Visit the root DSL context
	visitDsl = (ctx: DslContext): void => {
		this.visitChildren(ctx);
	};

	// Visit services section
	visitServices = (ctx: ServicesContext): void => {
		this.visitChildren(ctx);
	};

	// Visit individual service definition
	visitService_definition = (ctx: Service_definitionContext): void => {
		
		const serviceDefinition: ServiceDefinition = {
			name: 'Unknown Service',
			domains: [],
			dataStores: [],
			language: undefined,
			parentDomain: undefined,
			blockRange: {
				startLine: ctx.start?.line || 0,
				endLine: ctx.stop?.line || 0,
				fileUri: 'unknown'
			}
		};
		this.serviceDefinitions.push(serviceDefinition);

		// Visit children to collect service data
		this.visitChildren(ctx);
	};

	// Visit service name
	visitService_name = (ctx: Service_nameContext): void => {
		const nameText = ctx.getText();
		
		// The name is either an IDENTIFIER or STRING
		if (nameText.startsWith('"') && nameText.endsWith('"')) {
			// Remove quotes for STRING
			const serviceName = nameText.slice(1, -1);
			if (this.serviceDefinitions.length > 0) {
				this.serviceDefinitions[this.serviceDefinitions.length - 1].name = serviceName;
			}
		} else {
			// IDENTIFIER
			if (this.serviceDefinitions.length > 0) {
				this.serviceDefinitions[this.serviceDefinitions.length - 1].name = nameText;
			}
		}
	};

	// Visit service property (domains, data-stores, language)
	visitService_property = (ctx: Service_propertyContext): void => {
		const propertyText = ctx.getText();
		
		if (propertyText.startsWith('domains:')) {
			this.isInDomainList = true;
			this.currentDomainRefList = [];
			// Visit children to collect domains from domain_list
			this.visitChildren(ctx);
			this.isInDomainList = false;
		} else if (propertyText.startsWith('data-stores:')) {
			this.isInDomainList = false; // This is data-stores, not domains
			this.currentDatastoreList = [];
			// Visit children to collect data stores from datastore_list
			this.visitChildren(ctx);
		} else if (propertyText.startsWith('language:')) {
			const parts = propertyText.split(':');
			if (parts.length > 1 && this.serviceDefinitions.length > 0) {
				this.serviceDefinitions[this.serviceDefinitions.length - 1].language = parts[1].trim();
			}
		}
		
		// TODO: Handle parent_domain when you extend the grammar
		// else if (propertyText.startsWith('parent_domain:')) {
		//     const parts = propertyText.split(':');
		//     if (parts.length > 1 && this.serviceDefinitions.length > 0) {
		//         this.serviceDefinitions[this.serviceDefinitions.length - 1].parentDomain = parts[1].trim();
		//     }
		// }
	};

	// Visit domain list - THIS IS WHERE WE ADD DOMAINS
	visitDomain_list = (ctx: Domain_listContext): void => {
		
		// Clear the current list before collecting
		this.currentDomainRefList = [];
		
		// Visit children to collect domain_or_datastore items
		this.visitChildren(ctx);
		
		// Now we know this is a domain list, so add all items as domains
		this.currentDomainRefList.forEach(domainName => {
			this.domains.add(domainName);
			
			// Add to current service definition domains
			if (this.serviceDefinitions.length > 0 && this.isInDomainList) {
				this.serviceDefinitions[this.serviceDefinitions.length - 1].domains.push(domainName);
			}
		});
	};

	// Visit individual domain - COLLECT ITEMS BUT DON'T ADD AS DOMAINS YET
	visitDomain_ref = (ctx: Domain_refContext): void => {
		const itemName = ctx.getText().trim();
		if (itemName) {
			
			// Just collect the item - don't add as domain yet
			this.currentDomainRefList.push(itemName);
			
			// If this is in data-stores context, add to dataStores
			if (this.serviceDefinitions.length > 0 && !this.isInDomainList) {
				const currentService = this.serviceDefinitions[this.serviceDefinitions.length - 1];
				if (!currentService.dataStores) {currentService.dataStores = [];}
				currentService.dataStores.push(itemName);
			}
		}
	};

	// Visit individual datastore - COLLECT ITEMS BUT DON'T ADD YET
	visitDatastore = (ctx: DatastoreContext): void => {
		const itemName = ctx.getText().trim();
		if (itemName) {
			
			// Just collect the item - don't add as domain yet
			this.currentDatastoreList.push(itemName);
			
			// If this is in data-stores context, add to dataStores
			if (this.serviceDefinitions.length > 0 && !this.isInDomainList) {
				const currentService = this.serviceDefinitions[this.serviceDefinitions.length - 1];
				if (!currentService.dataStores) {currentService.dataStores = [];}
				currentService.dataStores.push(itemName);
			}
		}
	};

	// Visit domain context (used in actions) - THIS IS ALSO WHERE DOMAINS ARE CLEARLY DEFINED
	visitDomain = (ctx: DomainContext): void => {
		const domainName = ctx.getText().trim();
		if (domainName) {
			this.domains.add(domainName);
			
			// Add to current use case domains
			if (this.currentUseCase) {
				this.currentUseCaseDomains.add(domainName);
			}
		}
	};

	// Visit use case
	visitUse_case = (ctx: Use_caseContext): void => {
		
		// Initialize current use case
		this.currentUseCase = {
			name: 'Unknown Use Case',
			entryPointSubDomain: null,
			allDomains: [],
			scenarios: [],
			blockRange: {
				startLine: ctx.start?.line || 0,
				endLine: ctx.stop?.line || 0,
				fileUri: 'unknown'
			}
		};
		this.currentUseCaseDomains.clear();

		// Visit children to collect use case data
		this.visitChildren(ctx);

		// Finalize the use case
		const domainsArray = Array.from(this.currentUseCaseDomains);
		if (domainsArray.length > 0) {
			// Primary domain is the first domain encountered
			this.currentUseCase.entryPointSubDomain = domainsArray[0];
		}
		this.currentUseCase.allDomains = domainsArray;

		this.useCases.push(this.currentUseCase);

		// Reset current use case
		this.currentUseCase = null;
		this.currentUseCaseDomains.clear();
	};

	// Visit string (use case name)
	visitString = (ctx: StringContext): void => {
		const stringText = ctx.getText();
		if (stringText.startsWith('"') && stringText.endsWith('"')) {
			const content = stringText.slice(1, -1);
			
			// If we're in a use case, this is the use case name
			if (this.currentUseCase) {
				this.currentUseCase.name = content;
			}
		}
	};

	// Visit sync action (domain asks domain)
	visitSync_action = (ctx: Sync_actionContext): void => {
		const actionText = ctx.getText().replace(/\s+/g, ' ').trim();
		
		if (this.currentUseCase) {
			this.currentUseCase.scenarios.push(`Sync: ${actionText}`);
		}
		
		// Visit children to extract domains
		this.visitChildren(ctx);
	};

	// Visit async action (domain notifies event)
	visitAsync_action = (ctx: Async_actionContext): void => {
		const actionText = ctx.getText().replace(/\s+/g, ' ').trim();
		
		if (this.currentUseCase) {
			this.currentUseCase.scenarios.push(`Async: ${actionText}`);
		}
		
		// Visit children to extract domains
		this.visitChildren(ctx);
	};

	// Visit internal action (domain verb phrase)
	visitInternal_action = (ctx: Internal_actionContext): void => {
		const actionText = ctx.getText().replace(/\s+/g, ' ').trim();
		
		if (this.currentUseCase) {
			this.currentUseCase.scenarios.push(`Internal: ${actionText}`);
		}
		
		// Visit children to extract domains
		this.visitChildren(ctx);
	};
}
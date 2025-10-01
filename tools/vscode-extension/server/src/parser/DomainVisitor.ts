// server/src/DomainExtractor.ts
import { CraftVisitor } from './generated/CraftVisitor.js';
import { 
	DslContext, 
	Services_defContext, 
	Service_blockContext, 
	Service_block_listContext,
	Service_defContext,
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
	StringContext,
	Domain_defContext,
	Domains_defContext,
	Domain_blockContext,
	Domain_block_listContext,
	Domain_nameContext,
	Subdomain_listContext,
	SubdomainContext,
	Actor_defContext,
	Actors_defContext,
	Actor_definitionContext,
	Actor_definition_listContext,
	ActorTypeContext,
	Actor_nameContext
} from './generated/CraftParser.js';
import { ServiceDefinition, UseCaseInfo, DomainDefinition, ActorDefinition } from '../../../shared/lib/types/domain-extraction.js';


export class DomainVisitor extends CraftVisitor<void> {
	public domains = new Set<string>();
	public useCases: UseCaseInfo[] = [];
	public serviceDefinitions: ServiceDefinition[] = [];
	public domainDefinitions: DomainDefinition[] = [];
	public actorDefinitions: ActorDefinition[] = [];
	
	// Current use case being processed
	private currentUseCase: UseCaseInfo | null = null;
	private currentUseCaseDomains = new Set<string>();
	
	// Current lists being collected
	private currentDomainRefList: string[] = [];
	private currentDatastoreList: string[] = [];
	private isInDomainList = false;
	
	// Current domain definition being processed
	private currentDomainDefinition: DomainDefinition | null = null;
	private currentSubDomainList: string[] = [];

	// Visit the root DSL context
	visitDsl = (ctx: DslContext): void => {
		this.visitChildren(ctx);
	};

	// Visit services section
	visitServices_def = (ctx: Services_defContext): void => {
		this.visitChildren(ctx);
	};

	// Visit service block list
	visitService_block_list = (ctx: Service_block_listContext): void => {
		this.visitChildren(ctx);
	};

	// Visit individual service block
	visitService_block = (ctx: Service_blockContext): void => {
		const serviceDefinition: ServiceDefinition = {
			name: 'Unknown Service',
			domains: [],
			dataStores: [],
			language: undefined,
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

	// Visit single service definition
	visitService_def = (ctx: Service_defContext): void => {
		const serviceDefinition: ServiceDefinition = {
			name: 'Unknown Service',
			domains: [],
			dataStores: [],
			language: undefined,
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

	// =====================================
	// Domain Definition Visitors
	// =====================================

	// Visit single domain definition - "domain domain_name { subdomain_list }"
	visitDomain_def = (ctx: Domain_defContext): void => {
		this.currentDomainDefinition = {
			name: 'Unknown Domain',
			subDomains: [],
			blockRange: {
				startLine: ctx.start?.line || 0,
				endLine: ctx.stop?.line || 0,
				fileUri: 'unknown'
			}
		};
		this.currentSubDomainList = [];

		// Visit children to extract domain name and subdomain list
		this.visitChildren(ctx);

		// Finalize domain definition
		if (this.currentDomainDefinition) {
			this.currentDomainDefinition.subDomains = [...this.currentSubDomainList];
			this.addOrMergeDomainDefinition(this.currentDomainDefinition);
		}

		// Reset state
		this.currentDomainDefinition = null;
		this.currentSubDomainList = [];
	};

	// Visit multiple domains definition - "domains { domain_block_list }"
	visitDomains_def = (ctx: Domains_defContext): void => {
		// Just visit children - individual domain blocks will be handled by visitDomain_block
		this.visitChildren(ctx);
	};

	// Visit domain block list
	visitDomain_block_list = (ctx: Domain_block_listContext): void => {
		this.visitChildren(ctx);
	};

	// Visit individual domain block
	visitDomain_block = (ctx: Domain_blockContext): void => {
		this.currentDomainDefinition = {
			name: 'Unknown Domain',
			subDomains: [],
			blockRange: {
				startLine: ctx.start?.line || 0,
				endLine: ctx.stop?.line || 0,
				fileUri: 'unknown'
			}
		};
		this.currentSubDomainList = [];

		// Visit children to extract domain name and subdomain list
		this.visitChildren(ctx);

		// Finalize domain definition
		if (this.currentDomainDefinition) {
			this.currentDomainDefinition.subDomains = [...this.currentSubDomainList];
			this.addOrMergeDomainDefinition(this.currentDomainDefinition);
		}

		// Reset state
		this.currentDomainDefinition = null;
		this.currentSubDomainList = [];
	};

	// Visit domain name
	visitDomain_name = (ctx: Domain_nameContext): void => {
		const domainName = ctx.getText().trim();
		if (domainName && this.currentDomainDefinition) {
			this.currentDomainDefinition.name = domainName;
		}
	};

	// Visit subdomain list
	visitSubdomain_list = (ctx: Subdomain_listContext): void => {
		// Visit children to collect individual subdomains
		this.visitChildren(ctx);
	};

	// Visit individual subdomain
	visitSubdomain = (ctx: SubdomainContext): void => {
		const subdomainName = ctx.getText().trim();
		if (subdomainName && this.currentDomainDefinition) {
			// Add to current subdomain list (duplicates will be handled by addOrMergeDomainDefinition)
			this.currentSubDomainList.push(subdomainName);
		}
	};

	// Helper method to add or merge domain definitions (similar to Go implementation)
	private addOrMergeDomainDefinition(newDomainDefinition: DomainDefinition): void {
		// Check if domain definition already exists
		const existingIndex = this.domainDefinitions.findIndex(def => def.name === newDomainDefinition.name);
		
		if (existingIndex !== -1) {
			// Domain exists, merge subdomains
			const existing = this.domainDefinitions[existingIndex];
			const mergedSubDomains = this.mergeSubDomains(existing.subDomains, newDomainDefinition.subDomains);
			this.domainDefinitions[existingIndex] = {
				...existing,
				subDomains: mergedSubDomains
			};
		} else {
			// Domain doesn't exist, add it (but first deduplicate subdomains)
			const deduplicatedSubDomains = this.deduplicateSubDomains(newDomainDefinition.subDomains);
			this.domainDefinitions.push({
				...newDomainDefinition,
				subDomains: deduplicatedSubDomains
			});
		}
	}

	// Helper method to merge two subdomain arrays, avoiding duplicates
	private mergeSubDomains(existing: string[], newSubDomains: string[]): string[] {
		const subDomainSet = new Set<string>();
		
		// Add existing subdomains
		existing.forEach(sub => subDomainSet.add(sub));
		
		// Add new subdomains
		newSubDomains.forEach(sub => subDomainSet.add(sub));
		
		return Array.from(subDomainSet);
	}

	// Helper method to remove duplicate subdomains from an array
	private deduplicateSubDomains(subDomains: string[]): string[] {
		return Array.from(new Set(subDomains));
	}

	// =====================================
	// Actor Definition Visitors
	// =====================================

	// Visit single actor definition - "actor actorType actor_name"
	visitActor_def = (ctx: Actor_defContext): void => {
		const actorDefinition: ActorDefinition = {
			name: 'Unknown Actor',
			type: 'user',
			blockRange: {
				startLine: ctx.start?.line || 0,
				endLine: ctx.stop?.line || 0,
				fileUri: 'unknown'
			}
		};

		// Visit children to extract actor type and name
		this.visitChildren(ctx);

		// Extract actor type
		const actorTypeChild = ctx.actorType();
		if (actorTypeChild) {
			const typeText = actorTypeChild.getText().trim();
			if (typeText === 'user' || typeText === 'system' || typeText === 'service') {
				actorDefinition.type = typeText;
			}
		}

		// Extract actor name
		const actorNameChild = ctx.actor_name();
		if (actorNameChild) {
			actorDefinition.name = actorNameChild.getText().trim();
		}

		this.actorDefinitions.push(actorDefinition);
	};

	// Visit multiple actors definition - "actors { actor_definition_list }"
	visitActors_def = (ctx: Actors_defContext): void => {
		// Just visit children - individual actor definitions will be handled by visitActor_definition
		this.visitChildren(ctx);
	};

	// Visit actor definition list
	visitActor_definition_list = (ctx: Actor_definition_listContext): void => {
		this.visitChildren(ctx);
	};

	// Visit individual actor definition within actors block
	visitActor_definition = (ctx: Actor_definitionContext): void => {
		const actorDefinition: ActorDefinition = {
			name: 'Unknown Actor',
			type: 'user',
			blockRange: {
				startLine: ctx.start?.line || 0,
				endLine: ctx.stop?.line || 0,
				fileUri: 'unknown'
			}
		};

		// Extract actor type
		const actorTypeChild = ctx.actorType();
		if (actorTypeChild) {
			const typeText = actorTypeChild.getText().trim();
			if (typeText === 'user' || typeText === 'system' || typeText === 'service') {
				actorDefinition.type = typeText;
			}
		}

		// Extract actor name
		const actorNameChild = ctx.actor_name();
		if (actorNameChild) {
			actorDefinition.name = actorNameChild.getText().trim();
		}

		this.actorDefinitions.push(actorDefinition);
	};

	// Visit actor type (redundant but included for completeness)
	visitActorType = (ctx: ActorTypeContext): void => {
		// This is handled by the parent contexts
		this.visitChildren(ctx);
	};

	// Visit actor name (redundant but included for completeness)
	visitActor_name = (ctx: Actor_nameContext): void => {
		// This is handled by the parent contexts
		this.visitChildren(ctx);
	};
}
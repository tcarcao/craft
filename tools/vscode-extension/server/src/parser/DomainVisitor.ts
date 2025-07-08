// server/src/DomainExtractor.ts
import { ArchDSLVisitor } from './generated/ArchDSLVisitor';
import { 
	DslContext, 
	ServicesContext, 
	Service_definitionContext, 
	Use_caseContext,
	Domain_listContext,
	Domain_or_datastoreContext,
	Service_nameContext,
	// Service_propertiesContext,
	Service_propertyContext,
	Sync_actionContext,
	Async_actionContext,
	Internal_actionContext,
	DomainContext,
	StringContext
} from './generated/ArchDSLParser';
import { UseCaseInfo } from '../../../shared/lib/types/domain-extraction';


export class DomainVisitor extends ArchDSLVisitor<void> {
	public domains = new Set<string>();
	public useCases: UseCaseInfo[] = [];
	
	// Current use case being processed
	private currentUseCase: UseCaseInfo | null = null;
	private currentUseCaseDomains = new Set<string>();

	// Visit the root DSL context
	visitDsl = (ctx: DslContext): void => {
		this.visitChildren(ctx);
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
import { ParseTree, ParserRuleContext, TerminalNode, Token } from 'antlr4ng';
import { BlockRange } from '../../../shared/lib/types/domain-extraction.js';

/**
 * Extracts minimal subtree from Craft AST with ancestry
 * @param ast - The full AST from antlr4ng (ParserRuleContext)
 * @param selectedRanges - Array of {startLine, endLine} objects for selected blocks
 * @param originalText - The original DSL text
 * @returns The extracted DSL text with minimal ancestry
 */
export function extractMinimalSubtree(
  ast: ParserRuleContext, 
  selectedRanges: BlockRange[], 
  originalText: string
): string {
  const lines = originalText.split('\n');
  const selectedNodes = findNodesInRanges(ast, selectedRanges);
  const requiredNodes = new Set<ParserRuleContext>();
  
  
  
  
  // Check if this is architectural-only extraction (no specific ranges)
  const isArchitecturalOnly = selectedRanges.length === 0;
  
  if (!isArchitecturalOnly) {
    // Collect all required nodes (selected + their ancestry)
    selectedNodes.forEach(node => {
      collectAncestryPath(node, requiredNodes);
    });
  }

  // Always include architectural context (actors, arch, exposure, domains)
  
  const architecturalNodes = findArchitecturalNodes(ast);
  
  architecturalNodes.forEach(node => {
    const nodeType = node.constructor.name;
    
    if (isArchitecturalOnly) {
      // For architectural-only extraction, include all architectural blocks
      if (nodeType === 'ArchContext' || 
          nodeType === 'Actors_defContext' || 
          nodeType === 'Actor_defContext' ||
          nodeType === 'ExposureContext' ||
          nodeType === 'Domain_defContext' ||
          nodeType === 'Domains_defContext') {
        collectAncestryPath(node, requiredNodes);
      }
    } else {
      // For normal extraction, always include all architectural context
      if (nodeType === 'ArchContext' || 
          nodeType === 'Actors_defContext' || 
          nodeType === 'Actor_defContext' ||
          nodeType === 'ExposureContext' ||
          nodeType === 'Domain_defContext' ||
          nodeType === 'Domains_defContext') {
        collectAncestryPath(node, requiredNodes);
      }
    }
  });
  
  // Generate the minimal DSL text
  return generateMinimalDSL(ast, requiredNodes, lines);
}

/**
 * Find all AST nodes that fall within the selected ranges
 */
function findNodesInRanges(node: ParserRuleContext, selectedRanges: BlockRange[]): ParserRuleContext[] {
  const selectedNodes: ParserRuleContext[] = [];
  
  function traverse(currentNode: ParseTree): void {
    if (!currentNode) return;
    
    if (currentNode instanceof ParserRuleContext && currentNode.start && currentNode.stop) {
      const nodeStartLine = currentNode.start.line;
      const nodeEndLine = currentNode.stop.line;
      
      let isNodeSelected = false;
      for (const range of selectedRanges) {
        if (nodeStartLine >= range.startLine && nodeEndLine <= range.endLine) {
          selectedNodes.push(currentNode);
          isNodeSelected = true;
          break;
        }
      }
      
      // Even if this node isn't selected, check if any of its children fall within ranges
      // This is important for scenarios within use_cases
      if (!isNodeSelected) {
        for (let i = 0; i < currentNode.getChildCount(); i++) {
          traverse(currentNode.getChild(i)!);
        }
      }
      // If node is selected, we still need to traverse children to find nested selections
      else {
        for (let i = 0; i < currentNode.getChildCount(); i++) {
          const child = currentNode.getChild(i)!;
          if (child instanceof ParserRuleContext && child.start && child.stop) {
            const childStartLine = child.start.line;
            const childEndLine = child.stop.line;
            
            // Check if child has a more specific selection
            for (const range of selectedRanges) {
              if (childStartLine >= range.startLine && childEndLine <= range.endLine && 
                  (childStartLine > nodeStartLine || childEndLine < nodeEndLine)) {
                traverse(child);
                break;
              }
            }
          }
        }
      }
    } else {
      // For non-ParserRuleContext nodes, continue traversing
      if (currentNode.getChildCount() > 0) {
        for (let i = 0; i < currentNode.getChildCount(); i++) {
          traverse(currentNode.getChild(i)!);
        }
      }
    }
  }
  
  traverse(node);
  return selectedNodes;
}

/**
 * Collect ancestry path from selected node to root and also include all descendants
 */
function collectAncestryPath(node: ParserRuleContext, requiredNodes: Set<ParserRuleContext>): void {
  // Add the node itself
  requiredNodes.add(node);
  
  // Add all descendants of the selected node
  function addAllDescendants(currentNode: ParseTree): void {
    if (currentNode instanceof ParserRuleContext) {
      requiredNodes.add(currentNode);
    }
    
    for (let i = 0; i < currentNode.getChildCount(); i++) {
      addAllDescendants(currentNode.getChild(i)!);
    }
  }
  
  addAllDescendants(node);
  
  // Add minimal ancestry path to root (only structural parents, not siblings)
  let current: ParserRuleContext | undefined = node.parent instanceof ParserRuleContext ? node.parent : undefined;
  while (current) {
    requiredNodes.add(current);
    current = current.parent instanceof ParserRuleContext ? current.parent : undefined;
  }
}

/**
 * Find architectural nodes (arch, domain definitions, exposure) that should always be included
 */
function findArchitecturalNodes(rootNode: ParserRuleContext): ParserRuleContext[] {
  const architecturalNodes: ParserRuleContext[] = [];
  
  function traverse(node: ParseTree): void {
    if (!node) return;
    
    if (node instanceof ParserRuleContext) {
      const nodeType = node.constructor.name;
      
      // Include these node types automatically
      if (nodeType === 'ArchContext' || 
          nodeType === 'Domain_defContext' || 
          nodeType === 'Domains_defContext' || 
          nodeType === 'ExposureContext' ||
          nodeType === 'Actor_defContext' ||
          nodeType === 'Actors_defContext') {
        architecturalNodes.push(node);
      }
    }
    
    // Continue traversing children
    for (let i = 0; i < node.getChildCount(); i++) {
      traverse(node.getChild(i)!);
    }
  }
  
  traverse(rootNode);
  return architecturalNodes;
}

/**
 * Generate minimal DSL text from required nodes
 */
function generateMinimalDSL(
  rootNode: ParserRuleContext, 
  requiredNodes: Set<ParserRuleContext>, 
  lines: string[]
): string {
  function shouldIncludeNode(node: ParserRuleContext): boolean {
    return requiredNodes.has(node);
  }
  
  function hasRequiredDescendant(node: ParseTree): boolean {
    if (node instanceof ParserRuleContext && requiredNodes.has(node)) return true;
    
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (hasRequiredDescendant(child)) return true;
    }
    return false;
  }
  
  function generateNode(node: ParseTree, depth: number = 0): string {
    if (!node) return '';
    
    const nodeType = node.constructor.name;
    
    // Handle different node types based on your grammar
    switch (nodeType) {
      case 'DslContext':
        return generateDsl(node, depth);
      case 'Services_defContext':
        return generateServices(node, depth);
      case 'Use_caseContext':
        return generateUseCase(node, depth);
      case 'Service_definitionContext':
        return generateServiceDefinition(node, depth);
      case 'Service_blockContext':
        return generateServiceBlock(node, depth);
      case 'ScenarioContext':
        return generateScenario(node, depth);
      case 'ArchContext':
        return generateArch(node, depth);
      case 'Domain_defContext':
      case 'Domains_defContext':
        return generateDomainDefs(node, depth);
      case 'ExposureContext':
        return generateExposure(node, depth);
      case 'Actor_defContext':
        return generateActorDef(node, depth);
      case 'Actors_defContext':
        return generateActorDefs(node, depth);
      default:
        // Ensure comments and other generic nodes end with newline
        const text = generateGenericNode(node, depth);
        return text.endsWith('\n') ? text : text + '\n';
    }
  }
  
  function generateDsl(node: ParseTree, depth: number): string {
    let result = '';
    let lastWasBlock = false;
    
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child instanceof ParserRuleContext && (shouldIncludeNode(child) || hasRequiredDescendant(child))) {
        const childText = generateNode(child, depth);
        if (childText.trim()) {
          // Add spacing between blocks to prevent comment/use_case merging
          if (lastWasBlock && !childText.startsWith('\n')) {
            result += '\n';
          }
          result += childText;
          lastWasBlock = true;
        }
      }
    }
    
    return result;
  }
  
  function generateServices(node: ParseTree, depth: number): string {
    if (!hasRequiredDescendant(node)) return '';
    
    let result = 'services {\n';
    
    // Find service_block_list (not service_definition_list)
    let serviceList: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_block_listContext') {
        serviceList = child;
        break;
      }
    }
    
    if (serviceList && hasRequiredDescendant(serviceList)) {
      result += generateServiceBlockList(serviceList, depth + 1);
    }
    
    result += '\n}\n\n';
    return result;
  }
  
  function generateServiceBlockList(node: ParseTree, depth: number): string {
    let result = '';
    const serviceBlocks: ParseTree[] = [];
    
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_blockContext') {
        serviceBlocks.push(child);
      }
    }
    
    const requiredServices = serviceBlocks.filter(service => 
      service instanceof ParserRuleContext && shouldIncludeNode(service)
    );
    
    requiredServices.forEach((service, index) => {
      result += generateServiceBlock(service, depth);
      if (index < requiredServices.length - 1) {
        result += ',\n';
      }
    });
    
    return result;
  }

  function generateServiceDefinitionList(node: ParseTree, depth: number): string {
    let result = '';
    const serviceDefinitions: ParseTree[] = [];
    
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_definitionContext') {
        serviceDefinitions.push(child);
      }
    }
    
    const requiredServices = serviceDefinitions.filter(service => 
      service instanceof ParserRuleContext && shouldIncludeNode(service)
    );
    
    requiredServices.forEach((service, index) => {
      result += generateServiceDefinition(service, depth);
      if (index < requiredServices.length - 1) {
        result += ',\n';
      }
    });
    
    return result;
  }
  
  function generateServiceBlock(node: ParseTree, depth: number): string {
    const indent = '  '.repeat(depth);
    
    // Get service name
    let nameNode: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_nameContext') {
        nameNode = child;
        break;
      }
    }
    const serviceName = nameNode ? getNodeText(nameNode as ParserRuleContext, lines) : '';
    
    let result = `${indent}${serviceName} {\n`;
    
    // Get properties
    let propertiesNode: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_propertiesContext') {
        propertiesNode = child;
        break;
      }
    }
    
    if (propertiesNode) {
      result += generateServiceProperties(propertiesNode, depth + 1);
    }
    
    result += `${indent}}`;
    return result;
  }

  function generateServiceDefinition(node: ParseTree, depth: number): string {
    const indent = '  '.repeat(depth);
    
    // Get service name
    let nameNode: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_nameContext') {
        nameNode = child;
        break;
      }
    }
    const serviceName = nameNode ? getNodeText(nameNode as ParserRuleContext, lines) : '';
    
    let result = `${indent}${serviceName}: {\n`;
    
    // Get properties
    let propertiesNode: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_propertiesContext') {
        propertiesNode = child;
        break;
      }
    }
    
    if (propertiesNode) {
      result += generateServiceProperties(propertiesNode, depth + 1);
    }
    
    result += `${indent}}`;
    return result;
  }
  
  function generateServiceProperties(node: ParseTree, depth: number): string {
    const indent = '  '.repeat(depth);
    let result = '';
    
    const properties: ParseTree[] = [];
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_propertyContext') {
        properties.push(child);
      }
    }
    
    properties.forEach((prop, index) => {
      const propText = getNodeText(prop as ParserRuleContext, lines);
      result += `${indent}${propText}`;
      if (index < properties.length - 1) {
        result += '\n';
      }
    });
    
    result += '\n';
    return result;
  }
  
  function generateUseCase(node: ParseTree, depth: number): string {
    if (!hasRequiredDescendant(node)) return '';
    
    // Get use case name
    let stringNode: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'StringContext') {
        stringNode = child;
        break;
      }
    }
    const useCaseName = stringNode ? getNodeText(stringNode as ParserRuleContext, lines) : '';
    
    let result = `use_case ${useCaseName} {\n`;
    
    // Get scenarios
    const scenarios: ParseTree[] = [];
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'ScenarioContext') {
        scenarios.push(child);
      }
    }
    
    const requiredScenarios = scenarios.filter(scenario => 
      scenario instanceof ParserRuleContext && (shouldIncludeNode(scenario) || hasRequiredDescendant(scenario))
    );
    
    requiredScenarios.forEach(scenario => {
      result += generateScenario(scenario, depth + 1);
    });
    
    result += '}\n';
    return result;
  }
  
  function generateScenario(node: ParseTree, depth: number): string {
    const indent = '  '.repeat(depth);
    
    // Get trigger
    let trigger: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'TriggerContext') {
        trigger = child;
        break;
      }
    }
    
    // Get action block
    let actionBlock: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Action_blockContext') {
        actionBlock = child;
        break;
      }
    }
    
    let result = '';
    if (trigger) {
      result += `${indent}${getNodeText(trigger as ParserRuleContext, lines)}\n`;
    }
    
    if (actionBlock) {
      for (let i = 0; i < actionBlock.getChildCount(); i++) {
        const action = actionBlock.getChild(i)!;
        if (action.constructor.name.includes('Action')) {
          result += `${indent}  ${getNodeText(action as ParserRuleContext, lines)}\n`;
        }
      }
    }
    
    result += '\n';
    return result;
  }

  function generateArch(node: ParseTree, depth: number): string {
    // For architecture blocks, include the entire block as-is
    const archText = getNodeText(node as ParserRuleContext, lines);
    return archText + '\n\n';
  }

  function generateDomainDefs(node: ParseTree, depth: number): string {
    // For domain definitions, include the entire block as-is
    const domainText = getNodeText(node as ParserRuleContext, lines);
    return domainText + '\n\n';
  }

  function generateExposure(node: ParseTree, depth: number): string {
    // For exposure blocks, include the entire block as-is
    const exposureText = getNodeText(node as ParserRuleContext, lines);
    return exposureText + '\n\n';
  }

  function generateActorDef(node: ParseTree, depth: number): string {
    // For single actor definition, include as-is
    const actorText = getNodeText(node as ParserRuleContext, lines);
    return actorText + '\n\n';
  }

  function generateActorDefs(node: ParseTree, depth: number): string {
    // For actor definitions block, include the entire block as-is
    const actorText = getNodeText(node as ParserRuleContext, lines);
    return actorText + '\n\n';
  }
  
  function generateGenericNode(node: ParseTree, depth: number): string {
    return getNodeText(node as ParserRuleContext, lines);
  }
  
  function getNodeText(node: ParserRuleContext, lines: string[]): string {
    if (!node || !node.start || !node.stop) return '';
    
    const startLine = node.start.line - 1;
    const startCol = node.start.column;
    const endLine = node.stop.line - 1;
    // Fix: Use the stop token's end position correctly
    const endCol = node.stop.column + (node.stop.text?.length || 0);
    
    if (startLine === endLine) {
      return lines[startLine].substring(startCol, endCol);
    }
    
    let result = lines[startLine].substring(startCol);
    for (let i = startLine + 1; i < endLine; i++) {
      result += '\n' + lines[i];
    }
    if (endLine < lines.length) {
      result += '\n' + lines[endLine].substring(0, endCol);
    }
    
    return result;
  }
  
  return generateNode(rootNode);
}



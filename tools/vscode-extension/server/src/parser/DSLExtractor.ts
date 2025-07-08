import { ParseTree, ParserRuleContext, TerminalNode, Token } from 'antlr4ng';
import { BlockRange } from '../../../shared/lib/types/domain-extraction';

/**
 * Extracts minimal subtree from ArchDSL AST with ancestry
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
  
  // Collect all required nodes (selected + their ancestry)
  selectedNodes.forEach(node => {
    collectAncestryPath(node, requiredNodes);
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
  
  // Add ancestry path to root
  let current: ParserRuleContext | undefined = node;
  while (current) {
    requiredNodes.add(current);
    // Use the parent property from ParserRuleContext
    current = current.parent instanceof ParserRuleContext ? current.parent : undefined;
  }
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
      case 'ServicesContext':
        return generateServices(node, depth);
      case 'Use_caseContext':
        return generateUseCase(node, depth);
      case 'Service_definitionContext':
        return generateServiceDefinition(node, depth);
      case 'ScenarioContext':
        return generateScenario(node, depth);
      default:
        return generateGenericNode(node, depth);
    }
  }
  
  function generateDsl(node: ParseTree, depth: number): string {
    let result = '';
    
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child instanceof ParserRuleContext && (shouldIncludeNode(child) || hasRequiredDescendant(child))) {
        const childText = generateNode(child, depth);
        if (childText.trim()) {
          result += childText;
        }
      }
    }
    
    return result;
  }
  
  function generateServices(node: ParseTree, depth: number): string {
    if (!hasRequiredDescendant(node)) return '';
    
    let result = 'services {\n';
    
    // Find service_definition_list
    let serviceList: ParseTree | null = null;
    for (let i = 0; i < node.getChildCount(); i++) {
      const child = node.getChild(i)!;
      if (child.constructor.name === 'Service_definition_listContext') {
        serviceList = child;
        break;
      }
    }
    
    if (serviceList && hasRequiredDescendant(serviceList)) {
      result += generateServiceDefinitionList(serviceList, depth + 1);
    }
    
    result += '\n}\n\n';
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
      service instanceof ParserRuleContext && (shouldIncludeNode(service) || hasRequiredDescendant(service))
    );
    
    requiredServices.forEach((service, index) => {
      result += generateServiceDefinition(service, depth);
      if (index < requiredServices.length - 1) {
        result += ',\n';
      }
    });
    
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
  
  function generateGenericNode(node: ParseTree, depth: number): string {
    return getNodeText(node as ParserRuleContext, lines);
  }
  
  function getNodeText(node: ParserRuleContext, lines: string[]): string {
    if (!node || !node.start || !node.stop) return '';
    
    const startLine = node.start.line - 1;
    const startCol = node.start.column;
    const endLine = node.stop.line - 1;
    const endCol = node.stop.column + (node.stop.text?.length || 1);
    
    if (startLine === endLine) {
      return lines[startLine].substring(startCol, endCol);
    }
    
    let result = lines[startLine].substring(startCol);
    for (let i = startLine + 1; i < endLine; i++) {
      result += '\n' + lines[i];
    }
    result += '\n' + lines[endLine].substring(0, endCol);
    
    return result;
  }
  
  return generateNode(rootNode);
}


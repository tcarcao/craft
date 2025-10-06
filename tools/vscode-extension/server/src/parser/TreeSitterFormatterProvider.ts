import { TextDocument } from 'vscode-languageserver-textdocument';
import { TextEdit, Range, Position } from 'vscode-languageserver/node.js';

// Import native tree-sitter with tree-sitter-craft npm package
import Parser from 'tree-sitter';
import Craft from 'tree-sitter-craft';
import { Logger } from '../utils/Logger.js';

/**
 * Tree-sitter based formatter for Craft DSL
 * Uses the native Node.js parser for optimal performance
 */
export class TreeSitterFormatterProvider {
  private parser: any = null;
  private initializationPromise: Promise<void>;

  constructor() {
    this.initializationPromise = this.initializeParser();
  }

  private async initializeParser(): Promise<void> {
    try {
      // Use native Node.js tree-sitter with tree-sitter-craft npm package
      this.parser = new Parser();
      this.parser.setLanguage(Craft);
      
      Logger.info('✅ TreeSitterFormatterProvider Native tree-sitter Craft formatter ready');
      Logger.info('✅ TreeSitterFormatterProvider Using native Node.js performance instead of WASM');
      
    } catch (error) {
      Logger.error('Failed to initialize native Tree-sitter parser:', error);
      Logger.warn('Tree-sitter formatter will be disabled');
    }
  }

  async formatDocument(document: TextDocument): Promise<TextEdit[]> {
    // Wait for initialization to complete
    await this.initializationPromise;
    
    if (!this.parser) {
      Logger.warn('Tree-sitter parser not initialized - formatting disabled');
      return [];
    }
    
    const text = document.getText();
    const formatted = this.formatCraftContent(text);
    
    if (formatted === text) {
      return []; // No changes needed
    }
    
    // Return a single edit that replaces the entire document
    return [{
      range: Range.create(
        Position.create(0, 0),
        document.positionAt(text.length)
      ),
      newText: formatted
    }];
  }

  private formatCraftContent(content: string): string {
    const tree = this.parser.parse(content);
    const rootNode = tree.rootNode;
    
    return this.formatNode(rootNode, content, 0);
  }

  private formatNode(node: any, sourceText: string, indentLevel: number): string {
    const indentSize = 4;
    
    if (node.type === 'source_file') {
      // Format top-level items with spacing between them
      const children = node.children;
      const formattedChildren: string[] = [];
      
      for (let i = 0; i < children.length; i++) {
        const child = children[i];
        const formattedChild = this.formatNode(child, sourceText, 0);
        if (formattedChild.trim()) {
          formattedChildren.push(formattedChild);
        }
      }
      
      return formattedChildren.join('\n\n');
    }
    
    if (node.type === 'services_block') {
      return this.formatServicesBlock(node, sourceText, indentLevel);
    }
    
    if (node.type === 'service_definition') {
      return this.formatServiceDefinition(node, sourceText, indentLevel);
    }
    
    if (node.type === 'arch_block') {
      return this.formatArchBlock(node, sourceText, indentLevel);
    }
    
    if (node.type === 'use_case_block') {
      return this.formatUseCaseBlock(node, sourceText, indentLevel);
    }
    
    if (node.type === 'domain_block') {
      return this.formatDomainBlock(node, sourceText, indentLevel);
    }
    
    if (node.type === 'exposure_block') {
      return this.formatExposureBlock(node, sourceText, indentLevel);
    }
    
    // Default formatting - just extract the text
    return node.text || sourceText.slice(node.startIndex, node.endIndex);
  }

  private formatServicesBlock(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    const childIndent = ' '.repeat((indentLevel + 1) * 4);
    
    const result: string[] = [];
    result.push(`${indent}services {`);
    
    // Format each service definition
    for (const child of node.children) {
      if (child.type === 'service_definition') {
        const formattedService = this.formatServiceDefinition(child, sourceText, indentLevel + 1);
        result.push(formattedService);
      }
    }
    
    result.push(`${indent}}`);
    return result.join('\n');
  }

  private formatServiceDefinition(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    
    const result: string[] = [];
    
    // Get service name - look for identifier node
    let serviceName = 'Service';
    for (const child of node.children) {
      if (child.type === 'identifier') {
        serviceName = child.text || sourceText.slice(child.startIndex, child.endIndex);
        break;
      }
    }
    result.push(`${indent}${serviceName} {`);
    
    // Format service properties
    for (let i = 1; i < node.children.length; i++) {
      const child = node.children[i];
      if (child.type === 'service_property') {
        const formattedProperty = this.formatServiceProperty(child, sourceText, indentLevel + 1);
        result.push(formattedProperty);
      }
    }
    
    result.push(`${indent}}`);
    return result.join('\n');
  }

  private formatServiceProperty(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    
    // Extract the property text and clean it up
    const propertyText = node.text || sourceText.slice(node.startIndex, node.endIndex);
    
    // Simple cleanup - ensure proper spacing around colons and commas
    const cleaned = propertyText
      .replace(/\s*:\s*/g, ': ')
      .replace(/\s*,\s*/g, ', ')
      .trim();
    
    return `${indent}${cleaned}`;
  }

  private formatArchBlock(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    const result: string[] = [];
    
    result.push(`${indent}arch {`);
    
    // Format arch sections - look for arch_section nodes that contain presentation_section or gateway_section
    const archSections: string[] = [];
    for (const child of node.children) {
      if (child.type === 'arch_section') {
        // arch_section contains presentation_section or gateway_section
        for (const sectionChild of child.children) {
          if (sectionChild.type === 'presentation_section' || sectionChild.type === 'gateway_section') {
            const formattedSection = this.formatArchSection(sectionChild, sourceText, indentLevel + 1);
            archSections.push(formattedSection);
          }
        }
      }
    }
    
    // Add arch sections with empty lines between them
    for (let i = 0; i < archSections.length; i++) {
      result.push(archSections[i]);
      // Add empty line after each arch section except the last one
      if (i < archSections.length - 1) {
        result.push('');
      }
    }
    
    result.push(`${indent}}`);
    return result.join('\n');
  }

  private formatArchSection(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    const componentIndent = ' '.repeat((indentLevel + 1) * 4);
    
    const result: string[] = [];
    
    // Get section name
    const sectionName = node.type === 'presentation_section' ? 'presentation' : 'gateway';
    result.push(`${indent}${sectionName}:`);
    
    // Format arch components
    for (const child of node.children) {
      if (child.type === 'arch_component') {
        const componentText = child.text || sourceText.slice(child.startIndex, child.endIndex);
        result.push(`${componentIndent}${componentText.trim()}`);
      }
    }
    
    return result.join('\n');
  }

  private formatUseCaseBlock(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    const result: string[] = [];
    
    // Get use case name (should be a string)
    const useCaseName = node.children.find((child: any) => child.type === 'string')?.text || '"Use Case"';
    result.push(`${indent}use_case ${useCaseName} {`);
    
    // Format scenarios - look for scenario nodes, then format their when_clause children
    const whenClauses: string[] = [];
    for (const child of node.children) {
      if (child.type === 'scenario') {
        // Scenario contains when_clause nodes
        for (const scenarioChild of child.children) {
          if (scenarioChild.type === 'when_clause') {
            const formattedWhenClause = this.formatWhenClause(scenarioChild, sourceText, indentLevel + 1);
            whenClauses.push(formattedWhenClause);
          }
        }
      }
    }
    
    // Add when clauses with empty lines between them
    for (let i = 0; i < whenClauses.length; i++) {
      result.push(whenClauses[i]);
      // Add empty line after each when clause except the last one
      if (i < whenClauses.length - 1) {
        result.push('');
      }
    }
    
    result.push(`${indent}}`);
    return result.join('\n');
  }

  private formatWhenClause(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    const actionIndent = ' '.repeat((indentLevel + 1) * 4);
    
    const result: string[] = [];
    
    // Build the when line from when + external_trigger
    let whenLine = 'when';
    
    for (const child of node.children) {
      if (child.type === 'external_trigger' || child.type === 'event_trigger' || 
          child.type === 'domain_listener' || child.type === 'cron_trigger') {
        const triggerText = child.text || sourceText.slice(child.startIndex, child.endIndex);
        whenLine += ` ${triggerText.trim()}`;
        break;
      }
    }
    
    // For external triggers, we need to handle the case where the trigger line continues
    // Look for actions and see if the first one starts with text that should be part of the trigger
    const actions: string[] = [];
    for (const child of node.children) {
      if (child.type === 'action') {
        const actionText = child.text || sourceText.slice(child.startIndex, child.endIndex);
        const lines = actionText.split('\n').map((line: string) => line.trim()).filter((line: string) => line);
        
        // If first action starts with a continuation of the trigger, add it to when line
        if (actions.length === 0 && lines.length > 0 && !lines[0].includes(' ')) {
          whenLine += ` ${lines[0]}`;
          // Add remaining lines as separate actions
          lines.slice(1).forEach((line: string) => actions.push(line));
        } else {
          lines.forEach((line: string) => actions.push(line));
        }
      }
    }
    
    result.push(`${indent}${whenLine}`);
    
    // Add all actions
    actions.forEach(action => {
      if (action.trim()) {
        result.push(`${actionIndent}${action}`);
      }
    });
    
    return result.join('\n');
  }


  private formatDomainBlock(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    const subdomainIndent = ' '.repeat((indentLevel + 1) * 4);
    
    const result: string[] = [];
    
    // Get domain name - look for identifier after "domain" keyword
    let domainName = 'Domain';
    for (const child of node.children) {
      if (child.type === 'identifier') {
        domainName = child.text || sourceText.slice(child.startIndex, child.endIndex);
        break;
      }
    }
    
    result.push(`${indent}domain ${domainName} {`);
    
    // Format subdomains
    for (let i = 1; i < node.children.length; i++) {
      const child = node.children[i];
      if (child.type === 'subdomain') {
        const subdomainText = child.text || sourceText.slice(child.startIndex, child.endIndex);
        result.push(`${subdomainIndent}${subdomainText.trim()}`);
      }
    }
    
    result.push(`${indent}}`);
    return result.join('\n');
  }

  private formatExposureBlock(node: any, sourceText: string, indentLevel: number): string {
    const indent = ' '.repeat(indentLevel * 4);
    const propertyIndent = ' '.repeat((indentLevel + 1) * 4);
    
    const result: string[] = [];
    
    // Get exposure name - look for identifier after "exposure" keyword
    let exposureName = 'default';
    for (const child of node.children) {
      if (child.type === 'identifier') {
        exposureName = child.text || sourceText.slice(child.startIndex, child.endIndex);
        break;
      }
    }
    result.push(`${indent}exposure ${exposureName} {`);
    
    // Format exposure properties
    for (let i = 1; i < node.children.length; i++) {
      const child = node.children[i];
      if (child.type === 'exposure_property') {
        const propertyText = child.text || sourceText.slice(child.startIndex, child.endIndex);
        result.push(`${propertyIndent}${propertyText.trim()}`);
      }
    }
    
    result.push(`${indent}}`);
    return result.join('\n');
  }

}
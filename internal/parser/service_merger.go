package parser

import (
	"slices"
)

// ServiceMerger handles merging of services with the same name from multiple sources
type ServiceMerger struct {
	services map[string]*Service
}

// NewServiceMerger creates a new service merger
func NewServiceMerger() *ServiceMerger {
	return &ServiceMerger{
		services: make(map[string]*Service),
	}
}

// AddService adds a service to the merger, merging with existing service if name matches
func (sm *ServiceMerger) AddService(service Service) {
	if existing, exists := sm.services[service.Name]; exists {
		sm.mergeService(existing, service)
	} else {
		// Create a copy to avoid modifying original
		merged := service
		merged.Domains = slices.Clone(service.Domains)
		merged.DataStores = slices.Clone(service.DataStores)
		merged.Deployment.Rules = slices.Clone(service.Deployment.Rules)
		sm.services[service.Name] = &merged
	}
}

// GetMergedServices returns all merged services as a slice
func (sm *ServiceMerger) GetMergedServices() []Service {
	result := make([]Service, 0, len(sm.services))
	for _, service := range sm.services {
		result = append(result, *service)
	}
	return result
}

// mergeService merges a new service into an existing one
func (sm *ServiceMerger) mergeService(existing *Service, new Service) {
	// Merge domains (deduplicate)
	existing.Domains = mergeStringSlices(existing.Domains, new.Domains)
	
	// Merge data stores (deduplicate)
	existing.DataStores = mergeStringSlices(existing.DataStores, new.DataStores)
	
	// Handle language: prefer non-empty, warn on conflicts
	if existing.Language == "" {
		existing.Language = new.Language
	} else if new.Language != "" && existing.Language != new.Language {
		// TODO: Add logging for language conflicts
		// For now, keep the existing language
	}
	
	// Merge deployment strategies
	sm.mergeDeploymentStrategy(&existing.Deployment, new.Deployment)
}

// mergeStringSlices merges two string slices, removing duplicates
func mergeStringSlices(slice1, slice2 []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice1)+len(slice2))
	
	// Add all items from first slice
	for _, item := range slice1 {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	// Add new items from second slice
	for _, item := range slice2 {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// mergeDeploymentStrategy merges deployment strategies
func (sm *ServiceMerger) mergeDeploymentStrategy(existing *DeploymentStrategy, new DeploymentStrategy) {
	// If existing has no deployment strategy, use the new one
	if existing.Type == "" {
		existing.Type = new.Type
		existing.Rules = slices.Clone(new.Rules)
		return
	}
	
	// If new has no deployment strategy, keep existing
	if new.Type == "" {
		return
	}
	
	// If strategies have different types, keep existing (could log warning)
	if existing.Type != new.Type {
		// TODO: Add logging for deployment strategy conflicts
		return
	}
	
	// Same deployment type, merge rules (deduplicate)
	existing.Rules = mergeDeploymentRules(existing.Rules, new.Rules)
}

// mergeDeploymentRules merges deployment rules, avoiding duplicates
func mergeDeploymentRules(rules1, rules2 []DeploymentRule) []DeploymentRule {
	seen := make(map[string]bool) // Use "percentage:target" as key
	result := make([]DeploymentRule, 0, len(rules1)+len(rules2))
	
	// Add all rules from first slice
	for _, rule := range rules1 {
		key := rule.Percentage + ":" + rule.Target
		if !seen[key] {
			seen[key] = true
			result = append(result, rule)
		}
	}
	
	// Add new rules from second slice
	for _, rule := range rules2 {
		key := rule.Percentage + ":" + rule.Target
		if !seen[key] {
			seen[key] = true
			result = append(result, rule)
		}
	}
	
	return result
}

// MergeServices is a convenience function to merge a slice of services
func MergeServices(services []Service) []Service {
	merger := NewServiceMerger()
	for _, service := range services {
		merger.AddService(service)
	}
	return merger.GetMergedServices()
}
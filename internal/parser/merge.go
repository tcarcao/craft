package parser

// MergeModels combines multiple parsed DSL models into one.
// Services are merged by name; all other entities are appended with name-based deduplication.
func MergeModels(models []*DSLModel) *DSLModel {
	if len(models) == 0 {
		return &DSLModel{}
	}
	if len(models) == 1 {
		return models[0]
	}

	merged := &DSLModel{}
	seenActors := map[string]bool{}
	seenDomains := map[string]int{} // name → index in merged.Domains
	seenUseCases := map[string]bool{}
	seenExposures := map[string]bool{}
	merger := NewServiceMerger()

	for _, m := range models {
		for _, a := range m.Actors {
			if !seenActors[a.Name] {
				seenActors[a.Name] = true
				merged.Actors = append(merged.Actors, a)
			}
		}

		for _, d := range m.Domains {
			if idx, ok := seenDomains[d.Name]; ok {
				merged.Domains[idx].BoundedContexts = mergeStringSlices(merged.Domains[idx].BoundedContexts, d.BoundedContexts)
			} else {
				seenDomains[d.Name] = len(merged.Domains)
				merged.Domains = append(merged.Domains, d)
			}
		}

		for _, s := range m.Services {
			merger.AddService(s)
		}

		for _, uc := range m.UseCases {
			if !seenUseCases[uc.Name] {
				seenUseCases[uc.Name] = true
				merged.UseCases = append(merged.UseCases, uc)
			}
		}

		for _, e := range m.Exposures {
			if !seenExposures[e.Name] {
				seenExposures[e.Name] = true
				merged.Exposures = append(merged.Exposures, e)
			}
		}

		merged.Architectures = append(merged.Architectures, m.Architectures...)
	}

	merged.Services = merger.GetMergedServices()
	return merged
}

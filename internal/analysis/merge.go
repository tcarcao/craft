package analysis

import "github.com/tcarcao/craft/v2/internal/model"

// mergeServices combines the same-name service blocks of ONE file into one
// service, keeping the position of the first. List fields are unioned in order
// of appearance; each scalar field takes the first non-empty value. Across
// files the first declaration wins instead (see mergeDocs).
func mergeServices(services []model.Service) []model.Service {
	var out []model.Service
	index := map[string]int{}
	for _, s := range services {
		i, seen := index[s.Name]
		if !seen {
			index[s.Name] = len(out)
			merged := s
			merged.Contexts = appendUnique(nil, s.Contexts...)
			merged.DataStores = appendUnique(nil, s.DataStores...)
			out = append(out, merged)
			continue
		}
		m := &out[i]
		m.Contexts = appendUnique(m.Contexts, s.Contexts...)
		m.DataStores = appendUnique(m.DataStores, s.DataStores...)
		m.Language = firstNonEmpty(m.Language, s.Language)
		m.CatalogRef = firstNonEmpty(m.CatalogRef, s.CatalogRef)
		m.Repo = firstNonEmpty(m.Repo, s.Repo)
		m.Deployment.Type = firstNonEmpty(m.Deployment.Type, s.Deployment.Type)
		if len(m.Deployment.Rules) == 0 {
			m.Deployment.Rules = s.Deployment.Rules
		}
	}
	return out
}

// firstByName keeps the first element of each name, in order. Within a file a
// repeated domain is a duplicate-name error; across files a repeated service or
// domain is a duplicate-name warning. Either way sema resolves against the first
// declaration, and the model follows it.
func firstByName[T any](items []T, name func(T) string) []T {
	var out []T
	seen := map[string]bool{}
	for _, it := range items {
		if n := name(it); !seen[n] {
			seen[n] = true
			out = append(out, it)
		}
	}
	return out
}

// mergeDocs concatenates the per-file documents (already in sorted file order).
// A service or domain declared in more than one file keeps its first
// declaration.
func mergeDocs(docs []*model.CraftDoc) *model.CraftDoc {
	merged := &model.CraftDoc{UseCases: []model.UseCase{}}
	for _, d := range docs {
		merged.Architectures = append(merged.Architectures, d.Architectures...)
		merged.Exposures = append(merged.Exposures, d.Exposures...)
		merged.Services = append(merged.Services, d.Services...)
		merged.UseCases = append(merged.UseCases, d.UseCases...)
		merged.Domains = append(merged.Domains, d.Domains...)
		merged.Actors = append(merged.Actors, d.Actors...)
		merged.ContextMap = append(merged.ContextMap, d.ContextMap...)
		merged.Glossary = append(merged.Glossary, d.Glossary...)
		merged.Surfaces = append(merged.Surfaces, d.Surfaces...)
	}
	merged.Services = firstByName(merged.Services, func(s model.Service) string { return s.Name })
	merged.Domains = firstByName(merged.Domains, func(d model.Domain) string { return d.Name })
	return merged
}

func appendUnique(dst []string, values ...string) []string {
	for _, v := range values {
		dup := false
		for _, existing := range dst {
			if existing == v {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, v)
		}
	}
	return dst
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

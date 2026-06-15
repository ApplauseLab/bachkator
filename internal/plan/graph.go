package plan

import "sort"

func BuildSelection(documents []Document) Selection {
	selection := Selection{Documents: append([]Document(nil), documents...)}
	diagnostics := []Diagnostic{}
	byID := map[string]Document{}
	for _, doc := range selection.Documents {
		if existing, ok := byID[doc.ID]; ok {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: "error",
				File:     doc.Path,
				Code:     "duplicate-plan-id",
				Message:  "duplicate Plan ID " + doc.ID + " also used by " + existing.Path,
			})
			continue
		}
		byID[doc.ID] = doc
	}
	for _, doc := range selection.Documents {
		for _, dep := range doc.DependsOn {
			if _, ok := byID[dep]; !ok {
				diagnostics = append(diagnostics, Diagnostic{
					Severity: "error",
					File:     doc.Path,
					Code:     "missing-plan-dependency",
					Message:  "Plan dependency " + dep + " is not in the selected Plan set",
				})
			}
		}
	}
	waves, cycleDiagnostics := waves(selection.Documents, byID)
	diagnostics = append(diagnostics, cycleDiagnostics...)
	selection.Waves = waves
	selection.Diagnostics = diagnostics
	return selection
}

func waves(documents []Document, byID map[string]Document) ([][]string, []Diagnostic) {
	remainingDeps := map[string]map[string]bool{}
	dependents := map[string][]string{}
	for _, doc := range documents {
		deps := map[string]bool{}
		for _, dep := range doc.DependsOn {
			if _, ok := byID[dep]; !ok {
				continue
			}
			deps[dep] = true
			dependents[dep] = append(dependents[dep], doc.ID)
		}
		remainingDeps[doc.ID] = deps
	}
	ready := []string{}
	for id, deps := range remainingDeps {
		if len(deps) == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	out := [][]string{}
	processed := map[string]bool{}
	for len(ready) > 0 {
		wave := append([]string(nil), ready...)
		out = append(out, wave)
		next := []string{}
		for _, id := range wave {
			processed[id] = true
			for _, dependent := range dependents[id] {
				delete(remainingDeps[dependent], id)
				if len(remainingDeps[dependent]) == 0 && !processed[dependent] {
					next = append(next, dependent)
				}
			}
		}
		sort.Strings(next)
		ready = next
	}
	if len(processed) == len(remainingDeps) {
		return out, nil
	}
	diagnostics := []Diagnostic{}
	ids := make([]string, 0, len(remainingDeps))
	for id := range remainingDeps {
		if !processed[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		doc := byID[id]
		diagnostics = append(diagnostics, Diagnostic{
			Severity: "error",
			File:     doc.Path,
			Code:     "plan-dependency-cycle",
			Message:  "Plan dependency cycle includes " + id,
		})
	}
	return out, diagnostics
}

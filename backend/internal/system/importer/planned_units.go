// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import "context"

// plannedUnits are the organization units a dry run would create, by id and by handle, and the projects it would
// create, by id. A dry run writes nothing, so a resource that the same import places in one of them would otherwise
// be checked against a unit or project that is not there yet, though the real import creates it before the resource.
type plannedUnits struct {
	byID     map[string]bool
	byHandle map[string]string
	projects map[string]bool
}

type plannedUnitsKey struct{}

// newPlannedUnits returns an empty set of planned units and projects.
func newPlannedUnits() *plannedUnits {
	return &plannedUnits{byID: map[string]bool{}, byHandle: map[string]string{}, projects: map[string]bool{}}
}

// withPlannedUnits returns a context that carries the units a dry run would create.
func withPlannedUnits(ctx context.Context, units *plannedUnits) context.Context {
	return context.WithValue(ctx, plannedUnitsKey{}, units)
}

// plannedUnitsOf returns the units a dry run would create, or nil outside a dry run.
func plannedUnitsOf(ctx context.Context) *plannedUnits {
	units, _ := ctx.Value(plannedUnitsKey{}).(*plannedUnits)
	return units
}

// add records a unit the dry run would create.
func (p *plannedUnits) add(id, handle string) {
	if id != "" {
		p.byID[id] = true
	}
	if handle != "" {
		p.byHandle[handle] = id
	}
}

// record adds a unit or project the dry run would create, as its document names it; anything else it leaves out.
func (p *plannedUnits) record(doc parsedDocument, outcome ImportItemOutcome) {
	if p == nil || outcome.Status != statusSuccess {
		return
	}
	if doc.ResourceType == resourceTypeProject {
		if outcome.ResourceID != "" {
			p.projects[outcome.ResourceID] = true
		}
		return
	}
	if doc.ResourceType != resourceTypeOrganizationUnit {
		return
	}
	var unit struct {
		ID     string `yaml:"id"`
		Handle string `yaml:"handle"`
	}
	if err := doc.Node.Decode(&unit); err == nil {
		p.add(unit.ID, unit.Handle)
	}
}

// resolve returns the id of the planned unit a resource names by id, or by handle when it names no id.
func (p *plannedUnits) resolve(id, handle string) (string, bool) {
	if p == nil {
		return "", false
	}
	if id != "" {
		return id, p.byID[id]
	}
	planned, ok := p.byHandle[handle]
	return planned, ok && handle != ""
}

// hasProject reports whether the dry run would create the project.
func (p *plannedUnits) hasProject(id string) bool {
	return p != nil && id != "" && p.projects[id]
}

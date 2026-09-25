// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// RequestFromDeclaration converts a policy a resource file declares into the framework's request
// shape.
//
// The declarative and framework shapes are separate types rather than one shared struct: the
// declarative types are a provider contract, and coupling them to this package's internals would
// make every change to one a change to the other. The conversion lives here, once, because every
// shareable resource type declares its policies with the same provider types and two copies of this
// would be two things free to disagree.
func RequestFromDeclaration(p providers.SharingPolicy) PolicyRequest {
	entries := make([]TargetEntry, 0, len(p.TargetOuScope.OUIDs))
	for _, e := range p.TargetOuScope.OUIDs {
		entries = append(entries, TargetEntry{OUID: e.OUID, AllChildren: e.AllChildren})
	}

	rules := make(map[string]OverlayRule, len(p.OverlayRules))
	for key, r := range p.OverlayRules {
		rules[key] = OverlayRule{
			Editable:       r.Editable,
			Value:          r.Value,
			AllowedValues:  r.AllowedValues,
			ExcludedValues: r.ExcludedValues,
		}
	}
	if len(rules) == 0 {
		rules = nil
	}

	return PolicyRequest{
		InitiatingOUID: p.InitiatingOuID,
		TargetOUScope: TargetOUScope{
			AllOUs:            p.TargetOuScope.AllOUs,
			AllRoots:          p.TargetOuScope.AllRoots,
			RootOUIDs:         p.TargetOuScope.RootOUIDs,
			ExcludedRootOUIDs: p.TargetOuScope.ExcludedRootOUIDs,
			AllChildren:       p.TargetOuScope.AllChildren,
			OUIDs:             entries,
			ExcludedOUIDs:     p.TargetOuScope.ExcludedOUIDs,
		},
		OverlayRules: rules,
	}
}

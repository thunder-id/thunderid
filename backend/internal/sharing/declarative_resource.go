// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"fmt"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// DeclaredResourcePolicies is the sharing half of one resource's declarative document: the resource
// the policies belong to, and the policies themselves.
//
// It is a bundle rather than a single policy because a document declares a list, and the loader
// stores one object per document.
type DeclaredResourcePolicies struct {
	// ResourceID identifies the resource the policies are declared on.
	ResourceID string
	// ResourceName is carried only so a startup failure can name the offending document; an id
	// alone sends the operator looking through every file in the directory.
	ResourceName string
	// OwningOUID is the organization unit that owns the resource.
	OwningOUID string
	// Policies are the decisions the document declares, in the order it declares them.
	Policies []providers.SharingPolicy
}

// DeclarativeLoaderConfig tells the framework where a resource type's documents live and how to
// read the sharing half out of them.
//
// The directory is the resource type's own, not one of this package's: a policy has no document of
// its own, it is carried inside the resource it shares. The document is therefore read twice, once
// by the resource type for itself and once here for its policies, which is the same arrangement
// inbound client profiles already use for the agents and applications that carry them.
type DeclarativeLoaderConfig struct {
	// ResourceType is the sharing resource type the policies are declared for.
	ResourceType ResourceType
	// DirectoryName is the declarative resources subdirectory the documents live in.
	DirectoryName string
	// Parser reads one document and returns its sharing half, or nil when it declares none.
	Parser func(data []byte) (*DeclaredResourcePolicies, error)
}

// loadDeclarativeResources reads a resource type's documents and declares the policies they carry.
//
// Returns nil silently when the deployment loads no declarative resources, so a consumer may call
// it without first asking whether there is anything to load.
func loadDeclarativeResources(
	ctx context.Context, svc SharingServiceInterface, fileStore *fileBasedStore,
	cfg DeclarativeLoaderConfig,
) error {
	if fileStore == nil {
		return nil
	}

	resourceCfg := declarativeresource.ResourceConfig{
		ResourceType:  string(cfg.ResourceType),
		DirectoryName: cfg.DirectoryName,
		Parser: func(data []byte) (interface{}, error) {
			return cfg.Parser(data)
		},
		IDExtractor: func(data interface{}) string {
			if d, ok := data.(*DeclaredResourcePolicies); ok && d != nil {
				return d.ResourceID
			}
			return ""
		},
	}

	seeder := &policySeeder{ctx: ctx, svc: svc, rt: cfg.ResourceType}
	loader := declarativeresource.NewResourceLoader(resourceCfg, seeder)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load %s sharing declarative resources: %w", cfg.ResourceType, err)
	}
	return nil
}

// policySeeder declares a document's policies as the loader hands them over.
//
// Declaring goes through the service rather than straight into the store so that a declared policy
// is held to the same rules as any other: the scope it may reach, the id it has to bring, and the
// one-policy-per-organization-unit rule the whole framework rests on.
type policySeeder struct {
	ctx context.Context
	svc SharingServiceInterface
	rt  ResourceType
}

// Create declares one document's policies. The id is the resource's, because the object the loader
// stores per document is that resource's policies rather than any single one of them.
func (s *policySeeder) Create(_ string, data interface{}) error {
	declared, ok := data.(*DeclaredResourcePolicies)
	if !ok {
		return fmt.Errorf("unexpected data type: %T", data)
	}
	if declared == nil || len(declared.Policies) == 0 {
		return nil
	}
	if err := requireOnePolicyPerInitiator(declared, s.rt); err != nil {
		return err
	}

	// Declaring carries no caller to scope against.
	ctx := security.WithRuntimeContext(s.ctx)
	for i, policy := range declared.Policies {
		_, svcErr := s.svc.CreateDeclarativePolicy(
			ctx, s.rt, declared.ResourceID, declared.OwningOUID, requestFromDeclaration(policy))
		if svcErr == nil {
			continue
		}
		initiator := policy.InitiatingOuID
		if initiator == "" {
			initiator = declared.OwningOUID
		}
		return fmt.Errorf(
			"failed to declare sharing policy %d of %s %q for organization unit %q: %s: %s",
			i+1, s.rt, declared.ResourceName, initiator,
			svcErr.Code, svcErr.ErrorDescription.DefaultValue)
	}
	return nil
}

// requestFromDeclaration converts a policy a resource file declares into the framework's request
// shape.
//
// The declarative and framework shapes are separate types rather than one shared struct: the
// declarative types are a provider contract, and coupling them to this package's internals would
// make every change to one a change to the other. The conversion lives here, once, because every
// shareable resource type declares its policies with the same provider types and two copies of this
// would be two things free to disagree.
//
// The id travels with the policy. A declared policy carries its own, so that a reshare beneath it
// points at the same policy on every startup; dropping it here would have the framework refuse the
// declaration rather than mint one.
func requestFromDeclaration(p providers.SharingPolicy) PolicyRequest {
	var entries []TargetEntry
	if len(p.TargetOuScope.ChildOUIDs) > 0 {
		entries = make([]TargetEntry, 0, len(p.TargetOuScope.ChildOUIDs))
		for _, e := range p.TargetOuScope.ChildOUIDs {
			entries = append(entries, TargetEntry{OUID: e.OUID, AllChildren: e.AllChildren})
		}
	}

	var rules map[string]OverlayRule
	if len(p.OverlayRules) > 0 {
		rules = make(map[string]OverlayRule, len(p.OverlayRules))
		for key, r := range p.OverlayRules {
			rules[key] = OverlayRule{
				Editable:       r.Editable,
				Value:          r.Value,
				AllowedValues:  r.AllowedValues,
				ExcludedValues: r.ExcludedValues,
			}
		}
	}

	return PolicyRequest{
		ID:             p.ID,
		InitiatingOUID: p.InitiatingOuID,
		TargetOUScope: TargetOUScope{
			AllOUs:            p.TargetOuScope.AllOUs,
			AllRoots:          p.TargetOuScope.AllRoots,
			RootOUIDs:         p.TargetOuScope.RootOUIDs,
			ExcludedRootOUIDs: p.TargetOuScope.ExcludedRootOUIDs,
			AllChildren:       p.TargetOuScope.AllChildren,
			ChildOUIDs:        entries,
			ExcludedOUIDs:     p.TargetOuScope.ExcludedOUIDs,
		},
		OverlayRules: rules,
	}
}

// requireOnePolicyPerInitiator refuses a document that declares two policies for one organization
// unit, before any of them is declared.
//
// The framework replaces on the resource and the initiating organization unit rather than on the
// policy id, so re-reading a document replaces what it declared even when it has since been
// rewritten under a new id. That is what keeps replay idempotent, and it is deliberate.
//
// Within a single document the same rule reads differently: the second policy would discard the
// first, and the file would describe something other than what it says with nothing reported. Two
// entries there can only be an authoring mistake, which is why this is the loader's question to ask
// and not the service's. The service cannot tell the two situations apart, because it sees one
// policy at a time and knows nothing of the document a policy arrived in.
//
// Policies are named by position rather than by id, because an id-less policy is a separate mistake
// the framework reports for itself and this message has to read sensibly before it is fixed.
func requireOnePolicyPerInitiator(declared *DeclaredResourcePolicies, rt ResourceType) error {
	seen := make(map[string]int, len(declared.Policies))
	for i, policy := range declared.Policies {
		initiator := policy.InitiatingOuID
		if initiator == "" {
			initiator = declared.OwningOUID
		}
		if first, repeated := seen[initiator]; repeated {
			return fmt.Errorf(
				"%s %q declares sharing policies %d and %d for the same organization unit %q: "+
					"each organization unit holds one policy per resource, and the later "+
					"declaration would replace the earlier one",
				rt, declared.ResourceName, first+1, i+1, initiator)
		}
		seen[initiator] = i
	}
	return nil
}

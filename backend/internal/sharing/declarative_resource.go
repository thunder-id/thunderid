// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"fmt"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// loadDeclarativeResources reads a resource type's documents and declares the policies they carry.
// Returns nil when the deployment loads no declarative resources, so a consumer may call it
// without first checking.
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

// policySeeder declares a document's policies as the loader hands them over. It goes through the
// service rather than the store, so a declared policy is held to the same rules as any other.
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
			ctx, s.rt, declared.ResourceID, declared.OwningOUID, policy)
		if svcErr == nil {
			continue
		}
		initiator := policy.InitiatingOUID
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

// requireOnePolicyPerInitiator refuses a document that declares two policies for one organization
// unit, before any of them is declared.
//
// The framework replaces on (resource, initiating organization unit), which keeps replay
// idempotent across startups. Within one document that same rule would silently discard the first
// policy, so the loader asks the question the service cannot: it sees one policy at a time and
// knows nothing of the document it arrived in.
//
// Policies are named by position, because an id-less policy is a separate mistake and this message
// has to read sensibly before it is fixed.
func requireOnePolicyPerInitiator(declared *DeclaredResourcePolicies, rt ResourceType) error {
	seen := make(map[string]int, len(declared.Policies))
	for i, policy := range declared.Policies {
		initiator := policy.InitiatingOUID
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

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"slices"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// entityEligibilityResolver lists the organization units an entity is in. The entity service implements it.
type entityEligibilityResolver interface {
	EntityOUs(ctx context.Context, entityID string) (ouIDs []string, found bool, err error)
}

// applicationAccessResolver answers where an application is usable. The inbound client service implements it.
type applicationAccessResolver interface {
	IsClientAccessibleFromOUByEntityID(ctx context.Context, entityID, ouID string) (bool, *tidcommon.ServiceError)
	IsClientShared(ctx context.Context, entityID string) (bool, *tidcommon.ServiceError)
}

// ConfigurableOUProvider extends the runtime organization unit provider with the two-phase wiring of the services
// that answer sign-in eligibility. Both import this package, so they are injected once they are built.
type ConfigurableOUProvider interface {
	providers.OrganizationUnitProvider
	SetSignInEligibilityResolvers(entities entityEligibilityResolver, applications applicationAccessResolver)
}

// organizationUnitProvider adapts the organization unit service to the runtime provider contract,
// mapping the management model onto the narrower view the OAuth and flow layers consume.
type organizationUnitProvider struct {
	ouService    OrganizationUnitServiceInterface
	entities     entityEligibilityResolver
	applications applicationAccessResolver
	logger       *log.Logger
}

// NewOrganizationUnitProvider adapts an organization unit service to the runtime provider
// contract. It performs no initialization of its own: it only converts between the management
// model and the runtime view.
func NewOrganizationUnitProvider(ouService OrganizationUnitServiceInterface) ConfigurableOUProvider {
	return &organizationUnitProvider{
		ouService: ouService,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OrganizationUnitProvider")),
	}
}

// SetSignInEligibilityResolvers injects the services that answer sign-in eligibility. Until they are set, every
// eligibility question is refused.
func (p *organizationUnitProvider) SetSignInEligibilityResolvers(entities entityEligibilityResolver,
	applications applicationAccessResolver) {
	p.entities = entities
	p.applications = applications
}

// GetOrganizationUnit returns the runtime view of the organization unit with the given ID.
func (p *organizationUnitProvider) GetOrganizationUnit(
	ctx context.Context, id string,
) (providers.OrganizationUnit, *tidcommon.ServiceError) {
	orgUnit, svcErr := p.ouService.GetOrganizationUnit(ctx, id)
	if svcErr != nil {
		return providers.OrganizationUnit{}, svcErr
	}
	return toProviderOU(orgUnit), nil
}

// CreateOrganizationUnit provisions an organization unit from a runtime request.
func (p *organizationUnitProvider) CreateOrganizationUnit(
	ctx context.Context, request providers.OrganizationUnitRequestWithID,
) (providers.OrganizationUnit, *tidcommon.ServiceError) {
	orgUnit, svcErr := p.ouService.CreateOrganizationUnit(ctx, toServiceRequest(request))
	if svcErr != nil {
		return providers.OrganizationUnit{}, svcErr
	}
	return toProviderOU(orgUnit), nil
}

// IsParent reports whether parentID is an ancestor of childID.
func (p *organizationUnitProvider) IsParent(
	ctx context.Context, parentID, childID string,
) (bool, *tidcommon.ServiceError) {
	return p.ouService.IsParent(ctx, parentID, childID)
}

// IsOrganizationUnitExists reports whether an organization unit with the given ID exists.
func (p *organizationUnitProvider) IsOrganizationUnitExists(
	ctx context.Context, id string,
) (bool, *tidcommon.ServiceError) {
	return p.ouService.IsOrganizationUnitExists(ctx, id)
}

// GetOrganizationUnitChildren returns a page of the organization unit's direct children.
func (p *organizationUnitProvider) GetOrganizationUnitChildren(
	ctx context.Context, id string, limit, offset int, f *tidcommon.FilterGroup,
) (*providers.OrganizationUnitListResponse, *tidcommon.ServiceError) {
	children, svcErr := p.ouService.GetOrganizationUnitChildren(ctx, id, limit, offset, f)
	if svcErr != nil {
		return nil, svcErr
	}
	return toProviderList(children), nil
}

// GetEligibleOUs returns the organization units the signed-in user of the sign-in on the context may sign in to,
// kept where its application is usable. Without a signed-in user the entity store holds, it lists none.
func (p *organizationUnitProvider) GetEligibleOUs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	if svcErr := p.checkEligibilityResolvers(ctx); svcErr != nil {
		return nil, svcErr
	}
	b, _ := authnprovidercm.SignInBoundaryFrom(ctx)
	if b.EntityID == "" {
		return nil, &ErrorSignedInUserRequired
	}
	ouIDs, found, svcErr := p.entityEligibleOUs(ctx, b)
	if svcErr != nil {
		return nil, svcErr
	}
	if !found {
		return nil, &ErrorSignedInUserRequired
	}
	return ouIDs, nil
}

// entityEligibleOUs returns the organization units the boundary's entity is in, kept to the one the boundary fixed
// and to those its application is usable from. found is false when the entity store holds no such entity.
func (p *organizationUnitProvider) entityEligibleOUs(ctx context.Context,
	b authnprovidercm.SignInBoundary) ([]string, bool, *tidcommon.ServiceError) {
	ouIDs, found, err := p.entities.EntityOUs(ctx, b.EntityID)
	if err != nil {
		p.logger.Error(ctx, "Failed to resolve the organization units an entity is in", log.Error(err))
		return nil, false, &tidcommon.InternalServerError
	}
	if !found {
		return nil, false, nil
	}
	eligible := make([]string, 0, len(ouIDs))
	for _, ouID := range ouIDs {
		if b.OUID != "" && ouID != b.OUID {
			continue
		}
		usable, svcErr := p.applicationUsableFrom(ctx, b.ApplicationID, ouID)
		if svcErr != nil {
			return nil, false, svcErr
		}
		if usable {
			eligible = append(eligible, ouID)
		}
	}
	return eligible, true, nil
}

// applicationUsableFrom reports whether the application can be used from the organization unit. Without an
// application there is nothing to check. An organization unit that does not exist is not usable.
func (p *organizationUnitProvider) applicationUsableFrom(ctx context.Context,
	applicationID, ouID string) (bool, *tidcommon.ServiceError) {
	if applicationID == "" {
		return true, nil
	}
	usable, svcErr := p.applications.IsClientAccessibleFromOUByEntityID(ctx, applicationID, ouID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return false, nil
		}
		return false, svcErr
	}
	return usable, nil
}

// IsEligibleOU reports whether the sign-in on the context may land in the organization unit: one its signed-in
// user is eligible for, or, before sign-in or for a user the entity store does not hold, one its application is
// usable from. An organization unit that does not exist is not eligible.
func (p *organizationUnitProvider) IsEligibleOU(ctx context.Context, ouID string) (bool, *tidcommon.ServiceError) {
	if svcErr := p.checkEligibilityResolvers(ctx); svcErr != nil {
		return false, svcErr
	}
	if ouID == "" {
		return false, nil
	}
	b, _ := authnprovidercm.SignInBoundaryFrom(ctx)
	if b.EntityID != "" {
		b.OUID = ouID
		ouIDs, found, svcErr := p.entityEligibleOUs(ctx, b)
		if svcErr != nil {
			return false, svcErr
		}
		if found {
			return slices.Contains(ouIDs, ouID), nil
		}
	}
	return p.applicationUsableFrom(ctx, b.ApplicationID, ouID)
}

// IsApplicationShared reports whether the application of the sign-in on the context is shared.
func (p *organizationUnitProvider) IsApplicationShared(ctx context.Context) (bool, *tidcommon.ServiceError) {
	if svcErr := p.checkEligibilityResolvers(ctx); svcErr != nil {
		return false, svcErr
	}
	b, _ := authnprovidercm.SignInBoundaryFrom(ctx)
	return p.applications.IsClientShared(ctx, b.ApplicationID)
}

// checkEligibilityResolvers refuses an eligibility question until the services that answer it are injected.
func (p *organizationUnitProvider) checkEligibilityResolvers(ctx context.Context) *tidcommon.ServiceError {
	if p.entities == nil || p.applications == nil {
		p.logger.Error(ctx, "Sign-in eligibility resolvers are not configured")
		return &tidcommon.InternalServerError
	}
	return nil
}

// toProviderOU maps the management organization unit onto the runtime view.
func toProviderOU(orgUnit OrganizationUnit) providers.OrganizationUnit {
	parent := ""
	if orgUnit.Parent != nil {
		parent = *orgUnit.Parent
	}
	return providers.OrganizationUnit{
		ID:              orgUnit.ID,
		Parent:          parent,
		Handle:          orgUnit.Handle,
		Name:            orgUnit.Name,
		Description:     orgUnit.Description,
		LogoURL:         orgUnit.LogoURL,
		TosURI:          orgUnit.TosURI,
		PolicyURI:       orgUnit.PolicyURI,
		CookiePolicyURI: orgUnit.CookiePolicyURI,
	}
}

// toServiceRequest maps a runtime provisioning request onto the management request.
func toServiceRequest(request providers.OrganizationUnitRequestWithID) OrganizationUnitRequestWithID {
	return OrganizationUnitRequestWithID{
		ID:          request.ID,
		Handle:      request.Handle,
		Name:        request.Name,
		Description: request.Description,
		Parent:      request.Parent,
	}
}

// toProviderList maps a management list response onto the runtime view. Pagination links are a
// management API concern and are not carried across.
func toProviderList(list *OrganizationUnitListResponse) *providers.OrganizationUnitListResponse {
	if list == nil {
		return nil
	}
	units := make([]providers.OrganizationUnitBasic, 0, len(list.OrganizationUnits))
	for _, unit := range list.OrganizationUnits {
		units = append(units, providers.OrganizationUnitBasic{
			ID:          unit.ID,
			Handle:      unit.Handle,
			Name:        unit.Name,
			Description: unit.Description,
		})
	}
	return &providers.OrganizationUnitListResponse{
		TotalResults:      list.TotalResults,
		StartIndex:        list.StartIndex,
		Count:             list.Count,
		OrganizationUnits: units,
	}
}

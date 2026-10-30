// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"slices"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// extensionAuthResolver signs in the local user an extension step's service verified. It trusts the
// identifier an extension stored under its namespace, so flow save binds it to the extension that
// routes to it.
type extensionAuthResolver struct {
	providers.Executor
	entityProvider entityprovider.EntityProviderInterface
	authnProvider  providers.AuthnProviderManager
	ouProvider     providers.OrganizationUnitProvider
	logger         *log.Logger
}

var _ providers.Executor = (*extensionAuthResolver)(nil)

// newExtensionAuthResolver creates an extension auth resolver.
func newExtensionAuthResolver(flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface, authnProvider providers.AuthnProviderManager,
	ouProvider providers.OrganizationUnitProvider) *extensionAuthResolver {
	base := flowFactory.CreateExecutor(ExecutorNameExtensionAuthResolver, providers.ExecutorTypeAuthentication,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: common.NodePropertySourceKey, IsRequired: true},
				{Property: common.NodePropertyMatchAttribute},
				{Property: common.NodePropertyAllowAuthenticationWithoutLocalUser},
				{Property: common.NodePropertyAttributeMapping},
			},
		})
	return &extensionAuthResolver{
		Executor:       base,
		entityProvider: entityProvider,
		authnProvider:  authnProvider,
		ouProvider:     ouProvider,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameExtensionAuthResolver),
			log.String(log.LoggerKeyExecutorName, ExecutorNameExtensionAuthResolver)),
	}
}

// Execute resolves the one active local user the extension's verified identifier names and signs
// them in. With allowAuthenticationWithoutLocalUser, an identifier that matches no one marks the user
// for a later provisioning step instead.
func (e *extensionAuthResolver) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	props, invalidProperty := common.ParseExtensionAuthResolverProperties(ctx.NodeProperties)
	if invalidProperty != "" {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrExtensionAuthResolverConfigInvalid
		return execResp, nil
	}
	value := ctx.RuntimeData[props.SourceKey]
	if value == "" {
		e.logger.Debug(ctx.Context, "No verified identifier from the extension",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
		return execResp, nil
	}

	entityID, noMatch, err := e.resolveUser(ctx, props.MatchAttribute, value, execResp)
	if err != nil {
		return nil, err
	}
	if entityID == "" {
		if noMatch && ctx.FlowType == providers.FlowTypeAuthentication && isAuthenticationWithoutLocalUserAllowed(ctx) {
			return e.markForProvisioning(ctx, props, execResp)
		}
		execResp.Status = providers.ExecFailure
		return execResp, nil
	}

	svcErr := authenticateEntity(ctx, e.authnProvider, authnprovidercm.CredentialTypeExtensionVerifiedEntityID,
		entityID, ctx.AuthUser, execResp)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, errors.New("failed to authenticate the extension-verified user")
		}
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrEntityAuthFailed, entitytype.TypeCategoryUser)
		return execResp, nil
	}
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// resolveUser returns the ID of the one active user matchAttribute and value name that the application
// allows, or "" with execResp.Error set. noMatch reports that an attribute lookup matched no entity at
// all, the one case a provisioned user could fill. Not found, not a user, not active and not allowed
// give the same error, so the resolver cannot be used to tell which accounts exist.
func (e *extensionAuthResolver) resolveUser(ctx *providers.NodeContext, matchAttribute, value string,
	execResp *providers.ExecutorResponse) (entityID string, noMatch bool, err error) {
	entityID = value
	if matchAttribute != userAttributeUserID {
		id, epErr := e.entityProvider.IdentifyEntity(map[string]interface{}{matchAttribute: value})
		switch {
		case epErr != nil && epErr.Code == entityprovider.ErrorCodeEntityNotFound,
			epErr == nil && (id == nil || *id == ""):
			execResp.Error = errForEntityCategory(ErrEntityNotFound, entitytype.TypeCategoryUser)
			return "", true, nil
		case epErr != nil && epErr.Code == entityprovider.ErrorCodeAmbiguousEntity:
			execResp.Error = errForEntityCategory(ErrAmbiguousEntityIdentity, entitytype.TypeCategoryUser)
			return "", false, nil
		case epErr != nil:
			return "", false, errors.New("failed to identify the extension-verified user: " + string(epErr.Code))
		}
		entityID = *id
	}

	entity, epErr := e.entityProvider.GetEntity(entityID)
	if epErr != nil && epErr.Code != entityprovider.ErrorCodeEntityNotFound {
		return "", false, errors.New("failed to read the extension-verified user: " + string(epErr.Code))
	}
	allowed := false
	if epErr == nil && entity != nil && entity.Category == providers.EntityCategoryUser &&
		entity.State == providers.EntityStateActive {
		if allowed, err = e.isAllowedByApplication(ctx, entity); err != nil {
			return "", false, err
		}
	}
	if !allowed {
		execResp.Error = errForEntityCategory(ErrEntityNotFound, entitytype.TypeCategoryUser)
		return "", false, nil
	}
	return entity.ID, false, nil
}

// isAllowedByApplication reports whether the application allows user to sign in: the user is of an
// allowed user type and in the application's organization unit or one below it.
func (e *extensionAuthResolver) isAllowedByApplication(ctx *providers.NodeContext,
	user *providers.Entity) (bool, error) {
	if !slices.Contains(ctx.Application.AllowedUserTypes, user.Type) || ctx.Application.OUID == "" {
		return false, nil
	}
	served, svcErr := e.ouProvider.IsParent(ctx.Context, ctx.Application.OUID, user.OUID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return false, errors.New("failed to check the extension-verified user's organization unit")
		}
		return false, nil
	}
	return served, nil
}

// markForProvisioning completes without signing anyone in, marking the user for a provisioning step
// and offering it the mapped attributes as external claims, which provisioning never takes a
// credential from.
func (e *extensionAuthResolver) markForProvisioning(ctx *providers.NodeContext,
	props common.ExtensionAuthResolverProperties,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	e.logger.Debug(ctx.Context, "No local user for the extension-verified identifier, marking it for provisioning",
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	execResp.Error = nil
	claims := make(map[string]interface{}, len(props.AttributeMapping))
	for attribute, key := range props.AttributeMapping {
		if value := ctx.RuntimeData[key]; value != "" {
			claims[attribute] = value
		}
	}
	if len(claims) > 0 {
		if err := setExternalIdentity(execResp, "", "", claims); err != nil {
			return nil, err
		}
	}
	execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

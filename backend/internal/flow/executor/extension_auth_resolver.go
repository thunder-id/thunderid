// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"errors"
	"fmt"

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
	logger         *log.Logger
}

var _ providers.Executor = (*extensionAuthResolver)(nil)

// userResolution is how the resolver's lookup of the verified identifier ended.
type userResolution int

const (
	// userResolved names the one active user the resolver signs in.
	userResolved userResolution = iota
	// userNoMatch is an attribute lookup that matched no entity, the one case a provisioned user could fill.
	userNoMatch
	// userRefused is any other identifier the resolver signs no one in for.
	userRefused
)

// newExtensionAuthResolver creates an extension auth resolver.
func newExtensionAuthResolver(flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager) *extensionAuthResolver {
	base := flowFactory.CreateExecutor(ExecutorNameExtensionAuthResolver, providers.ExecutorTypeAuthentication,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAuthentication},
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
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameExtensionAuthResolver),
			log.String(log.LoggerKeyExecutorName, ExecutorNameExtensionAuthResolver)),
	}
}

// Execute resolves the one active local user the extension's verified identifier names and signs
// them in. With allowAuthenticationWithoutLocalUser, an identifier that matches no one marks the user
// for a later provisioning step instead.
func (e *extensionAuthResolver) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	props, invalidProperty := common.ParseExtensionAuthResolverProperties(ctx.NodeProperties)
	if invalidProperty != "" {
		return nil, fmt.Errorf("extension auth resolver property %s has an unusable value", invalidProperty)
	}
	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}
	value := ctx.RuntimeData[props.SourceKey]
	if value == "" {
		e.logger.Debug(ctx.Context, "No verified identifier from the extension",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
		return execResp, nil
	}

	entityID, resolution, err := e.resolveUser(ctx, props.MatchAttribute, value)
	if err != nil {
		return nil, err
	}
	// A user already signed in is never replaced by one provisioned for the verified identifier.
	if resolution == userNoMatch && !ctx.AuthUser.IsAuthenticated() && isAuthenticationWithoutLocalUserAllowed(ctx) {
		return e.markForProvisioning(ctx, props, execResp)
	}
	if resolution != userResolved {
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrEntityNotFound, entitytype.TypeCategoryUser)
		return execResp, nil
	}

	svcErr := authenticateTrustedEntity(ctx, e.authnProvider, authnprovidercm.CredentialTypeExtensionVerifiedEntityID,
		entityID, ctx.AuthUser, execResp)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to authenticate the extension-verified user: %s", svcErr.Code)
		}
		execResp.Status = providers.ExecFailure
		execResp.Error = cmp.Or(statedHold(svcErr),
			errForEntityCategory(ErrEntityAuthFailed, entitytype.TypeCategoryUser))
		return execResp, nil
	}
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// resolveUser returns the ID of the one active user matchAttribute and value name, and how the lookup
// ended. Every outcome but userResolved gives the same error, so the resolver cannot be used to tell
// which accounts exist.
func (e *extensionAuthResolver) resolveUser(ctx *providers.NodeContext, matchAttribute,
	value string) (string, userResolution, error) {
	entityID := value
	if matchAttribute != authnprovidercm.UserAttributeUserID {
		id, epErr := e.entityProvider.IdentifyEntity(ctx.Context, map[string]interface{}{matchAttribute: value})
		switch {
		case epErr != nil && epErr.Code == entityprovider.ErrorCodeEntityNotFound,
			epErr == nil && (id == nil || *id == ""):
			return "", userNoMatch, nil
		case epErr != nil && epErr.Code == entityprovider.ErrorCodeAmbiguousEntity:
			return "", userRefused, nil
		case epErr != nil:
			return "", userRefused, errors.New("failed to identify the extension-verified user: " +
				string(epErr.Code))
		}
		entityID = *id
	}

	entity, epErr := e.entityProvider.GetEntity(ctx.Context, entityID)
	if epErr != nil && epErr.Code != entityprovider.ErrorCodeEntityNotFound {
		return "", userRefused, errors.New("failed to read the extension-verified user: " + string(epErr.Code))
	}
	// The authentication manager admits an agent of a type the application allows, so the resolver keeps
	// out every entity that is not a user itself.
	if epErr != nil || entity == nil || entity.Category != providers.EntityCategoryUser ||
		entity.State != providers.EntityStateActive {
		return "", userRefused, nil
	}
	signedIn, err := e.isSignedInUserOrNoOne(ctx, entity.ID)
	if err != nil {
		return "", userRefused, err
	}
	if !signedIn {
		return "", userRefused, nil
	}
	return entity.ID, userResolved, nil
}

// isSignedInUserOrNoOne reports whether entityID is the user the flow already signed in, or whether no
// one is signed in. After a first factor the resolver confirms that user and never switches to another.
func (e *extensionAuthResolver) isSignedInUserOrNoOne(ctx *providers.NodeContext, entityID string) (bool, error) {
	if !ctx.AuthUser.IsAuthenticated() {
		return true, nil
	}
	_, entityRef, svcErr := e.authnProvider.GetEntityReference(ctx.Context, ctx.AuthUser)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return false, fmt.Errorf("failed to read the signed-in user: %s", svcErr.Code)
		}
		return false, nil
	}
	if entityRef == nil || entityRef.EntityID != entityID {
		e.logger.Debug(ctx.Context, "The extension-verified user is not the user already signed in",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		return false, nil
	}
	return true, nil
}

// markForProvisioning completes without signing anyone in, marking the user for a provisioning step
// and offering it the mapped attributes as external claims, which provisioning never takes a
// credential from.
func (e *extensionAuthResolver) markForProvisioning(ctx *providers.NodeContext,
	props common.ExtensionAuthResolverProperties,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	e.logger.Debug(ctx.Context, "No local user for the extension-verified identifier, marking it for provisioning",
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	claims := make(map[string]interface{}, len(props.AttributeMapping)+1)
	for attribute, key := range props.AttributeMapping {
		if value := ctx.RuntimeData[key]; value != "" {
			claims[attribute] = value
		}
	}
	// The verified value is the one the next sign-in matches on, so it is always offered under the match
	// attribute, whatever the mapping holds, and provisioning takes it over a submitted value.
	claims[props.MatchAttribute] = ctx.RuntimeData[props.SourceKey]
	if err := setExternalIdentity(execResp, "", "", claims); err != nil {
		return nil, err
	}
	execResp.RuntimeData[common.RuntimeKeyExtensionVerifiedAttribute] = props.MatchAttribute
	execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

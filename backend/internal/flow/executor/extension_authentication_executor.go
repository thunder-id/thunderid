// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	// extensionFieldAuthenticatedUser is the field of the service's SUCCESS answer that names the user it
	// verified. It is a contract field, so it is never stored as data.
	extensionFieldAuthenticatedUser = "authenticatedUser"
	// extensionFieldSubject and extensionFieldAttributes are the fields of authenticatedUser: the subject the
	// service verified, and the attributes it returns for that subject.
	extensionFieldSubject    = "subject"
	extensionFieldAttributes = "attributes"
)

// extensionAuthenticationExecutor extends the extension executor: it calls the organization's service
// through an authentication endpoint connection the same way, and once the service answers SUCCESS with the
// user it verified, it authenticates that identity through the authn provider manager.
type extensionAuthenticationExecutor struct {
	*extensionExecutor
	idpService    providers.IDPProvider
	authnProvider providers.AuthnProviderManager
}

var _ providers.Executor = (*extensionAuthenticationExecutor)(nil)

// newExtensionAuthenticationExecutor creates an extension authentication executor.
func newExtensionAuthenticationExecutor(flowFactory core.FlowFactoryInterface,
	connectionProvider httpconnection.HTTPConnectionProvider, idpService providers.IDPProvider,
	authnProvider providers.AuthnProviderManager, client httpservice.HTTPClientInterface,
	observabilitySvc providers.ObservabilityProvider) *extensionAuthenticationExecutor {
	base := buildExtensionExecutor(ExecutorNameExtensionAuthentication, providers.ExecutorTypeAuthentication,
		[]providers.ExecutorSupportedProperties{
			{Property: common.NodePropertyAllowAuthenticationWithoutLocalUser},
			{Property: common.NodePropertyAllowRegistrationWithExistingUser},
		}, flowFactory, connectionProvider, client, observabilitySvc)
	return &extensionAuthenticationExecutor{
		extensionExecutor: base,
		idpService:        idpService,
		authnProvider:     authnProvider,
	}
}

// Execute makes the extension call. While the service redirects, asks for input or fails, the answer is the
// extension executor's own; once it answers SUCCESS it authenticates the user it verified.
func (e *extensionAuthenticationExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	execResp, body, err := e.run(ctx)
	if err != nil || execResp == nil || execResp.Status != providers.ExecComplete {
		return execResp, err
	}
	// The identity is consumed here and not left in the step's stored answer.
	delete(execResp.RuntimeData, common.ExtensionNamespace(ctx.CurrentNodeID)+extensionFieldAuthenticatedUser)
	execResp.AuthUser = ctx.AuthUser
	return e.authenticate(ctx, execResp, body)
}

// authenticate signs in the identity the service verified, in the way a federated sign-in does: the
// connection's mapping and account-linking rules build the result, and the local user, if there is one, is
// resolved from it.
func (e *extensionAuthenticationExecutor) authenticate(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, body map[string]interface{}) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	props, invalidProperty := common.ParseExtensionProperties(ctx.NodeProperties)
	if invalidProperty != "" {
		return nil, fmt.Errorf("extension authentication step property %s has an unusable value", invalidProperty)
	}
	subject, claims, ok := verifiedIdentity(body)
	if !ok {
		logger.Warn(ctx.Context, "The service answered SUCCESS without a verified user",
			log.String("connectionID", props.ConnectionID), log.String("nodeID", ctx.CurrentNodeID))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
		return execResp, nil
	}

	logger.Debug(ctx.Context, "Authenticating the identity the service verified",
		log.String("connectionID", props.ConnectionID), log.String("nodeID", ctx.CurrentNodeID),
		log.MaskedString("subject", subject), log.Any("attributeNames", sortedKeys(claims)),
		log.Bool("alreadySignedIn", execResp.AuthUser.IsAuthenticated()))

	existingAttributes := make(map[string]interface{})
	if execResp.AuthUser.IsAuthenticated() {
		metadata := core.BuildGetAttributesMetadata(ctx)
		authUser, attributes, err := e.authnProvider.GetUserAttributes(ctx.Context, nil, metadata, execResp.AuthUser)
		if err != nil {
			logger.Warn(ctx.Context,
				"Failed to fetch user attributes for authenticated user, proceeding without attributes")
		} else {
			execResp.AuthUser = authUser
			for key, value := range attributes.Attributes {
				if value != nil {
					existingAttributes[key] = value.Value
				}
			}
		}
	}

	credentials := map[string]interface{}{
		authnprovidercm.CredentialTypeExtensionVerifiedEntityID: &authncm.ExtensionVerifiedCredential{
			ConnectionID: props.ConnectionID,
			Subject:      subject,
			Claims:       claims,
		},
	}
	authUser, federatedAttributes, svcErr := e.authnProvider.AuthenticateUser(
		ctx.Context, nil, credentials, nil, federatedSignInMetadata(ctx), execResp.AuthUser)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			logger.Warn(ctx.Context, "The identity the service verified was refused",
				log.String("connectionID", props.ConnectionID), log.String("errorCode", svcErr.Code))
			execResp.Status = providers.ExecFailure
			execResp.Error = cmp.Or(statedHold(svcErr), svcErr)
			return execResp, nil
		}
		logger.Error(ctx.Context, "Extension authentication failed", log.String("errorCode", svcErr.Code))
		return nil, errors.New("extension authentication failed")
	}
	execResp.AuthUser = authUser
	if err := writeSessionValues(execResp, takeSessionValues(federatedAttributes)); err != nil {
		return nil, err
	}

	connection, svcErr := e.idpService.GetIdentityProvider(ctx.Context, props.ConnectionID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to retrieve authentication endpoint connection %s: %s",
			props.ConnectionID, svcErr.Code)
	}
	if !validateFederatedIdentifierConsistency(ctx, connection, federatedAttributes, existingAttributes) {
		logger.Warn(ctx.Context, "The verified identity does not match the user already signed in",
			log.String("connectionID", props.ConnectionID))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrInvalidFederatedUser
		return execResp, nil
	}

	sub := systemutils.ConvertInterfaceValueToString(federatedAttributes[userAttributeSub])
	if err := setExternalIdentity(execResp, connection, sub, federatedAttributes); err != nil {
		return nil, err
	}
	setFederatedEntityState(ctx.Context, execResp, e.authnProvider)

	switch ctx.FlowType {
	case providers.FlowTypeAuthentication:
		if isAuthenticationWithoutLocalUserAllowed(ctx) {
			execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
		}
	case providers.FlowTypeRegistration:
		if isRegistrationWithExistingUserAllowed(ctx) {
			execResp.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser] = dataValueTrue
		}
	}
	logger.Info(ctx.Context, "Extension authentication completed",
		log.String("connectionID", props.ConnectionID), log.String("nodeID", ctx.CurrentNodeID),
		log.String("entityState", execResp.RuntimeData[common.RuntimeKeyEntityState]),
		log.Bool("eligibleForProvisioning",
			execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] == dataValueTrue))
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// sortedKeys returns the names of the attributes in m, in order, so a log shows which attributes arrived and
// never their values.
func sortedKeys(m map[string]interface{}) []string {
	return slices.Sorted(maps.Keys(m))
}

// verifiedIdentity reads the user the service verified from its answer: a non-empty subject and the
// attributes it returns for it. Numbers are given as the JSON number they were sent as.
func verifiedIdentity(body map[string]interface{}) (string, map[string]interface{}, bool) {
	user, ok := body[extensionFieldAuthenticatedUser].(map[string]interface{})
	if !ok {
		return "", nil, false
	}
	subject, _ := user[extensionFieldSubject].(string)
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", nil, false
	}
	claims := make(map[string]interface{})
	if attributes, ok := user[extensionFieldAttributes].(map[string]interface{}); ok {
		for name, value := range attributes {
			if value == nil {
				continue
			}
			if number, isNumber := value.(json.Number); isNumber {
				claims[name] = number.String()
				continue
			}
			claims[name] = value
		}
	}
	return subject, claims, true
}

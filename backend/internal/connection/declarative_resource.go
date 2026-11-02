// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/varname"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeConnection = "connection"
	paramTypeConnection    = "Connection"
	// exportErrorCodeConnectionValidation is the code of an export error for a connection that cannot
	// be exported.
	exportErrorCodeConnectionValidation = "CONN_VALIDATION_ERROR"
)

// connectionExporter implements declarativeresource.ResourceExporter and
// declarativeresource.PerResourceRuler for connections, unifying the identity-provider and
// notification-sender services under the single "connection" declarative/export resource type
// that matches the /connections API and console — replacing the legacy "identity_provider" and
// "notification_sender" resource types.
type connectionExporter struct {
	idpService        idp.IDPServiceInterface
	senderService     notification.NotificationSenderMgtSvcInterface
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface
	httpService       httpconnection.HTTPConnectionServiceInterface
	sharingService    sharing.SharingServiceInterface
}

// newConnectionExporter creates a new connection exporter.
func newConnectionExporter(idpService idp.IDPServiceInterface,
	senderService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface) *connectionExporter {
	return &connectionExporter{
		idpService:        idpService,
		senderService:     senderService,
		authZENPDPService: authZENPDPService,
		httpService:       httpService,
	}
}

// NewConnectionExporterForTest creates a new connection exporter for testing purposes.
func NewConnectionExporterForTest(idpService idp.IDPServiceInterface,
	senderService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface) *connectionExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newConnectionExporter(idpService, senderService, authZENPDPService, httpService)
}

// GetResourceType returns the resource type for connections.
func (e *connectionExporter) GetResourceType() string {
	return resourceTypeConnection
}

// GetParameterizerType returns the parameterizer type for connections.
func (e *connectionExporter) GetParameterizerType() string {
	return paramTypeConnection
}

// GetAllResourceIDs returns the IDs of every configured connection instance across both
// backing services, restricted to vendors registered with /connections.
func (e *connectionExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	ids := make([]string, 0)

	idps, svcErr := e.idpService.GetIdentityProviderList(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	for _, instance := range idps {
		if _, ok := idpVendorName(instance.Type); ok {
			ids = append(ids, instance.ID)
		}
	}

	senders, svcErr := e.senderService.ListSenders(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	for _, sender := range senders {
		switch sender.Type {
		case ncommon.NotificationSenderTypeMessage:
			if _, ok := smsVendorName(sender.Provider); ok {
				ids = append(ids, sender.ID)
			}
		case ncommon.NotificationSenderTypeEmail:
			if _, ok := emailVendorName(sender.Provider); ok {
				ids = append(ids, sender.ID)
			}
		}
	}

	if e.authZENPDPService != nil {
		pdpConnections, svcErr := e.authZENPDPService.ListAuthZENPDPs(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, connection := range pdpConnections {
			if !connection.IsReadOnly {
				ids = append(ids, connection.ID)
			}
		}
	}

	if e.httpService != nil {
		httpConnections, svcErr := e.httpService.ListHTTPConnections(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, connection := range httpConnections {
			if !connection.IsReadOnly {
				ids = append(ids, connection.ID)
			}
		}
	}

	return ids, nil
}

// GetResourceByID retrieves a connection instance by ID for export, trying the identity-provider
// service first and falling back to the notification-sender service.
//
// Unlike the live /connections read API (which always masks secret property values), this
// returns them in plaintext: the export parameterizer needs the real value to externalize it to
// the generated .env file (see GetResourceRulesForResource). The rendered YAML itself never
// carries the plaintext value — the parameterizer replaces it with a template placeholder before
// the document is written out.
func (e *connectionExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError) {
	resource, name, svcErr := e.getConnectionModel(ctx, id)
	if svcErr != nil {
		return nil, "", svcErr
	}
	// A document that silently lost its sharing would re-import as a connection nobody else can use, so
	// this fails the export rather than thinning it.
	if e.sharingService != nil {
		replayable, sharingErr := e.sharingService.ExportPolicies(ctx, ConnectionSharingType, id)
		if sharingErr != nil {
			return nil, "", translateSharingError(sharingErr)
		}
		for _, r := range replayable {
			resource.SharingPolicies = append(resource.SharingPolicies, r.Request)
		}
	}
	return resource, name, nil
}

// getConnectionModel reads a connection from whichever service holds it.
func (e *connectionExporter) getConnectionModel(ctx context.Context, id string) (
	*connectionExportModel, string, *tidcommon.ServiceError) {
	idpDTO, svcErr := e.idpService.GetIdentityProvider(ctx, id)
	if svcErr == nil {
		model, err := connectionModelFromIDPDTO(*idpDTO)
		if err != nil {
			return nil, "", &tidcommon.InternalServerError
		}
		return &model, model.Name, nil
	}
	if svcErr.Code != idp.ErrorIDPNotFound.Code {
		return nil, "", svcErr
	}

	senderDTO, svcErr := e.senderService.GetSender(ctx, id)
	if svcErr == nil {
		model, err := connectionModelFromSenderDTO(*senderDTO)
		if err != nil {
			return nil, "", &tidcommon.InternalServerError
		}
		return &model, model.Name, nil
	}
	if svcErr.Code != notification.ErrorSenderNotFound.Code {
		return nil, "", svcErr
	}

	if e.authZENPDPService != nil {
		pdpConnection, svcErr := e.authZENPDPService.GetAuthZENPDP(ctx, id)
		if svcErr != nil {
			return nil, "", svcErr
		}
		if pdpConnection != nil {
			model, err := connectionModelFromAuthZENPDP(*pdpConnection)
			if err != nil {
				return nil, "", &tidcommon.InternalServerError
			}
			return &model, model.Name, nil
		}
	}

	if e.httpService != nil {
		httpConnection, httpErr := e.httpService.GetHTTPConnection(ctx, id)
		if httpErr == nil {
			model, err := connectionModelFromHTTP(*httpConnection)
			if err != nil {
				return nil, "", &tidcommon.InternalServerError
			}
			return &model, model.Name, nil
		}
		if httpErr.Code != httpconnection.ErrorNotFound.Code {
			return nil, "", httpErr
		}
	}

	return nil, "", svcErr
}

// ValidateResource validates a connection resource prior to export.
func (e *connectionExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger) (string, *declarativeresource.ExportError) {
	model, ok := resource.(*connectionExportModel)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeConnection, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
		model.Name, resourceTypeConnection, id, exportErrorCodeConnectionValidation, logger,
	); err != nil {
		return "", err
	}
	if reason := unexportableCredentialNames(model.Name, model.Authentication); reason != "" {
		return "", &declarativeresource.ExportError{
			ResourceType: resourceTypeConnection,
			ResourceID:   id,
			Error:        fmt.Sprintf("connection '%s' cannot be exported: %s", model.Name, reason),
			Code:         exportErrorCodeConnectionValidation,
		}
	}

	return model.Name, nil
}

// GetResourceRules returns the default (no-secret) parameterization rules for connections.
// GetResourceRulesForResource is used instead whenever the concrete resource is available, since
// which field (if any) holds a secret depends on the vendor.
func (e *connectionExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{}
}

// GetResourceRulesForResource returns per-vendor parameterization rules: the secret field (if
// any) for the resource's vendor is externalized to a template variable on export.
func (e *connectionExporter) GetResourceRulesForResource(
	resource interface{}) *declarativeresource.ResourceRules {
	model, ok := resource.(*connectionExportModel)
	if !ok {
		return e.GetResourceRules()
	}

	if isIDPBackedVendorName(model.Type) {
		if model.ClientSecret == "" {
			return &declarativeresource.ResourceRules{}
		}
		return &declarativeresource.ResourceRules{SecretVariables: []string{"ClientSecret"}}
	}

	switch model.Type {
	case "twilio":
		return &declarativeresource.ResourceRules{SecretVariables: []string{"AuthToken"}}
	case "vonage":
		return &declarativeresource.ResourceRules{SecretVariables: []string{"APISecret"}}
	case authZENPDPVendorName:
		return authenticationResourceRules(model.Authentication)
	case smsGatewayVendorName:
		return authenticationResourceRules(model.Authentication)
	case emailSMTPVendorName, httpVendorName:
		return authenticationResourceRules(model.Authentication)
	default:
		// Any future vendor with no secret of its own has nothing to externalize.
		return &declarativeresource.ResourceRules{}
	}
}

// SecretFields names every field an exported connection of any vendor holds a credential in: the
// identity provider client secret, each message vendor's own secret, and the credential fields of
// every outbound authentication method an email connection can use.
func (e *connectionExporter) SecretFields() []string {
	return append([]string{"clientSecret", "authToken", "apiSecret"},
		outboundauth.SecretFieldPaths("authentication.properties")...)
}

// authenticationResourceRules externalizes each secret field of a connection's outbound
// authentication to a template variable.
func authenticationResourceRules(
	auth *outboundauth.Authentication) *declarativeresource.ResourceRules {
	variables := outboundauth.SecretVariablePaths(auth, "Authentication.Properties")
	if len(variables) == 0 {
		return &declarativeresource.ResourceRules{}
	}
	return &declarativeresource.ResourceRules{SecretVariables: variables}
}

// unexportableCredentialNames reports why the credentials an authentication method keys by name, such
// as API-key headers, cannot each be exported as a parameter of their own, or "" when they can. The
// parameterizer reads a dot as a path separator, and two names deriving one parameter would share a
// value, so either would leave a secret in the YAML or lose one.
func unexportableCredentialNames(connectionName string, auth *outboundauth.Authentication) string {
	if auth == nil {
		return ""
	}
	authType, ok := outboundauth.ParseType(auth.Type)
	if !ok {
		return ""
	}
	method, ok := outboundauth.GetMethod(authType)
	if !ok || method.AdditionalProperties == nil || !method.AdditionalProperties.Credential {
		return ""
	}
	derivedFrom := make(map[string]string, len(auth.Properties))
	for _, name := range slices.Sorted(maps.Keys(auth.Properties)) {
		if strings.Contains(name, ".") {
			return fmt.Sprintf("the API-key header '%s' contains a dot", name)
		}
		parameter := varname.DeriveVariableName(paramTypeConnection, connectionName, name)
		if other, taken := derivedFrom[parameter]; taken {
			return fmt.Sprintf("the API-key headers '%s' and '%s' map to the same parameter %s", other, name,
				parameter)
		}
		derivedFrom[parameter] = name
	}
	return ""
}

// rawPropertyValues returns a name->value map for the given properties WITHOUT masking secret
// values. Unlike propertyValues (used by the live read API), this is only safe for internal
// consumers that never let the value reach an external response: the export parameterizer
// captures it here only to immediately externalize it to the generated .env file.
func rawPropertyValues(props []cmodels.Property) (map[string]string, error) {
	values := make(map[string]string, len(props))
	for i := range props {
		value, err := props[i].GetValue()
		if err != nil {
			return nil, err
		}
		values[props[i].GetName()] = value
	}
	return values, nil
}

// connectionModelFromIDPDTO builds the unified export model from an identity-provider DTO.
func connectionModelFromIDPDTO(dto providers.IDPDTO) (connectionExportModel, error) {
	vendor, ok := idpVendorName(dto.Type)
	if !ok {
		return connectionExportModel{}, fmt.Errorf(
			"unsupported identity provider type for connection export: %s", dto.Type)
	}
	values, err := rawPropertyValues(dto.Properties)
	if err != nil {
		return connectionExportModel{}, err
	}

	model := connectionExportModel{
		ID:                     dto.ID,
		Type:                   vendor,
		Name:                   dto.Name,
		Description:            dto.Description,
		OUID:                   dto.OUID,
		ClientID:               values[idp.PropClientID],
		ClientSecret:           values[idp.PropClientSecret],
		RedirectURI:            values[idp.PropRedirectURI],
		Scopes:                 splitScopes(values[idp.PropScopes]),
		Prompt:                 values[idp.PropPrompt],
		AuthorizationEndpoint:  values[idp.PropAuthorizationEndpoint],
		TokenEndpoint:          values[idp.PropTokenEndpoint],
		UserInfoEndpoint:       values[idp.PropUserInfoEndpoint],
		JwksEndpoint:           values[idp.PropJwksEndpoint],
		Issuer:                 values[idp.PropIssuer],
		TrustedTokenAudience:   values[idp.PropTrustedTokenAudience],
		AttributeConfiguration: dto.AttributeConfiguration,
	}
	if raw, ok := values[idp.PropTokenExchangeEnabled]; ok {
		if enabled, parseErr := strconv.ParseBool(raw); parseErr == nil {
			model.TokenExchangeEnabled = &enabled
		}
	}
	return model, nil
}

// connectionModelFromSenderDTO builds the unified export model from a notification-sender DTO.
func connectionModelFromSenderDTO(dto ncommon.NotificationSenderDTO) (connectionExportModel, error) {
	vendor, ok := smsVendorName(dto.Provider)
	if !ok {
		if vendor, ok = emailVendorName(dto.Provider); !ok {
			return connectionExportModel{}, fmt.Errorf(
				"unsupported notification provider for connection export: %s", dto.Provider)
		}
	}
	values, err := rawPropertyValues(dto.Properties)
	if err != nil {
		return connectionExportModel{}, err
	}

	model := connectionExportModel{
		ID:          dto.ID,
		Type:        vendor,
		Name:        dto.Name,
		Description: dto.Description,
		OUID:        dto.OUID,
	}
	switch dto.Provider {
	case ncommon.NotificationProviderTypeTwilio:
		model.AccountSID = values[ncommon.TwilioPropKeyAccountSID]
		model.AuthToken = values[ncommon.TwilioPropKeyAuthToken]
		model.SenderID = values[ncommon.TwilioPropKeySenderID]
	case ncommon.NotificationProviderTypeVonage:
		model.APIKey = values[ncommon.VonagePropKeyAPIKey]
		model.APISecret = values[ncommon.VonagePropKeyAPISecret]
		model.SenderID = values[ncommon.VonagePropKeySenderID]
	case ncommon.NotificationProviderTypeCustom:
		model.URL = values[ncommon.CustomPropKeyURL]
		model.HTTPMethod = values[ncommon.CustomPropKeyHTTPMethod]
		auth := outboundauth.AuthenticationFromValues(values)
		headers, err := smsGatewayHeaderValue(dto.Properties)
		if err != nil {
			return connectionExportModel{}, err
		}
		model.HTTPHeaders = headers
		model.Authentication = &auth
		model.ContentType = values[ncommon.CustomPropKeyContentType]
	case ncommon.NotificationProviderTypeSMTP:
		model.Host = values[ncommon.SMTPPropKeyHost]
		model.Port, _ = strconv.Atoi(values[ncommon.SMTPPropKeyPort])
		model.FromAddress = values[ncommon.SMTPPropKeyFromAddress]
		model.FromName = values[ncommon.SMTPPropKeyFromName]
		model.TLS = values[ncommon.SMTPPropKeyTLS]
		auth := outboundauth.AuthenticationFromValues(values)
		model.Authentication = &auth
	}
	return model, nil
}

// connectionModelFromAuthZENPDP builds the unified export model from an AuthZEN PDP connection.
func connectionModelFromAuthZENPDP(connection authzenpdp.AuthZENPDPConnection) (connectionExportModel, error) {
	authenticationConfig, err := outboundauth.FromProperties(connection.AuthenticationProperties)
	if err != nil {
		return connectionExportModel{}, err
	}
	authentication := &outboundauth.Authentication{
		Type: string(authenticationConfig.Type), Properties: authenticationConfig.Properties,
	}
	return connectionExportModel{
		ID:                       connection.ID,
		Type:                     authZENPDPVendorName,
		Name:                     connection.Name,
		Description:              connection.Description,
		OUID:                     connection.OUID,
		AuthZENPDPEndpoint:       connection.Endpoint,
		AuthZENPDPBatchEndpoint:  connection.BatchEndpoint,
		TimeoutMS:                connection.TimeoutMS,
		AuthZENPDPRetryCount:     &connection.RetryCount,
		Authentication:           authentication,
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
	}, nil
}

// connectionModelFromHTTP builds the unified export model from an HTTP connection. Its secrets are
// carried in plain text for the parameterizer to externalize, as for an SMTP connection.
func connectionModelFromHTTP(connection httpconnection.HTTPConnection) (connectionExportModel, error) {
	values, err := rawPropertyValues(connection.AuthProperties)
	if err != nil {
		return connectionExportModel{}, err
	}
	auth := outboundauth.AuthenticationFromValues(values)
	return connectionExportModel{
		ID:             connection.ID,
		Type:           httpVendorName,
		Name:           connection.Name,
		Description:    connection.Description,
		OUID:           connection.OUID,
		URL:            connection.URL,
		TimeoutMS:      connection.TimeoutMS,
		Headers:        connection.Headers,
		Authentication: &auth,
	}, nil
}

// connectionModelToDTO converts a parsed connection document into the underlying
// identity-provider or notification-sender DTO, dispatching on the vendor discriminator.
// Exactly one of the two returned DTOs is non-nil.
func connectionModelToDTO(model connectionExportModel) (*providers.IDPDTO, *ncommon.NotificationSenderDTO, error) {
	switch model.Type {
	case "google":
		dto, err := googleToIDPDTO(googleConnectionRequest{
			Name: model.Name, Description: model.Description, OUID: model.OUID, ClientID: model.ClientID,
			ClientSecret: model.ClientSecret, RedirectURI: model.RedirectURI, Scopes: model.Scopes,
			Prompt: model.Prompt, JwksEndpoint: model.JwksEndpoint, Issuer: model.Issuer,
			TokenExchangeEnabled: model.TokenExchangeEnabled, AttributeConfiguration: model.AttributeConfiguration,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		dto.OUHandle = model.OUHandle
		return dto, nil, nil
	case "github":
		dto, err := githubToIDPDTO(githubConnectionRequest{
			Name: model.Name, Description: model.Description, OUID: model.OUID, ClientID: model.ClientID,
			ClientSecret: model.ClientSecret, RedirectURI: model.RedirectURI, Scopes: model.Scopes,
			Prompt: model.Prompt, AttributeConfiguration: model.AttributeConfiguration,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		dto.OUHandle = model.OUHandle
		return dto, nil, nil
	case "oidc":
		dto, err := oidcToIDPDTO(oidcConnectionRequest{
			Name: model.Name, Description: model.Description, OUID: model.OUID, ClientID: model.ClientID,
			ClientSecret: model.ClientSecret, RedirectURI: model.RedirectURI,
			AuthorizationEndpoint: model.AuthorizationEndpoint, TokenEndpoint: model.TokenEndpoint,
			UserInfoEndpoint: model.UserInfoEndpoint, JwksEndpoint: model.JwksEndpoint,
			Issuer: model.Issuer, Scopes: model.Scopes,
			Prompt: model.Prompt, TokenExchangeEnabled: model.TokenExchangeEnabled,
			TrustedTokenAudience: model.TrustedTokenAudience, AttributeConfiguration: model.AttributeConfiguration,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		dto.OUHandle = model.OUHandle
		return dto, nil, nil
	case "oauth":
		dto, err := oauthToIDPDTO(oauthConnectionRequest{
			Name: model.Name, Description: model.Description, OUID: model.OUID, ClientID: model.ClientID,
			ClientSecret: model.ClientSecret, RedirectURI: model.RedirectURI,
			AuthorizationEndpoint: model.AuthorizationEndpoint, TokenEndpoint: model.TokenEndpoint,
			UserInfoEndpoint: model.UserInfoEndpoint, Scopes: model.Scopes, Prompt: model.Prompt,
			AttributeConfiguration: model.AttributeConfiguration,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		dto.OUHandle = model.OUHandle
		return dto, nil, nil
	case "twilio":
		dto, err := twilioToSenderDTO(twilioConnectionRequest{
			Name: model.Name, Description: model.Description, OUID: model.OUID, AccountSID: model.AccountSID,
			AuthToken: model.AuthToken, SenderID: model.SenderID,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		dto.OUHandle = model.OUHandle
		return nil, dto, nil
	case "vonage":
		dto, err := vonageToSenderDTO(vonageConnectionRequest{
			Name: model.Name, Description: model.Description, OUID: model.OUID, APIKey: model.APIKey,
			APISecret: model.APISecret, SenderID: model.SenderID,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		dto.OUHandle = model.OUHandle
		return nil, dto, nil
	case smsGatewayVendorName:
		dto, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
			Name: model.Name, Description: model.Description, OUID: model.OUID, URL: model.URL,
			HTTPMethod: model.HTTPMethod, HTTPHeaders: &model.HTTPHeaders,
			Authentication: model.Authentication, ContentType: model.ContentType,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		dto.OUHandle = model.OUHandle
		return nil, dto, nil
	case emailSMTPVendorName:
		dto, err := emailSMTPToSenderDTO(emailSMTPConnectionRequest{
			Name: model.Name, Description: model.Description, OUID: model.OUID, Host: model.Host, Port: model.Port,
			FromAddress: model.FromAddress, FromName: model.FromName, TLS: model.TLS,
			Authentication: model.Authentication,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		dto.OUHandle = model.OUHandle
		return nil, dto, nil
	default:
		return nil, nil, fmt.Errorf("unsupported connection vendor: %s", model.Type)
	}
}

// connectionModelToAuthZENPDP converts the unified connection export model into an AuthZEN PDP connection.
func connectionModelToAuthZENPDP(model connectionExportModel) (*authzenpdp.AuthZENPDPConnection, error) {
	retryCount := -1
	if model.AuthZENPDPRetryCount != nil {
		retryCount = *model.AuthZENPDPRetryCount
	}
	connection, err := authzenpdp.ConnectionFromRequest(authzenpdp.ConnectionRequest{
		ID:                       model.ID,
		Name:                     model.Name,
		Description:              model.Description,
		OUID:                     model.OUID,
		OUHandle:                 model.OUHandle,
		Endpoint:                 model.AuthZENPDPEndpoint,
		BatchEndpoint:            model.AuthZENPDPBatchEndpoint,
		TimeoutMS:                model.TimeoutMS,
		RetryCount:               &retryCount,
		Authentication:           model.Authentication,
		SubjectAttributeMappings: model.SubjectAttributeMappings,
	}, config.AuthZENPDPConfig{})
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

// connectionModelToHTTP converts the export model into an HTTP connection, read as an update so an
// upsert may omit a secret the retention rule keeps.
func connectionModelToHTTP(model connectionExportModel) (*httpconnection.HTTPConnection, error) {
	connection, err := httpUpdateToDTO(httpConnectionRequest{
		Name: model.Name, Description: model.Description, OUID: model.OUID, URL: model.URL,
		TimeoutMS: model.TimeoutMS, Headers: model.Headers, Authentication: model.Authentication,
	})
	if err != nil {
		return nil, err
	}
	connection.ID = model.ID
	connection.OUHandle = model.OUHandle
	return connection, nil
}

// ParseConnectionFromNode decodes a yaml.Node into the DTO of the service that holds the document's
// vendor: an identity provider, a notification sender, an AuthZEN PDP or an HTTP connection. Used by
// the runtime import service.
func ParseConnectionFromNode(node *yaml.Node) (interface{}, error) {
	var model connectionExportModel
	if err := node.Decode(&model); err != nil {
		return nil, fmt.Errorf("failed to parse connection document: %w", err)
	}
	return connectionModelToResource(model)
}

// parseToConnectionDTOWrapper wraps connectionModelToResource to match ResourceConfig.Parser.
func parseToConnectionDTOWrapper(data []byte) (interface{}, error) {
	var model connectionExportModel
	if err := yaml.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	return connectionModelToResource(model)
}

// connectionModelToResource converts a parsed connection document into the DTO its vendor maps to.
func connectionModelToResource(model connectionExportModel) (interface{}, error) {
	switch model.Type {
	case authZENPDPVendorName:
		return connectionModelToAuthZENPDP(model)
	case httpVendorName:
		return connectionModelToHTTP(model)
	}
	idpDTO, senderDTO, err := connectionModelToDTO(model)
	if err != nil {
		return nil, err
	}
	if idpDTO != nil {
		return idpDTO, nil
	}
	return senderDTO, nil
}

// connectionResourceID extracts the ID from a parsed connection DTO, whichever concrete type
// it is.
func connectionResourceID(dto interface{}) string {
	switch d := dto.(type) {
	case *providers.IDPDTO:
		return d.ID
	case *ncommon.NotificationSenderDTO:
		return d.ID
	case *authzenpdp.AuthZENPDPConnection:
		return d.ID
	case *httpconnection.HTTPConnection:
		return d.ID
	default:
		return ""
	}
}

// validateConnectionDTOWrapper validates a parsed connection DTO before it is stored declaratively.
// IdP DTOs go through idp.ValidateIDP, the same required-property and type-default checks the live
// /connections create/update API runs. Email senders go through the same validation as the API,
// so a provider that cannot deliver fails the load rather than every send. Message senders only
// get a name presence check; their full semantic validation (e.g. a custom sender's required URL)
// is deferred to runtime use, matching the legacy declarative notification-sender behavior.
//
// A connection has to name its organization unit by ouId or ouHandle. The owner is resolved by the service of the
// connection's kind, the way the live API resolves it. A service may be nil, in which case the owner has to be given as
// an ouId and, for an identity provider, the schema-aware defaults the live API applies are skipped, so the declarative
// document stands entirely on its own.
func validateConnectionDTOWrapper(dto interface{}, idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface) error {
	switch d := dto.(type) {
	case *providers.IDPDTO:
		if d.Name == "" {
			return fmt.Errorf("connection resource %q is missing a name", d.ID)
		}
		if err := idp.ValidateIDP(d); err != nil {
			return err
		}
		if idpService != nil {
			if svcErr := idpService.ResolveIdentityProviderOU(declarativeOwnerContext(), d); svcErr != nil {
				return ownerError(d.ID, d.OUHandle, svcErr)
			}
			// Declarative resources load at startup with no authenticated subject, so the
			// entity-type reads the seeding performs would otherwise be authorized against nothing
			// and return nothing. Elevate here, where the absence of a subject is a fact about the
			// caller, rather than inside the service, where it would also bypass a real
			// administrator's scope on the REST path.
			idpService.ApplySchemaAwareDefaults(security.WithRuntimeContext(context.Background()), d)
		}
		return requireOwner(d.ID, d.OUID)
	case *ncommon.NotificationSenderDTO:
		if d.Name == "" {
			return fmt.Errorf("connection resource %q is missing a name", d.ID)
		}
		if d.Type == ncommon.NotificationSenderTypeEmail {
			if err := notification.ValidateNotificationSender(*d); err != nil {
				return fmt.Errorf("connection resource %q is invalid: %w", d.ID, err)
			}
		}
		if notificationService != nil {
			if svcErr := notificationService.ResolveSenderOU(declarativeOwnerContext(), d); svcErr != nil {
				return ownerError(d.ID, d.OUHandle, svcErr)
			}
		}
		return requireOwner(d.ID, d.OUID)
	case *authzenpdp.AuthZENPDPConnection:
		if d.Name == "" || d.Endpoint == "" {
			return fmt.Errorf("connection resource %q requires a name and endpoint", d.ID)
		}
		if authZENPDPService != nil {
			if svcErr := authZENPDPService.ResolveConnectionOU(declarativeOwnerContext(), d); svcErr != nil {
				return ownerError(d.ID, d.OUHandle, svcErr)
			}
		}
		return requireOwner(d.ID, d.OUID)
	case *httpconnection.HTTPConnection:
		if svcErr := httpService.ValidateHTTPConnection(context.Background(), d); svcErr != nil {
			return fmt.Errorf("connection resource %q is invalid: %s", d.ID, svcErr.ErrorDescription.DefaultValue)
		}
		if svcErr := httpService.ResolveConnectionOU(declarativeOwnerContext(), d); svcErr != nil {
			return ownerError(d.ID, d.OUHandle, svcErr)
		}
		return requireOwner(d.ID, d.OUID)
	}
	return nil
}

// declarativeOwnerContext returns the context the owner of a declarative connection is resolved with. Declarative
// resources load at startup with no authenticated subject, so the lookup runs with the runtime context.
func declarativeOwnerContext() context.Context {
	return security.WithRuntimeContext(context.Background())
}

// requireOwner fails a connection document that names no organization unit.
func requireOwner(id, ouID string) error {
	if ouID == "" {
		return fmt.Errorf("connection resource %q needs an organization unit, give its ouId or ouHandle", id)
	}
	return nil
}

// ownerError describes why the owner of a connection document could not be resolved.
func ownerError(id, ouHandle string, svcErr *tidcommon.ServiceError) error {
	if ouHandle != "" {
		return fmt.Errorf("organization unit %q of connection resource %q could not be resolved: %s",
			ouHandle, id, svcErr.ErrorDescription.DefaultValue)
	}
	return fmt.Errorf("organization unit of connection resource %q could not be resolved: %s",
		id, svcErr.ErrorDescription.DefaultValue)
}

// connectionDeclarativeStore dispatches parsed connections to their file-based stores.
type connectionDeclarativeStore struct {
	idpStore            *declarativeresource.GenericFileBasedStore
	senderStore         *declarativeresource.GenericFileBasedStore
	authZENPDPStore     *declarativeresource.GenericFileBasedStore
	httpConnectionStore *declarativeresource.GenericFileBasedStore
}

// Create implements declarativeresource.Storer, routing to the store matching the DTO type.
// Connections whose resolved store mode is mutable are skipped.
func (s *connectionDeclarativeStore) Create(id string, data interface{}) error {
	switch dto := data.(type) {
	case *providers.IDPDTO:
		if !idp.ShouldLoadDeclarativeIDPResources() {
			return nil
		}
		return s.idpStore.Create(id, dto)
	case *ncommon.NotificationSenderDTO:
		return s.senderStore.Create(id, dto)
	case *authzenpdp.AuthZENPDPConnection:
		if !authzenpdp.ShouldLoadDeclarativeAuthZENPDPResources() {
			return nil
		}
		dto.ID = id
		return s.authZENPDPStore.Create(id, dto)
	case *httpconnection.HTTPConnection:
		if !httpconnection.ShouldLoadDeclarativeHTTPConnectionResources() {
			return nil
		}
		dto.ID = id
		return s.httpConnectionStore.Create(id, dto)
	default:
		return fmt.Errorf("unsupported connection resource type: %T", data)
	}
}

// shouldLoadDeclarativeConnections reports whether any connection kind loads from resource files.
func shouldLoadDeclarativeConnections() bool {
	return declarativeresource.IsDeclarativeModeEnabled() ||
		idp.ShouldLoadDeclarativeIDPResources() ||
		authzenpdp.ShouldLoadDeclarativeAuthZENPDPResources() ||
		httpconnection.ShouldLoadDeclarativeHTTPConnectionResources()
}

// loadDeclarativeResources loads connection files when a connection file store is enabled.
func loadDeclarativeResources(idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface) error {
	if !shouldLoadDeclarativeConnections() {
		return nil
	}

	storer := &connectionDeclarativeStore{
		idpStore:        declarativeresource.NewGenericFileBasedStore(entity.KeyTypeIDP),
		senderStore:     declarativeresource.NewGenericFileBasedStore(entity.KeyTypeNotificationSender),
		authZENPDPStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeAuthZENPDP),
		httpConnectionStore: declarativeresource.NewGenericFileBasedStore(
			entity.KeyTypeHTTPConnection),
	}
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  paramTypeConnection,
		DirectoryName: "connections",
		Parser:        parseToConnectionDTOWrapper,
		Validator: func(dto interface{}) error {
			return validateConnectionDTOWrapper(dto, idpService, notificationService, authZENPDPService, httpService)
		},
		IDExtractor: connectionResourceID,
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, storer)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load connection resources: %w", err)
	}
	return nil
}

// ViewResource shows an exported connection as the read of its vendor's
// /connections/{type}/{id} route returns it, with secret properties masked as that read masks
// them. The vendor is taken from the document's type.
func (e *connectionExporter) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	resource, err := ParseConnectionFromNode(document)
	if err != nil {
		return nil, err
	}
	switch dto := resource.(type) {
	case *authzenpdp.AuthZENPDPConnection:
		return authzenpdp.ToResponse(*dto), nil
	case *httpconnection.HTTPConnection:
		return httpFromDTO(*dto)
	case *providers.IDPDTO:
		return viewIDPConnection(*dto)
	case *ncommon.NotificationSenderDTO:
		return viewSMSConnection(*dto)
	default:
		return nil, fmt.Errorf("unsupported connection resource type: %T", resource)
	}
}

// viewIDPConnection maps an identity-provider DTO with the mapper its vendor's read uses.
func viewIDPConnection(dto providers.IDPDTO) (interface{}, error) {
	switch dto.Type {
	case providers.IDPTypeGoogle:
		return googleFromIDPDTO(dto)
	case providers.IDPTypeGitHub:
		return githubFromIDPDTO(dto)
	case providers.IDPTypeOIDC:
		return oidcFromIDPDTO(dto)
	case providers.IDPTypeOAuth:
		return oauthFromIDPDTO(dto)
	default:
		return nil, fmt.Errorf("unsupported identity provider type for connection view: %s", dto.Type)
	}
}

// viewSMSConnection maps a notification-sender DTO with the mapper its vendor's read uses.
func viewSMSConnection(dto ncommon.NotificationSenderDTO) (interface{}, error) {
	switch dto.Provider {
	case ncommon.NotificationProviderTypeTwilio:
		return twilioFromSenderDTO(dto)
	case ncommon.NotificationProviderTypeVonage:
		return vonageFromSenderDTO(dto)
	case ncommon.NotificationProviderTypeCustom:
		return smsGatewayFromSenderDTO(dto)
	default:
		return nil, fmt.Errorf("unsupported message provider for connection view: %s", dto.Provider)
	}
}

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"testing"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauthn"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeConnection = "connection"
	paramTypeConnection    = "Connection"
	authZENPDPVendorName   = "authzen-pdp"
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
}

// newConnectionExporter creates a new connection exporter.
func newConnectionExporter(idpService idp.IDPServiceInterface,
	senderService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface) *connectionExporter {
	return &connectionExporter{
		idpService:        idpService,
		senderService:     senderService,
		authZENPDPService: authZENPDPService,
	}
}

// NewConnectionExporterForTest creates a new connection exporter for testing purposes.
func NewConnectionExporterForTest(idpService idp.IDPServiceInterface,
	senderService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface) *connectionExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newConnectionExporter(idpService, senderService, authZENPDPService)
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
		if sender.Type != ncommon.NotificationSenderTypeMessage {
			continue
		}
		if _, ok := smsVendorName(sender.Provider); ok {
			ids = append(ids, sender.ID)
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
		model.Name, resourceTypeConnection, id, "CONN_VALIDATION_ERROR", logger,
	); err != nil {
		return "", err
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
		return &declarativeresource.ResourceRules{Variables: []string{"ClientSecret"}}
	}

	switch model.Type {
	case "twilio":
		return &declarativeresource.ResourceRules{Variables: []string{"AuthToken"}}
	case "vonage":
		return &declarativeresource.ResourceRules{Variables: []string{"APISecret"}}
	case authZENPDPVendorName:
		if model.Authentication == nil {
			return &declarativeresource.ResourceRules{}
		}
		switch model.Authentication.Scheme {
		case outboundauthn.SchemeBearer:
			return &declarativeresource.ResourceRules{Variables: []string{"Authentication.BearerToken"}}
		case outboundauthn.SchemeAPIKey:
			return &declarativeresource.ResourceRules{
				Variables: []string{"Authentication.APIKeyHeaders[].Value"},
			}
		default:
			return &declarativeresource.ResourceRules{}
		}
	case smsGatewayVendorName:
		if len(model.APIKeyHeaders) == 0 {
			return &declarativeresource.ResourceRules{}
		}
		return &declarativeresource.ResourceRules{Variables: []string{"APIKeyHeaders[].Value"}}
	default:
		// Future vendors with no secrets have nothing to externalize.
		return &declarativeresource.ResourceRules{}
	}
}

// isIDPBackedVendorName reports whether name is a registered IdP-backed vendor's connection
// name (e.g. "google"), as opposed to an SMS-backed vendor name.
func isIDPBackedVendorName(name string) bool {
	for _, vendor := range idpBackedVendors {
		if vendor.name == name {
			return true
		}
	}
	return false
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
		return connectionExportModel{}, fmt.Errorf(
			"unsupported message provider for connection export: %s", dto.Provider)
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
		model.APIKeyHeaders, err = smsGatewayAPIKeyHeaders(dto.Properties, false)
		if err != nil {
			return connectionExportModel{}, err
		}
		model.ContentType = values[ncommon.CustomPropKeyContentType]
	}
	return model, nil
}

// connectionModelFromAuthZENPDP builds the unified export model from an AuthZEN PDP connection.
func connectionModelFromAuthZENPDP(connection authzenpdp.AuthZENPDPConnection) (connectionExportModel, error) {
	authenticationConfig, err := connection.OutboundAuthenticationConfig()
	if err != nil {
		return connectionExportModel{}, err
	}
	authentication := &authzenpdp.AuthenticationRequest{
		Scheme:      authenticationConfig.Scheme,
		BearerToken: authenticationConfig.BearerToken,
	}
	headerNames := make([]string, 0, len(authenticationConfig.APIKeyHeaders))
	for name := range authenticationConfig.APIKeyHeaders {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)
	for _, name := range headerNames {
		authentication.APIKeyHeaders = append(authentication.APIKeyHeaders, outboundauthn.APIKeyHeader{
			Name: name, Value: authenticationConfig.APIKeyHeaders[name],
		})
	}
	return connectionExportModel{
		ID:                       connection.ID,
		Type:                     authZENPDPVendorName,
		Name:                     connection.Name,
		Description:              connection.Description,
		AuthZENPDPEndpoint:       connection.Endpoint,
		AuthZENPDPBatchEndpoint:  connection.BatchEndpoint,
		AuthZENPDPTimeoutMS:      connection.TimeoutMS,
		AuthZENPDPRetryCount:     &connection.RetryCount,
		Authentication:           authentication,
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
	}, nil
}

// connectionModelToDTO converts a parsed connection document into the underlying
// identity-provider or notification-sender DTO, dispatching on the vendor discriminator.
// Exactly one of the two returned DTOs is non-nil.
func connectionModelToDTO(model connectionExportModel) (*providers.IDPDTO, *ncommon.NotificationSenderDTO, error) {
	switch model.Type {
	case "google":
		dto, err := googleToIDPDTO(googleConnectionRequest{
			Name: model.Name, Description: model.Description, ClientID: model.ClientID,
			ClientSecret: model.ClientSecret, RedirectURI: model.RedirectURI, Scopes: model.Scopes,
			Prompt: model.Prompt, JwksEndpoint: model.JwksEndpoint, Issuer: model.Issuer,
			TokenExchangeEnabled: model.TokenExchangeEnabled, AttributeConfiguration: model.AttributeConfiguration,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		return dto, nil, nil
	case "github":
		dto, err := githubToIDPDTO(githubConnectionRequest{
			Name: model.Name, Description: model.Description, ClientID: model.ClientID,
			ClientSecret: model.ClientSecret, RedirectURI: model.RedirectURI, Scopes: model.Scopes,
			Prompt: model.Prompt, AttributeConfiguration: model.AttributeConfiguration,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		return dto, nil, nil
	case "oidc":
		dto, err := oidcToIDPDTO(oidcConnectionRequest{
			Name: model.Name, Description: model.Description, ClientID: model.ClientID,
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
		return dto, nil, nil
	case "oauth":
		dto, err := oauthToIDPDTO(oauthConnectionRequest{
			Name: model.Name, Description: model.Description, ClientID: model.ClientID,
			ClientSecret: model.ClientSecret, RedirectURI: model.RedirectURI,
			AuthorizationEndpoint: model.AuthorizationEndpoint, TokenEndpoint: model.TokenEndpoint,
			UserInfoEndpoint: model.UserInfoEndpoint, Scopes: model.Scopes, Prompt: model.Prompt,
			AttributeConfiguration: model.AttributeConfiguration,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		return dto, nil, nil
	case "twilio":
		dto, err := twilioToSenderDTO(twilioConnectionRequest{
			Name: model.Name, Description: model.Description, AccountSID: model.AccountSID,
			AuthToken: model.AuthToken, SenderID: model.SenderID,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		return nil, dto, nil
	case "vonage":
		dto, err := vonageToSenderDTO(vonageConnectionRequest{
			Name: model.Name, Description: model.Description, APIKey: model.APIKey,
			APISecret: model.APISecret, SenderID: model.SenderID,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		return nil, dto, nil
	case smsGatewayVendorName:
		dto, err := smsGatewayToSenderDTO(smsGatewayConnectionRequest{
			Name: model.Name, Description: model.Description, URL: model.URL,
			HTTPMethod: model.HTTPMethod, APIKeyHeaders: model.APIKeyHeaders, ContentType: model.ContentType,
		})
		if err != nil {
			return nil, nil, err
		}
		dto.ID = model.ID
		return nil, dto, nil
	default:
		return nil, nil, fmt.Errorf("unsupported connection vendor: %s", model.Type)
	}
}

// connectionModelToAuthZENPDP converts the unified connection export model into an AuthZEN PDP connection.
func connectionModelToAuthZENPDP(model connectionExportModel) (*authzenpdp.AuthZENPDPConnection, error) {
	connection := authzenpdp.AuthZENPDPConnection{
		ID:                       model.ID,
		Name:                     model.Name,
		Description:              model.Description,
		Endpoint:                 model.AuthZENPDPEndpoint,
		BatchEndpoint:            model.AuthZENPDPBatchEndpoint,
		TimeoutMS:                model.AuthZENPDPTimeoutMS,
		RetryCount:               -1,
		SubjectAttributeMappings: model.SubjectAttributeMappings,
	}
	if model.AuthZENPDPRetryCount != nil {
		connection.RetryCount = *model.AuthZENPDPRetryCount
	}
	if err := connection.SetAuthentication(model.Authentication); err != nil {
		return nil, err
	}
	return &connection, nil
}

// ParseConnectionFromNode decodes a yaml.Node into the underlying identity-provider or
// notification-sender DTO, dispatching on the vendor discriminator. Used by the runtime import
// service. Exactly one of the two returned DTOs is non-nil.
func ParseConnectionFromNode(node *yaml.Node) (*providers.IDPDTO, *ncommon.NotificationSenderDTO, error) {
	var model connectionExportModel
	if err := node.Decode(&model); err != nil {
		return nil, nil, fmt.Errorf("failed to parse connection document: %w", err)
	}
	return connectionModelToDTO(model)
}

// ParseAuthZENPDPConnectionFromNode decodes an AuthZEN PDP connection document.
func ParseAuthZENPDPConnectionFromNode(node *yaml.Node) (*authzenpdp.AuthZENPDPConnection, error) {
	var model connectionExportModel
	if err := node.Decode(&model); err != nil {
		return nil, fmt.Errorf("failed to parse connection document: %w", err)
	}
	if model.Type != authZENPDPVendorName {
		return nil, nil
	}
	return connectionModelToAuthZENPDP(model)
}

// parseToConnectionDTOWrapper wraps connectionModelToDTO to match ResourceConfig.Parser,
// returning whichever of the two underlying DTOs the document's vendor maps to.
func parseToConnectionDTOWrapper(data []byte) (interface{}, error) {
	var model connectionExportModel
	if err := yaml.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	if model.Type == authZENPDPVendorName {
		return connectionModelToAuthZENPDP(model)
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
	default:
		return ""
	}
}

// validateConnectionDTOWrapper validates a parsed connection DTO before it is stored declaratively.
// IdP DTOs go through idp.ValidateIDP, the same required-property and type-default checks the live
// /connections create/update API runs. Notification-sender DTOs only get a name presence check —
// full semantic validation for senders (e.g. a custom sender's required URL) is deferred to
// runtime use, matching the legacy declarative notification-sender behavior.
//
// idpService may be nil, in which case the schema-aware defaults the live API applies are skipped
// and the declarative document stands entirely on its own.
func validateConnectionDTOWrapper(dto interface{}, idpService idp.IDPServiceInterface) error {
	switch d := dto.(type) {
	case *providers.IDPDTO:
		if d.Name == "" {
			return fmt.Errorf("connection resource %q is missing a name", d.ID)
		}
		if err := idp.ValidateIDP(d); err != nil {
			return err
		}
		if idpService != nil {
			// Declarative resources load at startup with no authenticated subject, so the
			// entity-type reads the seeding performs would otherwise be authorized against nothing
			// and return nothing. Elevate here, where the absence of a subject is a fact about the
			// caller, rather than inside the service, where it would also bypass a real
			// administrator's scope on the REST path.
			idpService.ApplySchemaAwareDefaults(security.WithRuntimeContext(context.Background()), d)
		}
		return nil
	case *ncommon.NotificationSenderDTO:
		if d.Name == "" {
			return fmt.Errorf("connection resource %q is missing a name", d.ID)
		}
	case *authzenpdp.AuthZENPDPConnection:
		if d.Name == "" || d.Endpoint == "" {
			return fmt.Errorf("connection resource %q requires a name and endpoint", d.ID)
		}
	}
	return nil
}

// connectionDeclarativeStore dispatches parsed connections to their file-based stores.
type connectionDeclarativeStore struct {
	idpStore        *declarativeresource.GenericFileBasedStore
	senderStore     *declarativeresource.GenericFileBasedStore
	authZENPDPStore *declarativeresource.GenericFileBasedStore
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
	default:
		return fmt.Errorf("unsupported connection resource type: %T", data)
	}
}

// loadDeclarativeResources loads connection files when a connection file store is enabled.
func loadDeclarativeResources(idpService idp.IDPServiceInterface) error {
	if !declarativeresource.IsDeclarativeModeEnabled() &&
		!idp.ShouldLoadDeclarativeIDPResources() &&
		!authzenpdp.ShouldLoadDeclarativeAuthZENPDPResources() {
		return nil
	}

	storer := &connectionDeclarativeStore{
		idpStore:        declarativeresource.NewGenericFileBasedStore(entity.KeyTypeIDP),
		senderStore:     declarativeresource.NewGenericFileBasedStore(entity.KeyTypeNotificationSender),
		authZENPDPStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeAuthZENPDP),
	}
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  paramTypeConnection,
		DirectoryName: "connections",
		Parser:        parseToConnectionDTOWrapper,
		Validator: func(dto interface{}) error {
			return validateConnectionDTOWrapper(dto, idpService)
		},
		IDExtractor: connectionResourceID,
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, storer)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load connection resources: %w", err)
	}
	return nil
}

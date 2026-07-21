/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package connection

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeConnection = "connection"
	paramTypeConnection    = "Connection"
)

// connectionExporter implements declarativeresource.ResourceExporter and
// declarativeresource.PerResourceRuler for connections, unifying the identity-provider and
// notification-sender services under the single "connection" declarative/export resource type
// that matches the /connections API and console — replacing the legacy "identity_provider" and
// "notification_sender" resource types.
type connectionExporter struct {
	idpService    idp.IDPServiceInterface
	senderService notification.NotificationSenderMgtSvcInterface
}

// newConnectionExporter creates a new connection exporter.
func newConnectionExporter(idpService idp.IDPServiceInterface,
	senderService notification.NotificationSenderMgtSvcInterface) *connectionExporter {
	return &connectionExporter{idpService: idpService, senderService: senderService}
}

// NewConnectionExporterForTest creates a new connection exporter for testing purposes.
func NewConnectionExporterForTest(idpService idp.IDPServiceInterface,
	senderService notification.NotificationSenderMgtSvcInterface) *connectionExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newConnectionExporter(idpService, senderService)
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
	if svcErr != nil {
		return nil, "", svcErr
	}
	model, err := connectionModelFromSenderDTO(*senderDTO)
	if err != nil {
		return nil, "", &tidcommon.InternalServerError
	}
	return &model, model.Name, nil
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
	default:
		// sms-gateway (and any future no-secret vendor) has nothing to externalize.
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
		LogoutEndpoint:         values[idp.PropLogoutEndpoint],
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
	case ncommon.MessageProviderTypeTwilio:
		model.AccountSID = values[ncommon.TwilioPropKeyAccountSID]
		model.AuthToken = values[ncommon.TwilioPropKeyAuthToken]
		model.SenderID = values[ncommon.TwilioPropKeySenderID]
	case ncommon.MessageProviderTypeVonage:
		model.APIKey = values[ncommon.VonagePropKeyAPIKey]
		model.APISecret = values[ncommon.VonagePropKeyAPISecret]
		model.SenderID = values[ncommon.VonagePropKeySenderID]
	case ncommon.MessageProviderTypeCustom:
		model.URL = values[ncommon.CustomPropKeyURL]
		model.HTTPMethod = values[ncommon.CustomPropKeyHTTPMethod]
		model.HTTPHeaders = values[ncommon.CustomPropKeyHTTPHeaders]
		model.ContentType = values[ncommon.CustomPropKeyContentType]
	}
	return model, nil
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
			LogoutEndpoint: model.LogoutEndpoint, Issuer: model.Issuer, Scopes: model.Scopes,
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
			UserInfoEndpoint: model.UserInfoEndpoint, LogoutEndpoint: model.LogoutEndpoint,
			Scopes: model.Scopes, Prompt: model.Prompt, AttributeConfiguration: model.AttributeConfiguration,
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
			HTTPMethod: model.HTTPMethod, HTTPHeaders: model.HTTPHeaders, ContentType: model.ContentType,
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

// parseToConnectionDTOWrapper wraps connectionModelToDTO to match ResourceConfig.Parser,
// returning whichever of the two underlying DTOs the document's vendor maps to.
func parseToConnectionDTOWrapper(data []byte) (interface{}, error) {
	var model connectionExportModel
	if err := yaml.Unmarshal(data, &model); err != nil {
		return nil, err
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
	default:
		return ""
	}
}

// validateConnectionDTOWrapper validates a parsed connection DTO before it is stored declaratively.
// IdP DTOs go through idp.ValidateIDP, the same required-property and type-default checks the live
// /connections create/update API runs. Notification-sender DTOs only get a name presence check —
// full semantic validation for senders (e.g. a custom sender's required URL) is deferred to
// runtime use, matching the legacy declarative notification-sender behavior.
func validateConnectionDTOWrapper(dto interface{}) error {
	switch d := dto.(type) {
	case *providers.IDPDTO:
		if d.Name == "" {
			return fmt.Errorf("connection resource %q is missing a name", d.ID)
		}
		return idp.ValidateIDP(d)
	case *ncommon.NotificationSenderDTO:
		if d.Name == "" {
			return fmt.Errorf("connection resource %q is missing a name", d.ID)
		}
	}
	return nil
}

// connectionDeclarativeStore dispatches a parsed connection resource to the identity-provider or
// notification-sender file-based backing store, based on the concrete DTO type returned by
// parseToConnectionDTOWrapper. Both target stores share their underlying storage with the ones
// the idp/notification services read via composite/declarative store modes — see
// declarativeresource.GenericFileBasedStore, keyed by entity.KeyTypeIDP / KeyTypeNotificationSender.
type connectionDeclarativeStore struct {
	idpStore    *declarativeresource.GenericFileBasedStore
	senderStore *declarativeresource.GenericFileBasedStore
}

// Create implements declarativeresource.Storer, routing to the store matching the DTO type.
// IdP-typed documents are skipped when the identity-provider package's own per-service store
// mode (identity_provider.store) resolves to mutable, even though the global declarative flag
// that gates loadDeclarativeResources is enabled.
func (s *connectionDeclarativeStore) Create(id string, data interface{}) error {
	switch dto := data.(type) {
	case *providers.IDPDTO:
		if !idp.ShouldLoadDeclarativeIDPResources() {
			return nil
		}
		return s.idpStore.Create(id, dto)
	case *ncommon.NotificationSenderDTO:
		return s.senderStore.Create(id, dto)
	default:
		return fmt.Errorf("unsupported connection resource type: %T", data)
	}
}

// loadDeclarativeResources loads declarative connection resources from config/resources/connections
// (or the single-file/root-dir equivalents), dispatching each parsed document to the
// identity-provider or notification-sender backing store by vendor. A no-op when neither the
// global declarative flag nor the identity-provider package's own per-service store mode
// (identity_provider.store) calls for loading, or when no connection files are present.
// connectionDeclarativeStore.Create further gates IdP-typed documents individually so a
// composite/declarative identity_provider.store is honored even when the global flag is off.
func loadDeclarativeResources() error {
	if !declarativeresource.IsDeclarativeModeEnabled() && !idp.ShouldLoadDeclarativeIDPResources() {
		return nil
	}

	storer := &connectionDeclarativeStore{
		idpStore:    declarativeresource.NewGenericFileBasedStore(entity.KeyTypeIDP),
		senderStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeNotificationSender),
	}
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  paramTypeConnection,
		DirectoryName: "connections",
		Parser:        parseToConnectionDTOWrapper,
		Validator:     validateConnectionDTOWrapper,
		IDExtractor:   connectionResourceID,
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, storer)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load connection resources: %w", err)
	}
	return nil
}

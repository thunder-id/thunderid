// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// declarativeTypeYAML has a YAML schema with a quoted numeric property name.
const declarativeTypeYAML = `id: type-file
type: payment_initiation
name: Payment initiation
description: Initiates a payment
schema:
  type: object
  required: [instructedAmount]
  properties:
    instructedAmount:
      type: object
      properties:
        amount: {type: string}
        currency: {type: string}
    "123":
      type: string
consentTemplate: "Pay {{instructedAmount.amount}} {{instructedAmount.currency}}"
processor:
  endpoint: https://bank.example.com/rar
reuse: false
`

type DeclarativeResourceTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// setupConfig initializes the server runtime with the given authorization details type store mode.
func (s *DeclarativeResourceTestSuite) setupConfig(home, store string, declarativeEnabled bool) {
	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(home, &config.Config{
		AuthorizationDetailType: config.AuthorizationDetailTypeConfig{Store: store},
		DeclarativeResources:    config.DeclarativeResources{Enabled: declarativeEnabled},
	}))
}

// The type store mode is its own setting: the resource server store mode does not decide it.
func (s *DeclarativeResourceTestSuite) TestStoreModeIsItsOwnSetting() {
	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{
		Resource: engineconfig.ResourceConfig{Store: "declarative"},
	}))
	s.Equal(serverconst.StoreModeMutable, getAuthorizationDetailTypeStoreMode())

	s.setupConfig(s.T().TempDir(), "Composite ", false)
	s.Equal(serverconst.StoreModeComposite, getAuthorizationDetailTypeStoreMode())
	s.setupConfig(s.T().TempDir(), "", true)
	s.Equal(serverconst.StoreModeDeclarative, getAuthorizationDetailTypeStoreMode())
}

func (s *DeclarativeResourceTestSuite) writeResource(home, content string) {
	dir := filepath.Join(home, "config", "resources", "authorization_detail_types")
	s.Require().NoError(os.MkdirAll(dir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "type.yaml"), []byte(content), 0o600))
}

func (s *DeclarativeResourceTestSuite) TestStoreMode() {
	cases := []struct {
		store       string
		declarative bool
		want        serverconst.StoreMode
	}{
		{"", false, serverconst.StoreModeMutable},
		{"", true, serverconst.StoreModeDeclarative},
		{"composite", false, serverconst.StoreModeComposite},
		{" Declarative ", false, serverconst.StoreModeDeclarative},
		{"mutable", true, serverconst.StoreModeMutable},
		{"unknown", true, serverconst.StoreModeDeclarative},
	}
	for _, tc := range cases {
		s.setupConfig("", tc.store, tc.declarative)
		s.Equal(tc.want, getAuthorizationDetailTypeStoreMode(), tc.store)
		s.Equal(tc.want == serverconst.StoreModeDeclarative, IsDeclarativeModeEnabled(), tc.store)
	}
}

func (s *DeclarativeResourceTestSuite) TestInitializeStore() {
	s.setupConfig("", "mutable", false)
	store, fileStore, dbStore := initializeStore()
	s.IsType(&typeStore{}, store)
	s.Nil(fileStore)
	s.Nil(dbStore)

	s.setupConfig("", "declarative", false)
	store, fileStore, dbStore = initializeStore()
	s.Equal(fileStore, store)
	s.NotNil(fileStore)
	s.Nil(dbStore)

	s.setupConfig("", "composite", false)
	store, fileStore, dbStore = initializeStore()
	s.IsType(&compositeTypeStore{}, store)
	s.NotNil(fileStore)
	s.NotNil(dbStore)
}

func (s *DeclarativeResourceTestSuite) TestInitialize_LoadsDeclarativeTypes() {
	home := s.T().TempDir()
	s.writeResource(home, declarativeTypeYAML)
	s.setupConfig(home, "declarative", false)
	s.Require().NoError(newTypeFileBasedStore().ClearByType())
	s.T().Cleanup(func() { _ = newTypeFileBasedStore().ClearByType() })
	service, exporter, err := Initialize(http.NewServeMux(), nil, nil)
	s.Require().NoError(err)
	s.Equal(resourceTypeAuthorizationDetailType, exporter.GetResourceType())

	t, svcErr := service.GetAuthorizationDetailType(s.ctx, "type-file")
	s.Require().Nil(svcErr)
	s.True(t.IsReadOnly)
	s.False(t.Reuse)
	s.Equal(1, t.Version)
	s.Equal("https://bank.example.com/rar", t.Processor.Endpoint)
	s.JSONEq(`{"type":"object","required":["instructedAmount"],"properties":{
		"instructedAmount":{"type":"object","properties":{"amount":{"type":"string"},"currency":{"type":"string"}}},
		"123":{"type":"string"}}}`, string(t.Schema))

	_, svcErr = service.CreateAuthorizationDetailType(s.ctx, apiType(*t))
	s.Equal(ErrorDeclarativeModeCreateNotAllowed.Code, svcErr.Code)
	_, svcErr = service.UpdateAuthorizationDetailType(s.ctx, "type-file", *t)
	s.Equal(ErrorImmutableType.Code, svcErr.Code)
	s.Equal(ErrorImmutableType.Code, service.DeleteAuthorizationDetailType(s.ctx, "type-file").Code)
}

func (s *DeclarativeResourceTestSuite) TestInitialize_FailsOnInvalidFile() {
	home := s.T().TempDir()
	s.writeResource(home, "id: type-file\ntype: payment_initiation\nname: Payment\n")
	s.setupConfig(home, "declarative", false)
	s.Require().NoError(newTypeFileBasedStore().ClearByType())

	_, _, err := Initialize(http.NewServeMux(), nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to load authorization details type resources")
}

func (s *DeclarativeResourceTestSuite) TestParse() {
	t, err := parseToTypeWrapper([]byte(declarativeTypeYAML))
	s.Require().NoError(err)
	parsed := t.(*providers.AuthorizationDetailType)
	s.Equal("type-file", parsed.ID)
	s.Equal("Initiates a payment", parsed.Description)
	s.False(parsed.Reuse)
	s.Equal(1, parsed.Version)
	s.Nil(validateTypeDefinition(*parsed), "the parsed type passes the API validation")
	s.Equal(`{"type":"object","required":["instructedAmount"],"properties":{"instructedAmount":`+
		`{"type":"object","properties":{"amount":{"type":"string"},"currency":{"type":"string"}}},`+
		`"123":{"type":"string"}}}`, string(parsed.Schema), "a YAML schema keeps the order it is written in")

	t, err = parseToTypeWrapper([]byte("id: a\ntype: t\nschema:\n  type: object\n  properties:\n" +
		"    zeta: &str {type: string}\n    alpha: *str\n    count: {type: number, minimum: 1, default: 2.5}\n" +
		"    flag: {type: boolean, default: true, examples: [null, 'x']}\n"))
	s.Require().NoError(err)
	s.Equal(`{"type":"object","properties":{"zeta":{"type":"string"},"alpha":{"type":"string"},`+
		`"count":{"type":"number","minimum":1,"default":2.5},`+
		`"flag":{"type":"boolean","default":true,"examples":[null,"x"]}}}`,
		string(t.(*providers.AuthorizationDetailType).Schema))

	t, err = parseToTypeWrapper([]byte("id: a\ntype: t\n" +
		"schema: '{\"type\": \"object\", \"properties\": {\"b\": {}, \"a\": {}}}'\n"))
	s.Require().NoError(err)
	s.Equal(`{"type": "object", "properties": {"b": {}, "a": {}}}`,
		string(t.(*providers.AuthorizationDetailType).Schema), "a JSON schema is kept as written")

	t, err = parseToTypeWrapper([]byte("id: a\ntype: t\nversion: 3\nschema: '{\"type\":\"object\"}'\n"))
	s.Require().NoError(err)
	parsed = t.(*providers.AuthorizationDetailType)
	s.Equal(1, parsed.Version, "a declarative type is always version 1")
	s.False(parsed.Reuse, "reuse defaults to false")
	s.JSONEq(`{"type":"object"}`, string(parsed.Schema))

	t, err = parseToTypeWrapper([]byte("id: a\ntype: t\nreuse: true\nschema: '{\"type\":\"object\"}'\n"))
	s.Require().NoError(err)
	s.True(t.(*providers.AuthorizationDetailType).Reuse)

	t, err = parseToTypeWrapper([]byte("id: a\ntype: t\n"))
	s.Require().NoError(err)
	s.Nil(t.(*providers.AuthorizationDetailType).Schema)

	t, err = parseToTypeWrapper([]byte("id: " + strings.Repeat("a", maxTypeIDLength) + "\ntype: t\n"))
	s.Require().NoError(err, "an id as wide as the store's ID column is accepted")
	s.Len(t.(*providers.AuthorizationDetailType).ID, maxTypeIDLength)

	for name, doc := range map[string]string{
		"malformed YAML":      "id: [unterminated",
		"wrong field type":    "id: [a, b]",
		"schema not JSON":     "id: a\nschema: 'not json'\n",
		"undecodable schema":  "id: a\nschema: !!binary '%%%'\n",
		"YAML of wrong shape": "- a\n- b\n",
		"id too long":         "id: " + strings.Repeat("a", maxTypeIDLength+1) + "\ntype: t\n",
	} {
		_, err := parseToTypeWrapper([]byte(doc))
		s.Error(err, name)
	}
}

func (s *DeclarativeResourceTestSuite) TestValidateTypeWrapper() {
	valid := func() *providers.AuthorizationDetailType {
		t := flightType()
		t.ID = "type-new"
		return &t
	}
	fileStore := newTestFileStore(s.T(), paymentType())

	s.NoError(validateTypeWrapper(valid(), fileStore, nil))
	s.Error(validateTypeWrapper("not a type", fileStore, nil))

	noID := valid()
	noID.ID = ""
	s.ErrorContains(validateTypeWrapper(noID, fileStore, nil), "ID is required")

	invalid := valid()
	invalid.ConsentTemplate = "Book {{passenger}}"
	s.ErrorContains(validateTypeWrapper(invalid, fileStore, nil),
		"{{passenger}} is not a field of the schema")

	sameID := valid()
	sameID.ID = paymentType().ID
	s.ErrorContains(validateTypeWrapper(sameID, fileStore, nil),
		"already exists in declarative resources")

	sameValue := valid()
	sameValue.Type = testPaymentType
	s.ErrorContains(validateTypeWrapper(sameValue, fileStore, nil),
		"already used in declarative resources")
}

func (s *DeclarativeResourceTestSuite) TestValidateTypeWrapper_DatabaseDuplicates() {
	fileStore := newTestFileStore(s.T())
	t := flightType()

	dbStore := newTypeStoreInterfaceMock(s.T())
	dbStore.EXPECT().GetTypeByID(mock.Anything, t.ID).Return(t, nil).Once()
	s.ErrorContains(validateTypeWrapper(&t, fileStore, dbStore), "already exists in the database store")

	dbStore.EXPECT().GetTypeByID(mock.Anything, t.ID).Return(providers.AuthorizationDetailType{}, errTypeNotFound)
	dbStore.EXPECT().GetTypeByValue(mock.Anything, t.Type).Return(t, nil).Once()
	s.ErrorContains(validateTypeWrapper(&t, fileStore, dbStore), "already used in the database store")

	dbStore.EXPECT().GetTypeByValue(mock.Anything, t.Type).Return(providers.AuthorizationDetailType{},
		errTypeNotFound).Once()
	s.NoError(validateTypeWrapper(&t, fileStore, dbStore))

	dbStore.EXPECT().GetTypeByValue(mock.Anything, t.Type).
		Return(providers.AuthorizationDetailType{}, errors.New("db down")).Once()
	s.ErrorContains(validateTypeWrapper(&t, fileStore, dbStore), "failed to check")

	failing := newTypeStoreInterfaceMock(s.T())
	failing.EXPECT().GetTypeByID(mock.Anything, t.ID).Return(providers.AuthorizationDetailType{},
		errors.New("db down")).Once()
	s.ErrorContains(validateTypeWrapper(&t, fileStore, failing), "failed to check")
}

func (s *DeclarativeResourceTestSuite) TestExporter() {
	service := NewAuthorizationDetailMgtServiceInterfaceMock(s.T())
	exporter := newTypeExporter(service)
	s.Equal("authorization_detail_type", exporter.GetResourceType())
	s.Equal("AuthorizationDetailType", exporter.GetParameterizerType())
	s.Nil(exporter.GetResourceRules())

	service.EXPECT().ListAllAuthorizationDetailTypes(mock.Anything).
		Return([]providers.AuthorizationDetailType{flightType(), paymentType()}, nil).Once()
	service.EXPECT().IsAuthorizationDetailTypeDeclarative(mock.Anything, "type-1").Return(false, nil).Once()
	service.EXPECT().IsAuthorizationDetailTypeDeclarative(mock.Anything, paymentType().ID).Return(true, nil).Once()
	ids, svcErr := exporter.GetAllResourceIDs(s.ctx)
	s.Nil(svcErr)
	s.Equal([]string{"type-1"}, ids, "declarative types are not exported")

	service.EXPECT().ListAllAuthorizationDetailTypes(mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr = exporter.GetAllResourceIDs(s.ctx)
	s.NotNil(svcErr)

	service.EXPECT().ListAllAuthorizationDetailTypes(mock.Anything).
		Return([]providers.AuthorizationDetailType{flightType()}, nil).Once()
	service.EXPECT().IsAuthorizationDetailTypeDeclarative(mock.Anything, "type-1").
		Return(false, &tidcommon.InternalServerError).Once()
	_, svcErr = exporter.GetAllResourceIDs(s.ctx)
	s.NotNil(svcErr)

	service.EXPECT().GetAuthorizationDetailType(mock.Anything, "missing").Return(nil, &ErrorTypeNotFound).Once()
	_, _, svcErr = exporter.GetResourceByID(s.ctx, "missing")
	s.Equal(ErrorTypeNotFound.Code, svcErr.Code)
	corrupt := flightType()
	corrupt.Schema = json.RawMessage(`{`)
	service.EXPECT().GetAuthorizationDetailType(mock.Anything, "corrupt").Return(&corrupt, nil).Once()
	_, _, svcErr = exporter.GetResourceByID(s.ctx, "corrupt")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	logger := log.GetLogger()
	_, exportErr := exporter.ValidateResource(s.ctx, "not a type", "x", logger)
	s.NotNil(exportErr)
	_, exportErr = exporter.ValidateResource(s.ctx, &typeDeclarativeResource{}, "x", logger)
	s.NotNil(exportErr, "a type without a name cannot be exported")
}

func (s *DeclarativeResourceTestSuite) TestExportRoundTrip() {
	original := flightType()
	original.Schema = json.RawMessage(`{"type":"object","properties":{"route":{"type":"string"},` +
		`"maxFare":{"type":"object","properties":{"currency":{"type":"string"},"amount":{"type":"number"}}},` +
		`"actions":{"type":"array","items":{"type":"string"}}}}`)
	original.ConsentTemplate = shortConsentTemplate
	original.Description = "Books a flight"
	original.Reuse = false
	service := NewAuthorizationDetailMgtServiceInterfaceMock(s.T())
	service.EXPECT().GetAuthorizationDetailType(mock.Anything, "type-1").Return(&original, nil).Once()

	exporter := newTypeExporter(service)
	resource, name, svcErr := exporter.GetResourceByID(s.ctx, "type-1")
	s.Require().Nil(svcErr)
	s.Equal(original.Name, name)
	validatedName, exportErr := exporter.ValidateResource(s.ctx, resource, "type-1", log.GetLogger())
	s.Require().Nil(exportErr)
	s.Equal(original.Name, validatedName)

	content, err := yaml.Marshal(resource)
	s.Require().NoError(err)

	loaded, err := parseToTypeWrapper(content)
	s.Require().NoError(err)
	s.assertSameType(original, *loaded.(*providers.AuthorizationDetailType))

	var doc yaml.Node
	s.Require().NoError(yaml.Unmarshal(append([]byte("resource_type: authorization_detail_type\n"), content...),
		&doc))
	imported, err := ParseAuthorizationDetailTypeNode(doc.Content[0])
	s.Require().NoError(err)
	s.assertSameType(original, *imported)

	viewer, ok := interface{}(exporter).(declarativeresource.ResourceViewer)
	s.Require().True(ok, "the applied configuration view can show an exported type")
	view, err := viewer.ViewResource(s.ctx, doc.Content[0])
	s.Require().NoError(err)
	shown, ok := view.(AuthorizationDetailTypeResponse)
	s.Require().True(ok)
	s.Equal(original.Type, shown.Type)
	s.Equal(original.Name, shown.Name)
	s.Equal(string(original.Schema), string(shown.Schema))
}

// assertSameType checks that a type read back from its declarative form matches the original.
func (s *DeclarativeResourceTestSuite) assertSameType(want, got providers.AuthorizationDetailType) {
	s.Equal(string(want.Schema), string(got.Schema), "the schema keeps the order it declares its properties in")
	want.Schema, got.Schema = nil, nil
	s.Equal(want, got)
}

func (s *DeclarativeResourceTestSuite) TestTypeExport_PutsProcessorCredentialsInSecretVariables() {
	withTestCrypto(s.T())
	typ := flightType()
	typ.ID = "type-1"
	typ.Processor = storedBearer(s.T())
	service := NewAuthorizationDetailMgtServiceInterfaceMock(s.T())
	service.EXPECT().GetAuthorizationDetailType(mock.Anything, "type-1").Return(&typ, nil)
	exporter := newTypeExporter(service)

	resource, _, svcErr := exporter.GetResourceByID(context.Background(), "type-1")
	s.Require().Nil(svcErr)
	exported := resource.(*typeDeclarativeResource)
	s.Require().NotNil(exported.Processor.Authentication)
	s.Equal(testProcessorToken, exported.Processor.Authentication.Properties[outboundauth.FieldBearerToken],
		"the export reads the credential to put it in a template variable")

	rules := exporter.GetResourceRulesForResource(resource)
	s.Require().NotNil(rules)
	s.Equal([]string{"Processor.Authentication.Properties." + outboundauth.FieldBearerToken},
		rules.SecretVariables)
	s.True(secretAllowedAt(exporter.SecretFields(),
		"processor.authentication.properties."+outboundauth.FieldBearerToken))
	s.False(secretAllowedAt(exporter.SecretFields(), "name"), "a secret outside a credential is refused")
	s.False(secretAllowedAt(exporter.SecretFields(),
		"processor.authentication.properties."+outboundauth.FieldBasicUsername),
		"a secret in a field shown in clear is refused")

	typ.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL}
	resource, _, svcErr = exporter.GetResourceByID(context.Background(), "type-1")
	s.Require().Nil(svcErr)
	s.Equal(&outboundauth.Authentication{Type: string(outboundauth.TypeNone)},
		resource.(*typeDeclarativeResource).Processor.Authentication, "a processor without credentials exports none")
	s.Nil(exporter.GetResourceRulesForResource(resource), "a processor without credentials has none")
}

// A processor that authenticates nothing exports type none, so an import clears a credential the target stores,
// while a document without the block keeps it.
func (s *DeclarativeResourceTestSuite) TestExportedNoneAuthenticationClearsTheStoredCredential() {
	withTestCrypto(s.T())
	typ := flightType()
	properties, svcErr := processorAuthenticationProperties(nil,
		&providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL}, nil)
	s.Require().Nil(svcErr)
	typ.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL,
		AuthenticationProperties: properties}
	service := NewAuthorizationDetailMgtServiceInterfaceMock(s.T())
	service.EXPECT().GetAuthorizationDetailType(mock.Anything, typ.ID).Return(&typ, nil)
	exporter := newTypeExporter(service)

	resource, _, getErr := exporter.GetResourceByID(context.Background(), typ.ID)
	s.Require().Nil(getErr)
	s.Nil(exporter.GetResourceRulesForResource(resource), "none has no credential to put in a variable")
	content, err := yaml.Marshal(resource)
	s.Require().NoError(err)
	var written struct {
		Processor struct {
			Authentication map[string]interface{} `yaml:"authentication"`
		} `yaml:"processor"`
	}
	s.Require().NoError(yaml.Unmarshal(content, &written))
	s.Equal(map[string]interface{}{"type": "none"}, written.Processor.Authentication)

	var exported yaml.Node
	s.Require().NoError(yaml.Unmarshal(content, &exported))
	imported, err := ParseAuthorizationDetailTypeNode(exported.Content[0])
	s.Require().NoError(err)
	s.Require().Nil(setProcessorAuthentication(imported, storedBearer(s.T())))
	config, err := outboundauth.FromProperties(imported.Processor.AuthenticationProperties)
	s.Require().NoError(err)
	s.False(config.Enabled(), "the imported none replaces the stored bearer credential")

	var withoutBlock yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte("id: type-1\ntype: flight_booking\nname: Flight\n"+
		"schema: '{\"type\":\"object\"}'\nprocessor:\n  endpoint: "+testProcessorURL+"\n"), &withoutBlock))
	kept, err := ParseAuthorizationDetailTypeNode(withoutBlock.Content[0])
	s.Require().NoError(err)
	s.Require().Nil(setProcessorAuthentication(kept, storedBearer(s.T())))
	s.Equal(testProcessorToken, storedToken(s.T(), kept.Processor.AuthenticationProperties),
		"a document without the block keeps the stored credential")
}

func (s *DeclarativeResourceTestSuite) TestParseAuthorizationDetailTypeNode_ReadsProcessorAuthentication() {
	withTestCrypto(s.T())
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte(`id: type-1
type: flight_booking
name: Flight
schema: '{"type":"object","properties":{"route":{"type":"string"}}}'
processor:
  endpoint: https://bank.example.com/rar/v1/process
  authentication:
    type: bearer
    properties:
      token: processor-token
`), &node))

	typ, err := ParseAuthorizationDetailTypeNode(node.Content[0])

	s.Require().NoError(err)
	s.Equal("https://bank.example.com/rar/v1/process", typ.Processor.Endpoint)
	s.Equal(testProcessorToken, storedToken(s.T(), typ.Processor.AuthenticationProperties),
		"the file's credential is carried encrypted for the service to resolve")
}

// The applied configuration view of a type shows its processor credential masked, as a read does.
func (s *DeclarativeResourceTestSuite) TestViewResource_MasksProcessorCredential() {
	withTestCrypto(s.T())
	var node yaml.Node
	s.Require().NoError(yaml.Unmarshal([]byte(`id: type-1
type: flight_booking
name: Flight
schema: '{"type":"object","properties":{"route":{"type":"string"}}}'
processor:
  endpoint: https://bank.example.com/rar/v1/process
  authentication:
    type: bearer
    properties:
      token: processor-token
`), &node))

	view, err := newTypeExporter(nil).ViewResource(context.Background(), node.Content[0])

	s.Require().NoError(err)
	shown, ok := view.(AuthorizationDetailTypeResponse)
	s.Require().True(ok)
	s.Require().NotNil(shown.Processor)
	s.Equal(string(outboundauth.TypeBearer), shown.Processor.Authentication.Type)
	s.Equal("******", shown.Processor.Authentication.Properties[outboundauth.FieldBearerToken])
	body, err := json.Marshal(view)
	s.Require().NoError(err)
	s.NotContains(string(body), testProcessorToken)
}

// An API key's credential is named by its header, and the reference its export writes there imports back.
func (s *DeclarativeResourceTestSuite) TestExportedAPIKeyCredentialImportsBack() {
	withTestCrypto(s.T())
	typ := flightType()
	properties, svcErr := processorAuthenticationProperties(&outboundauth.Authentication{
		Type: string(outboundauth.TypeAPIKey), Properties: map[string]string{"X-Api-Key": "the-key"}},
		&providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL}, nil)
	s.Require().Nil(svcErr)
	typ.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL,
		AuthenticationProperties: properties}
	service := NewAuthorizationDetailMgtServiceInterfaceMock(s.T())
	service.EXPECT().GetAuthorizationDetailType(mock.Anything, typ.ID).Return(&typ, nil)
	exporter := newTypeExporter(service)

	resource, _, getErr := exporter.GetResourceByID(context.Background(), typ.ID)
	s.Require().Nil(getErr)
	rules := exporter.GetResourceRulesForResource(resource)
	s.Require().NotNil(rules)
	s.Require().Len(rules.SecretVariables, 1)
	s.True(secretAllowedAt(exporter.SecretFields(), "processor.authentication.properties.X-Api-Key"))
}

// secretAllowedAt reports whether an import accepts a secret reference at a dotted field path, judged the way
// the secret resolver judges an imported document.
func secretAllowedAt(fields []string, path string) bool {
	parts := strings.Split(path, ".")
	doc := map[string]interface{}{}
	current := doc
	for _, part := range parts[:len(parts)-1] {
		next := map[string]interface{}{}
		current[part] = next
		current = next
	}
	current[parts[len(parts)-1]] = "sec:TEST_SECRET"
	encoded, err := yaml.Marshal(doc)
	if err != nil {
		return false
	}
	var node yaml.Node
	if err := yaml.Unmarshal(encoded, &node); err != nil {
		return false
	}
	return len(secretresolver.MisplacedSecrets(&node, fields)) == 0
}

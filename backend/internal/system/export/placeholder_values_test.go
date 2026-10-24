// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/agent"
	agentmodel "github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/application"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
)

// confidentialApp is an application as its create returns it: the client secret still in the clear.
func confidentialApp(clientID, clientSecret string) *providers.Application {
	return &providers.Application{
		ID:   "app-1",
		Name: "My App",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURIs: []string{"https://app.test/callback"},
			},
		}},
	}
}

func referenceExportService(t *testing.T,
	exporters ...declarativeresource.ResourceExporter) ExportServiceInterface {
	t.Helper()
	return newExportService(exporters, newParameterizer(templatingRules{}, ValueReferences))
}

// The names a capture keeps values under are the ones the reference export of the same application
// writes, so applying a captured version finds every value it refers to.
func TestPlaceholderValuesNameWhatTheReferenceExportWrites(t *testing.T) {
	appService := applicationmock.NewApplicationServiceInterfaceMock(t)
	// A read returns no secret: it is hashed when stored.
	appService.On("GetApplication", context.Background(), "app-1").Return(confidentialApp("the-id", ""), nil)
	svc := referenceExportService(t, application.NewApplicationExporterForTest(appService, sharingWithNoPolicies(t)))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		confidentialApp("the-id", "the-secret"))
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"APPLICATION_MY_APP_CLIENT_ID": "the-id",
		"APPLICATION_MY_APP_REDIRECT_URIS": `["https://app.test/callback"]`}, variables)
	assert.Equal(t, map[string]string{"APPLICATION_MY_APP_CLIENT_SECRET": "the-secret"}, secrets)

	exported, svcErr := svc.ExportResources(context.Background(),
		&ExportRequest{Applications: []string{"app-1"}})
	require.Nil(t, svcErr)
	require.Len(t, exported.Files, 1)
	for name := range variables {
		assert.Contains(t, exported.Files[0].Content, "var:"+name)
	}
	for name := range secrets {
		assert.Contains(t, exported.Files[0].Content, "sec:"+name)
	}
}

// A list is kept as the JSON list of its items, which the reference the export writes for it expands
// from on import, and it is never a credential.
func TestPlaceholderValuesReportAListAsAJSONList(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil, nil))
	app := confidentialApp("the-id", "the-secret")
	app.InboundAuthConfig[0].OAuthConfig.RedirectURIs = []string{"https://app.test/callback", "https://app.test/alt"}

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication, app)
	require.NoError(t, err)

	assert.Equal(t, `["https://app.test/callback","https://app.test/alt"]`,
		variables["APPLICATION_MY_APP_REDIRECT_URIS"])
	assert.NotContains(t, secrets, "APPLICATION_MY_APP_REDIRECT_URIS")
}

// An empty list, or none at all, as a client that signs no user in has, holds nothing a gateway
// could vary, so the export leaves it as it is and nothing is kept for it.
func TestPlaceholderValuesLeaveOutAnEmptyList(t *testing.T) {
	for _, list := range [][]string{{}, nil} {
		appService := applicationmock.NewApplicationServiceInterfaceMock(t)
		app := confidentialApp("the-id", "")
		app.InboundAuthConfig[0].OAuthConfig.RedirectURIs = list
		appService.On("GetApplication", context.Background(), "app-1").Return(app, nil)
		exporter := application.NewApplicationExporterForTest(appService, sharingWithNoPolicies(t))
		svc := referenceExportService(t, exporter)

		variables, _, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication, app)
		require.NoError(t, err)

		assert.NotContains(t, variables, "APPLICATION_MY_APP_REDIRECT_URIS")
		exported, svcErr := svc.ExportResources(context.Background(), &ExportRequest{Applications: []string{"app-1"}})
		require.Nil(t, svcErr)
		require.Len(t, exported.Files, 1)
		assert.NotContains(t, exported.Files[0].Content, "APPLICATION_MY_APP_REDIRECT_URIS")
	}
}

// A list holding a reference is exported item by item rather than referred to, so nothing is kept
// under its name, and no reference is copied into a store that holds values. That holds for a list
// of references alone, for one mixing a reference with literal items, and for a secret reference.
func TestPlaceholderValuesLeaveOutAListHoldingAReference(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil, nil))
	lists := map[string][]string{
		"a reference":                 {"var:APPLICATION_MY_APP_REDIRECT_URIS"},
		"a reference beside literals": {"https://app.test/callback", "var:APPLICATION_OTHER_REDIRECT_URIS"},
		"a secret reference":          {"sec:APPLICATION_MY_APP_REDIRECT_URIS"},
	}
	for holding, list := range lists {
		app := confidentialApp("the-id", "the-secret")
		app.InboundAuthConfig[0].OAuthConfig.RedirectURIs = list

		variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication, app)
		require.NoError(t, err)

		assert.NotContains(t, variables, "APPLICATION_MY_APP_REDIRECT_URIS", "a list holding %s was kept", holding)
		assert.NotContains(t, secrets, "APPLICATION_MY_APP_REDIRECT_URIS", "a list holding %s was kept", holding)
		assert.Equal(t, "the-id", variables["APPLICATION_MY_APP_CLIENT_ID"])
	}
}

// A public client has no secret, and its export names none.
func TestPlaceholderValuesOfAPublicClientCarryNoSecret(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil, nil))
	app := confidentialApp("the-id", "")
	app.InboundAuthConfig[0].OAuthConfig.PublicClient = true

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication, app)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"APPLICATION_MY_APP_CLIENT_ID": "the-id",
		"APPLICATION_MY_APP_REDIRECT_URIS": `["https://app.test/callback"]`}, variables)
	assert.Empty(t, secrets)
}

// An update that leaves the secret alone carries none, and an empty value is nothing to keep.
func TestPlaceholderValuesLeaveOutAnEmptyValue(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil, nil))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		confidentialApp("", ""))
	require.NoError(t, err)

	assert.NotContains(t, variables, "APPLICATION_MY_APP_CLIENT_ID")
	assert.Empty(t, secrets)
}

// A value that is already a reference is kept by the export as it stands. Keeping it in a store
// would put a reference where the value it names belongs.
func TestPlaceholderValuesLeaveOutAStoredReference(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil, nil))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		confidentialApp("var:APPLICATION_OLD_NAME_CLIENT_ID", "sec:APPLICATION_OLD_NAME_CLIENT_SECRET"))
	require.NoError(t, err)

	assert.NotContains(t, variables, "APPLICATION_MY_APP_CLIENT_ID")
	assert.Empty(t, secrets)
}

// An agent is named under its own type, so it cannot collide with an application of the same name.
func TestPlaceholderValuesOfAnAgent(t *testing.T) {
	svc := referenceExportService(t, agent.NewAgentExporterForTest(nil))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeAgent,
		&agentmodel.AgentGetResponse{
			ID:      "agent-1",
			Display: "My App",
			InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:     "agent-id",
					ClientSecret: "agent-secret",
				},
			}},
		})
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"AGENT_MY_APP_CLIENT_ID": "agent-id"}, variables)
	assert.Equal(t, map[string]string{"AGENT_MY_APP_CLIENT_SECRET": "agent-secret"}, secrets)
}

// A resource of a type nothing exports has no export to name its values after.
func TestPlaceholderValuesRefuseAnUnregisteredType(t *testing.T) {
	svc := referenceExportService(t)

	_, _, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication, confidentialApp("a", "b"))

	assert.Error(t, err)
}

// A resource in a shape its exporter does not read back would be named after nothing the export
// writes, so it is refused rather than guessed at.
func TestPlaceholderValuesRefuseAResourceOfTheWrongShape(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil, nil))

	_, _, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		&agentmodel.AgentGetResponse{Display: "My App"})

	assert.Error(t, err)
}

// userShaped mirrors the shape a user is exported in: attributes, and credentials held as a map
// beside them rather than as a slice of properties.
type userShaped struct {
	Name        string                 `yaml:"name"`
	Credentials map[string]interface{} `yaml:"credentials,omitempty"`
	ClaimValues map[string][]string    `yaml:"claimValues,omitempty"`
}

// Each string entry of a map under a dynamic property field is a credential, named the way the
// user exporter names its placeholder. A map of lists holds nothing one name could stand for.
func TestPlaceholderValuesReadCredentialsHeldAsAMap(t *testing.T) {
	rules := &declarativeresource.ResourceRules{DynamicPropertyFields: []string{"Credentials", "ClaimValues"}}

	variables, secrets, err := newParameterizer(templatingRules{}, ValueReferences).PlaceholderValues(
		context.Background(), &userShaped{
			Name: "alice@example.com",
			Credentials: map[string]interface{}{
				"password": "s3cret",
				"pin":      "sec:USER_ALICE_EXAMPLE_COM_PIN",
				"empty":    "",
			},
			ClaimValues: map[string][]string{"country": {"LK"}},
		}, "User", "alice@example.com", rules)
	require.NoError(t, err)

	assert.Empty(t, variables)
	assert.Equal(t, map[string]string{"USER_ALICE_EXAMPLE_COM_PASSWORD": "s3cret"}, secrets)
}

// A dynamic property is a credential or not by what it says of itself.
func TestPlaceholderValuesSplitDynamicPropertiesByWhetherTheyAreSecret(t *testing.T) {
	rules := &declarativeresource.ResourceRules{DynamicPropertyFields: []string{"Properties"}}

	variables, secrets, err := newParameterizer(templatingRules{}, ValueReferences).PlaceholderValues(
		context.Background(), &connectionWithProperties{
			Name: "My Connection",
			Properties: newProperties(t,
				propertySpec{name: "host", value: "https://idp.test"},
				propertySpec{name: "apiKey", value: "the-key", isSecret: true},
			),
		}, "Connection", "My Connection", rules)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"CONNECTION_MY_CONNECTION_HOST": "https://idp.test"}, variables)
	assert.Equal(t, map[string]string{"CONNECTION_MY_CONNECTION_API_KEY": "the-key"}, secrets)
}

// Without rules there is nothing an export would refer to.
func TestPlaceholderValuesWithoutRulesAreEmpty(t *testing.T) {
	variables, secrets, err := newParameterizer(templatingRules{}, ValueReferences).PlaceholderValues(
		context.Background(), &userShaped{Name: "x"}, "User", "x", nil)
	require.NoError(t, err)

	assert.Empty(t, variables)
	assert.Empty(t, secrets)
}

// A resource server's identifier is captured under the name its export writes, and one that is
// already a reference is not captured again.
func TestPlaceholderValuesOfAResourceServer(t *testing.T) {
	p := newParameterizer(templatingRules{}, ValueReferences)
	rs := &providers.ResourceServer{Name: "Orders API", Identifier: "https://orders.example.com", Delimiter: ":"}

	variables, secrets, err := p.PlaceholderValues(context.Background(), rs, "ResourceServer", rs.Name,
		resourceServerRules)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"RESOURCE_SERVER_ORDERS_API_IDENTIFIER": "https://orders.example.com"},
		variables)
	assert.Empty(t, secrets)

	rs.Identifier = "var:RESOURCE_SERVER_ORDERS_API_IDENTIFIER"
	variables, secrets, err = p.PlaceholderValues(context.Background(), rs, "ResourceServer", rs.Name,
		resourceServerRules)
	require.NoError(t, err)
	assert.Empty(t, variables)
	assert.Empty(t, secrets)
}

// The fields an exporter says may hold a secret are the YAML side of its SecretVariables: where its
// reference export writes "sec:", and so the only places an import may put a secret.
func TestSecretFieldsMatchWhereTheExportWritesSecrets(t *testing.T) {
	p := newParameterizer(templatingRules{}, ValueReferences)
	for name, tc := range map[string]struct {
		exporter declarativeresource.ResourceExporter
		model    interface{}
	}{
		"application": {application.NewApplicationExporterForTest(nil, nil), &providers.Application{}},
		"agent":       {agent.NewAgentExporterForTest(nil), &agentmodel.AgentGetResponse{}},
	} {
		lister, ok := tc.exporter.(declarativeresource.SecretFieldLister)
		require.True(t, ok, name)
		for _, path := range tc.exporter.GetResourceRules().SecretVariables {
			assert.Contains(t, lister.SecretFields(), p.convertPathToYAMLPath(reflect.TypeOf(tc.model), path), name)
		}
	}
}

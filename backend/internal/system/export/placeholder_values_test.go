// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"context"
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
	svc := referenceExportService(t, application.NewApplicationExporterForTest(appService))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		confidentialApp("the-id", "the-secret"))
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"APPLICATION_MY_APP_CLIENT_ID": "the-id"}, variables)
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

// A list stays literal in a reference export, so there is nothing to keep for it.
func TestPlaceholderValuesLeaveOutAList(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		confidentialApp("the-id", "the-secret"))
	require.NoError(t, err)

	assert.NotContains(t, variables, "APPLICATION_MY_APP_REDIRECT_URIS")
	assert.NotContains(t, secrets, "APPLICATION_MY_APP_REDIRECT_URIS")
}

// A public client has no secret, and its export names none.
func TestPlaceholderValuesOfAPublicClientCarryNoSecret(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil))
	app := confidentialApp("the-id", "")
	app.InboundAuthConfig[0].OAuthConfig.PublicClient = true

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication, app)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"APPLICATION_MY_APP_CLIENT_ID": "the-id"}, variables)
	assert.Empty(t, secrets)
}

// An update that leaves the secret alone carries none, and an empty value is nothing to keep.
func TestPlaceholderValuesLeaveOutAnEmptyValue(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		confidentialApp("", ""))
	require.NoError(t, err)

	assert.Empty(t, variables)
	assert.Empty(t, secrets)
}

// A value that is already a reference is kept by the export as it stands. Keeping it in a store
// would put a reference where the value it names belongs.
func TestPlaceholderValuesLeaveOutAStoredReference(t *testing.T) {
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		confidentialApp("var:APPLICATION_OLD_NAME_CLIENT_ID", "sec:APPLICATION_OLD_NAME_CLIENT_SECRET"))
	require.NoError(t, err)

	assert.Empty(t, variables)
	assert.Empty(t, secrets)
}

// An agent is named under its own type, so it cannot collide with an application of the same name.
func TestPlaceholderValuesOfAnAgent(t *testing.T) {
	svc := referenceExportService(t, agent.NewAgentExporterForTest(nil))

	variables, secrets, err := svc.PlaceholderValues(context.Background(), resourceTypeAgent,
		&agentmodel.AgentGetResponse{
			ID:   "agent-1",
			Name: "My App",
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
	svc := referenceExportService(t, application.NewApplicationExporterForTest(nil))

	_, _, err := svc.PlaceholderValues(context.Background(), resourceTypeApplication,
		&agentmodel.AgentGetResponse{Name: "My App"})

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

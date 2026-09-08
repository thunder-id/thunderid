// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTemplate(t *testing.T) {
	content := "name: test\nclientId: {{.CLIENT_ID}}\n"

	resolved, err := resolveTemplate(content, map[string]interface{}{"CLIENT_ID": "abc"})
	require.NoError(t, err)
	assert.Contains(t, resolved, "clientId: abc")
}

func TestResolveTemplate_MissingKey(t *testing.T) {
	content := "clientId: {{.CLIENT_ID}}\n"

	_, err := resolveTemplate(content, map[string]interface{}{})
	require.Error(t, err)
}

func TestResolveTemplate_PreservesLiteralExpressions(t *testing.T) {
	content := "label: {{ t(signin:forms.credentials.title) }}\n" +
		"src: {{ meta(application.logoUrl) }}\n"

	resolved, err := resolveTemplate(content, nil)
	require.NoError(t, err)
	assert.Contains(t, resolved, "{{ t(signin:forms.credentials.title) }}")
	assert.Contains(t, resolved, "{{ meta(application.logoUrl) }}")
}

func TestResolveTemplate_PreservesHelperStyleLiteralExpression(t *testing.T) {
	content := "title: {{ appName }}\n"

	resolved, err := resolveTemplate(content, nil)
	require.NoError(t, err)
	assert.Contains(t, resolved, "{{ appName }}")
}

func TestResolveTemplate_PreservesHelperStyleLiteralExpressionWithArgs(t *testing.T) {
	content := "title: {{ appName application.name }}\n"

	resolved, err := resolveTemplate(content, nil)
	require.NoError(t, err)
	assert.Contains(t, resolved, "{{ appName application.name }}")
}

func TestResolveTemplate_ResolvesVariablesAndRangeWhilePreservingLiterals(t *testing.T) {
	content := "clientId: {{.CLIENT_ID}}\n" +
		"redirectUris:\n" +
		"{{- range .REDIRECT_URIS}}\n" +
		"- {{.}}\n" +
		"{{- end}}\n" +
		"label: {{ t(signin:forms.credentials.title) }}\n"

	resolved, err := resolveTemplate(content, map[string]interface{}{
		"CLIENT_ID":     "console",
		"REDIRECT_URIS": []string{"https://localhost:8090/console", "https://localhost:3000/callback"},
	})
	require.NoError(t, err)
	assert.Contains(t, resolved, "clientId: console")
	assert.Contains(t, resolved, "- https://localhost:8090/console")
	assert.Contains(t, resolved, "- https://localhost:3000/callback")
	assert.Contains(t, resolved, "{{ t(signin:forms.credentials.title) }}")
}

func TestResolveTemplate_DoesNotCollideWithLiteralPlaceholderLookingText(t *testing.T) {
	content := strings.Join([]string{
		"name: test",
		"existing: __LITERAL_TEMPLATE_EXPR_0__",
		"label: {{ t(signin:forms.credentials.title) }}",
		"",
	}, "\n")

	resolved, err := resolveTemplate(content, nil)
	require.NoError(t, err)
	assert.Contains(t, resolved, "existing: __LITERAL_TEMPLATE_EXPR_0__")
	assert.Contains(t, resolved, "{{ t(signin:forms.credentials.title) }}")
}

func TestParseDocuments(t *testing.T) {
	content := strings.Join([]string{
		"resource_type: application",
		"name: app-one",
		"authFlowId: flow-1",
		"---",
		"resource_type: connection",
		"name: idp-one",
		"type: google",
		"clientId: abc",
		"---",
		"resource_type: flow",
		"id: flow-1",
		"handle: login",
		"name: Login Flow",
		"flowType: AUTHENTICATION",
		"nodes: []",
		"",
	}, "\n")

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 3)
	assert.Equal(t, resourceTypeApplication, docs[0].ResourceType)
	assert.Equal(t, resourceTypeConnection, docs[1].ResourceType)
	assert.Equal(t, resourceTypeFlow, docs[2].ResourceType)
}

func TestParseDocuments_AgentDocument(t *testing.T) {
	content := strings.Join([]string{
		"resource_type: agent",
		"id: agt-1",
		"ouId: ou-1",
		"type: default",
		"name: Test Agent",
		"owner: owner-id-1",
		"",
	}, "\n")

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, resourceTypeAgent, docs[0].ResourceType)
}

func TestParseDocuments_AgentWithOAuthNotClassifiedAsApplication(t *testing.T) {
	content := strings.Join([]string{
		"resource_type: agent",
		"id: agt-2",
		"ouId: ou-1",
		"type: default",
		"name: OAuth Agent",
		"owner: owner-id-1",
		"authFlowId: flow-1",
		"inboundAuthConfig:",
		"- type: oauth2",
		"  config:",
		"    clientId: client-1",
		"",
	}, "\n")

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, resourceTypeAgent, docs[0].ResourceType)
}

func TestParseDocuments_AgentTypeDocument(t *testing.T) {
	content := strings.Join([]string{
		"resource_type: agent_type",
		"id: at-1",
		"name: Test Agent Type",
		"schema: '{}'",
		"",
	}, "\n")

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, resourceTypeAgentType, docs[0].ResourceType)
}

func TestParseDocuments_UsesResourceTypeField(t *testing.T) {
	content := "resource_type: application\nname: idp-one\ntype: GOOGLE\nproperties: []\n"

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, resourceTypeApplication, docs[0].ResourceType)
}

func TestParseDocuments_ResourceTypeFieldTakesPrecedenceOverStructure(t *testing.T) {
	content := "resource_type: application\nname: app-one\nauth_flow_id: flow-1\n"

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, resourceTypeApplication, docs[0].ResourceType)
}

func TestParseDocuments_FailsForNonMappingRoot(t *testing.T) {
	content := strings.Join([]string{
		"- item1",
		"- item2",
		"",
	}, "\n")

	_, err := parseDocuments(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root must be a YAML mapping")
}

func TestParseDocuments_FailsForUnknownResourceType(t *testing.T) {
	content := strings.Join([]string{
		"foo: unknown-resource",
		"bar: unknown-value",
		"extra: value",
		"",
	}, "\n")

	_, err := parseDocuments(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to determine resource type")
}

func TestParseDocuments_FailsForInvalidResourceType(t *testing.T) {
	content := "resource_type: not_a_real_type\nname: something\n"

	_, err := parseDocuments(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to determine resource type")
}

func TestParseDocuments_ExplicitResourceTypeOverridesAmbiguousStructure(t *testing.T) {
	content := strings.Join([]string{
		"resource_type: connection",
		"name: google",
		"type: google",
		"permissions: []",
		"clientId: abc",
		"",
	}, "\n")

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, resourceTypeConnection, docs[0].ResourceType)
}

// A deployment holds its own credentials as environment variables, so a configuration can travel
// without them and still resolve where it lands.
func TestResolveTemplateFillsAMissingVariableFromTheEnvironment(t *testing.T) {
	t.Setenv("APP_SECRET", "from-the-environment")

	resolved, err := resolveTemplate(`secret: "{{.APP_SECRET}}"`, nil)
	require.NoError(t, err)
	assert.Equal(t, `secret: "from-the-environment"`, resolved)
}

// What the caller supplies wins, so an apply that knows the value is not overridden by whatever the
// deployment happens to have in its environment.
func TestResolveTemplatePrefersTheSuppliedVariable(t *testing.T) {
	t.Setenv("APP_URL", "from-the-environment")

	resolved, err := resolveTemplate(`url: "{{.APP_URL}}"`,
		map[string]interface{}{"APP_URL": "from-the-caller"})
	require.NoError(t, err)
	assert.Equal(t, `url: "from-the-caller"`, resolved)
}

// A placeholder nobody can supply still fails, rather than resolving to nothing.
func TestResolveTemplateStillFailsWhenNothingSuppliesTheVariable(t *testing.T) {
	_, err := resolveTemplate(`secret: "{{.NOBODY_HAS_THIS}}"`, nil)
	assert.Error(t, err, "an unresolvable placeholder should fail the import")
}

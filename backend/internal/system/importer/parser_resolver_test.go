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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTemplate(t *testing.T) {
	content := "name: test\nclient_id: {{.CLIENT_ID}}\n"

	resolved, err := resolveTemplate(content, map[string]interface{}{"CLIENT_ID": "abc"})
	require.NoError(t, err)
	assert.Contains(t, resolved, "client_id: abc")
}

func TestResolveTemplate_MissingKey(t *testing.T) {
	content := "client_id: {{.CLIENT_ID}}\n"

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
	content := "client_id: {{.CLIENT_ID}}\n" +
		"redirect_uris:\n" +
		"{{- range .REDIRECT_URIS}}\n" +
		"- {{.}}\n" +
		"{{- end}}\n" +
		"label: {{ t(signin:forms.credentials.title) }}\n"

	resolved, err := resolveTemplate(content, map[string]interface{}{
		"CLIENT_ID":     "console",
		"REDIRECT_URIS": []string{"https://localhost:8090/console", "https://localhost:3000/callback"},
	})
	require.NoError(t, err)
	assert.Contains(t, resolved, "client_id: console")
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
		"name: app-one",
		"auth_flow_id: flow-1",
		"---",
		"name: idp-one",
		"type: GOOGLE",
		"properties:",
		"- name: client_id",
		"  value: abc",
		"---",
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
	assert.Equal(t, resourceTypeIdentityProvider, docs[1].ResourceType)
	assert.Equal(t, resourceTypeFlow, docs[2].ResourceType)
}

func TestClassifyResourceType_AdditionalResources(t *testing.T) {
	testCases := []struct {
		name     string
		yamlDoc  string
		expected string
	}{
		{
			name:     "organization unit",
			yamlDoc:  "id: ou-1\nhandle: root\nname: Root\n",
			expected: resourceTypeOrganizationUnit,
		},
		{
			name:     "user type with organization_unit_id",
			yamlDoc:  "id: sch-1\nname: Schema\norganization_unit_id: ou-1\nschema: '{}'\n",
			expected: resourceTypeEntityType,
		},
		{
			name:     "user type with ou_handle",
			yamlDoc:  "id: sch-1\nname: Schema\nou_handle: customers\nschema: '{}'\n",
			expected: resourceTypeEntityType,
		},
		{
			name:     "resource server",
			yamlDoc:  "id: rs-1\nidentifier: api://rs-1\ndelimiter: ':'\nresources: []\n",
			expected: resourceTypeResourceServer,
		},
		{name: "role", yamlDoc: "id: role-1\nname: Admin\npermissions: []\n", expected: resourceTypeRole},
		{name: "theme", yamlDoc: "id: th-1\ndisplayName: Theme\ntheme: {}\n", expected: resourceTypeTheme},
		{name: "layout", yamlDoc: "id: ly-1\ndisplayName: Layout\nlayout: {}\n", expected: resourceTypeLayout},
		{name: "user", yamlDoc: "id: u-1\ntype: person\nou_id: ou-1\nattributes: {}\n", expected: resourceTypeUser},
		{name: "translation", yamlDoc: "language: en-US\ntranslations: {}\n", expected: resourceTypeTranslation},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			docs, err := parseDocuments(testCase.yamlDoc)
			require.NoError(t, err)
			require.Len(t, docs, 1)
			assert.Equal(t, testCase.expected, docs[0].ResourceType)
		})
	}
}

func TestParseDocuments_UsesResourceTypeComment(t *testing.T) {
	content := "# resource_type: application\nname: idp-one\ntype: GOOGLE\nproperties: []\n"

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, resourceTypeApplication, docs[0].ResourceType)
}

func TestParseDocuments_UsesResourceTypeCommentWithFileHeader(t *testing.T) {
	content := "# File: app-one.yaml\n# resource_type: application\nname: app-one\nauth_flow_id: flow-1\n"

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

func TestParseDocuments_FailsForAmbiguousResourceType(t *testing.T) {
	content := strings.Join([]string{
		"name: maybe-role-or-idp",
		"type: GOOGLE",
		"permissions: []",
		"properties: []",
		"",
	}, "\n")

	_, err := parseDocuments(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to determine resource type")
}

func TestParseDocuments_ExplicitResourceTypeOverridesAmbiguousHeuristics(t *testing.T) {
	content := strings.Join([]string{
		"# resource_type: identity_provider",
		"name: google",
		"type: GOOGLE",
		"permissions: []",
		"properties: []",
		"",
	}, "\n")

	docs, err := parseDocuments(content)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, resourceTypeIdentityProvider, docs[0].ResourceType)
}

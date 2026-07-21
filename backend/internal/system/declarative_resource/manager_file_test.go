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

package declarativeresource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// ResourcesFileTestSuite tests the single-file declarative resource loading functions:
// GetConfigsFromFile, splitYAMLDocuments, and documentMatchesResourceType.
type ResourcesFileTestSuite struct {
	suite.Suite
	originalEnvVars map[string]string
}

func TestResourcesFileTestSuite(t *testing.T) {
	suite.Run(t, new(ResourcesFileTestSuite))
}

func (s *ResourcesFileTestSuite) SetupTest() {
	s.originalEnvVars = make(map[string]string)
}

func (s *ResourcesFileTestSuite) TearDownTest() {
	for key, value := range s.originalEnvVars {
		if value == "" {
			_ = os.Unsetenv(key)
		} else {
			_ = os.Setenv(key, value)
		}
	}
}

func (s *ResourcesFileTestSuite) setEnvVar(key, value string) {
	if _, exists := s.originalEnvVars[key]; !exists {
		if orig, has := os.LookupEnv(key); has {
			s.originalEnvVars[key] = orig
		} else {
			s.originalEnvVars[key] = ""
		}
	}
	s.Require().NoError(os.Setenv(key, value))
}

func (s *ResourcesFileTestSuite) writeResourcesFile(content string) string {
	f, err := os.CreateTemp(s.T().TempDir(), "resources-*.yaml")
	s.Require().NoError(err)
	_, err = f.WriteString(content)
	s.Require().NoError(err)
	s.Require().NoError(f.Close())
	return f.Name()
}

// ─── splitYAMLDocuments ──────────────────────────────────────────────────────

func (s *ResourcesFileTestSuite) TestSplitYAMLDocuments_SingleDocument() {
	content := []byte("key: value\nother: 123\n")
	docs := splitYAMLDocuments(content)
	s.Len(docs, 1)
	s.Contains(string(docs[0]), "key: value")
}

func (s *ResourcesFileTestSuite) TestSplitYAMLDocuments_MultipleDocuments() {
	content := []byte("resource_type: identity_provider\nname: idp1\n---\nresource_type: flow\nhandle: flow1\n")
	docs := splitYAMLDocuments(content)
	s.Len(docs, 2)
	s.Contains(string(docs[0]), "identity_provider")
	s.Contains(string(docs[1]), "flow1")
}

func (s *ResourcesFileTestSuite) TestSplitYAMLDocuments_LeadingSeparator() {
	content := []byte("---\nresource_type: identity_provider\nname: idp1\n")
	docs := splitYAMLDocuments(content)
	s.Len(docs, 1)
	s.Contains(string(docs[0]), "identity_provider")
}

func (s *ResourcesFileTestSuite) TestSplitYAMLDocuments_EmptyContent() {
	docs := splitYAMLDocuments([]byte{})
	s.Empty(docs)
}

func (s *ResourcesFileTestSuite) TestSplitYAMLDocuments_OnlyWhitespace() {
	docs := splitYAMLDocuments([]byte("  \n  \n---\n  \n"))
	s.Empty(docs)
}

// ─── documentMatchesResourceType ─────────────────────────────────────────────

func (s *ResourcesFileTestSuite) TestDocumentMatchesResourceType_Match() {
	doc := []byte("resource_type: identity_provider\nname: my-idp\n")
	s.True(documentMatchesResourceType(doc, "identity_provider"))
}

func (s *ResourcesFileTestSuite) TestDocumentMatchesResourceType_NoMatch() {
	doc := []byte("resource_type: flow\nhandle: my-flow\n")
	s.False(documentMatchesResourceType(doc, "identity_provider"))
}

func (s *ResourcesFileTestSuite) TestDocumentMatchesResourceType_NoField() {
	doc := []byte("name: my-idp\ntype: GOOGLE\n")
	s.False(documentMatchesResourceType(doc, "identity_provider"))
}

func (s *ResourcesFileTestSuite) TestDocumentMatchesResourceType_QuotedValue() {
	doc := []byte("resource_type: \"translation\"\nlanguage: en-US\n")
	s.True(documentMatchesResourceType(doc, "translation"))
}

func (s *ResourcesFileTestSuite) TestDocumentMatchesResourceType_WithTemplateVariables() {
	doc := []byte("resource_type: application\nname: Console\nclient_id: {{.CONSOLE_CLIENT_ID}}\n")
	s.True(documentMatchesResourceType(doc, "application"))
}

// ─── GetConfigsFromFile ───────────────────────────────────────────────────────

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_ReturnsOnlyMatchingType() {
	content := `resource_type: identity_provider
name: google-idp
type: GOOGLE
---
resource_type: flow
handle: signin
name: Sign-in Flow
---
resource_type: identity_provider
name: github-idp
type: GITHUB
`
	filePath := s.writeResourcesFile(content)

	configs, err := GetConfigsFromFile(filePath, "identity_provider")

	s.NoError(err)
	s.Len(configs, 2)
	s.Contains(string(configs[0])+string(configs[1]), "google-idp")
	s.Contains(string(configs[0])+string(configs[1]), "github-idp")
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_NoMatchingType_ReturnsEmpty() {
	content := `resource_type: flow
handle: signin
---
resource_type: flow
handle: signup
`
	filePath := s.writeResourcesFile(content)

	configs, err := GetConfigsFromFile(filePath, "identity_provider")

	s.NoError(err)
	s.Empty(configs)
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_SubstitutesEnvVars() {
	s.setEnvVar("TEST_IDP_CLIENT_ID", "my-client-id-123")

	content := `resource_type: identity_provider
name: oauth-idp
clientId: {{.TEST_IDP_CLIENT_ID}}
`
	filePath := s.writeResourcesFile(content)

	configs, err := GetConfigsFromFile(filePath, "identity_provider")

	s.NoError(err)
	s.Len(configs, 1)
	s.Contains(string(configs[0]), "my-client-id-123")
	s.NotContains(string(configs[0]), "{{.TEST_IDP_CLIENT_ID}}")
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_NonGoTemplateExpressionsPassThrough() {
	// Flow document contains {{ t(...) }} and {{meta(...)}} — these are UI expressions
	// that must not be mangled by the env-var substitution step.
	content := `resource_type: flow
handle: signin
name: Sign-in Flow
meta: '{"label":"{{ t(signin:title) }}","url":"{{meta(application.url)}}"}'
`
	filePath := s.writeResourcesFile(content)

	configs, err := GetConfigsFromFile(filePath, "flow")

	s.NoError(err)
	s.Len(configs, 1)
	s.Contains(string(configs[0]), `{{ t(signin:title) }}`)
	s.Contains(string(configs[0]), `{{meta(application.url)}}`)
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_BareIdentifierExpressionsPassThrough() {
	// Translation documents may contain {{appName}} (no dot) as a runtime placeholder.
	content := `resource_type: translation
language: en-US
translations:
  complete: Welcome back to {{appName}}
`
	filePath := s.writeResourcesFile(content)

	configs, err := GetConfigsFromFile(filePath, "translation")

	s.NoError(err)
	s.Len(configs, 1)
	s.Contains(string(configs[0]), "{{appName}}")
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_EnvVarAndNonGoTemplateInSameFile() {
	// Application doc has {{.Var}} substitution; flow doc in the same file has {{ t(...) }}.
	// Each is processed independently — the flow doc must never cause a parse error.
	s.setEnvVar("TEST_CLIENT_ID", "app-client-id")

	content := `resource_type: application
name: My App
clientId: {{.TEST_CLIENT_ID}}
---
resource_type: flow
handle: signin
meta: '{"label":"{{ t(signin:title) }}"}'
`
	filePath := s.writeResourcesFile(content)

	appConfigs, err := GetConfigsFromFile(filePath, "application")
	s.NoError(err)
	s.Len(appConfigs, 1)
	s.Contains(string(appConfigs[0]), "app-client-id")

	flowConfigs, err := GetConfigsFromFile(filePath, "flow")
	s.NoError(err)
	s.Len(flowConfigs, 1)
	s.Contains(string(flowConfigs[0]), `{{ t(signin:title) }}`)
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_MissingEnvVar_ReturnsError() {
	content := `resource_type: identity_provider
name: oauth-idp
clientId: {{.MISSING_ENV_VAR_XYZ}}
`
	filePath := s.writeResourcesFile(content)

	configs, err := GetConfigsFromFile(filePath, "identity_provider")

	s.Error(err)
	s.Nil(configs)
	s.Contains(err.Error(), "MISSING_ENV_VAR_XYZ")
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_FileNotFound_ReturnsError() {
	configs, err := GetConfigsFromFile(filepath.Join(s.T().TempDir(), "does-not-exist.yaml"), "identity_provider")

	s.Error(err)
	s.Nil(configs)
	s.Contains(err.Error(), "failed to read resources file")
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_EmptyFile_ReturnsEmpty() {
	filePath := s.writeResourcesFile("")

	configs, err := GetConfigsFromFile(filePath, "identity_provider")

	s.NoError(err)
	s.Empty(configs)
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_MultipleTypesInFile() {
	content := `resource_type: organization_unit
handle: default
name: Default OU
---
resource_type: identity_provider
name: google-idp
type: GOOGLE
---
resource_type: translation
language: en-US
translations:
  title: Sign In
---
resource_type: flow
handle: signin
name: Sign-in Flow
`
	filePath := s.writeResourcesFile(content)

	ouConfigs, err := GetConfigsFromFile(filePath, "organization_unit")
	s.NoError(err)
	s.Len(ouConfigs, 1)

	idpConfigs, err := GetConfigsFromFile(filePath, "identity_provider")
	s.NoError(err)
	s.Len(idpConfigs, 1)

	translationConfigs, err := GetConfigsFromFile(filePath, "translation")
	s.NoError(err)
	s.Len(translationConfigs, 1)

	flowConfigs, err := GetConfigsFromFile(filePath, "flow")
	s.NoError(err)
	s.Len(flowConfigs, 1)
}

func (s *ResourcesFileTestSuite) TestGetConfigsFromFile_RelativePathResolvesFromCWD() {
	tmpDir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(tmpDir, "resources.yaml"),
		[]byte("resource_type: flow\nhandle: x\n"), 0600))

	origDir, err := os.Getwd()
	s.Require().NoError(err)
	s.Require().NoError(os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Pass a relative path — the function must resolve it against the current working directory.
	configs, err := GetConfigsFromFile("resources.yaml", "flow")
	s.NoError(err)
	s.Len(configs, 1)
}

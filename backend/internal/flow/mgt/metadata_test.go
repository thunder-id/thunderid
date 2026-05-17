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

package flowmgt

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/executor"
)

type FlowsMgtMetadataTestSuite struct {
	suite.Suite
}

func TestFlowsMgtMetadataTestSuite(t *testing.T) {
	suite.Run(t, new(FlowsMgtMetadataTestSuite))
}

func (s *FlowsMgtMetadataTestSuite) TestCatalogJSONParsing() {
	var elements []ElementItem
	s.NoError(json.Unmarshal(catalogElementsJSON, &elements), "elements catalog must parse cleanly")
	s.NotEmpty(elements)

	var steps []StepItem
	s.NoError(json.Unmarshal(catalogStepsJSON, &steps), "steps catalog must parse cleanly")
	s.NotEmpty(steps)

	var actions []ActionItem
	s.NoError(json.Unmarshal(catalogActionsJSON, &actions), "actions catalog must parse cleanly")
	s.NotEmpty(actions)

	var templates []TemplateItem
	s.NoError(json.Unmarshal(catalogTemplatesJSON, &templates), "templates catalog must parse cleanly")
	s.NotEmpty(templates)

	var executors []ExecutorItem
	s.NoError(json.Unmarshal(catalogExecutorsJSON, &executors), "executors catalog must parse cleanly")
	s.NotEmpty(executors)
}

func (s *FlowsMgtMetadataTestSuite) TestAllExecutorsCatalogHasEntry() {
	knownExecutors := []string{
		executor.ExecutorNameBasicAuth,
		executor.ExecutorNameSMSAuth,
		executor.ExecutorNameMagicLinkAuth,
		executor.ExecutorNamePasskeyAuth,
		executor.ExecutorNameOAuth,
		executor.ExecutorNameOIDCAuth,
		executor.ExecutorNameGitHubAuth,
		executor.ExecutorNameGoogleAuth,
		executor.ExecutorNameIdentifying,
		executor.ExecutorNameAuthAssert,
		executor.ExecutorNameProvisioning,
		executor.ExecutorNameAttributeCollect,
		executor.ExecutorNameAuthorization,
		executor.ExecutorNamePermissionValidator,
		executor.ExecutorNameOUCreation,
		executor.ExecutorNameHTTPRequest,
		executor.ExecutorNameUserTypeResolver,
		executor.ExecutorNameInviteExecutor,
		executor.ExecutorNameEmailExecutor,
		executor.ExecutorNameCredentialSetter,
		executor.ExecutorNameConsent,
		executor.ExecutorNameOUResolver,
		executor.ExecutorNameAttributeUniquenessValidator,
		executor.ExecutorNameSMSExecutor,
		executor.ExecutorNameFederatedAuthResolver,
	}

	s.Require().NoError(initCatalog())
	catalogByName := make(map[string]struct{}, len(parsedExecutors))
	for _, e := range parsedExecutors {
		catalogByName[e.Name] = struct{}{}
	}

	for _, name := range knownExecutors {
		s.Contains(catalogByName, name, "executor %q has no metadata catalog entry", name)
	}
}

func (s *FlowsMgtMetadataTestSuite) TestExecutorCatalogRequiredFields() {
	s.Require().NoError(initCatalog())
	for _, e := range parsedExecutors {
		s.NotEmpty(e.Name, "executor catalog entry must have a name")
		s.NotEmpty(e.DisplayName, "executor %q must have a displayName", e.Name)
		s.NotNil(e.SupportedFlowTypes, "executor %q must have supportedFlowTypes", e.Name)
		s.NotEmpty(e.SupportedFlowTypes, "executor %q must support at least one flow type", e.Name)
		s.NotNil(e.DefaultInputs, "executor %q must have a defaultInputs slice (may be empty)", e.Name)
		s.NotNil(e.Properties, "executor %q must have a properties slice (may be empty)", e.Name)
	}
}

func (s *FlowsMgtMetadataTestSuite) TestTemplateItems_RequiredFields() {
	s.Require().NoError(initCatalog())
	for i, t := range parsedTemplates {
		name, _ := t["name"].(string)
		s.NotEmpty(name, "template[%d] must have a name", i)
		s.NotEmpty(t["displayName"], "template %q must have a displayName", name)
		s.NotEmpty(t["flowType"], "template %q must have a flowType", name)
		s.NotEmpty(t["category"], "template %q must have a category", name)
		s.NotNil(t["config"], "template %q must have a config", name)
	}
}

func (s *FlowsMgtMetadataTestSuite) TestElementItems_RequiredFields() {
	s.Require().NoError(initCatalog())
	for i, e := range parsedElements {
		name, _ := e["name"].(string)
		s.NotEmpty(name, "element[%d] must have a name", i)
		s.NotEmpty(e["displayName"], "element %q must have a displayName", name)
		s.NotEmpty(e["category"], "element %q must have a category", name)
	}
}

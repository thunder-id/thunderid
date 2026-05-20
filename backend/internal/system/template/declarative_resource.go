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

package template

import (
	"fmt"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"

	"gopkg.in/yaml.v3"
)

// loadDeclarativeResources loads template resources from YAML files.
func loadDeclarativeResources(store *templateFileBasedStore) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Template",
		DirectoryName: "templates",
		Parser:        parseToTemplateDTO,
		Validator:     validateTemplateDTO,
		IDExtractor: func(dto interface{}) string {
			return dto.(*TemplateDTO).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, store)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load template resources: %w", err)
	}

	return nil
}

// parseToTemplateDTO converts raw YAML data into a TemplateDTO.
func parseToTemplateDTO(data []byte) (interface{}, error) {
	var tmpl TemplateDTO
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// validateTemplateDTO ensures the provided object is a TemplateDTO and that required fields are set.
func validateTemplateDTO(dto interface{}) error {
	tmpl, ok := dto.(*TemplateDTO)
	if !ok {
		return fmt.Errorf("invalid type: expected *TemplateDTO")
	}

	if tmpl.ID == "" {
		return fmt.Errorf("template ID is required")
	}
	if tmpl.Scenario == "" {
		return fmt.Errorf("template scenario is required")
	}
	if !IsValidScenario(tmpl.Scenario) {
		return fmt.Errorf("unsupported template scenario: %s", tmpl.Scenario)
	}
	if tmpl.Type != TemplateTypeSMS && tmpl.Subject == "" {
		return fmt.Errorf("template subject is required")
	}
	if tmpl.Body == "" {
		return fmt.Errorf("template body is required")
	}

	return nil
}

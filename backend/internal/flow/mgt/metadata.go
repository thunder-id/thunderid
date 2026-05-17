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
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executor"
)

// Flow metadata catalog JSON data embedded from the meta/ directory at compile time.

//go:embed meta/elements.json
var catalogElementsJSON []byte

//go:embed meta/steps.json
var catalogStepsJSON []byte

//go:embed meta/actions.json
var catalogActionsJSON []byte

//go:embed meta/templates.json
var catalogTemplatesJSON []byte

//go:embed meta/executors.json
var catalogExecutorsJSON []byte

var (
	parsedElements  []ElementItem
	parsedSteps     []StepItem
	parsedActions   []ActionItem
	parsedTemplates []TemplateItem
	parsedExecutors []ExecutorItem
)

// initCatalog parses the embedded JSON catalog data. Returns an error if any catalog fails
// to parse, which should be treated as a fatal startup error.
func initCatalog() error {
	for _, entry := range []struct {
		data   []byte
		target interface{}
		name   string
	}{
		{catalogElementsJSON, &parsedElements, "elements"},
		{catalogStepsJSON, &parsedSteps, "steps"},
		{catalogActionsJSON, &parsedActions, "actions"},
		{catalogTemplatesJSON, &parsedTemplates, "templates"},
		{catalogExecutorsJSON, &parsedExecutors, "executors"},
	} {
		if err := json.Unmarshal(entry.data, entry.target); err != nil {
			return fmt.Errorf("failed to parse %s catalog: %w", entry.name, err)
		}
	}
	return nil
}

// flowTypeCatalog returns the supported list of flow types with their metadata.
func flowTypeCatalog() []FlowTypeItem {
	return []FlowTypeItem{
		{
			Name:        string(common.FlowTypeAuthentication),
			DisplayName: "Sign-in",
			Description: "User login flows.",
		},
		{
			Name:        string(common.FlowTypeRegistration),
			DisplayName: "Sign-up",
			Description: "User self-registration flows.",
		},
		{
			Name:        string(common.FlowTypeUserOnboarding),
			DisplayName: "User Onboarding",
			Description: "User onboarding flows.",
		},
		{
			Name:        string(common.FlowTypeRecovery),
			DisplayName: "Recovery",
			Description: "Account recovery and password reset flows.",
		},
	}
}

// nodeTypeCatalog returns the supported list of node types with their metadata and allowed fields for validation.
func nodeTypeCatalog() []NodeTypeItem {
	return []NodeTypeItem{
		{
			Name:          string(common.NodeTypeStart),
			DisplayName:   "Start",
			Description:   "Entry point of the flow.",
			AllowedFields: []string{"onSuccess"},
		},
		{
			Name:          string(common.NodeTypePrompt),
			DisplayName:   "Prompt",
			Description:   "Interactive UI step that displays components and collects user input.",
			AllowedFields: []string{"meta", "inputs", "actions", "next", "message"},
		},
		{
			Name:          string(common.NodeTypeTaskExecution),
			DisplayName:   "Task Execution",
			Description:   "Runs a server-side executor (authentication, provisioning, etc.).",
			AllowedFields: []string{"executor", "inputs", "properties", "onSuccess", "onFailure", "onIncomplete"},
		},
		{
			Name:          string(common.NodeTypeEnd),
			DisplayName:   "End",
			Description:   "Terminal node indicating the end of the flow.",
			AllowedFields: []string{},
		},
	}
}

// componentTypeCatalog returns the supported list of component types with their metadata and allowed
// properties for validation.
func componentTypeCatalog() []ComponentTypeItem {
	return []ComponentTypeItem{
		{
			Name:        common.MetaComponentTypeBlock,
			DisplayName: "Block",
			Description: "Container for grouping components.",
			Category:    "LAYOUT",
			Properties:  []string{"components"},
		},
		{
			Name:        common.MetaComponentTypeAction,
			DisplayName: "Action",
			Description: "Button or clickable element.",
			Category:    "ACTION",
			Properties:  []string{"label", "variant", "eventType"},
		},
		{
			Name:        common.MetaComponentTypeText,
			DisplayName: "Text",
			Description: "Display text element.",
			Category:    "DISPLAY",
			Properties:  []string{"label", "variant", "align"},
		},
		{
			Name:        common.MetaComponentTypeRichText,
			DisplayName: "Rich Text",
			Description: "HTML rich text display element.",
			Category:    "DISPLAY",
			Properties:  []string{"label"},
		},
		{
			Name:        common.MetaComponentTypeImage,
			DisplayName: "Image",
			Description: "Image display element.",
			Category:    "DISPLAY",
			Properties:  []string{"src", "alt", "width", "height"},
		},
		{
			Name:        common.MetaComponentTypeIcon,
			DisplayName: "Icon",
			Description: "Icon display element.",
			Category:    "DISPLAY",
			Properties:  []string{"name", "size", "color"},
		},
		{
			Name:        common.MetaComponentTypeDivider,
			DisplayName: "Divider",
			Description: "Visual separator.",
			Category:    "DISPLAY",
			Properties:  []string{"label", "variant"},
		},
		{
			Name:        common.MetaComponentTypeTimer,
			DisplayName: "Timer",
			Description: "Countdown timer display.",
			Category:    "DISPLAY",
			Properties:  []string{"label"},
		},
		{
			Name:        common.MetaComponentTypeStack,
			DisplayName: "Stack",
			Description: "Horizontal/vertical layout container.",
			Category:    "LAYOUT",
			Properties:  []string{"items", "direction", "gap", "align", "justify"},
		},
		{
			Name:        common.MetaComponentTypeTextInput,
			DisplayName: "Text Input",
			Description: "Free-text input field.",
			Category:    "INPUT",
			Properties:  []string{"label", "placeholder", "required", "hint"},
		},
		{
			Name:        common.MetaComponentTypePasswordInput,
			DisplayName: "Password Input",
			Description: "Masked password input field.",
			Category:    "INPUT",
			Properties:  []string{"label", "placeholder", "required", "hint"},
		},
		{
			Name:        common.MetaComponentTypeEmailInput,
			DisplayName: "Email Input",
			Description: "Email address input field.",
			Category:    "INPUT",
			Properties:  []string{"label", "placeholder", "required", "hint"},
		},
		{
			Name:        common.MetaComponentTypePhoneInput,
			DisplayName: "Phone Input",
			Description: "Phone number input field.",
			Category:    "INPUT",
			Properties:  []string{"label", "placeholder", "required", "hint"},
		},
		{
			Name:        common.MetaComponentTypeNumberInput,
			DisplayName: "Number Input",
			Description: "Numeric input field.",
			Category:    "INPUT",
			Properties:  []string{"label", "placeholder", "required", "hint"},
		},
		{
			Name:        common.MetaComponentTypeDateInput,
			DisplayName: "Date Input",
			Description: "Date picker input field.",
			Category:    "INPUT",
			Properties:  []string{"label", "placeholder", "required", "hint"},
		},
		{
			Name:        common.MetaComponentTypeOTPInput,
			DisplayName: "OTP Input",
			Description: "One-time passcode input field.",
			Category:    "INPUT",
			Properties:  []string{"label", "required", "length"},
		},
		{
			Name:        common.MetaComponentTypeCheckboxInput,
			DisplayName: "Checkbox",
			Description: "Boolean checkbox input.",
			Category:    "INPUT",
			Properties:  []string{"label", "required", "hint"},
		},
		{
			Name:        common.MetaComponentTypeSelectInput,
			DisplayName: "Select",
			Description: "Single-choice select input.",
			Category:    "INPUT",
			Properties:  []string{"label", "placeholder", "required", "options"},
		},
		{
			Name:        common.MetaComponentTypeDropdownInput,
			DisplayName: "Dropdown",
			Description: "Choice dropdown input.",
			Category:    "INPUT",
			Properties:  []string{"label", "required", "options", "defaultValue"},
		},
		{
			Name:        common.MetaComponentTypeResendAction,
			DisplayName: "Resend",
			Description: "Resend action button.",
			Category:    "ACTION",
			Properties:  []string{"label"},
		},
		{
			Name:        common.MetaComponentTypeCAPTCHA,
			DisplayName: "CAPTCHA",
			Description: "CAPTCHA challenge element.",
			Category:    "CAPTCHA",
			Properties:  []string{"variant"},
		},
		{
			Name:        common.MetaComponentTypeCustom,
			DisplayName: "Custom",
			Description: "Custom component.",
			Category:    "MISCELLANEOUS",
			Properties:  []string{},
		},
		{
			Name:        common.MetaComponentTypeDynamicInputPlaceholder,
			DisplayName: "Dynamic Input Placeholder",
			Description: "Insertion point for dynamically derived input components.",
			Category:    "LAYOUT",
			Properties:  []string{},
		},
	}
}

// inputTypeCatalog returns the supported list of input types for executor inputs and their metadata.
func inputTypeCatalog() []InputTypeItem {
	return []InputTypeItem{
		{
			Name:        common.InputTypeText,
			DisplayName: "Text",
			Description: "Free-text input.",
			Category:    "INPUT",
		},
		{
			Name:        common.InputTypeEmail,
			DisplayName: "Email",
			Description: "Email address input.",
			Category:    "INPUT",
		},
		{
			Name:        common.InputTypePassword,
			DisplayName: "Password",
			Description: "Password input.",
			Category:    "INPUT",
		},
		{
			Name:        common.InputTypeOTP,
			DisplayName: "OTP",
			Description: "One-time passcode.",
			Category:    "INPUT",
		},
		{
			Name:        common.InputTypePhone,
			DisplayName: "Phone",
			Description: "Phone number input.",
			Category:    "INPUT",
		},
		{
			Name:        common.InputTypeConsent,
			DisplayName: "Consent",
			Description: "Consent decision input",
			Category:    "INPUT",
		},
		{
			Name:        common.InputTypeHidden,
			DisplayName: "Hidden",
			Description: "Non-rendered value.",
			Category:    "INPUT",
		},
		{
			Name:        common.InputTypeSelect,
			DisplayName: "Select",
			Description: "Dropdown selection input.",
			Category:    "INPUT",
		},
		{
			Name:        common.InputTypeOUSelect,
			DisplayName: "OU Select",
			Description: "Organizational unit selection input.",
			Category:    "INPUT",
		},
	}
}

// registeredExecutorCatalog returns the list of executors that are registered in the system
// along with their metadata.
// TODO: Migrate executor metadata to executor-side self-registration.
func registeredExecutorCatalog(registry executor.ExecutorRegistryInterface) []ExecutorItem {
	registered := make([]ExecutorItem, 0, len(parsedExecutors))
	for _, e := range parsedExecutors {
		if registry.IsRegistered(e.Name) {
			registered = append(registered, e)
		}
	}
	return registered
}

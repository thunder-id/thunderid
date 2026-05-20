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

// TemplateType represents the type of template (e.g., email, sms).
type TemplateType string

const (
	// TemplateTypeEmail represents an email template.
	TemplateTypeEmail TemplateType = "email"
	// TemplateTypeSMS represents an SMS template.
	TemplateTypeSMS TemplateType = "sms"
)

// ScenarioType represents the scenario for which a template is used.
type ScenarioType string

const (
	// ScenarioUserInvite represents the user invitation scenario.
	ScenarioUserInvite ScenarioType = "USER_INVITE"
	// ScenarioMagicLink represents the magic link sign-in scenario.
	ScenarioMagicLink ScenarioType = "MAGIC_LINK"
	// ScenarioSelfRegistration represents the self-registration via invite link scenario.
	ScenarioSelfRegistration ScenarioType = "SELF_REGISTRATION"
	// ScenarioOTP represents the OTP verification scenario.
	ScenarioOTP ScenarioType = "OTP"
	// ScenarioPasswordRecovery represents the password recovery via email link scenario.
	ScenarioPasswordRecovery ScenarioType = "PASSWORD_RECOVERY"
)

// supportedScenarios contains all valid scenario types.
var supportedScenarios = map[ScenarioType]bool{
	ScenarioUserInvite:       true,
	ScenarioMagicLink:        true,
	ScenarioSelfRegistration: true,
	ScenarioOTP:              true,
	ScenarioPasswordRecovery: true,
}

// IsValidScenario checks if the given scenario type is supported.
func IsValidScenario(scenario ScenarioType) bool {
	return supportedScenarios[scenario]
}

// TemplateDTO represents a template with embedded metadata.
type TemplateDTO struct {
	ID          string       `yaml:"id"`
	DisplayName string       `yaml:"displayName"`
	Scenario    ScenarioType `yaml:"scenario"`
	Type        TemplateType `yaml:"type"`
	Subject     string       `yaml:"subject"`
	ContentType string       `yaml:"contentType"`
	Body        string       `yaml:"body"`
}

// TemplateData holds key-value pairs for template substitution.
type TemplateData = map[string]string

// RenderedTemplate holds the result after template processing.
type RenderedTemplate struct {
	Subject string
	Body    string
	IsHTML  bool
}

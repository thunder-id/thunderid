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

package bundle

import (
	"strings"
	"testing"
)

func TestSecretVariablesFindsCredentialPlaceholders(t *testing.T) {
	resources := `resource_type: user
attributes:
  username: "admin2@wso2.com"
credentials:
  password: "{{.ADMIN2_WSO2_COM_PASSWORD}}"
---
resource_type: application
name: My App
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: {{.MY_APP_CLIENT_ID}}
      clientSecret: {{.MY_APP_CLIENT_SECRET}}
      redirectUris:
        {{- range .MY_APP_REDIRECT_URIS}}
        - {{.}}
        {{- end}}
`

	got := SecretVariables(resources)
	want := []string{"ADMIN2_WSO2_COM_PASSWORD", "MY_APP_CLIENT_SECRET"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestMissingVariablesIgnoresACredentialPlaceholder(t *testing.T) {
	resources := "credentials:\n  password: \"{{.ADMIN2_WSO2_COM_PASSWORD}}\"\n"

	// A password comes from the secret service, so reporting it as an unset variable would send an
	// operator to set a value that is not theirs to set.
	if missing := MissingVariables(resources, nil, SecretVariables(resources)); len(missing) != 0 {
		t.Fatalf("a credential placeholder is not a missing variable, got %v", missing)
	}
	// It is still missing as a plain variable when nothing classifies it, which is the old behavior.
	if missing := MissingVariables(resources, nil, nil); len(missing) != 1 {
		t.Fatalf("expected the placeholder to be reported without classification, got %v", missing)
	}
}

// A notification sender's credential is held by the secret service like any other, so its
// placeholder must not be reported as a variable an operator should set.
func TestSecretVariablesClassifiesASenderCredential(t *testing.T) {
	resources := `resource_type: connection
type: twilio
name: Twilio
accountSid: ACxxxx
authToken: {{.TWILIO_AUTH_TOKEN}}
---
resource_type: connection
type: vonage
name: Vonage
apiKey: {{.VONAGE_API_KEY}}
apiSecret: {{.VONAGE_API_SECRET}}
`

	got := SecretVariables(resources)
	want := []string{"TWILIO_AUTH_TOKEN", "VONAGE_API_KEY", "VONAGE_API_SECRET"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestStripCredentialLinesRemovesOnlyTheNamedCredentialFields(t *testing.T) {
	resources := "resource_type: application\nid: app-a\nname: app-a\n" +
		"clientSecret: {{.APP_CLIENT_SECRET}}\nredirectUri: {{.APP_REDIRECT_URI}}\n"

	stripped := StripCredentialLines(resources, []string{"APP_CLIENT_SECRET"})

	// The whole field goes: an empty credential is rejected as invalid by several resource types,
	// while an absent one simply leaves the resource without it.
	if strings.Contains(stripped, "APP_CLIENT_SECRET") {
		t.Fatalf("the credential field should be gone, got %q", stripped)
	}
	// Configuration placeholders are not credentials and must survive.
	if !strings.Contains(stripped, "{{.APP_REDIRECT_URI}}") || !strings.Contains(stripped, "name: app-a") {
		t.Fatalf("only the named credential should be removed, got %q", stripped)
	}
}

// A deployment's default resource server identifier is the audience its tokens are bound to. Captured
// verbatim it would travel to every environment promoted to, so each would name the audience of the
// one it was captured from. Only the origin is replaced, so an operator's chosen path survives.
func TestTemplateDeploymentURLReplacesOnlyTheOrigin(t *testing.T) {
	resources := `resource_type: resource_server
id: rs-1
name: System
identifier: "https://dev.example.com/mcp"
---
resource_type: server_config
name: defaultResourceServer
value: {"resourceServerId":"rs-1"}`

	got := TemplateDeploymentURL(resources)

	if !strings.Contains(got, `identifier: "{{.DEPLOYMENT_URL}}/mcp"`) {
		t.Fatalf("expected the origin templated and the path kept, got:\n%s", got)
	}
}

// Only the resource server the default points at is this deployment's own. Every other one is
// configuration an operator authored, and promoting it changed would be rewriting their work.
func TestTemplateDeploymentURLLeavesOtherResourceServersAlone(t *testing.T) {
	resources := `resource_type: resource_server
id: rs-1
name: System
identifier: "https://dev.example.com/mcp"
---
resource_type: resource_server
id: rs-2
name: Payments
identifier: "https://payments.example.com/api"
---
resource_type: server_config
name: defaultResourceServer
value: {"resourceServerId":"rs-1"}`

	got := TemplateDeploymentURL(resources)

	if !strings.Contains(got, `identifier: "https://payments.example.com/api"`) {
		t.Fatalf("an authored resource server must be promoted as it stands, got:\n%s", got)
	}
	if !strings.Contains(got, `identifier: "{{.DEPLOYMENT_URL}}/mcp"`) {
		t.Fatalf("expected the deployment's own identifier templated, got:\n%s", got)
	}
}

// A bundle from a deployment with no default configured has no audience of its own to template.
func TestTemplateDeploymentURLIsAnIdentityWithoutADefault(t *testing.T) {
	resources := `resource_type: resource_server
id: rs-1
identifier: "https://dev.example.com/mcp"`

	if got := TemplateDeploymentURL(resources); got != resources {
		t.Fatalf("expected the bundle unchanged, got:\n%s", got)
	}
}

// The templated placeholder has to be a variable the apply actually resolves, or every promotion
// would report it missing.
func TestTemplatedDeploymentURLIsARequiredVariable(t *testing.T) {
	resources := TemplateDeploymentURL(`resource_type: resource_server
id: rs-1
identifier: "https://dev.example.com/mcp"
---
resource_type: server_config
name: defaultResourceServer
value: {"resourceServerId":"rs-1"}`)

	scalars, _ := RequiredVariables(resources)

	for _, name := range scalars {
		if name == DeploymentURLVariable {
			return
		}
	}
	t.Fatalf("expected %s among the required variables, got %v", DeploymentURLVariable, scalars)
}

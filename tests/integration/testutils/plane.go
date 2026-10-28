// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

// Plane is the deployment the integration run tests. A run tests one plane: the all-in-one server
// under make test_integration, the Control Plane under make test_integration_cp.
type Plane string

const (
	// DataPlane is the all-in-one server, which authenticates its own administrator.
	DataPlane Plane = "data"
	// ControlPlane is the Control Plane binary, which trusts an external issuer for management
	// tokens.
	ControlPlane Plane = "control"

	// PlaneEnv names the plane under test. main.go reads it, and the test processes inherit it.
	PlaneEnv = "INTEGRATION_PLANE"
	// PersonaTokensEnv carries the personas' tokens, as a JSON object keyed by persona, from main.go
	// to the test processes.
	PersonaTokensEnv = "TEST_PERSONA_TOKENS"
)

// Persona is a management identity a test can act as. A suite asks for it by name, so the same
// test runs against each plane and the token behind the name is the plane's to provide.
type Persona string

const (
	// PersonaAdmin holds the system scope and is the identity GetHTTPClient sends on every plane.
	PersonaAdmin Persona = "admin"
	// PersonaNoScope is an identity the plane authenticates but does not authorize, since its token
	// lacks the system scope. Only the Control Plane provides it.
	PersonaNoScope Persona = "no-scope"
	// PersonaStranger is an identity the Control Plane's issuer signs tokens for but whose record its
	// user endpoint does not hold. Only the Control Plane provides it.
	PersonaStranger Persona = "stranger"
)

// CurrentPlane returns the plane the run tests.
func CurrentPlane() Plane {
	if os.Getenv(PlaneEnv) == string(ControlPlane) {
		return ControlPlane
	}
	return DataPlane
}

// RequirePlane skips the test unless the run tests the given plane.
func RequirePlane(t *testing.T, plane Plane) {
	t.Helper()
	if CurrentPlane() != plane {
		t.Skipf("the test is for the %s plane, and the run tests the %s plane", plane, CurrentPlane())
	}
}

// SetPersonaTokens makes tokens the ones PersonaClient sends for the other personas, and exports
// them to the test processes. PersonaAdmin is not among them: it is the token of
// ObtainAdminAccessToken or UseAdminAccessToken.
func SetPersonaTokens(tokens map[Persona]string) error {
	encoded, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("failed to serialize the persona tokens: %w", err)
	}
	return os.Setenv(PersonaTokensEnv, string(encoded))
}

// PersonaClient returns a client that sends the persona's token. It reports false when the run's
// plane does not provide the persona, so a caller can skip what needs it.
func PersonaClient(persona Persona) (*http.Client, bool) {
	if persona == PersonaAdmin {
		return GetHTTPClient(), true
	}
	var tokens map[Persona]string
	if raw := os.Getenv(PersonaTokensEnv); raw != "" {
		if err := json.Unmarshal([]byte(raw), &tokens); err != nil {
			return nil, false
		}
	}
	token, ok := tokens[persona]
	if !ok || token == "" {
		return nil, false
	}
	return GetHTTPClientWithToken(token), true
}

// RequirePersonaClient returns PersonaClient's client, and skips the test when the run's plane does
// not provide the persona.
func RequirePersonaClient(t *testing.T, persona Persona) *http.Client {
	t.Helper()
	client, ok := PersonaClient(persona)
	if !ok {
		t.Skipf("the %s plane does not provide the %s persona", CurrentPlane(), persona)
	}
	return client
}

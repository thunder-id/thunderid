// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// The Control Plane suite runs the Control Plane binary inside the distribution build_backend
// packages: the binary reads the same layout (config/, dbscripts/, bootstrap/) and the bundled
// setup.sh bootstraps it as it does the all-in-one server. Only the binary and deployment.yaml
// differ.
const (
	// ControlPlaneBinaryPath is where build_cp_backend leaves the Control Plane binary.
	ControlPlaneBinaryPath = "../../target/out/.build/thunderid-cp"
	// ControlPlaneDeploymentYamlPath is the configuration the Control Plane suite runs with.
	ControlPlaneDeploymentYamlPath = "./resources/controlplane/deployment.yaml"

	// TrustedIssuerPort is where the suite's stand-in for an external identity provider answers.
	// A Control Plane issues no tokens, so the management tokens the suite uses come from it. It
	// must match server.security.trusted_issuer in ControlPlaneDeploymentYamlPath.
	TrustedIssuerPort = 8091
	// TrustedIssuerAudience is the audience the Control Plane accepts a trusted issuer's token for.
	TrustedIssuerAudience = "thunderid-control-plane"
	// TrustedIssuerAdminSubject is the user the suite's management tokens name, and
	// TrustedIssuerAdminName the username the issuer's user endpoint reports for them.
	TrustedIssuerAdminSubject = "control-plane-admin"
	TrustedIssuerAdminName    = "cp-admin"
	// TrustedIssuerStrangerSubject is a user the issuer signs tokens for but whose record its user
	// endpoint does not hold, so the Control Plane can only show them by id.
	TrustedIssuerStrangerSubject = "control-plane-stranger"
	// TrustedIssuerLookupClientID and TrustedIssuerLookupClientSecret are the client the Control Plane
	// presents when it looks a user's name up at the issuer. They must match
	// server.security.trusted_issuer.user_lookup in ControlPlaneDeploymentYamlPath.
	TrustedIssuerLookupClientID     = "control-plane-lookup"
	TrustedIssuerLookupClientSecret = "control-plane-lookup-secret"
)

// UseControlPlane makes ReplaceResources install the Control Plane's deployment.yaml.
func UseControlPlane() {
	deploymentYamlSource = ControlPlaneDeploymentYamlPath
}

// InstallControlPlaneBinary puts the Control Plane binary where the server binary was, so setup.sh,
// StartServer and StopServer drive it unchanged.
func InstallControlPlaneBinary() error {
	ensureInitialized()
	source := ControlPlaneBinaryPath
	if runtime.GOOS == "windows" {
		source += ".exe"
	}
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("control plane binary not found at %s, run build_cp_backend first: %w", source, err)
	}
	dest := filepath.Join(extractedProductHome, ServerBinary)
	if err := copyFile(source, dest); err != nil {
		return fmt.Errorf("failed to install the control plane binary: %w", err)
	}
	return os.Chmod(dest, 0o755)
}

// StartTrustedIssuer starts the identity provider the Control Plane trusts for management tokens.
func StartTrustedIssuer() (*MockOIDCServer, error) {
	issuer, err := NewMockOIDCServer(TrustedIssuerPort, TrustedIssuerLookupClientID, TrustedIssuerLookupClientSecret)
	if err != nil {
		return nil, err
	}
	// The administrator the management tokens name, so the Control Plane can put a name to them.
	issuer.AddUser(&OIDCUserInfo{Sub: TrustedIssuerAdminSubject, Name: TrustedIssuerAdminName})
	if err := issuer.Start(); err != nil {
		return nil, err
	}
	return issuer, nil
}

// IssueManagementToken signs an access token for the Control Plane, as an external identity
// provider would issue one to an administrator.
func IssueManagementToken(issuer *MockOIDCServer, scope string, validFor time.Duration) (string, error) {
	return IssueManagementTokenFor(issuer, TrustedIssuerAdminSubject, scope, validFor)
}

// IssueManagementTokenFor signs an access token for the Control Plane naming the given user.
func IssueManagementTokenFor(issuer *MockOIDCServer, subject, scope string,
	validFor time.Duration) (string, error) {
	now := time.Now()
	claims := map[string]interface{}{
		"iss": issuer.GetURL(),
		"aud": TrustedIssuerAudience,
		"sub": subject,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": now.Add(validFor).Unix(),
	}
	if scope != "" {
		claims["scope"] = scope
	}
	return issuer.SignJWT(map[string]interface{}{"typ": "at+jwt"}, claims)
}

// UseAdminAccessToken makes token the one every test client sends, in place of the one
// ObtainAdminAccessToken would obtain from this server, and exports it to the test packages. It is
// never refreshed, so it must outlive the run.
func UseAdminAccessToken(token string, validFor time.Duration) error {
	adminTokenState = &TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   validFor.Seconds(),
		ExpiresAt:   time.Now().Add(validFor),
	}
	return exportTokenStateToEnv()
}

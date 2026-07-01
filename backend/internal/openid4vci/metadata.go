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

package openid4vci

import "github.com/thunder-id/thunderid/internal/openid4vci/credential"

// buildMetadata assembles the OpenID4VCI credential issuer metadata document
// served at /.well-known/openid-credential-issuer. It advertises one entry per
// managed credential configuration and the credential/nonce endpoints.
func buildMetadata(cfg serviceConfig, creds []credential.CredentialConfigurationDTO) map[string]interface{} {
	configs := make(map[string]interface{}, len(creds))
	for _, c := range creds {
		format := c.Format
		if format == "" {
			format = credential.DefaultCredentialFormat
		}
		entry := map[string]interface{}{
			"format": format,
			"scope":  c.Handle,
			"vct":    c.VCT,
			"cryptographic_binding_methods_supported": []string{"jwk"},
			"credential_signing_alg_values_supported": []string{"ES256"},
			"proof_types_supported": map[string]interface{}{
				"jwt": map[string]interface{}{
					"proof_signing_alg_values_supported": []string{"ES256"},
				},
			},
		}
		if d := credentialDisplay(c.Display); d != nil {
			entry["display"] = d
		}
		if cl := credentialClaims(c.Claims); cl != nil {
			entry["claims"] = cl
		}
		configs[c.Handle] = entry
	}

	metadata := map[string]interface{}{
		"credential_issuer":                   cfg.CredentialIssuer,
		"credential_endpoint":                 cfg.BaseURL + credentialPath,
		"nonce_endpoint":                      cfg.BaseURL + noncePath,
		"credential_configurations_supported": configs,
	}
	if len(cfg.AuthorizationServers) > 0 {
		metadata["authorization_servers"] = cfg.AuthorizationServers
	}
	if cfg.BatchSize > 1 {
		metadata["batch_credential_issuance"] = map[string]interface{}{"batch_size": cfg.BatchSize}
	}
	return metadata
}

// credentialClaims builds the per-claim display map for the metadata document.
// Only claims with a DisplayName set are included; returns nil when none qualify.
func credentialClaims(claims []credential.ClaimMapping) map[string]interface{} {
	out := make(map[string]interface{}, len(claims))
	for _, c := range claims {
		if c.DisplayName == "" {
			continue
		}
		out[c.Name] = map[string]interface{}{
			"display": []interface{}{
				map[string]interface{}{"name": c.DisplayName},
			},
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// credentialDisplay maps a managed display to the metadata display array, or nil.
func credentialDisplay(d *credential.CredentialDisplay) []interface{} {
	if d == nil {
		return nil
	}
	entry := map[string]interface{}{}
	if d.Name != "" {
		entry["name"] = d.Name
	}
	if d.Locale != "" {
		entry["locale"] = d.Locale
	}
	if d.LogoURI != "" {
		entry["logo"] = map[string]interface{}{"uri": d.LogoURI}
	}
	if len(entry) == 0 {
		return nil
	}
	return []interface{}{entry}
}

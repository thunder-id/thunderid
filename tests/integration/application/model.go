/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package application

// Application represents the structure for application request and response in tests.
type Application struct {
	ID                        string              `json:"id,omitempty"`
	OUID                      string              `json:"ouId,omitempty"`
	Name                      string              `json:"name"`
	Description               string              `json:"description,omitempty"`
	ClientID                  string              `json:"clientId,omitempty"`
	ClientSecret              string              `json:"clientSecret,omitempty"`
	AuthFlowID                string              `json:"authFlowId,omitempty"`
	RegistrationFlowID        string              `json:"registrationFlowId,omitempty"`
	IsRegistrationFlowEnabled bool                `json:"isRegistrationFlowEnabled"`
	ThemeID                   string              `json:"themeId,omitempty"`
	LayoutID                  string              `json:"layoutId,omitempty"`
	Template                  string              `json:"template,omitempty"`
	URL                       string              `json:"url,omitempty"`
	LogoURL                   string              `json:"logoUrl,omitempty"`
	Certificate               *ApplicationCert    `json:"certificate,omitempty"`
	Assertion                 *AssertionConfig    `json:"assertion,omitempty"`
	TosURI                    string              `json:"tosUri,omitempty"`
	PolicyURI                 string              `json:"policyUri,omitempty"`
	Contacts                  []string            `json:"contacts,omitempty"`
	AllowedUserTypes          []string            `json:"allowedUserTypes,omitempty"`
	LoginConsent              *LoginConsentConfig `json:"loginConsent,omitempty"`
	InboundAuthConfig         []InboundAuthConfig `json:"inboundAuthConfig,omitempty"`
}

// ApplicationCert represents the certificate structure in the application.
type ApplicationCert struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// InboundAuthConfig represents the inbound authentication configuration.
type InboundAuthConfig struct {
	Type           string          `json:"type"`
	OAuthAppConfig *OAuthAppConfig `json:"config,omitempty"`
}

// OAuthAppConfig represents the OAuth application configuration.
type OAuthAppConfig struct {
	ClientID                string              `json:"clientId"`
	ClientSecret            string              `json:"clientSecret,omitempty"`
	RedirectURIs            []string            `json:"redirectUris"`
	GrantTypes              []string            `json:"grantTypes"`
	ResponseTypes           []string            `json:"responseTypes"`
	TokenEndpointAuthMethod string              `json:"tokenEndpointAuthMethod"`
	PKCERequired            bool                `json:"pkceRequired"`
	PublicClient            bool                `json:"publicClient"`
	Scopes                  []string            `json:"scopes,omitempty"`
	Token                   *OAuthTokenConfig   `json:"token,omitempty"`
	ScopeClaims             map[string][]string `json:"scopeClaims,omitempty"`
	UserInfo                *UserInfoConfig     `json:"userInfo,omitempty"`
	Certificate             *ApplicationCert    `json:"certificate,omitempty"`
	AcrValues               []string            `json:"acrValues,omitempty"`
}

// OAuthTokenConfig represents the OAuth token configuration.
type OAuthTokenConfig struct {
	AccessToken *AccessTokenConfig `json:"accessToken,omitempty"`
	IDToken     *IDTokenConfig     `json:"idToken,omitempty"`
}

// UserInfoConfig represents the UserInfo endpoint configuration.
type UserInfoConfig struct {
	ResponseType   string   `json:"responseType,omitempty"`
	SigningAlg     string   `json:"signingAlg,omitempty"`
	EncryptionAlg  string   `json:"encryptionAlg,omitempty"`
	EncryptionEnc  string   `json:"encryptionEnc,omitempty"`
	UserAttributes []string `json:"userAttributes,omitempty"`
}

// AssertionConfig represents the assertion configuration (used for application-level assertion config).
type AssertionConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty"`
	UserAttributes []string `json:"userAttributes,omitempty"`
}

// LoginConsentConfig represents the login consent configuration for an application.
type LoginConsentConfig struct {
	ValidityPeriod int64 `json:"validityPeriod,omitempty"`
}

// AccessTokenConfig represents the access token configuration, split by token subject.
type AccessTokenConfig struct {
	UserConfig   *AccessTokenSubConfig `json:"userConfig,omitempty"`
	ClientConfig *AccessTokenSubConfig `json:"clientConfig,omitempty"`
}

// AccessTokenSubConfig represents the validity period and attribute selection for one
// access token subject type (user or client).
type AccessTokenSubConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty"`
	Attributes     []string `json:"attributes,omitempty"`
}

// IDTokenConfig represents the ID token configuration.
type IDTokenConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty"`
	UserAttributes []string `json:"userAttributes,omitempty"`
	ResponseType   string   `json:"responseType,omitempty"`
	EncryptionAlg  string   `json:"encryptionAlg,omitempty"`
	EncryptionEnc  string   `json:"encryptionEnc,omitempty"`
}

// ApplicationList represents the response structure for listing applications.
type ApplicationList struct {
	TotalResults int           `json:"totalResults"`
	Count        int           `json:"count"`
	Applications []Application `json:"applications"`
}

func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (app *Application) equals(expectedApp Application) bool {
	// Basic fields
	if app.ID != expectedApp.ID ||
		app.Name != expectedApp.Name ||
		app.Description != expectedApp.Description {
		return false
	}

	// For ClientID, we need to handle it being in both the root and OAuth config
	if app.ClientID != expectedApp.ClientID {
		return false
	}

	// Auth flow fields
	if app.AuthFlowID != expectedApp.AuthFlowID ||
		app.RegistrationFlowID != expectedApp.RegistrationFlowID ||
		app.IsRegistrationFlowEnabled != expectedApp.IsRegistrationFlowEnabled {
		return false
	}

	// Theme and Layout IDs
	if app.ThemeID != expectedApp.ThemeID || app.LayoutID != expectedApp.LayoutID {
		return false
	}

	// Template
	if app.Template != expectedApp.Template {
		return false
	}

	// URL fields
	if app.URL != expectedApp.URL ||
		app.LogoURL != expectedApp.LogoURL {
		return false
	}

	// Metadata fields
	if app.TosURI != expectedApp.TosURI ||
		app.PolicyURI != expectedApp.PolicyURI {
		return false
	}

	// Contacts
	if !compareStringSlices(app.Contacts, expectedApp.Contacts) {
		return false
	}

	// AllowedUserTypes
	if !compareStringSlices(app.AllowedUserTypes, expectedApp.AllowedUserTypes) {
		return false
	}

	// Assertion config
	if (app.Assertion != nil) && (expectedApp.Assertion != nil) {
		if app.Assertion.ValidityPeriod != expectedApp.Assertion.ValidityPeriod {
			return false
		}
		if !compareStringSlices(app.Assertion.UserAttributes, expectedApp.Assertion.UserAttributes) {
			return false
		}
	} else if (app.Assertion == nil && expectedApp.Assertion != nil) ||
		(app.Assertion != nil && expectedApp.Assertion == nil) {
		return false
	}

	// LoginConsent config
	if (app.LoginConsent != nil) && (expectedApp.LoginConsent != nil) {
		if app.LoginConsent.ValidityPeriod != expectedApp.LoginConsent.ValidityPeriod {
			return false
		}
	} else if app.LoginConsent == nil && expectedApp.LoginConsent != nil {
		// Expected a LoginConsent object but absent
		return false
	} else if app.LoginConsent != nil && expectedApp.LoginConsent == nil {
		// Actual has LoginConsent object but expected omitted it
		return false
	}

	// ClientSecret is only checked when both have it (create/update operations)
	// Don't check it for get operations where it shouldn't be returned
	if app.ClientSecret != "" && expectedApp.ClientSecret != "" &&
		app.ClientSecret != expectedApp.ClientSecret {
		return false
	}

	// Check certificate
	if (app.Certificate == nil) != (expectedApp.Certificate == nil) {
		return false
	}
	if app.Certificate != nil && expectedApp.Certificate != nil {
		if app.Certificate.Type != expectedApp.Certificate.Type ||
			app.Certificate.Value != expectedApp.Certificate.Value {
			return false
		}
	}

	// Check inbound auth config if present
	if len(app.InboundAuthConfig) != len(expectedApp.InboundAuthConfig) {
		return false
	}

	// Compare inbound auth config details
	if len(app.InboundAuthConfig) > 0 {
		for i, cfg := range app.InboundAuthConfig {
			expectedCfg := expectedApp.InboundAuthConfig[i]
			if cfg.Type != expectedCfg.Type {
				return false
			}

			// Compare OAuth configs if they exist
			if cfg.OAuthAppConfig != nil && expectedCfg.OAuthAppConfig != nil {
				oauth := cfg.OAuthAppConfig
				expectedOAuth := expectedCfg.OAuthAppConfig

				// Compare the fields
				if oauth.ClientID != expectedOAuth.ClientID {
					return false
				}

				if !compareStringSlices(oauth.RedirectURIs, expectedOAuth.RedirectURIs) {
					return false
				}

				if !compareStringSlices(oauth.GrantTypes, expectedOAuth.GrantTypes) {
					return false
				}

				if !compareStringSlices(oauth.ResponseTypes, expectedOAuth.ResponseTypes) {
					return false
				}

				if oauth.TokenEndpointAuthMethod != expectedOAuth.TokenEndpointAuthMethod {
					return false
				}

				if oauth.PKCERequired != expectedOAuth.PKCERequired {
					return false
				}

				if oauth.PublicClient != expectedOAuth.PublicClient {
					return false
				}

				if !compareStringSlices(oauth.AcrValues, expectedOAuth.AcrValues) {
					return false
				}

				// Compare ScopeClaims - lenient if expected is nil but actual is empty
				if expectedOAuth.ScopeClaims != nil {
					if !compareScopeClaimsMaps(oauth.ScopeClaims, expectedOAuth.ScopeClaims) {
						return false
					}
				}

				// Compare UserInfo config - lenient if expected is nil but actual is empty
				if expectedOAuth.UserInfo != nil {
					if oauth.UserInfo == nil {
						return false
					}
					if oauth.UserInfo.ResponseType != expectedOAuth.UserInfo.ResponseType {
						return false
					}
					if oauth.UserInfo.SigningAlg != expectedOAuth.UserInfo.SigningAlg {
						return false
					}
					if oauth.UserInfo.EncryptionAlg != expectedOAuth.UserInfo.EncryptionAlg {
						return false
					}
					if oauth.UserInfo.EncryptionEnc != expectedOAuth.UserInfo.EncryptionEnc {
						return false
					}
					if !compareStringSlices(oauth.UserInfo.UserAttributes, expectedOAuth.UserInfo.UserAttributes) {
						return false
					}
				}
				// If expected UserInfo is nil, we accept any value in actual (including empty object)

				// Compare OAuth certificate
				if (oauth.Certificate == nil) != (expectedOAuth.Certificate == nil) {
					return false
				}
				if oauth.Certificate != nil && expectedOAuth.Certificate != nil {
					if oauth.Certificate.Type != expectedOAuth.Certificate.Type ||
						oauth.Certificate.Value != expectedOAuth.Certificate.Value {
						return false
					}
				}
			} else if (cfg.OAuthAppConfig == nil && expectedCfg.OAuthAppConfig != nil) ||
				(cfg.OAuthAppConfig != nil && expectedCfg.OAuthAppConfig == nil) {
				return false
			}
		}
	}

	return true
}

// compareScopeClaimsMaps compares two scope claims maps for equality.
func compareScopeClaimsMaps(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, aVal := range a {
		bVal, exists := b[key]
		if !exists {
			return false
		}
		if !compareStringSlices(aVal, bVal) {
			return false
		}
	}
	return true
}
